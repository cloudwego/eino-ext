/*
 * Copyright 2024 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package qianfan

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"

	"github.com/baidubce/bce-qianfan-sdk/go/qianfan"

	"github.com/cloudwego/eino/components"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

var _ model.ToolCallingChatModel = (*ChatModel)(nil)

// GetQianfanSingletonConfig qianfan config is singleton, you should set ak+sk / bear_token before init chat model
// Set with code: GetQianfanSingletonConfig().AccessKey = "your_access_key"
// Set with env: os.Setenv("QIANFAN_ACCESS_KEY", "your_iam_ak") or with env file
func GetQianfanSingletonConfig() *qianfan.Config {
	return qianfan.GetConfig()
}

// ChatModelConfig config for qianfan chat completion
// see: https://cloud.baidu.com/doc/WENXINWORKSHOP/s/Wm3fhy2vb
type ChatModelConfig struct {
	Model                 string   // 使用的模型
	LLMRetryCount         *int     // 重试次数
	LLMRetryTimeout       *float32 // 重试超时时间
	LLMRetryBackoffFactor *float32 // 重试退避因子

	Temperature         *float32 // 较高的数值会使输出更加随机，而较低的数值会使其更加集中和确定，默认 0.95，范围 (0, 1.0]
	TopP                *float32 // 影响输出文本的多样性，取值越大，生成文本的多样性越强。默认 0.7，取值范围 [0, 1.0]
	PenaltyScore        *float64 // 通过对已生成的token增加惩罚，减少重复生成的现象。说明：值越大表示惩罚越大，取值范围：[1.0, 2.0]
	MaxCompletionTokens *int     // 指定模型最大输出token数, [2, 2048]
	Seed                *int     // 随机种子, (0,2147483647‌）
	Stop                []string // 生成停止标识，当模型生成结果以stop中某个元素结尾时，停止文本生成
	User                *string  // 表示最终用户的唯一标识符
	FrequencyPenalty    *float64 // 指定频率惩罚，用于控制生成文本的重复程度。取值范围 [-2.0, 2.0]
	PresencePenalty     *float64 // 指定存在惩罚，用于控制生成文本的重复程度。取值范围 [-2.0, 2.0]
	ParallelToolCalls   *bool    // 是否并行调用工具, 默认开启
	ResponseFormat      *qianfan.ResponseFormat
}

type ChatModel struct {
	cc         *qianfan.ChatCompletionV2
	rawTools   []*schema.ToolInfo
	tools      []qianfan.Tool
	toolChoice *schema.ToolChoice
	config     *ChatModelConfig
}

func NewChatModel(ctx context.Context, config *ChatModelConfig) (*ChatModel, error) {
	opts := []qianfan.Option{qianfan.WithModel(config.Model)}
	if config.LLMRetryCount != nil {
		opts = append(opts, qianfan.WithLLMRetryCount(*config.LLMRetryCount))
	}
	if config.LLMRetryTimeout != nil {
		opts = append(opts, qianfan.WithLLMRetryTimeout(*config.LLMRetryTimeout))
	}
	if config.LLMRetryBackoffFactor != nil {
		opts = append(opts, qianfan.WithLLMRetryBackoffFactor(*config.LLMRetryBackoffFactor))
	}

	if config.Temperature == nil {
		config.Temperature = of(defaultTemperature)
	}
	if config.TopP == nil {
		config.TopP = of(defaultTopP)
	}
	if config.ParallelToolCalls == nil {
		config.ParallelToolCalls = of(defaultParallelToolCalls)
	}

	cc := qianfan.NewChatCompletionV2(opts...)

	return &ChatModel{cc, nil, nil, nil, config}, nil
}

func (cm *ChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (
	outMsg *schema.Message, err error) {

	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)

	req, cbInput, err := cm.genRequest(input, false, opts...)
	if err != nil {
		return nil, err
	}

	ctx = callbacks.OnStart(ctx, cbInput)
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	r, err := cm.cc.Do(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("[qianfan][Generate] ChatCompletionV2 error, %w", err)
	}

	outMsg, err = resolveQianfanResponse(r)
	if err != nil {
		return nil, fmt.Errorf("[qianfan][Generate] resolve resp failed, %w", err)
	}

	ctx = callbacks.OnEnd(ctx, &model.CallbackOutput{
		Message:    outMsg,
		Config:     cbInput.Config,
		TokenUsage: toModelCallbackUsage(outMsg),
	})

	return outMsg, nil
}

func (cm *ChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (
	outStream *schema.StreamReader[*schema.Message], err error) {

	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)

	req, cbInput, err := cm.genRequest(input, true, opts...)
	if err != nil {
		return nil, err
	}

	ctx = callbacks.OnStart(ctx, cbInput)
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	r, err := cm.cc.Stream(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("[qianfan][Stream] ChatCompletionV2 error, %w", err)
	}

	sr, sw := schema.Pipe[*model.CallbackOutput](1)
	go func() {
		defer func() {
			if pe := recover(); pe != nil {
				_ = sw.Send(nil, newPanicErr(pe, debug.Stack()))
			}

			r.Close()
			sw.Close()
		}()

		for !r.IsEnd {
			item := &qianfan.ChatCompletionV2Response{}
			if e := r.Recv(item); e != nil {
				sw.Send(nil, e)
				return
			}

			msg, found, err := resolveQianfanStreamResponse(item)
			if err != nil {
				sw.Send(nil, err)
				return
			}

			if !found {
				continue
			}

			if closed := sw.Send(&model.CallbackOutput{
				Message:    msg,
				Config:     cbInput.Config,
				TokenUsage: toModelCallbackUsage(msg),
			}, nil); closed {
				return
			}
		}

	}()

	ctx, nsr := callbacks.OnEndWithStreamOutput(ctx, schema.StreamReaderWithConvert(
		sr, func(src *model.CallbackOutput) (callbacks.CallbackOutput, error) {
			return src, nil
		},
	))

	outStream = schema.StreamReaderWithConvert(nsr,
		func(src callbacks.CallbackOutput) (*schema.Message, error) {
			s := src.(*model.CallbackOutput)
			if s.Message == nil {
				return nil, schema.ErrNoValue
			}

			return s.Message, nil
		},
	)

	return outStream, nil
}

func (cm *ChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	if len(tools) == 0 {
		return nil, errors.New("no tools to bind")
	}
	qianfanTools, err := toQianfanTools(tools)
	if err != nil {
		return nil, fmt.Errorf("convert to qianfan tools fail: %w", err)
	}

	tc := schema.ToolChoiceAllowed
	ncm := *cm
	ncm.tools = qianfanTools
	ncm.rawTools = tools
	ncm.toolChoice = &tc
	return &ncm, nil
}

func (cm *ChatModel) BindTools(tools []*schema.ToolInfo) error {
	if len(tools) == 0 {
		return errors.New("no tools to bind")
	}
	var err error
	cm.tools, err = toQianfanTools(tools)
	if err != nil {
		return err
	}
	cm.rawTools = tools
	tc := schema.ToolChoiceAllowed
	cm.toolChoice = &tc
	return nil
}

func (cm *ChatModel) BindForcedTools(tools []*schema.ToolInfo) error {
	if len(tools) == 0 {
		return errors.New("no tools to bind")
	}
	var err error
	cm.tools, err = toQianfanTools(tools)
	if err != nil {
		return err
	}
	cm.rawTools = tools
	tc := schema.ToolChoiceForced
	cm.toolChoice = &tc
	return nil
}

func (cm *ChatModel) genRequest(input []*schema.Message, isStream bool, opts ...model.Option) (
	*qianfan.ChatCompletionV2Request, *model.CallbackInput, error) {

	options := model.GetCommonOptions(&model.Options{
		Temperature: cm.config.Temperature,
		MaxTokens:   cm.config.MaxCompletionTokens,
		Model:       &cm.config.Model,
		TopP:        cm.config.TopP,
		Stop:        cm.config.Stop,
		ToolChoice:  cm.toolChoice,
	}, opts...)

	cbInput := &model.CallbackInput{
		Messages: input,
		Tools:    cm.rawTools,
		Config: &model.Config{
			Model:       dereferenceOrZero(options.Model),
			MaxTokens:   dereferenceOrZero(options.MaxTokens),
			Temperature: dereferenceOrZero(options.Temperature),
			TopP:        dereferenceOrZero(options.TopP),
			Stop:        options.Stop,
		},
	}

	tools := cm.tools
	if options.Tools != nil {
		var err error
		if tools, err = toQianfanTools(options.Tools); err != nil {
			return nil, nil, err
		}
		cbInput.Tools = options.Tools
	}

	req := &qianfan.ChatCompletionV2Request{
		BaseRequestBody:     qianfan.BaseRequestBody{},
		Model:               *options.Model,
		Messages:            toQianfanMessages(input),
		StreamOptions:       nil,
		Temperature:         float64(dereferenceOrZero(options.Temperature)),
		TopP:                float64(dereferenceOrZero(options.TopP)),
		PenaltyScore:        dereferenceOrZero(cm.config.PenaltyScore),
		MaxCompletionTokens: dereferenceOrZero(options.MaxTokens),
		Seed:                dereferenceOrZero(cm.config.Seed),
		Stop:                options.Stop,
		User:                dereferenceOrZero(cm.config.User),
		FrequencyPenalty:    dereferenceOrZero(cm.config.FrequencyPenalty),
		PresencePenalty:     dereferenceOrZero(cm.config.PresencePenalty),
		Tools:               tools,
		ParallelToolCalls:   dereferenceOrZero(cm.config.ParallelToolCalls),
		ResponseFormat:      cm.config.ResponseFormat,
	}

	if isStream {
		req.StreamOptions = &qianfan.StreamOptions{IncludeUsage: true}
	}

	if options.ToolChoice != nil {
		switch *options.ToolChoice {
		case schema.ToolChoiceForbidden:
			req.ToolChoice = toolChoiceNone
		case schema.ToolChoiceAllowed:
			req.ToolChoice = toolChoiceAuto
		case schema.ToolChoiceForced:
			if len(req.Tools) == 0 {
				return nil, nil, fmt.Errorf("[qianfan][genRequest] tool choice is forced but tool is not provided")
			} else if len(req.Tools) > 1 {
				req.ToolChoice = toolChoiceRequired
			} else {
				req.ToolChoice = qianfan.ToolChoice{
					Type: "function",
					Function: &qianfan.Function{
						Name: req.Tools[0].Function.Name,
					},
				}
			}
		default:
			return nil, nil, fmt.Errorf("[qianfan][genRequest] tool choice=%s not support", *options.ToolChoice)
		}
	}

	return req, cbInput, nil
}

func toQianfanMessages(input []*schema.Message) []qianfan.ChatCompletionV2Message {
	r := make([]qianfan.ChatCompletionV2Message, len(input))
	for i, m := range input {
		msg := qianfan.ChatCompletionV2Message{
			Role:       string(m.Role),
			Content:    m.Content,
			Name:       m.Name,
			ToolCalls:  make([]qianfan.ToolCall, len(m.ToolCalls)),
			ToolCallId: m.ToolCallID,
		}

		for j, tc := range m.ToolCalls {
			msg.ToolCalls[j] = qianfan.ToolCall{
				Id:       tc.ID,
				ToolType: tc.Type,
				Function: qianfan.FunctionCallV2{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			}
		}

		r[i] = msg
	}

	return r
}

func resolveQianfanResponse(resp *qianfan.ChatCompletionV2Response) (*schema.Message, error) {
	if resp.Error != nil {
		return nil, fmt.Errorf("[resolveQianfanResponse] resp with err: code=%s, msg=%s, type=%s",
			resp.Error.Code, resp.Error.Message, resp.Error.Type)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("[resolveQianfanResponse] choice is empty")
	}

	var choice *qianfan.ChatCompletionV2Choice
	for i, c := range resp.Choices {
		if c.Index == 0 {
			choice = &resp.Choices[i]
			break
		}
	}

	if choice == nil {
		return nil, fmt.Errorf("[resolveQianfanResponse] unexpected choices without index=0")
	}

	if choice.Message.Content == "" && len(choice.Message.ToolCalls) == 0 {
		return nil, fmt.Errorf("[resolveQianfanResponse] unexpected message with empty content and tool calls")
	}

	msg := &schema.Message{
		Role:       schema.RoleType(choice.Message.Role),
		Content:    choice.Message.Content,
		Name:       choice.Message.Name,
		ToolCalls:  toMessageToolCalls(choice.Message.ToolCalls),
		ToolCallID: choice.Message.ToolCallId,
		ResponseMeta: &schema.ResponseMeta{
			FinishReason: choice.FinishReason,
			Usage:        toMessageTokenUsage(resp.Usage),
		},
	}

	return msg, nil
}

func resolveQianfanStreamResponse(resp *qianfan.ChatCompletionV2Response) (
	msg *schema.Message, found bool, err error) {
	if resp.Error != nil {
		return nil, false, fmt.Errorf("[resolveQianfanResponse] resp with err: code=%s, msg=%s, type=%s",
			resp.Error.Code, resp.Error.Message, resp.Error.Type)
	}

	for _, choice := range resp.Choices {
		if choice.Index != 0 {
			continue
		}
		found = true
		// delta role assistant see: https://cloud.baidu.com/doc/WENXINWORKSHOP/s/Fm2vrveyu#function-call%E7%A4%BA%E4%BE%8B
		msg = &schema.Message{
			Role:      schema.Assistant,
			Content:   choice.Delta.Content,
			ToolCalls: toMessageToolCalls(choice.Delta.ToolCalls),
			ResponseMeta: &schema.ResponseMeta{
				FinishReason: choice.FinishReason,
				Usage:        toMessageTokenUsage(resp.Usage),
			},
		}
		break
	}

	if !found && resp.Usage != nil {
		found = true
		msg = &schema.Message{
			ResponseMeta: &schema.ResponseMeta{
				Usage: toMessageTokenUsage(resp.Usage),
			},
		}
	}

	return msg, found, nil
}

func toMessageToolCalls(toolCalls []qianfan.ToolCall) []schema.ToolCall {
	if len(toolCalls) == 0 {
		return nil
	}

	ret := make([]schema.ToolCall, len(toolCalls))
	for i, toolCall := range toolCalls {
		idx := i
		ret[i] = schema.ToolCall{
			Index: &idx,
			ID:    toolCall.Id,
			Type:  toolCall.ToolType,
			Function: schema.FunctionCall{
				Name:      toolCall.Function.Name,
				Arguments: toolCall.Function.Arguments,
			},
		}
	}

	return ret
}

func toMessageTokenUsage(usage *qianfan.ModelUsage) *schema.TokenUsage {
	if usage == nil {
		return nil
	}

	return &schema.TokenUsage{
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
	}
}

func toModelCallbackUsage(msg *schema.Message) *model.TokenUsage {
	if msg == nil || msg.ResponseMeta == nil || msg.ResponseMeta.Usage == nil {
		return nil
	}

	return &model.TokenUsage{
		CompletionTokens: msg.ResponseMeta.Usage.CompletionTokens,
		PromptTokens:     msg.ResponseMeta.Usage.PromptTokens,
		TotalTokens:      msg.ResponseMeta.Usage.TotalTokens,
	}
}

func toQianfanTools(tools []*schema.ToolInfo) ([]qianfan.Tool, error) {
	if len(tools) == 0 {
		return nil, nil
	}

	r := make([]qianfan.Tool, len(tools))
	for i, tool := range tools {
		parameters, err := tool.ParamsOneOf.ToJSONSchema()
		if err != nil {
			return nil, err
		}

		r[i] = qianfan.Tool{
			ToolType: "function",
			Function: qianfan.FunctionV2{
				Name:        tool.Name,
				Description: tool.Desc,
				Parameters:  parameters,
			},
		}
	}

	return r, nil
}

func (cm *ChatModel) GetType() string {
	return getType()
}

func (cm *ChatModel) IsCallbacksEnabled() bool {
	return true
}

type panicErr struct {
	info  any
	stack []byte
}

func (p *panicErr) Error() string {
	return fmt.Sprintf("panic error: %v, \nstack: %s", p.info, string(p.stack))
}

func newPanicErr(info any, stack []byte) error {
	return &panicErr{
		info:  info,
		stack: stack,
	}
}

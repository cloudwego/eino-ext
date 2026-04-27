/*
 * Copyright 2025 CloudWeGo Authors
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

package minimax

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/param"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

var _ model.ToolCallingChatModel = (*ChatModel)(nil)

const defaultBaseURL = "https://api.minimax.io/anthropic"

func NewChatModel(ctx context.Context, config *Config) (*ChatModel, error) {
	if config.APIKey == "" {
		return nil, errors.New("APIKey is required")
	}
	if config.Model == "" {
		return nil, errors.New("model is required")
	}
	if config.MaxTokens <= 0 {
		return nil, errors.New("maxTokens is required and must be positive")
	}

	var opts []option.RequestOption

	opts = append(opts, option.WithAPIKey(config.APIKey))

	if config.BaseURL != nil {
		opts = append(opts, option.WithBaseURL(*config.BaseURL))
	} else {
		opts = append(opts, option.WithBaseURL(defaultBaseURL))
	}

	if config.HTTPClient != nil {
		opts = append(opts, option.WithHTTPClient(config.HTTPClient))
	}

	for key, value := range config.AdditionalHeaderFields {
		opts = append(opts, option.WithHeaderAdd(key, value))
	}

	for key, value := range config.AdditionalRequestFields {
		opts = append(opts, option.WithJSONSet(key, value))
	}

	cli := anthropic.NewClient(opts...)

	return &ChatModel{
		cli:         cli,
		maxTokens:   config.MaxTokens,
		model:       config.Model,
		temperature: config.Temperature,
		topP:        config.TopP,
	}, nil
}

type ChatModel struct {
	cli anthropic.Client

	maxTokens   int
	model       string
	temperature *float32
	topP        *float32

	tools      []anthropic.ToolUnionParam
	origTools  []*schema.ToolInfo
	toolChoice *schema.ToolChoice
}

func (cm *ChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (message *schema.Message, err error) {
	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)
	ctx = callbacks.OnStart(ctx, cm.getCallbackInput(input, opts...))
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	msgParam, err := cm.genMessageNewParams(input, opts...)
	if err != nil {
		return nil, err
	}

	resp, err := cm.cli.Messages.New(ctx, msgParam)
	if err != nil {
		return nil, fmt.Errorf("create new message fail: %w", convOrigAPIError(err))
	}

	message, err = convOutputMessage(resp)
	if err != nil {
		return nil, fmt.Errorf("convert response to schema message fail: %w", err)
	}

	callbacks.OnEnd(ctx, cm.getCallbackOutput(message))

	return message, nil
}

func (cm *ChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (result *schema.StreamReader[*schema.Message], err error) {
	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)
	ctx = callbacks.OnStart(ctx, cm.getCallbackInput(input, opts...))
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	msgParam, err := cm.genMessageNewParams(input, opts...)
	if err != nil {
		return nil, err
	}

	stream := cm.cli.Messages.NewStreaming(ctx, msgParam)
	if stream.Err() != nil {
		return nil, fmt.Errorf("create new streaming message fail: %w", convOrigAPIError(stream.Err()))
	}

	sr, sw := schema.Pipe[*model.CallbackOutput](1)
	go func() {
		defer func() {
			pe := recover()
			if pe != nil {
				_ = sw.Send(nil, newPanicErr(pe, debug.Stack()))
			}

			_ = stream.Close()
			sw.Close()
		}()
		var waitList []*schema.Message
		streamCtx := &streamContext{}
		for stream.Next() {
			message, err_ := convStreamEvent(stream.Current(), streamCtx)
			if err_ != nil {
				_ = sw.Send(nil, fmt.Errorf("convert response chunk to schema message fail: %w", err_))
				return
			}
			if message == nil {
				continue
			}
			if isMessageEmpty(message) {
				waitList = append(waitList, message)
				continue
			}

			if len(waitList) != 0 {
				message, err = schema.ConcatMessages(append(waitList, message))
				if err != nil {
					_ = sw.Send(nil, fmt.Errorf("concat empty message fail: %w", err))
					return
				}
				waitList = []*schema.Message{}
			}

			closed := sw.Send(cm.getCallbackOutput(message), nil)
			if closed {
				return
			}
		}

		if len(waitList) > 0 {
			message, err_ := schema.ConcatMessages(waitList)
			if err_ != nil {
				_ = sw.Send(nil, fmt.Errorf("concat empty message fail: %w", err_))
				return
			}

			closed := sw.Send(cm.getCallbackOutput(message), nil)
			if closed {
				return
			}
		}

		if stream.Err() != nil {
			_ = sw.Send(nil, stream.Err())
			return
		}

	}()

	_, sr = callbacks.OnEndWithStreamOutput(ctx, sr)
	return schema.StreamReaderWithConvert(sr, func(t *model.CallbackOutput) (*schema.Message, error) {
		return t.Message, nil
	}), nil
}

func (cm *ChatModel) GetType() string {
	return "minimax"
}

func (cm *ChatModel) IsCallbacksEnabled() bool {
	return true
}

func (cm *ChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	if len(tools) == 0 {
		return nil, errors.New("no tools to bind")
	}
	aTools, err := toAnthropicToolParam(tools)
	if err != nil {
		return nil, fmt.Errorf("to anthropic tool param fail: %w", err)
	}

	tc := schema.ToolChoiceAllowed
	ncm := *cm
	ncm.tools = aTools
	ncm.toolChoice = &tc
	ncm.origTools = tools
	return &ncm, nil
}

func (cm *ChatModel) BindTools(tools []*schema.ToolInfo) error {
	if len(tools) == 0 {
		return errors.New("no tools to bind")
	}
	result, err := toAnthropicToolParam(tools)
	if err != nil {
		return err
	}

	cm.tools = result
	cm.origTools = tools
	tc := schema.ToolChoiceAllowed
	cm.toolChoice = &tc
	return nil
}

func (cm *ChatModel) BindForcedTools(tools []*schema.ToolInfo) error {
	if len(tools) == 0 {
		return errors.New("no tools to bind")
	}
	result, err := toAnthropicToolParam(tools)
	if err != nil {
		return err
	}

	cm.tools = result
	cm.origTools = tools
	tc := schema.ToolChoiceForced
	cm.toolChoice = &tc
	return nil
}

func toAnthropicToolParam(tools []*schema.ToolInfo) ([]anthropic.ToolUnionParam, error) {
	if len(tools) == 0 {
		return nil, nil
	}

	result := make([]anthropic.ToolUnionParam, 0, len(tools))
	for _, tool := range tools {
		if tool == nil {
			return nil, errors.New("tool cannot be nil")
		}

		js, err := tool.ToJSONSchema()
		if err != nil {
			return nil, fmt.Errorf("convert to json schema fail: %w", err)
		}

		var inputSchema anthropic.ToolInputSchemaParam
		if js != nil {
			inputSchema = anthropic.ToolInputSchemaParam{
				Properties: js.Properties,
				Required:   js.Required,
			}
		}

		toolParam := &anthropic.ToolParam{
			Name:        tool.Name,
			Description: param.NewOpt(tool.Desc),
			InputSchema: inputSchema,
		}

		result = append(result, anthropic.ToolUnionParam{OfTool: toolParam})
	}

	return result, nil
}

func (cm *ChatModel) genMessageNewParams(input []*schema.Message, opts ...model.Option) (anthropic.MessageNewParams, error) {
	modelOpts := model.GetCommonOptions(&model.Options{
		Temperature: cm.temperature,
		TopP:        cm.topP,
		Model:       &cm.model,
		MaxTokens:   &cm.maxTokens,
		Tools:       nil,
		ToolChoice:  cm.toolChoice,
	}, opts...)

	specOptions := model.GetImplSpecificOptions(&options{}, opts...)

	params := anthropic.MessageNewParams{
		Model: anthropic.Model(*modelOpts.Model),
		MaxTokens: func() int64 {
			if modelOpts.MaxTokens != nil && *modelOpts.MaxTokens > 0 {
				return int64(*modelOpts.MaxTokens)
			}
			return int64(cm.maxTokens)
		}(),
	}

	if modelOpts.Temperature != nil {
		params.Temperature = param.NewOpt(float64(*modelOpts.Temperature))
	}

	if modelOpts.TopP != nil {
		params.TopP = param.NewOpt(float64(*modelOpts.TopP))
	}

	if specOptions.Thinking != nil && specOptions.Thinking.Enable {
		params.Thinking = anthropic.ThinkingConfigParamUnion{
			OfEnabled: &anthropic.ThinkingConfigEnabledParam{
				BudgetTokens: int64(specOptions.Thinking.BudgetTokens),
			},
		}
	}

	if len(input) > 0 {
		messages, system, err := splitSystemMessage(input)
		if err != nil {
			return params, err
		}

		if system != nil {
			params.System = []anthropic.TextBlockParam{
				{Text: system.Content},
			}
		}

		msgParams, err := convertMessages(messages)
		if err != nil {
			return params, fmt.Errorf("convert messages fail: %w", err)
		}
		params.Messages = msgParams
	}

	if len(cm.tools) > 0 {
		params.Tools = cm.tools
	}

	if cm.toolChoice != nil {
		tc, err := toAnthropicToolChoice(cm.toolChoice)
		if err != nil {
			return params, err
		}
		params.ToolChoice = tc
	}

	return params, nil
}

func splitSystemMessage(messages []*schema.Message) ([]*schema.Message, *schema.Message, error) {
	if len(messages) == 0 {
		return messages, nil, nil
	}

	if messages[0].Role == schema.System {
		return messages[1:], messages[0], nil
	}

	return messages, nil, nil
}

func convertMessages(messages []*schema.Message) ([]anthropic.MessageParam, error) {
	result := make([]anthropic.MessageParam, 0, len(messages))

	for _, msg := range messages {
		content, err := convertMessageContent(msg)
		if err != nil {
			return nil, err
		}

		result = append(result, anthropic.MessageParam{
			Role:    toAnthropicRole(msg.Role),
			Content: content,
		})
	}

	return result, nil
}

func toAnthropicRole(role schema.RoleType) anthropic.MessageParamRole {
	switch role {
	case schema.User:
		return "user"
	case schema.Assistant:
		return "assistant"
	case schema.System:
		return "system"
	case schema.Tool:
		return "user"
	default:
		return anthropic.MessageParamRole(role)
	}
}

func convertMessageContent(msg *schema.Message) ([]anthropic.ContentBlockParamUnion, error) {
	if msg.Content == "" && len(msg.ToolCalls) == 0 {
		return nil, nil
	}

	result := make([]anthropic.ContentBlockParamUnion, 0)

	if msg.Content != "" {
		result = append(result, anthropic.ContentBlockParamUnion{
			OfText: &anthropic.TextBlockParam{Text: msg.Content},
		})
	}

	for _, tc := range msg.ToolCalls {
		result = append(result, anthropic.ContentBlockParamUnion{
			OfToolUse: &anthropic.ToolUseBlockParam{
				ID:    tc.ID,
				Name:  tc.Function.Name,
				Input: tc.Function.Arguments,
			},
		})
	}

	return result, nil
}

func toAnthropicToolChoice(toolChoice *schema.ToolChoice) (anthropic.ToolChoiceUnionParam, error) {
	if toolChoice == nil {
		return anthropic.ToolChoiceUnionParam{}, nil
	}

	switch *toolChoice {
	case schema.ToolChoiceForbidden:
		return anthropic.ToolChoiceUnionParam{
			OfNone: &anthropic.ToolChoiceNoneParam{},
		}, nil
	case schema.ToolChoiceAllowed:
		return anthropic.ToolChoiceUnionParam{
			OfAuto: &anthropic.ToolChoiceAutoParam{},
		}, nil
	case schema.ToolChoiceForced:
		return anthropic.ToolChoiceUnionParam{
			OfAny: &anthropic.ToolChoiceAnyParam{},
		}, nil
	default:
		return anthropic.ToolChoiceUnionParam{}, fmt.Errorf("unknown tool choice: %v", toolChoice)
	}
}

type streamContext struct {
}

func convStreamEvent(event anthropic.MessageStreamEventUnion, streamCtx *streamContext) (*schema.Message, error) {
	switch e := event.AsAny().(type) {
	case anthropic.MessageStartEvent:
		return convOutputMessage(&e.Message)
	case anthropic.MessageDeltaEvent:
		result := &schema.Message{
			Role:         schema.Assistant,
			ResponseMeta: &schema.ResponseMeta{},
		}
		if e.Usage.OutputTokens > 0 || e.Usage.InputTokens > 0 {
			result.ResponseMeta.Usage = &schema.TokenUsage{
				CompletionTokens: int(e.Usage.OutputTokens),
				TotalTokens:      int(e.Usage.InputTokens) + int(e.Usage.OutputTokens),
			}
		}
		return result, nil
	case anthropic.MessageStopEvent, anthropic.ContentBlockStopEvent:
		return nil, nil
	case anthropic.ContentBlockStartEvent:
		result := &schema.Message{
			Role:  schema.Assistant,
			Extra: make(map[string]any),
		}
		_ = convContentBlockToEinoMsg(e.ContentBlock.AsAny(), result, streamCtx)
		return result, nil
	case anthropic.ContentBlockDeltaEvent:
		result := &schema.Message{
			Role:  schema.Assistant,
			Extra: make(map[string]any),
		}
		switch delta := e.Delta.AsAny().(type) {
		case anthropic.TextDelta:
			result.Content = delta.Text
		case anthropic.ThinkingDelta:
			setThinking(result, delta.Thinking)
			result.ReasoningContent = delta.Thinking
		case anthropic.InputJSONDelta:
			args := string(delta.PartialJSON)
			result.ToolCalls = append(result.ToolCalls, schema.ToolCall{
				Function: schema.FunctionCall{
					Arguments: args,
				},
			})
		}
		return result, nil
	default:
		return nil, nil
	}
}

func convContentBlockToEinoMsg(contentBlock any, dstMsg *schema.Message, streamCtx *streamContext) error {
	switch block := contentBlock.(type) {
	case anthropic.TextBlock:
		dstMsg.Content += block.Text
	case anthropic.ToolUseBlock:
		args := string(block.Input)
		dstMsg.ToolCalls = append(dstMsg.ToolCalls, schema.ToolCall{
			ID: block.ID,
			Function: schema.FunctionCall{
				Name:      string(block.Name),
				Arguments: args,
			},
		})
	case anthropic.ThinkingBlock:
		setThinking(dstMsg, block.Thinking)
		dstMsg.ReasoningContent = block.Thinking
	}
	return nil
}

func isMessageEmpty(message *schema.Message) bool {
	_, hasThinking := GetThinking(message)
	return message.Content == "" && len(message.ToolCalls) == 0 && !hasThinking
}

func convOutputMessage(resp *anthropic.Message) (*schema.Message, error) {
	message := &schema.Message{
		Role: schema.Assistant,
		ResponseMeta: &schema.ResponseMeta{
			FinishReason: string(resp.StopReason),
			Usage: &schema.TokenUsage{
				PromptTokens:     int(resp.Usage.InputTokens),
				CompletionTokens: int(resp.Usage.OutputTokens),
				TotalTokens:      int(resp.Usage.InputTokens) + int(resp.Usage.OutputTokens),
			},
		},
	}

	for _, item := range resp.Content {
		err := convContentBlockToEinoMsg(item.AsAny(), message, &streamContext{})
		if err != nil {
			return nil, err
		}
	}

	return message, nil
}

func (cm *ChatModel) getCallbackInput(messages []*schema.Message, opts ...model.Option) *model.CallbackInput {
	_ = model.GetCommonOptions(&model.Options{
		Temperature: cm.temperature,
		TopP:        cm.topP,
		Model:       &cm.model,
		MaxTokens:   &cm.maxTokens,
	}, opts...)

	return &model.CallbackInput{
		Messages:   messages,
		Tools:      cm.origTools,
		ToolChoice: cm.toolChoice,
		Config: &model.Config{
			Model:       cm.model,
			MaxTokens:   cm.maxTokens,
			Temperature: dereferenceOrZero(cm.temperature),
			TopP:        dereferenceOrZero(cm.topP),
		},
	}
}

func (cm *ChatModel) getCallbackOutput(message *schema.Message) *model.CallbackOutput {
	result := &model.CallbackOutput{
		Message: message,
		Config: &model.Config{
			Model:       cm.model,
			MaxTokens:   cm.maxTokens,
			Temperature: dereferenceOrZero(cm.temperature),
			TopP:        dereferenceOrZero(cm.topP),
		},
	}
	if message.ResponseMeta != nil && message.ResponseMeta.Usage != nil {
		result.TokenUsage = &model.TokenUsage{
			PromptTokens:     message.ResponseMeta.Usage.PromptTokens,
			CompletionTokens: message.ResponseMeta.Usage.CompletionTokens,
			TotalTokens:      message.ResponseMeta.Usage.TotalTokens,
		}
	}
	return result
}

func dereferenceOrZero[T any](v *T) T {
	if v == nil {
		var t T
		return t
	}
	return *v
}

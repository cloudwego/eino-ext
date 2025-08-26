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

package deepseek

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/cohesion-org/deepseek-go"
	"github.com/eino-contrib/jsonschema"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

var _ model.ToolCallingChatModel = (*ChatModel)(nil)

type ResponseFormatType string

const (
	ResponseFormatTypeText       = "text"
	ResponseFormatTypeJSONObject = "json_object"
)

const (
	toolChoiceNone     = "none"     // none means the model will not call any tool and instead generates a message.
	toolChoiceAuto     = "auto"     // auto means the model can pick between generating a message or calling one or more tools.
	toolChoiceRequired = "required" // required means the model must call one or more tools.
)

type ChatModelConfig struct {
	// APIKey is your authentication key
	// Required
	APIKey string `json:"api_key"`

	// Timeout specifies the maximum duration to wait for API responses
	// Optional. Default: 5 minutes
	Timeout time.Duration `json:"timeout"`

	// HTTPClient specifies the client to send HTTP requests.
	// Optional. Default http.DefaultClient
	HTTPClient *http.Client `json:"http_client"`

	// BaseURL is your custom deepseek endpoint url
	// Optional. Default: https://api.deepseek.com/
	BaseURL string `json:"base_url"`

	// Path sets the path for the API request. Defaults to "chat/completions", if not set.
	// Example usages would be "/c/chat/" or any http after the baseURL extension
	Path string `json:"path"`

	// The following fields correspond to DeepSeek's chat API parameters
	// Ref: https://api-docs.deepseek.com/api/create-chat-completion

	// Model specifies the ID of the model to use
	// Required
	Model string `json:"model"`

	// MaxTokens limits the maximum number of tokens that can be generated in the chat completion
	// Range: [1, 8192].
	// Optional. Default: 4096
	MaxTokens int `json:"max_tokens,omitempty"`

	// Temperature specifies what sampling temperature to use
	// Generally recommend altering this or TopP but not both.
	// Range: [0.0, 2.0]. Higher values make output more random
	// Optional. Default: 1.0
	Temperature float32 `json:"temperature,omitempty"`

	// TopP controls diversity via nucleus sampling
	// Generally recommend altering this or Temperature but not both.
	// Range: [0.0, 1.0]. Lower values make output more focused
	// Optional. Default: 1.0
	TopP float32 `json:"top_p,omitempty"`

	// Stop sequences where the API will stop generating further tokens
	// Optional. Example: []string{"\n", "User:"}
	Stop []string `json:"stop,omitempty"`

	// PresencePenalty prevents repetition by penalizing tokens based on presence
	// Range: [-2.0, 2.0]. Positive values increase likelihood of new topics
	// Optional. Default: 0
	PresencePenalty float32 `json:"presence_penalty,omitempty"`

	// ResponseFormat specifies the format of the model's response
	// Optional. Use for structured outputs
	ResponseFormatType ResponseFormatType `json:"response_format_type,omitempty"`

	// FrequencyPenalty prevents repetition by penalizing tokens based on frequency
	// Range: [-2.0, 2.0]. Positive values decrease likelihood of repetition
	// Optional. Default: 0
	FrequencyPenalty float32 `json:"frequency_penalty,omitempty"`

	// LogProbs specifies whether to return log probabilities of the output tokens.
	LogProbs bool `json:"log_probs"`

	// TopLogProbs specifies the number of most likely tokens to return at each token position, each with an associated log probability.
	TopLogProbs int `json:"top_log_probs"`
}

var _ model.ToolCallingChatModel = (*ChatModel)(nil)

type ChatModel struct {
	cli  *deepseek.Client
	conf *ChatModelConfig

	tools      []deepseek.Tool
	rawTools   []*schema.ToolInfo
	toolChoice *schema.ToolChoice
}

func NewChatModel(_ context.Context, config *ChatModelConfig) (*ChatModel, error) {
	if len(config.Model) == 0 {
		return nil, fmt.Errorf("model is required")
	}

	var opts []deepseek.Option
	if config.Timeout > 0 {
		opts = append(opts, deepseek.WithTimeout(config.Timeout))
	}
	if config.HTTPClient != nil {
		opts = append(opts, deepseek.WithHTTPClient(config.HTTPClient))
	}
	if len(config.BaseURL) > 0 {
		baseURL := config.BaseURL
		// sdk won't add '/' automatically
		if !strings.HasSuffix(baseURL, "/") {
			baseURL = baseURL + "/"
		}
		opts = append(opts, deepseek.WithBaseURL(baseURL))
	}
	if len(config.Path) > 0 {
		opts = append(opts, deepseek.WithPath(config.Path))
	}

	cli, err := deepseek.NewClientWithOptions(config.APIKey, opts...)
	if err != nil {
		return nil, err
	}
	return &ChatModel{cli: cli, conf: config}, nil
}

func toLogProbs(probs *deepseek.Logprobs) *schema.LogProbs {
	if probs == nil {
		return nil
	}
	ret := &schema.LogProbs{}
	for _, content := range probs.Content {
		schemaContent := schema.LogProb{
			Token:       content.Token,
			LogProb:     content.Logprob,
			Bytes:       intSlice2int64(content.Bytes),
			TopLogProbs: toTopLogProb(content.TopLogprobs),
		}
		ret.Content = append(ret.Content, schemaContent)
	}
	return ret
}

func toTopLogProb(probs []deepseek.TopLogprobToken) []schema.TopLogProb {
	ret := make([]schema.TopLogProb, 0, len(probs))
	for _, prob := range probs {
		ret = append(ret, schema.TopLogProb{
			Token:   prob.Token,
			LogProb: prob.Logprob,
			Bytes:   intSlice2int64(prob.Bytes),
		})
	}
	return ret
}

func intSlice2int64(in []int) []int64 {
	ret := make([]int64, 0, len(in))
	for _, v := range in {
		ret = append(ret, int64(v))
	}
	return ret
}

func (cm *ChatModel) Generate(ctx context.Context, in []*schema.Message, opts ...model.Option) (outMsg *schema.Message, err error) {

	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)

	req, cbInput, err := cm.generateRequest(ctx, in, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to generate request: %w", err)
	}

	ctx = callbacks.OnStart(ctx, cbInput)
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	resp, err := cm.cli.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat completion: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("received empty choices from DeepSeek API response")
	}

	for _, choice := range resp.Choices {
		if choice.Index != 0 {
			continue
		}

		lp, err := extractLogProbs(choice.Logprobs)
		if err != nil {
			return nil, fmt.Errorf("failed to extract log probs: %w", err)
		}
		outMsg = &schema.Message{
			Role:      toMessageRole(choice.Message.Role),
			Content:   choice.Message.Content,
			ToolCalls: toMessageToolCalls(choice.Message.ToolCalls),
			ResponseMeta: &schema.ResponseMeta{
				FinishReason: choice.FinishReason,
				Usage:        toEinoTokenUsage(&resp.Usage),
				LogProbs:     lp,
			},
		}
		if len(choice.Message.ReasoningContent) > 0 {
			SetReasoningContent(outMsg, choice.Message.ReasoningContent)
			outMsg.ReasoningContent = choice.Message.ReasoningContent
		}

		break
	}

	if outMsg == nil {
		return nil, fmt.Errorf("invalid response format: choice with index 0 not found")
	}

	callbacks.OnEnd(ctx, &model.CallbackOutput{
		Message:    outMsg,
		Config:     cbInput.Config,
		TokenUsage: toCallbackUsage(outMsg.ResponseMeta.Usage),
	})

	return outMsg, nil
}

func (cm *ChatModel) Stream(ctx context.Context, in []*schema.Message, opts ...model.Option) (outStream *schema.StreamReader[*schema.Message], err error) {

	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)

	req, cbInput, err := cm.generateStreamRequest(ctx, in, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to generate stream request: %w", err)
	}

	ctx = callbacks.OnStart(ctx, cbInput)
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	stream, err := cm.cli.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat stream completion: %w", err)
	}

	sr, sw := schema.Pipe[*model.CallbackOutput](1)
	go func() {
		defer func() {
			panicErr := recover()
			_ = stream.Close()

			if panicErr != nil {
				_ = sw.Send(nil, newPanicErr(panicErr, debug.Stack()))
			}

			sw.Close()
		}()

		var lastEmptyMsg *schema.Message

		for {
			chunk, chunkErr := stream.Recv()
			if errors.Is(chunkErr, io.EOF) {
				if lastEmptyMsg != nil {
					sw.Send(&model.CallbackOutput{
						Message:    lastEmptyMsg,
						Config:     cbInput.Config,
						TokenUsage: toModelCallbackUsage(lastEmptyMsg.ResponseMeta),
					}, nil)
				}
				return
			}

			if chunkErr != nil {
				_ = sw.Send(nil, fmt.Errorf("failed to receive stream chunk from DeepSeek: %w", chunkErr))
				return
			}

			msg, found, err := resolveStreamResponse(chunk)
			if err != nil {
				_ = sw.Send(nil, fmt.Errorf("failed to resolve stream response from DeepSeek: %w", err))
			}
			if !found {
				continue
			}

			if lastEmptyMsg != nil {
				cMsg, cErr := schema.ConcatMessages([]*schema.Message{lastEmptyMsg, msg})
				if cErr != nil {
					_ = sw.Send(nil, fmt.Errorf("failed to concatenate stream messages: %w", cErr))
					return
				}

				msg = cMsg
			}

			if msg.Content == "" && len(msg.ToolCalls) == 0 {
				if _, ok := GetReasoningContent(msg); !ok {
					lastEmptyMsg = msg
					continue
				}
			}

			lastEmptyMsg = nil

			closed := sw.Send(&model.CallbackOutput{
				Message:    msg,
				Config:     cbInput.Config,
				TokenUsage: toModelCallbackUsage(msg.ResponseMeta),
			}, nil)

			if closed {
				return
			}
		}

	}()

	ctx, nsr := callbacks.OnEndWithStreamOutput(ctx, schema.StreamReaderWithConvert(sr,
		func(src *model.CallbackOutput) (callbacks.CallbackOutput, error) {
			return src, nil
		}))

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
	deepseekTools, err := toTools(tools)
	if err != nil {
		return nil, fmt.Errorf("convert to deepseek tools fail: %w", err)
	}

	tc := schema.ToolChoiceAllowed
	ncm := *cm
	ncm.tools = deepseekTools
	ncm.rawTools = tools
	ncm.toolChoice = &tc
	return &ncm, nil
}

func (cm *ChatModel) BindTools(tools []*schema.ToolInfo) error {
	if len(tools) == 0 {
		return errors.New("no tools to bind")
	}
	var err error
	cm.tools, err = toTools(tools)
	if err != nil {
		return err
	}

	tc := schema.ToolChoiceAllowed
	cm.toolChoice = &tc
	cm.rawTools = tools

	return nil
}

func (cm *ChatModel) BindForcedTools(tools []*schema.ToolInfo) error {
	if len(tools) == 0 {
		return errors.New("no tools to bind")
	}
	var err error
	cm.tools, err = toTools(tools)
	if err != nil {
		return err
	}

	tc := schema.ToolChoiceForced
	cm.toolChoice = &tc
	cm.rawTools = tools

	return nil
}

func toTools(tis []*schema.ToolInfo) ([]deepseek.Tool, error) {
	tools := make([]deepseek.Tool, len(tis))
	for i := range tis {
		ti := tis[i]
		if ti == nil {
			return nil, fmt.Errorf("tool info cannot be nil in BindTools")
		}

		paramsJSONSchema, err := ti.ParamsOneOf.ToJSONSchema()
		if err != nil {
			return nil, fmt.Errorf("failed to convert tool parameters to JSONSchema: %w", err)
		}

		tools[i] = deepseek.Tool{
			Type: "function",
			Function: deepseek.Function{
				Name:        ti.Name,
				Description: ti.Desc,
				Parameters:  toToolParam(paramsJSONSchema),
			},
		}
	}

	return tools, nil
}

func toToolParam(s *jsonschema.Schema) *deepseek.FunctionParameters {
	if s == nil {
		return nil
	}
	ret := &deepseek.FunctionParameters{
		Type:       s.Type,
		Properties: map[string]interface{}{},
		Required:   nil,
	}
	if len(s.Required) > 0 {
		required := make([]string, len(s.Required))
		copy(required, s.Required)
		ret.Required = required
	}
	for pair := s.Properties.Oldest(); pair != nil; pair = pair.Next() {
		ret.Properties[pair.Key] = pair.Value
	}
	return ret
}

func toMessageToolCalls(toolCalls []deepseek.ToolCall) []schema.ToolCall {
	if len(toolCalls) == 0 {
		return nil
	}

	ret := make([]schema.ToolCall, len(toolCalls))
	for i := range toolCalls {
		toolCall := toolCalls[i]
		ret[i] = schema.ToolCall{
			Index: &toolCall.Index,
			ID:    toolCall.ID,
			Type:  toolCall.Type,
			Function: schema.FunctionCall{
				Name:      toolCall.Function.Name,
				Arguments: toolCall.Function.Arguments,
			},
		}
	}

	return ret
}

const typ = "DeepSeek"

func (cm *ChatModel) GetType() string {
	return typ
}

func (cm *ChatModel) IsCallbacksEnabled() bool {
	return true
}

func (cm *ChatModel) generateStreamRequest(ctx context.Context, in []*schema.Message, opts ...model.Option) (*deepseek.StreamChatCompletionRequest, *model.CallbackInput, error) {
	origReq, cbIn, err := cm.generateRequest(ctx, in, opts...)
	if err != nil {
		return nil, nil, err
	}
	req := &deepseek.StreamChatCompletionRequest{
		Stream:           true,
		StreamOptions:    deepseek.StreamOptions{IncludeUsage: false},
		Model:            origReq.Model,
		Messages:         origReq.Messages,
		FrequencyPenalty: origReq.FrequencyPenalty,
		MaxTokens:        origReq.MaxTokens,
		PresencePenalty:  origReq.PresencePenalty,
		Temperature:      origReq.Temperature,
		TopP:             origReq.TopP,
		ResponseFormat:   origReq.ResponseFormat,
		Stop:             origReq.Stop,
		Tools:            origReq.Tools,
		LogProbs:         origReq.LogProbs,
		TopLogProbs:      origReq.TopLogProbs,
	}
	return req, cbIn, nil
}

func (cm *ChatModel) generateRequest(_ context.Context, in []*schema.Message, opts ...model.Option) (*deepseek.ChatCompletionRequest, *model.CallbackInput, error) {

	options := model.GetCommonOptions(&model.Options{
		Temperature: &cm.conf.Temperature,
		MaxTokens:   &cm.conf.MaxTokens,
		Model:       &cm.conf.Model,
		TopP:        &cm.conf.TopP,
		Stop:        cm.conf.Stop,
		Tools:       nil,
		ToolChoice:  cm.toolChoice,
	}, opts...)

	req := &deepseek.ChatCompletionRequest{
		Model:            *options.Model,
		MaxTokens:        dereferenceOrZero(options.MaxTokens),
		Temperature:      dereferenceOrZero(options.Temperature),
		TopP:             dereferenceOrZero(options.TopP),
		Stop:             options.Stop,
		PresencePenalty:  cm.conf.PresencePenalty,
		FrequencyPenalty: cm.conf.FrequencyPenalty,
		LogProbs:         cm.conf.LogProbs,
		TopLogProbs:      cm.conf.TopLogProbs,
	}

	cbInput := &model.CallbackInput{
		Messages: in,
		Tools:    cm.rawTools,
		Config: &model.Config{
			Model:       req.Model,
			MaxTokens:   req.MaxTokens,
			Temperature: req.Temperature,
			TopP:        req.TopP,
			Stop:        req.Stop,
		},
	}

	tools := cm.tools
	if options.Tools != nil {
		var err error
		if tools, err = toTools(options.Tools); err != nil {
			return nil, nil, err
		}
		cbInput.Tools = options.Tools
	}

	if len(tools) > 0 {
		req.Tools = make([]deepseek.Tool, len(tools))
		for i := range tools {
			req.Tools[i] = tools[i]
		}
	}

	if options.ToolChoice != nil {
		/*
			tool_choice is string or object
			Controls which (if any) tool is called by the model.
			"none" means the model will not call any tool and instead generates a message.
			"auto" means the model can pick between generating a message or calling one or more tools.
			"required" means the model must call one or more tools.

			Specifying a particular tool via {"type": "function", "function": {"name": "my_function"}} forces the model to call that tool.

			"none" is the default when no tools are present.
			"auto" is the default if tools are present.
		*/

		switch *options.ToolChoice {
		case schema.ToolChoiceForbidden:
			req.ToolChoice = toolChoiceNone
		case schema.ToolChoiceAllowed:
			req.ToolChoice = toolChoiceAuto
		case schema.ToolChoiceForced:
			if len(req.Tools) == 0 {
				return nil, nil, fmt.Errorf("tool choice is forced but tool is not provided")
			} else if len(req.Tools) > 1 {
				req.ToolChoice = toolChoiceRequired
			} else {
				req.ToolChoice = deepseek.ToolChoice{
					Type: req.Tools[0].Type,
					Function: deepseek.ToolChoiceFunction{
						Name: req.Tools[0].Function.Name,
					},
				}
			}
		default:
			return nil, nil, fmt.Errorf("tool choice=%s not support", *options.ToolChoice)
		}
	}

	msgs := make([]deepseek.ChatCompletionMessage, 0, len(in))
	for _, inMsg := range in {
		msg, e := toDeepSeekMessage(inMsg)
		if e != nil {
			return nil, nil, e
		}

		msgs = append(msgs, *msg)
	}

	req.Messages = msgs

	if len(cm.conf.ResponseFormatType) > 0 {
		req.ResponseFormat = &deepseek.ResponseFormat{
			Type: string(cm.conf.ResponseFormatType),
		}
	}

	return req, cbInput, nil
}

const (
	roleAssistant = "assistant"
	roleSystem    = "system"
	roleUser      = "user"
	roleTool      = "tool"
)

func toDeepSeekMessage(m *schema.Message) (*deepseek.ChatCompletionMessage, error) {
	if len(m.MultiContent) > 0 {
		return nil, fmt.Errorf("multi content is not supported in deepseek")
	}
	var role string
	switch m.Role {
	case schema.Assistant:
		role = roleAssistant
	case schema.System:
		role = roleSystem
	case schema.User:
		role = roleUser
	case schema.Tool:
		role = roleTool
	default:
		return nil, fmt.Errorf("unknown role type: %s", m.Role)
	}
	ret := &deepseek.ChatCompletionMessage{
		Role:    role,
		Content: m.Content,
		// TODO: tool call id
		Prefix: HasPrefix(m),
	}
	if ret.Role != roleAssistant && ret.Prefix {
		return nil, fmt.Errorf("prefix only supported for assistant message")
	}
	if ret.Prefix {
		if reasoning, ok := GetReasoningContent(m); ok {
			ret.ReasoningContent = reasoning
		}
	}
	if ret.Role == roleTool && m.ToolCallID != "" {
		ret.ToolCallID = m.ToolCallID
	}
	if ret.Role == roleAssistant && len(m.ToolCalls) > 0 {
		ret.ToolCalls = make([]deepseek.ToolCall, len(m.ToolCalls))
		for i, call := range m.ToolCalls {
			var index int
			if call.Index != nil {
				index = *call.Index
			}
			ret.ToolCalls[i] = deepseek.ToolCall{
				Index: index,
				ID:    call.ID,
				Type:  call.Type,
				Function: deepseek.ToolCallFunction{
					Name:      call.Function.Name,
					Arguments: call.Function.Arguments,
				},
			}
		}
	}
	return ret, nil
}

func dereferenceOrZero[T any](v *T) T {
	if v == nil {
		var t T
		return t
	}

	return *v
}

func toMessageRole(role string) schema.RoleType {
	switch role {
	case roleUser:
		return schema.User
	case roleAssistant:
		return schema.Assistant
	case roleSystem:
		return schema.System
	case roleTool:
		return schema.Tool
	default:
		return schema.RoleType(role)
	}
}

func toModelCallbackUsage(respMeta *schema.ResponseMeta) *model.TokenUsage {
	if respMeta == nil {
		return nil
	}
	usage := respMeta.Usage
	if usage == nil {
		return nil
	}
	return &model.TokenUsage{
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
	}
}

func resolveStreamResponse(resp *deepseek.StreamChatCompletionResponse) (msg *schema.Message, found bool, err error) {
	for _, choice := range resp.Choices {
		// take 0 index as response, rewrite if needed
		if choice.Index != 0 {
			continue
		}

		lp, err := extractLogProbs(choice.Logprobs)
		if err != nil {
			return nil, false, fmt.Errorf("failed to extract log probs: %w", err)
		}
		found = true
		msg = &schema.Message{
			Role:      toMessageRole(choice.Delta.Role),
			Content:   choice.Delta.Content,
			ToolCalls: toMessageToolCalls(choice.Delta.ToolCalls),
			ResponseMeta: &schema.ResponseMeta{
				FinishReason: choice.FinishReason,
				Usage:        streamToEinoTokenUsage(resp.Usage),
				LogProbs:     lp,
			},
		}
		if len(choice.Delta.ReasoningContent) > 0 {
			SetReasoningContent(msg, choice.Delta.ReasoningContent)
			msg.ReasoningContent = choice.Delta.ReasoningContent
		}

		break
	}

	if resp.Usage != nil && !found {
		msg = &schema.Message{
			ResponseMeta: &schema.ResponseMeta{
				Usage: streamToEinoTokenUsage(resp.Usage),
			},
		}
		found = true
	}

	return msg, found, nil
}

func streamToEinoTokenUsage(usage *deepseek.StreamUsage) *schema.TokenUsage {
	if usage == nil {
		return nil
	}
	if usage.PromptTokens == 0 &&
		usage.CompletionTokens == 0 &&
		usage.TotalTokens == 0 {
		return nil
	}
	return toEinoTokenUsage(&deepseek.Usage{
		PromptTokens:         usage.PromptTokens,
		PromptCacheHitTokens: usage.PromptCacheHitTokens,
		CompletionTokens:     usage.CompletionTokens,
		TotalTokens:          usage.TotalTokens,
	})
}

func toEinoTokenUsage(usage *deepseek.Usage) *schema.TokenUsage {
	if usage == nil {
		return nil
	}
	return &schema.TokenUsage{
		PromptTokens: usage.PromptTokens,
		PromptTokenDetails: schema.PromptTokenDetails{
			CachedTokens: usage.PromptCacheHitTokens,
		},
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
	}
}

func toCallbackUsage(usage *schema.TokenUsage) *model.TokenUsage {
	if usage == nil {
		return nil
	}
	return &model.TokenUsage{
		PromptTokens: usage.PromptTokens,
		PromptTokenDetails: model.PromptTokenDetails{
			CachedTokens: usage.PromptTokenDetails.CachedTokens,
		},
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
	}
}

func extractLogProbs(in any) (*schema.LogProbs, error) {
	if in == nil {
		return nil, nil
	}
	m, ok := in.(map[string]any)
	if !ok {
		// unreachable
		return nil, fmt.Errorf("logprobs are expected to map[string]any, actually %T", in)
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	ret := &schema.LogProbs{}
	err = json.Unmarshal(b, ret)
	if err != nil {
		return nil, err
	}
	return ret, nil
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

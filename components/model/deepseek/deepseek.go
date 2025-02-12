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
	"errors"
	"fmt"
	"io"
	"runtime/debug"
	"time"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/eino/utils/safe"
	"github.com/cohesion-org/deepseek-go"
)

type ResponseFormatType string

const (
	ResponseFormatTypeText       = "text"
	ResponseFormatTypeJSONObject = "json_object"
)

type ChatModelConfig struct {
	// APIKey is your authentication key
	// Required
	APIKey string `json:"api_key"`

	// Timeout specifies the maximum duration to wait for API responses
	// Optional. Default: 5 minutes
	Timeout time.Duration `json:"timeout"`

	// BaseURL is your custom deepseek endpoint url
	// Optional. Default: https://api.deepseek.com/
	BaseURL string `json:"base_url"`

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
}

var _ model.ChatModel = (*ChatModel)(nil)

type ChatModel struct {
	cli  *deepseek.Client
	conf *ChatModelConfig

	tools      []deepseek.Tool
	rawTools   []*schema.ToolInfo
	toolChoice *schema.ToolChoice
}

func NewChatModel(_ context.Context, config *ChatModelConfig) (*ChatModel, error) {
	if len(config.APIKey) == 0 {
		return nil, fmt.Errorf("API key is required")
	}
	if len(config.Model) == 0 {
		return nil, fmt.Errorf("model is required")
	}

	var opts []deepseek.Option
	if config.Timeout > 0 {
		opts = append(opts, deepseek.WithTimeout(config.Timeout))
	}
	if len(config.BaseURL) > 0 {
		opts = append(opts, deepseek.WithBaseURL(config.BaseURL))
	}

	cli, err := deepseek.NewClientWithOptions(config.APIKey, opts...)
	if err != nil {
		return nil, err
	}
	return &ChatModel{cli: cli, conf: config}, nil
}

func (cm *ChatModel) Generate(ctx context.Context, in []*schema.Message, opts ...model.Option) (outMsg *schema.Message, err error) {
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	req, cbInput, err := cm.generateRequest(ctx, in, opts...)

	ctx = callbacks.OnStart(ctx, cbInput)

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

		outMsg = &schema.Message{
			Role:    toMessageRole(choice.Message.Role),
			Content: choice.Message.Content,
			// TODO: tool call
			ResponseMeta: &schema.ResponseMeta{
				FinishReason: choice.FinishReason,
				Usage:        toEinoTokenUsage(&resp.Usage),
			},
		}
		if len(choice.Message.ReasoningContent) > 0 {
			SetReasoningContent(outMsg, choice.Message.ReasoningContent)
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
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()
	req, cbInput, err := cm.generateStreamRequest(ctx, in, opts...)

	ctx = callbacks.OnStart(ctx, cbInput)

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
				_ = sw.Send(nil, safe.NewPanicErr(panicErr, debug.Stack()))
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

			msg, found := resolveStreamResponse(chunk)
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

const toolErrorStr = "DeepSeek temporarily does not support tool call. If you have any requirements, please feel free to provide feedback to us at https://github.com/cloudwego/eino-ext/issues"

func (cm *ChatModel) BindTools(_ []*schema.ToolInfo) error {
	return fmt.Errorf(toolErrorStr)
}

func (cm *ChatModel) BindForcedTools(_ []*schema.ToolInfo) error {
	return fmt.Errorf(toolErrorStr)
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
		Stop:             cm.conf.Stop,
		PresencePenalty:  cm.conf.PresencePenalty,
		FrequencyPenalty: cm.conf.FrequencyPenalty,
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

	if options.Tools != nil {
		return nil, nil, fmt.Errorf(toolErrorStr)
	}

	if options.ToolChoice != nil {
		return nil, nil, fmt.Errorf(toolErrorStr)
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

func resolveStreamResponse(resp *deepseek.StreamChatCompletionResponse) (msg *schema.Message, found bool) {
	for _, choice := range resp.Choices {
		// take 0 index as response, rewrite if needed
		if choice.Index != 0 {
			continue
		}

		found = true
		msg = &schema.Message{
			Role:    toMessageRole(choice.Delta.Role),
			Content: choice.Delta.Content,
			// todo: handle tool call
			ResponseMeta: &schema.ResponseMeta{
				FinishReason: choice.FinishReason,
				Usage:        streamToEinoTokenUsage(resp.Usage),
			},
		}
		if len(choice.Delta.ReasoningContent) > 0 {
			SetReasoningContent(msg, choice.Delta.ReasoningContent)
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

	return msg, found
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
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
	})
}

func toEinoTokenUsage(usage *deepseek.Usage) *schema.TokenUsage {
	if usage == nil {
		return nil
	}
	return &schema.TokenUsage{
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
	}
}

func toCallbackUsage(usage *schema.TokenUsage) *model.TokenUsage {
	if usage == nil {
		return nil
	}
	return &model.TokenUsage{
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
	}
}

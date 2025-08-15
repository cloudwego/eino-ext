package grok

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime/debug"
	"time"

	grokgo "github.com/SimonMorphy/grok-go"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

var _ model.ToolCallingChatModel = (*ChatModel)(nil)

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

// Config contains the configuration options for the Grok model
type Config struct {
	// APIKey is your X.AI API key
	// Required
	APIKey string `json:"api_key"`

	// BaseURL is the custom API endpoint URL
	// Optional. Default: "https://api.x.ai/v1/"
	BaseURL *string `json:"base_url,omitempty"`

	// Timeout specifies the maximum duration to wait for API responses
	// Optional. Default: 30 seconds
	Timeout time.Duration `json:"timeout,omitempty"`

	// Model specifies which Grok model to use
	// Required. Example: "grok-3-beta"
	Model string `json:"model"`

	// MaxTokens limits the maximum number of tokens in the response
	// Optional. Example: 1000
	MaxTokens *int `json:"max_tokens,omitempty"`

	// Temperature controls randomness in responses
	// Range: [0.0, 2.0], where 0.0 is more focused and 2.0 is more creative
	// Optional. Example: float32(0.7)
	Temperature *float32 `json:"temperature,omitempty"`

	// TopP controls diversity via nucleus sampling
	// Range: [0.0, 1.0], where 1.0 disables nucleus sampling
	// Optional. Example: float32(0.95)
	TopP *float32 `json:"top_p,omitempty"`

	// TopK controls diversity by limiting the top K tokens to sample from
	// Optional. Example: 40
	TopK *int `json:"top_k,omitempty"`

	// Stop sequences where the API will stop generating further tokens
	// Optional. Example: []string{"\n", "User:"}
	Stop []string `json:"stop,omitempty"`

	// HTTPClient specifies the client to send HTTP requests
	// Optional.
	HTTPClient *http.Client `json:"http_client,omitempty"`
}

// ChatModel represents a Grok chat model client.
type ChatModel struct {
	cli *grokgo.Client

	model       string
	maxTokens   *int
	topP        *float32
	temperature *float32
	topK        *int
	stop        []string
	oriTools    []*schema.ToolInfo
	tools       []grokgo.Tool
	toolChoice  *schema.ToolChoice
}

// NewChatModel creates a new Grok chat model instance
//
// Parameters:
//   - ctx: The context for the operation
//   - conf: Configuration for the Grok model
//
// Returns:
//   - model.ChatModel: A chat model interface implementation
//   - error: Any error that occurred during creation
//
// Example:
//
//	model, err := grok.NewChatModel(ctx, &grok.Config{
//	    APIKey: "your-api-key",
//	    Model:  "grok-3-beta",
//	    MaxTokens: 1000,
//	})
func NewChatModel(ctx context.Context, config *Config) (*ChatModel, error) {
	if config.APIKey == "" {
		return nil, errors.New("api key is required")
	}
	if config.Model == "" {
		return nil, errors.New("model is required")
	}

	var opts []grokgo.ClientOption
	if config.BaseURL != nil {
		opts = append(opts, grokgo.WithBaseURL(*config.BaseURL))
	}
	if config.Timeout > 0 {
		opts = append(opts, grokgo.WithTimeout(config.Timeout))
	}
	if config.HTTPClient != nil {
		opts = append(opts, grokgo.WithHTTPClient(config.HTTPClient))
	}

	client, err := grokgo.NewClientWithOptions(config.APIKey, opts...)
	if err != nil {
		return nil, fmt.Errorf("create grok client fail: %w", err)
	}

	return &ChatModel{
		cli:         client,
		model:       config.Model,
		maxTokens:   config.MaxTokens,
		temperature: config.Temperature,
		topP:        config.TopP,
		topK:        config.TopK,
		stop:        config.Stop,
	}, nil
}

func (cm *ChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (message *schema.Message, err error) {
	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)
	callbackInput := cm.getCallbackInput(input, opts...)
	ctx = callbacks.OnStart(ctx, callbackInput)
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	// Prepare request parameters
	req, err := cm.createChatCompletionRequest(input, opts...)
	if err != nil {
		return nil, err
	}

	// Call API
	resp, err := grokgo.CreateChatCompletion(ctx, cm.cli, req)
	if err != nil {
		return nil, fmt.Errorf("create chat completion fail: %w", err)
	}

	// Convert response to schema message
	message, err = cm.convertResponseToMessage(resp)
	if err != nil {
		return nil, fmt.Errorf("convert response to schema message fail: %w", err)
	}

	callbacks.OnEnd(ctx, cm.getCallbackOutput(message))
	return message, nil
}

func (cm *ChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (result *schema.StreamReader[*schema.Message], err error) {
	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)
	callbackInput := cm.getCallbackInput(input, opts...)
	ctx = callbacks.OnStart(ctx, callbackInput)
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	// Prepare request parameters
	req, err := cm.createChatCompletionRequest(input, opts...)
	if err != nil {
		return nil, err
	}
	req.Stream = true

	// Call API with streaming
	stream, err := grokgo.CreateChatCompletionStream(ctx, cm.cli, req)
	if err != nil {
		return nil, fmt.Errorf("create chat completion stream fail: %w", err)
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

		var waitList []*schema.Message
		for {
			chunk, chunkErr := stream.Recv()
			if errors.Is(chunkErr, io.EOF) {
				return
			}
			if chunkErr != nil {
				_ = sw.Send(nil, fmt.Errorf("receive stream chunk fail: %w", chunkErr))
				return
			}

			message, err := cm.convertStreamResponseToMessage(chunk)
			if err != nil {
				_ = sw.Send(nil, fmt.Errorf("convert stream response to schema message fail: %w", err))
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
	}()

	_, sr = callbacks.OnEndWithStreamOutput(ctx, sr)
	return schema.StreamReaderWithConvert(sr, func(t *model.CallbackOutput) (*schema.Message, error) {
		return t.Message, nil
	}), nil
}

func (cm *ChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	if len(tools) == 0 {
		return nil, errors.New("no tools to bind")
	}
	grokTools, err := cm.toGrokTools(tools)
	if err != nil {
		return nil, fmt.Errorf("convert to grok tools fail: %w", err)
	}

	tc := schema.ToolChoiceAllowed
	ncm := *cm
	ncm.tools = grokTools
	ncm.oriTools = tools
	ncm.toolChoice = &tc
	return &ncm, nil
}

func (cm *ChatModel) BindTools(tools []*schema.ToolInfo) error {
	if len(tools) == 0 {
		return errors.New("no tools to bind")
	}
	grokTools, err := cm.toGrokTools(tools)
	if err != nil {
		return fmt.Errorf("convert to grok tools fail: %w", err)
	}

	cm.tools = grokTools
	cm.oriTools = tools
	tc := schema.ToolChoiceAllowed
	cm.toolChoice = &tc
	return nil
}

func (cm *ChatModel) BindForcedTools(tools []*schema.ToolInfo) error {
	if len(tools) == 0 {
		return errors.New("no tools to bind")
	}
	grokTools, err := cm.toGrokTools(tools)
	if err != nil {
		return fmt.Errorf("convert to grok tools fail: %w", err)
	}

	cm.tools = grokTools
	cm.oriTools = tools
	tc := schema.ToolChoiceForced
	cm.toolChoice = &tc
	return nil
}

func (cm *ChatModel) toGrokTools(tools []*schema.ToolInfo) ([]grokgo.Tool, error) {
	result := make([]grokgo.Tool, 0, len(tools))
	for _, tool := range tools {
		s, err := tool.ToOpenAPIV3()
		if err != nil {
			return nil, fmt.Errorf("convert to openapi v3 schema fail: %w", err)
		}

		// Convert OpenAPI schema to Grok function parameters
		params := &grokgo.FunctionParameters{
			Type:       "object",
			Properties: make(map[string]interface{}),
		}

		if s.Properties != nil {
			for name, prop := range s.Properties {
				params.Properties[name] = prop.Value
			}
		}

		if len(s.Required) > 0 {
			params.Required = s.Required
		}

		result = append(result, grokgo.Tool{
			Type: "function",
			Function: grokgo.Function{
				Name:        tool.Name,
				Description: tool.Desc,
				Parameters:  params,
			},
		})
	}

	return result, nil
}

func (cm *ChatModel) createChatCompletionRequest(input []*schema.Message, opts ...model.Option) (*grokgo.ChatCompletionRequest, error) {
	if len(input) == 0 {
		return nil, errors.New("input is empty")
	}

	commonOptions := model.GetCommonOptions(&model.Options{
		Model:       &cm.model,
		Temperature: cm.temperature,
		MaxTokens:   cm.maxTokens,
		TopP:        cm.topP,
		Stop:        cm.stop,
		Tools:       nil,
		ToolChoice:  cm.toolChoice,
	}, opts...)

	grokOptions := model.GetImplSpecificOptions(&options{
		TopK: cm.topK,
	}, opts...)

	req := &grokgo.ChatCompletionRequest{
		Model: *commonOptions.Model,
	}

	if commonOptions.MaxTokens != nil {
		req.MaxTokens = *commonOptions.MaxTokens
	}

	if commonOptions.Temperature != nil {
		req.Temperature = float64(*commonOptions.Temperature)
	}

	if commonOptions.TopP != nil {
		req.TopP = float64(*commonOptions.TopP)
	}

	if len(commonOptions.Stop) > 0 {
		req.Stop = commonOptions.Stop
	}

	if grokOptions.TopK != nil {
		req.TopK = *grokOptions.TopK
	}

	// Handle tools
	tools := cm.tools
	if commonOptions.Tools != nil {
		var err error
		if tools, err = cm.toGrokTools(commonOptions.Tools); err != nil {
			return nil, err
		}
	}

	if len(tools) > 0 {
		req.Tools = tools
	}

	// Handle tool choice
	if commonOptions.ToolChoice != nil {
		switch *commonOptions.ToolChoice {
		case schema.ToolChoiceForbidden:
			req.ToolChoice = "none"
		case schema.ToolChoiceAllowed:
			req.ToolChoice = "auto"
		case schema.ToolChoiceForced:
			if len(tools) == 0 {
				return nil, errors.New("tool choice is forced but tool is not provided")
			} else if len(tools) == 1 {
				req.ToolChoice = map[string]interface{}{
					"type":     "function",
					"function": map[string]string{"name": tools[0].Function.Name},
				}
			} else {
				req.ToolChoice = "required"
			}
		default:
			return nil, fmt.Errorf("tool choice=%s not support", *commonOptions.ToolChoice)
		}
	}

	// Convert messages
	messages, err := cm.convertMessagesToGrok(input)
	if err != nil {
		return nil, err
	}
	req.Messages = messages

	return req, nil
}

func (cm *ChatModel) convertMessagesToGrok(messages []*schema.Message) ([]grokgo.ChatCompletionMessage, error) {
	result := make([]grokgo.ChatCompletionMessage, 0, len(messages))
	for _, msg := range messages {
		grokMsg := grokgo.ChatCompletionMessage{
			Role:    convertRole(msg.Role),
			Content: msg.Content,
		}

		// Handle tool calls
		if len(msg.ToolCalls) > 0 {
			grokMsg.ToolCalls = make([]grokgo.APIToolCall, 0, len(msg.ToolCalls))
			for _, tc := range msg.ToolCalls {
				grokMsg.ToolCalls = append(grokMsg.ToolCalls, grokgo.APIToolCall{
					ID:   tc.ID,
					Type: "function",
					Function: grokgo.FunctionCall{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				})
			}
		}

		// Handle tool response
		if msg.Role == schema.Tool && msg.ToolCallID != "" {
			grokMsg.ToolCallID = msg.ToolCallID
		}

		result = append(result, grokMsg)
	}
	return result, nil
}

func (cm *ChatModel) convertResponseToMessage(resp *grokgo.Response) (*schema.Message, error) {
	if len(resp.Choices) == 0 {
		return nil, errors.New("no choices in response")
	}

	choice := resp.Choices[0]
	message := &schema.Message{
		Role:    schema.Assistant,
		Content: choice.Message.Content,
		ResponseMeta: &schema.ResponseMeta{
			FinishReason: choice.FinishReason,
			Usage: &schema.TokenUsage{
				PromptTokens:     resp.Usage.PromptTokens,
				CompletionTokens: resp.Usage.CompletionTokens,
				TotalTokens:      resp.Usage.TotalTokens,
			},
		},
	}

	// Handle tool calls
	if len(choice.Message.ToolCalls) > 0 {
		message.ToolCalls = make([]schema.ToolCall, 0, len(choice.Message.ToolCalls))
		for _, tc := range choice.Message.ToolCalls {
			message.ToolCalls = append(message.ToolCalls, schema.ToolCall{
				ID:   tc.ID,
				Type: tc.Type,
				Function: schema.FunctionCall{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			})
		}
	}

	return message, nil
}

func (cm *ChatModel) convertStreamResponseToMessage(resp *grokgo.StreamResponse) (*schema.Message, error) {
	if len(resp.Choices) == 0 {
		return nil, nil
	}

	choice := resp.Choices[0]
	message := &schema.Message{
		Role:    schema.Assistant,
		Content: choice.Delta.Content,
		ResponseMeta: &schema.ResponseMeta{
			FinishReason: choice.FinishReason,
		},
	}

	if resp.Usage != nil {
		message.ResponseMeta.Usage = &schema.TokenUsage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		}
	}

	// Handle tool calls
	if len(choice.Delta.ToolCalls) > 0 {
		message.ToolCalls = make([]schema.ToolCall, 0, len(choice.Delta.ToolCalls))
		for _, tc := range choice.Delta.ToolCalls {
			message.ToolCalls = append(message.ToolCalls, schema.ToolCall{
				ID:   tc.ID,
				Type: tc.Type,
				Function: schema.FunctionCall{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			})
		}
	}

	return message, nil
}

func (cm *ChatModel) getCallbackInput(input []*schema.Message, opts ...model.Option) *model.CallbackInput {
	result := &model.CallbackInput{
		Messages: input,
		Tools: model.GetCommonOptions(&model.Options{
			Tools: cm.oriTools,
		}, opts...).Tools,
		Config: cm.getConfig(),
	}
	return result
}

func (cm *ChatModel) getCallbackOutput(output *schema.Message) *model.CallbackOutput {
	result := &model.CallbackOutput{
		Message: output,
		Config:  cm.getConfig(),
	}
	if output.ResponseMeta != nil && output.ResponseMeta.Usage != nil {
		result.TokenUsage = &model.TokenUsage{
			PromptTokens:     output.ResponseMeta.Usage.PromptTokens,
			CompletionTokens: output.ResponseMeta.Usage.CompletionTokens,
			TotalTokens:      output.ResponseMeta.Usage.TotalTokens,
		}
	}
	return result
}

func (cm *ChatModel) getConfig() *model.Config {
	result := &model.Config{
		Model: cm.model,
		Stop:  cm.stop,
	}
	if cm.maxTokens != nil {
		result.MaxTokens = *cm.maxTokens
	}
	if cm.temperature != nil {
		result.Temperature = *cm.temperature
	}
	if cm.topP != nil {
		result.TopP = *cm.topP
	}
	return result
}

func (cm *ChatModel) GetType() string {
	return "Grok"
}

func (cm *ChatModel) IsCallbacksEnabled() bool {
	return true
}

func convertRole(role schema.RoleType) string {
	switch role {
	case schema.Assistant:
		return "assistant"
	case schema.System:
		return "system"
	case schema.User:
		return "user"
	case schema.Tool:
		return "tool"
	default:
		return string(role)
	}
}

func isMessageEmpty(message *schema.Message) bool {
	return len(message.Content) == 0 && len(message.ToolCalls) == 0 && len(message.MultiContent) == 0
}

// options holds implementation-specific options for Grok
type options struct {
	// TopK controls diversity by limiting the top K tokens to sample from
	TopK *int
}

// WithTopK sets the TopK parameter for the Grok model
func WithTopK(topK int) model.Option {
	return model.WrapImplSpecificOptFn(func(o *options) {
		o.TopK = &topK
	})
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

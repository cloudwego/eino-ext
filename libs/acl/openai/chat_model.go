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

package openai

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime/debug"

	"github.com/sashabaranov/go-openai"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/eino/utils/safe"
	"github.com/getkin/kin-openapi/openapi3"
)

type ChatCompletionResponseFormatType string

const (
	ChatCompletionResponseFormatTypeJSONObject ChatCompletionResponseFormatType = "json_object"
	ChatCompletionResponseFormatTypeJSONSchema ChatCompletionResponseFormatType = "json_schema"
	ChatCompletionResponseFormatTypeText       ChatCompletionResponseFormatType = "text"
)

const (
	toolChoiceRequired = "required"
)

type ChatCompletionResponseFormat struct {
	Type       ChatCompletionResponseFormatType        `json:"type,omitempty"`
	JSONSchema *ChatCompletionResponseFormatJSONSchema `json:"json_schema,omitempty"`
}

type ChatCompletionResponseFormatJSONSchema struct {
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Schema      *openapi3.Schema `json:"schema"`
	Strict      bool             `json:"strict"`
}

type Config struct {
	// if you want to use Azure OpenAI Service, set the next three fields. refs: https://learn.microsoft.com/en-us/azure/ai-services/openai/
	// ByAzure set this field to true when using Azure OpenAI Service, otherwise it does not need to be set.
	ByAzure bool `json:"by_azure"`
	// BaseURL https://{{$YOUR_RESOURCE_NAME}}.openai.azure.com, YOUR_RESOURCE_NAME is the name of your resource that you have created on Azure.
	BaseURL string `json:"base_url"`
	// APIVersion specifies the API version you want to use.
	APIVersion string `json:"api_version"`

	// APIKey is typically OPENAI_API_KEY, but if you have set up Azure, then it is Azure API_KEY.
	APIKey string `json:"api_key"`

	// HTTPClient is used to send request.
	HTTPClient *http.Client

	// The following fields have the same meaning as the fields in the openai chat completion API request. Ref: https://platform.openai.com/docs/api-reference/chat/create
	Model            string                        `json:"model"`
	MaxTokens        *int                          `json:"max_tokens,omitempty"`
	Temperature      *float32                      `json:"temperature,omitempty"`
	TopP             *float32                      `json:"top_p,omitempty"`
	N                *int                          `json:"n,omitempty"`
	Stop             []string                      `json:"stop,omitempty"`
	PresencePenalty  *float32                      `json:"presence_penalty,omitempty"`
	ResponseFormat   *ChatCompletionResponseFormat `json:"response_format,omitempty"`
	Seed             *int                          `json:"seed,omitempty"`
	FrequencyPenalty *float32                      `json:"frequency_penalty,omitempty"`
	LogitBias        map[string]int                `json:"logit_bias,omitempty"`
	LogProbs         *bool                         `json:"logprobs,omitempty"`
	TopLogProbs      *int                          `json:"top_logprobs,omitempty"`
	User             *string                       `json:"user,omitempty"`
}

var _ model.ChatModel = (*Client)(nil)

type Client struct {
	cli    *openai.Client
	config *Config

	tools         []tool
	rawTools      []*schema.ToolInfo
	forceToolCall bool
}

func NewClient(ctx context.Context, config *Config) (*Client, error) {
	if config == nil {
		config = &Config{Model: "gpt-3.5-turbo"}
	}

	var clientConf openai.ClientConfig

	if config.ByAzure {
		clientConf = openai.DefaultAzureConfig(config.APIKey, config.BaseURL)
		if config.APIVersion != "" {
			clientConf.APIVersion = config.APIVersion
		}
	} else {
		clientConf = openai.DefaultConfig(config.APIKey)
		if len(config.BaseURL) > 0 {
			clientConf.BaseURL = config.BaseURL
		}
	}

	clientConf.HTTPClient = config.HTTPClient
	if clientConf.HTTPClient == nil {
		clientConf.HTTPClient = http.DefaultClient
	}

	return &Client{
		cli:    openai.NewClientWithConfig(clientConf),
		config: config,
	}, nil
}

func toOpenAIRole(role schema.RoleType) string {
	switch role {
	case schema.User:
		return openai.ChatMessageRoleUser
	case schema.Assistant:
		return openai.ChatMessageRoleAssistant
	case schema.System:
		return openai.ChatMessageRoleSystem
	case schema.Tool:
		return openai.ChatMessageRoleTool
	default:
		return string(role)
	}
}

func toOpenAIMultiContent(mc []schema.ChatMessagePart) ([]openai.ChatMessagePart, error) {
	if len(mc) == 0 {
		return nil, nil
	}

	ret := make([]openai.ChatMessagePart, 0, len(mc))

	for _, part := range mc {
		switch part.Type {
		case schema.ChatMessagePartTypeText:
			ret = append(ret, openai.ChatMessagePart{
				Type: openai.ChatMessagePartTypeText,
				Text: part.Text,
			})
		case schema.ChatMessagePartTypeImageURL:
			if part.ImageURL == nil {
				return nil, fmt.Errorf("image_url should not be nil")
			}
			ret = append(ret, openai.ChatMessagePart{
				Type: openai.ChatMessagePartTypeImageURL,
				ImageURL: &openai.ChatMessageImageURL{
					URL:    part.ImageURL.URL,
					Detail: openai.ImageURLDetail(part.ImageURL.Detail),
				},
			})
		default:
			return nil, fmt.Errorf("unsupported chat message part type: %s", part.Type)
		}
	}

	return ret, nil
}

func toMessageRole(role string) schema.RoleType {
	switch role {
	case openai.ChatMessageRoleUser:
		return schema.User
	case openai.ChatMessageRoleAssistant:
		return schema.Assistant
	case openai.ChatMessageRoleSystem:
		return schema.System
	case openai.ChatMessageRoleTool:
		return schema.Tool
	default:
		return schema.RoleType(role)
	}
}

func toMessageToolCalls(toolCalls []openai.ToolCall) []schema.ToolCall {
	if len(toolCalls) == 0 {
		return nil
	}

	ret := make([]schema.ToolCall, len(toolCalls))
	for i := range toolCalls {
		toolCall := toolCalls[i]
		ret[i] = schema.ToolCall{
			Index: toolCall.Index,
			ID:    toolCall.ID,
			Type:  string(toolCall.Type),
			Function: schema.FunctionCall{
				Name:      toolCall.Function.Name,
				Arguments: toolCall.Function.Arguments,
			},
		}
	}

	return ret
}

func toOpenAIToolCalls(toolCalls []schema.ToolCall) []openai.ToolCall {
	if len(toolCalls) == 0 {
		return nil
	}

	ret := make([]openai.ToolCall, len(toolCalls))
	for i := range toolCalls {
		toolCall := toolCalls[i]
		ret[i] = openai.ToolCall{
			Index: toolCall.Index,
			ID:    toolCall.ID,
			Type:  openai.ToolTypeFunction,
			Function: openai.FunctionCall{
				Name:      toolCall.Function.Name,
				Arguments: toolCall.Function.Arguments,
			},
		}
	}

	return ret
}

func (cm *Client) genRequest(in []*schema.Message, options *model.Options) (*openai.ChatCompletionRequest, error) {
	if options.Model == nil || len(*options.Model) == 0 {
		return nil, fmt.Errorf("open chat model gen request with empty model")
	}

	req := &openai.ChatCompletionRequest{
		Model:            *options.Model,
		MaxTokens:        dereferenceOrZero(options.MaxTokens),
		Temperature:      dereferenceOrZero(options.Temperature),
		TopP:             dereferenceOrZero(options.TopP),
		N:                dereferenceOrZero(cm.config.N),
		Stop:             cm.config.Stop,
		PresencePenalty:  dereferenceOrZero(cm.config.PresencePenalty),
		Seed:             cm.config.Seed,
		FrequencyPenalty: dereferenceOrZero(cm.config.FrequencyPenalty),
		LogitBias:        cm.config.LogitBias,
		LogProbs:         dereferenceOrZero(cm.config.LogProbs),
		TopLogProbs:      dereferenceOrZero(cm.config.TopLogProbs),
		User:             dereferenceOrZero(cm.config.User),
	}

	if len(cm.tools) > 0 {
		req.Tools = make([]openai.Tool, len(cm.tools))
		for i := range cm.tools {
			t := cm.tools[i]

			req.Tools[i] = openai.Tool{
				Type: openai.ToolTypeFunction,
				Function: &openai.FunctionDefinition{
					Name:        t.Function.Name,
					Description: t.Function.Description,
					Parameters:  t.Function.Parameters,
				},
			}
		}

		if cm.forceToolCall && len(cm.tools) > 0 {

			/* // nolint: byted_s_comment_space
			tool_choice is string or object
			Controls which (if any) tool is called by the model.
			"none" means the model will not call any tool and instead generates a message.
			"auto" means the model can pick between generating a message or calling one or more tools.
			"required" means the model must call one or more tools.

			Specifying a particular tool via {"type": "function", "function": {"name": "my_function"}} forces the model to call that tool.

			"none" is the default when no tools are present.
			"auto" is the default if tools are present.
			*/

			if len(req.Tools) > 1 {
				req.ToolChoice = toolChoiceRequired
			} else {
				req.ToolChoice = openai.ToolChoice{
					Type: req.Tools[0].Type,
					Function: openai.ToolFunction{
						Name: req.Tools[0].Function.Name,
					},
				}
			}
		}
	}

	msgs := make([]openai.ChatCompletionMessage, 0, len(in))
	for _, inMsg := range in {
		mc, e := toOpenAIMultiContent(inMsg.MultiContent)
		if e != nil {
			return nil, e
		}
		msg := openai.ChatCompletionMessage{
			Role:         toOpenAIRole(inMsg.Role),
			Content:      inMsg.Content,
			MultiContent: mc,
			Name:         inMsg.Name,
			ToolCalls:    toOpenAIToolCalls(inMsg.ToolCalls),
			ToolCallID:   inMsg.ToolCallID,
		}

		msgs = append(msgs, msg)
	}

	req.Messages = msgs

	if cm.config.ResponseFormat != nil {
		req.ResponseFormat = &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatType(cm.config.ResponseFormat.Type),
		}
		if cm.config.ResponseFormat.JSONSchema != nil {
			req.ResponseFormat.JSONSchema = &openai.ChatCompletionResponseFormatJSONSchema{
				Name:        cm.config.ResponseFormat.JSONSchema.Name,
				Description: cm.config.ResponseFormat.JSONSchema.Description,
				Schema:      cm.config.ResponseFormat.JSONSchema.Schema,
				Strict:      cm.config.ResponseFormat.JSONSchema.Strict,
			}
		}
	}

	return req, nil
}

func (cm *Client) Generate(ctx context.Context, in []*schema.Message, opts ...model.Option) (
	outMsg *schema.Message, err error) {

	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	options := model.GetCommonOptions(&model.Options{
		Temperature: cm.config.Temperature,
		MaxTokens:   cm.config.MaxTokens,
		Model:       &cm.config.Model,
		TopP:        cm.config.TopP,
		Stop:        cm.config.Stop,
	}, opts...)

	req, err := cm.genRequest(in, options)
	if err != nil {
		return nil, err
	}

	reqConf := &model.Config{
		Model:       req.Model,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stop:        req.Stop,
	}

	ctx = callbacks.OnStart(ctx, &model.CallbackInput{
		Messages:   in,
		Tools:      append(cm.rawTools), // join tool info from call options
		ToolChoice: getToolChoice(req.ToolChoice),
		Config:     reqConf,
	})

	resp, err := cm.cli.CreateChatCompletion(ctx, *req)
	if err != nil {
		return nil, err
	}

	for _, choice := range resp.Choices {
		if choice.Index != 0 {
			continue
		}

		msg := choice.Message
		outMsg = &schema.Message{
			Role:       toMessageRole(msg.Role),
			Content:    msg.Content,
			Name:       msg.Name,
			ToolCallID: msg.ToolCallID,
			ToolCalls:  toMessageToolCalls(msg.ToolCalls),
			ResponseMeta: &schema.ResponseMeta{
				FinishReason: string(choice.FinishReason),
				Usage:        toEinoTokenUsage(&resp.Usage),
			},
		}

		break
	}

	if outMsg == nil { // unexpected
		return nil, fmt.Errorf("unexpected completion choices without index=0")
	}

	usage := &model.TokenUsage{
		PromptTokens:     resp.Usage.PromptTokens,
		CompletionTokens: resp.Usage.CompletionTokens,
		TotalTokens:      resp.Usage.TotalTokens,
	}

	callbacks.OnEnd(ctx, &model.CallbackOutput{
		Message:    outMsg,
		Config:     reqConf,
		TokenUsage: usage,
	})

	return outMsg, nil
}

func (cm *Client) Stream(ctx context.Context, in []*schema.Message, // nolint: byted_s_too_many_lines_in_func
	opts ...model.Option) (outStream *schema.StreamReader[*schema.Message], err error) {

	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	options := model.GetCommonOptions(&model.Options{
		Temperature: cm.config.Temperature,
		MaxTokens:   cm.config.MaxTokens,
		Model:       &cm.config.Model,
		TopP:        cm.config.TopP,
		Stop:        cm.config.Stop,
	}, opts...)

	req, err := cm.genRequest(in, options)
	if err != nil {
		return nil, err
	}

	req.Stream = true
	req.StreamOptions = &openai.StreamOptions{IncludeUsage: true}

	reqConf := &model.Config{
		Model:       req.Model,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stop:        req.Stop,
	}

	ctx = callbacks.OnStart(ctx, &model.CallbackInput{
		Messages:   in,
		Tools:      append(cm.rawTools), // join tool info from call options
		ToolChoice: getToolChoice(req.ToolChoice),
		Config:     reqConf,
	})

	stream, err := cm.cli.CreateChatCompletionStream(ctx, *req)
	if err != nil {
		return nil, err
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
						Config:     reqConf,
						TokenUsage: toModelCallbackUsage(lastEmptyMsg.ResponseMeta),
					}, nil)
				}
				return
			}

			if chunkErr != nil {
				_ = sw.Send(nil, chunkErr)
				return
			}

			// stream usage return in last chunk without message content, then
			// last message received from callback output stream: Message == nil and TokenUsage != nil
			// last message received from outStream: Message != nil
			msg, found := cm.resolveStreamResponse(chunk)
			if !found {
				continue
			}

			// skip empty message
			// when openai return parallel tool calls, first frame can be empty
			// skip empty frame in stream, then stream first frame could know whether is tool call msg.
			if lastEmptyMsg != nil {
				cMsg, cErr := schema.ConcatMessages([]*schema.Message{lastEmptyMsg, msg})
				if cErr != nil { // nolint: byted_s_too_many_nests_in_func
					_ = sw.Send(nil, cErr)
					return
				}

				msg = cMsg
			}

			if msg.Content == "" && len(msg.ToolCalls) == 0 {
				lastEmptyMsg = msg
				continue
			}

			lastEmptyMsg = nil

			closed := sw.Send(&model.CallbackOutput{
				Message:    msg,
				Config:     reqConf,
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

func (cm *Client) resolveStreamResponse(resp openai.ChatCompletionStreamResponse) (msg *schema.Message, found bool) {
	for _, choice := range resp.Choices {
		// take 0 index as response, rewrite if needed
		if choice.Index != 0 {
			continue
		}

		found = true
		msg = &schema.Message{
			Role:      toMessageRole(choice.Delta.Role),
			Content:   choice.Delta.Content,
			ToolCalls: toMessageToolCalls(choice.Delta.ToolCalls),
			ResponseMeta: &schema.ResponseMeta{
				FinishReason: string(choice.FinishReason),
				Usage:        toEinoTokenUsage(resp.Usage),
			},
		}

		break
	}

	if resp.Usage != nil && !found {
		msg = &schema.Message{
			ResponseMeta: &schema.ResponseMeta{
				Usage: toEinoTokenUsage(resp.Usage),
			},
		}
		found = true
	}

	return msg, found
}

func toTools(tis []*schema.ToolInfo) ([]tool, error) {
	tools := make([]tool, len(tis))
	for i := range tis {
		ti := tis[i]
		if ti == nil {
			return nil, errors.New("unexpected nil tool")
		}

		paramsJSONSchema, err := ti.ParamsOneOf.ToOpenAPIV3()
		if err != nil {
			return nil, fmt.Errorf("convert toolInfo ParamsOneOf to JSONSchema failed: %w", err)
		}

		tools[i] = tool{
			Function: &functionDefinition{
				Name:        ti.Name,
				Description: ti.Desc,
				Parameters:  paramsJSONSchema,
			},
		}
	}

	return tools, nil
}

func toEinoTokenUsage(usage *openai.Usage) *schema.TokenUsage {
	if usage == nil {
		return nil
	}
	return &schema.TokenUsage{
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
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

func (cm *Client) BindTools(tools []*schema.ToolInfo) error {
	var err error
	cm.tools, err = toTools(tools)
	if err != nil {
		return err
	}

	cm.forceToolCall = false
	cm.rawTools = tools

	return nil
}

func (cm *Client) BindForcedTools(tools []*schema.ToolInfo) error {
	var err error
	cm.tools, err = toTools(tools)
	if err != nil {
		return err
	}

	cm.forceToolCall = true
	cm.rawTools = tools

	return nil
}

func getToolChoice(choice any) any {
	switch t := choice.(type) {
	case string:
		return t
	case openai.ToolChoice:
		return &schema.ToolInfo{
			Name: t.Function.Name,
		}
	default:
		return nil
	}
}

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
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/bytedance/sonic"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/azure"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/shared"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/getkin/kin-openapi/openapi3"
)

type ChatCompletionResponseFormatType string

const (
	ChatCompletionResponseFormatTypeJSONObject ChatCompletionResponseFormatType = "json_object"
	ChatCompletionResponseFormatTypeJSONSchema ChatCompletionResponseFormatType = "json_schema"
	ChatCompletionResponseFormatTypeText       ChatCompletionResponseFormatType = "text"
)

const (
	toolChoiceNone     = "none"     // none means the model will not call any tool and instead generates a message.
	toolChoiceAuto     = "auto"     // auto means the model can pick between generating a message or calling one or more tools.
	toolChoiceRequired = "required" // required means the model must call one or more tools.
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
	// APIKey is your authentication key
	// Use OpenAI API key or Azure API key depending on the service
	// Required
	APIKey string `json:"api_key"`

	// HTTPClient is used to send HTTP requests
	// Optional. Default: http.DefaultClient
	HTTPClient *http.Client `json:"-"`

	// The following three fields are only required when using Azure OpenAI Service, otherwise they can be ignored.
	// For more details, see: https://learn.microsoft.com/en-us/azure/ai-services/openai/

	// ByAzure indicates whether to use Azure OpenAI Service
	// Required for Azure
	ByAzure bool `json:"by_azure"`

	// BaseURL is the Azure OpenAI endpoint URL
	// Format: https://{YOUR_RESOURCE_NAME}.openai.azure.com. YOUR_RESOURCE_NAME is the name of your resource that you have created on Azure.
	// Required for Azure
	BaseURL string `json:"base_url"`

	// APIVersion specifies the Azure OpenAI API version
	// Required for Azure
	APIVersion string `json:"api_version"`

	// The following fields correspond to OpenAI's common parameters

	// Model specifies the ID of the model to use
	// Required
	Model string `json:"model"`

	// User unique identifier representing end-user
	// Optional. Helps OpenAI monitor and detect abuse
	User *string `json:"user,omitempty"`

	// The following fields correspond to OpenAI's chat completion API parameters
	// Ref: https://platform.openai.com/docs/api-reference/chat/create

	// MaxTokens limits the maximum number of tokens that can be generated in the chat completion
	// Optional. Default: model's maximum
	MaxTokens *int `json:"max_tokens,omitempty"`

	// Temperature specifies what sampling temperature to use
	// Generally recommend altering this or TopP but not both.
	// Range: 0.0 to 2.0. Higher values make output more random
	// Optional. Default: 1.0
	Temperature *float32 `json:"temperature,omitempty"`

	// TopP controls diversity via nucleus sampling
	// Generally recommend altering this or Temperature but not both.
	// Range: 0.0 to 1.0. Lower values make output more focused
	// Optional. Default: 1.0
	TopP *float32 `json:"top_p,omitempty"`

	// Stop sequences where the API will stop generating further tokens
	// Optional. Example: []string{"\n", "User:"}
	Stop []string `json:"stop,omitempty"`

	// PresencePenalty prevents repetition by penalizing tokens based on presence
	// Range: -2.0 to 2.0. Positive values increase likelihood of new topics
	// Optional. Default: 0
	PresencePenalty *float32 `json:"presence_penalty,omitempty"`

	// ResponseFormat specifies the format of the model's response
	// Optional. Use for structured outputs
	ResponseFormat *ChatCompletionResponseFormat `json:"response_format,omitempty"`

	// Seed enables deterministic sampling for consistent outputs
	// Optional. Set for reproducible results
	Seed *int `json:"seed,omitempty"`

	// FrequencyPenalty prevents repetition by penalizing tokens based on frequency
	// Range: -2.0 to 2.0. Positive values decrease likelihood of repetition
	// Optional. Default: 0
	FrequencyPenalty *float32 `json:"frequency_penalty,omitempty"`

	// LogitBias modifies likelihood of specific tokens appearing in completion
	// Optional. Map token IDs to bias values from -100 to 100
	LogitBias map[string]int `json:"logit_bias,omitempty"`

	// The following fields correspond to OpenAI's embedding API parameters
	//Ref: https://platform.openai.com/docs/api-reference/embeddings/create

	// EncodingFormat specifies the format of the embeddings output
	// Optional. Default: EmbeddingEncodingFormatFloat
	EncodingFormat *EmbeddingEncodingFormat `json:"encoding_format,omitempty"`

	// Dimensions specifies the number of dimensions the resulting output embeddings should have
	// Optional. Only supported in text-embedding-3 and later models
	Dimensions *int `json:"dimensions,omitempty"`
}

var _ model.ChatModel = (*Client)(nil)

type Client struct {
	cli    *openai.Client
	config *Config

	tools      []tool
	rawTools   []*schema.ToolInfo
	toolChoice *schema.ToolChoice
}

func NewClient(_ context.Context, config *Config) (*Client, error) {
	if config == nil {
		return nil, fmt.Errorf("OpenAI client config cannot be nil")
	}

	var opts []option.RequestOption

	if config.ByAzure {
		opts = append(opts, azure.WithAPIKey(config.APIKey), azure.WithEndpoint(config.BaseURL, config.APIVersion))
	} else {
		opts = append(opts, option.WithAPIKey(config.APIKey))
		if len(config.BaseURL) > 0 {
			opts = append(opts, option.WithBaseURL(config.BaseURL))
		}
	}

	if config.HTTPClient != nil {
		opts = append(opts, option.WithHTTPClient(config.HTTPClient))
	}

	return &Client{
		cli:    openai.NewClient(opts...),
		config: config,
	}, nil
}

func toOpenAIRole(role schema.RoleType) openai.ChatCompletionMessageParamRole {
	switch role {
	case schema.User:
		return openai.ChatCompletionMessageParamRoleUser
	case schema.Assistant:
		return openai.ChatCompletionMessageParamRoleAssistant
	case schema.System:
		return openai.ChatCompletionMessageParamRoleSystem
	case schema.Tool:
		return openai.ChatCompletionMessageParamRoleTool
	default:
		return openai.ChatCompletionMessageParamRole(role)
	}
}

func toOpenAIMultiContent(mc []schema.ChatMessagePart) ([]openai.ChatCompletionContentPartUnionParam, error) {
	if len(mc) == 0 {
		return nil, nil
	}

	ret := make([]openai.ChatCompletionContentPartUnionParam, 0, len(mc))

	for _, part := range mc {
		switch part.Type {
		case schema.ChatMessagePartTypeText:
			ret = append(ret, openai.ChatCompletionContentPartTextParam{
				Type: openai.F(openai.ChatCompletionContentPartTextTypeText),
				Text: openai.F(part.Text),
			})
		case schema.ChatMessagePartTypeImageURL:
			if part.ImageURL == nil {
				return nil, fmt.Errorf("ImageURL field must not be nil when Type is ChatMessagePartTypeImageURL")
			}
			ret = append(ret, openai.ChatCompletionContentPartImageParam{
				Type: openai.F(openai.ChatCompletionContentPartImageTypeImageURL),
				ImageURL: openai.F(openai.ChatCompletionContentPartImageImageURLParam{
					URL:    openai.F(part.ImageURL.URL),
					Detail: openai.F(openai.ChatCompletionContentPartImageImageURLDetail(part.ImageURL.Detail)),
				}),
			})
		default:
			return nil, fmt.Errorf("unsupported chat message part type: %s", part.Type)
		}
	}

	return ret, nil
}

func toMessageRole(role openai.ChatCompletionMessageRole) schema.RoleType {
	switch role {
	case openai.ChatCompletionMessageRoleAssistant:
		return schema.Assistant
	default:
		return schema.RoleType(role)
	}
}

func chunkToMessageRole(role openai.ChatCompletionChunkChoicesDeltaRole) schema.RoleType {
	switch role {
	case openai.ChatCompletionChunkChoicesDeltaRoleAssistant:
		return schema.Assistant
	case openai.ChatCompletionChunkChoicesDeltaRoleSystem:
		return schema.System
	case openai.ChatCompletionChunkChoicesDeltaRoleTool:
		return schema.Tool
	case openai.ChatCompletionChunkChoicesDeltaRoleUser:
		return schema.User
	case openai.ChatCompletionChunkChoicesDeltaRoleDeveloper:
		return schema.System
	default:
		return schema.RoleType(role)
	}
}

func toMessageToolCalls(toolCalls []openai.ChatCompletionMessageToolCall) []schema.ToolCall {
	if len(toolCalls) == 0 {
		return nil
	}

	ret := make([]schema.ToolCall, len(toolCalls))
	for i := range toolCalls {
		toolCall := toolCalls[i]
		ret[i] = schema.ToolCall{
			ID:   toolCall.ID,
			Type: string(toolCall.Type),
			Function: schema.FunctionCall{
				Name:      toolCall.Function.Name,
				Arguments: toolCall.Function.Arguments,
			},
		}
	}

	return ret
}

func chunkToMessageToolCalls(toolCalls []openai.ChatCompletionChunkChoicesDeltaToolCall) []schema.ToolCall {
	if len(toolCalls) == 0 {
		return nil
	}

	ret := make([]schema.ToolCall, len(toolCalls))
	for i := range toolCalls {
		toolCall := toolCalls[i]
		idx := int(toolCall.Index)
		ret[i] = schema.ToolCall{
			Index: &idx,
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

func toOpenAIToolCalls(toolCalls []schema.ToolCall) []openai.ChatCompletionMessageToolCallParam {
	if len(toolCalls) == 0 {
		return nil
	}
	ret := make([]openai.ChatCompletionMessageToolCallParam, len(toolCalls))
	for i := range toolCalls {
		toolCall := toolCalls[i]
		ret[i] = openai.ChatCompletionMessageToolCallParam{
			ID:   openai.F(toolCall.ID),
			Type: openai.F(openai.ChatCompletionMessageToolCallTypeFunction),
			Function: openai.F(openai.ChatCompletionMessageToolCallFunctionParam{
				Name:      openai.F(toolCall.Function.Name),
				Arguments: openai.F(toolCall.Function.Arguments),
			}),
		}
	}

	return ret
}

func (cm *Client) genRequest(in []*schema.Message, opts ...model.Option) (*openai.ChatCompletionNewParams, *model.CallbackInput, error) {
	options := model.GetCommonOptions(&model.Options{
		Temperature: cm.config.Temperature,
		MaxTokens:   cm.config.MaxTokens,
		Model:       &cm.config.Model,
		TopP:        cm.config.TopP,
		Stop:        cm.config.Stop,
		Tools:       nil,
		ToolChoice:  cm.toolChoice,
	}, opts...)

	req := &openai.ChatCompletionNewParams{
		Model: openai.F[openai.ChatModel](*options.Model),
	}
	if options.MaxTokens != nil {
		req.MaxTokens = openai.F(int64(*options.MaxTokens))
	}
	if options.Temperature != nil {
		req.Temperature = openai.F(float64(*options.Temperature))
	}
	if options.TopP != nil {
		req.TopP = openai.F(float64(*options.TopP))
	}
	if len(options.Stop) > 0 {
		req.Stop = openai.F[openai.ChatCompletionNewParamsStopUnion](openai.ChatCompletionNewParamsStopArray(cm.config.Stop))
	}
	if cm.config.PresencePenalty != nil {
		req.PresencePenalty = openai.F(float64(*cm.config.PresencePenalty))
	}
	if cm.config.Seed != nil {
		req.Seed = openai.F(int64(*cm.config.Seed))
	}
	if cm.config.FrequencyPenalty != nil {
		req.FrequencyPenalty = openai.F(float64(*cm.config.FrequencyPenalty))
	}
	if cm.config.LogitBias != nil && len(cm.config.LogitBias) > 0 {
		logitBias := make(map[string]int64, len(cm.config.LogitBias))
		for k, v := range cm.config.LogitBias {
			logitBias[k] = int64(v)
		}
		req.LogitBias = openai.F(logitBias)
	}
	if cm.config.User != nil {
		req.User = openai.F(*cm.config.User)
	}

	cbInput := &model.CallbackInput{
		Messages: in,
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
		if tools, err = toTools(options.Tools); err != nil {
			return nil, nil, err
		}
		cbInput.Tools = options.Tools
	}

	if len(tools) > 0 {
		reqTools := make([]openai.ChatCompletionToolParam, len(cm.tools))
		for i := range tools {
			t := tools[i]

			param := make(map[string]any)
			intermediate, err := sonic.Marshal(t.Function.Parameters)
			if err != nil {
				return nil, nil, fmt.Errorf("convert function parameter fail, tool name: %s, error: %w", t.Function.Name, err)
			}
			err = sonic.Unmarshal(intermediate, &param)
			if err != nil {
				return nil, nil, fmt.Errorf("convert function parameter fail, tool name: %s, error: %w", t.Function.Name, err)
			}

			reqTools[i] = openai.ChatCompletionToolParam{
				Type: openai.F(openai.ChatCompletionToolTypeFunction),
				Function: openai.F(shared.FunctionDefinitionParam{
					Name:        openai.F(t.Function.Name),
					Description: openai.F(t.Function.Description),
					Parameters:  openai.F[openai.FunctionParameters](param),
				}),
			}
		}
		req.Tools = openai.F(reqTools)
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
			req.ToolChoice = openai.F[openai.ChatCompletionToolChoiceOptionUnionParam](openai.ChatCompletionToolChoiceOptionAutoNone)
		case schema.ToolChoiceAllowed:
			req.ToolChoice = openai.F[openai.ChatCompletionToolChoiceOptionUnionParam](openai.ChatCompletionToolChoiceOptionAutoAuto)
		case schema.ToolChoiceForced:
			if len(req.Tools.Value) == 0 {
				return nil, nil, fmt.Errorf("tool choice is forced but tool is not provided")
			} else if len(req.Tools.Value) > 1 {
				req.ToolChoice = openai.F[openai.ChatCompletionToolChoiceOptionUnionParam](openai.ChatCompletionToolChoiceOptionAutoRequired)
			} else {
				req.ToolChoice = openai.F[openai.ChatCompletionToolChoiceOptionUnionParam](openai.ChatCompletionNamedToolChoiceParam{
					Type: openai.F(openai.ChatCompletionNamedToolChoiceTypeFunction),
					Function: openai.F(openai.ChatCompletionNamedToolChoiceFunctionParam{
						Name: req.Tools.Value[0].Function.Value.Name,
					}),
				})
			}
		default:
			return nil, nil, fmt.Errorf("tool choice=%s not support", *options.ToolChoice)
		}
	}

	msgs := make([]openai.ChatCompletionMessageParamUnion, 0, len(in))
	for _, inMsg := range in {
		if len(inMsg.Content) > 0 && len(inMsg.MultiContent) > 0 {
			return nil, nil, fmt.Errorf("can't use both Content and MultiContent properties simultaneously")
		}

		mc, e := toOpenAIMultiContent(inMsg.MultiContent)
		if e != nil {
			return nil, nil, e
		}
		msg := openai.ChatCompletionMessageParam{
			Role:       openai.F(toOpenAIRole(inMsg.Role)),
			ToolCalls:  openai.F[any](toOpenAIToolCalls(inMsg.ToolCalls)),
			ToolCallID: openai.F(inMsg.ToolCallID),
		}
		if len(inMsg.Name) > 0 {
			msg.Name = openai.F(inMsg.Name)
		}
		if len(mc) > 0 {
			msg.Content = openai.F[any](mc)
		} else {
			msg.Content = openai.F[any](inMsg.Content)
		}

		msgs = append(msgs, msg)
	}
	req.Messages = openai.F(msgs)

	if cm.config.ResponseFormat != nil {
		var respFormat openai.ChatCompletionNewParamsResponseFormatUnion
		switch cm.config.ResponseFormat.Type {
		case ChatCompletionResponseFormatTypeText:
			respFormat = openai.ResponseFormatTextParam{Type: openai.F(shared.ResponseFormatTextTypeText)}
		case ChatCompletionResponseFormatTypeJSONObject:
			respFormat = openai.ResponseFormatJSONObjectParam{Type: openai.F(shared.ResponseFormatJSONObjectTypeJSONObject)}
		case ChatCompletionResponseFormatTypeJSONSchema:
			respFormat = openai.ResponseFormatJSONSchemaParam{Type: openai.F(shared.ResponseFormatJSONSchemaTypeJSONSchema), JSONSchema: openai.F(openai.ResponseFormatJSONSchemaJSONSchemaParam{
				Name:        openai.F(cm.config.ResponseFormat.JSONSchema.Name),
				Description: openai.F(cm.config.ResponseFormat.JSONSchema.Description),
				Schema:      openai.F[any](cm.config.ResponseFormat.JSONSchema.Schema),
				Strict:      openai.F(cm.config.ResponseFormat.JSONSchema.Strict),
			})}
		default:
			return nil, nil, fmt.Errorf("unknown response format type: %s", cm.config.ResponseFormat.Type)
		}
		req.ResponseFormat = openai.F(respFormat)
	}

	return req, cbInput, nil
}

func (cm *Client) Generate(ctx context.Context, in []*schema.Message, opts ...model.Option) (
	outMsg *schema.Message, err error) {

	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	req, cbInput, err := cm.genRequest(in, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat completion request: %w", err)
	}

	ctx = callbacks.OnStart(ctx, cbInput)

	resp, err := cm.cli.Chat.Completions.New(ctx, *req)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat completion: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("received empty choices from OpenAI API response")
	}

	for _, choice := range resp.Choices {
		if choice.Index != 0 {
			continue
		}

		msg := choice.Message
		outMsg = &schema.Message{
			Role:      toMessageRole(msg.Role),
			Content:   msg.Content,
			ToolCalls: toMessageToolCalls(msg.ToolCalls),
			ResponseMeta: &schema.ResponseMeta{
				FinishReason: string(choice.FinishReason),
				Usage:        toEinoTokenUsage(&resp.Usage),
			},
		}

		break
	}

	if outMsg == nil {
		return nil, fmt.Errorf("invalid response format: choice with index 0 not found")
	}

	usage := &model.TokenUsage{
		PromptTokens:     int(resp.Usage.PromptTokens),
		CompletionTokens: int(resp.Usage.CompletionTokens),
		TotalTokens:      int(resp.Usage.TotalTokens),
	}

	callbacks.OnEnd(ctx, &model.CallbackOutput{
		Message:    outMsg,
		Config:     cbInput.Config,
		TokenUsage: usage,
	})

	return outMsg, nil
}

func (cm *Client) Stream(ctx context.Context, in []*schema.Message,
	opts ...model.Option) (outStream *schema.StreamReader[*schema.Message], err error) {

	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	req, cbInput, err := cm.genRequest(in, opts...)
	if err != nil {
		return nil, err
	}

	req.StreamOptions = openai.F(openai.ChatCompletionStreamOptionsParam{IncludeUsage: openai.F(true)})

	ctx = callbacks.OnStart(ctx, cbInput)

	stream := cm.cli.Chat.Completions.NewStreaming(ctx, *req)

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

		for stream.Next() {
			chunk := stream.Current()

			// stream usage return in last chunk without message content, then
			// last message received from callback output stream: Message == nil and TokenUsage != nil
			// last message received from outStream: Message != nil
			msg, found := resolveStreamResponse(chunk)
			if !found {
				continue
			}

			// skip empty message
			// when openai return parallel tool calls, first frame can be empty
			// skip empty frame in stream, then stream first frame could know whether is tool call msg.
			if lastEmptyMsg != nil {
				cMsg, cErr := schema.ConcatMessages([]*schema.Message{lastEmptyMsg, msg})
				if cErr != nil {
					_ = sw.Send(nil, fmt.Errorf("failed to concatenate stream messages: %w", cErr))
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
				Config:     cbInput.Config,
				TokenUsage: toModelCallbackUsage(msg.ResponseMeta),
			}, nil)

			if closed {
				return
			}
		}
		if streamErr := stream.Err(); streamErr != nil {
			_ = sw.Send(nil, streamErr)
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

func resolveStreamResponse(resp openai.ChatCompletionChunk) (msg *schema.Message, found bool) {
	for _, choice := range resp.Choices {
		// take 0 index as response, rewrite if needed
		if choice.Index != 0 {
			continue
		}

		found = true
		msg = &schema.Message{
			Role:      chunkToMessageRole(choice.Delta.Role),
			Content:   choice.Delta.Content,
			ToolCalls: chunkToMessageToolCalls(choice.Delta.ToolCalls),
			ResponseMeta: &schema.ResponseMeta{
				FinishReason: string(choice.FinishReason),
				Usage:        toEinoTokenUsage(&resp.Usage),
			},
		}

		break
	}

	if !found {
		msg = &schema.Message{
			ResponseMeta: &schema.ResponseMeta{
				Usage: toEinoTokenUsage(&resp.Usage),
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
			return nil, fmt.Errorf("tool info cannot be nil in BindTools")
		}

		paramsJSONSchema, err := ti.ParamsOneOf.ToOpenAPIV3()
		if err != nil {
			return nil, fmt.Errorf("failed to convert tool parameters to JSONSchema: %w", err)
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

func toEinoTokenUsage(usage *openai.CompletionUsage) *schema.TokenUsage {
	if usage == nil {
		return nil
	}
	return &schema.TokenUsage{
		PromptTokens:     int(usage.PromptTokens),
		CompletionTokens: int(usage.CompletionTokens),
		TotalTokens:      int(usage.TotalTokens),
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

	tc := schema.ToolChoiceAllowed
	cm.toolChoice = &tc
	cm.rawTools = tools

	return nil
}

func (cm *Client) BindForcedTools(tools []*schema.ToolInfo) error {
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

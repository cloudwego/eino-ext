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
	"sort"

	"github.com/eino-contrib/jsonschema"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/meguminnnnnnnnn/go-openai"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
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
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	// Deprecated: use JSONSchema instead.
	Schema     *openapi3.Schema   `json:"-"`
	JSONSchema *jsonschema.Schema `json:"-"`
	Strict     bool               `json:"strict"`
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

	// AzureModelMapperFunc is used to map the model name to the deployment name for Azure OpenAI Service.
	// This is useful when the model name is different from the deployment name.
	// Optional for Azure, remove [,:] from the model name by default.
	AzureModelMapperFunc func(model string) string

	// BaseURL is the Azure OpenAI endpoint URL
	// Format: https://{YOUR_RESOURCE_NAME}.openai.azure.com. YOUR_RESOURCE_NAME is the name of your resource that you have created on Azure.
	// Required for Azure
	BaseURL string `json:"base_url"`

	// APIVersion specifies the Azure OpenAI API version
	// Required for Azure
	APIVersion string `json:"api_version"`

	// The following fields correspond to OpenAI's chat completion API parameters
	// Ref: https://platform.openai.com/docs/api-reference/chat/create

	// Model specifies the ID of the model to use
	// Required
	Model string `json:"model"`

	// MaxTokens limits the maximum number of tokens that can be generated in the chat completion
	// Optional. Default: model's maximum
	MaxTokens *int `json:"max_tokens,omitempty"`

	// MaxCompletionTokens specifies an upper bound for the number of tokens that can be generated for a completion, including visible output tokens and reasoning tokens.
	MaxCompletionTokens *int `json:"max_completion_tokens,omitempty"`

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

	// User unique identifier representing end-user
	// Optional. Helps OpenAI monitor and detect abuse
	User *string `json:"user,omitempty"`

	// LogProbs specifies whether to return log probabilities of the output tokens.
	LogProbs bool `json:"log_probs"`

	// TopLogProbs specifies the number of most likely tokens to return at each token position, each with an associated log probability.
	TopLogProbs int `json:"top_log_probs"`

	// ExtraFields will override any existing fields with the same key.
	// Optional. Useful for experimental features not yet officially supported.
	ExtraFields map[string]any `json:"-"`

	// ReasoningEffort will override the default reasoning level of "medium"
	// Optional. Useful for fine tuning response latency vs. accuracy
	ReasoningEffort ReasoningEffortLevel
}

type Client struct {
	cli    *openai.Client
	config *Config

	tools      []tool
	rawTools   []*schema.ToolInfo
	toolChoice *schema.ToolChoice
}

func NewClient(ctx context.Context, config *Config) (*Client, error) {
	if config == nil {
		return nil, fmt.Errorf("OpenAI client config cannot be nil")
	}

	var clientConf openai.ClientConfig

	if config.ByAzure {
		clientConf = openai.DefaultAzureConfig(config.APIKey, config.BaseURL)
		if config.APIVersion != "" {
			clientConf.APIVersion = config.APIVersion
		}
		if config.AzureModelMapperFunc != nil {
			clientConf.AzureModelMapperFunc = config.AzureModelMapperFunc
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
				return nil, fmt.Errorf("ImageURL field must not be nil when Type is ChatMessagePartTypeImageURL")
			}
			ret = append(ret, openai.ChatMessagePart{
				Type: openai.ChatMessagePartTypeImageURL,
				ImageURL: &openai.ChatMessageImageURL{
					URL:    part.ImageURL.URL,
					Detail: openai.ImageURLDetail(part.ImageURL.Detail),
				},
			})
		case schema.ChatMessagePartTypeAudioURL:
			if part.AudioURL == nil {
				return nil, fmt.Errorf("AudioURL field must not be nil when Type is ChatMessagePartTypeAudioURL")
			}
			ret = append(ret, openai.ChatMessagePart{
				Type: openai.ChatMessagePartTypeInputAudio,
				InputAudio: &openai.ChatMessageInputAudio{
					Data:   part.AudioURL.URL,
					Format: part.AudioURL.MIMEType,
				},
			})
		case schema.ChatMessagePartTypeVideoURL:
			if part.VideoURL == nil {
				return nil, fmt.Errorf("VideoURL field must not be nil when Type is ChatMessagePartTypeVideoURL")
			}
			ret = append(ret, openai.ChatMessagePart{
				Type: openai.ChatMessagePartTypeVideoURL,
				VideoURL: &openai.ChatMessageVideoURL{
					URL: part.VideoURL.URL,
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
	case "":
		// When the role field is an empty string, populate it with the schema.Assistant.
		return schema.Assistant
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

func (c *Client) genRequest(in []*schema.Message, opts ...model.Option) (*openai.ChatCompletionRequest, *model.CallbackInput, error) {

	options := model.GetCommonOptions(&model.Options{
		Temperature: c.config.Temperature,
		MaxTokens:   c.config.MaxTokens,
		Model:       &c.config.Model,
		TopP:        c.config.TopP,
		Stop:        c.config.Stop,
		Tools:       nil,
		ToolChoice:  c.toolChoice,
	}, opts...)
	specOptions := model.GetImplSpecificOptions(&openaiOptions{
		ExtraFields:         c.config.ExtraFields,
		ReasoningEffort:     c.config.ReasoningEffort,
		MaxCompletionTokens: c.config.MaxCompletionTokens,
	}, opts...)

	req := &openai.ChatCompletionRequest{
		Model:               *options.Model,
		MaxTokens:           dereferenceOrZero(options.MaxTokens),
		MaxCompletionTokens: dereferenceOrZero(specOptions.MaxCompletionTokens),
		Temperature:         options.Temperature,
		TopP:                dereferenceOrZero(options.TopP),
		Stop:                options.Stop,
		PresencePenalty:     dereferenceOrZero(c.config.PresencePenalty),
		Seed:                c.config.Seed,
		FrequencyPenalty:    dereferenceOrZero(c.config.FrequencyPenalty),
		LogitBias:           c.config.LogitBias,
		User:                dereferenceOrZero(c.config.User),
		LogProbs:            c.config.LogProbs,
		TopLogProbs:         c.config.TopLogProbs,
		ReasoningEffort:     string(specOptions.ReasoningEffort),
	}

	if len(specOptions.ExtraFields) > 0 {
		req.SetExtraFields(specOptions.ExtraFields)
	}

	cbInput := &model.CallbackInput{
		Messages: in,
		Tools:    c.rawTools,
		Config: &model.Config{
			Model:       req.Model,
			MaxTokens:   req.MaxTokens,
			Temperature: dereferenceOrZero(req.Temperature),
			TopP:        req.TopP,
			Stop:        req.Stop,
		},
	}

	tools := c.tools
	if options.Tools != nil {
		var err error
		if tools, err = toTools(options.Tools); err != nil {
			return nil, nil, err
		}
		cbInput.Tools = options.Tools
	}

	if len(tools) > 0 {
		req.Tools = make([]openai.Tool, len(tools))
		for i := range tools {
			t := tools[i]

			req.Tools[i] = openai.Tool{
				Type: openai.ToolTypeFunction,
				Function: &openai.FunctionDefinition{
					Name:        t.Function.Name,
					Description: t.Function.Description,
					Parameters:  t.Function.Parameters,
				},
			}
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
				req.ToolChoice = openai.ToolChoice{
					Type: req.Tools[0].Type,
					Function: openai.ToolFunction{
						Name: req.Tools[0].Function.Name,
					},
				}
			}
		default:
			return nil, nil, fmt.Errorf("tool choice=%s not support", *options.ToolChoice)
		}
	}

	msgs := make([]openai.ChatCompletionMessage, 0, len(in))
	for _, inMsg := range in {
		mc, e := toOpenAIMultiContent(inMsg.MultiContent)
		if e != nil {
			return nil, nil, e
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

	if c.config.ResponseFormat != nil {
		req.ResponseFormat = &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatType(c.config.ResponseFormat.Type),
		}
		if c.config.ResponseFormat.JSONSchema != nil {
			req.ResponseFormat.JSONSchema = &openai.ChatCompletionResponseFormatJSONSchema{
				Name:        c.config.ResponseFormat.JSONSchema.Name,
				Description: c.config.ResponseFormat.JSONSchema.Description,
				Schema:      c.config.ResponseFormat.JSONSchema.Schema,
				Strict:      c.config.ResponseFormat.JSONSchema.Strict,
			}
		}
	}

	return req, cbInput, nil
}

func (c *Client) Generate(ctx context.Context, in []*schema.Message, opts ...model.Option) (
	outMsg *schema.Message, err error) {

	req, cbInput, err := c.genRequest(in, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat completion request: %w", err)
	}

	ctx = callbacks.OnStart(ctx, cbInput)
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	reqOpts := c.getChatCompletionRequestOptions(opts)

	resp, err := c.cli.CreateChatCompletion(ctx, *req, reqOpts...)
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
			Role:       toMessageRole(msg.Role),
			Content:    msg.Content,
			Name:       msg.Name,
			ToolCallID: msg.ToolCallID,
			ToolCalls:  toMessageToolCalls(msg.ToolCalls),
			ResponseMeta: &schema.ResponseMeta{
				FinishReason: string(choice.FinishReason),
				Usage:        toEinoTokenUsage(&resp.Usage),
				LogProbs:     toLogProbs(choice.LogProbs),
			},
		}
		if len(msg.ReasoningContent) > 0 {
			outMsg.ReasoningContent = msg.ReasoningContent
			setReasoningContent(outMsg, msg.ReasoningContent)
		}

		break
	}

	if outMsg == nil {
		return nil, fmt.Errorf("invalid response format: choice with index 0 not found")
	}

	callbacks.OnEnd(ctx, &model.CallbackOutput{
		Message:    outMsg,
		Config:     cbInput.Config,
		TokenUsage: toModelCallbackUsage(outMsg.ResponseMeta),
	})

	return outMsg, nil
}

func (c *Client) Stream(ctx context.Context, in []*schema.Message,
	opts ...model.Option) (outStream *schema.StreamReader[*schema.Message], err error) {

	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	req, cbInput, err := c.genRequest(in, opts...)
	if err != nil {
		return nil, err
	}

	req.Stream = true
	req.StreamOptions = &openai.StreamOptions{IncludeUsage: true}

	ctx = callbacks.OnStart(ctx, cbInput)

	reqOpts := c.getChatCompletionRequestOptions(opts)

	stream, err := c.cli.CreateChatCompletionStream(ctx, *req, reqOpts...)
	if err != nil {
		return nil, err
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
				_ = sw.Send(nil, fmt.Errorf("failed to receive stream chunk from OpenAI: %w", chunkErr))
				return
			}

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
			rc, ok := GetReasoningContent(msg)
			if lastEmptyMsg != nil {
				cMsg, cErr := schema.ConcatMessages([]*schema.Message{lastEmptyMsg, msg})
				if cErr != nil {
					_ = sw.Send(nil, fmt.Errorf("failed to concatenate stream messages: %w", cErr))
					return
				}

				msg = cMsg
			}

			if msg.Content == "" && len(msg.ToolCalls) == 0 && !(ok && len(rc) > 0) {
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

func (c *Client) getChatCompletionRequestOptions(opts []model.Option) []openai.ChatCompletionRequestOption {
	specOptions := model.GetImplSpecificOptions(&openaiOptions{
		ExtraFields:     c.config.ExtraFields,
		ReasoningEffort: c.config.ReasoningEffort,
	}, opts...)

	var options []openai.ChatCompletionRequestOption

	if specOptions.RequestBodyModifier != nil {
		options = append(options, openai.WithRequestBodyModifier(specOptions.RequestBodyModifier))
	}

	if specOptions.ExtraHeader != nil {
		options = append(options, openai.WithExtraHeader(specOptions.ExtraHeader))
	}

	return options
}

func toStreamProbs(probs *openai.ChatCompletionStreamChoiceLogprobs) *schema.LogProbs {
	if probs == nil {
		return nil
	}
	ret := &schema.LogProbs{}
	for _, content := range probs.Content {
		schemaContent := schema.LogProb{
			Token:       content.Token,
			LogProb:     content.Logprob,
			Bytes:       content.Bytes,
			TopLogProbs: toStreamTopLogProb(content.TopLogprobs),
		}
		ret.Content = append(ret.Content, schemaContent)
	}
	return ret
}

func toLogProbs(probs *openai.LogProbs) *schema.LogProbs {
	if probs == nil {
		return nil
	}
	ret := &schema.LogProbs{}
	for _, content := range probs.Content {
		schemaContent := schema.LogProb{
			Token:       content.Token,
			LogProb:     content.LogProb,
			Bytes:       byteSlice2int64(content.Bytes),
			TopLogProbs: toTopLogProb(content.TopLogProbs),
		}
		ret.Content = append(ret.Content, schemaContent)
	}
	return ret
}

func toStreamTopLogProb(probs []openai.ChatCompletionTokenLogprobTopLogprob) []schema.TopLogProb {
	ret := make([]schema.TopLogProb, 0, len(probs))
	for _, prob := range probs {
		ret = append(ret, schema.TopLogProb{
			Token:   prob.Token,
			LogProb: prob.Logprob,
			Bytes:   prob.Bytes,
		})
	}
	return ret
}

func toTopLogProb(probs []openai.TopLogProbs) []schema.TopLogProb {
	ret := make([]schema.TopLogProb, 0, len(probs))
	for _, prob := range probs {
		ret = append(ret, schema.TopLogProb{
			Token:   prob.Token,
			LogProb: prob.LogProb,
			Bytes:   byteSlice2int64(prob.Bytes),
		})
	}
	return ret
}

func byteSlice2int64(in []byte) []int64 {
	ret := make([]int64, 0, len(in))
	for _, v := range in {
		ret = append(ret, int64(v))
	}
	return ret
}

func resolveStreamResponse(resp openai.ChatCompletionStreamResponse) (msg *schema.Message, found bool) {
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
				LogProbs:     toStreamProbs(choice.Logprobs),
			},
		}

		if len(choice.Delta.ReasoningContent) > 0 {
			msg.ReasoningContent = choice.Delta.ReasoningContent
			setReasoningContent(msg, choice.Delta.ReasoningContent)
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
	var sortArrayFields func(*jsonschema.Schema)
	sortArrayFields = func(sc *jsonschema.Schema) {
		if sc == nil {
			return
		}

		switch sc.Type {
		case string(schema.Object):
			if len(sc.Required) == 0 {
				return
			}

			sort.Strings(sc.Required)
			for pair := sc.Properties.Oldest(); pair != nil; pair = pair.Next() {
				sortArrayFields(pair.Value)
			}

		case string(schema.Array):
			if sc.Items != nil {
				sortArrayFields(sc.Items)
			}

		default:
			return
		}
	}

	tools := make([]tool, len(tis))
	for i := range tis {
		ti := tis[i]
		if ti == nil {
			return nil, fmt.Errorf("tool info cannot be nil in BindTools")
		}

		paramsJSONSchema, err := ti.ParamsOneOf.ToJSONSchema()
		if err != nil {
			return nil, fmt.Errorf("failed to convert tool parameters to JSONSchema: %w", err)
		}

		sortArrayFields(paramsJSONSchema)

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

	promptTokenDetails := schema.PromptTokenDetails{}
	if usage.PromptTokensDetails != nil {
		promptTokenDetails.CachedTokens = usage.PromptTokensDetails.CachedTokens
	}

	return &schema.TokenUsage{
		PromptTokens:       usage.PromptTokens,
		PromptTokenDetails: promptTokenDetails,
		CompletionTokens:   usage.CompletionTokens,
		TotalTokens:        usage.TotalTokens,
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
		PromptTokens: usage.PromptTokens,
		PromptTokenDetails: model.PromptTokenDetails{
			CachedTokens: usage.PromptTokenDetails.CachedTokens,
		},
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
	}
}

func (c *Client) WithToolsForClient(tools []*schema.ToolInfo) (*Client, error) {
	if len(tools) == 0 {
		return nil, errors.New("no tools to bind")
	}
	openaiTools, err := toTools(tools)
	if err != nil {
		return nil, fmt.Errorf("convert to tools fail: %w", err)
	}

	tc := schema.ToolChoiceAllowed
	nc := *c
	nc.tools = openaiTools
	nc.rawTools = tools
	nc.toolChoice = &tc
	return &nc, nil
}

func (c *Client) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return c.WithToolsForClient(tools)
}

func (c *Client) BindTools(tools []*schema.ToolInfo) error {
	if len(tools) == 0 {
		return errors.New("no tools to bind")
	}
	var err error
	c.tools, err = toTools(tools)
	if err != nil {
		return err
	}

	tc := schema.ToolChoiceAllowed
	c.toolChoice = &tc
	c.rawTools = tools

	return nil
}

func (c *Client) BindForcedTools(tools []*schema.ToolInfo) error {
	if len(tools) == 0 {
		return errors.New("no tools to bind")
	}
	var err error
	c.tools, err = toTools(tools)
	if err != nil {
		return err
	}

	tc := schema.ToolChoiceForced
	c.toolChoice = &tc
	c.rawTools = tools

	return nil
}

func (c *Client) IsCallbacksEnabled() bool {
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

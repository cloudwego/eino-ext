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

package openairesponse

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

	openaiSDK "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/packages/param"
	"github.com/openai/openai-go/v3/packages/ssestream"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// ResponsesAPIConfig holds configuration for the OpenAI Responses API ChatModel.
type ResponsesAPIConfig struct {
	// APIKey is the OpenAI API key.
	// Required.
	APIKey string `json:"api_key"`

	// BaseURL overrides the default OpenAI API base URL.
	// Optional. Default: "https://api.openai.com/v1"
	BaseURL string `json:"base_url,omitempty"`

	// Timeout specifies the HTTP client timeout.
	// If HTTPClient is set, Timeout will not be used.
	// Optional. Default: no timeout
	Timeout time.Duration `json:"timeout,omitempty"`

	// HTTPClient specifies a custom HTTP client.
	// If set, Timeout will not be used.
	// Optional.
	HTTPClient *http.Client `json:"-"`

	Model string `json:"model"`

	// MaxOutputTokens sets an upper bound on the number of tokens the model may generate.
	// Optional.
	MaxOutputTokens *int `json:"max_output_tokens,omitempty"`

	// ResponseFormat configures the output format (text, json_object, or json_schema).
	// Optional.
	ResponseFormat *ChatCompletionResponseFormat `json:"response_format,omitempty"`

	BuiltinTools    []*schema.ToolInfo `json:"tools,omitempty"`
	EnableWebSearch bool               `json:"enable_web_search,omitempty"`
}

// ResponsesAPIChatModel implements the eino ChatModel interface using the OpenAI Responses API.
type ResponsesAPIChatModel struct {
	cli *openaiSDK.Client

	model           string
	maxTokens       *int
	temperature     *float32
	topP            *float32
	store           *bool
	prevResponseID  *string
	responseFormat  *ChatCompletionResponseFormat
	tools           []responses.ToolUnionParam
	rawTools        []*schema.ToolInfo
	toolChoice      *schema.ToolChoice
	builtinTools    []*schema.ToolInfo
	enableWebSearch bool
}

var _ model.ToolCallingChatModel = (*ResponsesAPIChatModel)(nil)

// NewResponsesAPIChatModel creates a new ResponsesAPIChatModel from the provided config.
func NewResponsesAPIChatModel(_ context.Context, config *ResponsesAPIConfig) (*ResponsesAPIChatModel, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if config.APIKey == "" {
		return nil, fmt.Errorf("APIKey is required")
	}
	if config.Model == "" {
		return nil, fmt.Errorf("Model is required")
	}

	opts := []option.RequestOption{
		option.WithAPIKey(config.APIKey),
	}
	if config.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(config.BaseURL))
	}

	var httpClient *http.Client
	if config.HTTPClient != nil {
		httpClient = config.HTTPClient
	} else if config.Timeout > 0 {
		httpClient = &http.Client{Timeout: config.Timeout}
	}
	if httpClient != nil {
		opts = append(opts, option.WithHTTPClient(httpClient))
	}

	cli := openaiSDK.NewClient(opts...)

	return &ResponsesAPIChatModel{
		cli:             &cli,
		model:           config.Model,
		maxTokens:       config.MaxOutputTokens,
		responseFormat:  config.ResponseFormat,
		builtinTools:    config.BuiltinTools,
		enableWebSearch: config.EnableWebSearch,
	}, nil
}

func (cm *ResponsesAPIChatModel) GetType() string {
	return "OpenAIResponsesAPI"
}

func (cm *ResponsesAPIChatModel) IsCallbacksEnabled() bool {
	return true
}

func (cm *ResponsesAPIChatModel) Generate(ctx context.Context, input []*schema.Message,
	opts ...model.Option,
) (outMsg *schema.Message, err error) {
	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)

	options, specOptions, err := cm.getOptions(opts)
	if err != nil {
		return nil, err
	}

	req, err := cm.genRequestAndOptions(input, options, specOptions)
	if err != nil {
		return nil, fmt.Errorf("build request failed: %w", err)
	}

	config := cm.toCallbackConfig(req)
	tools := cm.rawTools
	if options.Tools != nil {
		tools = options.Tools
	}

	ctx = callbacks.OnStart(ctx, &model.CallbackInput{
		Messages:   input,
		Tools:      tools,
		ToolChoice: options.ToolChoice,
		Config:     config,
	})

	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()
	resp, err := cm.cli.Responses.New(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("responses API call failed: %w", err)
	}

	outMsg, err = cm.toOutputMessage(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to convert response to schema.Message: %w", err)
	}

	callbacks.OnEnd(ctx, &model.CallbackOutput{
		Message:    outMsg,
		Config:     config,
		TokenUsage: cm.toModelTokenUsage(resp.Usage),
	})
	return outMsg, nil
}

func (cm *ResponsesAPIChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (outStream *schema.StreamReader[*schema.Message], err error) {
	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)

	options, specOptions, err := cm.getOptions(opts)
	if err != nil {
		return nil, err
	}

	req, err := cm.genRequestAndOptions(input, options, specOptions)
	if err != nil {
		return nil, fmt.Errorf("build request failed: %w", err)
	}

	config := cm.toCallbackConfig(req)
	tools := cm.rawTools
	if options.Tools != nil {
		tools = options.Tools
	}
	if options.AllowedToolNames != nil {
	}

	ctx = callbacks.OnStart(ctx, &model.CallbackInput{
		Messages:   input,
		Tools:      tools,
		ToolChoice: options.ToolChoice,
		Config:     config,
	})

	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	stream := cm.cli.Responses.NewStreaming(ctx, req)

	sr, sw := schema.Pipe[*model.CallbackOutput](1)

	go func() {
		defer func() {
			pe := recover()
			if pe != nil {
				_ = sw.Send(nil, newResponsesPanicErr(pe, debug.Stack()))
			}
			sw.Close()
		}()

		cm.receiveStreamResponse(stream, config, sw)
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

func (cm *ResponsesAPIChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	if len(tools) == 0 {
		return nil, errors.New("no tools to bind")
	}

	respTools, err := cm.toTools(tools)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to responses API tools: %w", err)
	}

	nCM := *cm
	nCM.rawTools = tools
	nCM.tools = respTools
	return &nCM, nil
}

func (cm *ResponsesAPIChatModel) getOptions(opts []model.Option) (*model.Options, *responsesAPIOptions, error) {
	options := model.GetCommonOptions(&model.Options{
		Temperature: cm.temperature,
		MaxTokens:   cm.maxTokens,
		Model:       &cm.model,
		TopP:        cm.topP,
		ToolChoice:  cm.toolChoice,
	}, opts...)

	specOptions := model.GetImplSpecificOptions(&responsesAPIOptions{
		EnableWebSearch: cm.enableWebSearch,
	}, opts...)

	return options, specOptions, nil
}

func (cm *ResponsesAPIChatModel) genRequestAndOptions(in []*schema.Message, options *model.Options,
	specOptions *responsesAPIOptions,
) (responses.ResponseNewParams, error) {
	req := responses.ResponseNewParams{}

	// Model
	if options.Model != nil {
		req.Model = shared.ResponsesModel(*options.Model)
	} else {
		req.Model = shared.ResponsesModel(cm.model)
	}

	// Sampling params
	if options.Temperature != nil {
		req.Temperature = param.NewOpt(float64(*options.Temperature))
	}
	if options.TopP != nil {
		req.TopP = param.NewOpt(float64(*options.TopP))
	}
	if options.MaxTokens != nil {
		req.MaxOutputTokens = param.NewOpt(int64(*options.MaxTokens))
	}

	// Store
	if cm.store != nil {
		req.Store = param.NewOpt(*cm.store)
	}

	// Response format
	if cm.responseFormat != nil {
		textCfg, err := cm.toResponseTextConfig(cm.responseFormat)
		if err != nil {
			return req, err
		}
		req.Text = textCfg
	}

	// Input messages
	inputItems, err := cm.buildInputItems(in)
	if err != nil {
		return req, err
	}
	req.Input = responses.ResponseNewParamsInputUnion{
		OfInputItemList: inputItems,
	}

	// Tools
	tools := cm.tools
	if options.Tools != nil {
		if tools, err = cm.toTools(options.Tools); err != nil {
			return req, err
		}
		req.Tools = append(req.Tools, tools...)
	}

	if specOptions.EnableWebSearch {
		req.Tools = append(req.Tools, responses.ToolUnionParam{
			OfWebSearch: &responses.WebSearchToolParam{
				Type: responses.WebSearchToolTypeWebSearch,
			},
		})
	}

	// Tool choice
	if options.ToolChoice != nil {
		tc, err := cm.toToolChoice(*options.ToolChoice, options.AllowedToolNames, tools)
		if err != nil {
			return req, err
		}
		req.ToolChoice = tc
	}
	return req, nil
}

func (cm *ResponsesAPIChatModel) buildInputItems(in []*schema.Message) (responses.ResponseInputParam, error) {
	items := make(responses.ResponseInputParam, 0, len(in))
	for _, msg := range in {
		switch msg.Role {
		case schema.User:
			item, err := cm.buildUserInputItem(msg)
			if err != nil {
				return nil, err
			}
			items = append(items, item)

		case schema.System:
			item, err := cm.buildSystemInputItem(msg)
			if err != nil {
				return nil, err
			}
			items = append(items, item)

		case schema.Assistant:
			msgItem, err := cm.buildAssistantInputItem(msg)
			if err != nil {
				return nil, err
			}
			if msgItem != nil {
				items = append(items, *msgItem)
			}
			// Tool calls from assistant become separate function_call items
			for _, tc := range msg.ToolCalls {
				items = append(items, responses.ResponseInputItemParamOfFunctionCall(
					tc.Function.Arguments,
					tc.ID,
					tc.Function.Name,
				))
			}

		case schema.Tool:
			items = append(items, responses.ResponseInputItemParamOfFunctionCallOutput(
				msg.ToolCallID,
				msg.Content,
			))

		default:
			return nil, fmt.Errorf("unsupported message role: %s", msg.Role)
		}
	}
	return items, nil
}

func (cm *ResponsesAPIChatModel) buildUserInputItem(msg *schema.Message) (responses.ResponseInputItemUnionParam, error) {
	content, err := cm.buildInputContent(msg)
	if err != nil {
		return responses.ResponseInputItemUnionParam{}, err
	}
	return responses.ResponseInputItemParamOfMessage(content, "user"), nil
}

func (cm *ResponsesAPIChatModel) buildSystemInputItem(msg *schema.Message) (responses.ResponseInputItemUnionParam, error) {
	content, err := cm.buildInputContent(msg)
	if err != nil {
		return responses.ResponseInputItemUnionParam{}, err
	}
	return responses.ResponseInputItemParamOfMessage(content, "system"), nil
}

func (cm *ResponsesAPIChatModel) buildAssistantInputItem(msg *schema.Message) (*responses.ResponseInputItemUnionParam, error) {
	// If only tool calls, no need for a message item
	if msg.Content == "" && len(msg.MultiContent) == 0 && len(msg.AssistantGenMultiContent) == 0 {
		return nil, nil
	}
	content, err := cm.buildAssistantContent(msg)
	if err != nil {
		return nil, err
	}
	item := responses.ResponseInputItemParamOfMessage(content, "assistant")
	return &item, nil
}

func (cm *ResponsesAPIChatModel) buildInputContent(msg *schema.Message) (responses.ResponseInputMessageContentListParam, error) {
	// UserInputMultiContent (new-style multi-modal)
	if len(msg.UserInputMultiContent) > 0 {
		return cm.buildFromUserInputMultiContent(msg.UserInputMultiContent)
	}
	// MultiContent (legacy multi-modal)
	if len(msg.MultiContent) > 0 {
		return cm.buildFromMultiContent(msg.MultiContent)
	}
	// Plain text
	if msg.Content != "" {
		return responses.ResponseInputMessageContentListParam{
			{OfInputText: &responses.ResponseInputTextParam{Text: msg.Content}},
		}, nil
	}
	return nil, fmt.Errorf("message content is empty for role %s", msg.Role)
}

func (cm *ResponsesAPIChatModel) buildAssistantContent(msg *schema.Message) (responses.ResponseInputMessageContentListParam, error) {
	if len(msg.AssistantGenMultiContent) > 0 {
		parts := make(responses.ResponseInputMessageContentListParam, 0, len(msg.AssistantGenMultiContent))
		for _, part := range msg.AssistantGenMultiContent {
			if part.Type != schema.ChatMessagePartTypeText {
				return nil, fmt.Errorf("unsupported content type in AssistantGenMultiContent: %s", part.Type)
			}
			parts = append(parts, responses.ResponseInputContentUnionParam{
				OfInputText: &responses.ResponseInputTextParam{Text: part.Text, Type: "output_text"},
			})
		}
		return parts, nil
	}
	if msg.Content != "" {
		return responses.ResponseInputMessageContentListParam{
			{OfInputText: &responses.ResponseInputTextParam{Text: msg.Content, Type: "output_text"}},
		}, nil
	}
	return nil, nil
}

func (cm *ResponsesAPIChatModel) buildFromUserInputMultiContent(parts []schema.MessageInputPart) (responses.ResponseInputMessageContentListParam, error) {
	result := make(responses.ResponseInputMessageContentListParam, 0, len(parts))
	for _, part := range parts {
		switch part.Type {
		case schema.ChatMessagePartTypeText:
			result = append(result, responses.ResponseInputContentUnionParam{
				OfInputText: &responses.ResponseInputTextParam{Text: part.Text},
			})
		case schema.ChatMessagePartTypeImageURL:
			if part.Image == nil {
				return nil, fmt.Errorf("image field must not be nil when type is image_url")
			}
			imageURL, err := messagePartCommonToURL(part.Image.MessagePartCommon)
			if err != nil {
				return nil, fmt.Errorf("convert image failed: %w", err)
			}
			imgParam := &responses.ResponseInputImageParam{
				Detail:   toResponsesImageDetail(part.Image.Detail),
				ImageURL: param.NewOpt(imageURL),
			}
			result = append(result, responses.ResponseInputContentUnionParam{OfInputImage: imgParam})
		case schema.ChatMessagePartTypeFileURL:
			if part.File == nil {
				return nil, fmt.Errorf("file field must not be nil when type is file_url")
			}
			fileParam, err := cm.buildFileParam(part.File)
			if err != nil {
				return nil, err
			}
			result = append(result, responses.ResponseInputContentUnionParam{OfInputFile: fileParam})
		default:
			return nil, fmt.Errorf("unsupported content type in UserInputMultiContent: %s", part.Type)
		}
	}
	return result, nil
}

func (cm *ResponsesAPIChatModel) buildFromMultiContent(parts []schema.ChatMessagePart) (responses.ResponseInputMessageContentListParam, error) {
	result := make(responses.ResponseInputMessageContentListParam, 0, len(parts))
	for _, part := range parts {
		switch part.Type {
		case schema.ChatMessagePartTypeText:
			result = append(result, responses.ResponseInputContentUnionParam{
				OfInputText: &responses.ResponseInputTextParam{Text: part.Text},
			})
		case schema.ChatMessagePartTypeImageURL:
			if part.ImageURL == nil {
				continue
			}
			imgParam := &responses.ResponseInputImageParam{
				Detail:   toResponsesImageDetail(schema.ImageURLDetail(part.ImageURL.Detail)),
				ImageURL: param.NewOpt(part.ImageURL.URL),
			}
			result = append(result, responses.ResponseInputContentUnionParam{OfInputImage: imgParam})
		case schema.ChatMessagePartTypeFileURL:
			if part.FileURL == nil {
				continue
			}
			fileParam := cm.buildFileParamFromURL(part.FileURL)
			result = append(result, responses.ResponseInputContentUnionParam{OfInputFile: fileParam})
		default:
			return nil, fmt.Errorf("unsupported content type: %s", part.Type)
		}
	}
	return result, nil
}

func (cm *ResponsesAPIChatModel) buildFileParam(file *schema.MessageInputFile) (*responses.ResponseInputFileParam, error) {
	fp := &responses.ResponseInputFileParam{}
	if file.Name != "" {
		fp.Filename = param.NewOpt(file.Name)
	}
	if file.URL != nil {
		fp.FileURL = param.NewOpt(*file.URL)
	} else if file.Base64Data != nil {
		if file.MIMEType == "" {
			return nil, fmt.Errorf("MIMEType is required when using Base64Data for file")
		}
		dataURL := fmt.Sprintf("data:%s;base64,%s", file.MIMEType, *file.Base64Data)
		fp.FileData = param.NewOpt(dataURL)
	} else {
		return nil, fmt.Errorf("file must have URL or Base64Data")
	}
	return fp, nil
}

func (cm *ResponsesAPIChatModel) buildFileParamFromURL(file *schema.ChatMessageFileURL) *responses.ResponseInputFileParam {
	fp := &responses.ResponseInputFileParam{}
	if file.Name != "" {
		fp.Filename = param.NewOpt(file.Name)
	}
	if strings.HasPrefix(file.URL, "data:") {
		fp.FileData = param.NewOpt(file.URL)
	} else {
		fp.FileURL = param.NewOpt(file.URL)
	}
	return fp
}

func messagePartCommonToURL(common schema.MessagePartCommon) (string, error) {
	if common.URL != nil {
		return *common.URL, nil
	}
	if common.Base64Data != nil {
		if common.MIMEType == "" {
			return "", fmt.Errorf("MIMEType is required when using Base64Data")
		}
		return fmt.Sprintf("data:%s;base64,%s", common.MIMEType, *common.Base64Data), nil
	}
	return "", fmt.Errorf("message part must have URL or Base64Data")
}

func toResponsesImageDetail(detail schema.ImageURLDetail) responses.ResponseInputImageDetail {
	switch detail {
	case schema.ImageURLDetailHigh:
		return responses.ResponseInputImageDetailHigh
	case schema.ImageURLDetailLow:
		return responses.ResponseInputImageDetailLow
	default:
		return responses.ResponseInputImageDetailAuto
	}
}

func (cm *ResponsesAPIChatModel) toTools(tis []*schema.ToolInfo) ([]responses.ToolUnionParam, error) {
	tools := make([]responses.ToolUnionParam, 0, len(tis))
	for _, ti := range tis {
		if ti == nil {
			return nil, fmt.Errorf("tool info cannot be nil")
		}
		paramsSchema, err := ti.ParamsOneOf.ToJSONSchema()
		if err != nil {
			return nil, fmt.Errorf("failed to convert tool parameters to JSONSchema: %w", err)
		}

		var paramsMap map[string]any
		b, err := json.Marshal(paramsSchema)
		if err != nil {
			return nil, fmt.Errorf("marshal tool params failed: %w", err)
		}
		if err = json.Unmarshal(b, &paramsMap); err != nil {
			return nil, fmt.Errorf("unmarshal tool params failed: %w", err)
		}

		tools = append(tools, responses.ToolUnionParam{
			OfFunction: &responses.FunctionToolParam{
				Name:        ti.Name,
				Description: param.NewOpt(ti.Desc),
				Parameters:  paramsMap,
				Strict:      param.NewOpt(false),
			},
		})
	}
	return tools, nil
}

func (cm *ResponsesAPIChatModel) toToolChoice(tc schema.ToolChoice, allowedNames []string, tools []responses.ToolUnionParam) (responses.ResponseNewParamsToolChoiceUnion, error) {
	switch tc {
	case schema.ToolChoiceForbidden:
		return responses.ResponseNewParamsToolChoiceUnion{
			OfToolChoiceMode: param.NewOpt(responses.ToolChoiceOptionsNone),
		}, nil
	case schema.ToolChoiceAllowed:
		return responses.ResponseNewParamsToolChoiceUnion{
			OfToolChoiceMode: param.NewOpt(responses.ToolChoiceOptionsAuto),
		}, nil
	case schema.ToolChoiceForced:
		if len(tools) == 0 {
			return responses.ResponseNewParamsToolChoiceUnion{}, fmt.Errorf("tool_choice is forced but no tools are provided")
		}

		var forcedName string
		if len(allowedNames) == 1 {
			forcedName = allowedNames[0]
		} else if len(allowedNames) > 1 {
			return responses.ResponseNewParamsToolChoiceUnion{}, fmt.Errorf("only one allowed tool name can be configured")
		} else if len(tools) == 1 && tools[0].OfFunction != nil {
			forcedName = tools[0].OfFunction.Name
		}

		if forcedName != "" {
			return responses.ResponseNewParamsToolChoiceUnion{
				OfFunctionTool: &responses.ToolChoiceFunctionParam{
					Name: forcedName,
				},
			}, nil
		}
		return responses.ResponseNewParamsToolChoiceUnion{
			OfToolChoiceMode: param.NewOpt(responses.ToolChoiceOptionsRequired),
		}, nil
	default:
		return responses.ResponseNewParamsToolChoiceUnion{
			OfToolChoiceMode: param.NewOpt(responses.ToolChoiceOptionsAuto),
		}, nil
	}
}

func (cm *ResponsesAPIChatModel) toResponseTextConfig(rf *ChatCompletionResponseFormat) (responses.ResponseTextConfigParam, error) {
	if rf == nil {
		return responses.ResponseTextConfigParam{}, nil
	}
	switch rf.Type {
	case ChatCompletionResponseFormatTypeText:
		return responses.ResponseTextConfigParam{
			Format: responses.ResponseFormatTextConfigUnionParam{
				OfText: &responses.ResponseFormatTextParam{},
			},
		}, nil
	case ChatCompletionResponseFormatTypeJSONObject:
		return responses.ResponseTextConfigParam{
			Format: responses.ResponseFormatTextConfigUnionParam{
				OfJSONObject: &responses.ResponseFormatJSONObjectParam{},
			},
		}, nil
	case ChatCompletionResponseFormatTypeJSONSchema:
		if rf.JSONSchema == nil {
			return responses.ResponseTextConfigParam{}, fmt.Errorf("JSONSchema is required when ResponseFormat type is json_schema")
		}
		schemaBytes, err := json.Marshal(rf.JSONSchema.JSONSchema)
		if err != nil {
			return responses.ResponseTextConfigParam{}, fmt.Errorf("marshal JSONSchema failed: %w", err)
		}
		var schemaMap map[string]any
		if err = json.Unmarshal(schemaBytes, &schemaMap); err != nil {
			return responses.ResponseTextConfigParam{}, fmt.Errorf("unmarshal JSONSchema failed: %w", err)
		}
		return responses.ResponseTextConfigParam{
			Format: responses.ResponseFormatTextConfigUnionParam{
				OfJSONSchema: &responses.ResponseFormatTextJSONSchemaConfigParam{
					Name:        rf.JSONSchema.Name,
					Description: param.NewOpt(rf.JSONSchema.Description),
					Schema:      schemaMap,
					Strict:      param.NewOpt(rf.JSONSchema.Strict),
				},
			},
		}, nil
	default:
		return responses.ResponseTextConfigParam{}, fmt.Errorf("unsupported response format type: %s", rf.Type)
	}
}

func (cm *ResponsesAPIChatModel) toOutputMessage(resp *responses.Response) (*schema.Message, error) {
	msg := &schema.Message{
		Role: schema.Assistant,
		ResponseMeta: &schema.ResponseMeta{
			FinishReason: string(resp.Status),
			Usage:        cm.toEinoTokenUsage(resp.Usage),
		},
	}

	if resp.Status == responses.ResponseStatusFailed {
		msg.ResponseMeta.FinishReason = resp.Error.Message
		return msg, nil
	}
	if resp.Status == responses.ResponseStatusIncomplete {
		msg.ResponseMeta.FinishReason = resp.IncompleteDetails.Reason
		return msg, nil
	}

	if len(resp.Output) == 0 {
		return nil, fmt.Errorf("received empty output from OpenAI Responses API")
	}

	for _, item := range resp.Output {
		switch item.Type {
		case "message":
			outputMsg := item.AsMessage()
			isMulti := len(outputMsg.Content) > 1
			for _, contentPart := range outputMsg.Content {
				if contentPart.Type != "output_text" {
					continue
				}
				text := contentPart.AsOutputText().Text
				if !isMulti {
					msg.Content = text
				} else {
					msg.AssistantGenMultiContent = append(msg.AssistantGenMultiContent, schema.MessageOutputPart{
						Type: schema.ChatMessagePartTypeText,
						Text: text,
					})
				}
			}
		case "reasoning":
			reasoningItem := item.AsReasoning()
			for _, summary := range reasoningItem.Summary {
				if summary.Text == "" {
					continue
				}
				if msg.ReasoningContent == "" {
					msg.ReasoningContent = summary.Text
				} else {
					msg.ReasoningContent = fmt.Sprintf("%s\n\n%s", msg.ReasoningContent, summary.Text)
				}
			}
		case "function_call":
			fc := item.AsFunctionCall()
			msg.ToolCalls = append(msg.ToolCalls, schema.ToolCall{
				ID:   fc.CallID,
				Type: string(fc.Type),
				Function: schema.FunctionCall{
					Name:      fc.Name,
					Arguments: fc.Arguments,
				},
			})
		}
	}

	return msg, nil
}

func (cm *ResponsesAPIChatModel) toEinoTokenUsage(usage responses.ResponseUsage) *schema.TokenUsage {
	tu := &schema.TokenUsage{
		PromptTokens:     int(usage.InputTokens),
		CompletionTokens: int(usage.OutputTokens),
		TotalTokens:      int(usage.TotalTokens),
	}
	tu.PromptTokenDetails.CachedTokens = int(usage.InputTokensDetails.CachedTokens)
	tu.CompletionTokensDetails.ReasoningTokens = int(usage.OutputTokensDetails.ReasoningTokens)
	return tu
}

func (cm *ResponsesAPIChatModel) toModelTokenUsage(usage responses.ResponseUsage) *model.TokenUsage {
	tu := &model.TokenUsage{
		PromptTokens:     int(usage.InputTokens),
		CompletionTokens: int(usage.OutputTokens),
		TotalTokens:      int(usage.TotalTokens),
	}
	tu.PromptTokenDetails.CachedTokens = int(usage.InputTokensDetails.CachedTokens)
	tu.CompletionTokensDetails.ReasoningTokens = int(usage.OutputTokensDetails.ReasoningTokens)
	return tu
}

func (cm *ResponsesAPIChatModel) toCallbackConfig(req responses.ResponseNewParams) *model.Config {
	cfg := &model.Config{
		Model: string(req.Model),
	}
	if req.Temperature.Valid() {
		cfg.Temperature = float32(req.Temperature.Value)
	}
	if req.TopP.Valid() {
		cfg.TopP = float32(req.TopP.Value)
	}
	if req.MaxOutputTokens.Valid() {
		cfg.MaxTokens = int(req.MaxOutputTokens.Value)
	}
	return cfg
}

func (cm *ResponsesAPIChatModel) receiveStreamResponse(
	stream *ssestream.Stream[responses.ResponseStreamEventUnion],
	config *model.Config,
	sw *schema.StreamWriter[*model.CallbackOutput],
) {
	// Track current function call being built (for streaming tool calls)
	type pendingFuncCall struct {
		index     int
		callID    string
		callType  string
		name      string
		arguments strings.Builder
	}
	funcCalls := map[string]*pendingFuncCall{}

	for stream.Next() {
		event := stream.Current()

		switch event.Type {
		case "response.created", "response.in_progress":
			// nothing to emit

		case "response.output_item.added":
			addedEvt := event.AsResponseOutputItemAdded()
			item := addedEvt.Item
			if item.Type == "function_call" {
				fc := item.AsFunctionCall()
				funcCalls[fc.ID] = &pendingFuncCall{
					index:    int(addedEvt.OutputIndex),
					callID:   fc.CallID,
					callType: string(fc.Type),
					name:     fc.Name,
				}
			}

		case "response.function_call_arguments.delta":
			deltaEvt := event.AsResponseFunctionCallArgumentsDelta()
			if pfc, ok := funcCalls[deltaEvt.ItemID]; ok {
				pfc.arguments.WriteString(deltaEvt.Delta)
				idx := pfc.index
				sw.Send(&model.CallbackOutput{
					Message: &schema.Message{
						Role: schema.Assistant,
						ToolCalls: []schema.ToolCall{
							{
								Index: &idx,
								ID:    pfc.callID,
								Type:  pfc.callType,
								Function: schema.FunctionCall{
									Name:      pfc.name,
									Arguments: deltaEvt.Delta,
								},
							},
						},
					},
					Config: config,
				}, nil)
			}

		case "response.output_text.delta":
			textEvt := event.AsResponseOutputTextDelta()
			sw.Send(&model.CallbackOutput{
				Message: &schema.Message{
					Role:    schema.Assistant,
					Content: textEvt.Delta,
				},
				Config: config,
			}, nil)

		case "response.reasoning_text.delta":
			reasonEvt := event.AsResponseReasoningTextDelta()
			sw.Send(&model.CallbackOutput{
				Message: &schema.Message{
					Role:             schema.Assistant,
					ReasoningContent: reasonEvt.Delta,
				},
				Config: config,
			}, nil)

		case "response.completed":
			completedEvt := event.AsResponseCompleted()
			resp := completedEvt.Response
			sw.Send(&model.CallbackOutput{
				Message: &schema.Message{
					Role: schema.Assistant,
					ResponseMeta: &schema.ResponseMeta{
						FinishReason: string(resp.Status),
						Usage:        cm.toEinoTokenUsage(resp.Usage),
					},
				},
				Config:     config,
				TokenUsage: cm.toModelTokenUsage(resp.Usage),
			}, nil)

		case "response.failed":
			failedEvt := event.AsResponseFailed()
			resp := failedEvt.Response
			var errMsg string
			if resp.Error.Message != "" {
				errMsg = resp.Error.Message
			} else {
				errMsg = string(resp.Status)
			}
			sw.Send(&model.CallbackOutput{
				Message: &schema.Message{
					Role: schema.Assistant,
					ResponseMeta: &schema.ResponseMeta{
						FinishReason: errMsg,
						Usage:        cm.toEinoTokenUsage(resp.Usage),
					},
				},
				Config: config,
			}, nil)

		case "response.incomplete":
			incompleteEvt := event.AsResponseIncomplete()
			resp := incompleteEvt.Response
			sw.Send(&model.CallbackOutput{
				Message: &schema.Message{
					Role: schema.Assistant,
					ResponseMeta: &schema.ResponseMeta{
						FinishReason: resp.IncompleteDetails.Reason,
						Usage:        cm.toEinoTokenUsage(resp.Usage),
					},
				},
				Config: config,
			}, nil)

		case "error":
			errEvt := event.AsError()
			sw.Send(nil, fmt.Errorf("received error from responses API: %s", errEvt.Message))
			return
		}
	}

	if err := stream.Err(); err != nil && !errors.Is(err, io.EOF) {
		sw.Send(nil, fmt.Errorf("stream read error: %w", err))
	}
}

type responsesPanicErr struct {
	info  any
	stack []byte
}

func (p *responsesPanicErr) Error() string {
	return fmt.Sprintf("panic error: %v, \nstack: %s", p.info, string(p.stack))
}

func newResponsesPanicErr(info any, stack []byte) error {
	return &responsesPanicErr{info: info, stack: stack}
}

/*
 * Copyright 2026 CloudWeGo Authors
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
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"
	"sort"
	"strings"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"
)

const (
	responsesModelType       = "OpenAIResponses"
	responsesRequestIDHeader = "X-Request-Id"

	// ResponsesExtraKeyResponseID identifies the server-side response in Message.Extra.
	ResponsesExtraKeyResponseID = "openai_response_id"
	// ResponsesExtraKeyRequestID records the HTTP request ID when provided.
	ResponsesExtraKeyRequestID = "openai_request_id"
)

// ResponsesReasoningEffort is the SDK reasoning effort for the Responses API.
type ResponsesReasoningEffort = shared.ReasoningEffort

const (
	ResponsesReasoningEffortNone    ResponsesReasoningEffort = shared.ReasoningEffortNone
	ResponsesReasoningEffortMinimal ResponsesReasoningEffort = shared.ReasoningEffortMinimal
	ResponsesReasoningEffortLow     ResponsesReasoningEffort = shared.ReasoningEffortLow
	ResponsesReasoningEffortMedium  ResponsesReasoningEffort = shared.ReasoningEffortMedium
	ResponsesReasoningEffortHigh    ResponsesReasoningEffort = shared.ReasoningEffortHigh
	ResponsesReasoningEffortXhigh   ResponsesReasoningEffort = shared.ReasoningEffortXhigh
)

// ResponsesPromptCacheRetention is the SDK prompt cache retention policy.
type ResponsesPromptCacheRetention = responses.ResponseNewParamsPromptCacheRetention

const (
	ResponsesPromptCacheRetentionInMemory ResponsesPromptCacheRetention = "in_memory"
	ResponsesPromptCacheRetention24h      ResponsesPromptCacheRetention = responses.ResponseNewParamsPromptCacheRetention24h
)

// ResponsesChatModelConfig configures the Responses API for schema.Message callers.
// Fields left unset use the provider defaults. SDK retries are disabled so the
// caller or ADK retry policy owns retry decisions.
type ResponsesChatModelConfig struct {
	// BaseURL defaults to https://api.openai.com/v1.
	BaseURL string
	APIKey  string
	Model   string
	// MaxOutputTokens includes both visible output and internal reasoning tokens.
	MaxOutputTokens      *int64
	Temperature          *float32
	TopP                 *float32
	ReasoningEffort      ResponsesReasoningEffort
	Store                *bool
	PromptCacheRetention ResponsesPromptCacheRetention
	HTTPClient           *http.Client
	// StreamEventHandler optionally inspects stream events before standard parsing.
	// It can translate provider extensions into immediate or incomplete-stream errors.
	StreamEventHandler ResponsesStreamEventHandler
}

// ResponsesStreamEventHandler inspects every SSE event, including provider-specific
// events whose additional fields are accessible through event.RawJSON(). It runs
// synchronously for each stream and may be called concurrently by different streams.
// Returning a zero result leaves standard event processing unchanged.
type ResponsesStreamEventHandler func(context.Context, responses.ResponseStreamEventUnion) ResponsesStreamEventResult

// ResponsesStreamEventResult controls how a stream event affects error handling.
type ResponsesStreamEventResult struct {
	// Error stops the stream immediately and is returned unchanged to the reader.
	Error error
	// IncompleteError is retained for a clean EOF without a terminal response.
	// A terminal response or an SDK/transport error takes precedence. The latest
	// non-nil value wins, and the retained value is isolated to this stream.
	IncompleteError error
}

type responseAPIOptions struct {
	ReasoningEffort      ResponsesReasoningEffort
	Store                *bool
	PromptCacheKey       *string
	PromptCacheRetention ResponsesPromptCacheRetention
	UseResponseIDFeature bool
}

// WithResponsesReasoningEffort overrides reasoning effort for a Responses call.
func WithResponsesReasoningEffort(effort ResponsesReasoningEffort) model.Option {
	return model.WrapImplSpecificOptFn(func(o *responseAPIOptions) {
		o.ReasoningEffort = effort
	})
}

// WithResponsesStore overrides server-side response storage.
func WithResponsesStore(store bool) model.Option {
	return model.WrapImplSpecificOptFn(func(o *responseAPIOptions) {
		o.Store = &store
	})
}

// WithResponsesPromptCacheKey sets the cache grouping key for a Responses call.
func WithResponsesPromptCacheKey(key string) model.Option {
	return model.WrapImplSpecificOptFn(func(o *responseAPIOptions) {
		o.PromptCacheKey = &key
	})
}

// WithResponsesPromptCacheRetention sets the prompt cache retention policy.
func WithResponsesPromptCacheRetention(retention ResponsesPromptCacheRetention) model.Option {
	return model.WrapImplSpecificOptFn(func(o *responseAPIOptions) {
		o.PromptCacheRetention = retention
	})
}

// WithResponsesUseResponseID enables continuation from the most recent assistant
// response ID instead of resending the complete history. The provider must have
// stored that response. This is disabled by default.
func WithResponsesUseResponseID(useResponseIDFeature bool) model.Option {
	return model.WrapImplSpecificOptFn(func(o *responseAPIOptions) {
		o.UseResponseIDFeature = useResponseIDFeature
	})
}

var _ model.ToolCallingChatModel = (*ResponsesChatModel)(nil)

// ResponsesChatModel implements the Responses API using schema.Message.
// NewChatModel continues to use Chat Completions.
type ResponsesChatModel struct {
	client     *openai.Client
	config     *ResponsesChatModelConfig
	tools      []responses.ToolUnionParam
	rawTools   []*schema.ToolInfo
	toolChoice *schema.ToolChoice
}

// NewResponsesChatModel creates a ToolCallingChatModel backed by /responses.
func NewResponsesChatModel(ctx context.Context, config *ResponsesChatModelConfig) (*ResponsesChatModel, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	opts := []option.RequestOption{
		option.WithAPIKey(config.APIKey),
		option.WithMaxRetries(0),
	}
	if config.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(config.BaseURL))
	}
	if config.HTTPClient != nil {
		opts = append(opts, option.WithHTTPClient(config.HTTPClient))
	}

	client := openai.NewClient(opts...)

	return &ResponsesChatModel{
		client: &client,
		config: config,
	}, nil
}

func (m *ResponsesChatModel) GetType() string {
	return responsesModelType
}

func (m *ResponsesChatModel) IsCallbacksEnabled() bool {
	return true
}

// region Generate

func (m *ResponsesChatModel) Generate(ctx context.Context, in []*schema.Message, opts ...model.Option) (
	outMsg *schema.Message, err error) {

	ctx = callbacks.EnsureRunInfo(ctx, m.GetType(), components.ComponentOfChatModel)

	params, cbInput, err := m.buildRequest(in, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}

	ctx = callbacks.OnStart(ctx, cbInput)
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	var rawResponse *http.Response
	resp, err := m.client.Responses.New(ctx, *params, option.WithResponseInto(&rawResponse))
	if err != nil {
		return nil, fmt.Errorf("failed to create response: %w", err)
	}

	outMsg, err = m.convertResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to convert response: %w", err)
	}
	if err = saveReasoning(outMsg, resp.Output); err != nil {
		return nil, err
	}

	traceID := responseTraceID(rawResponse)
	setResponseTraceID(outMsg, traceID)
	callbacks.OnEnd(ctx, newCallbackOutput(outMsg, cbInput.Config, traceID))

	return outMsg, nil
}

// endregion

// region Stream

func (m *ResponsesChatModel) Stream(ctx context.Context, in []*schema.Message, opts ...model.Option) (
	outStream *schema.StreamReader[*schema.Message], err error) {

	ctx = callbacks.EnsureRunInfo(ctx, m.GetType(), components.ComponentOfChatModel)

	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	params, cbInput, err := m.buildRequest(in, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}

	ctx = callbacks.OnStart(ctx, cbInput)

	var rawResponse *http.Response
	stream := m.client.Responses.NewStreaming(ctx, *params, option.WithResponseInto(&rawResponse))
	traceID := responseTraceID(rawResponse)

	sr, sw := schema.Pipe[*model.CallbackOutput](1)

	go func(ctx_ context.Context) {
		defer func() {
			if panicErr := recover(); panicErr != nil {
				_ = sw.Send(nil, fmt.Errorf("panic: %v\nstack: %s", panicErr, debug.Stack()))
			}
			_ = stream.Close()
			sw.Close()
		}()

		builder := newStreamBuilder()
		var incompleteErr error

		for stream.Next() {
			event := stream.Current()
			if handler := m.config.StreamEventHandler; handler != nil {
				result := handler(ctx_, event)
				if result.Error != nil {
					_ = sw.Send(nil, result.Error)
					return
				}
				if result.IncompleteError != nil {
					incompleteErr = result.IncompleteError
				}
			}
			msg, found, eventErr := builder.processEvent(ctx_, event)
			if eventErr != nil {
				_ = sw.Send(nil, eventErr)
				return
			}
			if !found {
				continue
			}

			closed := sw.Send(newCallbackOutput(msg, cbInput.Config, ""), nil)

			if closed {
				return
			}
		}

		if stream.Err() != nil {
			_ = sw.Send(nil, fmt.Errorf("stream error: %w, trace_id=%s", stream.Err(), traceID))
			return
		}
		if builder.finalResponse == nil && incompleteErr != nil {
			_ = sw.Send(nil, incompleteErr)
			return
		}

		finalMsg, finishErr := builder.finish(traceID)
		if finishErr != nil {
			_ = sw.Send(nil, finishErr)
			return
		}
		output := builder.finalResponse.Output
		if len(output) == 0 {
			output = builder.reasoningItems
		}
		if reasoningErr := saveReasoning(finalMsg, output); reasoningErr != nil {
			_ = sw.Send(nil, reasoningErr)
			return
		}

		setResponseTraceID(finalMsg, traceID)
		sw.Send(newCallbackOutput(finalMsg, cbInput.Config, traceID), nil)
	}(ctx)

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

// endregion

// region WithTools

func (m *ResponsesChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	if len(tools) == 0 {
		return nil, fmt.Errorf("no tools to bind")
	}

	responsesTools, err := toResponseTools(tools)
	if err != nil {
		return nil, fmt.Errorf("failed to convert tools: %w", err)
	}

	tc := schema.ToolChoiceAllowed
	nm := *m
	nm.tools = responsesTools
	nm.rawTools = tools
	nm.toolChoice = &tc
	return &nm, nil
}

func (m *ResponsesChatModel) BindTools(tools []*schema.ToolInfo) error {
	if len(tools) == 0 {
		return fmt.Errorf("no tools to bind")
	}

	responsesTools, err := toResponseTools(tools)
	if err != nil {
		return fmt.Errorf("failed to convert tools: %w", err)
	}

	tc := schema.ToolChoiceAllowed
	m.tools = responsesTools
	m.rawTools = tools
	m.toolChoice = &tc
	return nil
}

// endregion

// region Request Building

func (m *ResponsesChatModel) buildRequest(in []*schema.Message, opts ...model.Option) (
	*responses.ResponseNewParams, *model.CallbackInput, error) {

	options := model.GetCommonOptions(&model.Options{
		Temperature: m.config.Temperature,
		MaxTokens:   intPtrToInt(m.config.MaxOutputTokens),
		Model:       &m.config.Model,
		TopP:        m.config.TopP,
		Tools:       nil,
		ToolChoice:  m.toolChoice,
	}, opts...)

	specOptions := model.GetImplSpecificOptions(&responseAPIOptions{
		ReasoningEffort:      m.config.ReasoningEffort,
		Store:                m.config.Store,
		PromptCacheRetention: m.config.PromptCacheRetention,
	}, opts...)

	params := &responses.ResponseNewParams{
		Model:   *options.Model,
		Include: []responses.ResponseIncludable{"reasoning.encrypted_content"},
	}

	if options.Temperature != nil {
		params.Temperature = openai.Float(float64(*options.Temperature))
	}
	if options.TopP != nil {
		params.TopP = openai.Float(float64(*options.TopP))
	}
	if options.MaxTokens != nil {
		params.MaxOutputTokens = openai.Int(int64(*options.MaxTokens))
	}
	if specOptions.ReasoningEffort != "" {
		params.Reasoning = shared.ReasoningParam{
			Effort: specOptions.ReasoningEffort,
			// Summary: openai.ReasoningSummaryAuto,
		}
	}
	if specOptions.Store != nil {
		params.Store = openai.Bool(*specOptions.Store)
	}
	if specOptions.PromptCacheKey != nil && *specOptions.PromptCacheKey != "" {
		params.PromptCacheKey = openai.String(*specOptions.PromptCacheKey)
	}
	if specOptions.PromptCacheRetention != "" {
		params.PromptCacheRetention = specOptions.PromptCacheRetention
	}

	inputMessages := in
	for i := len(in) - 1; i >= 0; i-- {
		if in[i] == nil || in[i].Role != schema.Assistant {
			continue
		}
		if id, _ := in[i].Extra[ResponsesExtraKeyResponseID].(string); specOptions.UseResponseIDFeature && id != "" {
			params.PreviousResponseID = openai.String(id)
			inputMessages = in[i+1:]
			for _, msg := range in {
				if msg != nil && msg.Role == schema.System && msg.Content != "" {
					params.Instructions = openai.String(msg.Content)
					break
				}
			}
		}
		break
	}

	inputItems, err := messagesToInputItems(inputMessages)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to convert messages: %w", err)
	}
	params.Input = responses.ResponseNewParamsInputUnion{
		OfInputItemList: inputItems,
	}

	tools := m.tools
	cbTools := m.rawTools
	if options.Tools != nil {
		tools, err = toResponseTools(options.Tools)
		if err != nil {
			return nil, nil, err
		}
		cbTools = options.Tools
	}
	if len(tools) > 0 {
		params.Tools = tools
	}

	if err := populateToolChoice(params, options.ToolChoice, options.AllowedToolNames); err != nil {
		return nil, nil, err
	}

	cbInput := &model.CallbackInput{
		Messages:   in,
		Tools:      cbTools,
		ToolChoice: options.ToolChoice,
		Config: &model.Config{
			Model:       *options.Model,
			MaxTokens:   derefOrZero(options.MaxTokens),
			Temperature: derefOrZero(options.Temperature),
			TopP:        derefOrZero(options.TopP),
		},
	}

	return params, cbInput, nil
}

// endregion

// region Message Conversion

func messagesToInputItems(msgs []*schema.Message) (responses.ResponseInputParam, error) {
	var items responses.ResponseInputParam

	for i, msg := range msgs {
		if msg == nil {
			return nil, fmt.Errorf("message at index %d is nil", i)
		}
		converted, err := messageToInputItems(msg)
		if err != nil {
			return nil, err
		}
		items = append(items, converted...)
	}

	return items, nil
}

func messageToInputItems(msg *schema.Message) ([]responses.ResponseInputItemUnionParam, error) {
	var items []responses.ResponseInputItemUnionParam

	switch msg.Role {
	case schema.System:
		items = append(items,
			responses.ResponseInputItemParamOfMessage(msg.Content, responses.EasyInputMessageRoleSystem))

	case schema.User:
		if len(msg.UserInputMultiContent) > 0 {
			contentList, err := userMultiContentToInputContent(msg.UserInputMultiContent)
			if err != nil {
				return nil, err
			}
			items = append(items,
				responses.ResponseInputItemParamOfMessage(contentList, responses.EasyInputMessageRoleUser))
		} else if len(msg.MultiContent) > 0 {
			contentList, err := multiContentToInputContent(msg.MultiContent)
			if err != nil {
				return nil, err
			}
			items = append(items,
				responses.ResponseInputItemParamOfMessage(contentList, responses.EasyInputMessageRoleUser))
		} else {
			items = append(items,
				responses.ResponseInputItemParamOfMessage(msg.Content, responses.EasyInputMessageRoleUser))
		}

	case schema.Assistant:
		reasoning, err := reasoningInput(msg)
		if err != nil {
			return nil, err
		}
		items = append(items, reasoning...)
		if msg.Content != "" {
			items = append(items,
				responses.ResponseInputItemParamOfMessage(msg.Content, responses.EasyInputMessageRoleAssistant))
		}
		for _, tc := range msg.ToolCalls {
			items = append(items,
				responses.ResponseInputItemParamOfFunctionCall(tc.Function.Arguments, tc.ID, tc.Function.Name))
		}

	case schema.Tool:
		items = append(items,
			responses.ResponseInputItemParamOfFunctionCallOutput(msg.ToolCallID, msg.Content))

	default:
		items = append(items,
			responses.ResponseInputItemParamOfMessage(msg.Content, responses.EasyInputMessageRole(msg.Role)))
	}

	return items, nil
}

func userMultiContentToInputContent(parts []schema.MessageInputPart) (responses.ResponseInputMessageContentListParam, error) {
	var contentList responses.ResponseInputMessageContentListParam

	for _, part := range parts {
		switch part.Type {
		case schema.ChatMessagePartTypeText:
			contentList = append(contentList, responses.ResponseInputContentUnionParam{
				OfInputText: &responses.ResponseInputTextParam{
					Text: part.Text,
				},
			})
		case schema.ChatMessagePartTypeImageURL:
			if part.Image == nil {
				return nil, fmt.Errorf("image field must not be nil for image_url part type")
			}
			var url string
			if part.Image.URL != nil {
				url = *part.Image.URL
			} else if part.Image.Base64Data != nil {
				if part.Image.MIMEType == "" {
					return nil, fmt.Errorf("mimetype is required when using base64data")
				}
				url = fmt.Sprintf("data:%s;base64,%s", part.Image.MIMEType, *part.Image.Base64Data)
			} else {
				return nil, fmt.Errorf("image message part must have url or base64 data")
			}
			detail := responses.ResponseInputImageDetailAuto
			if part.Image.Detail != "" {
				detail = responses.ResponseInputImageDetail(part.Image.Detail)
			}
			contentList = append(contentList, responses.ResponseInputContentUnionParam{
				OfInputImage: &responses.ResponseInputImageParam{
					ImageURL: openai.String(url),
					Detail:   detail,
				},
			})
		default:
			return nil, fmt.Errorf("unsupported user input part type: %s", part.Type)
		}
	}

	return contentList, nil
}

func multiContentToInputContent(parts []schema.ChatMessagePart) (responses.ResponseInputMessageContentListParam, error) {
	var contentList responses.ResponseInputMessageContentListParam

	for _, part := range parts {
		switch part.Type {
		case schema.ChatMessagePartTypeText:
			contentList = append(contentList, responses.ResponseInputContentUnionParam{
				OfInputText: &responses.ResponseInputTextParam{
					Text: part.Text,
				},
			})
		case schema.ChatMessagePartTypeImageURL:
			if part.ImageURL == nil {
				return nil, fmt.Errorf("ImageURL field must not be nil for image_url part type")
			}
			detail := responses.ResponseInputImageDetailAuto
			if part.ImageURL.Detail != "" {
				detail = responses.ResponseInputImageDetail(part.ImageURL.Detail)
			}
			contentList = append(contentList, responses.ResponseInputContentUnionParam{
				OfInputImage: &responses.ResponseInputImageParam{
					ImageURL: openai.String(part.ImageURL.URL),
					Detail:   detail,
				},
			})
		default:
			return nil, fmt.Errorf("unsupported multi content part type: %s", part.Type)
		}
	}

	return contentList, nil
}

// endregion

// region Response Conversion

func (m *ResponsesChatModel) convertResponse(resp *responses.Response) (*schema.Message, error) {
	if resp == nil {
		return nil, fmt.Errorf("response is nil")
	}
	if err := responseFailure(resp); err != nil {
		return nil, err
	}

	outMsg := &schema.Message{
		Role: schema.Assistant,
		Extra: map[string]any{
			ResponsesExtraKeyResponseID: resp.ID,
		},
		ResponseMeta: &schema.ResponseMeta{
			Usage: &schema.TokenUsage{
				PromptTokens:     int(resp.Usage.InputTokens),
				CompletionTokens: int(resp.Usage.OutputTokens),
				TotalTokens:      int(resp.Usage.TotalTokens),
				PromptTokenDetails: schema.PromptTokenDetails{
					CachedTokens: int(resp.Usage.InputTokensDetails.CachedTokens),
				},
				CompletionTokensDetails: schema.CompletionTokensDetails{
					ReasoningTokens: int(resp.Usage.OutputTokensDetails.ReasoningTokens),
				},
			},
		},
	}

	var textParts []string
	var reasoningParts []string
	var toolCalls []schema.ToolCall
	hasRefusal := false
	toolCallIdx := 0

	for _, item := range resp.Output {
		switch item.Type {
		case "message":
			msg := item.AsMessage()
			for _, content := range msg.Content {
				switch content.Type {
				case "output_text":
					textParts = append(textParts, content.Text)
				case "refusal":
					textParts = append(textParts, content.Refusal)
					hasRefusal = true
				}
			}

		case "function_call":
			fc := item.AsFunctionCall()
			idx := toolCallIdx
			toolCalls = append(toolCalls, schema.ToolCall{
				Index: &idx,
				ID:    fc.CallID,
				Type:  "function",
				Function: schema.FunctionCall{
					Name:      fc.Name,
					Arguments: fc.Arguments,
				},
			})
			toolCallIdx++

		case "reasoning":
			ri := item.AsReasoning()
			for _, summary := range ri.Summary {
				if summary.Text != "" {
					reasoningParts = append(reasoningParts, summary.Text)
				}
			}
		}
	}

	outMsg.Content = strings.Join(textParts, "")

	if len(reasoningParts) > 0 {
		outMsg.ReasoningContent = strings.Join(reasoningParts, "")
	}

	if len(toolCalls) > 0 {
		outMsg.ToolCalls = toolCalls
	}

	outMsg.ResponseMeta.FinishReason = mapFinishReason(resp, outputTraits{
		hasToolCalls: len(toolCalls) > 0,
		hasRefusal:   hasRefusal,
	})

	return outMsg, nil
}

// ResponsesFailedError reports a response whose terminal status carries no usable
// output. Such responses still arrive over HTTP 200, so the SDK raises nothing
// and the failure has to be turned into an error here.
//
// Code classifies the failure (server_error, rate_limit_exceeded, invalid_prompt,
// the image_* family, ...) but the API does not guarantee it is populated.
type ResponsesFailedError struct {
	ResponseID string
	Status     responses.ResponseStatus
	Code       responses.ResponseErrorCode
	Message    string
}

func (e *ResponsesFailedError) Error() string {
	// The provider message is passed through verbatim because retry policies
	// match on its wording to tell a context-window overflow apart from the
	// other invalid_prompt failures.
	return fmt.Sprintf("openai response %s ended with status=%s code=%s message=%s",
		e.ResponseID, e.Status, e.Code, e.Message)
}

// responseFailure returns a non-nil error when resp reached a terminal status
// that yields no usable output.
func responseFailure(resp *responses.Response) error {
	switch resp.Status {
	case responses.ResponseStatusFailed, responses.ResponseStatusCancelled:
		return &ResponsesFailedError{
			ResponseID: resp.ID,
			Status:     resp.Status,
			Code:       resp.Error.Code,
			Message:    resp.Error.Message,
		}
	default:
		return nil
	}
}

// outputTraits carries the parts of the finish reason that the response status
// cannot express on its own.
type outputTraits struct {
	hasToolCalls bool
	hasRefusal   bool
}

// mapFinishReason translates a Responses API status into the finish reason
// vocabulary this codebase shares with the Chat Completions API, so the retry
// policies in comm can judge both APIs by the same rules. Statuses without
// usable output are reported by responseFailure and never reach here.
func mapFinishReason(resp *responses.Response, traits outputTraits) string {
	switch resp.Status {
	case responses.ResponseStatusCompleted:
		// Tool calls outrank a refusal: they mean the model is still driving the
		// task forward, and reporting content_filter would make the caller throw
		// the call away and retry the whole turn.
		if traits.hasToolCalls {
			return "tool_calls"
		}
		// The API counts a refusal as a completed turn, but the model declined to
		// answer, so it is reported like a filtered response rather than as
		// usable output.
		if traits.hasRefusal {
			return "content_filter"
		}
	case responses.ResponseStatusIncomplete:
		// Truncated output is still usable, so the reason decides whether the
		// caller retries (length) or accepts the turn (content_filter).
		switch resp.IncompleteDetails.Reason {
		case "max_output_tokens":
			return "length"
		case "content_filter":
			return "content_filter"
		}
	}
	return string(resp.Status)
}

// endregion

// region Tool Conversion

func toResponseTools(tools []*schema.ToolInfo) ([]responses.ToolUnionParam, error) {
	result := make([]responses.ToolUnionParam, 0, len(tools))

	for _, ti := range tools {
		if ti == nil {
			return nil, fmt.Errorf("tool info cannot be nil")
		}

		paramsSchema, err := ti.ParamsOneOf.ToJSONSchema()
		if err != nil {
			return nil, fmt.Errorf("failed to convert tool parameters to JSONSchema: %w", err)
		}

		sortSchemaArrayFields(paramsSchema)

		paramsMap, err := jsonSchemaToMap(paramsSchema)
		if err != nil {
			return nil, fmt.Errorf("failed to convert JSONSchema to map: %w", err)
		}

		result = append(result, responses.ToolUnionParam{
			OfFunction: &responses.FunctionToolParam{
				Name:        ti.Name,
				Description: openai.String(ti.Desc),
				Parameters:  paramsMap,
				// Responses treats an omitted strict differently from strict=false.
				// Keep optional tool parameters optional instead of letting the API
				// constrain calls into emitting every property with a zero value.
				Strict: openai.Bool(false),
			},
		})
	}

	return result, nil
}

func jsonSchemaToMap(s *jsonschema.Schema) (map[string]any, error) {
	if s == nil {
		return nil, nil
	}
	data, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func sortSchemaArrayFields(sc *jsonschema.Schema) {
	if sc == nil {
		return
	}
	switch sc.Type {
	case "object":
		if len(sc.Required) > 0 {
			sort.Strings(sc.Required)
		}
		if sc.Properties != nil {
			for pair := sc.Properties.Oldest(); pair != nil; pair = pair.Next() {
				sortSchemaArrayFields(pair.Value)
			}
		}
	case "array":
		if sc.Items != nil {
			sortSchemaArrayFields(sc.Items)
		}
	}
}

// endregion

// region Tool Choice

func populateToolChoice(params *responses.ResponseNewParams, tc *schema.ToolChoice, allowedToolNames []string) error {
	if tc == nil {
		return nil
	}

	switch *tc {
	case schema.ToolChoiceForbidden:
		params.ToolChoice = responses.ResponseNewParamsToolChoiceUnion{
			OfToolChoiceMode: openai.Opt(responses.ToolChoiceOptionsNone),
		}
	case schema.ToolChoiceAllowed:
		params.ToolChoice = responses.ResponseNewParamsToolChoiceUnion{
			OfToolChoiceMode: openai.Opt(responses.ToolChoiceOptionsAuto),
		}
	case schema.ToolChoiceForced:
		if len(params.Tools) == 0 {
			return fmt.Errorf("tool_choice is forced but no tools are provided")
		}
		if len(allowedToolNames) == 1 {
			params.ToolChoice = responses.ResponseNewParamsToolChoiceUnion{
				OfFunctionTool: &responses.ToolChoiceFunctionParam{
					Name: allowedToolNames[0],
				},
			}
		} else {
			params.ToolChoice = responses.ResponseNewParamsToolChoiceUnion{
				OfToolChoiceMode: openai.Opt(responses.ToolChoiceOptionsRequired),
			}
		}
	}
	return nil
}

// endregion

// region Stream Builder

type streamBuilder struct {
	pendingToolCalls map[string]*schema.ToolCall
	streamedRefusals map[string]bool
	toolCallIdx      int
	hasToolCalls     bool
	hasRefusal       bool
	finalResponse    *responses.Response
	reasoningItems   []responses.ResponseOutputItemUnion
}

func newStreamBuilder() *streamBuilder {
	return &streamBuilder{
		pendingToolCalls: make(map[string]*schema.ToolCall),
		streamedRefusals: make(map[string]bool),
	}
}

func (b *streamBuilder) processEvent(ctx context.Context, event responses.ResponseStreamEventUnion) (msg *schema.Message, emit bool, err error) {
	switch event.Type {
	case "response.output_text.delta":
		return &schema.Message{
			Role:    schema.Assistant,
			Content: event.Delta,
		}, true, nil

	case "response.reasoning_summary_text.delta":
		return &schema.Message{
			Role:             schema.Assistant,
			ReasoningContent: event.Delta,
		}, true, nil

	// A refusal replaces the text output for the turn, so it is streamed as
	// content just like convertResponse does for the non-streaming path.
	case "response.refusal.delta":
		b.hasRefusal = true
		b.streamedRefusals[event.ItemID] = true
		return &schema.Message{
			Role:    schema.Assistant,
			Content: event.Delta,
		}, true, nil

	case "response.refusal.done":
		b.hasRefusal = true
		// Carries the whole refusal after the deltas, so it is only emitted when
		// no delta arrived for this item.
		if b.streamedRefusals[event.ItemID] || event.Refusal == "" {
			return nil, false, nil
		}
		return &schema.Message{
			Role:    schema.Assistant,
			Content: event.Refusal,
		}, true, nil

	case "response.function_call_arguments.delta":
		itemID := event.ItemID
		tc, ok := b.pendingToolCalls[itemID]
		if !ok {
			idx := b.toolCallIdx
			b.toolCallIdx++
			tc = &schema.ToolCall{
				Index: &idx,
				Type:  "function",
			}
			b.pendingToolCalls[itemID] = tc
		}
		return &schema.Message{
			Role: schema.Assistant,
			ToolCalls: []schema.ToolCall{{
				Index: tc.Index,
				Type:  "function",
				Function: schema.FunctionCall{
					Arguments: event.Delta,
				},
			}},
		}, true, nil

	case "response.function_call_arguments.done":
		if tc, ok := b.pendingToolCalls[event.ItemID]; ok {
			tc.Function.Name = event.Name
			tc.Function.Arguments = event.Arguments
		}
		return nil, false, nil

	case "response.output_item.added":
		if event.Item.Type == "function_call" {
			b.hasToolCalls = true
			fc := event.Item.AsFunctionCall()
			tc, exists := b.pendingToolCalls[event.Item.ID]
			if !exists {
				idx := b.toolCallIdx
				b.toolCallIdx++
				tc = &schema.ToolCall{
					Index: &idx,
					Type:  "function",
				}
				b.pendingToolCalls[event.Item.ID] = tc
			}
			tc.ID = fc.CallID
			tc.Function.Name = fc.Name
			return &schema.Message{
				Role: schema.Assistant,
				ToolCalls: []schema.ToolCall{{
					Index: tc.Index,
					ID:    fc.CallID,
					Type:  "function",
					Function: schema.FunctionCall{
						Name: fc.Name,
					},
				}},
			}, true, nil
		}
		return nil, false, nil

	case "response.output_item.done":
		if event.Item.Type == "reasoning" {
			b.reasoningItems = append(b.reasoningItems, event.Item)
		}
		if event.Item.Type == "function_call" {
			fc := event.Item.AsFunctionCall()
			if tc, ok := b.pendingToolCalls[event.Item.ID]; ok {
				tc.ID = fc.CallID
				tc.Function.Name = fc.Name
				tc.Function.Arguments = fc.Arguments
			}
		}
		return nil, false, nil

	// These lifecycle and completion events carry no additional output needed
	// by the builder.
	case "response.created",
		"response.in_progress",
		"response.queued",
		"response.content_part.added",
		"response.content_part.done",
		"response.output_text.done",
		"response.reasoning_summary_part.added",
		"response.reasoning_summary_part.done",
		"response.reasoning_summary_text.done":
		return nil, false, nil

	case "response.completed", "response.incomplete":
		b.finalResponse = &event.Response
		return nil, false, nil

	case "response.failed":
		resp := event.Response
		if err := responseFailure(&resp); err != nil {
			return nil, false, err
		}
		b.finalResponse = &resp
		return nil, false, nil

	case "error":
		return nil, false, fmt.Errorf("response API error: [%s] %s", event.Code, event.Message)

	default:
		// Ignore unknown informational events for forward compatibility.
		return nil, false, nil
	}
}

func (b *streamBuilder) finish(traceID string) (*schema.Message, error) {
	// Every response stream ends with a terminal event. Reaching the end without
	// one means the SSE stream was cut short, which would otherwise surface as a
	// silently truncated message carrying no finish reason.
	if b.finalResponse == nil {
		return nil, fmt.Errorf("response stream ended without a terminal event, trace_id=%s", traceID)
	}

	return &schema.Message{
		Role: schema.Assistant,
		Extra: map[string]any{
			ResponsesExtraKeyResponseID: b.finalResponse.ID,
		},
		ResponseMeta: &schema.ResponseMeta{
			FinishReason: mapFinishReason(b.finalResponse, outputTraits{
				hasToolCalls: b.hasToolCalls,
				hasRefusal:   b.hasRefusal,
			}),
			Usage: &schema.TokenUsage{
				PromptTokens:     int(b.finalResponse.Usage.InputTokens),
				CompletionTokens: int(b.finalResponse.Usage.OutputTokens),
				TotalTokens:      int(b.finalResponse.Usage.TotalTokens),
				PromptTokenDetails: schema.PromptTokenDetails{
					CachedTokens: int(b.finalResponse.Usage.InputTokensDetails.CachedTokens),
				},
				CompletionTokensDetails: schema.CompletionTokensDetails{
					ReasoningTokens: int(b.finalResponse.Usage.OutputTokensDetails.ReasoningTokens),
				},
			},
		},
	}, nil
}

// endregion

// region Helpers

func toModelCallbackUsage(meta *schema.ResponseMeta) *model.TokenUsage {
	if meta == nil || meta.Usage == nil {
		return nil
	}
	usage := meta.Usage
	return &model.TokenUsage{
		PromptTokens: usage.PromptTokens,
		PromptTokenDetails: model.PromptTokenDetails{
			CachedTokens: usage.PromptTokenDetails.CachedTokens,
		},
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
		CompletionTokensDetails: model.CompletionTokensDetails{
			ReasoningTokens: usage.CompletionTokensDetails.ReasoningTokens,
		},
	}
}

func responseTraceID(response *http.Response) string {
	if response == nil {
		return ""
	}
	return strings.TrimSpace(response.Header.Get(responsesRequestIDHeader))
}

func setResponseTraceID(msg *schema.Message, traceID string) {
	if msg == nil || traceID == "" {
		return
	}
	if msg.Extra == nil {
		msg.Extra = make(map[string]any, 1)
	}
	msg.Extra[ResponsesExtraKeyRequestID] = traceID
}

func newCallbackOutput(msg *schema.Message, config *model.Config, traceID string) *model.CallbackOutput {
	output := &model.CallbackOutput{
		Message: msg,
		Config:  config,
	}
	if msg != nil {
		output.TokenUsage = toModelCallbackUsage(msg.ResponseMeta)
	}
	if traceID != "" {
		output.Extra = map[string]any{ResponsesExtraKeyRequestID: traceID}
	}
	return output
}

func derefOrZero[T any](ptr *T) T {
	if ptr == nil {
		var zero T
		return zero
	}
	return *ptr
}

func intPtrToInt(p *int64) *int {
	if p == nil {
		return nil
	}
	v := int(*p)
	return &v
}

// endregion

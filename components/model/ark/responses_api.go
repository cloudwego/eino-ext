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

package ark

import (
	"context"
	"fmt"
	"runtime/debug"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/packages/ssestream"
	"github.com/openai/openai-go/responses"
	"github.com/openai/openai-go/shared"
	arkModel "github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type responsesAPIChatModel struct {
	client responses.ResponseService

	tools    []responses.ToolUnionParam
	rawTools []*schema.ToolInfo

	model          string
	maxTokens      *int
	temperature    *float32
	topP           *float32
	customHeader   map[string]string
	responseFormat *ResponseFormat
	thinking       *arkModel.Thinking
	cache          *CacheConfig
	serviceTier    *string
}

func (cm *responsesAPIChatModel) Generate(ctx context.Context, input []*schema.Message,
	opts ...model.Option) (outMsg *schema.Message, err error) {

	options, specOptions, err := cm.getOptions(opts)
	if err != nil {
		return nil, err
	}

	req, reqOpts, err := cm.genRequestAndOptions(input, options, specOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create generate request: %w", err)
	}

	config := cm.toCallbackConfig(req)

	tools := cm.rawTools
	if options.Tools != nil {
		tools = options.Tools
	}

	ctx = callbacks.OnStart(ctx, &model.CallbackInput{
		Messages: input,
		Tools:    tools,
		Config:   config,
		Extra:    map[string]any{callbackExtraKeyThinking: specOptions.thinking},
	})

	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	resp, err := cm.client.New(ctx, req, reqOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create generate request: %w", err)
	}

	outMsg, err = cm.toOutputMessage(resp, req.Store.Value)
	if err != nil {
		return nil, fmt.Errorf("failed to convert output to schema.Message: %w", err)
	}

	callbacks.OnEnd(ctx, &model.CallbackOutput{
		Message:    outMsg,
		Config:     config,
		TokenUsage: cm.toModelTokenUsage(resp.Usage),
		Extra:      map[string]any{callbackExtraKeyThinking: specOptions.thinking},
	})

	return outMsg, nil
}

func (cm *responsesAPIChatModel) Stream(ctx context.Context, input []*schema.Message,
	opts ...model.Option) (outStream *schema.StreamReader[*schema.Message], err error) {

	options, specOptions, err := cm.getOptions(opts)
	if err != nil {
		return nil, err
	}

	req, reqOpts, err := cm.genRequestAndOptions(input, options, specOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create stream request: %w", err)
	}

	config := cm.toCallbackConfig(req)

	tools := cm.rawTools
	if options.Tools != nil {
		tools = options.Tools
	}

	ctx = callbacks.OnStart(ctx, &model.CallbackInput{
		Messages: input,
		Tools:    tools,
		Config:   config,
		Extra:    map[string]any{callbackExtraKeyThinking: specOptions.thinking},
	})

	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	streamResp := cm.client.NewStreaming(ctx, req, reqOpts...)
	if streamResp.Err() != nil {
		return nil, fmt.Errorf("failed to create stream request: %w", streamResp.Err())
	}

	sr, sw := schema.Pipe[*model.CallbackOutput](1)

	go func() {
		defer func() {
			pe := recover()
			if pe != nil {
				_ = sw.Send(nil, newPanicErr(pe, debug.Stack()))
			}

			_ = streamResp.Close()
			sw.Close()
		}()

		cm.receivedStreamResponse(streamResp, config, req.Store.Value, sw)

	}()

	ctx, nsr := callbacks.OnEndWithStreamOutput(ctx, schema.StreamReaderWithConvert(sr,
		func(src *model.CallbackOutput) (callbacks.CallbackOutput, error) {
			if src.Extra == nil {
				src.Extra = make(map[string]any)
			}
			src.Extra[callbackExtraKeyThinking] = specOptions.thinking
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

func (cm *responsesAPIChatModel) setStreamChunkDefaultExtra(msg *schema.Message, response responses.Response, enableCache bool) {
	if enableCache {
		setResponseCaching(msg, cachingEnabled)
	}
	setContextID(msg, response.ID)
	setResponseID(msg, response.ID)
	setServiceTier(msg, string(response.ServiceTier))
}

func (cm *responsesAPIChatModel) receivedStreamResponse(streamResp *ssestream.Stream[responses.ResponseStreamEventUnion],
	config *model.Config, enableCache bool, sw *schema.StreamWriter[*model.CallbackOutput]) {

	var toolCallMetaMsg *schema.Message

	defer func() {
		if toolCallMetaMsg != nil {
			cm.sendCallbackOutput(sw, config, toolCallMetaMsg)
		}
	}()

Outer:
	for streamResp.Next() {
		cur := streamResp.Current()

		if msg, ok := cm.isAddedToolCall(cur); ok {
			toolCallMetaMsg = msg
			continue
		}

		event := cur.AsAny()

		switch asEvent := event.(type) {
		case responses.ResponseCreatedEvent:
			msg := &schema.Message{
				Role: schema.Assistant,
			}

			cm.setStreamChunkDefaultExtra(msg, asEvent.Response, enableCache)
			cm.sendCallbackOutput(sw, config, msg)

			continue

		case responses.ResponseCompletedEvent:
			msg := cm.handleCompletedStreamEvent(asEvent)

			cm.setStreamChunkDefaultExtra(msg, asEvent.Response, enableCache)
			cm.sendCallbackOutput(sw, config, msg)

			break Outer

		case responses.ResponseErrorEvent:
			sw.Send(nil, fmt.Errorf("received error: %s", asEvent.Message))
			break Outer

		case responses.ResponseIncompleteEvent:
			msg := cm.handleIncompleteStreamEvent(asEvent)

			cm.setStreamChunkDefaultExtra(msg, asEvent.Response, enableCache)
			cm.sendCallbackOutput(sw, config, msg)

			break Outer

		case responses.ResponseFailedEvent:
			msg := cm.handleFailedStreamEvent(asEvent)
			cm.setStreamChunkDefaultExtra(msg, asEvent.Response, enableCache)
			cm.sendCallbackOutput(sw, config, msg)

			break Outer

		default:
			msg := cm.handleDeltaStreamEvent(event)
			if msg == nil {
				continue
			}

			if toolCallMetaMsg != nil && len(msg.ToolCalls) > 0 {
				toolCallMeta := toolCallMetaMsg.ToolCalls[0]
				toolCall := msg.ToolCalls[0]

				toolCall.ID = toolCallMeta.ID
				toolCall.Type = toolCallMeta.Type
				toolCall.Function.Name = toolCallMeta.Function.Name
				for k, v := range toolCallMeta.Extra {
					_, ok := toolCall.Extra[k]
					if !ok {
						toolCall.Extra[k] = v
					}
				}

				msg.ToolCalls[0] = toolCall
				toolCallMetaMsg = nil
			}

			cm.sendCallbackOutput(sw, config, msg)
		}
	}

	if streamResp.Err() != nil {
		_ = sw.Send(nil, fmt.Errorf("failed to read stream: %w", streamResp.Err()))
	}
}

func (cm *responsesAPIChatModel) sendCallbackOutput(sw *schema.StreamWriter[*model.CallbackOutput], reqConf *model.Config,
	msg *schema.Message) {

	var token *model.TokenUsage
	if msg.ResponseMeta != nil && msg.ResponseMeta.Usage != nil {
		token = &model.TokenUsage{
			PromptTokens: msg.ResponseMeta.Usage.PromptTokens,
			PromptTokenDetails: model.PromptTokenDetails{
				CachedTokens: msg.ResponseMeta.Usage.PromptTokenDetails.CachedTokens,
			},
			CompletionTokens: msg.ResponseMeta.Usage.CompletionTokens,
			TotalTokens:      msg.ResponseMeta.Usage.TotalTokens,
		}
	}

	sw.Send(&model.CallbackOutput{
		Message:    msg,
		Config:     reqConf,
		TokenUsage: token,
	}, nil)
}

func (cm *responsesAPIChatModel) isAddedToolCall(event responses.ResponseStreamEventUnion) (*schema.Message, bool) {
	asEvent, ok := event.AsAny().(responses.ResponseOutputItemAddedEvent)
	if !ok {
		return nil, false
	}

	asItem, ok := asEvent.Item.AsAny().(responses.ResponseFunctionToolCall)
	if !ok {
		return nil, false
	}

	msg := &schema.Message{
		Role: schema.Assistant,
		ToolCalls: []schema.ToolCall{
			{
				ID:   asItem.CallID,
				Type: string(asItem.Type),
				Function: schema.FunctionCall{
					Name: asItem.Name,
				},
			},
		},
	}

	return msg, true
}

func (cm *responsesAPIChatModel) handleCompletedStreamEvent(asChunk responses.ResponseCompletedEvent) *schema.Message {
	return &schema.Message{
		Role: schema.Assistant,
		ResponseMeta: &schema.ResponseMeta{
			FinishReason: string(asChunk.Response.Status),
			Usage:        cm.toEinoTokenUsage(asChunk.Response.Usage),
		},
	}
}

func (cm *responsesAPIChatModel) handleIncompleteStreamEvent(asChunk responses.ResponseIncompleteEvent) *schema.Message {
	return &schema.Message{
		Role: schema.Assistant,
		ResponseMeta: &schema.ResponseMeta{
			FinishReason: asChunk.Response.IncompleteDetails.Reason,
			Usage:        cm.toEinoTokenUsage(asChunk.Response.Usage),
		},
	}
}

func (cm *responsesAPIChatModel) handleFailedStreamEvent(asChunk responses.ResponseFailedEvent) *schema.Message {
	return &schema.Message{
		Role: schema.Assistant,
		ResponseMeta: &schema.ResponseMeta{
			FinishReason: asChunk.Response.Error.Message,
			Usage:        cm.toEinoTokenUsage(asChunk.Response.Usage),
		},
	}
}

func (cm *responsesAPIChatModel) handleDeltaStreamEvent(asChunk any) *schema.Message {
	switch asEvent := asChunk.(type) {
	case responses.ResponseTextDeltaEvent:
		return &schema.Message{
			Role:    schema.Assistant,
			Content: asEvent.Delta,
		}

	case responses.ResponseFunctionCallArgumentsDeltaEvent:
		return &schema.Message{
			Role: schema.Assistant,
			ToolCalls: []schema.ToolCall{
				{
					Index: ptrOf(int(asEvent.OutputIndex)),
					Function: schema.FunctionCall{
						Arguments: asEvent.Delta,
					},
				},
			},
		}

	case responses.ResponseReasoningSummaryTextDeltaEvent:
		msg := &schema.Message{
			Role:             schema.Assistant,
			ReasoningContent: asEvent.Delta,
		}
		setReasoningContent(msg, asEvent.Delta)

		return msg
	}

	return nil
}

func (cm *responsesAPIChatModel) toTools(tis []*schema.ToolInfo) ([]responses.ToolUnionParam, error) {
	tools := make([]responses.ToolUnionParam, len(tis))
	for i := range tis {
		ti := tis[i]
		if ti == nil {
			return nil, fmt.Errorf("tool info cannot be nil in WithTools")
		}

		paramsJSONSchema, err := ti.ParamsOneOf.ToJSONSchema()
		if err != nil {
			return nil, fmt.Errorf("failed to convert tool parameters to JSONSchema: %w", err)
		}

		b, err := sonic.Marshal(paramsJSONSchema)
		if err != nil {
			return nil, fmt.Errorf("marshal paramsJSONSchema fail: %w", err)
		}

		params := map[string]any{}
		if err = sonic.Unmarshal(b, &params); err != nil {
			return nil, fmt.Errorf("unmarshal paramsJSONSchema fail: %w", err)
		}

		tools[i] = responses.ToolUnionParam{
			OfFunction: &responses.FunctionToolParam{
				Name:        ti.Name,
				Description: newOpenaiStringOpt(&ti.Desc),
				Parameters:  params,
			},
		}
	}

	return tools, nil
}

func (cm *responsesAPIChatModel) genRequestAndOptions(in []*schema.Message, options *model.Options, specOptions *arkOptions) (req responses.ResponseNewParams,
	reqOpts []option.RequestOption, err error) {

	var text *responses.ResponseTextConfigParam
	if cm.responseFormat != nil {
		text = &responses.ResponseTextConfigParam{
			Format: responses.ResponseFormatTextConfigUnionParam{
				OfText: ptrOf(shared.NewResponseFormatTextParam()),
			},
		}
		if cm.responseFormat.Type == arkModel.ResponseFormatJsonObject {
			text.Format = responses.ResponseFormatTextConfigUnionParam{
				OfJSONObject: ptrOf(shared.NewResponseFormatJSONObjectParam()),
			}
		}
	}

	req = responses.ResponseNewParams{
		Text:            ptrFromOrZero(text),
		Model:           ptrFromOrZero(options.Model),
		MaxOutputTokens: newOpenaiIntOpt(options.MaxTokens),
		Temperature:     newOpenaiFloatOpt(options.Temperature),
		TopP:            newOpenaiFloatOpt(options.TopP),
		ServiceTier:     responses.ResponseNewParamsServiceTier(ptrFromOrZero(cm.serviceTier)),
	}

	var in_ []*schema.Message
	if in_, req, reqOpts, err = cm.populateCache(in, req, specOptions, reqOpts); err != nil {
		return req, nil, err
	}

	if err = cm.populateInput(&req, in_); err != nil {
		return req, nil, err
	}

	if err = cm.populateTools(&req, options.Tools); err != nil {
		return req, nil, err
	}

	for k, v := range specOptions.customHeaders {
		reqOpts = append(reqOpts, option.WithHeaderAdd(k, v))
	}

	if specOptions.thinking != nil {
		reqOpts = append(reqOpts, option.WithJSONSet("thinking", specOptions.thinking))
	}

	return req, reqOpts, nil
}

func (cm *responsesAPIChatModel) checkOptions(mOpts *model.Options, _ *arkOptions) error {
	if len(mOpts.Stop) > 0 {
		return fmt.Errorf("'Stop' is not supported by responses API")
	}
	return nil
}

func (cm *responsesAPIChatModel) populateCache(in []*schema.Message, req responses.ResponseNewParams, arkOpts *arkOptions,
	reqOpts []option.RequestOption) ([]*schema.Message, responses.ResponseNewParams, []option.RequestOption, error) {

	var (
		store       = param.NewOpt(false)
		cacheStatus = cachingDisabled
		cacheTTL    *int
		headRespID  *string
		contextID   *string
	)

	if cm.cache != nil {
		if sCache := cm.cache.SessionCache; sCache != nil {
			if sCache.EnableCache {
				store = param.NewOpt(true)
				cacheStatus = cachingEnabled
			}
			cacheTTL = &sCache.TTL
		}
	}

	if cacheOpt := arkOpts.cache; cacheOpt != nil {
		// ContextID may be passed in the old logic
		contextID = cacheOpt.ContextID
		headRespID = cacheOpt.HeadPreviousResponseID

		if sCacheOpt := cacheOpt.SessionCache; sCacheOpt != nil {
			cacheTTL = &sCacheOpt.TTL

			if sCacheOpt.EnableCache {
				store = param.NewOpt(true)
				cacheStatus = cachingEnabled
			} else {
				store = param.NewOpt(false)
				cacheStatus = cachingDisabled
			}
		}
	}

	var (
		preRespID *string
		inputIdx  int
	)

	// If the user implements session caching with ContextID,
	// ContextID and ResponseID will exist at the same time.
	// Using ContextID is prioritized to maintain compatibility with the old logic.
	// In this usage scenario, ResponseID cannot be used.
	if cacheStatus == cachingEnabled && contextID == nil {
		for i := len(in) - 1; i >= 0; i-- {
			msg := in[i]
			inputIdx = i
			if caching_, _ := getResponseCaching(msg); caching_ != string(cachingEnabled) {
				continue
			}
			if id, ok := GetResponseID(msg); ok {
				preRespID = &id
				break
			}
		}
	}

	if preRespID != nil {
		if inputIdx+1 >= len(in) {
			return in, req, reqOpts, fmt.Errorf("not found incremental input after ResponseID")
		}
		in = in[inputIdx+1:]
	}

	// ResponseID has a higher priority than HeadPreviousResponseID
	if preRespID == nil {
		preRespID = headRespID
		if contextID != nil { // Prioritize ContextID
			preRespID = contextID
		}
	}

	req.PreviousResponseID = newOpenaiStringOpt(preRespID)
	req.Store = store

	if cacheTTL != nil {
		reqOpts = append(reqOpts, option.WithJSONSet("expire_at", time.Now().Unix()+int64(*cacheTTL)))
	}

	reqOpts = append(reqOpts, option.WithJSONSet("caching", map[string]any{
		"type": cacheStatus,
	}))

	return in, req, reqOpts, nil
}

func (cm *responsesAPIChatModel) populateInput(req *responses.ResponseNewParams, in []*schema.Message) error {
	itemList := make([]responses.ResponseInputItemUnionParam, 0, len(in))

	if len(in) == 0 {
		return nil
	}

	for _, msg := range in {
		content, err := cm.toOpenaiMultiModalContent(msg)
		if err != nil {
			return err
		}

		switch msg.Role {
		case schema.User:
			itemList = append(itemList, responses.ResponseInputItemUnionParam{
				OfMessage: &responses.EasyInputMessageParam{
					Role:    responses.EasyInputMessageRoleUser,
					Content: content,
				},
			})

		case schema.Assistant:
			if content.OfString.Valid() || len(content.OfInputItemContentList) > 0 {
				itemList = append(itemList, responses.ResponseInputItemUnionParam{
					OfMessage: &responses.EasyInputMessageParam{
						Role:    responses.EasyInputMessageRoleAssistant,
						Content: content,
					},
				})
			}

			for _, toolCall := range msg.ToolCalls {
				itemList = append(itemList, responses.ResponseInputItemUnionParam{
					OfFunctionCall: &responses.ResponseFunctionToolCallParam{
						CallID:    toolCall.ID,
						Name:      toolCall.Function.Name,
						Arguments: toolCall.Function.Arguments,
					},
				})
			}

		case schema.System:
			itemList = append(itemList, responses.ResponseInputItemUnionParam{
				OfMessage: &responses.EasyInputMessageParam{
					Role:    responses.EasyInputMessageRoleSystem,
					Content: content,
				},
			})

		case schema.Tool:
			itemList = append(itemList, responses.ResponseInputItemUnionParam{
				OfFunctionCallOutput: &responses.ResponseInputItemFunctionCallOutputParam{
					CallID: msg.ToolCallID,
					Output: msg.Content,
				},
			})

		default:
			return fmt.Errorf("unknown role: %s", msg.Role)
		}
	}

	req.Input = responses.ResponseNewParamsInputUnion{
		OfInputItemList: itemList,
	}

	return nil
}

func (cm *responsesAPIChatModel) toOpenaiMultiModalContent(msg *schema.Message) (responses.EasyInputMessageContentUnionParam, error) {
	content := responses.EasyInputMessageContentUnionParam{}

	if msg.Content != "" {
		if len(msg.MultiContent) == 0 && len(msg.UserInputMultiContent) == 0 && len(msg.AssistantGenMultiContent) == 0 {
			content.OfString = param.NewOpt(msg.Content)
			return content, nil
		}

		content.OfInputItemContentList = append(content.OfInputItemContentList, responses.ResponseInputContentUnionParam{
			OfInputText: &responses.ResponseInputTextParam{
				Text: msg.Content,
			},
		})
	}

	if len(msg.UserInputMultiContent) > 0 && len(msg.AssistantGenMultiContent) > 0 {
		return content, fmt.Errorf("a message cannot contain both UserInputMultiContent and AssistantGenMultiContent")
	}

	if len(msg.UserInputMultiContent) > 0 {
		if msg.Role != schema.User {
			return content, fmt.Errorf("user input multi content only support user role, got %s", msg.Role)
		}
		for _, part := range msg.UserInputMultiContent {
			switch part.Type {
			case schema.ChatMessagePartTypeText:
				content.OfInputItemContentList = append(content.OfInputItemContentList, responses.ResponseInputContentUnionParam{
					OfInputText: &responses.ResponseInputTextParam{
						Text: part.Text,
					},
				})
			case schema.ChatMessagePartTypeImageURL:
				if part.Image == nil {
					return content, fmt.Errorf("image field must not be nil when Type is ChatMessagePartTypeImageURL in user message")
				} else {
					var imageURL string
					var err error
					if part.Image.URL != nil {
						imageURL = *part.Image.URL
					} else if part.Image.Base64Data != nil {
						if part.Image.MIMEType == "" {
							return content, fmt.Errorf("image part must have MIMEType when use Base64Data")
						}
						imageURL, err = ensureDataURL(*part.Image.Base64Data, part.Image.MIMEType)
						if err != nil {
							return content, err
						}
					}
					content.OfInputItemContentList = append(content.OfInputItemContentList, responses.ResponseInputContentUnionParam{
						OfInputImage: &responses.ResponseInputImageParam{
							ImageURL: param.NewOpt(imageURL),
						},
					})
				}
			default:
				return content, fmt.Errorf("unsupported content type in UserInputMultiContent: %s", part.Type)
			}
		}
		return content, nil
	} else if len(msg.AssistantGenMultiContent) > 0 {
		if msg.Role != schema.Assistant {
			return content, fmt.Errorf("assistant gen multi content only support assistant role, got %s", msg.Role)
		}
		for _, part := range msg.AssistantGenMultiContent {
			switch part.Type {
			case schema.ChatMessagePartTypeText:
				content.OfInputItemContentList = append(content.OfInputItemContentList, responses.ResponseInputContentUnionParam{
					OfInputText: &responses.ResponseInputTextParam{
						Text: part.Text,
					},
				})
			case schema.ChatMessagePartTypeImageURL:
				if part.Image == nil {
					return content, fmt.Errorf("image field must not be nil when Type is ChatMessagePartTypeImageURL in assistant message")
				} else {
					var imageURL string
					var err error
					if part.Image.URL != nil {
						imageURL = *part.Image.URL
					} else if part.Image.Base64Data != nil {
						if part.Image.MIMEType == "" {
							return content, fmt.Errorf("image part must have MIMEType when use Base64Data")
						}
						imageURL, err = ensureDataURL(*part.Image.Base64Data, part.Image.MIMEType)
						if err != nil {
							return content, err
						}
					}
					content.OfInputItemContentList = append(content.OfInputItemContentList, responses.ResponseInputContentUnionParam{
						OfInputImage: &responses.ResponseInputImageParam{
							ImageURL: param.NewOpt(imageURL),
						},
					})
				}
			default:
				return content, fmt.Errorf("unsupported content type in AssistantGenMultiContent: %s", part.Type)
			}
		}
		return content, nil
	} else {
		for _, c := range msg.MultiContent {
			switch c.Type {
			case schema.ChatMessagePartTypeText:
				content.OfInputItemContentList = append(content.OfInputItemContentList, responses.ResponseInputContentUnionParam{
					OfInputText: &responses.ResponseInputTextParam{
						Text: c.Text,
					},
				})

			case schema.ChatMessagePartTypeImageURL:
				if c.ImageURL == nil {
					continue
				}
				content.OfInputItemContentList = append(content.OfInputItemContentList, responses.ResponseInputContentUnionParam{
					OfInputImage: &responses.ResponseInputImageParam{
						ImageURL: param.NewOpt(c.ImageURL.URL),
					},
				})

			default:
				return content, fmt.Errorf("unsupported content type: %s", c.Type)
			}
		}
	}

	return content, nil
}

func (cm *responsesAPIChatModel) populateTools(req *responses.ResponseNewParams, optTools []*schema.ToolInfo) error {
	// When caching is enabled, the tool is only passed on the first request.
	if req.PreviousResponseID.Valid() {
		return nil
	}

	tools := cm.tools

	if optTools != nil {
		var err error
		if tools, err = cm.toTools(optTools); err != nil {
			return err
		}
	}

	req.Tools = tools

	return nil
}

func (cm *responsesAPIChatModel) toCallbackConfig(req responses.ResponseNewParams) *model.Config {
	return &model.Config{
		Model:       req.Model,
		MaxTokens:   int(req.MaxOutputTokens.Value),
		Temperature: float32(req.Temperature.Value),
		TopP:        float32(req.TopP.Value),
	}
}

func (cm *responsesAPIChatModel) toOutputMessage(resp *responses.Response, enableCache bool) (*schema.Message, error) {
	msg := &schema.Message{
		Role: schema.Assistant,
		ResponseMeta: &schema.ResponseMeta{
			FinishReason: string(resp.Status),
			Usage:        cm.toEinoTokenUsage(resp.Usage),
		},
	}

	if enableCache {
		setResponseCaching(msg, cachingEnabled)
	}
	setContextID(msg, resp.ID)
	setResponseID(msg, resp.ID)

	if len(resp.ServiceTier) > 0 {
		setServiceTier(msg, string(resp.ServiceTier))
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
		return nil, fmt.Errorf("received empty output from ARK")
	}

	for _, item := range resp.Output {
		switch asItem := item.AsAny().(type) {
		case responses.ResponseOutputMessage:
			if len(asItem.Content) == 0 {
				return nil, fmt.Errorf("received empty message content from ARK")
			}
			msg.Content = asItem.Content[0].Text

		case responses.ResponseReasoningItem:
			if len(asItem.Summary) == 0 {
				continue
			}
			msg.ReasoningContent = asItem.Summary[0].Text
			setReasoningContent(msg, msg.ReasoningContent)

		case responses.ResponseFunctionToolCall:
			msg.ToolCalls = append(msg.ToolCalls, schema.ToolCall{
				ID:   asItem.CallID,
				Type: string(asItem.Type),
				Function: schema.FunctionCall{
					Name:      asItem.Name,
					Arguments: asItem.Arguments,
				},
			})

		default:
			continue
		}
	}

	return msg, nil
}

func (cm *responsesAPIChatModel) toEinoTokenUsage(usage responses.ResponseUsage) *schema.TokenUsage {
	return &schema.TokenUsage{
		PromptTokens: int(usage.InputTokens),
		PromptTokenDetails: schema.PromptTokenDetails{
			CachedTokens: int(usage.InputTokensDetails.CachedTokens),
		},
		CompletionTokens: int(usage.OutputTokens),
		TotalTokens:      int(usage.TotalTokens),
	}
}

func (cm *responsesAPIChatModel) toModelTokenUsage(usage responses.ResponseUsage) *model.TokenUsage {
	return &model.TokenUsage{
		PromptTokens: int(usage.InputTokens),
		PromptTokenDetails: model.PromptTokenDetails{
			CachedTokens: int(usage.InputTokensDetails.CachedTokens),
		},
		CompletionTokens: int(usage.OutputTokens),
		TotalTokens:      int(usage.TotalTokens),
	}
}

func (cm *responsesAPIChatModel) getOptions(opts []model.Option) (*model.Options, *arkOptions, error) {
	options := model.GetCommonOptions(&model.Options{
		Temperature: cm.temperature,
		MaxTokens:   cm.maxTokens,
		Model:       &cm.model,
		TopP:        cm.topP,
		ToolChoice:  ptrOf(schema.ToolChoiceAllowed),
	}, opts...)

	arkOpts := model.GetImplSpecificOptions(&arkOptions{
		customHeaders: cm.customHeader,
		thinking:      cm.thinking,
	}, opts...)

	if err := cm.checkOptions(options, arkOpts); err != nil {
		return nil, nil, err
	}

	return options, arkOpts, nil
}

func ensureDataURL(dataOfBase64, mimeType string) (string, error) {
	if strings.HasPrefix(dataOfBase64, "data:") {
		return "", fmt.Errorf("base64Data field must be a raw base64 string, but got a string with prefix 'data:'")
	}
	if mimeType == "" {
		return "", fmt.Errorf("mimeType field is required")
	}
	return fmt.Sprintf("data:%s;base64,%s", mimeType, dataOfBase64), nil
}

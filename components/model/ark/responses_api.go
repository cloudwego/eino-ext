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
	"errors"
	"fmt"
	"io"
	"runtime/debug"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime"
	arkModel "github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model/responses"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/utils"
)

type responsesAPIChatModel struct {
	client     *arkruntime.Client
	tools      []*responses.ResponsesTool
	rawTools   []*schema.ToolInfo
	toolChoice *schema.ToolChoice

	model           string
	maxTokens       *int
	temperature     *float32
	topP            *float32
	customHeader    map[string]string
	responseFormat  *ResponseFormat
	thinking        *arkModel.Thinking
	cache           *CacheConfig
	serviceTier     *string
	reasoningEffort *arkModel.ReasoningEffort
}
type cacheConfig struct {
	Enabled  bool
	ExpireAt *int64
}

func (cm *responsesAPIChatModel) Generate(ctx context.Context, input []*schema.Message,
	opts ...model.Option) (outMsg *schema.Message, err error) {
	options, specOptions, err := cm.getOptions(opts)
	if err != nil {
		return nil, err
	}

	responseReq, err := cm.genRequestAndOptions(input, options, specOptions)
	if err != nil {
		return nil, fmt.Errorf("genRequestAndOptions failed: %w", err)
	}
	config := cm.toCallbackConfig(responseReq)

	tools := cm.rawTools
	if options.Tools != nil {
		tools = options.Tools
	}

	callbackExtra := map[string]any{
		callbackExtraKeyThinking: specOptions.thinking,
	}
	if responseReq.PreviousResponseId != nil {
		callbackExtra[callbackExtraKeyPreResponseID] = *responseReq.PreviousResponseId
	}

	ctx = callbacks.OnStart(ctx, &model.CallbackInput{
		Messages:   input,
		Tools:      tools,
		ToolChoice: options.ToolChoice,
		Config:     config,
		Extra:      callbackExtra,
	})

	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	responseObject, err := cm.client.CreateResponses(ctx, responseReq, arkruntime.WithCustomHeaders(specOptions.customHeaders))
	if err != nil {
		return nil, fmt.Errorf("failed to create responses: %w", err)
	}

	cacheCfg := &cacheConfig{}
	if responseReq.Caching != nil && responseReq.Caching.Type != nil {
		cacheCfg.Enabled = *responseReq.Caching.Type == responses.CacheType_enabled
		cacheCfg.ExpireAt = responseReq.ExpireAt
	}

	outMsg, err = cm.toOutputMessage(responseObject, cacheCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to convert output to schema.Message: %w", err)
	}

	callbackExtra[callbackExtraModelName] = responseObject.Model

	callbacks.OnEnd(ctx, &model.CallbackOutput{
		Message:    outMsg,
		Config:     config,
		TokenUsage: cm.toModelTokenUsage(responseObject.Usage),
		Extra:      callbackExtra,
	})
	return outMsg, nil

}

func (cm *responsesAPIChatModel) Stream(ctx context.Context, input []*schema.Message,
	opts ...model.Option) (outStream *schema.StreamReader[*schema.Message], err error) {

	options, specOptions, err := cm.getOptions(opts)
	if err != nil {
		return nil, err
	}

	responseReq, err := cm.genRequestAndOptions(input, options, specOptions)
	if err != nil {
		return nil, fmt.Errorf("genRequestAndOptions failed: %w", err)
	}
	config := cm.toCallbackConfig(responseReq)
	tools := cm.rawTools
	if options.Tools != nil {
		tools = options.Tools
	}

	callbackExtra := map[string]any{
		callbackExtraKeyThinking: specOptions.thinking,
	}
	if responseReq.PreviousResponseId != nil {
		callbackExtra[callbackExtraKeyPreResponseID] = *responseReq.PreviousResponseId
	}

	ctx = callbacks.OnStart(ctx, &model.CallbackInput{
		Messages:   input,
		Tools:      tools,
		ToolChoice: options.ToolChoice,
		Config:     config,
		Extra:      callbackExtra,
	})

	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	responseStreamReader, err := cm.client.CreateResponsesStream(ctx, responseReq, arkruntime.WithCustomHeaders(specOptions.customHeaders))
	if err != nil {
		return nil, fmt.Errorf("failed to create responses: %w", err)
	}

	sr, sw := schema.Pipe[*model.CallbackOutput](1)

	go func() {
		defer func() {
			pe := recover()
			if pe != nil {
				_ = sw.Send(nil, newPanicErr(pe, debug.Stack()))
			}

			_ = responseStreamReader.Close()
			sw.Close()
		}()

		var cacheCfg = &cacheConfig{}
		if responseReq.Caching != nil && responseReq.Caching.Type != nil {
			cacheCfg.Enabled = *responseReq.Caching.Type == responses.CacheType_enabled
			cacheCfg.ExpireAt = responseReq.ExpireAt
		}

		cm.receivedStreamResponse(responseStreamReader, config, cacheCfg, sw)

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

	return outStream, err
}

func (cm *responsesAPIChatModel) prePopulateConfig(responseReq *responses.ResponsesRequest, options *model.Options,
	specOptions *arkOptions) error {

	if cm.responseFormat != nil {
		textFormat := &responses.ResponsesText{Format: &responses.TextFormat{}}
		switch cm.responseFormat.Type {
		case arkModel.ResponseFormatText:
			textFormat.Format.Type = responses.TextType_text
		case arkModel.ResponseFormatJsonObject:
			textFormat.Format.Type = responses.TextType_json_object
		case arkModel.ResponseFormatJSONSchema:
			textFormat.Format.Type = responses.TextType_json_schema
			b, err := sonic.Marshal(cm.responseFormat.JSONSchema)
			if err != nil {
				return fmt.Errorf("marshal JSONSchema fail: %w", err)
			}
			textFormat.Format.Schema = &responses.Bytes{Value: b}
			textFormat.Format.Name = cm.responseFormat.JSONSchema.Name
			textFormat.Format.Description = &cm.responseFormat.JSONSchema.Description
			textFormat.Format.Strict = &cm.responseFormat.JSONSchema.Strict
		default:
			return fmt.Errorf("unsupported response format type: %s", cm.responseFormat.Type)
		}
		responseReq.Text = textFormat
	}
	if options.Model != nil {
		responseReq.Model = *options.Model
	}
	if options.MaxTokens != nil {
		responseReq.MaxOutputTokens = ptrOf(int64(*options.MaxTokens))
	}
	if options.Temperature != nil {
		responseReq.Temperature = ptrOf(float64(*options.Temperature))
	}
	if options.TopP != nil {
		responseReq.TopP = ptrOf(float64(*options.TopP))
	}
	if cm.serviceTier != nil {
		switch *cm.serviceTier {
		case "auto":
			responseReq.ServiceTier = responses.ResponsesServiceTier_auto.Enum()
		case "default":
			responseReq.ServiceTier = responses.ResponsesServiceTier_default.Enum()
		}
	}

	if specOptions.thinking != nil {
		var respThinking *responses.ResponsesThinking
		switch specOptions.thinking.Type {
		case arkModel.ThinkingTypeEnabled:
			respThinking = &responses.ResponsesThinking{
				Type: responses.ThinkingType_enabled.Enum(),
			}
		case arkModel.ThinkingTypeDisabled:
			respThinking = &responses.ResponsesThinking{
				Type: responses.ThinkingType_disabled.Enum(),
			}
		case arkModel.ThinkingTypeAuto:
			respThinking = &responses.ResponsesThinking{
				Type: responses.ThinkingType_auto.Enum(),
			}
		}
		responseReq.Thinking = respThinking
	}

	if specOptions.reasoningEffort != nil {
		var reasoning *responses.ResponsesReasoning
		switch *specOptions.reasoningEffort {
		case arkModel.ReasoningEffortMinimal:
			reasoning = &responses.ResponsesReasoning{
				Effort: responses.ReasoningEffort_minimal,
			}
		case arkModel.ReasoningEffortLow:
			reasoning = &responses.ResponsesReasoning{
				Effort: responses.ReasoningEffort_low,
			}
		case arkModel.ReasoningEffortMedium:
			reasoning = &responses.ResponsesReasoning{
				Effort: responses.ReasoningEffort_medium,
			}
		case arkModel.ReasoningEffortHigh:
			reasoning = &responses.ResponsesReasoning{
				Effort: responses.ReasoningEffort_high,
			}
		}
		responseReq.Reasoning = reasoning

	}

	return nil
}

func (cm *responsesAPIChatModel) genRequestAndOptions(in []*schema.Message, options *model.Options,
	specOptions *arkOptions) (responseReq *responses.ResponsesRequest, err error) {
	responseReq = &responses.ResponsesRequest{}

	err = cm.prePopulateConfig(responseReq, options, specOptions)
	if err != nil {
		return nil, err
	}
	in, err = cm.populateCache(in, responseReq, specOptions)
	if err != nil {
		return nil, err
	}

	err = cm.populateInput(in, responseReq)
	if err != nil {
		return nil, err
	}

	err = cm.populateTools(responseReq, options)
	if err != nil {
		return nil, err
	}

	return responseReq, nil

}

func (cm *responsesAPIChatModel) populateCache(in []*schema.Message, responseReq *responses.ResponsesRequest, arkOpts *arkOptions,
) ([]*schema.Message, error) {

	var (
		store       = false
		cacheStatus = cachingDisabled
		cacheTTL    *int
		headRespID  *string
		contextID   *string
	)

	if cm.cache != nil {
		if sCache := cm.cache.SessionCache; sCache != nil {
			if sCache.EnableCache {
				store = true
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
				store = true
				cacheStatus = cachingEnabled
			} else {
				store = false
				cacheStatus = cachingDisabled
			}
		}
	}

	var (
		preRespID *string
		inputIdx  int
	)

	now := time.Now().Unix()

	// If the user implements session caching with ContextID,
	// ContextID and ResponseID will exist at the same time.
	// Using ContextID is prioritized to maintain compatibility with the old logic.
	// In this usage scenario, ResponseID cannot be used.
	if cacheStatus == cachingEnabled && contextID == nil {
		for i := len(in) - 1; i >= 0; i-- {
			msg := in[i]
			inputIdx = i
			if expireAtSec, ok := GetCacheExpiration(msg); !ok || expireAtSec < now {
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
			return in, fmt.Errorf("not found incremental input after ResponseID")
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

	responseReq.PreviousResponseId = preRespID
	responseReq.Store = &store

	if cacheTTL != nil {
		responseReq.ExpireAt = ptrOf(now + int64(*cacheTTL))
	}

	var cacheType *responses.CacheType_Enum
	if cacheStatus == cachingDisabled {
		cacheType = responses.CacheType_disabled.Enum()
	} else {
		cacheType = responses.CacheType_enabled.Enum()
	}

	responseReq.Caching = &responses.ResponsesCaching{
		Type: cacheType,
	}

	return in, nil
}

func (cm *responsesAPIChatModel) populateInput(in []*schema.Message, responseReq *responses.ResponsesRequest) error {
	itemList := make([]*responses.InputItem, 0, len(in))
	if len(in) == 0 {
		return nil
	}
	for _, msg := range in {
		switch msg.Role {
		case schema.User:
			inputMessage, err := cm.toArkUserRoleItemInputMessage(msg)
			if err != nil {
				return err
			}
			itemList = append(itemList, &responses.InputItem{Union: &responses.InputItem_InputMessage{InputMessage: inputMessage}})
		case schema.Assistant:
			inputMessage, err := cm.toArkAssistantRoleItemInputMessage(msg)
			if err != nil {
				return err
			}
			if len(inputMessage.GetContent()) > 0 {
				itemList = append(itemList, &responses.InputItem{Union: &responses.InputItem_InputMessage{InputMessage: inputMessage}})
			}

			for _, toolCall := range msg.ToolCalls {
				itemList = append(itemList, &responses.InputItem{Union: &responses.InputItem_FunctionToolCall{
					FunctionToolCall: &responses.ItemFunctionToolCall{
						Type:      responses.ItemType_function_call,
						CallId:    toolCall.ID,
						Arguments: toolCall.Function.Arguments,
						Name:      toolCall.Function.Name,
					},
				}})
			}
		case schema.System:
			inputMessage, err := cm.toArkSystemRoleItemInputMessage(msg)
			if err != nil {
				return err
			}

			itemList = append(itemList, &responses.InputItem{Union: &responses.InputItem_InputMessage{InputMessage: inputMessage}})
		case schema.Tool:
			if len(msg.UserInputMultiContent) > 0 {
				return fmt.Errorf("ark response api doesn't support multi modal tool result")
			}
			itemList = append(itemList, &responses.InputItem{Union: &responses.InputItem_FunctionToolCallOutput{
				FunctionToolCallOutput: &responses.ItemFunctionToolCallOutput{
					Type:   responses.ItemType_function_call_output,
					CallId: msg.ToolCallID,
					Output: msg.Content,
				},
			}})

		default:
			return fmt.Errorf("unknown role: %s", msg.Role)
		}
	}
	responseReq.Input = &responses.ResponsesInput{
		Union: &responses.ResponsesInput_ListValue{
			ListValue: &responses.InputItemList{
				ListValue: itemList,
			},
		},
	}
	return nil
}

func (cm *responsesAPIChatModel) populateTools(responseReq *responses.ResponsesRequest, options *model.Options) error {
	if responseReq.PreviousResponseId != nil {
		return nil
	}
	tools := cm.tools
	if options.Tools != nil {
		var err error
		if tools, err = cm.toTools(options.Tools); err != nil {
			return err
		}
	}
	responseReq.Tools = tools

	err := populateResponseAPIToolChoice(responseReq, options.ToolChoice, options.AllowedToolNames)
	if err != nil {
		return err
	}

	return nil
}

func (cm *responsesAPIChatModel) toArkUserRoleItemInputMessage(msg *schema.Message) (*responses.ItemInputMessage, error) {
	inputItemMessage := &responses.ItemInputMessage{
		Type: responses.ItemType_message.Enum(),
		Role: responses.MessageRole_user,
	}

	if len(msg.AssistantGenMultiContent) > 0 {
		return nil, fmt.Errorf("if user role, AssistantGenMultiContent cannot be set")
	}
	if len(msg.UserInputMultiContent) > 0 {
		items, err := convUserInputMultiContentToContentItems(msg.UserInputMultiContent)
		if err != nil {
			return nil, err
		}
		inputItemMessage.Content = append(inputItemMessage.Content, items...)
		return inputItemMessage, nil
	}

	if msg.Content != "" {
		inputItemMessage.Content = append(inputItemMessage.Content, &responses.ContentItem{Union: &responses.ContentItem_Text{
			Text: &responses.ContentItemText{
				Type: responses.ContentItemType_input_text,
				Text: msg.Content,
			},
		}})
		return inputItemMessage, nil
	}

	if len(msg.MultiContent) > 0 {
		items, err := convMultiContentToContentItems(msg.MultiContent)
		if err != nil {
			return nil, err
		}
		inputItemMessage.Content = append(inputItemMessage.Content, items...)

		return inputItemMessage, nil
	}

	return nil, fmt.Errorf("user role message content is empty")

}

func (cm *responsesAPIChatModel) toArkAssistantRoleItemInputMessage(msg *schema.Message) (*responses.ItemInputMessage, error) {
	inputItemMessage := &responses.ItemInputMessage{
		Type: responses.ItemType_message.Enum(),
		Role: responses.MessageRole_assistant,
	}

	if len(msg.UserInputMultiContent) > 0 {
		return nil, fmt.Errorf("if assistant role, UserInputMultiContent cannot be set")
	}

	if len(msg.AssistantGenMultiContent) > 0 {
		for _, part := range msg.AssistantGenMultiContent {
			if part.Type != schema.ChatMessagePartTypeText {
				return inputItemMessage, fmt.Errorf("unsupported content type in AssistantGenMultiContent: %s", part.Type)
			}
			inputItemMessage.Content = append(inputItemMessage.Content, &responses.ContentItem{Union: &responses.ContentItem_Text{
				Text: &responses.ContentItemText{
					Type: responses.ContentItemType_input_text,
					Text: part.Text,
				},
			}})
		}
		return inputItemMessage, nil
	}

	if msg.Content != "" {
		inputItemMessage.Content = append(inputItemMessage.Content, &responses.ContentItem{Union: &responses.ContentItem_Text{
			Text: &responses.ContentItemText{
				Type: responses.ContentItemType_input_text,
				Text: msg.Content,
			},
		}})
		return inputItemMessage, nil
	}

	if len(msg.MultiContent) > 0 {
		for _, c := range msg.MultiContent {
			if c.Type != schema.ChatMessagePartTypeText {
				return inputItemMessage, fmt.Errorf("unsupported content type: %s", c.Type)
			}
			inputItemMessage.Content = append(inputItemMessage.Content, &responses.ContentItem{Union: &responses.ContentItem_Text{
				Text: &responses.ContentItemText{
					Type: responses.ContentItemType_input_text,
					Text: c.Text,
				},
			}})
		}
		return inputItemMessage, nil
	}

	return inputItemMessage, nil

}

func (cm *responsesAPIChatModel) toArkSystemRoleItemInputMessage(msg *schema.Message) (*responses.ItemInputMessage, error) {
	inputItemMessage := &responses.ItemInputMessage{
		Type: responses.ItemType_message.Enum(),
		Role: responses.MessageRole_system,
	}

	if len(msg.AssistantGenMultiContent) > 0 {
		return nil, fmt.Errorf("if system role, AssistantGenMultiContent cannot be set")
	}

	if msg.Content != "" {
		inputItemMessage.Content = append(inputItemMessage.Content, &responses.ContentItem{Union: &responses.ContentItem_Text{
			Text: &responses.ContentItemText{
				Type: responses.ContentItemType_input_text,
				Text: msg.Content,
			},
		}})
		return inputItemMessage, nil
	}

	if len(msg.UserInputMultiContent) > 0 {
		items, err := convUserInputMultiContentToContentItems(msg.UserInputMultiContent)
		if err != nil {
			return nil, err
		}
		inputItemMessage.Content = append(inputItemMessage.Content, items...)
		return inputItemMessage, nil
	}

	if len(msg.MultiContent) > 0 {
		items, err := convMultiContentToContentItems(msg.MultiContent)
		if err != nil {
			return nil, err
		}

		inputItemMessage.Content = append(inputItemMessage.Content, items...)
		return inputItemMessage, nil
	}

	return nil, fmt.Errorf("system role message content is empty")

}

func (cm *responsesAPIChatModel) getOptions(opts []model.Option) (*model.Options, *arkOptions, error) {
	options := model.GetCommonOptions(&model.Options{
		Temperature: cm.temperature,
		MaxTokens:   cm.maxTokens,
		Model:       &cm.model,
		TopP:        cm.topP,
		ToolChoice:  cm.toolChoice,
	}, opts...)

	arkOpts := model.GetImplSpecificOptions(&arkOptions{
		customHeaders:   cm.customHeader,
		thinking:        cm.thinking,
		reasoningEffort: cm.reasoningEffort,
	}, opts...)

	if err := cm.checkOptions(options, arkOpts); err != nil {
		return nil, nil, err
	}
	return options, arkOpts, nil
}

func (cm *responsesAPIChatModel) toTools(tis []*schema.ToolInfo) ([]*responses.ResponsesTool, error) {
	tools := make([]*responses.ResponsesTool, len(tis))
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

		tools[i] = &responses.ResponsesTool{
			Union: &responses.ResponsesTool_ToolFunction{
				ToolFunction: &responses.ToolFunction{
					Name:        ti.Name,
					Type:        responses.ToolType_function,
					Description: &ti.Desc,
					Parameters: &responses.Bytes{
						Value: b,
					},
				},
			},
		}
	}

	return tools, nil
}

func (cm *responsesAPIChatModel) toOutputMessage(resp *responses.ResponseObject, cache *cacheConfig) (*schema.Message, error) {
	msg := &schema.Message{
		Role: schema.Assistant,
		ResponseMeta: &schema.ResponseMeta{
			FinishReason: resp.Status.String(),
			Usage:        cm.toEinoTokenUsage(resp.Usage),
		},
	}

	if cache != nil && cache.Enabled {
		setResponseCacheExpireAt(msg, arkResponseCacheExpireAt(ptrFromOrZero(cache.ExpireAt)))
	}
	setContextID(msg, resp.Id)
	setResponseID(msg, resp.Id)

	if resp.ServiceTier != nil {
		setServiceTier(msg, resp.ServiceTier.String())
	}

	if resp.Status == responses.ResponseStatus_failed {
		msg.ResponseMeta.FinishReason = resp.Error.Message
		return msg, nil
	}

	if resp.Status == responses.ResponseStatus_incomplete {
		msg.ResponseMeta.FinishReason = resp.IncompleteDetails.Reason
		return msg, nil
	}

	if len(resp.Output) == 0 {
		return nil, fmt.Errorf("received empty output from ARK")
	}

	for _, item := range resp.Output {
		switch asItem := item.GetUnion().(type) {
		case *responses.OutputItem_OutputMessage:
			if asItem.OutputMessage == nil {
				continue
			}
			isMultiContent := len(asItem.OutputMessage.Content) > 1
			for _, content := range asItem.OutputMessage.Content {
				if content.GetText() == nil {
					continue
				}
				if !isMultiContent {
					msg.Content = content.GetText().GetText()
				} else {
					msg.AssistantGenMultiContent = append(msg.AssistantGenMultiContent, schema.MessageOutputPart{
						Type: schema.ChatMessagePartTypeText,
						Text: content.GetText().GetText(),
					})
				}
			}

		case *responses.OutputItem_Reasoning:
			if asItem.Reasoning == nil {
				continue
			}
			for _, s := range asItem.Reasoning.GetSummary() {
				if s.Text == "" {
					continue
				}
				if msg.ReasoningContent == "" {
					msg.ReasoningContent = s.Text
					continue
				}
				msg.ReasoningContent = fmt.Sprintf("%s\n\n%s", msg.ReasoningContent, s.Text)
			}

		case *responses.OutputItem_FunctionToolCall:
			if asItem.FunctionToolCall == nil {
				continue
			}
			msg.ToolCalls = append(msg.ToolCalls, schema.ToolCall{
				ID:   asItem.FunctionToolCall.CallId,
				Type: asItem.FunctionToolCall.Type.String(),
				Function: schema.FunctionCall{
					Name:      asItem.FunctionToolCall.Name,
					Arguments: asItem.FunctionToolCall.Arguments,
				},
			})
		}
	}

	return msg, nil
}

func (cm *responsesAPIChatModel) toEinoTokenUsage(usage *responses.Usage) *schema.TokenUsage {
	tokenUsage := &schema.TokenUsage{
		PromptTokens:     int(usage.InputTokens),
		CompletionTokens: int(usage.OutputTokens),
		TotalTokens:      int(usage.TotalTokens),
	}
	if usage.InputTokensDetails != nil {
		tokenUsage.PromptTokenDetails.CachedTokens = int(usage.InputTokensDetails.CachedTokens)
	}

	if usage.OutputTokensDetails != nil {
		tokenUsage.CompletionTokensDetails.ReasoningTokens = int(usage.OutputTokensDetails.ReasoningTokens)
	}
	return tokenUsage
}

func (cm *responsesAPIChatModel) toModelTokenUsage(usage *responses.Usage) *model.TokenUsage {
	tokenUsage := &model.TokenUsage{
		PromptTokens:     int(usage.InputTokens),
		CompletionTokens: int(usage.OutputTokens),
		TotalTokens:      int(usage.TotalTokens),
	}
	if usage.InputTokensDetails != nil {
		tokenUsage.PromptTokenDetails.CachedTokens = int(usage.InputTokensDetails.CachedTokens)
	}

	if usage.OutputTokensDetails != nil {
		tokenUsage.CompletionTokensDetails.ReasoningTokens = int(usage.OutputTokensDetails.ReasoningTokens)
	}

	return tokenUsage
}

func (cm *responsesAPIChatModel) checkOptions(mOpts *model.Options, _ *arkOptions) error {
	if len(mOpts.Stop) > 0 {
		return fmt.Errorf("'Stop' is not supported by responses API")
	}
	return nil
}

func (cm *responsesAPIChatModel) toCallbackConfig(req *responses.ResponsesRequest) *model.Config {
	return &model.Config{
		Model:       req.Model,
		MaxTokens:   int(ptrFromOrZero(req.MaxOutputTokens)),
		Temperature: float32(ptrFromOrZero(req.Temperature)),
		TopP:        float32(ptrFromOrZero(req.TopP)),
	}
}

func (cm *responsesAPIChatModel) receivedStreamResponse(streamReader *utils.ResponsesStreamReader,
	config *model.Config, cacheConfig *cacheConfig, sw *schema.StreamWriter[*model.CallbackOutput]) {
	var itemFunctionToolCall *responses.ItemFunctionToolCall

	for {
		event, err := streamReader.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			_ = sw.Send(nil, fmt.Errorf("failed to read stream: %w", err))
			return
		}

		switch ev := event.GetEvent().(type) {
		case *responses.Event_Response:
			if ev.Response == nil || ev.Response.Response == nil {
				continue
			}
			msg := &schema.Message{Role: schema.Assistant}
			cm.setStreamChunkDefaultExtra(msg, ev.Response.Response, cacheConfig)
			cm.sendCallbackOutput(sw, config, ev.Response.Response.Model, msg)

		case *responses.Event_ResponseCompleted:
			if ev.ResponseCompleted == nil || ev.ResponseCompleted.Response == nil {
				continue
			}
			msg := cm.handleCompletedStreamEvent(ev.ResponseCompleted.Response)
			cm.setStreamChunkDefaultExtra(msg, ev.ResponseCompleted.Response, cacheConfig)
			cm.sendCallbackOutput(sw, config, ev.ResponseCompleted.Response.Model, msg)

		case *responses.Event_Error:
			sw.Send(nil, fmt.Errorf("received error: %s", ev.Error.Message))

		case *responses.Event_ResponseIncomplete:
			if ev.ResponseIncomplete == nil || ev.ResponseIncomplete.Response == nil || ev.ResponseIncomplete.Response.IncompleteDetails == nil {
				continue
			}
			detail := ev.ResponseIncomplete.Response.IncompleteDetails.Reason
			msg := &schema.Message{
				Role: schema.Assistant,
				ResponseMeta: &schema.ResponseMeta{
					FinishReason: detail,
					Usage:        cm.toEinoTokenUsage(ev.ResponseIncomplete.Response.Usage),
				},
			}
			cm.setStreamChunkDefaultExtra(msg, ev.ResponseIncomplete.Response, cacheConfig)
			cm.sendCallbackOutput(sw, config, ev.ResponseIncomplete.Response.Model, msg)

		case *responses.Event_ResponseFailed:
			if ev.ResponseFailed == nil || ev.ResponseFailed.Response == nil {
				continue
			}
			var errorMessage string
			if ev.ResponseFailed.Response.Error != nil {
				errorMessage = ev.ResponseFailed.Response.Error.Message
			}
			msg := &schema.Message{
				Role: schema.Assistant,
				ResponseMeta: &schema.ResponseMeta{
					FinishReason: errorMessage,
					Usage:        cm.toEinoTokenUsage(ev.ResponseFailed.Response.Usage),
				},
			}
			cm.setStreamChunkDefaultExtra(msg, ev.ResponseFailed.Response, cacheConfig)
			cm.sendCallbackOutput(sw, config, ev.ResponseFailed.Response.Model, msg)

		case *responses.Event_Item:
			if ev.Item == nil || ev.Item.GetItem() == nil || ev.Item.GetItem().GetUnion() == nil {
				continue
			}
			if outputItemFuncCall, ok := ev.Item.GetItem().GetUnion().(*responses.OutputItem_FunctionToolCall); ok {
				itemFunctionToolCall = outputItemFuncCall.FunctionToolCall
			}

		case *responses.Event_FunctionCallArguments:
			if ev.FunctionCallArguments == nil {
				continue
			}

			delta := *ev.FunctionCallArguments.Delta
			outputIndex := ev.FunctionCallArguments.OutputIndex

			if itemFunctionToolCall != nil && itemFunctionToolCall.Id != nil && *itemFunctionToolCall.Id == ev.FunctionCallArguments.ItemId {
				msg := &schema.Message{
					Role: schema.Assistant,
					ToolCalls: []schema.ToolCall{
						{
							Index: ptrOf(int(outputIndex)),
							ID:    itemFunctionToolCall.CallId,
							Type:  itemFunctionToolCall.Type.String(),
							Function: schema.FunctionCall{
								Name:      itemFunctionToolCall.Name,
								Arguments: delta,
							},
						},
					},
				}
				cm.sendCallbackOutput(sw, config, "", msg)
			}

		case *responses.Event_ReasoningText:
			if ev.ReasoningText == nil || ev.ReasoningText.Delta == nil {
				continue
			}
			delta := *ev.ReasoningText.Delta
			msg := &schema.Message{
				Role:             schema.Assistant,
				ReasoningContent: delta,
			}
			setReasoningContent(msg, delta)
			cm.sendCallbackOutput(sw, config, "", msg)

		case *responses.Event_Text:
			if ev.Text == nil || ev.Text.Delta == nil {
				continue
			}
			msg := &schema.Message{
				Role:    schema.Assistant,
				Content: *ev.Text.Delta,
			}
			cm.sendCallbackOutput(sw, config, "", msg)

		}

	}

}

func (cm *responsesAPIChatModel) setStreamChunkDefaultExtra(msg *schema.Message, object *responses.ResponseObject,
	cacheConfig *cacheConfig) {

	if cacheConfig.Enabled {
		setResponseCacheExpireAt(msg, arkResponseCacheExpireAt(ptrFromOrZero(cacheConfig.ExpireAt)))
	}
	setContextID(msg, object.Id)
	setResponseID(msg, object.Id)
	if object.ServiceTier != nil {
		setServiceTier(msg, object.ServiceTier.String())
	}

}

func (cm *responsesAPIChatModel) sendCallbackOutput(sw *schema.StreamWriter[*model.CallbackOutput], reqConf *model.Config, modelName string,
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

	var extra map[string]any
	if len(modelName) > 0 {
		extra = map[string]any{
			callbackExtraModelName: modelName,
		}
	}

	sw.Send(&model.CallbackOutput{
		Message:    msg,
		Config:     reqConf,
		TokenUsage: token,
		Extra:      extra,
	}, nil)
}

func (cm *responsesAPIChatModel) handleCompletedStreamEvent(RespObject *responses.ResponseObject) *schema.Message {
	return &schema.Message{
		Role: schema.Assistant,
		ResponseMeta: &schema.ResponseMeta{
			FinishReason: RespObject.Status.String(),
			Usage:        cm.toEinoTokenUsage(RespObject.Usage),
		},
	}
}

func convChatMsgImageURLToResponseContentItem(image *schema.ChatMessageImageURL) (*responses.ContentItem, error) {
	contentItemImage := &responses.ContentItemImage{
		Type:     responses.ContentItemType_input_image,
		ImageUrl: &image.URL,
	}
	err := toContentItemImageDetail(contentItemImage, image.Detail)
	if err != nil {
		return nil, err
	}
	return &responses.ContentItem{
		Union: &responses.ContentItem_Image{Image: contentItemImage}}, nil
}
func isDataBase64Schema(url string) bool {
	if strings.HasPrefix(url, "data:") {
		return true
	}
	return false

}
func convChatMsgFileURLToResponseContentItem(file *schema.ChatMessageFileURL) (*responses.ContentItem, error) {
	if file.URL == "" {
		return nil, fmt.Errorf("file url is empty")
	}
	if isDataBase64Schema(file.URL) {
		if file.Name == "" {
			return nil, fmt.Errorf("the 'name' field is required for 'file_url' parts when the 'data' URL Schema is provided")
		}
		return &responses.ContentItem{
			Union: &responses.ContentItem_File{
				File: &responses.ContentItemFile{
					Type:     responses.ContentItemType_input_file,
					FileData: &file.URL,
					Filename: &file.Name,
				},
			}}, nil
	}

	return &responses.ContentItem{
		Union: &responses.ContentItem_File{
			File: &responses.ContentItemFile{
				Type:     responses.ContentItemType_input_file,
				FileUrl:  &file.URL,
				Filename: &file.Name,
			},
		}}, nil

}

func convMultiContentToContentItems(parts []schema.ChatMessagePart) ([]*responses.ContentItem, error) {
	items := make([]*responses.ContentItem, 0, len(parts))
	for _, c := range parts {
		switch c.Type {
		case schema.ChatMessagePartTypeText:
			items = append(items, &responses.ContentItem{Union: &responses.ContentItem_Text{
				Text: &responses.ContentItemText{
					Type: responses.ContentItemType_input_text,
					Text: c.Text,
				},
			}})

		case schema.ChatMessagePartTypeImageURL:
			if c.ImageURL == nil {
				continue
			}
			item, err := convChatMsgImageURLToResponseContentItem(c.ImageURL)
			if err != nil {
				return nil, err
			}

			items = append(items, item)
		case schema.ChatMessagePartTypeFileURL:
			if c.FileURL == nil {
				continue
			}

			item, err := convChatMsgFileURLToResponseContentItem(c.FileURL)
			if err != nil {
				return nil, err
			}
			items = append(items, item)
		default:
			return nil, fmt.Errorf("unsupported content type: %s", c.Type)
		}
	}
	return items, nil
}
func convUserInputMultiContentToContentItems(parts []schema.MessageInputPart) ([]*responses.ContentItem, error) {
	items := make([]*responses.ContentItem, 0, len(parts))
	for _, part := range parts {
		switch part.Type {
		case schema.ChatMessagePartTypeText:
			items = append(items, &responses.ContentItem{Union: &responses.ContentItem_Text{
				Text: &responses.ContentItemText{
					Type: responses.ContentItemType_input_text,
					Text: part.Text,
				},
			}})
		case schema.ChatMessagePartTypeImageURL:
			if part.Image == nil {
				return nil, fmt.Errorf("image field must not be nil when type is 'ChatMessagePartTypeImageURL' in user message")
			}

			item, err := convMsgInputImageToResponseContentItem(part.Image)
			if err != nil {
				return nil, err
			}
			items = append(items, item)
		case schema.ChatMessagePartTypeVideoURL:
			if part.Video == nil {
				return nil, fmt.Errorf("video field must not be nil when type is 'ChatMessagePartTypeVideoURL'")
			}
			item, err := convMsgInputVideoToResponseContentItem(part.Video)
			if err != nil {
				return nil, err
			}

			items = append(items, item)
		case schema.ChatMessagePartTypeFileURL:
			if part.File == nil {
				return nil, fmt.Errorf("file field must not be nil when Type is ChatMessagePartTypeFileURL")
			}

			item, err := convMsgInputFileToResponseContentItem(part.File)
			if err != nil {
				return nil, err
			}
			items = append(items, item)

		default:
			return nil, fmt.Errorf("unsupported content type in UserInputMultiContent: %s", part.Type)
		}
	}
	return items, nil
}
func convMsgInputImageToResponseContentItem(image *schema.MessageInputImage) (*responses.ContentItem, error) {
	imageURL, err := convMessagePartCommonToURL(image.MessagePartCommon)
	if err != nil {
		return nil, fmt.Errorf("convert message input image failed err: %w", err)
	}
	contentItemImage := &responses.ContentItemImage{
		Type:     responses.ContentItemType_input_image,
		ImageUrl: &imageURL,
	}
	err = toContentItemImageDetail(contentItemImage, image.Detail)
	if err != nil {
		return nil, err
	}
	return &responses.ContentItem{
		Union: &responses.ContentItem_Image{
			Image: contentItemImage,
		},
	}, nil
}

func convMsgInputVideoToResponseContentItem(video *schema.MessageInputVideo) (*responses.ContentItem, error) {
	videoURL, err := convMessagePartCommonToURL(video.MessagePartCommon)
	if err != nil {
		return nil, fmt.Errorf("convert message input video failed err: %w", err)
	}

	contentItemVideo := &responses.ContentItemVideo{
		Type:     responses.ContentItemType_input_video,
		VideoUrl: videoURL,
	}

	var fps *float32
	if GetInputVideoFPS(video) != nil {
		fps = ptrOf(float32(*GetInputVideoFPS(video)))
	}
	contentItemVideo.Fps = fps

	return &responses.ContentItem{
		Union: &responses.ContentItem_Video{
			Video: contentItemVideo,
		},
	}, nil

}

func convMsgInputFileToResponseContentItem(file *schema.MessageInputFile) (*responses.ContentItem, error) {
	var (
		fileURL  string
		err      error
		fileName *string
	)
	if file.Name != "" {
		fileName = &file.Name
	}

	fileURL, err = convMessagePartCommonToURL(file.MessagePartCommon)
	if err != nil {
		return nil, fmt.Errorf("convert message input file failed err: %w", err)
	}
	var contentItemFile *responses.ContentItemFile

	if file.URL != nil {
		contentItemFile = &responses.ContentItemFile{
			Type:     responses.ContentItemType_input_file,
			FileUrl:  &fileURL,
			Filename: fileName,
		}
	} else if file.Base64Data != nil {
		contentItemFile = &responses.ContentItemFile{
			Type:     responses.ContentItemType_input_file,
			FileData: &fileURL,
			Filename: fileName,
		}
	}

	return &responses.ContentItem{
		Union: &responses.ContentItem_File{
			File: contentItemFile,
		},
	}, nil

}

func convMessagePartCommonToURL(common schema.MessagePartCommon) (url string, err error) {
	if common.URL == nil && common.Base64Data == nil {
		return "", fmt.Errorf("message part must have URL or Base64Data field")
	}

	if common.URL != nil {
		return *common.URL, nil
	}

	if common.MIMEType == "" {
		return "", fmt.Errorf("message part must have MIMEType when use Base64Data")
	}
	url, err = ensureDataURL(*common.Base64Data, common.MIMEType)
	if err != nil {
		return "", err
	}

	return url, nil
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

func toContentItemImageDetail(cImage *responses.ContentItemImage, detail schema.ImageURLDetail) error {
	if len(detail) == 0 {
		return nil
	}
	switch detail {
	case schema.ImageURLDetailHigh:
		cImage.Detail = responses.ContentItemImageDetail_high.Enum()
	case schema.ImageURLDetailLow:
		cImage.Detail = responses.ContentItemImageDetail_low.Enum()
	case schema.ImageURLDetailAuto:
		cImage.Detail = responses.ContentItemImageDetail_auto.Enum()
	default:
		return fmt.Errorf("unknown image detail %v", detail)
	}
	return nil
}

func populateResponseAPIToolChoice(responseReq *responses.ResponsesRequest, tc *schema.ToolChoice, allowedToolNames []string) error {

	if tc == nil {
		return nil
	}

	var mode responses.ToolChoiceMode_Enum
	switch *tc {
	case schema.ToolChoiceForbidden:
		mode = responses.ToolChoiceMode_none
	case schema.ToolChoiceAllowed:
		mode = responses.ToolChoiceMode_auto
	case schema.ToolChoiceForced:
		mode = responses.ToolChoiceMode_required
	default:
		mode = responses.ToolChoiceMode_auto
	}

	if mode == responses.ToolChoiceMode_required && len(responseReq.Tools) == 0 {
		return fmt.Errorf("tool_choice is forced but no tools are provided")
	}

	if mode == responses.ToolChoiceMode_required {
		var onlyOneToolName string
		if len(allowedToolNames) > 0 {
			if len(allowedToolNames) > 1 {
				return fmt.Errorf("only one allowed tool name can be configured")
			}

			allowedToolName := allowedToolNames[0]
			toolsMap := make(map[string]bool, len(responseReq.Tools))
			for _, t := range responseReq.Tools {
				if t.GetToolFunction() != nil {
					toolsMap[t.GetToolFunction().Name] = true
				}
			}

			if _, ok := toolsMap[allowedToolName]; !ok {
				return fmt.Errorf("allowed tool name '%s' not found in tools list", allowedToolName)
			}

			onlyOneToolName = allowedToolName
		} else if len(responseReq.Tools) == 1 && responseReq.Tools[0].GetToolFunction() != nil {
			onlyOneToolName = responseReq.Tools[0].GetToolFunction().GetName()
		}

		if onlyOneToolName != "" {
			responseReq.ToolChoice = &responses.ResponsesToolChoice{
				Union: &responses.ResponsesToolChoice_FunctionToolChoice{
					FunctionToolChoice: &responses.FunctionToolChoice{
						Type: responses.ToolType_function,
						Name: onlyOneToolName,
					},
				},
			}
			return nil
		}
	}

	responseReq.ToolChoice = &responses.ResponsesToolChoice{
		Union: &responses.ResponsesToolChoice_Mode{
			Mode: mode,
		},
	}

	return nil

}

func (cm *responsesAPIChatModel) createPrefixCacheByResponseAPI(ctx context.Context, prefix []*schema.Message, ttl int, opts ...model.Option) (info *CacheInfo, err error) {
	responseReq := &responses.ResponsesRequest{
		Model:    cm.model,
		ExpireAt: ptrOf(time.Now().Unix() + int64(ttl)),
		Store:    ptrOf(true),
		Caching: &responses.ResponsesCaching{
			Type:   responses.CacheType_enabled.Enum(),
			Prefix: ptrOf(true),
		},
	}

	options, specOptions, err := cm.getOptions(opts)
	if err != nil {
		return nil, err
	}

	err = cm.prePopulateConfig(responseReq, options, specOptions)
	if err != nil {
		return nil, err
	}
	err = cm.populateInput(prefix, responseReq)
	if err != nil {
		return nil, err
	}

	err = cm.populateTools(responseReq, options)
	if err != nil {
		return nil, err
	}

	responseObject, err := cm.client.CreateResponses(ctx, responseReq)
	if err != nil {
		return nil, err
	}

	info = &CacheInfo{
		ResponseID: responseObject.Id,
		Usage:      *cm.toEinoTokenUsage(responseObject.Usage),
	}

	return info, nil
}

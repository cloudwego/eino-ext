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

package openaigo

import (
	"fmt"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"
)

func (cm *ChatModel) buildParams(in []*schema.Message, stream bool, opts ...model.Option) (responses.ResponseNewParams, *model.CallbackInput, error) {
	common := model.GetCommonOptions(&model.Options{
		Temperature: cm.temperature,
		MaxTokens: func() *int {
			// Responses API uses MaxOutputTokens; keep MaxTokens in common opts unused.
			return nil
		}(),
		Model:      &cm.model,
		TopP:       cm.topP,
		Tools:      cm.rawTools,
		ToolChoice: cm.toolChoice,
	}, opts...)

	spec := model.GetImplSpecificOptions(&options{
		MaxOutputTokens: cm.maxOutTok,
		Reasoning:       cm.reasoning,
		Store:           cm.store,
		Metadata:        cm.metadata,
		ExtraFields:     cm.extraFields,
	}, opts...)

	params := responses.ResponseNewParams{}
	if common.Model != nil {
		params.Model = responsesModelFromString(*common.Model)
	}
	if spec.MaxOutputTokens != nil {
		params.MaxOutputTokens = openai.Int(int64(*spec.MaxOutputTokens))
	}
	if common.Temperature != nil {
		params.Temperature = openai.Float(float64(*common.Temperature))
	}
	if common.TopP != nil {
		params.TopP = openai.Float(float64(*common.TopP))
	}
	if spec.Store != nil {
		params.Store = openai.Bool(*spec.Store)
	}
	if len(spec.Metadata) > 0 {
		params.Metadata = spec.Metadata
	}
	if spec.Reasoning != nil {
		params.Reasoning = spec.Reasoning.toSDK()
	}
	if stream {
		params.StreamOptions = responses.ResponseNewParamsStreamOptions{IncludeObfuscation: openai.Bool(false)}
	}

	// Tools.
	tools := cm.tools
	cbTools := cm.rawTools
	if common.Tools != nil {
		var err error
		tools, cbTools, err = toOpenAITools(common.Tools)
		if err != nil {
			return responses.ResponseNewParams{}, nil, err
		}
	}
	if len(tools) > 0 {
		params.Tools = tools
	}

	if err := populateToolChoice(&params, common.ToolChoice, common.AllowedToolNames, tools); err != nil {
		return responses.ResponseNewParams{}, nil, err
	}

	// Input.
	inputItems, err := toInputItems(in)
	if err != nil {
		return responses.ResponseNewParams{}, nil, err
	}
	params.Input = responses.ResponseNewParamsInputUnion{OfInputItemList: inputItems}

	if len(spec.ExtraFields) > 0 {
		params.SetExtraFields(spec.ExtraFields)
	}

	cbIn := &model.CallbackInput{
		Messages:   in,
		Tools:      cbTools,
		ToolChoice: common.ToolChoice,
		Config: &model.Config{
			Model:       string(params.Model),
			MaxTokens:   int(optInt64(params.MaxOutputTokens)),
			Temperature: float32(optFloat64(params.Temperature)),
			TopP:        float32(optFloat64(params.TopP)),
		},
	}

	return params, cbIn, nil
}

func toInputItems(in []*schema.Message) (responses.ResponseInputParam, error) {
	items := make([]responses.ResponseInputItemUnionParam, 0, len(in))
	for _, msg := range in {
		if msg == nil {
			continue
		}
		switch msg.Role {
		case schema.User:
			content, err := toInputContentFromMessage(msg)
			if err != nil {
				return nil, err
			}
			items = append(items, responses.ResponseInputItemParamOfMessage(content, responses.EasyInputMessageRoleUser))
		case schema.System:
			content, err := toInputContentFromMessage(msg)
			if err != nil {
				return nil, err
			}
			items = append(items, responses.ResponseInputItemParamOfMessage(content, responses.EasyInputMessageRoleSystem))
		case schema.Assistant:
			assistantText, hasAssistantText, err := extractAssistantTextForHistory(msg)
			if err != nil {
				return nil, err
			}
			if hasAssistantText {
				items = append(items, responses.ResponseInputItemParamOfMessage(assistantText, responses.EasyInputMessageRoleAssistant))
			}

			// assistant tool calls
			for _, tc := range msg.ToolCalls {
				items = append(items, responses.ResponseInputItemParamOfFunctionCall(tc.Function.Arguments, tc.ID, tc.Function.Name))
			}
		case schema.Tool:
			// tool call output
			if msg.ToolCallID == "" {
				return nil, fmt.Errorf("tool message missing ToolCallID")
			}
			if len(msg.UserInputMultiContent) == 0 {
				items = append(items, responses.ResponseInputItemParamOfFunctionCallOutput(msg.ToolCallID, msg.Content))
				break
			}
			outItems := make([]responses.ResponseFunctionCallOutputItemUnionParam, 0, len(msg.UserInputMultiContent))
			for _, part := range msg.UserInputMultiContent {
				switch part.Type {
				case schema.ChatMessagePartTypeText:
					outItems = append(outItems, responses.ResponseFunctionCallOutputItemUnionParam{OfInputText: &responses.ResponseInputTextContentParam{Text: part.Text}})
				case schema.ChatMessagePartTypeImageURL:
					if part.Image == nil {
						return nil, fmt.Errorf("image field must not be nil in tool message")
					}
					url, err := commonToDataOrURL(part.Image.MessagePartCommon)
					if err != nil {
						return nil, err
					}
					outItems = append(outItems, responses.ResponseFunctionCallOutputItemUnionParam{OfInputImage: &responses.ResponseInputImageContentParam{ImageURL: openai.String(url)}})
				case schema.ChatMessagePartTypeFileURL:
					if part.File == nil {
						return nil, fmt.Errorf("file field must not be nil in tool message")
					}
					url, err := commonToDataOrURL(part.File.MessagePartCommon)
					if err != nil {
						return nil, err
					}
					p := &responses.ResponseInputFileContentParam{}
					if part.File.URL != nil {
						p.FileURL = openai.String(url)
					} else {
						p.FileData = openai.String(url)
					}
					if part.File.Name != "" {
						p.Filename = openai.String(part.File.Name)
					}
					outItems = append(outItems, responses.ResponseFunctionCallOutputItemUnionParam{OfInputFile: p})
				default:
					return nil, fmt.Errorf("unsupported tool output content type: %s", part.Type)
				}
			}
			items = append(items, responses.ResponseInputItemParamOfFunctionCallOutput(msg.ToolCallID, responses.ResponseFunctionCallOutputItemListParam(outItems)))
		default:
			return nil, fmt.Errorf("unknown role: %s", msg.Role)
		}
	}

	return items, nil
}

func toInputContentFromMessage(msg *schema.Message) (responses.ResponseInputMessageContentListParam, error) {
	if len(msg.UserInputMultiContent) > 0 && len(msg.AssistantGenMultiContent) > 0 {
		return nil, fmt.Errorf("a message cannot contain both UserInputMultiContent and AssistantGenMultiContent")
	}
	if len(msg.UserInputMultiContent) > 0 {
		parts := make([]responses.ResponseInputContentUnionParam, 0, len(msg.UserInputMultiContent))
		for _, part := range msg.UserInputMultiContent {
			p, err := toInputContentPartFromInputPart(part)
			if err != nil {
				return nil, err
			}
			parts = append(parts, p)
		}
		return responses.ResponseInputMessageContentListParam(parts), nil
	}
	if len(msg.AssistantGenMultiContent) > 0 {
		// For assistant messages, only text parts can be re-sent as input.
		parts := make([]responses.ResponseInputContentUnionParam, 0, len(msg.AssistantGenMultiContent))
		for _, part := range msg.AssistantGenMultiContent {
			if part.Type != schema.ChatMessagePartTypeText {
				return nil, fmt.Errorf("unsupported assistant output part type in re-input: %s", part.Type)
			}
			parts = append(parts, responses.ResponseInputContentUnionParam{OfInputText: &responses.ResponseInputTextParam{Text: part.Text}})
		}
		return responses.ResponseInputMessageContentListParam(parts), nil
	}

	// Backward compatible deprecated MultiContent.
	if len(msg.MultiContent) > 0 {
		parts := make([]responses.ResponseInputContentUnionParam, 0, len(msg.MultiContent))
		for _, c := range msg.MultiContent {
			switch c.Type {
			case schema.ChatMessagePartTypeText:
				parts = append(parts, responses.ResponseInputContentUnionParam{OfInputText: &responses.ResponseInputTextParam{Text: c.Text}})
			case schema.ChatMessagePartTypeImageURL:
				if c.ImageURL == nil {
					continue
				}
				parts = append(parts, responses.ResponseInputContentUnionParam{OfInputImage: &responses.ResponseInputImageParam{
					Detail: responses.ResponseInputImageDetailAuto,
					ImageURL: openai.String(func() string {
						if c.ImageURL.URI != "" {
							return c.ImageURL.URI
						}
						return c.ImageURL.URL
					}()),
				}})
			default:
				return nil, fmt.Errorf("unsupported deprecated MultiContent part type: %s", c.Type)
			}
		}
		return responses.ResponseInputMessageContentListParam(parts), nil
	}

	if msg.Content == "" {
		// allow empty content for assistant messages
		if msg.Role == schema.Assistant {
			return responses.ResponseInputMessageContentListParam([]responses.ResponseInputContentUnionParam{}), nil
		}
		return nil, fmt.Errorf("message content is empty")
	}
	return responses.ResponseInputMessageContentListParam([]responses.ResponseInputContentUnionParam{{
		OfInputText: &responses.ResponseInputTextParam{Text: msg.Content},
	}}), nil
}

func toInputContentPartFromInputPart(part schema.MessageInputPart) (responses.ResponseInputContentUnionParam, error) {
	switch part.Type {
	case schema.ChatMessagePartTypeText:
		return responses.ResponseInputContentUnionParam{OfInputText: &responses.ResponseInputTextParam{Text: part.Text}}, nil
	case schema.ChatMessagePartTypeImageURL:
		if part.Image == nil {
			return responses.ResponseInputContentUnionParam{}, fmt.Errorf("image field must not be nil when type is %s", part.Type)
		}
		url, err := commonToDataOrURL(part.Image.MessagePartCommon)
		if err != nil {
			return responses.ResponseInputContentUnionParam{}, err
		}
		return responses.ResponseInputContentUnionParam{OfInputImage: &responses.ResponseInputImageParam{
			Detail:   toSDKImageDetail(part.Image.Detail),
			ImageURL: openai.String(url),
		}}, nil
	case schema.ChatMessagePartTypeFileURL:
		if part.File == nil {
			return responses.ResponseInputContentUnionParam{}, fmt.Errorf("file field must not be nil when type is %s", part.Type)
		}
		fileURL, err := commonToDataOrURL(part.File.MessagePartCommon)
		if err != nil {
			return responses.ResponseInputContentUnionParam{}, err
		}
		fileParam := &responses.ResponseInputFileParam{}
		if part.File.URL != nil {
			fileParam.FileURL = openai.String(fileURL)
		} else if part.File.Base64Data != nil {
			fileParam.FileData = openai.String(fileURL)
		}
		if part.File.Name != "" {
			fileParam.Filename = openai.String(part.File.Name)
		}
		return responses.ResponseInputContentUnionParam{OfInputFile: fileParam}, nil
	default:
		return responses.ResponseInputContentUnionParam{}, fmt.Errorf("unsupported content type: %s", part.Type)
	}
}

// Deprecated: tool call outputs are constructed inline in toInputItems.
func toFunctionCallOutputFromToolMessage(_ *schema.Message) (any, error) {
	return nil, fmt.Errorf("deprecated")
}

func populateToolChoice(params *responses.ResponseNewParams, tc *schema.ToolChoice, allowedToolNames []string, tools []responses.ToolUnionParam) error {
	if tc == nil {
		return nil
	}

	switch *tc {
	case schema.ToolChoiceForbidden:
		params.ToolChoice = responses.ResponseNewParamsToolChoiceUnion{OfToolChoiceMode: openai.Opt(responses.ToolChoiceOptionsNone)}
		return nil
	case schema.ToolChoiceAllowed:
		params.ToolChoice = responses.ResponseNewParamsToolChoiceUnion{OfToolChoiceMode: openai.Opt(responses.ToolChoiceOptionsAuto)}
		return nil
	case schema.ToolChoiceForced:
		if len(tools) == 0 {
			return fmt.Errorf("tool_choice is forced but no tools are provided")
		}

		// If a single allowed tool is specified (or only one tool exists), force it.
		var onlyOneToolName string
		if len(allowedToolNames) > 0 {
			if len(allowedToolNames) > 1 {
				return fmt.Errorf("only one allowed tool name can be configured")
			}
			allowed := allowedToolNames[0]
			if !toolNameExists(tools, allowed) {
				return fmt.Errorf("allowed tool name '%s' not found in tools list", allowed)
			}
			onlyOneToolName = allowed
		} else if len(tools) == 1 {
			if tools[0].OfFunction != nil {
				onlyOneToolName = tools[0].OfFunction.Name
			}
		}

		if onlyOneToolName != "" {
			params.ToolChoice = responses.ResponseNewParamsToolChoiceUnion{OfFunctionTool: &responses.ToolChoiceFunctionParam{Name: onlyOneToolName}}
			return nil
		}

		params.ToolChoice = responses.ResponseNewParamsToolChoiceUnion{OfToolChoiceMode: openai.Opt(responses.ToolChoiceOptionsRequired)}
		return nil
	default:
		return fmt.Errorf("unknown tool choice: %s", *tc)
	}
}

func toOpenAITools(tis []*schema.ToolInfo) ([]responses.ToolUnionParam, []*schema.ToolInfo, error) {
	tools := make([]responses.ToolUnionParam, len(tis))
	rawTools := make([]*schema.ToolInfo, len(tis))
	copy(rawTools, tis)
	for i := range tis {
		ti := tis[i]
		if ti == nil {
			return nil, nil, fmt.Errorf("tool info cannot be nil")
		}
		paramsJSONSchema, err := ti.ParamsOneOf.ToJSONSchema()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to convert tool parameters to JSONSchema: %w", err)
		}
		paramsMap, err := jsonSchemaToMap(paramsJSONSchema)
		if err != nil {
			return nil, nil, err
		}
		enforceOpenAIStrictJSONSchema(paramsMap)
		t := responses.ToolUnionParam{OfFunction: &responses.FunctionToolParam{
			Name:        ti.Name,
			Description: openai.String(ti.Desc),
			Parameters:  paramsMap,
			Strict:      openai.Bool(true),
		}}
		tools[i] = t
	}
	return tools, rawTools, nil
}

func toSDKImageDetail(detail schema.ImageURLDetail) responses.ResponseInputImageDetail {
	switch detail {
	case schema.ImageURLDetailHigh:
		return responses.ResponseInputImageDetailHigh
	case schema.ImageURLDetailLow:
		return responses.ResponseInputImageDetailLow
	case schema.ImageURLDetailAuto:
		return responses.ResponseInputImageDetailAuto
	default:
		return responses.ResponseInputImageDetailAuto
	}
}

func (cm *ChatModel) convertResponseToMessage(resp *responses.Response) (*schema.Message, error) {
	if resp == nil {
		return nil, fmt.Errorf("nil response")
	}

	msg := &schema.Message{Role: schema.Assistant}
	msg.ResponseMeta = ensureResponseMeta(msg.ResponseMeta)
	msg.ResponseMeta.FinishReason = string(resp.Status)
	msg.ResponseMeta.Usage = toEinoTokenUsage(resp.Usage)

	// Extract tool calls and assistant text.
	msg.Content = resp.OutputText()
	for _, item := range resp.Output {
		switch v := item.AsAny().(type) {
		case responses.ResponseFunctionToolCall:
			msg.ToolCalls = append(msg.ToolCalls, schema.ToolCall{
				ID:   v.CallID,
				Type: "function",
				Function: schema.FunctionCall{
					Name:      v.Name,
					Arguments: v.Arguments,
				},
			})
		case responses.ResponseOutputItemImageGenerationCall:
			// result is base64 image (no data: prefix)
			if v.Result != "" {
				b64 := v.Result
				msg.AssistantGenMultiContent = append(msg.AssistantGenMultiContent, schema.MessageOutputPart{
					Type: schema.ChatMessagePartTypeImageURL,
					Image: &schema.MessageOutputImage{
						MessagePartCommon: schema.MessagePartCommon{
							Base64Data: &b64,
							MIMEType:   "image/png",
						},
					},
				})
			}
		case responses.ResponseReasoningItem:
			// Prefer summary text when provided; otherwise content.
			msg.ReasoningContent = joinReasoningText(v)
		}
	}

	if len(msg.Content) > 0 {
		// keep assistant text as first part if no parts exist yet.
		if len(msg.AssistantGenMultiContent) == 0 {
			msg.AssistantGenMultiContent = append(msg.AssistantGenMultiContent, schema.MessageOutputPart{
				Type: schema.ChatMessagePartTypeText,
				Text: msg.Content,
			})
		} else {
			// prepend text part to existing parts
			msg.AssistantGenMultiContent = append([]schema.MessageOutputPart{{
				Type: schema.ChatMessagePartTypeText,
				Text: msg.Content,
			}}, msg.AssistantGenMultiContent...)
		}
	}

	return msg, nil
}

func toEinoTokenUsage(usage responses.ResponseUsage) *schema.TokenUsage {
	// usage is a value type; if it is all zeros, treat as absent.
	if usage.InputTokens == 0 && usage.OutputTokens == 0 && usage.TotalTokens == 0 {
		return nil
	}
	return &schema.TokenUsage{
		PromptTokens: int(usage.InputTokens),
		PromptTokenDetails: schema.PromptTokenDetails{
			CachedTokens: int(usage.InputTokensDetails.CachedTokens),
		},
		CompletionTokens: int(usage.OutputTokens),
		TotalTokens:      int(usage.TotalTokens),
		CompletionTokensDetails: schema.CompletionTokensDetails{
			ReasoningTokens: int(usage.OutputTokensDetails.ReasoningTokens),
		},
	}
}

func toModelTokenUsage(meta *schema.ResponseMeta) *model.TokenUsage {
	if meta == nil || meta.Usage == nil {
		return nil
	}
	u := meta.Usage
	return &model.TokenUsage{
		PromptTokens: u.PromptTokens,
		PromptTokenDetails: model.PromptTokenDetails{
			CachedTokens: u.PromptTokenDetails.CachedTokens,
		},
		CompletionTokens: u.CompletionTokens,
		TotalTokens:      u.TotalTokens,
		CompletionTokensDetails: model.CompletionTokensDetails{
			ReasoningTokens: u.CompletionTokensDetails.ReasoningTokens,
		},
	}
}

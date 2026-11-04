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

package ark

import (
	"encoding/json"
	"fmt"

	"strings"

	"github.com/bytedance/sonic"

	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/responses"
)

func toResponsesContent(msg *schema.Message) (responses.MessageContent, error) {
	if err := validateMessageContent(msg); err != nil {
		return responses.MessageContent{}, err
	}
	var parts []responses.ContentItem
	switch {
	case len(msg.UserInputMultiContent) > 0:
		for _, input := range msg.UserInputMultiContent {
			part, err := toResponsesContentItem(input)
			if err != nil {
				return responses.MessageContent{}, err
			}
			parts = append(parts, part)
		}
	case len(msg.MultiContent) > 0:
		for _, legacy := range msg.MultiContent {
			part, err := toResponsesLegacyContentItem(legacy)
			if err != nil {
				return responses.MessageContent{}, err
			}
			parts = append(parts, part)
		}
	default:
		return responses.NewStringMessageContent(msg.Content), nil
	}
	return responses.NewContentItemArrayMessageContent(parts), nil
}

func toResponsesContentItem(part schema.MessageInputPart) (responses.ContentItem, error) {
	var sum responses.ContentItemSum
	switch part.Type {
	case schema.ChatMessagePartTypeText:
		sum.SetContentItemText(responses.ContentItemText{Type: responses.ContentItemTextTypeInputText, Text: part.Text})
	case schema.ChatMessagePartTypeImageURL:
		if part.Image == nil {
			return responses.ContentItem{}, fmt.Errorf("image must not be nil")
		}
		return responsesImagePart(part.Image.MessagePartCommon, part.Image.Detail)
	case schema.ChatMessagePartTypeVideoURL:
		if part.Video == nil {
			return responses.ContentItem{}, fmt.Errorf("video must not be nil")
		}
		return responsesVideoPart(part.Video.MessagePartCommon, GetInputVideoFPS(part.Video))
	case schema.ChatMessagePartTypeAudioURL:
		if part.Audio == nil {
			return responses.ContentItem{}, fmt.Errorf("audio must not be nil")
		}
		url, fileID, base64, err := mediaSource(part.Audio.MessagePartCommon, mediaKindAudio)
		if err != nil {
			return responses.ContentItem{}, err
		}
		if base64 {
			url = mediaDataURL(part.Audio.MIMEType, url)
		}
		sum.SetContentItemAudio(responses.ContentItemAudio{Type: responses.ContentItemAudioTypeInputAudio, AudioURL: responseString(url), FileID: responseString(fileID)})
	case schema.ChatMessagePartTypeFileURL:
		if part.File == nil {
			return responses.ContentItem{}, fmt.Errorf("file must not be nil")
		}
		url, fileID, base64, err := mediaSource(part.File.MessagePartCommon, mediaKindFile)
		if err != nil {
			return responses.ContentItem{}, err
		}
		item := responses.ContentItemFile{Type: responses.ContentItemFileTypeInputFile, FileID: responseString(fileID)}
		if base64 {
			if part.File.Name == "" {
				return responses.ContentItem{}, fmt.Errorf("file Name is required with Base64Data")
			}
			item.FileData = responses.NewOptString(mediaDataURL(part.File.MIMEType, url))
			item.Filename = responses.NewOptString(part.File.Name)
		} else {
			item.FileURL = responseString(url)
		}
		sum.SetContentItemFile(item)
	default:
		return responses.ContentItem{}, fmt.Errorf("unsupported input part type %s", part.Type)
	}
	return responses.ContentItem{OneOf: sum}, nil
}

func responseString(value string) responses.OptString {
	if value == "" {
		return responses.OptString{}
	}
	return responses.NewOptString(value)
}

func toResponsesInput(messages []*schema.Message, passback bool) (*responsesInput, error) {
	input := &responsesInput{items: make([]responses.InputItem, 0, len(messages)), output: make(map[int]json.RawMessage)}
	for i, msg := range messages {
		if msg == nil {
			return nil, fmt.Errorf("message %d must not be nil", i)
		}
		if err := validateMessageContent(msg); err != nil {
			return nil, fmt.Errorf("message %d: %w", i, err)
		}
		if msg.Role == schema.Assistant {
			output, ok, err := responsesOutputForReplay(msg)
			if err != nil {
				return nil, fmt.Errorf("message %d: %w", i, err)
			}
			if ok {
				for _, raw := range output {
					var item responses.InputItem
					if err := sonic.Unmarshal(raw, &item); err != nil {
						return nil, fmt.Errorf("message %d output: %w", i, err)
					}
					input.output[len(input.items)] = raw
					input.items = append(input.items, item)
				}
				continue
			}
			items, err := toResponsesAssistantInput(msg, passback)
			if err != nil {
				return nil, fmt.Errorf("message %d: %w", i, err)
			}
			input.items = append(input.items, items...)
			continue
		}
		if len(msg.ToolCalls) > 0 {
			return nil, fmt.Errorf("message %d: tool calls require assistant role", i)
		}
		content, err := toResponsesContent(msg)
		if err != nil {
			return nil, fmt.Errorf("message %d: %w", i, err)
		}
		var sum responses.InputItemSum
		switch msg.Role {
		case schema.User, schema.System, schema.RoleType("developer"):
			sum.SetItemEasyMessage(responses.ItemEasyMessage{Type: responses.NewOptItemEasyMessageType(responses.ItemEasyMessageTypeMessage),
				Role: responses.NewOptMessageRole(responses.MessageRole(msg.Role)), Content: content})
		case schema.Tool:
			if msg.ToolCallID == "" {
				return nil, fmt.Errorf("tool message %d requires ToolCallID", i)
			}
			sum.SetItemFunctionToolCallOutput(responses.ItemFunctionToolCallOutput{Type: responses.ItemFunctionToolCallOutputTypeFunctionCallOutput,
				CallID: responses.NewOptString(msg.ToolCallID), Output: responses.NewOptMessageContent(content)})
		default:
			return nil, fmt.Errorf("unsupported role %q", msg.Role)
		}
		input.items = append(input.items, responses.InputItem{OneOf: sum})
	}
	return input, nil
}

// This fallback handles assistant messages created or edited by applications.
// Responses-generated history takes the native output replay path above.
func toResponsesAssistantInput(msg *schema.Message, passback bool) ([]responses.InputItem, error) {
	var items []responses.InputItem
	appendReasoning := func(text, signature string) {
		item := responses.ItemReasoning{Type: responses.ItemReasoningTypeReasoning, Status: responses.ItemStatusCompleted}
		if text != "" {
			item.Summary = []responses.ReasoningSummaryPart{{Type: responses.ReasoningSummaryPartTypeSummaryText, Text: responses.NewOptString(text)}}
		}
		item.EncryptedContent = responseString(signature)
		items = append(items, responses.InputItem{OneOf: responses.NewItemReasoningInputItemSum(item)})
	}
	hasReasoningParts := false
	for _, part := range msg.AssistantGenMultiContent {
		if part.Type == schema.ChatMessagePartTypeReasoning {
			hasReasoningParts = true
		}
	}
	if passback && !hasReasoningParts && msg.ReasoningContent != "" {
		appendReasoning(msg.ReasoningContent, "")
	}
	appendMessage := func(content responses.MessageContent) {
		items = append(items, responses.InputItem{OneOf: responses.NewItemEasyMessageInputItemSum(responses.ItemEasyMessage{
			Type: responses.NewOptItemEasyMessageType(responses.ItemEasyMessageTypeMessage), Role: responses.NewOptMessageRole(responses.MessageRoleAssistant), Content: content,
		})})
	}
	if len(msg.AssistantGenMultiContent) > 0 {
		var parts []responses.ContentItem
		flush := func() {
			if len(parts) > 0 {
				appendMessage(responses.NewContentItemArrayMessageContent(parts))
				parts = nil
			}
		}
		for _, part := range msg.AssistantGenMultiContent {
			if part.Type == schema.ChatMessagePartTypeReasoning {
				if part.Reasoning == nil {
					return nil, fmt.Errorf("assistant reasoning must not be nil")
				}
				flush()
				appendReasoning(part.Reasoning.Text, part.Reasoning.Signature)
				continue
			}
			content, err := toResponsesAssistantContentItem(part)
			if err != nil {
				return nil, err
			}
			parts = append(parts, content)
		}
		flush()
	} else if strings.TrimSpace(msg.Content) != "" || len(msg.MultiContent) > 0 {
		content, err := toResponsesContent(msg)
		if err != nil {
			return nil, err
		}
		appendMessage(content)
	}
	for _, call := range msg.ToolCalls {
		items = append(items, responses.InputItem{OneOf: responses.NewItemFunctionToolCallInputItemSum(responses.ItemFunctionToolCall{
			Type: responses.ItemFunctionToolCallTypeFunctionCall, CallID: responses.NewOptString(call.ID), Name: responses.NewOptString(call.Function.Name), Arguments: responses.NewOptString(call.Function.Arguments),
		})})
	}
	return items, nil
}

func responsesTextPart(text string) responses.ContentItem {
	var sum responses.ContentItemSum
	sum.SetContentItemText(responses.ContentItemText{Type: responses.ContentItemTextTypeInputText, Text: text})
	return responses.ContentItem{OneOf: sum}
}

func responsesImagePart(common schema.MessagePartCommon, detail schema.ImageURLDetail) (responses.ContentItem, error) {
	var sum responses.ContentItemSum
	url, fileID, base64, err := mediaSource(common, mediaKindImage)
	if err != nil {
		return responses.ContentItem{}, err
	}
	if base64 {
		url = mediaDataURL(common.MIMEType, url)
	}
	item := responses.ContentItemImage{Type: responses.ContentItemImageTypeInputImage, ImageURL: responseString(url), FileID: responseString(fileID)}
	if detail != "" {
		item.Detail = responses.NewOptImageDetail(responses.ImageDetail(detail))
	}
	sum.SetContentItemImage(item)
	return responses.ContentItem{OneOf: sum}, nil
}

func responsesVideoPart(common schema.MessagePartCommon, fps *float64) (responses.ContentItem, error) {
	var sum responses.ContentItemSum
	url, fileID, base64, err := mediaSource(common, mediaKindVideo)
	if err != nil {
		return responses.ContentItem{}, err
	}
	if base64 {
		url = mediaDataURL(common.MIMEType, url)
	}
	item := responses.ContentItemVideo{Type: responses.ContentItemVideoTypeInputVideo, VideoURL: url, FileID: responseString(fileID)}
	if fps != nil {
		item.Fps = responses.NewOptFloat64(*fps)
	}
	sum.SetContentItemVideo(item)
	return responses.ContentItem{OneOf: sum}, nil
}

func toResponsesAssistantContentItem(part schema.MessageOutputPart) (responses.ContentItem, error) {
	switch part.Type {
	case schema.ChatMessagePartTypeText:
		return responsesTextPart(part.Text), nil
	case schema.ChatMessagePartTypeImageURL:
		if part.Image == nil {
			return responses.ContentItem{}, fmt.Errorf("assistant image must not be nil")
		}
		return responsesImagePart(part.Image.MessagePartCommon, "")
	case schema.ChatMessagePartTypeVideoURL:
		if part.Video == nil {
			return responses.ContentItem{}, fmt.Errorf("assistant video must not be nil")
		}
		return responsesVideoPart(part.Video.MessagePartCommon, GetOutputVideoFPS(part.Video))
	default:
		return responses.ContentItem{}, fmt.Errorf("unsupported assistant part type %s", part.Type)
	}
}

func toResponsesLegacyContentItem(part schema.ChatMessagePart) (responses.ContentItem, error) {
	switch part.Type {
	case schema.ChatMessagePartTypeText:
		return responsesTextPart(part.Text), nil
	case schema.ChatMessagePartTypeImageURL:
		if part.ImageURL == nil {
			return responses.ContentItem{}, fmt.Errorf("ImageURL must not be nil")
		}
		return responsesImagePart(schema.MessagePartCommon{URL: &part.ImageURL.URL}, part.ImageURL.Detail)
	case schema.ChatMessagePartTypeVideoURL:
		if part.VideoURL == nil {
			return responses.ContentItem{}, fmt.Errorf("VideoURL must not be nil")
		}
		return responsesVideoPart(schema.MessagePartCommon{URL: &part.VideoURL.URL}, getFPS(part.VideoURL.Extra))
	default:
		return responses.ContentItem{}, fmt.Errorf("unsupported legacy part type %s", part.Type)
	}
}

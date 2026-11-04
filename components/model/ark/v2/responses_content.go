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
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/responses"
)

func toResponsesContent(msg *schema.Message) (responses.MessageContent, error) {
	parts, err := messageInputParts(msg)
	if err != nil {
		return responses.MessageContent{}, err
	}
	if parts == nil {
		return responses.NewStringMessageContent(msg.Content), nil
	}
	items := make([]responses.ContentItem, 0, len(parts))
	for _, part := range parts {
		item, err := toResponsesContentItem(part)
		if err != nil {
			return responses.MessageContent{}, err
		}
		items = append(items, item)
	}
	return responses.NewContentItemArrayMessageContent(items), nil
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
		url, fileID, base64, err := mediaSource(part.Image.MessagePartCommon, mediaKindImage)
		if err != nil {
			return responses.ContentItem{}, err
		}
		if base64 {
			url = mediaDataURL(part.Image.MIMEType, url)
		}
		item := responses.ContentItemImage{Type: responses.ContentItemImageTypeInputImage, ImageURL: responseString(url), FileID: responseString(fileID)}
		if part.Image.Detail != "" {
			item.Detail = responses.NewOptImageDetail(responses.ImageDetail(part.Image.Detail))
		}
		sum.SetContentItemImage(item)
	case schema.ChatMessagePartTypeVideoURL:
		if part.Video == nil {
			return responses.ContentItem{}, fmt.Errorf("video must not be nil")
		}
		url, fileID, base64, err := mediaSource(part.Video.MessagePartCommon, mediaKindVideo)
		if err != nil {
			return responses.ContentItem{}, err
		}
		if base64 {
			url = mediaDataURL(part.Video.MIMEType, url)
		}
		item := responses.ContentItemVideo{Type: responses.ContentItemVideoTypeInputVideo, VideoURL: url, FileID: responseString(fileID)}
		if fps := GetInputVideoFPS(part.Video); fps != nil {
			item.Fps = responses.NewOptFloat64(*fps)
		}
		sum.SetContentItemVideo(item)
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

func toResponsesInput(messages []*schema.Message, passback bool) ([]responses.InputItem, error) {
	items := make([]responses.InputItem, 0, len(messages))
	for i, msg := range messages {
		if msg == nil {
			return nil, fmt.Errorf("message %d must not be nil", i)
		}
		content, err := toResponsesContent(msg)
		if err != nil {
			return nil, fmt.Errorf("message %d: %w", i, err)
		}
		var sum responses.InputItemSum
		switch msg.Role {
		case schema.User, schema.System, schema.Assistant, schema.RoleType("developer"):
			if msg.Role == schema.Assistant && passback {
				if saved, ok := getResponsesReasoning(msg); ok {
					for _, reasoning := range saved {
						sum.SetItemReasoning(reasoning)
						items = append(items, responses.InputItem{OneOf: sum})
					}
				} else if msg.ReasoningContent != "" {
					sum.SetItemReasoning(responses.ItemReasoning{Type: responses.ItemReasoningTypeReasoning, Status: responses.ItemStatusCompleted,
						Summary: []responses.ReasoningSummaryPart{{Type: responses.ReasoningSummaryPartTypeSummaryText, Text: responses.NewOptString(msg.ReasoningContent)}}})
					items = append(items, responses.InputItem{OneOf: sum})
				}
			}
			// Tool-only assistant messages must not introduce an empty message item.
			if msg.Role != schema.Assistant || strings.TrimSpace(msg.Content) != "" || len(msg.AssistantGenMultiContent)+len(msg.MultiContent) > 0 {
				sum = responses.InputItemSum{}
				sum.SetItemEasyMessage(responses.ItemEasyMessage{Type: responses.NewOptItemEasyMessageType(responses.ItemEasyMessageTypeMessage),
					Role: responses.NewOptMessageRole(responses.MessageRole(msg.Role)), Content: content})
				items = append(items, responses.InputItem{OneOf: sum})
			}
			for _, call := range msg.ToolCalls {
				if msg.Role != schema.Assistant {
					return nil, fmt.Errorf("tool calls require assistant role")
				}
				sum = responses.InputItemSum{}
				sum.SetItemFunctionToolCall(responses.ItemFunctionToolCall{Type: responses.ItemFunctionToolCallTypeFunctionCall,
					CallID: responses.NewOptString(call.ID), Name: responses.NewOptString(call.Function.Name), Arguments: responses.NewOptString(call.Function.Arguments)})
				items = append(items, responses.InputItem{OneOf: sum})
			}
		case schema.Tool:
			if msg.ToolCallID == "" {
				return nil, fmt.Errorf("tool message %d requires ToolCallID", i)
			}
			sum.SetItemFunctionToolCallOutput(responses.ItemFunctionToolCallOutput{Type: responses.ItemFunctionToolCallOutputTypeFunctionCallOutput,
				CallID: responses.NewOptString(msg.ToolCallID), Output: responses.NewOptMessageContent(content)})
			items = append(items, responses.InputItem{OneOf: sum})
		default:
			return nil, fmt.Errorf("unsupported role %q", msg.Role)
		}
	}
	return items, nil
}

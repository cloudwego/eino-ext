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
	"github.com/volcengine/ark-runtime-go/arkruntime/model/chat"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/responses"
)

// Reuse the Chat adapter's content validation and source precedence, then map
// the selected sources to the Responses API's flat content items.
func toResponsesContent(msg *schema.Message) (responses.MessageContent, error) {
	content, err := toArkContent(msg)
	if err != nil {
		return responses.MessageContent{}, err
	}
	if text, ok := content.GetString(); ok {
		return responses.NewStringMessageContent(text), nil
	}
	parts, _ := content.GetChatCompletionContentPartArray()
	items := make([]responses.ContentItem, 0, len(parts))
	for index, part := range parts {
		var sum responses.ContentItemSum
		switch part.OneOf.Type {
		case chat.ChatCompletionContentPartTextChatCompletionContentPartSum:
			sum.SetContentItemText(responses.ContentItemText{Type: responses.ContentItemTextTypeInputText, Text: part.OneOf.ChatCompletionContentPartText.Text})
		case chat.ChatCompletionContentPartImageChatCompletionContentPartSum:
			image := part.OneOf.ChatCompletionContentPartImage.ImageURL.ChatCompletionContentPartImageImageUrl
			item := responses.ContentItemImage{Type: responses.ContentItemImageTypeInputImage, ImageURL: responseString(image.URL), FileID: responseString(image.FileID)}
			if image.Detail.IsSet() {
				item.Detail = responses.NewOptImageDetail(responses.ImageDetail(image.Detail.Value))
			}
			sum.SetContentItemImage(item)
		case chat.ChatCompletionContentPartVideoChatCompletionContentPartSum:
			video := part.OneOf.ChatCompletionContentPartVideo.VideoURL
			item := responses.ContentItemVideo{Type: responses.ContentItemVideoTypeInputVideo, VideoURL: video.URL.Or(""), FileID: responseString(video.FileID)}
			if video.Fps.IsSet() {
				item.Fps = responses.NewOptFloat64(video.Fps.Value)
			}
			sum.SetContentItemVideo(item)
		case chat.ChatCompletionContentPartAudioChatCompletionContentPartSum:
			audio := part.OneOf.ChatCompletionContentPartAudio.InputAudio
			item := responses.ContentItemAudio{Type: responses.ContentItemAudioTypeInputAudio, AudioURL: responseString(audio.URL), FileID: responseString(audio.FileID)}
			if audio.Data.IsSet() {
				// Responses accepts audio data URLs rather than Chat's data/format pair.
				mime := msg.UserInputMultiContent[index].Audio.MIMEType
				item.AudioURL = responses.NewOptString("data:" + mime + ";base64," + audio.Data.Value)
			}
			sum.SetContentItemAudio(item)
		case chat.ChatCompletionContentPartFileChatCompletionContentPartSum:
			file := part.OneOf.ChatCompletionContentPartFile.File
			item := responses.ContentItemFile{Type: responses.ContentItemFileTypeInputFile,
				FileURL: responseString(file.FileURL), FileData: responseString(file.FileData), FileID: responseString(file.FileID)}
			// Ark accepts filename only alongside file_data, not file_url or file_id.
			if file.FileData.IsSet() {
				item.Filename = responseString(file.Filename)
			}
			sum.SetContentItemFile(item)
		default:
			return responses.MessageContent{}, fmt.Errorf("unsupported Responses content type %s", part.OneOf.Type)
		}
		items = append(items, responses.ContentItem{OneOf: sum})
	}
	return responses.NewContentItemArrayMessageContent(items), nil
}

func responseString(value chat.OptString) responses.OptString {
	if value.IsSet() {
		return responses.NewOptString(value.Value)
	}
	return responses.OptString{}
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

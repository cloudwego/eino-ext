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

package hunyuan

import (
	"fmt"
	"log"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	hunyuan "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/hunyuan/v20230901"
)

func convertResponse(resp *hunyuan.ChatCompletionsResponseParams) *schema.Message {
	msg := &schema.Message{
		ResponseMeta: &schema.ResponseMeta{},
	}
	for _, choice := range resp.Choices {
		var toolCalls []*hunyuan.ToolCall
		if choice.Message != nil {
			msg.Role = schema.RoleType(*choice.Message.Role)
			toolCalls = choice.Message.ToolCalls
			if choice.Message.Content != nil {
				msg.Content = *choice.Message.Content
			}
			if choice.Message.ReasoningContent != nil {
				msg.ReasoningContent = *choice.Message.ReasoningContent
			}
		} else if choice.Delta != nil {
			msg.Role = schema.RoleType(*choice.Delta.Role)
			toolCalls = choice.Delta.ToolCalls
			if choice.Delta.Content != nil {
				msg.Content = *choice.Delta.Content
			}
			if choice.Delta.ReasoningContent != nil {
				msg.ReasoningContent = *choice.Delta.ReasoningContent
			}
		}
		if choice.FinishReason != nil {
			msg.ResponseMeta.FinishReason = *choice.FinishReason
		}
		msg.ToolCalls = toMessageToolCalls(toolCalls)
	}
	if resp.Usage != nil {
		msg.ResponseMeta.Usage = &schema.TokenUsage{
			PromptTokens:     int(*resp.Usage.PromptTokens),
			CompletionTokens: int(*resp.Usage.CompletionTokens),
			TotalTokens:      int(*resp.Usage.TotalTokens),
		}
	}
	return msg
}

func toMessageToolCalls(toolCalls []*hunyuan.ToolCall) []schema.ToolCall {
	if len(toolCalls) == 0 {
		return nil
	}
	ret := make([]schema.ToolCall, 0, len(toolCalls))
	for i := range toolCalls {
		tc := schema.ToolCall{}
		if toolCalls[i].Index != nil {
			idx := int(*toolCalls[i].Index)
			tc.Index = &idx
		}
		if toolCalls[i].Id != nil {
			tc.ID = *toolCalls[i].Id
		}
		if toolCalls[i].Type != nil {
			tc.Type = *toolCalls[i].Type
		}
		if toolCalls[i].Function != nil {
			tc.Function.Name = *toolCalls[i].Function.Name
			tc.Function.Arguments = *toolCalls[i].Function.Arguments
		}
		ret = append(ret, tc)
	}
	return ret
}

func toCallbackUsage(usage *schema.TokenUsage) *model.TokenUsage {
	if usage == nil {
		return nil
	}
	return &model.TokenUsage{
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
	}
}

func toTools(tools []*schema.ToolInfo) ([]*hunyuan.Tool, error) {
	results := make([]*hunyuan.Tool, 0, len(tools))
	for _, tl := range tools {
		sc, err := tl.ToJSONSchema()
		if err != nil {
			return nil, err
		}
		parameters, err := sc.MarshalJSON()
		if err != nil {
			return nil, err
		}
		results = append(results, &hunyuan.Tool{
			Type: &[]string{"function"}[0],
			Function: &hunyuan.ToolFunction{
				Name:        toPtr(tl.Name),
				Description: toPtr(tl.Desc),
				Parameters:  toPtr(string(parameters)),
			},
		})
	}
	return results, nil
}

func convertMessage(msg *schema.Message) (*hunyuan.Message, error) {
	hMsg := &hunyuan.Message{}

	var role string
	switch msg.Role {
	case schema.Assistant:
		role = roleAssistant
	case schema.System:
		role = roleSystem
	case schema.User:
		role = roleUser
	case schema.Tool:
		role = roleTool
	default:
		return nil, fmt.Errorf("unknown role type: %s", msg.Role)
	}
	hMsg.Role = toPtr(role)

	if len(msg.Content) > 0 {
		hMsg.Content = toPtr(msg.Content)
	}
	if len(msg.ReasoningContent) > 0 {
		hMsg.ReasoningContent = toPtr(msg.ReasoningContent)
	}
	if *hMsg.Role == roleTool && len(msg.ToolCalls) == 0 {
		hMsg.ToolCallId = toPtr(msg.ToolCallID)
	}
	for _, call := range msg.ToolCalls {
		var index int
		if call.Index != nil {
			index = *call.Index
		}
		hMsg.ToolCalls = append(hMsg.ToolCalls, &hunyuan.ToolCall{
			Index: toPtr(int64(index)),
			Type:  toPtr(call.Type),
			Function: &hunyuan.ToolCallFunction{
				Name:      toPtr(call.Function.Name),
				Arguments: toPtr(call.Function.Arguments),
			},
		})
	}
	if len(msg.UserInputMultiContent) > 0 && len(msg.AssistantGenMultiContent) > 0 {
		return nil, fmt.Errorf("a message cannot contain both UserInputMultiContent and AssistantGenMultiContent")
	}
	if len(msg.UserInputMultiContent) > 0 {
		contents, err := convertInputMedia(msg.UserInputMultiContent)
		if err != nil {
			return nil, err
		}
		hMsg.Contents = append(hMsg.Contents, contents...)
	} else if len(msg.AssistantGenMultiContent) > 0 {
		contents, err := convertOutputMedia(msg.AssistantGenMultiContent)
		if err != nil {
			return nil, err
		}
		hMsg.Contents = append(hMsg.Contents, contents...)
	}
	if len(msg.MultiContent) > 0 {
		log.Printf("MultiContent field is deprecated, please use UserInputMultiContent or AssistantGenMultiContent instead")
		contents, err := convertMedia(msg.MultiContent)
		if err != nil {
			return nil, err
		}
		hMsg.Contents = append(hMsg.Contents, contents...)
	}
	return hMsg, nil
}

func convertInputMedia(contents []schema.MessageInputPart) ([]*hunyuan.Content, error) {
	result := make([]*hunyuan.Content, 0, len(contents))
	for _, content := range contents {
		switch content.Type {
		case schema.ChatMessagePartTypeText:
			if len(content.Text) == 0 {
				continue
			}
			result = append(result, &hunyuan.Content{
				Type: toPtr(contentTypeText),
				Text: toPtr(content.Text),
			})
		case schema.ChatMessagePartTypeImageURL:
			if content.Image == nil {
				return nil, fmt.Errorf("image field must not be nil when Type is ChatMessagePartTypeImageURL in user message")
			}
			imageUrl, err := imageToUrlOrBase64(content.Image.URL, content.Image.Base64Data, content.Image.MIMEType)
			if err != nil {
				return nil, err
			}
			result = append(result, &hunyuan.Content{
				Type:     toPtr(contentTypeImage),
				ImageUrl: imageUrl,
			})
		case schema.ChatMessagePartTypeVideoURL:
			if content.Video == nil || (content.Video.URL == nil || *content.Video.URL == "") {
				return nil, fmt.Errorf("video field must not be nil when Type is ChatMessagePartTypeVideoURL in user message")
			}
			result = append(result, &hunyuan.Content{
				Type: toPtr(contentTypeVideoURL),
				VideoUrl: &hunyuan.VideoUrl{
					Url: content.Video.URL,
				},
			})
		default:
			return nil, fmt.Errorf("unsupported chat message part type: %s", content.Type)
		}
	}
	return result, nil
}

func convertOutputMedia(contents []schema.MessageOutputPart) ([]*hunyuan.Content, error) {
	result := make([]*hunyuan.Content, 0, len(contents))
	for _, content := range contents {
		switch content.Type {
		case schema.ChatMessagePartTypeText:
			if len(content.Text) == 0 {
				continue
			}
			result = append(result, &hunyuan.Content{
				Type: toPtr(contentTypeText),
				Text: toPtr(content.Text),
			})
		case schema.ChatMessagePartTypeImageURL:
			if content.Image == nil {
				return nil, fmt.Errorf("image field must not be nil when Type is ChatMessagePartTypeImageURL in user message")
			}
			imageUrl, err := imageToUrlOrBase64(content.Image.URL, content.Image.Base64Data, content.Image.MIMEType)
			if err != nil {
				return nil, err
			}
			result = append(result, &hunyuan.Content{
				Type:     toPtr(contentTypeImage),
				ImageUrl: imageUrl,
			})
		case schema.ChatMessagePartTypeVideoURL:
			if content.Video == nil || (content.Video.URL == nil || *content.Video.URL == "") {
				return nil, fmt.Errorf("video field must not be nil when Type is ChatMessagePartTypeVideoURL in user message")
			}
			result = append(result, &hunyuan.Content{
				Type: toPtr(contentTypeVideoURL),
				VideoUrl: &hunyuan.VideoUrl{
					Url: content.Video.URL,
				},
			})
		default:
			return nil, fmt.Errorf("unsupported chat message part type: %s", content.Type)
		}
	}
	return result, nil
}

func convertMedia(contents []schema.ChatMessagePart) ([]*hunyuan.Content, error) {
	result := make([]*hunyuan.Content, 0, len(contents))
	for _, content := range contents {
		switch content.Type {
		case schema.ChatMessagePartTypeText:
			if len(content.Text) == 0 {
				continue
			}
			result = append(result, &hunyuan.Content{
				Type: toPtr(contentTypeText),
				Text: toPtr(content.Text),
			})
		case schema.ChatMessagePartTypeImageURL:
			if content.ImageURL == nil || content.ImageURL.URL == "" {
				return nil, fmt.Errorf("image field must not be nil when Type is ChatMessagePartTypeImageURL in user message")
			}
			result = append(result, &hunyuan.Content{
				Type: toPtr(contentTypeImage),
				ImageUrl: &hunyuan.ImageUrl{
					Url: toPtr(content.ImageURL.URL),
				},
			})
		case schema.ChatMessagePartTypeVideoURL:
			if content.VideoURL == nil || content.VideoURL.URL == "" {
				return nil, fmt.Errorf("video field must not be nil when Type is ChatMessagePartTypeVideoURL in user message")
			}
			result = append(result, &hunyuan.Content{
				Type: toPtr(contentTypeVideoURL),
				VideoUrl: &hunyuan.VideoUrl{
					Url: toPtr(content.VideoURL.URL),
				},
			})
		default:
			return nil, fmt.Errorf("unsupported chat message part type: %s", content.Type)
		}
	}
	return result, nil
}

func populateToolChoice(req *hunyuan.ChatCompletionsRequest, tc *schema.ToolChoice, allowedToolNames []string) error {
	if tc == nil {
		return nil
	}

	switch *tc {
	case schema.ToolChoiceForbidden:
		req.ToolChoice = toPtr(toolChoiceNone)
	case schema.ToolChoiceAllowed:
		req.ToolChoice = toPtr(toolChoiceAuto)
	case schema.ToolChoiceForced:
		if len(req.Tools) == 0 {
			return fmt.Errorf("tool choice is forced but tool is not provided")
		}

		var tl *hunyuan.Tool
		if len(allowedToolNames) > 0 {
			if len(allowedToolNames) > 1 {
				return fmt.Errorf("only one allowed tool name can be configured")
			}
			allowedToolName := allowedToolNames[0]
			toolsMap := make(map[string]*hunyuan.Tool, len(req.Tools))
			for _, t := range req.Tools {
				toolsMap[*t.Function.Name] = t
			}
			if t, ok := toolsMap[allowedToolName]; !ok {
				return fmt.Errorf("allowed tool name '%s' not found in tools list", allowedToolName)
			} else {
				tl = t
			}
		} else if len(req.Tools) == 1 {
			tl = req.Tools[0]
		}
		if tl != nil {
			req.CustomTool = tl
		}
		req.ToolChoice = toPtr(toolChoiceRequired)
	default:
		return fmt.Errorf("tool choice=%s not support", *tc)
	}
	return nil
}

func imageToUrlOrBase64(url *string, base64Data *string, mimeType string) (*hunyuan.ImageUrl, error) {
	if url != nil && *url != "" {
		return &hunyuan.ImageUrl{
			Url: url,
		}, nil
	}
	if base64Data != nil && *base64Data != "" {
		return &hunyuan.ImageUrl{
			Url: toPtr("data:" + mimeType + ";base64," + *base64Data),
		}, nil
	}
	return nil, fmt.Errorf("image part must have either a URL or Base64Data")
}

// toPtr returns a pointer to the given value.
// This is useful for converting values to pointers when needed for optional parameters.
func toPtr[T any](v T) *T {
	return &v
}

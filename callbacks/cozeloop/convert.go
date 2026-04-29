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

package cozeloop

import (
	"context"
	"fmt"
	"log"

	"github.com/bytedance/sonic"
	"github.com/coze-dev/cozeloop-go/spec/tracespec"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
)

const (
	toolTypeFunction   = "function"
	toolTypeServerTool = "server_tool"
	toolTypeMCPTool    = "mcp_tool"
)

// ChatModel

func convertModelInput(input *model.CallbackInput) *tracespec.ModelInput {
	return &tracespec.ModelInput{
		Messages:        iterSlice(input.Messages, convertModelMessage),
		Tools:           iterSlice(input.Tools, convertTool),
		ModelToolChoice: convertToolChoice(input.ToolChoice),
	}
}

func convertModelOutput(output *model.CallbackOutput) *tracespec.ModelOutput {
	if output == nil {
		return nil
	}
	return &tracespec.ModelOutput{
		Choices: []*tracespec.ModelChoice{
			{
				Index:        0,
				FinishReason: getFinishReason(output.Message),
				Message:      convertModelMessage(output.Message)},
		},
	}
}

func getFinishReason(msg *schema.Message) string {
	if msg == nil || msg.ResponseMeta == nil {
		return ""
	}

	return msg.ResponseMeta.FinishReason
}

func convertModelMessage(message *schema.Message) *tracespec.ModelMessage {
	if message == nil {
		return nil
	}

	msg := &tracespec.ModelMessage{
		Role:             string(message.Role),
		Content:          message.Content,
		Parts:            make([]*tracespec.ModelMessagePart, len(message.MultiContent)),
		Name:             message.Name,
		ToolCalls:        make([]*tracespec.ModelToolCall, len(message.ToolCalls)),
		ToolCallID:       message.ToolCallID,
		ReasoningContent: message.ReasoningContent,
	}
	if message.Role == schema.Tool {
		msg.Name = message.ToolName
	}

	if len(message.UserInputMultiContent) > 0 {
		msg.Parts = convertUserInputMultiContent(message.UserInputMultiContent)
	} else if len(message.AssistantGenMultiContent) > 0 {
		msg.Parts = convertAssistantGenMultiContent(message.AssistantGenMultiContent)
	} else {
		msg.Parts = convertMultiContent(message.MultiContent)
	}

	for i := range message.ToolCalls {
		tc := message.ToolCalls[i]

		msg.ToolCalls[i] = &tracespec.ModelToolCall{
			ID:   tc.ID,
			Type: toolTypeFunction,
			Function: &tracespec.ModelToolCallFunction{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		}
	}

	if message.Extra != nil {
		msg.Metadata = make(map[string]string, len(message.Extra))
		for k, v := range message.Extra {
			if sv, err := sonic.MarshalString(v); err == nil {
				msg.Metadata[k] = sv
			}
		}
	}

	return msg
}

func convertUserInputMultiContent(parts []schema.MessageInputPart) []*tracespec.ModelMessagePart {
	var result []*tracespec.ModelMessagePart
	for _, part := range parts {
		sign := GetBase64ThoughtSignatureFromExtra(part.Extra)
		switch part.Type {
		case schema.ChatMessagePartTypeText:
			result = append(result, &tracespec.ModelMessagePart{
				Type:      tracespec.ModelMessagePartType(part.Type),
				Text:      part.Text,
				Signature: sign,
			})

		case schema.ChatMessagePartTypeImageURL:
			if part.Image == nil {
				continue
			}

			if part.Image.MessagePartCommon.URL != nil {
				result = append(result, &tracespec.ModelMessagePart{
					Type: tracespec.ModelMessagePartType(part.Type),
					ImageURL: &tracespec.ModelImageURL{
						URL:    *part.Image.MessagePartCommon.URL,
						Detail: string(part.Image.Detail),
					},
					Signature: sign,
				})
			}
			if part.Image.MessagePartCommon.Base64Data != nil {
				result = append(result, &tracespec.ModelMessagePart{
					Type: tracespec.ModelMessagePartType(part.Type),
					ImageURL: &tracespec.ModelImageURL{
						URL:    fmt.Sprintf("data:%s;base64,%s", part.Image.MessagePartCommon.MIMEType, *part.Image.MessagePartCommon.Base64Data),
						Detail: string(part.Image.Detail),
					},
					Signature: sign,
				})
			}

		case schema.ChatMessagePartTypeFileURL:
			if part.File == nil {
				continue
			}
			if part.File.MessagePartCommon.URL != nil {
				result = append(result, &tracespec.ModelMessagePart{
					Type: tracespec.ModelMessagePartType(part.Type),
					FileURL: &tracespec.ModelFileURL{
						URL: *part.File.MessagePartCommon.URL,
					},
					Signature: sign,
				})
			}
			if part.File.MessagePartCommon.Base64Data != nil {
				result = append(result, &tracespec.ModelMessagePart{
					Type: tracespec.ModelMessagePartType(part.Type),
					FileURL: &tracespec.ModelFileURL{
						URL: fmt.Sprintf("data:%s;base64,%s", part.File.MessagePartCommon.MIMEType, *part.File.MessagePartCommon.Base64Data),
					},
					Signature: sign,
				})
			}

		default:
			log.Printf("unknown part type: %s", part.Type)
		}
	}
	return result
}

func convertAssistantGenMultiContent(parts []schema.MessageOutputPart) []*tracespec.ModelMessagePart {
	var result []*tracespec.ModelMessagePart
	for _, part := range parts {
		sign := GetBase64ThoughtSignatureFromExtra(part.Extra)
		switch part.Type {
		case schema.ChatMessagePartTypeText:
			result = append(result, &tracespec.ModelMessagePart{
				Type:      tracespec.ModelMessagePartType(part.Type),
				Text:      part.Text,
				Signature: sign,
			})
		case schema.ChatMessagePartTypeImageURL:
			if part.Image == nil {
				continue
			}
			if part.Image.MessagePartCommon.URL != nil {
				result = append(result, &tracespec.ModelMessagePart{
					Type: tracespec.ModelMessagePartType(part.Type),
					ImageURL: &tracespec.ModelImageURL{
						URL: *part.Image.MessagePartCommon.URL,
					},
					Signature: sign,
				})
			}
			if part.Image.MessagePartCommon.Base64Data != nil {
				result = append(result, &tracespec.ModelMessagePart{
					Type: tracespec.ModelMessagePartType(part.Type),
					ImageURL: &tracespec.ModelImageURL{
						URL: *part.Image.MessagePartCommon.Base64Data,
					},
					Signature: sign,
				})
			}
		default:
			log.Printf("unknown part type: %s", part.Type)
		}
	}
	return result
}

func convertMultiContent(parts []schema.ChatMessagePart) []*tracespec.ModelMessagePart {
	result := make([]*tracespec.ModelMessagePart, len(parts))
	for i := range parts {
		part := parts[i]

		result[i] = &tracespec.ModelMessagePart{
			Type: tracespec.ModelMessagePartType(part.Type),
			Text: part.Text,
		}

		if part.ImageURL != nil {
			result[i].ImageURL = &tracespec.ModelImageURL{
				URL:    part.ImageURL.URL,
				Detail: string(part.ImageURL.Detail),
			}
		}

		if part.FileURL != nil {
			result[i].FileURL = &tracespec.ModelFileURL{
				URL: part.FileURL.URL,
			}
		}
	}
	return result
}

func addToolName(ctx context.Context, message *tracespec.ModelMessage) *tracespec.ModelMessage {
	if message == nil {
		return message
	}

	toolIDNameMap := getToolIDNameMapFromCtx(ctx)
	if toolIDNameMap == nil {
		return message
	}
	toolName, ok := toolIDNameMap[message.ToolCallID]
	if !ok {
		return message
	}

	message.Name = toolName
	return message
}

func convertTool(tool *schema.ToolInfo) *tracespec.ModelTool {
	if tool == nil {
		return nil
	}

	var params []byte
	if raw, err := tool.ToJSONSchema(); err == nil && raw != nil {
		params, _ = raw.MarshalJSON()
	}

	t := &tracespec.ModelTool{
		Type: toolTypeFunction,
		Function: &tracespec.ModelToolFunction{
			Name:        tool.Name,
			Description: tool.Desc,
			Parameters:  params,
		},
	}

	return t
}

func convertToolChoice(tc *schema.ToolChoice) *tracespec.ModelToolChoice {
	if tc == nil {
		return nil
	}
	var v string
	switch *tc {
	case schema.ToolChoiceForbidden:
		v = tracespec.VToolChoiceNone
	case schema.ToolChoiceAllowed:
		v = tracespec.VToolChoiceAuto
	case schema.ToolChoiceForced:
		v = tracespec.VToolChoiceRequired
	default:
		v = tracespec.VToolChoiceAuto
	}
	return &tracespec.ModelToolChoice{Type: v}
}

func convertModelCallOption(config *model.Config) *tracespec.ModelCallOption {
	if config == nil {
		return nil
	}

	return &tracespec.ModelCallOption{
		Temperature: config.Temperature,
		MaxTokens:   int64(config.MaxTokens),
		TopP:        config.TopP,
	}
}

// Prompt

func convertPromptInput(input *prompt.CallbackInput) *tracespec.PromptInput {
	if input == nil {
		return nil
	}

	return &tracespec.PromptInput{
		Templates: iterSlice(input.Templates, convertTemplate),
		Arguments: convertPromptArguments(input.Variables),
	}
}

func convertPromptOutput(output *prompt.CallbackOutput) *tracespec.PromptOutput {
	if output == nil {
		return nil
	}

	return &tracespec.PromptOutput{
		Prompts: iterSlice(output.Result, convertModelMessage),
	}
}

func convertTemplate(template schema.MessagesTemplate) *tracespec.ModelMessage {
	if template == nil {
		return nil
	}

	switch t := template.(type) {
	case *schema.Message:
		return convertModelMessage(t)
	default: // messagePlaceholder etc.
		return nil
	}
}

func convertPromptArguments(variables map[string]any) []*tracespec.PromptArgument {
	if variables == nil {
		return nil
	}

	resp := make([]*tracespec.PromptArgument, 0, len(variables))

	for k := range variables {
		resp = append(resp, &tracespec.PromptArgument{
			Key:   k,
			Value: variables[k],
			// Source: "",
		})
	}

	return resp
}

// Retriever

func convertRetrieverOutput(output *retriever.CallbackOutput) *tracespec.RetrieverOutput {
	if output == nil {
		return nil
	}

	return &tracespec.RetrieverOutput{
		Documents: iterSlice(output.Docs, convertDocument),
	}
}

func convertRetrieverCallOption(input *retriever.CallbackInput) *tracespec.RetrieverCallOption {
	if input == nil {
		return nil
	}

	opt := &tracespec.RetrieverCallOption{
		TopK:   int64(input.TopK),
		Filter: input.Filter,
	}

	if input.ScoreThreshold != nil {
		opt.MinScore = input.ScoreThreshold
	}

	return opt
}

func convertDocument(doc *schema.Document) *tracespec.RetrieverDocument {
	if doc == nil {
		return nil
	}

	return &tracespec.RetrieverDocument{
		ID:      doc.ID,
		Content: doc.Content,
		Score:   doc.Score(),
		// Index:   "",
		Vector: doc.DenseVector(),
	}
}

// AgenticModel

func convertAgenticModelInput(input *model.AgenticCallbackInput) *tracespec.ModelInput {
	if input == nil {
		return nil
	}

	var messages []*tracespec.ModelMessage
	for _, msg := range input.Messages {
		messages = append(messages, expandAgenticModelMessage(msg)...)
	}

	return &tracespec.ModelInput{
		Messages: messages,
		Tools:    iterSlice(input.Tools, convertTool),
	}
}

func convertAgenticModelOutput(output *model.AgenticCallbackOutput) *tracespec.ModelOutput {
	if output == nil {
		return nil
	}

	msgs := expandAgenticModelMessage(output.Message)
	choices := make([]*tracespec.ModelChoice, len(msgs))
	for i, msg := range msgs {
		choices[i] = &tracespec.ModelChoice{
			Index:   int64(i),
			Message: msg,
		}
	}

	return &tracespec.ModelOutput{
		Choices: choices,
	}
}

func convertAgenticModelMessage(message *schema.AgenticMessage) *tracespec.ModelMessage {
	if message == nil {
		return nil
	}

	msg := &tracespec.ModelMessage{
		Role: string(message.Role),
	}

	var parts []*tracespec.ModelMessagePart
	var toolCalls []*tracespec.ModelToolCall

	for _, block := range message.ContentBlocks {
		if block == nil {
			continue
		}
		sign := getBase64AgenticThoughtSignatureFromExtra(block.Extra)

		switch block.Type {
		case schema.ContentBlockTypeReasoning:
			if block.Reasoning != nil {
				if msg.ReasoningContent != "" {
					msg.ReasoningContent += "\n"
				}
				msg.ReasoningContent += block.Reasoning.Text
			}

		case schema.ContentBlockTypeUserInputText:
			if block.UserInputText != nil {
				parts = append(parts, &tracespec.ModelMessagePart{
					Type:      tracespec.ModelMessagePartTypeText,
					Text:      block.UserInputText.Text,
					Signature: sign,
				})
			}

		case schema.ContentBlockTypeUserInputImage:
			if block.UserInputImage != nil {
				url := block.UserInputImage.URL
				if url == "" && block.UserInputImage.Base64Data != "" {
					url = fmt.Sprintf("data:%s;base64,%s", block.UserInputImage.MIMEType, block.UserInputImage.Base64Data)
				}
				parts = append(parts, &tracespec.ModelMessagePart{
					Type: tracespec.ModelMessagePartTypeImage,
					ImageURL: &tracespec.ModelImageURL{
						URL:    url,
						Detail: string(block.UserInputImage.Detail),
					},
					Signature: sign,
				})
			}

		case schema.ContentBlockTypeUserInputAudio:
			if block.UserInputAudio != nil {
				url := block.UserInputAudio.URL
				if url == "" && block.UserInputAudio.Base64Data != "" {
					url = fmt.Sprintf("data:%s;base64,%s", block.UserInputAudio.MIMEType, block.UserInputAudio.Base64Data)
				}
				parts = append(parts, &tracespec.ModelMessagePart{
					Type:      tracespec.ModelMessagePartTypeAudio,
					AudioURL:  &tracespec.ModelAudioURL{URL: url},
					Signature: sign,
				})
			}

		case schema.ContentBlockTypeUserInputVideo:
			if block.UserInputVideo != nil {
				url := block.UserInputVideo.URL
				if url == "" && block.UserInputVideo.Base64Data != "" {
					url = fmt.Sprintf("data:%s;base64,%s", block.UserInputVideo.MIMEType, block.UserInputVideo.Base64Data)
				}
				parts = append(parts, &tracespec.ModelMessagePart{
					Type:      tracespec.ModelMessagePartTypeVideo,
					VideoURL:  &tracespec.ModelVideoURL{URL: url},
					Signature: sign,
				})
			}

		case schema.ContentBlockTypeUserInputFile:
			if block.UserInputFile != nil {
				url := block.UserInputFile.URL
				if url == "" && block.UserInputFile.Base64Data != "" {
					url = fmt.Sprintf("data:%s;base64,%s", block.UserInputFile.MIMEType, block.UserInputFile.Base64Data)
				}
				parts = append(parts, &tracespec.ModelMessagePart{
					Type:      tracespec.ModelMessagePartTypeFile,
					FileURL:   &tracespec.ModelFileURL{URL: url},
					Signature: sign,
				})
			}

		case schema.ContentBlockTypeAssistantGenText:
			if block.AssistantGenText != nil {
				parts = append(parts, &tracespec.ModelMessagePart{
					Type:      tracespec.ModelMessagePartTypeText,
					Text:      block.AssistantGenText.Text,
					Signature: sign,
				})
			}

		case schema.ContentBlockTypeAssistantGenImage:
			if block.AssistantGenImage != nil {
				url := block.AssistantGenImage.URL
				if url == "" && block.AssistantGenImage.Base64Data != "" {
					if block.AssistantGenImage.MIMEType != "" {
						url = fmt.Sprintf("data:%s;base64,%s", block.AssistantGenImage.MIMEType, block.AssistantGenImage.Base64Data)
					} else {
						url = block.AssistantGenImage.Base64Data
					}
				}
				parts = append(parts, &tracespec.ModelMessagePart{
					Type:      tracespec.ModelMessagePartTypeImage,
					ImageURL:  &tracespec.ModelImageURL{URL: url},
					Signature: sign,
				})
			}

		case schema.ContentBlockTypeAssistantGenAudio:
			if block.AssistantGenAudio != nil {
				url := block.AssistantGenAudio.URL
				if url == "" && block.AssistantGenAudio.Base64Data != "" {
					url = fmt.Sprintf("data:%s;base64,%s", block.AssistantGenAudio.MIMEType, block.AssistantGenAudio.Base64Data)
				}
				parts = append(parts, &tracespec.ModelMessagePart{
					Type:      tracespec.ModelMessagePartTypeAudio,
					AudioURL:  &tracespec.ModelAudioURL{URL: url},
					Signature: sign,
				})
			}

		case schema.ContentBlockTypeAssistantGenVideo:
			if block.AssistantGenVideo != nil {
				url := block.AssistantGenVideo.URL
				if url == "" && block.AssistantGenVideo.Base64Data != "" {
					url = fmt.Sprintf("data:%s;base64,%s", block.AssistantGenVideo.MIMEType, block.AssistantGenVideo.Base64Data)
				}
				parts = append(parts, &tracespec.ModelMessagePart{
					Type:      tracespec.ModelMessagePartTypeVideo,
					VideoURL:  &tracespec.ModelVideoURL{URL: url},
					Signature: sign,
				})
			}

		case schema.ContentBlockTypeFunctionToolCall:
			if block.FunctionToolCall != nil {
				toolCalls = append(toolCalls, &tracespec.ModelToolCall{
					ID:   block.FunctionToolCall.CallID,
					Type: toolTypeFunction,
					Function: &tracespec.ModelToolCallFunction{
						Name:      block.FunctionToolCall.Name,
						Arguments: block.FunctionToolCall.Arguments,
					},
					Signature: sign,
				})
			}

		case schema.ContentBlockTypeServerToolCall:
			if block.ServerToolCall != nil {
				args, _ := sonic.MarshalString(block.ServerToolCall.Arguments)
				toolCalls = append(toolCalls, &tracespec.ModelToolCall{
					ID:   block.ServerToolCall.CallID,
					Type: toolTypeServerTool,
					Function: &tracespec.ModelToolCallFunction{
						Name:      block.ServerToolCall.Name,
						Arguments: args,
					},
					Signature: sign,
				})
			}

		case schema.ContentBlockTypeMCPToolCall:
			if block.MCPToolCall != nil {
				toolCalls = append(toolCalls, &tracespec.ModelToolCall{
					ID:   block.MCPToolCall.CallID,
					Type: toolTypeMCPTool,
					Function: &tracespec.ModelToolCallFunction{
						Name:      block.MCPToolCall.Name,
						Arguments: block.MCPToolCall.Arguments,
					},
					Signature: sign,
				})
			}

		default:
			continue
		}
	}

	msg.Parts = parts
	msg.ToolCalls = toolCalls

	metadata := make(map[string]string)
	if message.Extra != nil {
		for k, v := range message.Extra {
			if sv, err := sonic.MarshalString(v); err == nil {
				metadata[k] = sv
			}
		}
	}
	for _, block := range message.ContentBlocks {
		if block == nil {
			continue
		}
		switch block.Type {
		case schema.ContentBlockTypeMCPToolCall:
			if block.MCPToolCall != nil {
				if block.MCPToolCall.ServerLabel != "" {
					metadata["mcp_server_label"] = block.MCPToolCall.ServerLabel
				}
				if block.MCPToolCall.ApprovalRequestID != "" {
					metadata["mcp_approval_request_id"] = block.MCPToolCall.ApprovalRequestID
				}
			}
		}
	}
	if len(metadata) > 0 {
		msg.Metadata = metadata
	}

	return msg
}

func expandAgenticModelMessage(message *schema.AgenticMessage) []*tracespec.ModelMessage {
	if message == nil {
		return nil
	}

	var result []*tracespec.ModelMessage
	var pendingBlocks []*schema.ContentBlock
	var pendingToolCallType schema.ContentBlockType

	flushPending := func() {
		if len(pendingBlocks) == 0 {
			return
		}
		grouped := &schema.AgenticMessage{
			Role:          message.Role,
			ContentBlocks: pendingBlocks,
			Extra:         message.Extra,
		}
		result = append(result, convertAgenticModelMessage(grouped))
		pendingBlocks = nil
		pendingToolCallType = ""
	}

	isToolCallBlock := func(t schema.ContentBlockType) bool {
		return t == schema.ContentBlockTypeFunctionToolCall ||
			t == schema.ContentBlockTypeServerToolCall ||
			t == schema.ContentBlockTypeMCPToolCall
	}

	for _, block := range message.ContentBlocks {
		if block == nil {
			continue
		}
		if block.Type == schema.ContentBlockTypeFunctionToolResult && block.FunctionToolResult != nil {
			flushPending()
			result = append(result, &tracespec.ModelMessage{
				Role:       "tool",
				ToolCallID: block.FunctionToolResult.CallID,
				Name:       block.FunctionToolResult.Name,
				Content:    block.FunctionToolResult.Result,
			})
		} else if block.Type == schema.ContentBlockTypeServerToolResult && block.ServerToolResult != nil {
			flushPending()
			content, _ := sonic.MarshalString(block.ServerToolResult.Result)
			result = append(result, &tracespec.ModelMessage{
				Role:       "tool",
				ToolCallID: block.ServerToolResult.CallID,
				Name:       block.ServerToolResult.Name,
				Content:    content,
			})
		} else if block.Type == schema.ContentBlockTypeMCPToolResult && block.MCPToolResult != nil {
			flushPending()
			result = append(result, &tracespec.ModelMessage{
				Role:       "tool",
				ToolCallID: block.MCPToolResult.CallID,
				Name:       block.MCPToolResult.Name,
				Content:    block.MCPToolResult.Result,
			})
		} else {
			if isToolCallBlock(block.Type) && pendingToolCallType != "" && pendingToolCallType != block.Type {
				flushPending()
			}
			if isToolCallBlock(block.Type) {
				pendingToolCallType = block.Type
			}
			pendingBlocks = append(pendingBlocks, block)
		}
	}
	flushPending()

	if len(result) == 0 {
		result = append(result, &tracespec.ModelMessage{Role: string(message.Role)})
	}

	return result
}

func flatExpandAgenticMessages(messages []*schema.AgenticMessage) []*tracespec.ModelMessage {
	var result []*tracespec.ModelMessage
	for _, msg := range messages {
		result = append(result, expandAgenticModelMessage(msg)...)
	}
	return result
}

func convertAgenticModelCallOption(config *model.AgenticConfig) *tracespec.ModelCallOption {
	if config == nil {
		return nil
	}
	return &tracespec.ModelCallOption{
		Temperature: config.Temperature,
		MaxTokens:   int64(config.MaxTokens),
		TopP:        config.TopP,
	}
}

// AgenticPrompt

func convertAgenticPromptInput(input *prompt.AgenticCallbackInput) *tracespec.PromptInput {
	if input == nil {
		return nil
	}
	return &tracespec.PromptInput{
		Templates: iterSlice(input.Templates, convertAgenticTemplate),
		Arguments: convertPromptArguments(input.Variables),
	}
}

func convertAgenticPromptOutput(output *prompt.AgenticCallbackOutput) *tracespec.PromptOutput {
	if output == nil {
		return nil
	}

	var prompts []*tracespec.ModelMessage
	for _, msg := range output.Result {
		prompts = append(prompts, expandAgenticModelMessage(msg)...)
	}

	return &tracespec.PromptOutput{
		Prompts: prompts,
	}
}

func convertAgenticTemplate(template schema.AgenticMessagesTemplate) *tracespec.ModelMessage {
	if template == nil {
		return nil
	}

	msg, ok := template.(*schema.AgenticMessage)
	if !ok || msg == nil {
		return nil
	}

	result := &tracespec.ModelMessage{
		Role: string(msg.Role),
	}

	var parts []*tracespec.ModelMessagePart
	for _, block := range msg.ContentBlocks {
		if block == nil {
			continue
		}
		sign := getBase64AgenticThoughtSignatureFromExtra(block.Extra)

		switch block.Type {
		case schema.ContentBlockTypeUserInputText:
			if block.UserInputText != nil {
				parts = append(parts, &tracespec.ModelMessagePart{
					Type:      tracespec.ModelMessagePartTypeText,
					Text:      block.UserInputText.Text,
					Signature: sign,
				})
			}

		case schema.ContentBlockTypeUserInputImage:
			if block.UserInputImage != nil {
				url := block.UserInputImage.URL
				if url == "" && block.UserInputImage.Base64Data != "" {
					url = fmt.Sprintf("data:%s;base64,%s", block.UserInputImage.MIMEType, block.UserInputImage.Base64Data)
				}
				parts = append(parts, &tracespec.ModelMessagePart{
					Type: tracespec.ModelMessagePartTypeImage,
					ImageURL: &tracespec.ModelImageURL{
						URL:    url,
						Detail: string(block.UserInputImage.Detail),
					},
					Signature: sign,
				})
			}

		case schema.ContentBlockTypeUserInputAudio:
			if block.UserInputAudio != nil {
				url := block.UserInputAudio.URL
				if url == "" && block.UserInputAudio.Base64Data != "" {
					url = fmt.Sprintf("data:%s;base64,%s", block.UserInputAudio.MIMEType, block.UserInputAudio.Base64Data)
				}
				parts = append(parts, &tracespec.ModelMessagePart{
					Type:      tracespec.ModelMessagePartTypeAudio,
					AudioURL:  &tracespec.ModelAudioURL{URL: url},
					Signature: sign,
				})
			}

		case schema.ContentBlockTypeUserInputVideo:
			if block.UserInputVideo != nil {
				url := block.UserInputVideo.URL
				if url == "" && block.UserInputVideo.Base64Data != "" {
					url = fmt.Sprintf("data:%s;base64,%s", block.UserInputVideo.MIMEType, block.UserInputVideo.Base64Data)
				}
				parts = append(parts, &tracespec.ModelMessagePart{
					Type:      tracespec.ModelMessagePartTypeVideo,
					VideoURL:  &tracespec.ModelVideoURL{URL: url},
					Signature: sign,
				})
			}

		case schema.ContentBlockTypeUserInputFile:
			if block.UserInputFile != nil {
				url := block.UserInputFile.URL
				if url == "" && block.UserInputFile.Base64Data != "" {
					url = fmt.Sprintf("data:%s;base64,%s", block.UserInputFile.MIMEType, block.UserInputFile.Base64Data)
				}
				parts = append(parts, &tracespec.ModelMessagePart{
					Type:      tracespec.ModelMessagePartTypeFile,
					FileURL:   &tracespec.ModelFileURL{URL: url},
					Signature: sign,
				})
			}
		}
	}

	result.Parts = parts
	return result
}

func iterSlice[A, B any](sa []A, fb func(a A) B) []B {
	r := make([]B, len(sa))
	for i := range sa {
		r[i] = fb(sa[i])
	}

	return r
}

func iterSliceWithCtx[A, B any](ctx context.Context, sa []A, fb func(ctx context.Context, a A) B) []B {
	r := make([]B, len(sa))
	for i := range sa {
		r[i] = fb(ctx, sa[i])
	}

	return r
}

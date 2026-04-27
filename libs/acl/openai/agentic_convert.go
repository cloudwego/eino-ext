/*
 * Copyright 2024 CloudWeGo Authors
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
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"
)

func agenticMessagesToMessages(in []*schema.AgenticMessage) ([]*schema.Message, error) {
	var result []*schema.Message

	for _, msg := range in {
		ms, err := agenticMessageToMessages(msg)
		if err != nil {
			return nil, err
		}
		result = append(result, ms...)
	}

	return result, nil
}

func messageToAgenticMessage(msg *schema.Message) (*schema.AgenticMessage, error) {
	out := &schema.AgenticMessage{
		Role:  schema.AgenticRoleTypeAssistant,
		Extra: msg.Extra,
	}

	multiContentHasReasoning := multiContentContainsType(msg.AssistantGenMultiContent, schema.ChatMessagePartTypeReasoning)
	multiContentHasText := multiContentContainsType(msg.AssistantGenMultiContent, schema.ChatMessagePartTypeText)

	if !multiContentHasReasoning && len(msg.ReasoningContent) > 0 {
		out.ContentBlocks = append(out.ContentBlocks,
			schema.NewContentBlock(&schema.Reasoning{Text: msg.ReasoningContent}))
	}

	if len(msg.AssistantGenMultiContent) > 0 {
		for _, part := range msg.AssistantGenMultiContent {
			block, err := outputPartToContentBlock(part)
			if err != nil {
				return nil, err
			}
			out.ContentBlocks = append(out.ContentBlocks, block)
		}
	}

	if !multiContentHasText && len(msg.Content) > 0 {
		out.ContentBlocks = append(out.ContentBlocks,
			schema.NewContentBlock(&schema.AssistantGenText{Text: msg.Content}))
	}

	for _, tc := range msg.ToolCalls {
		block := schema.NewContentBlock(&schema.FunctionToolCall{
			CallID:    tc.ID,
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		})
		block.Extra = tc.Extra
		out.ContentBlocks = append(out.ContentBlocks, block)
	}

	if msg.ResponseMeta != nil {
		out.ResponseMeta = &schema.AgenticResponseMeta{
			TokenUsage: msg.ResponseMeta.Usage,
		}
	}

	return out, nil
}

type chunkConverter struct {
	curIndex              int
	lastContentType       schema.ContentBlockType
	lastContentPartIndex  int
	lastToolCallIndex     int
	started               bool
	inToolCalls           bool
}

func newChunkConverter() *chunkConverter {
	return &chunkConverter{}
}

func (c *chunkConverter) advanceContent(blockType schema.ContentBlockType, sourceIndex int) {
	if !c.started {
		c.started = true
		c.lastContentType = blockType
		c.lastContentPartIndex = sourceIndex
		return
	}
	if blockType != c.lastContentType || sourceIndex != c.lastContentPartIndex {
		c.curIndex++
		c.lastContentType = blockType
		c.lastContentPartIndex = sourceIndex
	}
}

func (c *chunkConverter) advanceToolCall(toolCallIndex int) {
	if !c.inToolCalls {
		c.inToolCalls = true
		if c.started {
			c.curIndex++
		} else {
			c.started = true
		}
		c.lastToolCallIndex = toolCallIndex
		return
	}
	if toolCallIndex != c.lastToolCallIndex {
		c.curIndex++
		c.lastToolCallIndex = toolCallIndex
	}
}

func (c *chunkConverter) convert(msg *schema.Message) (*schema.AgenticMessage, error) {
	out := &schema.AgenticMessage{
		Role:  schema.AgenticRoleTypeAssistant,
		Extra: msg.Extra,
	}

	multiContentHasReasoning := multiContentContainsType(msg.AssistantGenMultiContent, schema.ChatMessagePartTypeReasoning)
	multiContentHasText := multiContentContainsType(msg.AssistantGenMultiContent, schema.ChatMessagePartTypeText)

	if !multiContentHasReasoning && len(msg.ReasoningContent) > 0 {
		c.advanceContent(schema.ContentBlockTypeReasoning, -1)
		block := schema.NewContentBlock(&schema.Reasoning{Text: msg.ReasoningContent})
		block.StreamingMeta = &schema.StreamingMeta{Index: c.curIndex}
		out.ContentBlocks = append(out.ContentBlocks, block)
	}

	if len(msg.AssistantGenMultiContent) > 0 {
		for _, part := range msg.AssistantGenMultiContent {
			sourceIndex := 0
			if part.StreamingMeta != nil {
				sourceIndex = part.StreamingMeta.Index
			}
			block, err := outputPartToContentBlock(part)
			if err != nil {
				return nil, err
			}
			c.advanceContent(block.Type, sourceIndex)
			block.StreamingMeta = &schema.StreamingMeta{Index: c.curIndex}
			out.ContentBlocks = append(out.ContentBlocks, block)
		}
	}

	if !multiContentHasText && len(msg.Content) > 0 {
		c.advanceContent(schema.ContentBlockTypeAssistantGenText, -1)
		block := schema.NewContentBlock(&schema.AssistantGenText{Text: msg.Content})
		block.StreamingMeta = &schema.StreamingMeta{Index: c.curIndex}
		out.ContentBlocks = append(out.ContentBlocks, block)
	}

	for _, tc := range msg.ToolCalls {
		toolCallIndex := 0
		if tc.Index != nil {
			toolCallIndex = *tc.Index
		}
		c.advanceToolCall(toolCallIndex)
		block := schema.NewContentBlock(&schema.FunctionToolCall{
			CallID:    tc.ID,
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		})
		block.Extra = tc.Extra
		block.StreamingMeta = &schema.StreamingMeta{Index: c.curIndex}
		out.ContentBlocks = append(out.ContentBlocks, block)
	}

	if msg.ResponseMeta != nil {
		out.ResponseMeta = &schema.AgenticResponseMeta{
			TokenUsage: msg.ResponseMeta.Usage,
		}
	}

	return out, nil
}

func multiContentContainsType(parts []schema.MessageOutputPart, typ schema.ChatMessagePartType) bool {
	for _, part := range parts {
		if part.Type == typ {
			return true
		}
	}
	return false
}

func agenticMessageToMessages(msg *schema.AgenticMessage) ([]*schema.Message, error) {
	switch msg.Role {
	case schema.AgenticRoleTypeSystem:
		return agenticSystemToMessages(msg)
	case schema.AgenticRoleTypeUser:
		return agenticUserToMessages(msg)
	case schema.AgenticRoleTypeAssistant:
		return agenticAssistantToMessages(msg)
	default:
		return nil, fmt.Errorf("unsupported agentic message role: %s", msg.Role)
	}
}

func agenticSystemToMessages(msg *schema.AgenticMessage) ([]*schema.Message, error) {
	var parts []string
	for _, block := range msg.ContentBlocks {
		switch block.Type {
		case schema.ContentBlockTypeUserInputText:
			if block.UserInputText != nil {
				parts = append(parts, block.UserInputText.Text)
			}
		default:
			return nil, fmt.Errorf("unsupported content block type %q in system message", block.Type)
		}
	}

	return []*schema.Message{{
		Role:    schema.System,
		Content: strings.Join(parts, "\n"),
		Extra:   msg.Extra,
	}}, nil
}

func agenticUserToMessages(msg *schema.AgenticMessage) ([]*schema.Message, error) {
	var result []*schema.Message
	var inputParts []schema.MessageInputPart

	flushInputParts := func() {
		if len(inputParts) == 0 {
			return
		}
		result = append(result, &schema.Message{
			Role:                  schema.User,
			UserInputMultiContent: inputParts,
			Extra:                 msg.Extra,
		})
		inputParts = nil
	}

	for _, block := range msg.ContentBlocks {
		switch block.Type {
		case schema.ContentBlockTypeFunctionToolResult:
			flushInputParts()
			result = append(result, &schema.Message{
				Role:       schema.Tool,
				Content:    block.FunctionToolResult.Result,
				ToolCallID: block.FunctionToolResult.CallID,
				ToolName:   block.FunctionToolResult.Name,
				Extra:      block.Extra,
			})

		case schema.ContentBlockTypeUserInputText:
			inputParts = append(inputParts, schema.MessageInputPart{
				Type:  schema.ChatMessagePartTypeText,
				Text:  block.UserInputText.Text,
				Extra: block.Extra,
			})

		case schema.ContentBlockTypeUserInputImage:
			inputParts = append(inputParts, imageBlockToInputPart(block.UserInputImage, block.Extra))

		case schema.ContentBlockTypeUserInputAudio:
			inputParts = append(inputParts, audioBlockToInputPart(block.UserInputAudio, block.Extra))

		case schema.ContentBlockTypeUserInputVideo:
			inputParts = append(inputParts, videoBlockToInputPart(block.UserInputVideo, block.Extra))

		default:
			return nil, fmt.Errorf("unsupported content block type %q in user message", block.Type)
		}
	}

	flushInputParts()

	return result, nil
}

func agenticAssistantToMessages(msg *schema.AgenticMessage) ([]*schema.Message, error) {
	m := &schema.Message{
		Role:  schema.Assistant,
		Extra: msg.Extra,
	}

	for _, block := range msg.ContentBlocks {
		switch block.Type {
		case schema.ContentBlockTypeAssistantGenText:
			m.AssistantGenMultiContent = append(m.AssistantGenMultiContent, schema.MessageOutputPart{
				Type:  schema.ChatMessagePartTypeText,
				Text:  block.AssistantGenText.Text,
				Extra: block.Extra,
			})

		case schema.ContentBlockTypeReasoning:
			m.AssistantGenMultiContent = append(m.AssistantGenMultiContent, schema.MessageOutputPart{
				Type: schema.ChatMessagePartTypeReasoning,
				Reasoning: &schema.MessageOutputReasoning{
					Text:      block.Reasoning.Text,
					Signature: block.Reasoning.Signature,
				},
				Extra: block.Extra,
			})

		case schema.ContentBlockTypeFunctionToolCall:
			m.ToolCalls = append(m.ToolCalls, schema.ToolCall{
				ID: block.FunctionToolCall.CallID,
				Function: schema.FunctionCall{
					Name:      block.FunctionToolCall.Name,
					Arguments: block.FunctionToolCall.Arguments,
				},
				Extra: block.Extra,
			})

		case schema.ContentBlockTypeAssistantGenImage:
			m.AssistantGenMultiContent = append(m.AssistantGenMultiContent,
				assistantGenImageToOutputPart(block.AssistantGenImage, block.Extra))

		case schema.ContentBlockTypeAssistantGenAudio:
			m.AssistantGenMultiContent = append(m.AssistantGenMultiContent,
				assistantGenAudioToOutputPart(block.AssistantGenAudio, block.Extra))

		case schema.ContentBlockTypeAssistantGenVideo:
			m.AssistantGenMultiContent = append(m.AssistantGenMultiContent,
				assistantGenVideoToOutputPart(block.AssistantGenVideo, block.Extra))

		default:
			return nil, fmt.Errorf("unsupported content block type %q in assistant message", block.Type)
		}
	}

	if msg.ResponseMeta != nil {
		m.ResponseMeta = &schema.ResponseMeta{
			Usage: msg.ResponseMeta.TokenUsage,
		}
	}

	return []*schema.Message{m}, nil
}

func imageBlockToInputPart(img *schema.UserInputImage, extra map[string]any) schema.MessageInputPart {
	part := schema.MessageInputPart{
		Type: schema.ChatMessagePartTypeImageURL,
		Image: &schema.MessageInputImage{
			MessagePartCommon: schema.MessagePartCommon{
				MIMEType: img.MIMEType,
			},
			Detail: img.Detail,
		},
		Extra: extra,
	}

	if img.URL != "" {
		part.Image.URL = &img.URL
	} else if img.Base64Data != "" {
		part.Image.Base64Data = &img.Base64Data
	}

	return part
}

func audioBlockToInputPart(audio *schema.UserInputAudio, extra map[string]any) schema.MessageInputPart {
	part := schema.MessageInputPart{
		Type: schema.ChatMessagePartTypeAudioURL,
		Audio: &schema.MessageInputAudio{
			MessagePartCommon: schema.MessagePartCommon{
				MIMEType: audio.MIMEType,
			},
		},
		Extra: extra,
	}

	if audio.URL != "" {
		part.Audio.URL = &audio.URL
	} else if audio.Base64Data != "" {
		part.Audio.Base64Data = &audio.Base64Data
	}

	return part
}

func videoBlockToInputPart(video *schema.UserInputVideo, extra map[string]any) schema.MessageInputPart {
	part := schema.MessageInputPart{
		Type: schema.ChatMessagePartTypeVideoURL,
		Video: &schema.MessageInputVideo{
			MessagePartCommon: schema.MessagePartCommon{
				MIMEType: video.MIMEType,
			},
		},
		Extra: extra,
	}

	if video.URL != "" {
		part.Video.URL = &video.URL
	} else if video.Base64Data != "" {
		part.Video.Base64Data = &video.Base64Data
	}

	return part
}

func assistantGenImageToOutputPart(img *schema.AssistantGenImage, extra map[string]any) schema.MessageOutputPart {
	part := schema.MessageOutputPart{
		Type: schema.ChatMessagePartTypeImageURL,
		Image: &schema.MessageOutputImage{
			MessagePartCommon: schema.MessagePartCommon{
				MIMEType: img.MIMEType,
			},
		},
		Extra: extra,
	}
	if img.URL != "" {
		part.Image.URL = &img.URL
	} else if img.Base64Data != "" {
		part.Image.Base64Data = &img.Base64Data
	}
	return part
}

func assistantGenAudioToOutputPart(audio *schema.AssistantGenAudio, extra map[string]any) schema.MessageOutputPart {
	part := schema.MessageOutputPart{
		Type: schema.ChatMessagePartTypeAudioURL,
		Audio: &schema.MessageOutputAudio{
			MessagePartCommon: schema.MessagePartCommon{
				MIMEType: audio.MIMEType,
			},
		},
		Extra: extra,
	}
	if audio.URL != "" {
		part.Audio.URL = &audio.URL
	} else if audio.Base64Data != "" {
		part.Audio.Base64Data = &audio.Base64Data
	}
	return part
}

func assistantGenVideoToOutputPart(video *schema.AssistantGenVideo, extra map[string]any) schema.MessageOutputPart {
	part := schema.MessageOutputPart{
		Type: schema.ChatMessagePartTypeVideoURL,
		Video: &schema.MessageOutputVideo{
			MessagePartCommon: schema.MessagePartCommon{
				MIMEType: video.MIMEType,
			},
		},
		Extra: extra,
	}
	if video.URL != "" {
		part.Video.URL = &video.URL
	} else if video.Base64Data != "" {
		part.Video.Base64Data = &video.Base64Data
	}
	return part
}

func outputPartToContentBlock(part schema.MessageOutputPart) (*schema.ContentBlock, error) {
	var block *schema.ContentBlock
	switch part.Type {
	case schema.ChatMessagePartTypeText:
		block = schema.NewContentBlock(&schema.AssistantGenText{
			Text: part.Text,
		})
	case schema.ChatMessagePartTypeReasoning:
		block = schema.NewContentBlock(&schema.Reasoning{
			Text:      part.Reasoning.Text,
			Signature: part.Reasoning.Signature,
		})
	case schema.ChatMessagePartTypeImageURL:
		block = schema.NewContentBlock(&schema.AssistantGenImage{
			URL:        ptrToString(part.Image.URL),
			Base64Data: ptrToString(part.Image.Base64Data),
			MIMEType:   part.Image.MIMEType,
		})
	case schema.ChatMessagePartTypeAudioURL:
		block = schema.NewContentBlock(&schema.AssistantGenAudio{
			URL:        ptrToString(part.Audio.URL),
			Base64Data: ptrToString(part.Audio.Base64Data),
			MIMEType:   part.Audio.MIMEType,
		})
	case schema.ChatMessagePartTypeVideoURL:
		block = schema.NewContentBlock(&schema.AssistantGenVideo{
			URL:        ptrToString(part.Video.URL),
			Base64Data: ptrToString(part.Video.Base64Data),
			MIMEType:   part.Video.MIMEType,
		})
	default:
		return nil, fmt.Errorf("unsupported output part type %q in assistant message", part.Type)
	}
	block.Extra = part.Extra
	return block, nil
}

func ptrToString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

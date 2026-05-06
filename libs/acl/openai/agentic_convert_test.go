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
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
)

func Test_agenticMessagesToMessages(t *testing.T) {
	t.Run("system message with text", func(t *testing.T) {
		msgs := []*schema.AgenticMessage{
			{
				Role: schema.AgenticRoleTypeSystem,
				ContentBlocks: []*schema.ContentBlock{
					schema.NewContentBlock(&schema.UserInputText{Text: "You are a helpful assistant."}),
					schema.NewContentBlock(&schema.UserInputText{Text: "Be concise."}),
				},
			},
		}

		result, err := agenticMessagesToMessages(msgs)
		assert.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, schema.System, result[0].Role)
		assert.Equal(t, "You are a helpful assistant.\nBe concise.", result[0].Content)
	})

	t.Run("system message with unsupported type", func(t *testing.T) {
		msgs := []*schema.AgenticMessage{
			{
				Role: schema.AgenticRoleTypeSystem,
				ContentBlocks: []*schema.ContentBlock{
					schema.NewContentBlock(&schema.FunctionToolCall{CallID: "1", Name: "fn"}),
				},
			},
		}

		_, err := agenticMessagesToMessages(msgs)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported content block type")
	})

	t.Run("user message with text only", func(t *testing.T) {
		msgs := []*schema.AgenticMessage{
			{
				Role: schema.AgenticRoleTypeUser,
				ContentBlocks: []*schema.ContentBlock{
					schema.NewContentBlock(&schema.UserInputText{Text: "Hello"}),
				},
			},
		}

		result, err := agenticMessagesToMessages(msgs)
		assert.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, schema.User, result[0].Role)
		assert.Len(t, result[0].UserInputMultiContent, 1)
		assert.Equal(t, "Hello", result[0].UserInputMultiContent[0].Text)
	})

	t.Run("user message with multimodal content", func(t *testing.T) {
		msgs := []*schema.AgenticMessage{
			{
				Role: schema.AgenticRoleTypeUser,
				ContentBlocks: []*schema.ContentBlock{
					schema.NewContentBlock(&schema.UserInputText{Text: "Describe this image"}),
					schema.NewContentBlock(&schema.UserInputImage{URL: "https://example.com/img.png", Detail: schema.ImageURLDetailHigh}),
				},
			},
		}

		result, err := agenticMessagesToMessages(msgs)
		assert.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, schema.User, result[0].Role)
		assert.Len(t, result[0].UserInputMultiContent, 2)
		assert.Equal(t, schema.ChatMessagePartTypeText, result[0].UserInputMultiContent[0].Type)
		assert.Equal(t, schema.ChatMessagePartTypeImageURL, result[0].UserInputMultiContent[1].Type)
		assert.Equal(t, "https://example.com/img.png", *result[0].UserInputMultiContent[1].Image.URL)
		assert.Equal(t, schema.ImageURLDetailHigh, result[0].UserInputMultiContent[1].Image.Detail)
	})

	t.Run("user message with base64 image", func(t *testing.T) {
		msgs := []*schema.AgenticMessage{
			{
				Role: schema.AgenticRoleTypeUser,
				ContentBlocks: []*schema.ContentBlock{
					schema.NewContentBlock(&schema.UserInputImage{Base64Data: "abc123", MIMEType: "image/png"}),
				},
			},
		}

		result, err := agenticMessagesToMessages(msgs)
		assert.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Len(t, result[0].UserInputMultiContent, 1)
		assert.Equal(t, "abc123", *result[0].UserInputMultiContent[0].Image.Base64Data)
		assert.Equal(t, "image/png", result[0].UserInputMultiContent[0].Image.MIMEType)
	})

	t.Run("user message with audio base64", func(t *testing.T) {
		msgs := []*schema.AgenticMessage{
			{
				Role: schema.AgenticRoleTypeUser,
				ContentBlocks: []*schema.ContentBlock{
					schema.NewContentBlock(&schema.UserInputAudio{Base64Data: "audiodata", MIMEType: "audio/wav"}),
				},
			},
		}

		result, err := agenticMessagesToMessages(msgs)
		assert.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Len(t, result[0].UserInputMultiContent, 1)
		assert.Equal(t, schema.ChatMessagePartTypeAudioURL, result[0].UserInputMultiContent[0].Type)
		assert.Equal(t, "audiodata", *result[0].UserInputMultiContent[0].Audio.Base64Data)
		assert.Equal(t, "audio/wav", result[0].UserInputMultiContent[0].Audio.MIMEType)
	})

	t.Run("user message with video URL", func(t *testing.T) {
		msgs := []*schema.AgenticMessage{
			{
				Role: schema.AgenticRoleTypeUser,
				ContentBlocks: []*schema.ContentBlock{
					schema.NewContentBlock(&schema.UserInputVideo{URL: "https://example.com/vid.mp4"}),
				},
			},
		}

		result, err := agenticMessagesToMessages(msgs)
		assert.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Len(t, result[0].UserInputMultiContent, 1)
		assert.Equal(t, schema.ChatMessagePartTypeVideoURL, result[0].UserInputMultiContent[0].Type)
		assert.Equal(t, "https://example.com/vid.mp4", *result[0].UserInputMultiContent[0].Video.URL)
	})

	t.Run("user message with tool results", func(t *testing.T) {
		msgs := []*schema.AgenticMessage{
			{
				Role: schema.AgenticRoleTypeUser,
				ContentBlocks: []*schema.ContentBlock{
					schema.NewContentBlock(&schema.FunctionToolResult{
						CallID: "call_1",
						Name:   "get_weather",
						Result: `{"temp": 22}`,
					}),
					schema.NewContentBlock(&schema.FunctionToolResult{
						CallID: "call_2",
						Name:   "get_time",
						Result: `{"time": "12:00"}`,
					}),
				},
			},
		}

		result, err := agenticMessagesToMessages(msgs)
		assert.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, schema.Tool, result[0].Role)
		assert.Equal(t, "call_1", result[0].ToolCallID)
		assert.Equal(t, `{"temp": 22}`, result[0].Content)
		assert.Equal(t, "get_weather", result[0].ToolName)
		assert.Equal(t, schema.Tool, result[1].Role)
		assert.Equal(t, "call_2", result[1].ToolCallID)
	})

	t.Run("user message with mixed tool results and text", func(t *testing.T) {
		msgs := []*schema.AgenticMessage{
			{
				Role: schema.AgenticRoleTypeUser,
				ContentBlocks: []*schema.ContentBlock{
					schema.NewContentBlock(&schema.FunctionToolResult{
						CallID: "call_1",
						Name:   "fn",
						Result: "result",
					}),
					schema.NewContentBlock(&schema.UserInputText{Text: "Now answer"}),
				},
			},
		}

		result, err := agenticMessagesToMessages(msgs)
		assert.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, schema.Tool, result[0].Role)
		assert.Equal(t, schema.User, result[1].Role)
		assert.Equal(t, "Now answer", result[1].UserInputMultiContent[0].Text)
	})

	t.Run("assistant message with text and tool calls", func(t *testing.T) {
		msgs := []*schema.AgenticMessage{
			{
				Role: schema.AgenticRoleTypeAssistant,
				ContentBlocks: []*schema.ContentBlock{
					schema.NewContentBlock(&schema.AssistantGenText{Text: "Let me search"}),
					schema.NewContentBlock(&schema.FunctionToolCall{
						CallID:    "call_1",
						Name:      "search",
						Arguments: `{"q": "test"}`,
					}),
				},
			},
		}

		result, err := agenticMessagesToMessages(msgs)
		assert.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, schema.Assistant, result[0].Role)
		assert.Len(t, result[0].AssistantGenMultiContent, 1)
		assert.Equal(t, schema.ChatMessagePartTypeText, result[0].AssistantGenMultiContent[0].Type)
		assert.Equal(t, "Let me search", result[0].AssistantGenMultiContent[0].Text)
		assert.Len(t, result[0].ToolCalls, 1)
		assert.Equal(t, "call_1", result[0].ToolCalls[0].ID)
		assert.Equal(t, "search", result[0].ToolCalls[0].Function.Name)
		assert.Equal(t, `{"q": "test"}`, result[0].ToolCalls[0].Function.Arguments)
	})

	t.Run("assistant message with reasoning", func(t *testing.T) {
		msgs := []*schema.AgenticMessage{
			{
				Role: schema.AgenticRoleTypeAssistant,
				ContentBlocks: []*schema.ContentBlock{
					schema.NewContentBlock(&schema.Reasoning{Text: "thinking...", Signature: "sig123"}),
					schema.NewContentBlock(&schema.AssistantGenText{Text: "answer"}),
				},
			},
		}

		result, err := agenticMessagesToMessages(msgs)
		assert.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Len(t, result[0].AssistantGenMultiContent, 2)
		assert.Equal(t, schema.ChatMessagePartTypeReasoning, result[0].AssistantGenMultiContent[0].Type)
		assert.Equal(t, "thinking...", result[0].AssistantGenMultiContent[0].Reasoning.Text)
		assert.Equal(t, "sig123", result[0].AssistantGenMultiContent[0].Reasoning.Signature)
		assert.Equal(t, schema.ChatMessagePartTypeText, result[0].AssistantGenMultiContent[1].Type)
		assert.Equal(t, "answer", result[0].AssistantGenMultiContent[1].Text)
	})

	t.Run("unsupported role", func(t *testing.T) {
		msgs := []*schema.AgenticMessage{
			{Role: "unknown"},
		}
		_, err := agenticMessagesToMessages(msgs)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported agentic message role")
	})
}

func Test_messageToAgenticMessage(t *testing.T) {
	t.Run("text response", func(t *testing.T) {
		msg := &schema.Message{
			Role:    schema.Assistant,
			Content: "Hello!",
			ResponseMeta: &schema.ResponseMeta{
				Usage: &schema.TokenUsage{
					PromptTokens:     10,
					CompletionTokens: 5,
					TotalTokens:      15,
				},
			},
		}

		out, err := messageToAgenticMessage(msg)
		assert.NoError(t, err)
		assert.Equal(t, schema.AgenticRoleTypeAssistant, out.Role)
		assert.Len(t, out.ContentBlocks, 1)
		assert.Equal(t, schema.ContentBlockTypeAssistantGenText, out.ContentBlocks[0].Type)
		assert.Equal(t, "Hello!", out.ContentBlocks[0].AssistantGenText.Text)
		assert.Equal(t, 15, out.ResponseMeta.TokenUsage.TotalTokens)
	})

	t.Run("reasoning plus text", func(t *testing.T) {
		msg := &schema.Message{
			Role:             schema.Assistant,
			Content:          "2",
			ReasoningContent: "1+1=2",
		}

		out, err := messageToAgenticMessage(msg)
		assert.NoError(t, err)
		assert.Len(t, out.ContentBlocks, 2)
		assert.Equal(t, schema.ContentBlockTypeReasoning, out.ContentBlocks[0].Type)
		assert.Equal(t, "1+1=2", out.ContentBlocks[0].Reasoning.Text)
		assert.Equal(t, schema.ContentBlockTypeAssistantGenText, out.ContentBlocks[1].Type)
		assert.Equal(t, "2", out.ContentBlocks[1].AssistantGenText.Text)
	})

	t.Run("tool calls", func(t *testing.T) {
		msg := &schema.Message{
			Role: schema.Assistant,
			ToolCalls: []schema.ToolCall{
				{
					ID:       "call_1",
					Function: schema.FunctionCall{Name: "search", Arguments: `{"q": "test"}`},
				},
				{
					ID:       "call_2",
					Function: schema.FunctionCall{Name: "calc", Arguments: `{"expr": "1+1"}`},
				},
			},
		}

		out, err := messageToAgenticMessage(msg)
		assert.NoError(t, err)
		assert.Len(t, out.ContentBlocks, 2)
		assert.Equal(t, schema.ContentBlockTypeFunctionToolCall, out.ContentBlocks[0].Type)
		assert.Equal(t, "call_1", out.ContentBlocks[0].FunctionToolCall.CallID)
		assert.Equal(t, "search", out.ContentBlocks[0].FunctionToolCall.Name)
		assert.Equal(t, schema.ContentBlockTypeFunctionToolCall, out.ContentBlocks[1].Type)
		assert.Equal(t, "call_2", out.ContentBlocks[1].FunctionToolCall.CallID)
	})

	t.Run("nil response meta", func(t *testing.T) {
		msg := &schema.Message{
			Role:    schema.Assistant,
			Content: "hi",
		}

		out, err := messageToAgenticMessage(msg)
		assert.NoError(t, err)
		assert.Nil(t, out.ResponseMeta)
	})
}

func Test_messageToAgenticMessageAlwaysAssistant(t *testing.T) {
	msg := &schema.Message{
		Role:    schema.User,
		Content: "test",
	}
	out, err := messageToAgenticMessage(msg)
	assert.NoError(t, err)
	assert.Equal(t, schema.AgenticRoleTypeAssistant, out.Role)
}

func Test_messageToAgenticMessage_MultiContentPriority(t *testing.T) {
	t.Run("multiContent with reasoning skips ReasoningContent", func(t *testing.T) {
		msg := &schema.Message{
			Role:             schema.Assistant,
			ReasoningContent: "should be ignored",
			AssistantGenMultiContent: []schema.MessageOutputPart{
				{Type: schema.ChatMessagePartTypeReasoning, Reasoning: &schema.MessageOutputReasoning{Text: "from multi"}},
				{Type: schema.ChatMessagePartTypeText, Text: "answer"},
			},
		}

		out, err := messageToAgenticMessage(msg)
		assert.NoError(t, err)
		assert.Len(t, out.ContentBlocks, 2)
		assert.Equal(t, schema.ContentBlockTypeReasoning, out.ContentBlocks[0].Type)
		assert.Equal(t, "from multi", out.ContentBlocks[0].Reasoning.Text)
		assert.Equal(t, schema.ContentBlockTypeAssistantGenText, out.ContentBlocks[1].Type)
		assert.Equal(t, "answer", out.ContentBlocks[1].AssistantGenText.Text)
	})

	t.Run("multiContent with text skips Content", func(t *testing.T) {
		msg := &schema.Message{
			Role:    schema.Assistant,
			Content: "should be ignored",
			AssistantGenMultiContent: []schema.MessageOutputPart{
				{Type: schema.ChatMessagePartTypeText, Text: "from multi"},
			},
		}

		out, err := messageToAgenticMessage(msg)
		assert.NoError(t, err)
		assert.Len(t, out.ContentBlocks, 1)
		assert.Equal(t, "from multi", out.ContentBlocks[0].AssistantGenText.Text)
	})

	t.Run("multiContent without text reads Content", func(t *testing.T) {
		msg := &schema.Message{
			Role:    schema.Assistant,
			Content: "extra text",
			AssistantGenMultiContent: []schema.MessageOutputPart{
				{Type: schema.ChatMessagePartTypeReasoning, Reasoning: &schema.MessageOutputReasoning{Text: "thinking"}},
			},
		}

		out, err := messageToAgenticMessage(msg)
		assert.NoError(t, err)
		assert.Len(t, out.ContentBlocks, 2)
		assert.Equal(t, schema.ContentBlockTypeReasoning, out.ContentBlocks[0].Type)
		assert.Equal(t, schema.ContentBlockTypeAssistantGenText, out.ContentBlocks[1].Type)
		assert.Equal(t, "extra text", out.ContentBlocks[1].AssistantGenText.Text)
	})

	t.Run("multiContent without reasoning reads ReasoningContent", func(t *testing.T) {
		msg := &schema.Message{
			Role:             schema.Assistant,
			ReasoningContent: "thinking",
			AssistantGenMultiContent: []schema.MessageOutputPart{
				{Type: schema.ChatMessagePartTypeText, Text: "answer"},
			},
		}

		out, err := messageToAgenticMessage(msg)
		assert.NoError(t, err)
		assert.Len(t, out.ContentBlocks, 2)
		assert.Equal(t, schema.ContentBlockTypeReasoning, out.ContentBlocks[0].Type)
		assert.Equal(t, "thinking", out.ContentBlocks[0].Reasoning.Text)
		assert.Equal(t, schema.ContentBlockTypeAssistantGenText, out.ContentBlocks[1].Type)
		assert.Equal(t, "answer", out.ContentBlocks[1].AssistantGenText.Text)
	})

	t.Run("no StreamingMeta on complete message blocks", func(t *testing.T) {
		msg := &schema.Message{
			Role:    schema.Assistant,
			Content: "hello",
		}

		out, err := messageToAgenticMessage(msg)
		assert.NoError(t, err)
		assert.Nil(t, out.ContentBlocks[0].StreamingMeta)
	})
}

func TestChunkConverter(t *testing.T) {
	intPtr := func(i int) *int { return &i }

	t.Run("reasoning then text streaming", func(t *testing.T) {
		conv := newChunkConverter()

		// Chunk 1: reasoning delta
		chunk1 := &schema.Message{
			Role:             schema.Assistant,
			ReasoningContent: "let me think",
		}
		out1, err := conv.convert(chunk1)
		assert.NoError(t, err)
		assert.Len(t, out1.ContentBlocks, 1)
		assert.Equal(t, 0, out1.ContentBlocks[0].StreamingMeta.Index)

		// Chunk 2: reasoning continues (same block)
		chunk2 := &schema.Message{
			Role:             schema.Assistant,
			ReasoningContent: " more",
		}
		out2, err := conv.convert(chunk2)
		assert.NoError(t, err)
		assert.Equal(t, 0, out2.ContentBlocks[0].StreamingMeta.Index)

		// Chunk 3: text starts (new block)
		chunk3 := &schema.Message{
			Role:    schema.Assistant,
			Content: "the answer",
		}
		out3, err := conv.convert(chunk3)
		assert.NoError(t, err)
		assert.Equal(t, 1, out3.ContentBlocks[0].StreamingMeta.Index)

		// Chunk 4: text continues (same block)
		chunk4 := &schema.Message{
			Role:    schema.Assistant,
			Content: " is 42",
		}
		out4, err := conv.convert(chunk4)
		assert.NoError(t, err)
		assert.Equal(t, 1, out4.ContentBlocks[0].StreamingMeta.Index)
	})

	t.Run("text then tool calls streaming", func(t *testing.T) {
		conv := newChunkConverter()

		// Chunk 1: text
		chunk1 := &schema.Message{
			Role:    schema.Assistant,
			Content: "let me search",
		}
		out1, err := conv.convert(chunk1)
		assert.NoError(t, err)
		assert.Equal(t, 0, out1.ContentBlocks[0].StreamingMeta.Index)

		// Chunk 2: tool call 0 starts
		chunk2 := &schema.Message{
			Role: schema.Assistant,
			ToolCalls: []schema.ToolCall{
				{Index: intPtr(0), ID: "call_1", Function: schema.FunctionCall{Name: "search", Arguments: `{"q"`}},
			},
		}
		out2, err := conv.convert(chunk2)
		assert.NoError(t, err)
		assert.Equal(t, 1, out2.ContentBlocks[0].StreamingMeta.Index)

		// Chunk 3: tool call 0 continues (same block)
		chunk3 := &schema.Message{
			Role: schema.Assistant,
			ToolCalls: []schema.ToolCall{
				{Index: intPtr(0), Function: schema.FunctionCall{Arguments: `: "test"}`}},
			},
		}
		out3, err := conv.convert(chunk3)
		assert.NoError(t, err)
		assert.Equal(t, 1, out3.ContentBlocks[0].StreamingMeta.Index)

		// Chunk 4: tool call 1 starts (new block)
		chunk4 := &schema.Message{
			Role: schema.Assistant,
			ToolCalls: []schema.ToolCall{
				{Index: intPtr(1), ID: "call_2", Function: schema.FunctionCall{Name: "calc", Arguments: `{"expr": "1+1"}`}},
			},
		}
		out4, err := conv.convert(chunk4)
		assert.NoError(t, err)
		assert.Equal(t, 2, out4.ContentBlocks[0].StreamingMeta.Index)
	})

	t.Run("multiContent with source index changes", func(t *testing.T) {
		conv := newChunkConverter()

		// Chunk 1: reasoning at source index 0
		chunk1 := &schema.Message{
			Role: schema.Assistant,
			AssistantGenMultiContent: []schema.MessageOutputPart{
				{
					Type:          schema.ChatMessagePartTypeReasoning,
					Reasoning:     &schema.MessageOutputReasoning{Text: "thinking"},
					StreamingMeta: &schema.MessageStreamingMeta{Index: 0},
				},
			},
		}
		out1, err := conv.convert(chunk1)
		assert.NoError(t, err)
		assert.Equal(t, 0, out1.ContentBlocks[0].StreamingMeta.Index)

		// Chunk 2: reasoning continues at source index 0 (same block)
		chunk2 := &schema.Message{
			Role: schema.Assistant,
			AssistantGenMultiContent: []schema.MessageOutputPart{
				{
					Type:          schema.ChatMessagePartTypeReasoning,
					Reasoning:     &schema.MessageOutputReasoning{Text: " more"},
					StreamingMeta: &schema.MessageStreamingMeta{Index: 0},
				},
			},
		}
		out2, err := conv.convert(chunk2)
		assert.NoError(t, err)
		assert.Equal(t, 0, out2.ContentBlocks[0].StreamingMeta.Index)

		// Chunk 3: text at source index 1 (new block)
		chunk3 := &schema.Message{
			Role: schema.Assistant,
			AssistantGenMultiContent: []schema.MessageOutputPart{
				{
					Type:          schema.ChatMessagePartTypeText,
					Text:          "answer",
					StreamingMeta: &schema.MessageStreamingMeta{Index: 1},
				},
			},
		}
		out3, err := conv.convert(chunk3)
		assert.NoError(t, err)
		assert.Equal(t, 1, out3.ContentBlocks[0].StreamingMeta.Index)
	})

	t.Run("multiContent multiple text parts in one chunk", func(t *testing.T) {
		conv := newChunkConverter()

		// Single chunk with two distinct text blocks (different source indices)
		chunk := &schema.Message{
			Role: schema.Assistant,
			AssistantGenMultiContent: []schema.MessageOutputPart{
				{
					Type:          schema.ChatMessagePartTypeText,
					Text:          "first paragraph",
					StreamingMeta: &schema.MessageStreamingMeta{Index: 0},
				},
				{
					Type:          schema.ChatMessagePartTypeText,
					Text:          "second paragraph",
					StreamingMeta: &schema.MessageStreamingMeta{Index: 1},
				},
			},
		}
		out, err := conv.convert(chunk)
		assert.NoError(t, err)
		assert.Len(t, out.ContentBlocks, 2)
		assert.Equal(t, 0, out.ContentBlocks[0].StreamingMeta.Index)
		assert.Equal(t, 1, out.ContentBlocks[1].StreamingMeta.Index)
	})

	t.Run("only tool calls no content", func(t *testing.T) {
		conv := newChunkConverter()

		chunk := &schema.Message{
			Role: schema.Assistant,
			ToolCalls: []schema.ToolCall{
				{Index: intPtr(0), ID: "call_1", Function: schema.FunctionCall{Name: "fn1", Arguments: "{}"}},
			},
		}
		out, err := conv.convert(chunk)
		assert.NoError(t, err)
		assert.Equal(t, 0, out.ContentBlocks[0].StreamingMeta.Index)

		// Second tool call
		chunk2 := &schema.Message{
			Role: schema.Assistant,
			ToolCalls: []schema.ToolCall{
				{Index: intPtr(1), ID: "call_2", Function: schema.FunctionCall{Name: "fn2", Arguments: "{}"}},
			},
		}
		out2, err := conv.convert(chunk2)
		assert.NoError(t, err)
		assert.Equal(t, 1, out2.ContentBlocks[0].StreamingMeta.Index)
	})

	t.Run("full sequence reasoning text toolcalls", func(t *testing.T) {
		conv := newChunkConverter()

		// Reasoning
		out, err := conv.convert(&schema.Message{Role: schema.Assistant, ReasoningContent: "think"})
		assert.NoError(t, err)
		assert.Equal(t, 0, out.ContentBlocks[0].StreamingMeta.Index)

		// Text
		out, err = conv.convert(&schema.Message{Role: schema.Assistant, Content: "answer"})
		assert.NoError(t, err)
		assert.Equal(t, 1, out.ContentBlocks[0].StreamingMeta.Index)

		// ToolCall 0
		out, err = conv.convert(&schema.Message{Role: schema.Assistant, ToolCalls: []schema.ToolCall{
			{Index: intPtr(0), ID: "c1", Function: schema.FunctionCall{Name: "f1", Arguments: "{}"}},
		}})
		assert.NoError(t, err)
		assert.Equal(t, 2, out.ContentBlocks[0].StreamingMeta.Index)

		// ToolCall 0 continues
		out, err = conv.convert(&schema.Message{Role: schema.Assistant, ToolCalls: []schema.ToolCall{
			{Index: intPtr(0), Function: schema.FunctionCall{Arguments: "more"}},
		}})
		assert.NoError(t, err)
		assert.Equal(t, 2, out.ContentBlocks[0].StreamingMeta.Index)

		// ToolCall 1
		out, err = conv.convert(&schema.Message{Role: schema.Assistant, ToolCalls: []schema.ToolCall{
			{Index: intPtr(1), ID: "c2", Function: schema.FunctionCall{Name: "f2", Arguments: "{}"}},
		}})
		assert.NoError(t, err)
		assert.Equal(t, 3, out.ContentBlocks[0].StreamingMeta.Index)
	})
}
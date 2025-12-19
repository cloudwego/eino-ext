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

package gemini

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/genai"

	"github.com/cloudwego/eino/schema"
)

func TestVideoMetaDataFunctions(t *testing.T) {
	ptr := func(f float64) *float64 { return &f }

	t.Run("TestSetVideoMetaData", func(t *testing.T) {
		videoURL := &schema.ChatMessageVideoURL{}

		// Success case
		metaData := &genai.VideoMetadata{FPS: ptr(24.0)}
		SetVideoMetaData(videoURL, metaData)
		assert.Equal(t, metaData, GetVideoMetaData(videoURL))

		// Boundary case: nil input
		SetVideoMetaData(nil, metaData)
		assert.Nil(t, GetVideoMetaData(nil))
	})

	t.Run("TestSetInputVideoMetaData", func(t *testing.T) {
		inputVideo := &schema.MessageInputVideo{}

		// Success case
		metaData := &genai.VideoMetadata{FPS: ptr(10.0)}
		setInputVideoMetaData(inputVideo, metaData)
		assert.Equal(t, metaData, GetInputVideoMetaData(inputVideo))

		// Boundary case: nil input
		setInputVideoMetaData(nil, metaData)
		assert.Nil(t, GetInputVideoMetaData(nil))
	})
}

func TestMessageThoughtSignatureFunctions(t *testing.T) {
	t.Run("TestSetMessageThoughtSignature", func(t *testing.T) {
		message := &schema.Message{
			Role:             schema.Assistant,
			ReasoningContent: "thinking process",
		}

		// Success case
		signature := []byte("message_thought_signature_data")
		setMessageThoughtSignature(message, signature)
		retrieved := getMessageThoughtSignature(message)
		assert.Equal(t, signature, retrieved)

		// Verify it's stored in Extra
		assert.NotNil(t, message.Extra)
		assert.Equal(t, signature, message.Extra[thoughtSignatureKey])
	})

	t.Run("TestSetMessageThoughtSignature_NilMessage", func(t *testing.T) {
		// Boundary case: nil message
		signature := []byte("test_sig")
		setMessageThoughtSignature(nil, signature)
		assert.Nil(t, getMessageThoughtSignature(nil))
	})

	t.Run("TestSetMessageThoughtSignature_EmptySignature", func(t *testing.T) {
		// Boundary case: empty signature
		message := &schema.Message{Role: schema.Assistant}
		setMessageThoughtSignature(message, []byte{})
		// Empty signature should not be set
		assert.Nil(t, getMessageThoughtSignature(message))
	})

	t.Run("TestGetMessageThoughtSignature_NilExtra", func(t *testing.T) {
		// Boundary case: message with nil Extra
		message := &schema.Message{Role: schema.Assistant}
		assert.Nil(t, getMessageThoughtSignature(message))
	})

	t.Run("MessageThoughtSignatureCanRoundTripJSON", func(t *testing.T) {
		message := &schema.Message{
			Role:             schema.Assistant,
			ReasoningContent: "thinking",
		}
		signature := []byte("msg_sig_json")

		setMessageThoughtSignature(message, signature)

		data, err := json.Marshal(message)
		assert.NoError(t, err)

		var restored schema.Message
		err = json.Unmarshal(data, &restored)
		assert.NoError(t, err)

		retrieved := getMessageThoughtSignature(&restored)
		assert.Equal(t, signature, retrieved)
	})
}

func TestToolCallThoughtSignatureFunctions(t *testing.T) {
	t.Run("TestSetToolCallThoughtSignature", func(t *testing.T) {
		toolCall := &schema.ToolCall{
			ID: "test_call",
			Function: schema.FunctionCall{
				Name:      "test_function",
				Arguments: `{"param":"value"}`,
			},
		}

		// Success case
		signature := []byte("toolcall_thought_signature_data")
		setToolCallThoughtSignature(toolCall, signature)
		retrieved := getToolCallThoughtSignature(toolCall)
		assert.Equal(t, signature, retrieved)

		// Verify it's stored in Extra
		assert.NotNil(t, toolCall.Extra)
		assert.Equal(t, signature, toolCall.Extra[thoughtSignatureKey])
	})

	t.Run("TestSetToolCallThoughtSignature_NilToolCall", func(t *testing.T) {
		// Boundary case: nil tool call
		signature := []byte("test_sig")
		setToolCallThoughtSignature(nil, signature)
		assert.Nil(t, getToolCallThoughtSignature(nil))
	})

	t.Run("TestSetToolCallThoughtSignature_EmptySignature", func(t *testing.T) {
		// Boundary case: empty signature
		toolCall := &schema.ToolCall{ID: "test"}
		setToolCallThoughtSignature(toolCall, []byte{})
		// Empty signature should not be set
		assert.Nil(t, getToolCallThoughtSignature(toolCall))
	})

	t.Run("TestGetToolCallThoughtSignature_NilExtra", func(t *testing.T) {
		// Boundary case: tool call with nil Extra
		toolCall := &schema.ToolCall{ID: "test"}
		assert.Nil(t, getToolCallThoughtSignature(toolCall))
	})

	t.Run("ToolCallThoughtSignatureCanRoundTripJSON", func(t *testing.T) {
		toolCall := &schema.ToolCall{
			ID: "test_call",
			Function: schema.FunctionCall{
				Name:      "check_flight",
				Arguments: `{"flight":"AA100"}`,
			},
		}
		signature := []byte("tc_sig_json")

		setToolCallThoughtSignature(toolCall, signature)

		data, err := json.Marshal(toolCall)
		assert.NoError(t, err)

		var restored schema.ToolCall
		err = json.Unmarshal(data, &restored)
		assert.NoError(t, err)

		retrieved := getToolCallThoughtSignature(&restored)
		assert.Equal(t, signature, retrieved)
	})
}

func TestSetPartSequenceByConvCandidate(t *testing.T) {
	c := &ChatModel{}

	t.Run("test generate", func(t *testing.T) {
		candidate := &genai.Candidate{
			Content: &genai.Content{
				Parts: []*genai.Part{
					{
						Text:    "mock thought",
						Thought: true,
					},
					{
						Text:    "mock text",
						Thought: false,
					},
					{
						FunctionCall: &genai.FunctionCall{
							ID: "1234",
							Args: map[string]any{
								"param": "value",
							},
							Name: "test_tool",
						},
					},
					{
						CodeExecutionResult: &genai.CodeExecutionResult{
							Output: "mock code execution result",
						},
					},
					{
						ExecutableCode: &genai.ExecutableCode{
							Code: "mock executable code",
						},
					},
					{
						InlineData: &genai.Blob{
							Data:     []byte("mock blob data"),
							MIMEType: "image/jpeg",
						},
					},
				},
				Role: "assistant",
			},
		}

		msg, err := c.convCandidate(candidate)
		assert.NoError(t, err)
		assert.NotNil(t, msg.Extra)
		v, ok := msg.Extra[partTypeKey].(geminiPartType)
		assert.True(t, ok)
		exp := []string{
			string(partTypeThoughtText),
			string(partTypeText),
			string(partTypeFunctionCall),
			string(partTypeCodeExecutionResult),
			string(partTypeExecutableCode),
			string(partTypeInlineData),
		}
		assert.Equal(t, geminiPartType(strings.Join(exp, ",")), v)
	})

	t.Run("test stream", func(t *testing.T) {
		parts := []*genai.Part{
			{
				Text:    "mock thought 1",
				Thought: true,
			},
			{
				Text:    "mock thought 2",
				Thought: true,
			},
			{
				Text:    "mock text 1",
				Thought: false,
			},
			{
				Text:    "mock text 2",
				Thought: false,
			},
			{
				FunctionCall: &genai.FunctionCall{
					ID: "1234",
					Args: map[string]any{
						"param": "value",
					},
					Name: "test_tool",
				},
			},
			{
				CodeExecutionResult: &genai.CodeExecutionResult{
					Output: "mock code execution result",
				},
			},
			{
				ExecutableCode: &genai.ExecutableCode{
					Code: "mock executable code",
				},
			},
			{
				InlineData: &genai.Blob{
					Data:     []byte("mock blob data"),
					MIMEType: "image/jpeg",
				},
			},
		}
		var chunks []*schema.Message
		for i := range parts {
			candidate := &genai.Candidate{
				Content: &genai.Content{
					Parts: []*genai.Part{parts[i]},
					Role:  "assistant",
				},
			}
			msg, err := c.convCandidate(candidate)
			assert.NoError(t, err)
			chunks = append(chunks, msg)
		}
		msg, err := schema.ConcatMessages(chunks)
		assert.NoError(t, err)
		assert.NotNil(t, msg.Extra)
		v, ok := msg.Extra[partTypeKey].(geminiPartType)
		assert.True(t, ok)
		exp := []string{
			string(partTypeThoughtText),
			string(partTypeText),
			string(partTypeFunctionCall),
			string(partTypeCodeExecutionResult),
			string(partTypeExecutableCode),
			string(partTypeInlineData),
		}
		assert.Equal(t, geminiPartType(strings.Join(exp, ",")), v)
	})
}

func TestSortByPartSequence(t *testing.T) {
	c := &ChatModel{}
	_ = c

	t.Run("test valid extra", func(t *testing.T) {
		exp := []string{
			string(partTypeThoughtText),
			string(partTypeText),
			string(partTypeCodeExecutionResult),
			string(partTypeExecutableCode),
			string(partTypeInlineData),
			string(partTypeFunctionCall),
		}

		msg := &schema.Message{
			Role: schema.Assistant,
			AssistantGenMultiContent: []schema.MessageOutputPart{
				{
					Type: schema.ChatMessagePartTypeText,
					Text: "mock text",
				},
			},
			Name:             "",
			ToolCalls:        nil,
			ToolCallID:       "",
			ToolName:         "",
			ResponseMeta:     nil,
			ReasoningContent: "mock thought",
			Extra: map[string]any{
				partTypeKey: geminiPartType(strings.Join(exp, ",")),
			},
		}

		_ = msg
	})

	t.Run("test extra key not found", func(t *testing.T) {

	})

	t.Run("test length not aligned", func(t *testing.T) {

	})
}

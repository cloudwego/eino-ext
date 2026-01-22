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
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/genai"

	"github.com/cloudwego/eino/schema"
)

func TestVideoMetaDataFunctions(t *testing.T) {
	ptr := func(f float64) *float64 { return &f }

	t.Run("TestSetInputVideoMetaData", func(t *testing.T) {
		inputVideo := &schema.MessageInputVideo{}

		// Success case
		metaData := &genai.VideoMetadata{FPS: ptr(10.0)}
		SetInputVideoMetaData(inputVideo, metaData)
		assert.Equal(t, metaData, GetInputVideoMetaData(inputVideo))

		// Boundary case: nil input
		SetInputVideoMetaData(nil, metaData)
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
		retrieved, ok := GetThoughtSignatureFromExtra(message.Extra)
		assert.True(t, ok)
		assert.Equal(t, signature, retrieved)

		// Verify it's stored in Extra
		assert.NotNil(t, message.Extra)
		assert.Equal(t, signature, message.Extra[thoughtSignatureKey])
	})

	t.Run("TestSetMessageThoughtSignature_NilMessage", func(t *testing.T) {
		// Boundary case: nil message
		signature := []byte("test_sig")
		setMessageThoughtSignature(nil, signature)
		retrieved, ok := GetThoughtSignatureFromExtra(nil)
		assert.False(t, ok)
		assert.Nil(t, retrieved)
	})

	t.Run("TestSetMessageThoughtSignature_EmptySignature", func(t *testing.T) {
		// Boundary case: empty signature
		message := &schema.Message{Role: schema.Assistant}
		setMessageThoughtSignature(message, []byte{})
		// Empty signature should not be set
		retrieved, ok := GetThoughtSignatureFromExtra(message.Extra)
		assert.False(t, ok)
		assert.Nil(t, retrieved)
	})

	t.Run("TestGetMessageThoughtSignature_NilExtra", func(t *testing.T) {
		// Boundary case: message with nil Extra
		message := &schema.Message{Role: schema.Assistant}
		retrieved, ok := GetThoughtSignatureFromExtra(message.Extra)
		assert.False(t, ok)
		assert.Nil(t, retrieved)
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

		retrieved, ok := GetThoughtSignatureFromExtra(restored.Extra)
		assert.True(t, ok)
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
		retrieved, ok := GetThoughtSignatureFromExtra(toolCall.Extra)
		assert.True(t, ok)
		assert.Equal(t, signature, retrieved)

		// Verify it's stored in Extra
		assert.NotNil(t, toolCall.Extra)
		assert.Equal(t, signature, toolCall.Extra[thoughtSignatureKey])
	})

	t.Run("TestSetToolCallThoughtSignature_NilToolCall", func(t *testing.T) {
		// Boundary case: nil tool call
		signature := []byte("test_sig")
		setToolCallThoughtSignature(nil, signature)
		retrieved, ok := GetThoughtSignatureFromExtra(nil)
		assert.False(t, ok)
		assert.Nil(t, retrieved)
	})

	t.Run("TestSetToolCallThoughtSignature_EmptySignature", func(t *testing.T) {
		// Boundary case: empty signature
		toolCall := &schema.ToolCall{ID: "test"}
		setToolCallThoughtSignature(toolCall, []byte{})
		// Empty signature should not be set
		retrieved, ok := GetThoughtSignatureFromExtra(toolCall.Extra)
		assert.False(t, ok)
		assert.Nil(t, retrieved)
	})

	t.Run("TestGetToolCallThoughtSignature_NilExtra", func(t *testing.T) {
		// Boundary case: tool call with nil Extra
		toolCall := &schema.ToolCall{ID: "test"}
		retrieved, ok := GetThoughtSignatureFromExtra(toolCall.Extra)
		assert.False(t, ok)
		assert.Nil(t, retrieved)
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

		retrieved, ok := GetThoughtSignatureFromExtra(restored.Extra)
		assert.True(t, ok)
		assert.Equal(t, signature, retrieved)
	})
}

func TestCustomConcat(t *testing.T) {
	extras := []map[string]any{
		{"ExecutableCode": &genai.ExecutableCode{Code: "1", Language: "1"}},
		{"ExecutableCode": &genai.ExecutableCode{Code: "2", Language: "2"}},
		{"ExecutableCode": &genai.ExecutableCode{Code: "3", Language: ""}},
		{"CodeExecutionResult": &genai.CodeExecutionResult{Outcome: "1", Output: "1"}},
		{"CodeExecutionResult": &genai.CodeExecutionResult{Outcome: "2", Output: "2"}},
		{"CodeExecutionResult": &genai.CodeExecutionResult{Outcome: "", Output: "3"}},
	}

	var msgs []*schema.Message
	for _, extra := range extras {
		msgs = append(msgs, &schema.Message{
			Role:  schema.Assistant,
			Extra: extra,
		})
	}

	msg, err := schema.ConcatMessages(msgs)
	assert.NoError(t, err)
	assert.Equal(t, map[string]any{
		"ExecutableCode":      &genai.ExecutableCode{Code: "123", Language: "2"},
		"CodeExecutionResult": &genai.CodeExecutionResult{Outcome: "2", Output: "123"},
	}, msg.Extra)
}

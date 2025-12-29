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

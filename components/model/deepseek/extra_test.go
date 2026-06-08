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

package deepseek

import (
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
)

func TestReasoningContent(t *testing.T) {
	msg := &schema.Message{}
	_, ok := GetReasoningContent(msg)
	assert.False(t, ok)
	SetReasoningContent(msg, "reasoning content")
	content, ok := GetReasoningContent(msg)
	assert.True(t, ok)
	assert.Equal(t, "reasoning content", content)

	// concat
	msgs := []*schema.Message{
		schema.UserMessage(""),
		schema.UserMessage(""),
		schema.UserMessage("hello"),
	}
	SetReasoningContent(msgs[0], "reasoning ")
	SetReasoningContent(msgs[1], "content")
	result, err := schema.ConcatMessages(msgs)
	assert.NoError(t, err)
	rc, ok := GetReasoningContent(result)
	assert.True(t, ok)
	assert.Equal(t, "reasoning content", rc)
}

func TestConcatTextParts(t *testing.T) {
	type part struct {
		typ  schema.ChatMessagePartType
		text string
	}
	extract := func(p part) (schema.ChatMessagePartType, string) {
		return p.typ, p.text
	}

	t.Run("empty parts", func(t *testing.T) {
		result, err := concatTextParts([]part{}, extract)
		assert.NoError(t, err)
		assert.Equal(t, "", result)
	})

	t.Run("single text part", func(t *testing.T) {
		result, err := concatTextParts([]part{
			{typ: schema.ChatMessagePartTypeText, text: "hello"},
		}, extract)
		assert.NoError(t, err)
		assert.Equal(t, "hello", result)
	})

	t.Run("multiple text parts", func(t *testing.T) {
		result, err := concatTextParts([]part{
			{typ: schema.ChatMessagePartTypeText, text: "hello"},
			{typ: schema.ChatMessagePartTypeText, text: "world"},
		}, extract)
		assert.NoError(t, err)
		assert.Equal(t, "hello\n\nworld", result)
	})

	t.Run("unsupported type returns error", func(t *testing.T) {
		_, err := concatTextParts([]part{
			{typ: schema.ChatMessagePartTypeImageURL, text: ""},
		}, extract)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "does not support")
		assert.Contains(t, err.Error(), string(schema.ChatMessagePartTypeImageURL))
	})

	t.Run("mixed types returns error", func(t *testing.T) {
		_, err := concatTextParts([]part{
			{typ: schema.ChatMessagePartTypeText, text: "hello"},
			{typ: schema.ChatMessagePartTypeImageURL, text: ""},
		}, extract)
		assert.Error(t, err)
	})
}

func TestPrefix(t *testing.T) {
	msg := &schema.Message{}
	assert.False(t, HasPrefix(msg))
	SetPrefix(msg)
	assert.True(t, HasPrefix(msg))
}

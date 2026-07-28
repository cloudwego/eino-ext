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

package claude

import (
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/bytedance/mockey"
	"github.com/stretchr/testify/assert"

	"github.com/cloudwego/eino/schema"
)

func TestToCacheUsage(t *testing.T) {
	t.Run("no cache activity yields nil", func(t *testing.T) {
		assert.Nil(t, toCacheUsage(anthropic.Usage{InputTokens: 12, OutputTokens: 3}))
		assert.Nil(t, toDeltaCacheUsage(anthropic.MessageDeltaUsage{OutputTokens: 3}))
	})

	t.Run("read and creation counts are kept apart", func(t *testing.T) {
		usage := toCacheUsage(anthropic.Usage{
			InputTokens:              7,
			CacheReadInputTokens:     1024,
			CacheCreationInputTokens: 2048,
		})
		assert.Equal(t, &CacheUsage{CacheReadInputTokens: 1024, CacheCreationInputTokens: 2048}, usage)

		delta := toDeltaCacheUsage(anthropic.MessageDeltaUsage{
			CacheReadInputTokens:     11,
			CacheCreationInputTokens: 22,
		})
		assert.Equal(t, &CacheUsage{CacheReadInputTokens: 11, CacheCreationInputTokens: 22}, delta)
	})
}

func TestGetCallbackOutputCacheUsage(t *testing.T) {
	cm := &ChatModel{model: "test model"}
	msg := &schema.Message{
		Role: schema.Assistant,
		ResponseMeta: &schema.ResponseMeta{
			Usage: toTokenUsage(anthropic.Usage{
				InputTokens:              7,
				CacheReadInputTokens:     1024,
				CacheCreationInputTokens: 2048,
				OutputTokens:             5,
			}),
		},
	}

	t.Run("cache usage is exposed under a namespaced key", func(t *testing.T) {
		out := cm.getCallbackOutput(msg, &CacheUsage{CacheReadInputTokens: 1024, CacheCreationInputTokens: 2048})

		usage, ok := GetCacheUsage(out)
		assert.True(t, ok)
		assert.Equal(t, 1024, usage.CacheReadInputTokens)
		assert.Equal(t, 2048, usage.CacheCreationInputTokens)

		// the standard token usage path still carries the read side on its own
		assert.Equal(t, 1024, out.TokenUsage.PromptTokenDetails.CachedTokens)
	})

	t.Run("no cache activity leaves extra empty", func(t *testing.T) {
		out := cm.getCallbackOutput(msg, nil)
		assert.Nil(t, out.Extra)

		usage, ok := GetCacheUsage(out)
		assert.False(t, ok)
		assert.Nil(t, usage)
	})

	t.Run("message extra is not forwarded to the callback output", func(t *testing.T) {
		// thinking text and its signature are message state that must not leak into
		// callback metadata consumed by tracing handlers
		withThinking := &schema.Message{Role: schema.Assistant}
		setThinking(withThinking, "internal reasoning")
		setThinkingSignature(withThinking, "signature")

		out := cm.getCallbackOutput(withThinking, nil)
		assert.Nil(t, out.Extra)

		out = cm.getCallbackOutput(withThinking, &CacheUsage{CacheReadInputTokens: 1})
		assert.Len(t, out.Extra, 1)
		assert.NotContains(t, out.Extra, keyOfThinking)
		assert.NotContains(t, out.Extra, keyOfThinkingSignature)
	})
}

func TestConvOutputMessageKeepsUsageOutOfMessageExtra(t *testing.T) {
	msg, err := convOutputMessage(&anthropic.Message{
		StopReason: anthropic.StopReasonEndTurn,
		Usage: anthropic.Usage{
			InputTokens:              7,
			CacheReadInputTokens:     1024,
			CacheCreationInputTokens: 2048,
			OutputTokens:             5,
		},
	})
	assert.NoError(t, err)

	// cache accounting belongs to the callback output, not to the message that
	// travels with the conversation history
	assert.Empty(t, msg.Extra)
	assert.Equal(t, 1024, msg.ResponseMeta.Usage.PromptTokenDetails.CachedTokens)
}

func TestConvStreamEventRecordsCacheUsage(t *testing.T) {
	streamCtx := &streamContext{}
	expected := &CacheUsage{CacheReadInputTokens: 1024, CacheCreationInputTokens: 2048}

	mockey.PatchConvey("message_start reports the cache token counts", t, func() {
		defer mockey.Mock(anthropic.MessageStreamEventUnion.AsAny).Return(anthropic.MessageStartEvent{
			Message: anthropic.Message{
				Usage: anthropic.Usage{
					InputTokens:              7,
					CacheReadInputTokens:     1024,
					CacheCreationInputTokens: 2048,
				},
			},
		}).Build().UnPatch()

		msg, err := convStreamEvent(anthropic.MessageStreamEventUnion{}, streamCtx)
		assert.NoError(t, err)
		assert.Empty(t, msg.Extra)
		assert.Equal(t, expected, streamCtx.cacheUsage)
	})

	mockey.PatchConvey("a later delta without cache tokens keeps them", t, func() {
		defer mockey.Mock(anthropic.MessageStreamEventUnion.AsAny).Return(anthropic.MessageDeltaEvent{
			Usage: anthropic.MessageDeltaUsage{OutputTokens: 5},
		}).Build().UnPatch()

		msg, err := convStreamEvent(anthropic.MessageStreamEventUnion{}, streamCtx)
		assert.NoError(t, err)
		assert.Empty(t, msg.Extra)
		assert.Equal(t, expected, streamCtx.cacheUsage)
	})
}

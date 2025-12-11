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

package openrouter

import (
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
)

func TestSetStreamTerminatedError(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		msg := &schema.Message{}
		errStr := `{"code": "error_code", "message": "error_message"}`
		err := setStreamTerminatedError(msg, errStr)
		assert.NoError(t, err)
		assert.NotNil(t, msg.Extra)
		e, ok := msg.Extra[openrouterTerminatedErrorKey].(*StreamTerminatedError)
		assert.True(t, ok)
		assert.Equal(t, "error_code", e.Code)
		assert.Equal(t, "error_message", e.Message)
	})

	t.Run("invalid json", func(t *testing.T) {
		msg := &schema.Message{}
		errStr := `invalid_json`
		err := setStreamTerminatedError(msg, errStr)
		assert.Error(t, err)
	})
}

func TestGetStreamTerminatedError(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		msg := &schema.Message{
			Extra: map[string]any{
				openrouterTerminatedErrorKey: &StreamTerminatedError{
					Code:    "error_code",
					Message: "error_message",
				},
			},
		}
		e, ok := GetStreamTerminatedError(msg)
		assert.True(t, ok)
		assert.NotNil(t, e)
		assert.Equal(t, "error_code", e.Code)
		assert.Equal(t, "error_message", e.Message)
	})

	t.Run("no extra", func(t *testing.T) {
		msg := &schema.Message{}
		e, ok := GetStreamTerminatedError(msg)
		assert.False(t, ok)
		assert.Nil(t, e)
	})

	t.Run("wrong type", func(t *testing.T) {
		msg := &schema.Message{
			Extra: map[string]any{
				openrouterTerminatedErrorKey: "not a StreamTerminatedError",
			},
		}
		e, ok := GetStreamTerminatedError(msg)
		assert.False(t, ok)
		assert.Nil(t, e)
	})
}

func TestSetReasoningDetails(t *testing.T) {
	msg := &schema.Message{}
	details := []*reasoningDetails{
		{
			Format: "text",
			Data:   "reasoning data",
		},
	}
	setReasoningDetails(msg, details)
	assert.NotNil(t, msg.Extra)
	d, ok := msg.Extra[openrouterReasoningDetailsKey].([]*reasoningDetails)
	assert.True(t, ok)
	assert.Equal(t, details, d)
}

func TestGetReasoningDetails(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		details := []*reasoningDetails{
			{
				Format: "text",
				Data:   "reasoning data",
			},
		}
		msg := &schema.Message{
			Extra: map[string]any{
				openrouterReasoningDetailsKey: details,
			},
		}
		d, ok := getReasoningDetails(msg)
		assert.True(t, ok)
		assert.Equal(t, details, d)
	})

	t.Run("no extra", func(t *testing.T) {
		msg := &schema.Message{}
		d, ok := getReasoningDetails(msg)
		assert.False(t, ok)
		assert.Nil(t, d)
	})

	t.Run("wrong type", func(t *testing.T) {
		msg := &schema.Message{
			Extra: map[string]any{
				openrouterReasoningDetailsKey: "not a []*ReasoningDetails",
			},
		}
		d, ok := getReasoningDetails(msg)
		assert.False(t, ok)
		assert.Nil(t, d)
	})
}

func TestEnableMessageInputPartCacheControl(t *testing.T) {
	t.Run("default ttl", func(t *testing.T) {
		part := &schema.MessageInputPart{}
		EnableMessageInputPartCacheControl(part)
		ctrl, ok := getMessageInputPartCacheControl(part)
		assert.True(t, ok)
		assert.NotNil(t, ctrl)
		assert.Equal(t, cacheControlEphemeralType, ctrl.Type)
		assert.Equal(t, CacheControlTTL5Minutes, ctrl.TTL)
	})

	t.Run("override ttl to 1h", func(t *testing.T) {
		part := &schema.MessageInputPart{}
		EnableMessageInputPartCacheControl(part, WithCacheControlTTL(CacheControlTTL1Hour))
		ctrl, ok := getMessageInputPartCacheControl(part)
		assert.True(t, ok)
		assert.NotNil(t, ctrl)
		assert.Equal(t, cacheControlEphemeralType, ctrl.Type)
		assert.Equal(t, CacheControlTTL1Hour, ctrl.TTL)
	})

	t.Run("no extra present", func(t *testing.T) {
		part := &schema.MessageInputPart{}
		ctrl, ok := getMessageInputPartCacheControl(part)
		assert.False(t, ok)
		assert.Nil(t, ctrl)
	})
}

func TestEnableMessageOutputPartCacheControl(t *testing.T) {
	t.Run("default ttl", func(t *testing.T) {
		part := &schema.MessageOutputPart{}
		EnableMessageOutputPartCacheControl(part)
		ctrl, ok := getMessageOutputPartCacheControl(part)
		assert.True(t, ok)
		assert.NotNil(t, ctrl)
		assert.Equal(t, cacheControlEphemeralType, ctrl.Type)
		assert.Equal(t, CacheControlTTL5Minutes, ctrl.TTL)
	})

	t.Run("override ttl to 1h", func(t *testing.T) {
		part := &schema.MessageOutputPart{}
		EnableMessageOutputPartCacheControl(part, WithCacheControlTTL(CacheControlTTL1Hour))
		ctrl, ok := getMessageOutputPartCacheControl(part)
		assert.True(t, ok)
		assert.NotNil(t, ctrl)
		assert.Equal(t, cacheControlEphemeralType, ctrl.Type)
		assert.Equal(t, CacheControlTTL1Hour, ctrl.TTL)
	})

	t.Run("no extra present", func(t *testing.T) {
		part := &schema.MessageOutputPart{}
		ctrl, ok := getMessageOutputPartCacheControl(part)
		assert.False(t, ok)
		assert.Nil(t, ctrl)
	})
}

func TestEnableMessageContentCacheControl(t *testing.T) {
	t.Run("default ttl", func(t *testing.T) {
		msg := &schema.Message{}
		EnableMessageContentCacheControl(msg)
		ctrl, ok := getMessageContentCacheControl(msg)
		assert.True(t, ok)
		assert.NotNil(t, ctrl)
		assert.Equal(t, cacheControlEphemeralType, ctrl.Type)
		assert.Equal(t, CacheControlTTL5Minutes, ctrl.TTL)
	})

	t.Run("override ttl to 1h", func(t *testing.T) {
		msg := &schema.Message{}
		EnableMessageContentCacheControl(msg, WithCacheControlTTL(CacheControlTTL1Hour))
		ctrl, ok := getMessageContentCacheControl(msg)
		assert.True(t, ok)
		assert.NotNil(t, ctrl)
		assert.Equal(t, cacheControlEphemeralType, ctrl.Type)
		assert.Equal(t, CacheControlTTL1Hour, ctrl.TTL)
	})

	t.Run("nil extra should be initialized", func(t *testing.T) {
		msg := &schema.Message{}
		ctrl, ok := getMessageContentCacheControl(msg)
		assert.False(t, ok)
		assert.Nil(t, ctrl)
		assert.NotNil(t, msg.Extra)
	})
}

func TestCacheControlConcatPreference(t *testing.T) {
	t.Run("prefer 1h when present", func(t *testing.T) {
		m1 := &schema.Message{}
		m2 := &schema.Message{}
		EnableMessageContentCacheControl(m1, WithCacheControlTTL(CacheControlTTL5Minutes))
		EnableMessageContentCacheControl(m2, WithCacheControlTTL(CacheControlTTL1Hour))

		final, err := schema.ConcatMessages([]*schema.Message{m1, m2})
		assert.NoError(t, err)
		ctrl, ok := getMessageContentCacheControl(final)
		assert.True(t, ok)
		assert.NotNil(t, ctrl)
		assert.Equal(t, CacheControlTTL1Hour, ctrl.TTL)
	})

	t.Run("fallback to last when no 1h", func(t *testing.T) {
		m1 := &schema.Message{}
		m2 := &schema.Message{}
		EnableMessageContentCacheControl(m1, WithCacheControlTTL(CacheControlTTL5Minutes))
		EnableMessageContentCacheControl(m2, WithCacheControlTTL(CacheControlTTL5Minutes))

		final, err := schema.ConcatMessages([]*schema.Message{m1, m2})
		assert.NoError(t, err)
		ctrl, ok := getMessageContentCacheControl(final)
		assert.True(t, ok)
		assert.NotNil(t, ctrl)
		assert.Equal(t, CacheControlTTL5Minutes, ctrl.TTL)
	})
}

func TestStreamTerminatedErrorConcat(t *testing.T) {
	t.Run("take last error", func(t *testing.T) {
		m1 := &schema.Message{}
		m2 := &schema.Message{}
		_ = setStreamTerminatedError(m1, `{"code":"A","message":"a"}`)
		_ = setStreamTerminatedError(m2, `{"code":"B","message":"b"}`)

		final, err := schema.ConcatMessages([]*schema.Message{m1, m2})
		assert.NoError(t, err)
		e, ok := GetStreamTerminatedError(final)
		assert.True(t, ok)
		assert.NotNil(t, e)
		assert.Equal(t, "B", e.Code)
		assert.Equal(t, "b", e.Message)
	})
}

func TestReasoningDetailsConcat(t *testing.T) {
	d1 := []*reasoningDetails{{Format: "text", Data: "r1"}, {Format: "text", Data: "r2"}}
	d2 := []*reasoningDetails{{Format: "text", Data: "r3"}}
	m1 := &schema.Message{}
	m2 := &schema.Message{}
	setReasoningDetails(m1, d1)
	setReasoningDetails(m2, d2)

	final, err := schema.ConcatMessages([]*schema.Message{m1, m2})
	assert.NoError(t, err)
	got, ok := getReasoningDetails(final)
	assert.True(t, ok)
	assert.Len(t, got, 3)
	assert.Equal(t, "r1", got[0].Data)
	assert.Equal(t, "r2", got[1].Data)
	assert.Equal(t, "r3", got[2].Data)
}

func TestSetStreamTerminatedErrorNilMessage(t *testing.T) {
	err := setStreamTerminatedError(nil, `{"code":"X","message":"x"}`)
	assert.NoError(t, err)
}

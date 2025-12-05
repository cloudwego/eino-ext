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

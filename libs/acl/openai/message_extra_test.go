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

package openai

import (
	"encoding/json"
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
)

func TestConcatMessages(t *testing.T) {
	msgs := []*schema.Message{
		{
			Extra: map[string]any{
				"key_of_string":       "hi!",
				"key_of_int":          int(10),
				keyOfReasoningContent: "how ",
			},
		},
		{
			Extra: map[string]any{
				"key_of_string":       "hello!",
				"key_of_int":          int(50),
				keyOfReasoningContent: "are you",
			},
		},
	}

	msg, err := schema.ConcatMessages(msgs)
	assert.NoError(t, err)
	assert.Equal(t, "hi!hello!", msg.Extra["key_of_string"])
	assert.Equal(t, int(50), msg.Extra["key_of_int"])

	reasoningContent, ok := GetReasoningContent(msg)
	assert.Equal(t, true, ok)
	assert.Equal(t, "how are you", reasoningContent)
}

func TestSetGetRequestID(t *testing.T) {
	msg := &schema.Message{}
	setRequestID(msg, "req-123")
	requestID := GetRequestID(msg)
	assert.Equal(t, "req-123", requestID)
}

func TestSetMsgExtra(t *testing.T) {
	msg := &schema.Message{}
	setMsgExtra(msg, "key", "val")
	extraVal, ok := getMsgExtraValue[string](msg, "key")
	assert.Equal(t, true, ok)
	assert.Equal(t, "val", extraVal)
}

func TestPopulateRCFromExtra(t *testing.T) {
	tests := []struct {
		name            string
		extra           map[string]json.RawMessage
		wantRC          string
		wantHasRC       bool
	}{
		{
			name:      "normal string value",
			extra:     map[string]json.RawMessage{"reasoning": json.RawMessage(`"thinking step by step"`)},
			wantRC:    "thinking step by step",
			wantHasRC: true,
		},
		{
			name:      "null value",
			extra:     map[string]json.RawMessage{"reasoning": json.RawMessage(`null`)},
			wantRC:    "",
			wantHasRC: false,
		},
		{
			name:      "empty string value",
			extra:     map[string]json.RawMessage{"reasoning": json.RawMessage(`""`)},
			wantRC:    "",
			wantHasRC: false,
		},
		{
			name:      "non-string json type",
			extra:     map[string]json.RawMessage{"reasoning": json.RawMessage(`{"key":"val"}`)},
			wantRC:    "",
			wantHasRC: false,
		},
		{
			name:      "key not present",
			extra:     map[string]json.RawMessage{"other": json.RawMessage(`"value"`)},
			wantRC:    "",
			wantHasRC: false,
		},
		{
			name:      "nil extra",
			extra:     nil,
			wantRC:    "",
			wantHasRC: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &schema.Message{}
			populateRCFromExtra(tt.extra, msg)
			assert.Equal(t, tt.wantRC, msg.ReasoningContent)
			rc, ok := GetReasoningContent(msg)
			assert.Equal(t, tt.wantHasRC, ok)
			if tt.wantHasRC {
				assert.Equal(t, tt.wantRC, rc)
			}
		})
	}
}

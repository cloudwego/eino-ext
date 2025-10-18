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

package langfuse

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestChatMessageWithPlaceHolder_validate(t *testing.T) {
	tests := []struct {
		name    string
		message ChatMessageWithPlaceHolder
		wantErr bool
	}{
		{
			name: "valid message",
			message: ChatMessageWithPlaceHolder{
				Role:    "user",
				Type:    "text",
				Content: "Hello world",
			},
			wantErr: false,
		},
		{
			name: "missing role",
			message: ChatMessageWithPlaceHolder{
				Type:    "text",
				Content: "Hello world",
			},
			wantErr: true,
		},
		{
			name: "missing content",
			message: ChatMessageWithPlaceHolder{
				Role: "user",
				Type: "text",
			},
			wantErr: true,
		},
		{
			name: "empty role",
			message: ChatMessageWithPlaceHolder{
				Role:    "",
				Type:    "text",
				Content: "Hello world",
			},
			wantErr: true,
		},
		{
			name: "empty content",
			message: ChatMessageWithPlaceHolder{
				Role:    "user",
				Type:    "text",
				Content: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.message.validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPromptEntry_validate(t *testing.T) {
	tests := []struct {
		name    string
		prompt  PromptEntry
		wantErr bool
	}{
		{
			name: "valid text prompt",
			prompt: PromptEntry{
				Name:   "test-prompt",
				Type:   "text",
				Prompt: "This is a test prompt",
			},
			wantErr: false,
		},
		{
			name: "valid chat prompt",
			prompt: PromptEntry{
				Name: "test-chat-prompt",
				Type: "chat",
				Prompt: []ChatMessageWithPlaceHolder{
					{
						Role:    "system",
						Type:    "text",
						Content: "You are a helpful assistant",
					},
					{
						Role:    "user",
						Type:    "text",
						Content: "Hello {{name}}",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing name",
			prompt: PromptEntry{
				Type:   "text",
				Prompt: "This is a test prompt",
			},
			wantErr: true,
		},
		{
			name: "empty name",
			prompt: PromptEntry{
				Name:   "",
				Type:   "text",
				Prompt: "This is a test prompt",
			},
			wantErr: true,
		},
		{
			name: "nil prompt",
			prompt: PromptEntry{
				Name:   "test-prompt",
				Type:   "text",
				Prompt: nil,
			},
			wantErr: true,
		},
		{
			name: "empty string for text type",
			prompt: PromptEntry{
				Name:   "test-prompt",
				Type:   "text",
				Prompt: "",
			},
			wantErr: true,
		},
		{
			name: "wrong type for text prompt",
			prompt: PromptEntry{
				Name:   "test-prompt",
				Type:   "text",
				Prompt: []ChatMessageWithPlaceHolder{},
			},
			wantErr: true,
		},
		{
			name: "empty messages for chat prompt",
			prompt: PromptEntry{
				Name:   "test-prompt",
				Type:   "chat",
				Prompt: []ChatMessageWithPlaceHolder{},
			},
			wantErr: true,
		},
		{
			name: "wrong type for chat prompt",
			prompt: PromptEntry{
				Name:   "test-prompt",
				Type:   "chat",
				Prompt: "This should be messages",
			},
			wantErr: true,
		},
		{
			name: "invalid message in chat prompt",
			prompt: PromptEntry{
				Name: "test-prompt",
				Type: "chat",
				Prompt: []ChatMessageWithPlaceHolder{
					{
						Role:    "",
						Content: "Invalid message",
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.prompt.validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestListParams_ToQueryString(t *testing.T) {
	tests := []struct {
		name   string
		params ListParams
		want   string
	}{
		{
			name:   "empty params",
			params: ListParams{},
			want:   "",
		},
		{
			name: "all params",
			params: ListParams{
				Name:          "test-prompt",
				Label:         "production",
				Tag:           "v1.0",
				Page:          1,
				Limit:         10,
				FromUpdatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				ToUpdatedAt:   time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC),
			},
			want: "name=test-prompt&label=production&tag=v1.0&page=1&limit=10&fromUpdatedAt=2024-01-01T00:00:00Z&toUpdatedAt=2024-12-31T23:59:59Z",
		},
		{
			name: "name only",
			params: ListParams{
				Name: "test-prompt",
			},
			want: "name=test-prompt",
		},
		{
			name: "pagination only",
			params: ListParams{
				Page:  2,
				Limit: 20,
			},
			want: "page=2&limit=20",
		},
		{
			name: "time range only",
			params: ListParams{
				FromUpdatedAt: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC),
				ToUpdatedAt:   time.Date(2024, 6, 30, 12, 0, 0, 0, time.UTC),
			},
			want: "fromUpdatedAt=2024-06-01T12:00:00Z&toUpdatedAt=2024-06-30T12:00:00Z",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.params.ToQueryString()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPromptEntry_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    PromptEntry
		wantErr bool
	}{
		{
			name:  "text type prompt",
			input: `{"name":"test","type":"text","prompt":"Hello world"}`,
			want: PromptEntry{
				Name:   "test",
				Type:   "text",
				Prompt: "Hello world",
			},
			wantErr: false,
		},
		{
			name:  "chat type prompt",
			input: `{"name":"test","type":"chat","prompt":[{"role":"user","type":"text","content":"Hello"}]}`,
			want: PromptEntry{
				Name: "test",
				Type: "chat",
				Prompt: []ChatMessageWithPlaceHolder{
					{
						Role:    "user",
						Type:    "text",
						Content: "Hello",
					},
				},
			},
			wantErr: false,
		},
		{
			name:  "prompt with version and tags",
			input: `{"name":"test","type":"text","prompt":"Hello","version":1,"tags":["tag1","tag2"],"labels":["label1"]}`,
			want: PromptEntry{
				Name:    "test",
				Type:    "text",
				Prompt:  "Hello",
				Version: 1,
				Tags:    []string{"tag1", "tag2"},
				Labels:  []string{"label1"},
			},
			wantErr: false,
		},
		{
			name:    "invalid json",
			input:   `{"name":"test","type":"text"`,
			want:    PromptEntry{},
			wantErr: true,
		},
		{
			name:    "invalid prompt for text type",
			input:   `{"name":"test","type":"text","prompt":123}`,
			want:    PromptEntry{},
			wantErr: true,
		},
		{
			name:    "invalid prompt for chat type",
			input:   `{"name":"test","type":"chat","prompt":"should be array"}`,
			want:    PromptEntry{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got PromptEntry
			err := json.Unmarshal([]byte(tt.input), &got)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

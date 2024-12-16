// Copyright 2024 CloudWeGo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package langfuse

import (
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
)

func Test_convMessageIfNeeded(t *testing.T) {
	type args struct {
		message *schema.Message
	}
	tests := []struct {
		name string
		args args
		want any
	}{
		{
			name: "base message",
			args: args{
				message: &schema.Message{
					Role:    schema.User,
					Content: "content",
					Name:    "name",
					ToolCalls: []schema.ToolCall{
						{
							ID:       "id",
							Function: schema.FunctionCall{Name: "name", Arguments: "arguments"},
						},
					},
					ToolCallID: "id",
					ResponseMeta: &schema.ResponseMeta{
						FinishReason: "stop",
						Usage: &schema.TokenUsage{
							PromptTokens:     1,
							CompletionTokens: 2,
							TotalTokens:      3,
						},
					},
					Extra: map[string]interface{}{"key": "value"},
				},
			},
			want: &schema.Message{
				Role:    schema.User,
				Content: "content",
				Name:    "name",
				ToolCalls: []schema.ToolCall{
					{
						ID:       "id",
						Function: schema.FunctionCall{Name: "name", Arguments: "arguments"},
					},
				},
				ToolCallID: "id",
				ResponseMeta: &schema.ResponseMeta{
					FinishReason: "stop",
					Usage: &schema.TokenUsage{
						PromptTokens:     1,
						CompletionTokens: 2,
						TotalTokens:      3,
					},
				},
				Extra: map[string]interface{}{"key": "value"},
			},
		},
		{
			name: "multi part message",
			args: args{
				message: &schema.Message{
					Role:    schema.User,
					Content: "content",
					MultiContent: []schema.ChatMessagePart{
						{
							Type: schema.ChatMessagePartTypeImageURL,
							ImageURL: &schema.ChatMessageImageURL{
								URL: "url",
							},
						},
					},
					Name: "name",
					ToolCalls: []schema.ToolCall{
						{
							ID:       "id",
							Function: schema.FunctionCall{Name: "name", Arguments: "arguments"},
						},
					},
					ToolCallID: "id",
					ResponseMeta: &schema.ResponseMeta{
						FinishReason: "stop",
						Usage: &schema.TokenUsage{
							PromptTokens:     1,
							CompletionTokens: 2,
							TotalTokens:      3,
						},
					},
					Extra: map[string]interface{}{"key": "value"},
				},
			},
			want: &mediaMessage{
				Role: schema.User,
				Content: []schema.ChatMessagePart{
					{
						Type: schema.ChatMessagePartTypeImageURL,
						ImageURL: &schema.ChatMessageImageURL{
							URL: "url",
						},
					},
				},
				Name: "name",
				ToolCalls: []schema.ToolCall{
					{
						ID:       "id",
						Function: schema.FunctionCall{Name: "name", Arguments: "arguments"},
					},
				},
				ToolCallID: "id",
				ResponseMeta: &schema.ResponseMeta{
					FinishReason: "stop",
					Usage: &schema.TokenUsage{
						PromptTokens:     1,
						CompletionTokens: 2,
						TotalTokens:      3,
					},
				},
				Extra: map[string]interface{}{"key": "value"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convMessageIfNeeded(tt.args.message)
			assert.Equal(t, tt.want, got)
		})
	}
}

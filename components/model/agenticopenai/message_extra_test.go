/*
 * Copyright 2026 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package agenticopenai

import (
	"encoding/json"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/openai/openai-go/v3/responses"
)

func TestGetCacheWriteTokens(t *testing.T) {
	tests := []struct {
		name       string
		msg        *schema.AgenticMessage
		wantTokens int
		wantOK     bool
	}{
		{name: "nil message"},
		{name: "nil extra", msg: &schema.AgenticMessage{}},
		{name: "missing key", msg: &schema.AgenticMessage{Extra: map[string]any{"other": 42}}},
		{
			name: "cache write tokens present",
			msg: &schema.AgenticMessage{Extra: map[string]any{
				keyOfCacheWriteTokens: 1234,
			}},
			wantTokens: 1234,
			wantOK:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTokens, gotOK := GetCacheWriteTokens(tt.msg)
			if gotTokens != tt.wantTokens || gotOK != tt.wantOK {
				t.Fatalf("GetCacheWriteTokens() = (%d, %v), want (%d, %v)",
					gotTokens, gotOK, tt.wantTokens, tt.wantOK)
			}
		})
	}
}

func TestCacheWriteTokensSetOnGeneratedMessage(t *testing.T) {
	t.Run("zero cache write tokens are omitted", func(t *testing.T) {
		resp := mustUnmarshalResponse(t, `{
			"id": "resp_1",
			"status": "completed",
			"output": [],
			"usage": {
				"input_tokens": 100,
				"input_tokens_details": {"cached_tokens": 0, "cache_write_tokens": 0},
				"output_tokens": 20,
				"output_tokens_details": {"reasoning_tokens": 0},
				"total_tokens": 120
			}
		}`)

		msg, err := toOutputMessage(resp, &model.Options{})
		if err != nil {
			t.Fatal(err)
		}
		if tokens, ok := GetCacheWriteTokens(msg); ok || tokens != 0 {
			t.Fatalf("expected (0, false), got (%d, %v)", tokens, ok)
		}
	})

	t.Run("nonzero cache write tokens are exposed", func(t *testing.T) {
		resp := mustUnmarshalResponse(t, `{
			"id": "resp_1",
			"status": "completed",
			"output": [],
			"usage": {
				"input_tokens": 600,
				"input_tokens_details": {"cached_tokens": 100, "cache_write_tokens": 500},
				"output_tokens": 20,
				"output_tokens_details": {"reasoning_tokens": 0},
				"total_tokens": 620
			}
		}`)

		msg, err := toOutputMessage(resp, &model.Options{})
		if err != nil {
			t.Fatal(err)
		}
		if tokens, ok := GetCacheWriteTokens(msg); !ok || tokens != 500 {
			t.Fatalf("expected (500, true), got (%d, %v)", tokens, ok)
		}
	})
}

func TestCacheWriteTokensSetOnStreamingResponse(t *testing.T) {
	resp := mustUnmarshalResponse(t, `{
		"id": "resp_1",
		"status": "completed",
		"output": [],
		"usage": {
			"input_tokens": 400,
			"input_tokens_details": {"cached_tokens": 100, "cache_write_tokens": 300},
			"output_tokens": 20,
			"output_tokens_details": {"reasoning_tokens": 0},
			"total_tokens": 420
		}
	}`)

	sr, sw := schema.Pipe[*model.AgenticCallbackOutput](1)
	reader := sr.Copy(1)[0]
	sender := newCallbackSender(sw, &model.AgenticConfig{})
	sender.sendResponse(resp, nil)

	out, err := reader.Recv()
	if err != nil {
		t.Fatal(err)
	}
	if tokens, ok := GetCacheWriteTokens(out.Message); !ok || tokens != 300 {
		t.Fatalf("expected (300, true), got (%d, %v)", tokens, ok)
	}
}

func TestCacheWriteTokensPreservedAfterConcat(t *testing.T) {
	usageChunk := &schema.AgenticMessage{
		Role:  schema.AgenticRoleTypeAssistant,
		Extra: map[string]any{keyOfCacheWriteTokens: 300},
	}
	textChunk := &schema.AgenticMessage{
		Role: schema.AgenticRoleTypeAssistant,
		ContentBlocks: []*schema.ContentBlock{
			schema.NewContentBlockChunk(&schema.AssistantGenText{Text: "hello"}, &schema.StreamingMeta{Index: 0}),
		},
	}

	msg, err := schema.ConcatAgenticMessages([]*schema.AgenticMessage{usageChunk, textChunk})
	if err != nil {
		t.Fatal(err)
	}
	if tokens, ok := GetCacheWriteTokens(msg); !ok || tokens != 300 {
		t.Fatalf("expected (300, true), got (%d, %v)", tokens, ok)
	}
}

func mustUnmarshalResponse(t *testing.T, raw string) *responses.Response {
	t.Helper()
	var resp responses.Response
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("json.Unmarshal(response) error = %v", err)
	}
	return &resp
}

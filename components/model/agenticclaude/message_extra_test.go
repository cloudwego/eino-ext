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

package agenticclaude

import (
	"encoding/json"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/cloudwego/eino/schema"
)

func TestGetCacheCreationInputTokens(t *testing.T) {
	t.Run("nil message", func(t *testing.T) {
		tokens, ok := GetCacheCreationInputTokens(nil)
		if ok || tokens != 0 {
			t.Fatalf("expected (0, false), got (%d, %v)", tokens, ok)
		}
	})

	t.Run("nil Extra", func(t *testing.T) {
		msg := &schema.AgenticMessage{}
		tokens, ok := GetCacheCreationInputTokens(msg)
		if ok || tokens != 0 {
			t.Fatalf("expected (0, false), got (%d, %v)", tokens, ok)
		}
	})

	t.Run("key not present", func(t *testing.T) {
		msg := &schema.AgenticMessage{Extra: map[string]any{"other": 42}}
		tokens, ok := GetCacheCreationInputTokens(msg)
		if ok || tokens != 0 {
			t.Fatalf("expected (0, false), got (%d, %v)", tokens, ok)
		}
	})

	t.Run("key present with value", func(t *testing.T) {
		msg := &schema.AgenticMessage{Extra: map[string]any{
			KeyOfCacheCreationInputTokens: 1234,
		}}
		tokens, ok := GetCacheCreationInputTokens(msg)
		if !ok || tokens != 1234 {
			t.Fatalf("expected (1234, true), got (%d, %v)", tokens, ok)
		}
	})
}

func TestCacheCreationInputTokens_SetInConvOutputMessage(t *testing.T) {
	t.Run("zero cache creation does not set Extra", func(t *testing.T) {
		var resp anthropic.Message
		if err := json.Unmarshal([]byte(`{
			"id": "msg_1",
			"type": "message",
			"role": "assistant",
			"content": [],
			"usage": {"input_tokens": 100, "output_tokens": 50, "cache_creation_input_tokens": 0}
		}`), &resp); err != nil {
			t.Fatal(err)
		}
		msg, err := toAgenticMessage(&resp)
		if err != nil {
			t.Fatal(err)
		}
		if msg.Extra != nil {
			t.Fatalf("expected nil Extra when cache_creation_input_tokens=0, got %v", msg.Extra)
		}
	})

	t.Run("nonzero cache creation sets Extra", func(t *testing.T) {
		var resp anthropic.Message
		if err := json.Unmarshal([]byte(`{
			"id": "msg_1",
			"type": "message",
			"role": "assistant",
			"content": [],
			"usage": {"input_tokens": 100, "output_tokens": 50, "cache_creation_input_tokens": 500}
		}`), &resp); err != nil {
			t.Fatal(err)
		}
		msg, err := toAgenticMessage(&resp)
		if err != nil {
			t.Fatal(err)
		}
		tokens, ok := GetCacheCreationInputTokens(msg)
		if !ok || tokens != 500 {
			t.Fatalf("expected (500, true), got (%d, %v)", tokens, ok)
		}
	})
}

func TestCacheCreationInputTokens_SetInStreamDelta(t *testing.T) {
	sc := newStreamConverter()

	t.Run("delta with zero cache creation", func(t *testing.T) {
		event := mustUnmarshalStreamEvent(t, `{
			"type": "message_delta",
			"delta": {"stop_reason": "end_turn"},
			"usage": {"output_tokens": 42, "cache_creation_input_tokens": 0}
		}`)
		msg, err := sc.toMessageStreamingChunk(event)
		if err != nil {
			t.Fatal(err)
		}
		if msg.Extra != nil {
			t.Fatalf("expected nil Extra for zero cache_creation, got %v", msg.Extra)
		}
	})

	t.Run("delta with nonzero cache creation", func(t *testing.T) {
		event := mustUnmarshalStreamEvent(t, `{
			"type": "message_delta",
			"delta": {"stop_reason": "end_turn"},
			"usage": {"output_tokens": 42, "cache_creation_input_tokens": 300}
		}`)
		msg, err := sc.toMessageStreamingChunk(event)
		if err != nil {
			t.Fatal(err)
		}
		tokens, ok := GetCacheCreationInputTokens(msg)
		if !ok || tokens != 300 {
			t.Fatalf("expected (300, true), got (%d, %v)", tokens, ok)
		}
	})
}

func mustUnmarshalStreamEvent(t *testing.T, raw string) anthropic.MessageStreamEventUnion {
	t.Helper()
	var event anthropic.MessageStreamEventUnion
	if err := json.Unmarshal([]byte(raw), &event); err != nil {
		t.Fatalf("json.Unmarshal(event) error = %v", err)
	}
	return event
}

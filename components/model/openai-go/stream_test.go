/*
 * Copyright 2026 CloudWeGo Authors
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

package openaigo

import (
	"strings"
	"testing"

	"github.com/openai/openai-go/v3/responses"
)

func TestStreamStateConsume(t *testing.T) {
	t.Run("created and progress events update model", func(t *testing.T) {
		s := newStreamState()
		msg, done, deltaOnly, err := s.consume(mustJSON[responses.ResponseStreamEventUnion](t, map[string]any{
			"type":     "response.created",
			"response": map[string]any{"model": "gpt-created"},
		}))
		if err != nil || msg != nil || done || !deltaOnly {
			t.Fatalf("unexpected created event result: msg=%#v done=%v deltaOnly=%v err=%v", msg, done, deltaOnly, err)
		}
		if s.modelName != "gpt-created" {
			t.Fatalf("expected model name to be recorded, got %q", s.modelName)
		}

		msg, done, deltaOnly, err = s.consume(mustJSON[responses.ResponseStreamEventUnion](t, map[string]any{
			"type":     "response.in_progress",
			"response": map[string]any{"model": "gpt-progress"},
		}))
		if err != nil || msg != nil || done || !deltaOnly {
			t.Fatalf("unexpected in-progress result: msg=%#v done=%v deltaOnly=%v err=%v", msg, done, deltaOnly, err)
		}
		if s.modelName != "gpt-progress" {
			t.Fatalf("expected model name from in-progress event, got %q", s.modelName)
		}
	})

	t.Run("text and reasoning deltas", func(t *testing.T) {
		s := newStreamState()
		msg, done, deltaOnly, err := s.consume(mustJSON[responses.ResponseStreamEventUnion](t, map[string]any{
			"type":  "response.output_text.delta",
			"delta": "hello",
		}))
		if err != nil || done || !deltaOnly || msg == nil || msg.Content != "hello" {
			t.Fatalf("unexpected text delta result: msg=%#v done=%v deltaOnly=%v err=%v", msg, done, deltaOnly, err)
		}

		msg, done, deltaOnly, err = s.consume(mustJSON[responses.ResponseStreamEventUnion](t, map[string]any{
			"type":  "response.reasoning_text.delta",
			"delta": "thinking",
		}))
		if err != nil || done || !deltaOnly || msg == nil || msg.ReasoningContent != "thinking" {
			t.Fatalf("unexpected reasoning delta result: msg=%#v done=%v deltaOnly=%v err=%v", msg, done, deltaOnly, err)
		}

		msg, done, deltaOnly, err = s.consume(mustJSON[responses.ResponseStreamEventUnion](t, map[string]any{
			"type": "response.output_text.delta",
		}))
		if err != nil || msg != nil || done || !deltaOnly {
			t.Fatalf("expected empty text delta to be ignored, got msg=%#v done=%v deltaOnly=%v err=%v", msg, done, deltaOnly, err)
		}
	})

	t.Run("function call lifecycle", func(t *testing.T) {
		s := newStreamState()
		_, _, _, err := s.consume(mustJSON[responses.ResponseStreamEventUnion](t, map[string]any{
			"type": "response.output_item.added",
			"item": map[string]any{
				"type":      "function_call",
				"id":        "item_1",
				"call_id":   "call_123",
				"name":      "lookup_weather",
				"arguments": "",
			},
		}))
		if err != nil {
			t.Fatalf("unexpected output item added error: %v", err)
		}
		if got := s.callIDByItemID["item_1"]; got != "call_123" {
			t.Fatalf("expected call id to be tracked, got %q", got)
		}
		if got := s.nameByItemID["item_1"]; got != "lookup_weather" {
			t.Fatalf("expected function name to be tracked, got %q", got)
		}

		msg, done, deltaOnly, err := s.consume(mustJSON[responses.ResponseStreamEventUnion](t, map[string]any{
			"type":    "response.function_call_arguments.delta",
			"item_id": "item_1",
			"delta":   `{"city":"`,
		}))
		if err != nil || msg != nil || done || !deltaOnly {
			t.Fatalf("unexpected function-call delta result: msg=%#v done=%v deltaOnly=%v err=%v", msg, done, deltaOnly, err)
		}
		if got := s.functionArgBufs["item_1"].String(); got != `{"city":"` {
			t.Fatalf("unexpected buffered args %q", got)
		}

		msg, done, deltaOnly, err = s.consume(mustJSON[responses.ResponseStreamEventUnion](t, map[string]any{
			"type":      "response.function_call_arguments.done",
			"item_id":   "item_1",
			"arguments": `{"city":"beijing"}`,
		}))
		if err != nil || done || !deltaOnly || msg == nil {
			t.Fatalf("unexpected function-call done result: msg=%#v done=%v deltaOnly=%v err=%v", msg, done, deltaOnly, err)
		}
		if len(msg.ToolCalls) != 1 || msg.ToolCalls[0].ID != "call_123" || msg.ToolCalls[0].Function.Name != "lookup_weather" || msg.ToolCalls[0].Function.Arguments != `{"city":"beijing"}` {
			t.Fatalf("unexpected emitted tool call: %#v", msg.ToolCalls)
		}

		msg, done, deltaOnly, err = s.consume(mustJSON[responses.ResponseStreamEventUnion](t, map[string]any{
			"type":    "response.function_call_arguments.done",
			"item_id": "item_fallback",
			"name":    "fallback_tool",
		}))
		if err != nil || done || !deltaOnly || msg == nil {
			t.Fatalf("unexpected fallback done result: msg=%#v done=%v deltaOnly=%v err=%v", msg, done, deltaOnly, err)
		}
		if msg.ToolCalls[0].ID != "item_fallback" || msg.ToolCalls[0].Function.Name != "" {
			t.Fatalf("unexpected fallback tool call: %#v", msg.ToolCalls[0])
		}
	})

	t.Run("completion and terminal events", func(t *testing.T) {
		s := newStreamState()
		msg, done, deltaOnly, err := s.consume(mustJSON[responses.ResponseStreamEventUnion](t, map[string]any{
			"type": "response.completed",
			"response": map[string]any{
				"status": "completed",
				"usage":  map[string]any{"input_tokens": 3, "output_tokens": 2, "total_tokens": 5},
			},
		}))
		if err != nil || !done || deltaOnly || msg == nil {
			t.Fatalf("unexpected completed result: msg=%#v done=%v deltaOnly=%v err=%v", msg, done, deltaOnly, err)
		}
		if msg.ResponseMeta == nil || msg.ResponseMeta.FinishReason != string(responses.ResponseStatusCompleted) || msg.ResponseMeta.Usage == nil || msg.ResponseMeta.Usage.TotalTokens != 5 {
			t.Fatalf("unexpected completed response meta: %#v", msg.ResponseMeta)
		}

		msg, done, deltaOnly, err = s.consume(mustJSON[responses.ResponseStreamEventUnion](t, map[string]any{
			"type":     "response.failed",
			"response": map[string]any{"status": "failed"},
		}))
		if err != nil || !done || deltaOnly || msg == nil || msg.ResponseMeta.FinishReason != string(responses.ResponseStatusFailed) {
			t.Fatalf("unexpected failed result: msg=%#v done=%v deltaOnly=%v err=%v", msg, done, deltaOnly, err)
		}

		msg, done, deltaOnly, err = s.consume(mustJSON[responses.ResponseStreamEventUnion](t, map[string]any{
			"type": "response.incomplete",
			"response": map[string]any{
				"status": "incomplete",
				"usage":  map[string]any{"input_tokens": 1, "output_tokens": 1, "total_tokens": 2},
			},
		}))
		if err != nil || !done || deltaOnly || msg == nil || msg.ResponseMeta.FinishReason != string(responses.ResponseStatusIncomplete) || msg.ResponseMeta.Usage == nil || msg.ResponseMeta.Usage.TotalTokens != 2 {
			t.Fatalf("unexpected incomplete result: msg=%#v done=%v deltaOnly=%v err=%v", msg, done, deltaOnly, err)
		}
	})

	t.Run("error and unknown events", func(t *testing.T) {
		s := newStreamState()
		msg, done, deltaOnly, err := s.consume(mustJSON[responses.ResponseStreamEventUnion](t, map[string]any{
			"type":    "error",
			"message": "boom",
			"code":    "bad_request",
		}))
		if err == nil || !strings.Contains(err.Error(), "boom") || msg != nil || done || deltaOnly {
			t.Fatalf("expected stream error, got msg=%#v done=%v deltaOnly=%v err=%v", msg, done, deltaOnly, err)
		}

		msg, done, deltaOnly, err = s.consume(responses.ResponseStreamEventUnion{Type: "response.output_item.done"})
		if err != nil || msg != nil || done || !deltaOnly {
			t.Fatalf("expected unknown event to be ignored, got msg=%#v done=%v deltaOnly=%v err=%v", msg, done, deltaOnly, err)
		}
	})
}

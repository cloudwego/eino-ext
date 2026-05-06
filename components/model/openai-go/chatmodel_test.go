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
	"context"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestNewChatModel_NilConfig(t *testing.T) {
	cm, err := NewChatModel(context.Background(), nil)
	if err == nil {
		t.Fatalf("expected error")
	}
	if cm != nil {
		t.Fatalf("expected nil model")
	}
}

func TestNewChatModel_Basic(t *testing.T) {
	cm, err := NewChatModel(context.Background(), &Config{APIKey: "test", Model: "gpt-4o-mini"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cm == nil {
		t.Fatalf("expected non-nil model")
	}
	if cm.GetType() != typ {
		t.Fatalf("expected type %q, got %q", typ, cm.GetType())
	}
	if !cm.IsCallbacksEnabled() {
		t.Fatalf("expected callbacks enabled")
	}
}

func TestNewChatModel_ClonesConfigMaps(t *testing.T) {
	metadata := map[string]string{"source": "config"}
	extra := map[string]any{"trace_id": "abc123"}

	cm, err := NewChatModel(context.Background(), &Config{
		Model:       "gpt-4o-mini",
		Metadata:    metadata,
		ExtraFields: extra,
		Reasoning: &Reasoning{
			Effort:  ReasoningEffortLow,
			Summary: ReasoningSummaryDetailed,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	metadata["source"] = "mutated"
	extra["trace_id"] = "changed"

	if got := cm.metadata["source"]; got != "config" {
		t.Fatalf("expected cloned metadata to stay unchanged, got %q", got)
	}
	if got := cm.extraFields["trace_id"]; got != "abc123" {
		t.Fatalf("expected cloned extra fields to stay unchanged, got %#v", got)
	}
	if cm.reasoning == nil || cm.reasoning.Effort != ReasoningEffortLow || cm.reasoning.Summary != ReasoningSummaryDetailed {
		t.Fatalf("unexpected reasoning config: %#v", cm.reasoning)
	}
}

func TestWithTools(t *testing.T) {
	cm := &ChatModel{}

	if _, err := cm.WithTools(nil); err == nil {
		t.Fatalf("expected error for empty tools")
	}

	if _, err := cm.WithTools([]*schema.ToolInfo{nil}); err == nil {
		t.Fatalf("expected error for nil tool")
	}

	tool := makeWeatherTool()
	binding, err := cm.WithTools([]*schema.ToolInfo{tool})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	bound, ok := binding.(*ChatModel)
	if !ok {
		t.Fatalf("expected *ChatModel, got %T", binding)
	}
	if bound == cm {
		t.Fatalf("expected WithTools to return a bound copy")
	}
	if len(bound.tools) != 1 || len(bound.rawTools) != 1 {
		t.Fatalf("expected one bound tool, got tools=%d rawTools=%d", len(bound.tools), len(bound.rawTools))
	}
	if bound.rawTools[0] != tool {
		t.Fatalf("expected raw tool to be preserved")
	}
	if bound.toolChoice == nil || *bound.toolChoice != schema.ToolChoiceAllowed {
		t.Fatalf("expected allowed tool choice, got %#v", bound.toolChoice)
	}
	if len(cm.tools) != 0 || len(cm.rawTools) != 0 || cm.toolChoice != nil {
		t.Fatalf("expected receiver to remain unchanged")
	}
}

func TestToInputItems_ToolOutputString(t *testing.T) {
	items, err := toInputItems([]*schema.Message{{
		Role:       schema.Tool,
		ToolCallID: "call_1",
		Content:    "ok",
	}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
}

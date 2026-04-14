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

package acp

import (
	"encoding/json"
	"fmt"
	"iter"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	acpproto "github.com/eino-contrib/acp"
)

// collectUpdates drains an iter.Seq2 into slices for assertion.
func collectUpdates(seq iter.Seq2[acpproto.SessionUpdate, error]) ([]acpproto.SessionUpdate, []error) {
	var updates []acpproto.SessionUpdate
	var errs []error
	for su, err := range seq {
		if err != nil {
			errs = append(errs, err)
		} else {
			updates = append(updates, su)
		}
	}
	return updates, errs
}

func requireTextChunk(t *testing.T, su acpproto.SessionUpdate, wantText string) {
	t.Helper()
	chunk, ok := su.AsAgentMessageChunk()
	if !ok {
		t.Fatalf("expected AgentMessageChunk, got different variant")
	}
	tb, ok := chunk.Content.AsText()
	if !ok {
		t.Fatalf("expected text content block")
	}
	if tb.Text != wantText {
		t.Fatalf("text = %q, want %q", tb.Text, wantText)
	}
}

// --- AgentEventToSessionUpdate tests ---

func TestAgentEventToSessionUpdate_NilOutputNilAction(t *testing.T) {
	event := &adk.AgentEvent{}
	updates, errs := collectUpdates(AgentEventToSessionUpdate(event, nil))
	if len(updates) != 0 || len(errs) != 0 {
		t.Fatalf("expected no updates/errors, got %d updates, %d errors", len(updates), len(errs))
	}
}

func TestAgentEventToSessionUpdate_AssistantTextMessage(t *testing.T) {
	event := &adk.AgentEvent{
		Output: &adk.AgentOutput{
			MessageOutput: &adk.MessageVariant{
				Message: &schema.Message{Role: schema.Assistant, Content: "hello world"},
			},
		},
	}
	updates, errs := collectUpdates(AgentEventToSessionUpdate(event, nil))
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(updates))
	}
	requireTextChunk(t, updates[0], "hello world")
}

func TestAgentEventToSessionUpdate_AssistantWithReasoning(t *testing.T) {
	event := &adk.AgentEvent{
		Output: &adk.AgentOutput{
			MessageOutput: &adk.MessageVariant{
				Message: &schema.Message{
					Role:             schema.Assistant,
					Content:          "answer",
					ReasoningContent: "thinking...",
				},
			},
		},
	}
	updates, errs := collectUpdates(AgentEventToSessionUpdate(event, nil))
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(updates) != 2 {
		t.Fatalf("expected 2 updates, got %d", len(updates))
	}

	// First should be thought chunk
	thought, ok := updates[0].AsAgentThoughtChunk()
	if !ok {
		t.Fatal("expected AgentThoughtChunk for first update")
	}
	tb, ok := thought.Content.AsText()
	if !ok {
		t.Fatal("expected text content block in thought")
	}
	if tb.Text != "thinking..." {
		t.Fatalf("thought text = %q, want %q", tb.Text, "thinking...")
	}

	// Second should be message chunk
	requireTextChunk(t, updates[1], "answer")
}

func TestAgentEventToSessionUpdate_UserMessage(t *testing.T) {
	event := &adk.AgentEvent{
		Output: &adk.AgentOutput{
			MessageOutput: &adk.MessageVariant{
				Message: &schema.Message{Role: schema.User, Content: "user input"},
			},
		},
	}
	updates, errs := collectUpdates(AgentEventToSessionUpdate(event, nil))
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(updates))
	}
	chunk, ok := updates[0].AsUserMessageChunk()
	if !ok {
		t.Fatal("expected UserMessageChunk")
	}
	tb, ok := chunk.Content.AsText()
	if !ok {
		t.Fatal("expected text content block")
	}
	if tb.Text != "user input" {
		t.Fatalf("text = %q, want %q", tb.Text, "user input")
	}
}

func TestAgentEventToSessionUpdate_ToolCallAndResult(t *testing.T) {
	// Assistant message with tool calls
	event := &adk.AgentEvent{
		Output: &adk.AgentOutput{
			MessageOutput: &adk.MessageVariant{
				Message: &schema.Message{
					Role: schema.Assistant,
					ToolCalls: []schema.ToolCall{
						{
							ID:       "tc-1",
							Function: schema.FunctionCall{Name: "read_file", Arguments: `{"path":"/tmp/a.txt"}`},
						},
					},
				},
			},
		},
	}
	updates, errs := collectUpdates(AgentEventToSessionUpdate(event, nil))
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(updates))
	}
	tc, ok := updates[0].AsToolCall()
	if !ok {
		t.Fatal("expected ToolCall update")
	}
	if string(tc.ToolCallID) != "tc-1" {
		t.Fatalf("tool call id = %q, want %q", tc.ToolCallID, "tc-1")
	}
	if tc.Title != "read_file" {
		t.Fatalf("tool call title = %q, want %q", tc.Title, "read_file")
	}

	// Tool result message
	toolEvent := &adk.AgentEvent{
		Output: &adk.AgentOutput{
			MessageOutput: &adk.MessageVariant{
				Message: &schema.Message{
					Role:       schema.Tool,
					ToolCallID: "tc-1",
					Content:    "file content here",
				},
			},
		},
	}
	updates, errs = collectUpdates(AgentEventToSessionUpdate(toolEvent, nil))
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(updates))
	}
	tcu, ok := updates[0].AsToolCallUpdate()
	if !ok {
		t.Fatal("expected ToolCallUpdate")
	}
	if string(tcu.ToolCallID) != "tc-1" {
		t.Fatalf("tool call update id = %q, want %q", tcu.ToolCallID, "tc-1")
	}
}

func TestAgentEventToSessionUpdate_StreamingMessage(t *testing.T) {
	reader, writer := schema.Pipe[*schema.Message](3)
	go func() {
		writer.Send(&schema.Message{Role: schema.Assistant, Content: "chunk1"}, nil)
		writer.Send(&schema.Message{Role: schema.Assistant, Content: "chunk2"}, nil)
		writer.Close()
	}()

	event := &adk.AgentEvent{
		Output: &adk.AgentOutput{
			MessageOutput: &adk.MessageVariant{
				IsStreaming:   true,
				MessageStream: reader,
			},
		},
	}
	updates, errs := collectUpdates(AgentEventToSessionUpdate(event, nil))
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(updates) != 2 {
		t.Fatalf("expected 2 updates, got %d", len(updates))
	}
	requireTextChunk(t, updates[0], "chunk1")
	requireTextChunk(t, updates[1], "chunk2")
}

func TestAgentEventToSessionUpdate_StreamingInheritRole(t *testing.T) {
	reader, writer := schema.Pipe[*schema.Message](3)
	go func() {
		writer.Send(&schema.Message{Role: schema.Assistant, Content: "first"}, nil)
		// Subsequent chunks have empty role — should inherit "assistant"
		writer.Send(&schema.Message{Content: "second"}, nil)
		writer.Send(&schema.Message{Content: "third"}, nil)
		writer.Close()
	}()

	event := &adk.AgentEvent{
		Output: &adk.AgentOutput{
			MessageOutput: &adk.MessageVariant{
				IsStreaming:   true,
				MessageStream: reader,
			},
		},
	}
	updates, errs := collectUpdates(AgentEventToSessionUpdate(event, nil))
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(updates) != 3 {
		t.Fatalf("expected 3 updates, got %d", len(updates))
	}
	// All three should be AgentMessageChunk (inherited assistant role)
	for i, wantText := range []string{"first", "second", "third"} {
		requireTextChunk(t, updates[i], wantText)
	}
}

func TestAgentEventToSessionUpdate_StreamingToolCallAccumulation(t *testing.T) {
	reader, writer := schema.Pipe[*schema.Message](10)
	go func() {
		// Model streams tool call arguments in chunks:
		// chunk 1: tool call starts with partial arguments
		writer.Send(&schema.Message{
			Role: schema.Assistant,
			ToolCalls: []schema.ToolCall{
				{ID: "tc-1", Function: schema.FunctionCall{Name: "read_file", Arguments: `{"pat`}},
			},
		}, nil)
		// chunk 2: more argument data
		writer.Send(&schema.Message{
			Role: schema.Assistant,
			ToolCalls: []schema.ToolCall{
				{ID: "tc-1", Function: schema.FunctionCall{Arguments: `h":"/tmp`}},
			},
		}, nil)
		// chunk 3: argument completes
		writer.Send(&schema.Message{
			Role: schema.Assistant,
			ToolCalls: []schema.ToolCall{
				{ID: "tc-1", Function: schema.FunctionCall{Arguments: `/a.txt"}`}},
			},
		}, nil)
		// Next message is a tool result — triggers flush of pending tool calls
		writer.Send(&schema.Message{
			Role:       schema.Tool,
			ToolCallID: "tc-1",
			Content:    "file content",
		}, nil)
		writer.Close()
	}()

	event := &adk.AgentEvent{
		Output: &adk.AgentOutput{
			MessageOutput: &adk.MessageVariant{
				IsStreaming:   true,
				MessageStream: reader,
			},
		},
	}
	updates, errs := collectUpdates(AgentEventToSessionUpdate(event, nil))
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	// Should have: 1 ToolCall (accumulated) + 1 ToolCallUpdate (result)
	if len(updates) != 2 {
		t.Fatalf("expected 2 updates, got %d", len(updates))
	}

	// First: accumulated ToolCall with complete JSON
	tc, ok := updates[0].AsToolCall()
	if !ok {
		t.Fatal("expected ToolCall for first update")
	}
	if string(tc.ToolCallID) != "tc-1" {
		t.Fatalf("tool call id = %q, want %q", tc.ToolCallID, "tc-1")
	}
	if tc.Title != "read_file" {
		t.Fatalf("tool call title = %q, want %q", tc.Title, "read_file")
	}
	wantArgs := `{"path":"/tmp/a.txt"}`
	if string(tc.RawInput) != wantArgs {
		t.Fatalf("RawInput = %s, want %s", tc.RawInput, wantArgs)
	}
	// Verify it's valid JSON
	if !json.Valid(tc.RawInput) {
		t.Fatalf("RawInput is not valid JSON: %s", tc.RawInput)
	}

	// Second: tool result
	tcu, ok := updates[1].AsToolCallUpdate()
	if !ok {
		t.Fatal("expected ToolCallUpdate for second update")
	}
	if string(tcu.ToolCallID) != "tc-1" {
		t.Fatalf("tool call update id = %q, want %q", tcu.ToolCallID, "tc-1")
	}
}

func TestAgentEventToSessionUpdate_StreamingToolCallFlushOnEOF(t *testing.T) {
	reader, writer := schema.Pipe[*schema.Message](5)
	go func() {
		// Tool call streamed in two chunks, then stream ends (no subsequent non-tool message)
		writer.Send(&schema.Message{
			Role: schema.Assistant,
			ToolCalls: []schema.ToolCall{
				{ID: "tc-2", Function: schema.FunctionCall{Name: "search", Arguments: `{"q":`}},
			},
		}, nil)
		writer.Send(&schema.Message{
			Role: schema.Assistant,
			ToolCalls: []schema.ToolCall{
				{ID: "tc-2", Function: schema.FunctionCall{Arguments: `"hello"}`}},
			},
		}, nil)
		writer.Close()
	}()

	event := &adk.AgentEvent{
		Output: &adk.AgentOutput{
			MessageOutput: &adk.MessageVariant{
				IsStreaming:   true,
				MessageStream: reader,
			},
		},
	}
	updates, errs := collectUpdates(AgentEventToSessionUpdate(event, nil))
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(updates))
	}

	tc, ok := updates[0].AsToolCall()
	if !ok {
		t.Fatal("expected ToolCall")
	}
	wantArgs := `{"q":"hello"}`
	if string(tc.RawInput) != wantArgs {
		t.Fatalf("RawInput = %s, want %s", tc.RawInput, wantArgs)
	}
	if !json.Valid(tc.RawInput) {
		t.Fatalf("RawInput is not valid JSON: %s", tc.RawInput)
	}
}

func TestAgentEventToSessionUpdate_StreamingToolCallWithTextContent(t *testing.T) {
	reader, writer := schema.Pipe[*schema.Message](5)
	go func() {
		// Message has both text content and tool calls in the same chunk
		writer.Send(&schema.Message{
			Role:    schema.Assistant,
			Content: "Let me search for that",
			ToolCalls: []schema.ToolCall{
				{ID: "tc-3", Function: schema.FunctionCall{Name: "search", Arguments: `{"q":"test"}`}},
			},
		}, nil)
		writer.Close()
	}()

	event := &adk.AgentEvent{
		Output: &adk.AgentOutput{
			MessageOutput: &adk.MessageVariant{
				IsStreaming:   true,
				MessageStream: reader,
			},
		},
	}
	updates, errs := collectUpdates(AgentEventToSessionUpdate(event, nil))
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	// Should have: 1 AgentMessageChunk (text) + 1 ToolCall (flushed on EOF)
	if len(updates) != 2 {
		t.Fatalf("expected 2 updates, got %d", len(updates))
	}
	requireTextChunk(t, updates[0], "Let me search for that")
	tc, ok := updates[1].AsToolCall()
	if !ok {
		t.Fatal("expected ToolCall for second update")
	}
	if tc.Title != "search" {
		t.Fatalf("title = %q, want %q", tc.Title, "search")
	}
}

func TestAgentEventToSessionUpdate_ToolMessageNoContent(t *testing.T) {
	event := &adk.AgentEvent{
		Output: &adk.AgentOutput{
			MessageOutput: &adk.MessageVariant{
				Message: &schema.Message{
					Role:       schema.Tool,
					ToolCallID: "tc-empty",
				},
			},
		},
	}
	_, errs := collectUpdates(AgentEventToSessionUpdate(event, nil))
	if len(errs) == 0 {
		t.Fatal("expected error for tool message with no content")
	}
}

// --- Interrupt conversion tests ---

func TestDefaultInterruptConverter_StringData(t *testing.T) {
	info := &adk.InterruptInfo{Data: "please confirm"}
	updates, errs := collectUpdates(defaultInterruptConverter(info))
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(updates))
	}
	chunk, ok := updates[0].AsAgentMessageChunk()
	if !ok {
		t.Fatal("expected AgentMessageChunk")
	}

	// Check text
	tb, ok := chunk.Content.AsText()
	if !ok {
		t.Fatal("expected text content block")
	}
	if tb.Text != "please confirm" {
		t.Fatalf("text = %q, want %q", tb.Text, "please confirm")
	}

	// Check meta
	if chunk.Meta == nil {
		t.Fatal("expected meta to be set")
	}
	if chunk.Meta["eino:interrupted"] != true {
		t.Fatal("expected eino:interrupted = true in meta")
	}
}

func TestDefaultInterruptConverter_NilData(t *testing.T) {
	info := &adk.InterruptInfo{Data: nil}
	updates, errs := collectUpdates(defaultInterruptConverter(info))
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(updates))
	}
	chunk, ok := updates[0].AsAgentMessageChunk()
	if !ok {
		t.Fatal("expected AgentMessageChunk")
	}
	tb, _ := chunk.Content.AsText()
	if tb.Text != "" {
		t.Fatalf("expected empty text for nil data, got %q", tb.Text)
	}
}

func TestDefaultInterruptConverter_StructData(t *testing.T) {
	data := map[string]string{"action": "delete", "file": "/tmp/x"}
	info := &adk.InterruptInfo{Data: data}
	updates, errs := collectUpdates(defaultInterruptConverter(info))
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(updates))
	}
	chunk, ok := updates[0].AsAgentMessageChunk()
	if !ok {
		t.Fatal("expected AgentMessageChunk")
	}
	tb, _ := chunk.Content.AsText()

	// Should be JSON-serialized
	var parsed map[string]string
	if err := json.Unmarshal([]byte(tb.Text), &parsed); err != nil {
		t.Fatalf("text is not valid JSON: %v, text=%q", err, tb.Text)
	}
	if parsed["action"] != "delete" || parsed["file"] != "/tmp/x" {
		t.Fatalf("unexpected parsed data: %v", parsed)
	}
}

func TestDefaultInterruptConverter_WithContexts(t *testing.T) {
	parent := &adk.InterruptCtx{
		ID:          "agent:root",
		IsRootCause: false,
		Address: adk.Address{
			{Type: "agent", ID: "root"},
		},
		Info: "root agent paused",
	}
	child := &adk.InterruptCtx{
		ID:          "agent:root;tool:bash",
		IsRootCause: true,
		Address: adk.Address{
			{Type: "agent", ID: "root"},
			{Type: "tool", ID: "bash"},
		},
		Info:   map[string]string{"command": "rm -rf /"},
		Parent: parent,
	}
	info := &adk.InterruptInfo{
		Data:              "need confirmation",
		InterruptContexts: []*adk.InterruptCtx{parent, child},
	}
	updates, errs := collectUpdates(defaultInterruptConverter(info))
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(updates))
	}
	chunk, ok := updates[0].AsAgentMessageChunk()
	if !ok {
		t.Fatal("expected AgentMessageChunk")
	}

	contexts, ok := chunk.Meta["eino:interruptContexts"].([]map[string]any)
	if !ok {
		t.Fatalf("expected eino:interruptContexts in meta, got %T", chunk.Meta["eino:interruptContexts"])
	}
	if len(contexts) != 2 {
		t.Fatalf("expected 2 contexts, got %d", len(contexts))
	}

	// Check parent context
	if contexts[0]["id"] != "agent:root" {
		t.Fatalf("parent id = %v, want %q", contexts[0]["id"], "agent:root")
	}
	if contexts[0]["isRootCause"] != false {
		t.Fatalf("parent isRootCause = %v, want false", contexts[0]["isRootCause"])
	}
	if contexts[0]["info"] != "root agent paused" {
		t.Fatalf("parent info = %v, want %q", contexts[0]["info"], "root agent paused")
	}

	// Check child context
	if contexts[1]["id"] != "agent:root;tool:bash" {
		t.Fatalf("child id = %v, want %q", contexts[1]["id"], "agent:root;tool:bash")
	}
	if contexts[1]["isRootCause"] != true {
		t.Fatalf("child isRootCause = %v, want true", contexts[1]["isRootCause"])
	}
	if contexts[1]["parentId"] != "agent:root" {
		t.Fatalf("child parentId = %v, want %q", contexts[1]["parentId"], "agent:root")
	}

	// Child address should have 2 segments
	childAddr, ok := contexts[1]["address"].([]map[string]string)
	if !ok {
		t.Fatalf("child address type = %T, want []map[string]string", contexts[1]["address"])
	}
	if len(childAddr) != 2 {
		t.Fatalf("child address len = %d, want 2", len(childAddr))
	}
	if childAddr[1]["type"] != "tool" || childAddr[1]["id"] != "bash" {
		t.Fatalf("child address[1] = %v, want type=tool id=bash", childAddr[1])
	}
}

func TestDefaultInterruptConverter_MetaIsJSONSerializable(t *testing.T) {
	info := &adk.InterruptInfo{
		Data: "test",
		InterruptContexts: []*adk.InterruptCtx{
			{
				ID:          "agent:a",
				IsRootCause: true,
				Address:     adk.Address{{Type: "agent", ID: "a"}},
				Info:        map[string]int{"code": 42},
			},
		},
	}
	updates, errs := collectUpdates(defaultInterruptConverter(info))
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	chunk, _ := updates[0].AsAgentMessageChunk()

	// The entire meta must be JSON-serializable since it will be sent over the wire.
	b, err := json.Marshal(chunk.Meta)
	if err != nil {
		t.Fatalf("meta is not JSON serializable: %v", err)
	}
	if len(b) == 0 {
		t.Fatal("empty JSON output")
	}
}

func TestAgentEventToSessionUpdate_CustomInterruptConverter(t *testing.T) {
	customConverter := func(info *adk.InterruptInfo) iter.Seq2[acpproto.SessionUpdate, error] {
		return func(yield func(acpproto.SessionUpdate, error) bool) {
			yield(acpproto.NewSessionUpdateAgentMessageChunk(acpproto.ContentChunk{
				Content: acpproto.NewContentBlockText(acpproto.TextContent{
					Text: fmt.Sprintf("CUSTOM: %v", info.Data),
				}),
			}), nil)
		}
	}

	event := &adk.AgentEvent{
		Action: &adk.AgentAction{
			Interrupted: &adk.InterruptInfo{Data: "confirm?"},
		},
	}
	opt := &EventConverterOption{InterruptConverter: customConverter}
	updates, errs := collectUpdates(AgentEventToSessionUpdate(event, opt))
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(updates))
	}
	requireTextChunk(t, updates[0], "CUSTOM: confirm?")
}

func TestAgentEventToSessionUpdate_InterruptWithOutput(t *testing.T) {
	// An event can have both an interrupt action and message output.
	event := &adk.AgentEvent{
		Action: &adk.AgentAction{
			Interrupted: &adk.InterruptInfo{Data: "paused"},
		},
		Output: &adk.AgentOutput{
			MessageOutput: &adk.MessageVariant{
				Message: &schema.Message{Role: schema.Assistant, Content: "partial result"},
			},
		},
	}
	updates, errs := collectUpdates(AgentEventToSessionUpdate(event, nil))
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	// Should have interrupt update + message update
	if len(updates) != 2 {
		t.Fatalf("expected 2 updates, got %d", len(updates))
	}
	// First: interrupt as AgentMessageChunk
	chunk, ok := updates[0].AsAgentMessageChunk()
	if !ok {
		t.Fatal("expected first update to be AgentMessageChunk (interrupt)")
	}
	if chunk.Meta["eino:interrupted"] != true {
		t.Fatal("expected eino:interrupted in meta")
	}
	// Second: normal message
	requireTextChunk(t, updates[1], "partial result")
}

// --- inputPartToContentBlock tests ---

func TestInputPartToContentBlock_Text(t *testing.T) {
	part := schema.MessageInputPart{Type: schema.ChatMessagePartTypeText, Text: "hello"}
	cb, err := inputPartToContentBlock(part)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tb, ok := cb.AsText()
	if !ok {
		t.Fatal("expected text block")
	}
	if tb.Text != "hello" {
		t.Fatalf("text = %q, want %q", tb.Text, "hello")
	}
}

func TestInputPartToContentBlock_ImageURL(t *testing.T) {
	url := "https://example.com/img.png"
	part := schema.MessageInputPart{
		Type: schema.ChatMessagePartTypeImageURL,
		Image: &schema.MessageInputImage{
			MessagePartCommon: schema.MessagePartCommon{URL: &url, MIMEType: "image/png"},
		},
	}
	cb, err := inputPartToContentBlock(part)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	img, ok := cb.AsImage()
	if !ok {
		t.Fatal("expected image block")
	}
	if img.URI != url {
		t.Fatalf("URI = %q, want %q", img.URI, url)
	}
}

func TestInputPartToContentBlock_ImageBase64(t *testing.T) {
	b64 := "iVBORw0KGgo="
	part := schema.MessageInputPart{
		Type: schema.ChatMessagePartTypeImageURL,
		Image: &schema.MessageInputImage{
			MessagePartCommon: schema.MessagePartCommon{Base64Data: &b64, MIMEType: "image/png"},
		},
	}
	cb, err := inputPartToContentBlock(part)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	img, ok := cb.AsImage()
	if !ok {
		t.Fatal("expected image block")
	}
	if img.Data != b64 {
		t.Fatalf("Data = %q, want %q", img.Data, b64)
	}
}

func TestInputPartToContentBlock_ImageNil(t *testing.T) {
	part := schema.MessageInputPart{Type: schema.ChatMessagePartTypeImageURL}
	_, err := inputPartToContentBlock(part)
	if err == nil {
		t.Fatal("expected error for nil image")
	}
}

func TestInputPartToContentBlock_AudioBase64(t *testing.T) {
	b64 := "AAAA"
	part := schema.MessageInputPart{
		Type: schema.ChatMessagePartTypeAudioURL,
		Audio: &schema.MessageInputAudio{
			MessagePartCommon: schema.MessagePartCommon{Base64Data: &b64, MIMEType: "audio/wav"},
		},
	}
	cb, err := inputPartToContentBlock(part)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	audio, ok := cb.AsAudio()
	if !ok {
		t.Fatal("expected audio block")
	}
	if audio.Data != b64 {
		t.Fatalf("Data = %q, want %q", audio.Data, b64)
	}
}

func TestInputPartToContentBlock_FileURL(t *testing.T) {
	url := "https://example.com/doc.pdf"
	part := schema.MessageInputPart{
		Type: schema.ChatMessagePartTypeFileURL,
		File: &schema.MessageInputFile{
			MessagePartCommon: schema.MessagePartCommon{URL: &url, MIMEType: "application/pdf"},
			Name:              "doc.pdf",
		},
	}
	cb, err := inputPartToContentBlock(part)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rl, ok := cb.AsResourceLink()
	if !ok {
		t.Fatal("expected resource link block")
	}
	if rl.URI != url || rl.Name != "doc.pdf" {
		t.Fatalf("resource link = %+v, want URI=%q Name=%q", rl, url, "doc.pdf")
	}
}

func TestInputPartToContentBlock_UnsupportedType(t *testing.T) {
	part := schema.MessageInputPart{Type: "unknown_type"}
	_, err := inputPartToContentBlock(part)
	if err == nil {
		t.Fatal("expected error for unsupported type")
	}
}

// --- outputPartToSessionUpdate tests ---

func TestOutputPartToSessionUpdate_Text(t *testing.T) {
	part := schema.MessageOutputPart{Type: schema.ChatMessagePartTypeText, Text: "output"}
	su, err := outputPartToSessionUpdate(part)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	chunk, ok := su.AsAgentMessageChunk()
	if !ok {
		t.Fatal("expected AgentMessageChunk")
	}
	tb, _ := chunk.Content.AsText()
	if tb.Text != "output" {
		t.Fatalf("text = %q, want %q", tb.Text, "output")
	}
}

func TestOutputPartToSessionUpdate_Reasoning(t *testing.T) {
	part := schema.MessageOutputPart{
		Type:      schema.ChatMessagePartTypeReasoning,
		Reasoning: &schema.MessageOutputReasoning{Text: "let me think"},
	}
	su, err := outputPartToSessionUpdate(part)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	chunk, ok := su.AsAgentThoughtChunk()
	if !ok {
		t.Fatal("expected AgentThoughtChunk")
	}
	tb, _ := chunk.Content.AsText()
	if tb.Text != "let me think" {
		t.Fatalf("text = %q, want %q", tb.Text, "let me think")
	}
}

func TestOutputPartToSessionUpdate_ReasoningNil(t *testing.T) {
	part := schema.MessageOutputPart{Type: schema.ChatMessagePartTypeReasoning}
	_, err := outputPartToSessionUpdate(part)
	if err == nil {
		t.Fatal("expected error for nil reasoning")
	}
}

// --- fromToolCall test ---

func TestFromToolCall(t *testing.T) {
	tc := schema.ToolCall{
		ID:       "call-123",
		Function: schema.FunctionCall{Name: "search", Arguments: `{"q":"test"}`},
	}
	result := fromToolCall(tc)
	if string(result.ToolCallID) != "call-123" {
		t.Fatalf("ToolCallID = %q, want %q", result.ToolCallID, "call-123")
	}
	if result.Title != "search" {
		t.Fatalf("Title = %q, want %q", result.Title, "search")
	}
	if string(result.RawInput) != `{"q":"test"}` {
		t.Fatalf("RawInput = %s, want %s", result.RawInput, `{"q":"test"}`)
	}
}

// --- marshalInterruptInfo tests ---

func TestMarshalInterruptInfo_NonSerializableData(t *testing.T) {
	// A channel is not JSON-serializable; should fall back to fmt.Sprintf
	ch := make(chan int)
	info := &adk.InterruptInfo{Data: ch}
	text, meta, err := marshalInterruptInfo(info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text == "" {
		t.Fatal("expected non-empty fallback text")
	}
	if meta["eino:interrupted"] != true {
		t.Fatal("expected eino:interrupted = true")
	}
}

// --- interruptCtxToMap tests ---

func TestInterruptCtxToMap_MinimalFields(t *testing.T) {
	ic := &adk.InterruptCtx{
		ID:          "agent:x",
		IsRootCause: true,
	}
	m := interruptCtxToMap(ic)
	if m["id"] != "agent:x" {
		t.Fatalf("id = %v, want %q", m["id"], "agent:x")
	}
	if m["isRootCause"] != true {
		t.Fatalf("isRootCause = %v, want true", m["isRootCause"])
	}
	if _, exists := m["address"]; exists {
		t.Fatal("expected no address for empty Address")
	}
	if _, exists := m["info"]; exists {
		t.Fatal("expected no info for nil Info")
	}
	if _, exists := m["parentId"]; exists {
		t.Fatal("expected no parentId for nil Parent")
	}
}

func TestInterruptCtxToMap_NonSerializableInfo(t *testing.T) {
	ch := make(chan struct{})
	ic := &adk.InterruptCtx{
		ID:   "x",
		Info: ch,
	}
	m := interruptCtxToMap(ic)
	// Should fall back to Sprintf
	infoStr, ok := m["info"].(string)
	if !ok {
		t.Fatalf("expected string fallback for non-serializable info, got %T", m["info"])
	}
	if infoStr == "" {
		t.Fatal("expected non-empty fallback string")
	}
}

// --- yieldMessageUpdates: user multi-content ---

func TestYieldMessageUpdates_UserMultiContent(t *testing.T) {
	msg := &schema.Message{
		Role: schema.User,
		UserInputMultiContent: []schema.MessageInputPart{
			{Type: schema.ChatMessagePartTypeText, Text: "part1"},
			{Type: schema.ChatMessagePartTypeText, Text: "part2"},
		},
	}
	event := &adk.AgentEvent{
		Output: &adk.AgentOutput{
			MessageOutput: &adk.MessageVariant{Message: msg},
		},
	}
	updates, errs := collectUpdates(AgentEventToSessionUpdate(event, nil))
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(updates) != 2 {
		t.Fatalf("expected 2 updates, got %d", len(updates))
	}
	for i, wantText := range []string{"part1", "part2"} {
		chunk, ok := updates[i].AsUserMessageChunk()
		if !ok {
			t.Fatalf("update[%d]: expected UserMessageChunk", i)
		}
		tb, ok := chunk.Content.AsText()
		if !ok {
			t.Fatalf("update[%d]: expected text block", i)
		}
		if tb.Text != wantText {
			t.Fatalf("update[%d]: text = %q, want %q", i, tb.Text, wantText)
		}
	}
}

// --- yieldMessageUpdates: assistant with AssistantGenMultiContent ---

func TestYieldMessageUpdates_AssistantMultiContent(t *testing.T) {
	msg := &schema.Message{
		Role: schema.Assistant,
		AssistantGenMultiContent: []schema.MessageOutputPart{
			{Type: schema.ChatMessagePartTypeText, Text: "line1"},
			{Type: schema.ChatMessagePartTypeText, Text: "line2"},
		},
	}
	event := &adk.AgentEvent{
		Output: &adk.AgentOutput{
			MessageOutput: &adk.MessageVariant{Message: msg},
		},
	}
	updates, errs := collectUpdates(AgentEventToSessionUpdate(event, nil))
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(updates) != 2 {
		t.Fatalf("expected 2 updates, got %d", len(updates))
	}
	for i, wantText := range []string{"line1", "line2"} {
		requireTextChunk(t, updates[i], wantText)
	}
}

// --- yieldMessageUpdates: tool multi-content ---

func TestYieldMessageUpdates_ToolMultiContent(t *testing.T) {
	msg := &schema.Message{
		Role:       schema.Tool,
		ToolCallID: "tc-multi",
		UserInputMultiContent: []schema.MessageInputPart{
			{Type: schema.ChatMessagePartTypeText, Text: "result part 1"},
			{Type: schema.ChatMessagePartTypeText, Text: "result part 2"},
		},
	}
	event := &adk.AgentEvent{
		Output: &adk.AgentOutput{
			MessageOutput: &adk.MessageVariant{Message: msg},
		},
	}
	updates, errs := collectUpdates(AgentEventToSessionUpdate(event, nil))
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(updates) != 1 {
		t.Fatalf("expected 1 update (ToolCallUpdate), got %d", len(updates))
	}
	tcu, ok := updates[0].AsToolCallUpdate()
	if !ok {
		t.Fatal("expected ToolCallUpdate")
	}
	if string(tcu.ToolCallID) != "tc-multi" {
		t.Fatalf("tool call id = %q, want %q", tcu.ToolCallID, "tc-multi")
	}
}

// --- yieldMessageUpdates: unsupported role ---

func TestYieldMessageUpdates_UnsupportedRole(t *testing.T) {
	msg := &schema.Message{Role: "unknown_role"}
	event := &adk.AgentEvent{
		Output: &adk.AgentOutput{
			MessageOutput: &adk.MessageVariant{Message: msg},
		},
	}
	_, errs := collectUpdates(AgentEventToSessionUpdate(event, nil))
	if len(errs) == 0 {
		t.Fatal("expected error for unsupported role")
	}
}

// --- yieldMessageUpdates: assistant with tool calls and content ---

func TestYieldMessageUpdates_AssistantContentWithToolCalls(t *testing.T) {
	msg := &schema.Message{
		Role:    schema.Assistant,
		Content: "I'll read that file for you",
		ToolCalls: []schema.ToolCall{
			{ID: "tc-2", Function: schema.FunctionCall{Name: "read_file", Arguments: `{"path":"a.txt"}`}},
		},
	}
	event := &adk.AgentEvent{
		Output: &adk.AgentOutput{
			MessageOutput: &adk.MessageVariant{Message: msg},
		},
	}
	updates, errs := collectUpdates(AgentEventToSessionUpdate(event, nil))
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(updates) != 2 {
		t.Fatalf("expected 2 updates (message + tool call), got %d", len(updates))
	}
	requireTextChunk(t, updates[0], "I'll read that file for you")
	tc, ok := updates[1].AsToolCall()
	if !ok {
		t.Fatal("expected ToolCall for second update")
	}
	if tc.Title != "read_file" {
		t.Fatalf("tool call title = %q, want %q", tc.Title, "read_file")
	}
}

// --- streaming error ---

func TestAgentEventToSessionUpdate_StreamingError(t *testing.T) {
	reader, writer := schema.Pipe[*schema.Message](3)
	go func() {
		writer.Send(&schema.Message{Role: schema.Assistant, Content: "ok"}, nil)
		writer.Send(nil, fmt.Errorf("stream broken"))
		writer.Close()
	}()

	event := &adk.AgentEvent{
		Output: &adk.AgentOutput{
			MessageOutput: &adk.MessageVariant{
				IsStreaming:   true,
				MessageStream: reader,
			},
		},
	}
	updates, errs := collectUpdates(AgentEventToSessionUpdate(event, nil))
	if len(updates) != 1 {
		t.Fatalf("expected 1 update before error, got %d", len(updates))
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
}

// --- inputPartToContentBlock: more error cases ---

func TestInputPartToContentBlock_AudioNil(t *testing.T) {
	part := schema.MessageInputPart{Type: schema.ChatMessagePartTypeAudioURL}
	_, err := inputPartToContentBlock(part)
	if err == nil {
		t.Fatal("expected error for nil audio")
	}
}

func TestInputPartToContentBlock_AudioURLOnly(t *testing.T) {
	url := "https://example.com/audio.wav"
	part := schema.MessageInputPart{
		Type: schema.ChatMessagePartTypeAudioURL,
		Audio: &schema.MessageInputAudio{
			MessagePartCommon: schema.MessagePartCommon{URL: &url, MIMEType: "audio/wav"},
		},
	}
	_, err := inputPartToContentBlock(part)
	if err == nil {
		t.Fatal("expected error for audio with URL only (ACP only supports base64)")
	}
}

func TestInputPartToContentBlock_AudioNoData(t *testing.T) {
	part := schema.MessageInputPart{
		Type: schema.ChatMessagePartTypeAudioURL,
		Audio: &schema.MessageInputAudio{
			MessagePartCommon: schema.MessagePartCommon{MIMEType: "audio/wav"},
		},
	}
	_, err := inputPartToContentBlock(part)
	if err == nil {
		t.Fatal("expected error for audio with no data")
	}
}

func TestInputPartToContentBlock_ImageNoData(t *testing.T) {
	part := schema.MessageInputPart{
		Type: schema.ChatMessagePartTypeImageURL,
		Image: &schema.MessageInputImage{
			MessagePartCommon: schema.MessagePartCommon{MIMEType: "image/png"},
		},
	}
	_, err := inputPartToContentBlock(part)
	if err == nil {
		t.Fatal("expected error for image with no URL or base64")
	}
}

func TestInputPartToContentBlock_VideoURL(t *testing.T) {
	url := "https://example.com/video.mp4"
	part := schema.MessageInputPart{
		Type: schema.ChatMessagePartTypeVideoURL,
		Video: &schema.MessageInputVideo{
			MessagePartCommon: schema.MessagePartCommon{URL: &url, MIMEType: "video/mp4"},
		},
	}
	cb, err := inputPartToContentBlock(part)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rl, ok := cb.AsResourceLink()
	if !ok {
		t.Fatal("expected resource link block")
	}
	if rl.URI != url {
		t.Fatalf("URI = %q, want %q", rl.URI, url)
	}
}

func TestInputPartToContentBlock_VideoNil(t *testing.T) {
	part := schema.MessageInputPart{Type: schema.ChatMessagePartTypeVideoURL}
	_, err := inputPartToContentBlock(part)
	if err == nil {
		t.Fatal("expected error for nil video")
	}
}

func TestInputPartToContentBlock_VideoNoData(t *testing.T) {
	part := schema.MessageInputPart{
		Type: schema.ChatMessagePartTypeVideoURL,
		Video: &schema.MessageInputVideo{
			MessagePartCommon: schema.MessagePartCommon{MIMEType: "video/mp4"},
		},
	}
	_, err := inputPartToContentBlock(part)
	if err == nil {
		t.Fatal("expected error for video with no URL or base64")
	}
}

func TestInputPartToContentBlock_FileNil(t *testing.T) {
	part := schema.MessageInputPart{Type: schema.ChatMessagePartTypeFileURL}
	_, err := inputPartToContentBlock(part)
	if err == nil {
		t.Fatal("expected error for nil file")
	}
}

func TestInputPartToContentBlock_FileNoData(t *testing.T) {
	part := schema.MessageInputPart{
		Type: schema.ChatMessagePartTypeFileURL,
		File: &schema.MessageInputFile{
			MessagePartCommon: schema.MessagePartCommon{MIMEType: "text/plain"},
			Name:              "test.txt",
		},
	}
	_, err := inputPartToContentBlock(part)
	if err == nil {
		t.Fatal("expected error for file with no URL or base64")
	}
}

// --- outputPartToSessionUpdate: more cases ---

func TestOutputPartToSessionUpdate_ImageURL(t *testing.T) {
	url := "https://example.com/out.png"
	part := schema.MessageOutputPart{
		Type: schema.ChatMessagePartTypeImageURL,
		Image: &schema.MessageOutputImage{
			MessagePartCommon: schema.MessagePartCommon{URL: &url, MIMEType: "image/png"},
		},
	}
	su, err := outputPartToSessionUpdate(part)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	chunk, ok := su.AsAgentMessageChunk()
	if !ok {
		t.Fatal("expected AgentMessageChunk")
	}
	img, ok := chunk.Content.AsImage()
	if !ok {
		t.Fatal("expected image content block")
	}
	if img.URI != url {
		t.Fatalf("URI = %q, want %q", img.URI, url)
	}
}

func TestOutputPartToSessionUpdate_ImageBase64(t *testing.T) {
	b64 := "iVBORw0KGgo="
	part := schema.MessageOutputPart{
		Type: schema.ChatMessagePartTypeImageURL,
		Image: &schema.MessageOutputImage{
			MessagePartCommon: schema.MessagePartCommon{Base64Data: &b64, MIMEType: "image/png"},
		},
	}
	su, err := outputPartToSessionUpdate(part)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	chunk, ok := su.AsAgentMessageChunk()
	if !ok {
		t.Fatal("expected AgentMessageChunk")
	}
	img, ok := chunk.Content.AsImage()
	if !ok {
		t.Fatal("expected image content block")
	}
	if img.Data != b64 {
		t.Fatalf("Data = %q, want %q", img.Data, b64)
	}
}

func TestOutputPartToSessionUpdate_ImageNil(t *testing.T) {
	part := schema.MessageOutputPart{Type: schema.ChatMessagePartTypeImageURL}
	_, err := outputPartToSessionUpdate(part)
	if err == nil {
		t.Fatal("expected error for nil image")
	}
}

func TestOutputPartToSessionUpdate_ImageNoData(t *testing.T) {
	part := schema.MessageOutputPart{
		Type: schema.ChatMessagePartTypeImageURL,
		Image: &schema.MessageOutputImage{
			MessagePartCommon: schema.MessagePartCommon{MIMEType: "image/png"},
		},
	}
	_, err := outputPartToSessionUpdate(part)
	if err == nil {
		t.Fatal("expected error for image with no URL or base64")
	}
}

func TestOutputPartToSessionUpdate_AudioBase64(t *testing.T) {
	b64 := "AAAA"
	part := schema.MessageOutputPart{
		Type: schema.ChatMessagePartTypeAudioURL,
		Audio: &schema.MessageOutputAudio{
			MessagePartCommon: schema.MessagePartCommon{Base64Data: &b64, MIMEType: "audio/wav"},
		},
	}
	su, err := outputPartToSessionUpdate(part)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	chunk, ok := su.AsAgentMessageChunk()
	if !ok {
		t.Fatal("expected AgentMessageChunk")
	}
	audio, ok := chunk.Content.AsAudio()
	if !ok {
		t.Fatal("expected audio content block")
	}
	if audio.Data != b64 {
		t.Fatalf("Data = %q, want %q", audio.Data, b64)
	}
}

func TestOutputPartToSessionUpdate_AudioNil(t *testing.T) {
	part := schema.MessageOutputPart{Type: schema.ChatMessagePartTypeAudioURL}
	_, err := outputPartToSessionUpdate(part)
	if err == nil {
		t.Fatal("expected error for nil audio")
	}
}

func TestOutputPartToSessionUpdate_AudioURLOnly(t *testing.T) {
	url := "https://example.com/audio.wav"
	part := schema.MessageOutputPart{
		Type: schema.ChatMessagePartTypeAudioURL,
		Audio: &schema.MessageOutputAudio{
			MessagePartCommon: schema.MessagePartCommon{URL: &url, MIMEType: "audio/wav"},
		},
	}
	_, err := outputPartToSessionUpdate(part)
	if err == nil {
		t.Fatal("expected error for audio with URL only")
	}
}

func TestOutputPartToSessionUpdate_AudioNoData(t *testing.T) {
	part := schema.MessageOutputPart{
		Type: schema.ChatMessagePartTypeAudioURL,
		Audio: &schema.MessageOutputAudio{
			MessagePartCommon: schema.MessagePartCommon{MIMEType: "audio/wav"},
		},
	}
	_, err := outputPartToSessionUpdate(part)
	if err == nil {
		t.Fatal("expected error for audio with no data")
	}
}

func TestOutputPartToSessionUpdate_VideoURL(t *testing.T) {
	url := "https://example.com/video.mp4"
	part := schema.MessageOutputPart{
		Type: schema.ChatMessagePartTypeVideoURL,
		Video: &schema.MessageOutputVideo{
			MessagePartCommon: schema.MessagePartCommon{URL: &url, MIMEType: "video/mp4"},
		},
	}
	su, err := outputPartToSessionUpdate(part)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	chunk, ok := su.AsAgentMessageChunk()
	if !ok {
		t.Fatal("expected AgentMessageChunk")
	}
	rl, ok := chunk.Content.AsResourceLink()
	if !ok {
		t.Fatal("expected resource link content block")
	}
	if rl.URI != url {
		t.Fatalf("URI = %q, want %q", rl.URI, url)
	}
}

func TestOutputPartToSessionUpdate_VideoNil(t *testing.T) {
	part := schema.MessageOutputPart{Type: schema.ChatMessagePartTypeVideoURL}
	_, err := outputPartToSessionUpdate(part)
	if err == nil {
		t.Fatal("expected error for nil video")
	}
}

func TestOutputPartToSessionUpdate_VideoNoData(t *testing.T) {
	part := schema.MessageOutputPart{
		Type: schema.ChatMessagePartTypeVideoURL,
		Video: &schema.MessageOutputVideo{
			MessagePartCommon: schema.MessagePartCommon{MIMEType: "video/mp4"},
		},
	}
	_, err := outputPartToSessionUpdate(part)
	if err == nil {
		t.Fatal("expected error for video with no URL or base64")
	}
}

func TestOutputPartToSessionUpdate_UnsupportedType(t *testing.T) {
	part := schema.MessageOutputPart{Type: "unknown_type"}
	_, err := outputPartToSessionUpdate(part)
	if err == nil {
		t.Fatal("expected error for unsupported type")
	}
}

// --- AgentEventToSessionUpdate: interrupt only (default converter, no output) ---

func TestAgentEventToSessionUpdate_InterruptOnly(t *testing.T) {
	event := &adk.AgentEvent{
		Action: &adk.AgentAction{
			Interrupted: &adk.InterruptInfo{Data: "confirm deletion"},
		},
	}
	updates, errs := collectUpdates(AgentEventToSessionUpdate(event, nil))
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(updates))
	}
	chunk, ok := updates[0].AsAgentMessageChunk()
	if !ok {
		t.Fatal("expected AgentMessageChunk")
	}
	tb, _ := chunk.Content.AsText()
	if tb.Text != "confirm deletion" {
		t.Fatalf("text = %q, want %q", tb.Text, "confirm deletion")
	}
	if chunk.Meta["eino:interrupted"] != true {
		t.Fatal("expected eino:interrupted = true")
	}
}

// --- AgentEventToSessionUpdate: action without interrupt (no output) ---

func TestAgentEventToSessionUpdate_ActionWithoutInterrupt(t *testing.T) {
	event := &adk.AgentEvent{
		Action: &adk.AgentAction{Exit: true},
	}
	updates, errs := collectUpdates(AgentEventToSessionUpdate(event, nil))
	if len(updates) != 0 || len(errs) != 0 {
		t.Fatalf("expected no updates/errors for non-interrupt action, got %d updates, %d errors", len(updates), len(errs))
	}
}

// --- AgentEventToSessionUpdate: nil opt with interrupt uses default ---

func TestAgentEventToSessionUpdate_NilOptUsesDefaultConverter(t *testing.T) {
	event := &adk.AgentEvent{
		Action: &adk.AgentAction{
			Interrupted: &adk.InterruptInfo{Data: 42},
		},
	}
	updates, errs := collectUpdates(AgentEventToSessionUpdate(event, nil))
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(updates))
	}
	chunk, ok := updates[0].AsAgentMessageChunk()
	if !ok {
		t.Fatal("expected AgentMessageChunk")
	}
	tb, _ := chunk.Content.AsText()
	if tb.Text != "42" {
		t.Fatalf("text = %q, want %q", tb.Text, "42")
	}
}

// --- AgentEventToSessionUpdate: opt with nil InterruptConverter uses default ---

func TestAgentEventToSessionUpdate_OptNilConverterUsesDefault(t *testing.T) {
	event := &adk.AgentEvent{
		Action: &adk.AgentAction{
			Interrupted: &adk.InterruptInfo{Data: "hello"},
		},
	}
	opt := &EventConverterOption{InterruptConverter: nil}
	updates, errs := collectUpdates(AgentEventToSessionUpdate(event, opt))
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(updates))
	}
	requireTextChunk(t, updates[0], "hello")
}

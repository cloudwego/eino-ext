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
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/cloudwego/eino/schema"
)

func TestToMessageStreamingChunk(t *testing.T) {
	t.Run("message start event", func(t *testing.T) {
		c := newStreamConverter()
		event := mustMessageStreamEvent(t, `{
			"type":"message_start",
			"message":{
				"id":"msg_1",
				"type":"message",
				"role":"assistant",
				"model":"claude-sonnet-4-20250514",
				"content":[{"type":"text","text":"hello"}],
				"usage":{"input_tokens":3,"output_tokens":5}
			}
		}`)

		chunk, err := c.toMessageStreamingChunk(event)
		if err != nil {
			t.Fatalf("toMessageStreamingChunk() error = %v", err)
		}
		if chunk == nil || len(chunk.ContentBlocks) != 1 {
			t.Fatalf("chunk = %#v", chunk)
		}
		if chunk.ContentBlocks[0].AssistantGenText == nil || chunk.ContentBlocks[0].AssistantGenText.Text != "hello" {
			t.Fatalf("content block = %#v", chunk.ContentBlocks[0])
		}
	})

	t.Run("content block start thinking event", func(t *testing.T) {
		c := newStreamConverter()
		event := mustMessageStreamEvent(t, `{
			"type":"content_block_start",
			"index":2,
			"content_block":{"type":"thinking","thinking":"step 1","signature":"sig_1"}
		}`)

		chunk, err := c.toMessageStreamingChunk(event)
		if err != nil {
			t.Fatalf("toMessageStreamingChunk() error = %v", err)
		}
		if chunk == nil || len(chunk.ContentBlocks) != 1 {
			t.Fatalf("chunk = %#v", chunk)
		}
		if chunk.ContentBlocks[0].Reasoning == nil || chunk.ContentBlocks[0].Reasoning.Text != "step 1" {
			t.Fatalf("content block = %#v", chunk.ContentBlocks[0])
		}
		if c.blockKinds[2] != schema.ContentBlockTypeReasoning {
			t.Fatalf("blockKinds[2] = %q", c.blockKinds[2])
		}
	})

	t.Run("content block delta text event", func(t *testing.T) {
		c := newStreamConverter()
		event := mustMessageStreamEvent(t, `{
			"type":"content_block_delta",
			"index":1,
			"delta":{"type":"text_delta","text":"hello delta"}
		}`)

		chunk, err := c.toMessageStreamingChunk(event)
		if err != nil {
			t.Fatalf("toMessageStreamingChunk() error = %v", err)
		}
		if chunk == nil || len(chunk.ContentBlocks) != 1 {
			t.Fatalf("chunk = %#v", chunk)
		}
		if chunk.ContentBlocks[0].AssistantGenText == nil || chunk.ContentBlocks[0].AssistantGenText.Text != "hello delta" {
			t.Fatalf("content block = %#v", chunk.ContentBlocks[0])
		}
	})

	t.Run("message delta event", func(t *testing.T) {
		c := newStreamConverter()
		event := mustMessageStreamEvent(t, `{
			"type":"message_delta",
			"usage":{"input_tokens":3,"cache_read_input_tokens":1,"output_tokens":5},
			"delta":{"stop_reason":"end_turn","stop_sequence":"done"}
		}`)

		chunk, err := c.toMessageStreamingChunk(event)
		if err != nil {
			t.Fatalf("toMessageStreamingChunk() error = %v", err)
		}
		if chunk == nil || chunk.ResponseMeta == nil || chunk.ResponseMeta.TokenUsage == nil {
			t.Fatalf("chunk = %#v", chunk)
		}
		if chunk.ResponseMeta.TokenUsage.TotalTokens != 9 {
			t.Fatalf("token usage = %#v", chunk.ResponseMeta.TokenUsage)
		}
	})
}

func TestStreamingFunctionToolCallStartBlock(t *testing.T) {
	c := newStreamConverter()

	block := c.toStreamingFunctionToolCallStartBlock(2, anthropic.ToolUseBlock{
		ID:    "call_1",
		Name:  "get_weather",
		Input: json.RawMessage(`{"location":"beijing"}`),
	})
	if block == nil || block.FunctionToolCall == nil {
		t.Fatalf("toStreamingFunctionToolCallStartBlock() = %#v", block)
	}
	if block.FunctionToolCall.Arguments != "" {
		t.Fatalf("function tool arguments = %q", block.FunctionToolCall.Arguments)
	}
	if c.blockKinds[2] != schema.ContentBlockTypeFunctionToolCall {
		t.Fatalf("blockKinds[2] = %q", c.blockKinds[2])
	}
}

func TestStreamingServerToolCallStartBlock(t *testing.T) {
	c := newStreamConverter()

	block, err := c.toStreamingServerToolCallStartBlock(3, anthropic.ServerToolUseBlock{
		ID:    "call_1",
		Name:  anthropic.ServerToolUseBlockNameWebFetch,
		Input: map[string]any{"url": "https://example.com"},
	})
	if err != nil {
		t.Fatalf("toStreamingServerToolCallStartBlock() error = %v", err)
	}
	if block == nil || block.ServerToolCall == nil {
		t.Fatalf("toStreamingServerToolCallStartBlock() = %#v", block)
	}
	if block.StreamingMeta == nil || block.StreamingMeta.Index != 3 {
		t.Fatalf("streaming meta = %#v", block.StreamingMeta)
	}
	if c.blockKinds[3] != schema.ContentBlockTypeServerToolCall {
		t.Fatalf("blockKinds[3] = %q", c.blockKinds[3])
	}
	if c.serverToolNames[3] != ServerToolNameWebFetch {
		t.Fatalf("serverToolNames[3] = %q", c.serverToolNames[3])
	}
}

func TestToStreamingDeltaBlock(t *testing.T) {
	t.Run("server tool input json delta", func(t *testing.T) {
		c := newStreamConverter()
		c.blockKinds[1] = schema.ContentBlockTypeServerToolCall
		c.serverToolNames[1] = ServerToolNameToolSearchToolBm25

		block, err := c.toStreamingDeltaBlock(1, anthropic.InputJSONDelta{
			PartialJSON: `{"query":"find tools"}`,
		})
		if err != nil {
			t.Fatalf("toStreamingDeltaBlock() error = %v", err)
		}
		if block == nil || block.ServerToolCall == nil {
			t.Fatalf("toStreamingDeltaBlock() = %#v", block)
		}
		args, ok := block.ServerToolCall.Arguments.(*ServerToolCallArguments)
		if !ok {
			t.Fatalf("server tool arguments type = %T", block.ServerToolCall.Arguments)
		}
		if args.ToolSearchToolBm25 == nil || args.ToolSearchToolBm25.Query != "find tools" {
			t.Fatalf("tool search bm25 args = %#v", args.ToolSearchToolBm25)
		}
	})

	t.Run("function tool input json delta", func(t *testing.T) {
		c := newStreamConverter()
		c.blockKinds[2] = schema.ContentBlockTypeFunctionToolCall

		block, err := c.toStreamingDeltaBlock(2, anthropic.InputJSONDelta{
			PartialJSON: `{"name":"value"}`,
		})
		if err != nil {
			t.Fatalf("toStreamingDeltaBlock() error = %v", err)
		}
		if block == nil || block.FunctionToolCall == nil {
			t.Fatalf("toStreamingDeltaBlock() = %#v", block)
		}
		if block.FunctionToolCall.Arguments != `{"name":"value"}` {
			t.Fatalf("function tool arguments = %q", block.FunctionToolCall.Arguments)
		}
	})

	t.Run("invalid server tool name", func(t *testing.T) {
		c := newStreamConverter()
		c.blockKinds[4] = schema.ContentBlockTypeServerToolCall

		_, err := c.toStreamingDeltaBlock(4, anthropic.InputJSONDelta{
			PartialJSON: `{"query":"value"}`,
		})
		if err == nil || !strings.Contains(err.Error(), `invalid server tool name ""`) {
			t.Fatalf("toStreamingDeltaBlock() error = %v", err)
		}
	})
}

func TestStreamingServerToolCallDeltaBlock(t *testing.T) {
	t.Run("tool search regex", func(t *testing.T) {
		block, err := toStreamingToolSearchToolRegexCallDeltaBlock(`{"query":"find.*"}`, &schema.StreamingMeta{Index: 5})
		if err != nil {
			t.Fatalf("toStreamingToolSearchToolRegexCallDeltaBlock() error = %v", err)
		}
		if block == nil || block.ServerToolCall == nil {
			t.Fatalf("toStreamingToolSearchToolRegexCallDeltaBlock() = %#v", block)
		}
		args, ok := block.ServerToolCall.Arguments.(*ServerToolCallArguments)
		if !ok {
			t.Fatalf("server tool arguments type = %T", block.ServerToolCall.Arguments)
		}
		if args.ToolSearchToolRegex == nil || args.ToolSearchToolRegex.Query != "find.*" {
			t.Fatalf("tool search regex args = %#v", args.ToolSearchToolRegex)
		}
	})

	t.Run("invalid json returns stable error", func(t *testing.T) {
		_, err := toStreamingCodeExecutionToolCallDeltaBlock(`{"code":`, &schema.StreamingMeta{Index: 6})
		if err == nil || !strings.Contains(err.Error(), "failed to decode code_execution server tool arguments") {
			t.Fatalf("toStreamingCodeExecutionToolCallDeltaBlock() error = %v", err)
		}
	})
}

func TestAdditionalStreamingHelpers(t *testing.T) {
	t.Run("text start block", func(t *testing.T) {
		c := newStreamConverter()
		block := c.toStreamingTextStartBlock(0, anthropic.TextBlock{Text: "hello"})
		if block == nil || block.AssistantGenText == nil || block.AssistantGenText.Text != "hello" {
			t.Fatalf("toStreamingTextStartBlock() = %#v", block)
		}
		if c.blockKinds[0] != schema.ContentBlockTypeAssistantGenText {
			t.Fatalf("blockKinds[0] = %q", c.blockKinds[0])
		}
	})

	t.Run("web search and web fetch delta blocks", func(t *testing.T) {
		webSearchBlock, err := toStreamingWebSearchToolCallDeltaBlock(`{"query":"golang"}`, &schema.StreamingMeta{Index: 1})
		if err != nil {
			t.Fatalf("toStreamingWebSearchToolCallDeltaBlock() error = %v", err)
		}
		webFetchBlock, err := toStreamingWebFetchToolCallDeltaBlock(`{"url":"https://example.com"}`, &schema.StreamingMeta{Index: 2})
		if err != nil {
			t.Fatalf("toStreamingWebFetchToolCallDeltaBlock() error = %v", err)
		}
		if webSearchBlock.ServerToolCall == nil || webFetchBlock.ServerToolCall == nil {
			t.Fatalf("delta blocks = (%#v, %#v)", webSearchBlock, webFetchBlock)
		}
	})

	t.Run("bash and text editor delta blocks", func(t *testing.T) {
		bashBlock, err := toStreamingBashCodeExecutionToolCallDeltaBlock(`{"command":"ls"}`, &schema.StreamingMeta{Index: 3})
		if err != nil {
			t.Fatalf("toStreamingBashCodeExecutionToolCallDeltaBlock() error = %v", err)
		}
		textEditorBlock, err := toStreamingTextEditorCodeExecutionToolCallDeltaBlock(`{"command":"view","path":"/tmp/a.txt"}`, &schema.StreamingMeta{Index: 4})
		if err != nil {
			t.Fatalf("toStreamingTextEditorCodeExecutionToolCallDeltaBlock() error = %v", err)
		}
		if bashBlock.ServerToolCall == nil || textEditorBlock.ServerToolCall == nil {
			t.Fatalf("delta blocks = (%#v, %#v)", bashBlock, textEditorBlock)
		}
	})
}

func mustMessageStreamEvent(t *testing.T, raw string) anthropic.MessageStreamEventUnion {
	t.Helper()

	var event anthropic.MessageStreamEventUnion
	if err := json.Unmarshal([]byte(raw), &event); err != nil {
		t.Fatalf("json.Unmarshal(event) error = %v", err)
	}
	return event
}

func TestStreamingServerToolResultStartBlocks(t *testing.T) {
	t.Run("web search result start", func(t *testing.T) {
		c := newStreamConverter()
		var block anthropic.WebSearchToolResultBlock
		if err := json.Unmarshal([]byte(`{
			"type":"web_search_tool_result",
			"tool_use_id":"call_1",
			"content":[{"type":"web_search_result","title":"Go","url":"https://go.dev","encrypted_content":"e","page_age":"1d"}]
		}`), &block); err != nil {
			t.Fatalf("unmarshal error = %v", err)
		}

		cb, err := c.toStreamingWebSearchToolResultStartBlock(0, block)
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if cb == nil || cb.ServerToolResult == nil {
			t.Fatalf("content block = %#v", cb)
		}
		if c.blockKinds[0] != schema.ContentBlockTypeServerToolResult {
			t.Fatalf("blockKinds[0] = %q", c.blockKinds[0])
		}
	})

	t.Run("web fetch result start", func(t *testing.T) {
		c := newStreamConverter()
		var block anthropic.WebFetchToolResultBlock
		if err := json.Unmarshal([]byte(`{
			"type":"web_fetch_tool_result",
			"tool_use_id":"call_2",
			"content":{"type":"web_fetch_tool_result_error","error_code":"fetch_failed"}
		}`), &block); err != nil {
			t.Fatalf("unmarshal error = %v", err)
		}

		cb, err := c.toStreamingWebFetchToolResultStartBlock(1, block)
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if cb == nil || cb.ServerToolResult == nil {
			t.Fatalf("content block = %#v", cb)
		}
		if c.blockKinds[1] != schema.ContentBlockTypeServerToolResult {
			t.Fatalf("blockKinds[1] = %q", c.blockKinds[1])
		}
	})

	t.Run("code execution result start", func(t *testing.T) {
		c := newStreamConverter()
		var block anthropic.CodeExecutionToolResultBlock
		if err := json.Unmarshal([]byte(`{
			"type":"code_execution_tool_result",
			"tool_use_id":"call_3",
			"content":{"type":"code_execution_result","stdout":"hi","stderr":"","return_code":0,"content":[]}
		}`), &block); err != nil {
			t.Fatalf("unmarshal error = %v", err)
		}

		cb, err := c.toStreamingCodeExecutionToolResultStartBlock(2, block)
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if cb == nil || cb.ServerToolResult == nil {
			t.Fatalf("content block = %#v", cb)
		}
		if c.blockKinds[2] != schema.ContentBlockTypeServerToolResult {
			t.Fatalf("blockKinds[2] = %q", c.blockKinds[2])
		}
	})

	t.Run("bash code execution result start", func(t *testing.T) {
		c := newStreamConverter()
		var block anthropic.BashCodeExecutionToolResultBlock
		if err := json.Unmarshal([]byte(`{
			"type":"bash_code_execution_tool_result",
			"tool_use_id":"call_4",
			"content":{"type":"bash_code_execution_result","stdout":"ok","stderr":"","return_code":0,"content":[]}
		}`), &block); err != nil {
			t.Fatalf("unmarshal error = %v", err)
		}

		cb, err := c.toStreamingBashCodeExecutionToolResultStartBlock(3, block)
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if cb == nil || cb.ServerToolResult == nil {
			t.Fatalf("content block = %#v", cb)
		}
		if c.blockKinds[3] != schema.ContentBlockTypeServerToolResult {
			t.Fatalf("blockKinds[3] = %q", c.blockKinds[3])
		}
	})

	t.Run("text editor result start", func(t *testing.T) {
		c := newStreamConverter()
		var block anthropic.TextEditorCodeExecutionToolResultBlock
		if err := json.Unmarshal([]byte(`{
			"type":"text_editor_code_execution_tool_result",
			"tool_use_id":"call_5",
			"content":{"type":"text_editor_code_execution_view_result","file_type":"go","content":"pkg","num_lines":1,"start_line":1,"total_lines":1}
		}`), &block); err != nil {
			t.Fatalf("unmarshal error = %v", err)
		}

		cb, err := c.toStreamingTextEditorCodeExecutionToolResultStartBlock(4, block)
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if cb == nil || cb.ServerToolResult == nil {
			t.Fatalf("content block = %#v", cb)
		}
		if c.blockKinds[4] != schema.ContentBlockTypeServerToolResult {
			t.Fatalf("blockKinds[4] = %q", c.blockKinds[4])
		}
	})
}

func TestStreamingCitationsDelta(t *testing.T) {
	c := newStreamConverter()
	event := mustMessageStreamEvent(t, `{
		"type":"content_block_delta",
		"index":0,
		"delta":{"type":"citations_delta","citation":{"type":"char_location","cited_text":"hello","document_title":"doc","document_index":0,"start_char_index":0,"end_char_index":5}}
	}`)

	chunk, err := c.toMessageStreamingChunk(event)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if chunk == nil || len(chunk.ContentBlocks) != 1 {
		t.Fatalf("chunk = %#v", chunk)
	}
	if chunk.ContentBlocks[0].AssistantGenText == nil || chunk.ContentBlocks[0].AssistantGenText.ClaudeExtension == nil {
		t.Fatalf("content block = %#v", chunk.ContentBlocks[0])
	}
}

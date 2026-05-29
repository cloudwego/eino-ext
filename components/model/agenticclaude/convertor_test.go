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
	claudeschema "github.com/cloudwego/eino/schema/claude"
)

func TestToolSearchResultToBlockParam(t *testing.T) {
	blockParam, err := toolSearchResultToBlockParam(&schema.ToolSearchFunctionToolResult{
		CallID: "call_1",
		Result: &schema.ToolSearchResult{
			Tools: []*schema.ToolInfo{
				{Name: "tool_a"},
				{Name: "tool_b"},
			},
		},
	})
	if err != nil {
		t.Fatalf("toolSearchResultToBlockParam() error = %v", err)
	}

	got := mustJSON(t, blockParam)
	for _, want := range []string{
		`"type":"tool_result"`,
		`"tool_use_id":"call_1"`,
		`"tool_name":"tool_a","type":"tool_reference"`,
		`"tool_name":"tool_b","type":"tool_reference"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("toolSearchResultToBlockParam() json = %s, want substring %s", got, want)
		}
	}
}

func TestServerToolResultToBlockParam(t *testing.T) {
	t.Run("tool search search result", func(t *testing.T) {
		blockParam, err := serverToolResultToBlockParam(schema.NewContentBlock(&schema.ServerToolResult{
			CallID: "call_1",
			Name:   string(ServerToolNameToolSearchToolBm25),
			Content: &ServerToolResult{
				ToolSearchToolBm25: &ToolSearchToolResult{
					Type: ToolSearchToolResultTypeSearchResult,
					SearchResult: &ToolSearchToolSearchResult{
						ToolReferences: []*ToolSearchToolReference{
							{ToolName: "tool_a"},
						},
					},
				},
			},
		}))
		if err != nil {
			t.Fatalf("serverToolResultToBlockParam() error = %v", err)
		}

		got := mustJSON(t, blockParam)
		for _, want := range []string{
			`"type":"tool_search_tool_result"`,
			`"tool_use_id":"call_1"`,
			`"type":"tool_search_tool_search_result"`,
			`"tool_name":"tool_a"`,
		} {
			if !strings.Contains(got, want) {
				t.Fatalf("serverToolResultToBlockParam() json = %s, want substring %s", got, want)
			}
		}
	})

	t.Run("tool search error", func(t *testing.T) {
		blockParam, err := serverToolResultToBlockParam(schema.NewContentBlock(&schema.ServerToolResult{
			CallID: "call_2",
			Name:   string(ServerToolNameToolSearchToolRegex),
			Content: &ServerToolResult{
				ToolSearchToolRegex: &ToolSearchToolResult{
					Type:  ToolSearchToolResultTypeError,
					Error: &ToolSearchToolResultError{Code: "invalid_query"},
				},
			},
		}))
		if err != nil {
			t.Fatalf("serverToolResultToBlockParam() error = %v", err)
		}
		got := mustJSON(t, blockParam)
		for _, want := range []string{
			`"type":"tool_search_tool_result"`,
			`"type":"tool_search_tool_result_error"`,
			`"error_code":"invalid_query"`,
		} {
			if !strings.Contains(got, want) {
				t.Fatalf("serverToolResultToBlockParam() json = %s, want substring %s", got, want)
			}
		}
	})
}

func TestServerToolUseToContentBlock(t *testing.T) {
	t.Run("tool search bm25 input", func(t *testing.T) {
		block, err := serverToolUseToContentBlock(anthropic.ServerToolUseBlock{
			ID:    "call_1",
			Name:  anthropic.ServerToolUseBlockNameToolSearchToolBm25,
			Input: map[string]any{"query": "find tools"},
		})
		if err != nil {
			t.Fatalf("serverToolUseToContentBlock() error = %v", err)
		}
		if block.ServerToolCall == nil {
			t.Fatalf("serverToolUseToContentBlock() returned nil server tool call")
		}
		args, ok := block.ServerToolCall.Arguments.(*ServerToolCallArguments)
		if !ok {
			t.Fatalf("server tool call arguments type = %T", block.ServerToolCall.Arguments)
		}
		if args.ToolSearchToolBm25 == nil || args.ToolSearchToolBm25.Query != "find tools" {
			t.Fatalf("tool search bm25 args = %#v", args.ToolSearchToolBm25)
		}
	})

	t.Run("web fetch nil input keeps empty args", func(t *testing.T) {
		block, err := serverToolUseToContentBlock(anthropic.ServerToolUseBlock{
			ID:   "call_2",
			Name: anthropic.ServerToolUseBlockNameWebFetch,
		})
		if err != nil {
			t.Fatalf("serverToolUseToContentBlock() error = %v", err)
		}
		args, ok := block.ServerToolCall.Arguments.(*ServerToolCallArguments)
		if !ok {
			t.Fatalf("server tool call arguments type = %T", block.ServerToolCall.Arguments)
		}
		if args.WebFetch == nil || args.WebFetch.URL != "" {
			t.Fatalf("web fetch args = %#v", args.WebFetch)
		}
	})
}

func TestToDeltaResponseMeta(t *testing.T) {
	meta := toDeltaResponseMeta(anthropic.MessageDeltaEvent{
		Usage: anthropic.MessageDeltaUsage{
			InputTokens:              10,
			CacheReadInputTokens:     3,
			CacheCreationInputTokens: 2,
			OutputTokens:             7,
		},
		Delta: anthropic.MessageDeltaEventDelta{
			StopReason:   anthropic.StopReasonEndTurn,
			StopSequence: "done",
			StopDetails: anthropic.RefusalStopDetails{
				Category:    anthropic.RefusalStopDetailsCategoryBio,
				Explanation: "blocked",
			},
		},
	})

	if meta == nil || meta.TokenUsage == nil || meta.ClaudeExtension == nil {
		t.Fatalf("toDeltaResponseMeta() = %#v", meta)
	}
	if meta.TokenUsage.PromptTokens != 15 || meta.TokenUsage.CompletionTokens != 7 || meta.TokenUsage.TotalTokens != 22 {
		t.Fatalf("token usage = %#v", meta.TokenUsage)
	}
	if meta.TokenUsage.PromptTokenDetails.CachedTokens != 3 {
		t.Fatalf("cached tokens = %d, want 3", meta.TokenUsage.PromptTokenDetails.CachedTokens)
	}
	if meta.ClaudeExtension.StopReason != string(anthropic.StopReasonEndTurn) {
		t.Fatalf("stop reason = %q", meta.ClaudeExtension.StopReason)
	}
	if meta.ClaudeExtension.StopSequence != "done" {
		t.Fatalf("stop sequence = %q", meta.ClaudeExtension.StopSequence)
	}
	if meta.ClaudeExtension.StopDetails == nil || meta.ClaudeExtension.StopDetails.Explanation != "blocked" {
		t.Fatalf("stop details = %#v", meta.ClaudeExtension.StopDetails)
	}
}

func TestToAnthropicTextCitations(t *testing.T) {
	citations, err := toAnthropicTextCitations([]*claudeschema.TextCitation{
		{
			Type: claudeschema.TextCitationTypeWebSearchResultLocation,
			WebSearchResultLocation: &claudeschema.CitationWebSearchResultLocation{
				CitedText:      "snippet",
				Title:          "result title",
				URL:            "https://example.com",
				EncryptedIndex: "enc_1",
			},
		},
	})
	if err != nil {
		t.Fatalf("toAnthropicTextCitations() error = %v", err)
	}
	if len(citations) != 1 || citations[0].OfWebSearchResultLocation == nil {
		t.Fatalf("citations = %#v", citations)
	}
	got := citations[0].OfWebSearchResultLocation
	if got.URL != "https://example.com" || got.EncryptedIndex != "enc_1" {
		t.Fatalf("web search result citation = %#v", got)
	}
}

func TestToWebFetchDocumentBlockParam(t *testing.T) {
	t.Run("invalid mime type", func(t *testing.T) {
		_, err := toWebFetchDocumentBlockParam(&WebFetchDocument{
			Source: &WebFetchDocumentSource{
				MIMEType: "application/json",
				Data:     "{}",
			},
		})
		if err == nil || !strings.Contains(err.Error(), `invalid web fetch content mime type "application/json"`) {
			t.Fatalf("toWebFetchDocumentBlockParam() error = %v", err)
		}
	})

	t.Run("plain text document", func(t *testing.T) {
		blockParam, err := toWebFetchDocumentBlockParam(&WebFetchDocument{
			Title: "doc",
			Citations: &WebFetchDocumentCitations{
				Enabled: true,
			},
			Source: &WebFetchDocumentSource{
				MIMEType: "text/plain",
				Data:     "hello",
			},
		})
		if err != nil {
			t.Fatalf("toWebFetchDocumentBlockParam() error = %v", err)
		}
		got := mustJSON(t, blockParam)
		for _, want := range []string{
			`"title":"doc"`,
			`"type":"text"`,
			`"data":"hello"`,
			`"enabled":true`,
		} {
			if !strings.Contains(got, want) {
				t.Fatalf("toWebFetchDocumentBlockParam() json = %s, want substring %s", got, want)
			}
		}
	})
}

func TestToAnthropicMessages(t *testing.T) {
	input := []*schema.AgenticMessage{
		schema.SystemAgenticMessage("follow system"),
		{
			Role: schema.AgenticRoleTypeUser,
			ContentBlocks: []*schema.ContentBlock{
				schema.NewContentBlock(&schema.UserInputText{Text: "hello"}),
				schema.NewContentBlock(&schema.UserInputImage{URL: "https://example.com/image.png"}),
				schema.NewContentBlock(&schema.UserInputFile{URL: "https://example.com/doc.pdf"}),
				schema.NewContentBlock(&schema.FunctionToolResult{
					CallID: "call_fn",
					Name:   "get_weather",
					Content: []*schema.FunctionToolResultContentBlock{
						{
							Type: schema.FunctionToolResultContentBlockTypeText,
							Text: &schema.UserInputText{Text: "sunny"},
						},
					},
				}),
				schema.NewContentBlock(&schema.ToolSearchFunctionToolResult{
					CallID: "call_search",
					Result: &schema.ToolSearchResult{
						Tools: []*schema.ToolInfo{{Name: "tool_a"}},
					},
				}),
			},
		},
		{
			Role: schema.AgenticRoleTypeAssistant,
			ContentBlocks: []*schema.ContentBlock{
				schema.NewContentBlock(&schema.AssistantGenText{Text: "answer"}),
				schema.NewContentBlock(&schema.Reasoning{Text: "thinking", Signature: "sig_1"}),
				schema.NewContentBlock(&schema.FunctionToolCall{
					CallID:    "call_tool",
					Name:      "get_weather",
					Arguments: `{"city":"beijing"}`,
				}),
				schema.NewContentBlock(&schema.ServerToolCall{
					CallID: "call_server",
					Name:   string(ServerToolNameWebSearch),
					Arguments: &ServerToolCallArguments{
						WebSearch: &WebSearchArguments{Query: "golang"},
					},
				}),
			},
		},
	}

	systemBlocks, msgParams, err := toAnthropicMessages(input)
	if err != nil {
		t.Fatalf("toAnthropicMessages() error = %v", err)
	}
	if len(systemBlocks) != 1 || systemBlocks[0].Text != "follow system" {
		t.Fatalf("systemBlocks = %#v", systemBlocks)
	}
	if len(msgParams) != 2 {
		t.Fatalf("len(msgParams) = %d, want 2", len(msgParams))
	}
	if msgParams[0].Role != anthropic.MessageParamRoleUser || msgParams[1].Role != anthropic.MessageParamRoleAssistant {
		t.Fatalf("roles = [%q, %q]", msgParams[0].Role, msgParams[1].Role)
	}

	got := mustJSON(t, msgParams)
	for _, want := range []string{
		`"text":"hello"`,
		`"type":"image"`,
		`"type":"document"`,
		`"tool_use_id":"call_fn"`,
		`"tool_use_id":"call_search"`,
		`"tool_name":"tool_a"`,
		`"thinking":"thinking"`,
		`"signature":"sig_1"`,
		`"type":"tool_use"`,
		`"name":"web_search"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("toAnthropicMessages() json = %s, want substring %s", got, want)
		}
	}
}

func TestToAnthropicMessagesErrors(t *testing.T) {
	t.Run("empty input", func(t *testing.T) {
		_, _, err := toAnthropicMessages(nil)
		if err == nil || err.Error() != "input is empty" {
			t.Fatalf("toAnthropicMessages() error = %v", err)
		}
	})

	t.Run("system after user", func(t *testing.T) {
		_, _, err := toAnthropicMessages([]*schema.AgenticMessage{
			schema.UserAgenticMessage("hello"),
			schema.SystemAgenticMessage("late system"),
		})
		if err == nil || err.Error() != "system message must appear before all non-system messages" {
			t.Fatalf("toAnthropicMessages() error = %v", err)
		}
	})
}

func TestToAgenticMessage(t *testing.T) {
	resp := &anthropic.Message{}
	if err := json.Unmarshal([]byte(`{
		"id":"msg_1",
		"type":"message",
		"role":"assistant",
		"model":"claude-sonnet-4-20250514",
		"content":[
			{"type":"text","text":"hello"},
			{"type":"thinking","thinking":"reasoning","signature":"sig_1"},
			{"type":"tool_use","id":"call_1","name":"get_weather","input":{"city":"beijing"}},
			{"type":"redacted_thinking","data":"secret"}
		],
		"stop_reason":"end_turn",
		"usage":{"input_tokens":3,"output_tokens":5}
	}`), resp); err != nil {
		t.Fatalf("json.Unmarshal(message) error = %v", err)
	}

	msg, err := toAgenticMessage(resp)
	if err != nil {
		t.Fatalf("toAgenticMessage() error = %v", err)
	}
	if msg.Role != schema.AgenticRoleTypeAssistant {
		t.Fatalf("role = %q, want assistant", msg.Role)
	}
	if len(msg.ContentBlocks) != 3 {
		t.Fatalf("len(msg.ContentBlocks) = %d, want 3", len(msg.ContentBlocks))
	}
	if msg.ContentBlocks[0].AssistantGenText == nil || msg.ContentBlocks[0].AssistantGenText.Text != "hello" {
		t.Fatalf("text block = %#v", msg.ContentBlocks[0])
	}
	if msg.ContentBlocks[1].Reasoning == nil || msg.ContentBlocks[1].Reasoning.Signature != "sig_1" {
		t.Fatalf("reasoning block = %#v", msg.ContentBlocks[1])
	}
	if msg.ContentBlocks[2].FunctionToolCall == nil || msg.ContentBlocks[2].FunctionToolCall.Arguments != `{"city":"beijing"}` {
		t.Fatalf("tool call block = %#v", msg.ContentBlocks[2])
	}
	if msg.ResponseMeta == nil || msg.ResponseMeta.ClaudeExtension == nil || msg.ResponseMeta.ClaudeExtension.ID != "msg_1" {
		t.Fatalf("response meta = %#v", msg.ResponseMeta)
	}
}

func TestToAgenticContentBlock(t *testing.T) {
	t.Run("server tool use", func(t *testing.T) {
		block, err := toAgenticContentBlock(anthropic.ServerToolUseBlock{
			ID:    "call_1",
			Name:  anthropic.ServerToolUseBlockNameWebFetch,
			Input: map[string]any{"url": "https://example.com"},
		})
		if err != nil {
			t.Fatalf("toAgenticContentBlock() error = %v", err)
		}
		if block == nil || block.ServerToolCall == nil || block.ServerToolCall.Name != string(ServerToolNameWebFetch) {
			t.Fatalf("block = %#v", block)
		}
	})

	t.Run("redacted thinking dropped", func(t *testing.T) {
		block, err := toAgenticContentBlock(anthropic.RedactedThinkingBlock{})
		if err != nil {
			t.Fatalf("toAgenticContentBlock() error = %v", err)
		}
		if block != nil {
			t.Fatalf("block = %#v, want nil", block)
		}
	})

	t.Run("invalid type", func(t *testing.T) {
		_, err := toAgenticContentBlock(123)
		if err == nil || err.Error() != "invalid output block type int" {
			t.Fatalf("toAgenticContentBlock() error = %v", err)
		}
	})
}

func TestToolMediaAndResultConversionHelpers(t *testing.T) {
	t.Run("image and file block params", func(t *testing.T) {
		imageParam, err := imageToBlockParam(&schema.UserInputImage{
			Base64Data: "aGVsbG8=",
			MIMEType:   "image/png",
		})
		if err != nil {
			t.Fatalf("imageToBlockParam() error = %v", err)
		}
		fileParam, err := documentToBlockParam(&schema.UserInputFile{
			Base64Data: "aGVsbG8=",
		})
		if err != nil {
			t.Fatalf("documentToBlockParam() error = %v", err)
		}
		got := mustJSON(t, []anthropic.ContentBlockParamUnion{imageParam, fileParam})
		for _, want := range []string{
			`"media_type":"image/png"`,
			`"type":"image"`,
			`"media_type":"application/pdf"`,
			`"type":"document"`,
		} {
			if !strings.Contains(got, want) {
				t.Fatalf("conversion json = %s, want substring %s", got, want)
			}
		}
	})

	t.Run("function tool result content image and file", func(t *testing.T) {
		imageBlock, err := functionToolResultContentToBlockParam(&schema.FunctionToolResultContentBlock{
			Type: schema.FunctionToolResultContentBlockTypeImage,
			Image: &schema.UserInputImage{
				URL: "https://example.com/image.png",
			},
		})
		if err != nil {
			t.Fatalf("functionToolResultContentToBlockParam(image) error = %v", err)
		}
		fileBlock, err := functionToolResultContentToBlockParam(&schema.FunctionToolResultContentBlock{
			Type: schema.FunctionToolResultContentBlockTypeFile,
			File: &schema.UserInputFile{
				URL: "https://example.com/doc.pdf",
			},
		})
		if err != nil {
			t.Fatalf("functionToolResultContentToBlockParam(file) error = %v", err)
		}
		got := mustJSON(t, []anthropic.ToolResultBlockParamContentUnion{imageBlock, fileBlock})
		for _, want := range []string{
			`"type":"image"`,
			`"type":"document"`,
		} {
			if !strings.Contains(got, want) {
				t.Fatalf("conversion json = %s, want substring %s", got, want)
			}
		}
	})

	t.Run("web search and web fetch result block params", func(t *testing.T) {
		searchBlock := &schema.ContentBlock{}
		setWebSearchResultCaller(searchBlock, mustWebSearchCaller(t))
		searchParam, err := webSearchToolResultToBlockParam(&WebSearchResult{
			Type: WebSearchResultTypeResult,
			Result: &WebSearchResultBlock{
				Content: []*WebSearchResultItem{
					{
						Title:            "doc",
						URL:              "https://example.com",
						EncryptedContent: "enc_1",
						PageAge:          "1 day",
					},
				},
			},
		}, "call_search", searchBlock)
		if err != nil {
			t.Fatalf("webSearchToolResultToBlockParam() error = %v", err)
		}

		fetchBlock := &schema.ContentBlock{}
		setWebFetchResultCaller(fetchBlock, mustWebFetchCaller(t))
		fetchParam, err := webFetchToolResultToBlockParam(&WebFetchResult{
			Type: WebFetchResultTypeResult,
			Result: &WebFetchResultBlock{
				URL:         "https://example.com",
				RetrievedAt: "2026-05-19T00:00:00Z",
				Content: &WebFetchDocument{
					Title: "doc",
					Source: &WebFetchDocumentSource{
						MIMEType: "text/plain",
						Data:     "hello",
					},
				},
			},
		}, "call_fetch", fetchBlock)
		if err != nil {
			t.Fatalf("webFetchToolResultToBlockParam() error = %v", err)
		}

		got := mustJSON(t, []anthropic.ContentBlockParamUnion{searchParam, fetchParam})
		for _, want := range []string{
			`"type":"web_search_tool_result"`,
			`"encrypted_content":"enc_1"`,
			`"type":"web_fetch_tool_result"`,
			`"retrieved_at":"2026-05-19T00:00:00Z"`,
			`"caller":{"type":"direct"}`,
		} {
			if !strings.Contains(got, want) {
				t.Fatalf("conversion json = %s, want substring %s", got, want)
			}
		}
	})

	t.Run("execution result block params and output helpers", func(t *testing.T) {
		codeParam, err := codeExecutionToolResultToBlockParam(&CodeExecutionResult{
			Type: CodeExecutionResultTypeResult,
			Result: &CodeExecutionResultBlock{
				Content: []*CodeExecutionOutput{{FileID: "file_1"}, nil},
				Stdout:  "stdout",
			},
		}, "call_code")
		if err != nil {
			t.Fatalf("codeExecutionToolResultToBlockParam() error = %v", err)
		}
		bashParam, err := bashCodeExecutionToolResultToBlockParam(&BashCodeExecutionResult{
			Type: BashCodeExecutionResultTypeResult,
			Result: &BashCodeExecutionResultBlock{
				Content: []*CodeExecutionOutput{{FileID: "file_2"}, nil},
				Stdout:  "ok",
			},
		}, "call_bash")
		if err != nil {
			t.Fatalf("bashCodeExecutionToolResultToBlockParam() error = %v", err)
		}
		textEditorParam, err := textEditorCodeExecutionToolResultToBlockParam(&TextEditorCodeExecutionResult{
			Type: TextEditorCodeExecutionResultTypeView,
			View: &TextEditorCodeExecutionViewResult{
				FileType:   "text",
				Content:    "hello",
				NumLines:   1,
				StartLine:  1,
				TotalLines: 1,
			},
		}, "call_editor")
		if err != nil {
			t.Fatalf("textEditorCodeExecutionToolResultToBlockParam() error = %v", err)
		}

		if got := toCodeExecutionOutputs([]anthropic.CodeExecutionOutputBlock{{FileID: "file_1"}}); len(got) != 1 || got[0].FileID != "file_1" {
			t.Fatalf("toCodeExecutionOutputs() = %#v", got)
		}
		if got := toBashCodeExecutionOutputs([]anthropic.BashCodeExecutionOutputBlock{{FileID: "file_2"}}); len(got) != 1 || got[0].FileID != "file_2" {
			t.Fatalf("toBashCodeExecutionOutputs() = %#v", got)
		}

		got := mustJSON(t, []anthropic.ContentBlockParamUnion{codeParam, bashParam, textEditorParam})
		for _, want := range []string{
			`"type":"code_execution_tool_result"`,
			`"type":"bash_code_execution_tool_result"`,
			`"type":"text_editor_code_execution_tool_result"`,
			`"file_id":"file_1"`,
			`"file_id":"file_2"`,
		} {
			if !strings.Contains(got, want) {
				t.Fatalf("conversion json = %s, want substring %s", got, want)
			}
		}
	})

	t.Run("server tool use helpers", func(t *testing.T) {
		cases := []anthropic.ServerToolUseBlock{
			{ID: "call_search", Name: anthropic.ServerToolUseBlockNameWebSearch, Input: map[string]any{"query": "golang"}},
			{ID: "call_code", Name: anthropic.ServerToolUseBlockNameCodeExecution, Input: map[string]any{"code": "print(1)"}},
			{ID: "call_bash", Name: anthropic.ServerToolUseBlockNameBashCodeExecution, Input: map[string]any{"command": "ls"}},
			{ID: "call_editor", Name: anthropic.ServerToolUseBlockNameTextEditorCodeExecution, Input: map[string]any{"command": "view", "path": "/tmp/a.txt"}},
			{ID: "call_regex", Name: anthropic.ServerToolUseBlockNameToolSearchToolRegex, Input: map[string]any{"query": "find.*"}},
		}

		for _, tc := range cases {
			block, err := serverToolUseToContentBlock(tc)
			if err != nil {
				t.Fatalf("serverToolUseToContentBlock(%q) error = %v", tc.Name, err)
			}
			if block == nil || block.ServerToolCall == nil || block.ServerToolCall.Name == "" {
				t.Fatalf("server tool block = %#v", block)
			}
		}
	})
}

func mustWebSearchCaller(t *testing.T) anthropic.WebSearchToolResultBlockCallerUnion {
	t.Helper()

	var caller anthropic.WebSearchToolResultBlockCallerUnion
	if err := json.Unmarshal([]byte(`{"type":"direct"}`), &caller); err != nil {
		t.Fatalf("json.Unmarshal(web search caller) error = %v", err)
	}
	return caller
}

func mustWebFetchCaller(t *testing.T) anthropic.WebFetchToolResultBlockCallerUnion {
	t.Helper()

	var caller anthropic.WebFetchToolResultBlockCallerUnion
	if err := json.Unmarshal([]byte(`{"type":"direct"}`), &caller); err != nil {
		t.Fatalf("json.Unmarshal(web fetch caller) error = %v", err)
	}
	return caller
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()

	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal(%T) error = %v", v, err)
	}
	return string(data)
}

func TestWebSearchToolResultToContentBlock(t *testing.T) {
	t.Run("result type", func(t *testing.T) {
		var block anthropic.WebSearchToolResultBlock
		if err := json.Unmarshal([]byte(`{
			"type":"web_search_tool_result",
			"tool_use_id":"call_1",
			"content":[
				{"type":"web_search_result","title":"Go","url":"https://go.dev","encrypted_content":"enc","page_age":"2 days"}
			]
		}`), &block); err != nil {
			t.Fatalf("unmarshal error = %v", err)
		}

		cb, err := webSearchToolResultToContentBlock(block)
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		result := cb.ServerToolResult.Content.(*ServerToolResult)
		if result.WebSearch == nil || result.WebSearch.Type != WebSearchResultTypeResult {
			t.Fatalf("web search result = %#v", result.WebSearch)
		}
		if len(result.WebSearch.Result.Content) != 1 || result.WebSearch.Result.Content[0].Title != "Go" {
			t.Fatalf("web search items = %#v", result.WebSearch.Result.Content)
		}
	})

	t.Run("error type", func(t *testing.T) {
		var block anthropic.WebSearchToolResultBlock
		if err := json.Unmarshal([]byte(`{
			"type":"web_search_tool_result",
			"tool_use_id":"call_2",
			"content":{"type":"web_search_tool_result_error","error_code":"rate_limited"}
		}`), &block); err != nil {
			t.Fatalf("unmarshal error = %v", err)
		}

		cb, err := webSearchToolResultToContentBlock(block)
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		result := cb.ServerToolResult.Content.(*ServerToolResult)
		if result.WebSearch == nil || result.WebSearch.Type != WebSearchResultTypeError {
			t.Fatalf("web search result = %#v", result.WebSearch)
		}
	})

	t.Run("invalid type", func(t *testing.T) {
		var block anthropic.WebSearchToolResultBlock
		_ = json.Unmarshal([]byte(`{
			"type":"web_search_tool_result",
			"tool_use_id":"call_3",
			"content":{"type":"unknown_type"}
		}`), &block)

		_, err := webSearchToolResultToContentBlock(block)
		if err == nil || !strings.Contains(err.Error(), "invalid web search result type") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestWebFetchToolResultToContentBlock(t *testing.T) {
	t.Run("result type", func(t *testing.T) {
		var block anthropic.WebFetchToolResultBlock
		if err := json.Unmarshal([]byte(`{
			"type":"web_fetch_tool_result",
			"tool_use_id":"call_1",
			"content":{
				"type":"web_fetch_result",
				"url":"https://example.com",
				"retrieved_at":"2026-05-19T00:00:00Z",
				"content":{
					"type":"document",
					"title":"doc",
					"source":{"type":"text","media_type":"text/plain","data":"hello"},
					"citations":{"enabled":true}
				}
			}
		}`), &block); err != nil {
			t.Fatalf("unmarshal error = %v", err)
		}

		cb, err := webFetchToolResultToContentBlock(block)
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		result := cb.ServerToolResult.Content.(*ServerToolResult)
		if result.WebFetch == nil || result.WebFetch.Type != WebFetchResultTypeResult {
			t.Fatalf("web fetch result = %#v", result.WebFetch)
		}
		if result.WebFetch.Result.URL != "https://example.com" {
			t.Fatalf("url = %q", result.WebFetch.Result.URL)
		}
	})

	t.Run("error type", func(t *testing.T) {
		var block anthropic.WebFetchToolResultBlock
		if err := json.Unmarshal([]byte(`{
			"type":"web_fetch_tool_result",
			"tool_use_id":"call_2",
			"content":{"type":"web_fetch_tool_result_error","error_code":"fetch_failed"}
		}`), &block); err != nil {
			t.Fatalf("unmarshal error = %v", err)
		}

		cb, err := webFetchToolResultToContentBlock(block)
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		result := cb.ServerToolResult.Content.(*ServerToolResult)
		if result.WebFetch == nil || result.WebFetch.Type != WebFetchResultTypeError {
			t.Fatalf("web fetch result = %#v", result.WebFetch)
		}
	})

	t.Run("invalid type", func(t *testing.T) {
		var block anthropic.WebFetchToolResultBlock
		_ = json.Unmarshal([]byte(`{
			"type":"web_fetch_tool_result",
			"tool_use_id":"call_3",
			"content":{"type":"invalid"}
		}`), &block)

		_, err := webFetchToolResultToContentBlock(block)
		if err == nil || !strings.Contains(err.Error(), "invalid web fetch result type") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestCodeExecutionToolResultToContentBlock(t *testing.T) {
	t.Run("result type", func(t *testing.T) {
		var block anthropic.CodeExecutionToolResultBlock
		if err := json.Unmarshal([]byte(`{
			"type":"code_execution_tool_result",
			"tool_use_id":"call_1",
			"content":{
				"type":"code_execution_result",
				"stdout":"hello",
				"stderr":"",
				"return_code":0,
				"content":[{"type":"code_execution_output","file_id":"f1"}]
			}
		}`), &block); err != nil {
			t.Fatalf("unmarshal error = %v", err)
		}

		cb, err := codeExecutionToolResultToContentBlock(block)
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		result := cb.ServerToolResult.Content.(*ServerToolResult)
		if result.CodeExecution == nil || result.CodeExecution.Type != CodeExecutionResultTypeResult {
			t.Fatalf("code execution result = %#v", result.CodeExecution)
		}
		if result.CodeExecution.Result.Stdout != "hello" {
			t.Fatalf("stdout = %q", result.CodeExecution.Result.Stdout)
		}
	})

	t.Run("error type", func(t *testing.T) {
		var block anthropic.CodeExecutionToolResultBlock
		if err := json.Unmarshal([]byte(`{
			"type":"code_execution_tool_result",
			"tool_use_id":"call_2",
			"content":{"type":"code_execution_tool_result_error","error_code":"timeout"}
		}`), &block); err != nil {
			t.Fatalf("unmarshal error = %v", err)
		}

		cb, err := codeExecutionToolResultToContentBlock(block)
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		result := cb.ServerToolResult.Content.(*ServerToolResult)
		if result.CodeExecution == nil || result.CodeExecution.Type != CodeExecutionResultTypeError {
			t.Fatalf("result = %#v", result.CodeExecution)
		}
	})

	t.Run("encrypted type", func(t *testing.T) {
		var block anthropic.CodeExecutionToolResultBlock
		if err := json.Unmarshal([]byte(`{
			"type":"code_execution_tool_result",
			"tool_use_id":"call_3",
			"content":{
				"type":"encrypted_code_execution_result",
				"encrypted_stdout":"enc_data",
				"stderr":"err",
				"return_code":1,
				"content":[{"type":"code_execution_output","file_id":"f2"}]
			}
		}`), &block); err != nil {
			t.Fatalf("unmarshal error = %v", err)
		}

		cb, err := codeExecutionToolResultToContentBlock(block)
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		result := cb.ServerToolResult.Content.(*ServerToolResult)
		if result.CodeExecution == nil || result.CodeExecution.Type != CodeExecutionResultTypeEncrypted {
			t.Fatalf("result = %#v", result.CodeExecution)
		}
		if result.CodeExecution.EncryptedResult == nil || result.CodeExecution.EncryptedResult.EncryptedStdout != "enc_data" {
			t.Fatalf("encrypted result = %#v", result.CodeExecution.EncryptedResult)
		}
	})
}

func TestBashCodeExecutionToolResultToContentBlock(t *testing.T) {
	t.Run("result type", func(t *testing.T) {
		var block anthropic.BashCodeExecutionToolResultBlock
		if err := json.Unmarshal([]byte(`{
			"type":"bash_code_execution_tool_result",
			"tool_use_id":"call_1",
			"content":{"type":"bash_code_execution_result","stdout":"ok","stderr":"","return_code":0,"content":[]}
		}`), &block); err != nil {
			t.Fatalf("unmarshal error = %v", err)
		}

		cb, err := bashCodeExecutionToolResultToContentBlock(block)
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		result := cb.ServerToolResult.Content.(*ServerToolResult)
		if result.BashCodeExecution == nil || result.BashCodeExecution.Type != BashCodeExecutionResultTypeResult {
			t.Fatalf("result = %#v", result.BashCodeExecution)
		}
	})

	t.Run("error type", func(t *testing.T) {
		var block anthropic.BashCodeExecutionToolResultBlock
		if err := json.Unmarshal([]byte(`{
			"type":"bash_code_execution_tool_result",
			"tool_use_id":"call_2",
			"content":{"type":"bash_code_execution_tool_result_error","error_code":"sandbox_error"}
		}`), &block); err != nil {
			t.Fatalf("unmarshal error = %v", err)
		}

		cb, err := bashCodeExecutionToolResultToContentBlock(block)
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		result := cb.ServerToolResult.Content.(*ServerToolResult)
		if result.BashCodeExecution == nil || result.BashCodeExecution.Type != BashCodeExecutionResultTypeError {
			t.Fatalf("result = %#v", result.BashCodeExecution)
		}
	})
}

func TestTextEditorCodeExecutionToolResultToContentBlock(t *testing.T) {
	t.Run("view result", func(t *testing.T) {
		var block anthropic.TextEditorCodeExecutionToolResultBlock
		if err := json.Unmarshal([]byte(`{
			"type":"text_editor_code_execution_tool_result",
			"tool_use_id":"call_1",
			"content":{"type":"text_editor_code_execution_view_result","file_type":"python","content":"print(1)","num_lines":1,"start_line":1,"total_lines":1}
		}`), &block); err != nil {
			t.Fatalf("unmarshal error = %v", err)
		}

		cb, err := textEditorCodeExecutionToolResultToContentBlock(block)
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		result := cb.ServerToolResult.Content.(*ServerToolResult)
		if result.TextEditorCodeExecution == nil || result.TextEditorCodeExecution.Type != TextEditorCodeExecutionResultTypeView {
			t.Fatalf("result = %#v", result.TextEditorCodeExecution)
		}
	})

	t.Run("create result", func(t *testing.T) {
		var block anthropic.TextEditorCodeExecutionToolResultBlock
		if err := json.Unmarshal([]byte(`{
			"type":"text_editor_code_execution_tool_result",
			"tool_use_id":"call_2",
			"content":{"type":"text_editor_code_execution_create_result","is_file_update":true}
		}`), &block); err != nil {
			t.Fatalf("unmarshal error = %v", err)
		}

		cb, err := textEditorCodeExecutionToolResultToContentBlock(block)
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		result := cb.ServerToolResult.Content.(*ServerToolResult)
		if result.TextEditorCodeExecution == nil || result.TextEditorCodeExecution.Type != TextEditorCodeExecutionResultTypeCreate {
			t.Fatalf("result = %#v", result.TextEditorCodeExecution)
		}
	})

	t.Run("str replace result", func(t *testing.T) {
		var block anthropic.TextEditorCodeExecutionToolResultBlock
		if err := json.Unmarshal([]byte(`{
			"type":"text_editor_code_execution_tool_result",
			"tool_use_id":"call_3",
			"content":{"type":"text_editor_code_execution_str_replace_result","old_start":1,"old_lines":2,"new_start":1,"new_lines":3,"lines":["a","b"]}
		}`), &block); err != nil {
			t.Fatalf("unmarshal error = %v", err)
		}

		cb, err := textEditorCodeExecutionToolResultToContentBlock(block)
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		result := cb.ServerToolResult.Content.(*ServerToolResult)
		if result.TextEditorCodeExecution == nil || result.TextEditorCodeExecution.Type != TextEditorCodeExecutionResultTypeStrReplace {
			t.Fatalf("result = %#v", result.TextEditorCodeExecution)
		}
	})

	t.Run("error result", func(t *testing.T) {
		var block anthropic.TextEditorCodeExecutionToolResultBlock
		if err := json.Unmarshal([]byte(`{
			"type":"text_editor_code_execution_tool_result",
			"tool_use_id":"call_4",
			"content":{"type":"text_editor_code_execution_tool_result_error","error_code":"file_not_found","error_message":"not found"}
		}`), &block); err != nil {
			t.Fatalf("unmarshal error = %v", err)
		}

		cb, err := textEditorCodeExecutionToolResultToContentBlock(block)
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		result := cb.ServerToolResult.Content.(*ServerToolResult)
		if result.TextEditorCodeExecution == nil || result.TextEditorCodeExecution.Type != TextEditorCodeExecutionResultTypeError {
			t.Fatalf("result = %#v", result.TextEditorCodeExecution)
		}
	})
}

func TestToClaudeTextCitationFieldsCoverage(t *testing.T) {
	t.Run("char location", func(t *testing.T) {
		result := toClaudeTextCitationFields(claudeTextCitationFields{
			typ: string(claudeschema.TextCitationTypeCharLocation), citedText: "hello",
			documentTitle: "doc", documentIndex: 1, startCharIndex: 10, endCharIndex: 15,
		})
		if result == nil || result.CharLocation == nil || result.CharLocation.StartCharIndex != 10 {
			t.Fatalf("result = %#v", result)
		}
	})

	t.Run("page location", func(t *testing.T) {
		result := toClaudeTextCitationFields(claudeTextCitationFields{
			typ: string(claudeschema.TextCitationTypePageLocation), citedText: "page",
			documentIndex: 2, startPageNumber: 5, endPageNumber: 6,
		})
		if result == nil || result.PageLocation == nil || result.PageLocation.StartPageNumber != 5 {
			t.Fatalf("result = %#v", result)
		}
	})

	t.Run("content block location", func(t *testing.T) {
		result := toClaudeTextCitationFields(claudeTextCitationFields{
			typ: string(claudeschema.TextCitationTypeContentBlockLocation), citedText: "block",
			documentIndex: 3, startBlockIndex: 0, endBlockIndex: 2,
		})
		if result == nil || result.ContentBlockLocation == nil || result.ContentBlockLocation.EndBlockIndex != 2 {
			t.Fatalf("result = %#v", result)
		}
	})

	t.Run("web search result location", func(t *testing.T) {
		result := toClaudeTextCitationFields(claudeTextCitationFields{
			typ: string(claudeschema.TextCitationTypeWebSearchResultLocation), citedText: "web",
			title: "page title", url: "https://example.com", encryptedIndex: "enc_idx",
		})
		if result == nil || result.WebSearchResultLocation == nil || result.WebSearchResultLocation.URL != "https://example.com" {
			t.Fatalf("result = %#v", result)
		}
	})

	t.Run("unknown returns nil", func(t *testing.T) {
		result := toClaudeTextCitationFields(claudeTextCitationFields{typ: "unknown"})
		if result != nil {
			t.Fatalf("result = %#v, want nil", result)
		}
	})
}

func TestToAssistantTextExtension(t *testing.T) {
	t.Run("nil citations", func(t *testing.T) {
		if ext := toAssistantTextExtension(nil); ext != nil {
			t.Fatalf("ext = %#v, want nil", ext)
		}
	})

	t.Run("with citation", func(t *testing.T) {
		ext := toAssistantTextExtension([]anthropic.TextCitationUnion{
			{Type: "char_location", CitedText: "hello", DocumentIndex: 1, StartCharIndex: 0, EndCharIndex: 5},
		})
		if ext == nil || len(ext.Citations) != 1 {
			t.Fatalf("ext = %#v", ext)
		}
	})

	t.Run("all unknown returns nil", func(t *testing.T) {
		if ext := toAssistantTextExtension([]anthropic.TextCitationUnion{{Type: "unknown"}}); ext != nil {
			t.Fatalf("ext = %#v, want nil", ext)
		}
	})
}

func TestToAnthropicTextCitationsAdditional(t *testing.T) {
	t.Run("page location", func(t *testing.T) {
		citations, err := toAnthropicTextCitations([]*claudeschema.TextCitation{
			{Type: claudeschema.TextCitationTypePageLocation, PageLocation: &claudeschema.CitationPageLocation{
				CitedText: "page", DocumentIndex: 1, StartPageNumber: 3, EndPageNumber: 4,
			}},
		})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if len(citations) != 1 || citations[0].OfPageLocation == nil {
			t.Fatalf("citations = %#v", citations)
		}
	})

	t.Run("content block location", func(t *testing.T) {
		citations, err := toAnthropicTextCitations([]*claudeschema.TextCitation{
			{Type: claudeschema.TextCitationTypeContentBlockLocation, ContentBlockLocation: &claudeschema.CitationContentBlockLocation{
				CitedText: "block", DocumentIndex: 2, StartBlockIndex: 0, EndBlockIndex: 1,
			}},
		})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if len(citations) != 1 || citations[0].OfContentBlockLocation == nil {
			t.Fatalf("citations = %#v", citations)
		}
	})

	t.Run("invalid type", func(t *testing.T) {
		_, err := toAnthropicTextCitations([]*claudeschema.TextCitation{{Type: "invalid"}})
		if err == nil || !strings.Contains(err.Error(), "invalid text citation type") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestServerToolCallToBlockParam(t *testing.T) {
	t.Run("web search", func(t *testing.T) {
		blockParam, err := serverToolCallToBlockParam(&schema.ServerToolCall{
			CallID: "call_1", Name: string(ServerToolNameWebSearch),
			Arguments: &ServerToolCallArguments{WebSearch: &WebSearchArguments{Query: "golang"}},
		})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if !strings.Contains(mustJSON(t, blockParam), `"name":"web_search"`) {
			t.Fatalf("json = %s", mustJSON(t, blockParam))
		}
	})

	t.Run("code execution", func(t *testing.T) {
		blockParam, err := serverToolCallToBlockParam(&schema.ServerToolCall{
			CallID: "call_2", Name: string(ServerToolNameCodeExecution),
			Arguments: &ServerToolCallArguments{CodeExecution: &CodeExecutionArguments{Code: "print(1)"}},
		})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if !strings.Contains(mustJSON(t, blockParam), `"name":"code_execution"`) {
			t.Fatalf("json = %s", mustJSON(t, blockParam))
		}
	})

	t.Run("bash code execution", func(t *testing.T) {
		blockParam, err := serverToolCallToBlockParam(&schema.ServerToolCall{
			CallID: "call_3", Name: string(ServerToolNameBashCodeExecution),
			Arguments: &ServerToolCallArguments{BashCodeExecution: &BashCodeExecutionArguments{Command: "ls"}},
		})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if !strings.Contains(mustJSON(t, blockParam), `"name":"bash_code_execution"`) {
			t.Fatalf("json = %s", mustJSON(t, blockParam))
		}
	})

	t.Run("text editor", func(t *testing.T) {
		blockParam, err := serverToolCallToBlockParam(&schema.ServerToolCall{
			CallID: "call_4", Name: string(ServerToolNameTextEditorCodeExecution),
			Arguments: &ServerToolCallArguments{TextEditorCodeExecution: &TextEditorCodeExecutionArguments{Command: "view", Path: "/tmp/a.txt"}},
		})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if !strings.Contains(mustJSON(t, blockParam), `"name":"text_editor_code_execution"`) {
			t.Fatalf("json = %s", mustJSON(t, blockParam))
		}
	})

	t.Run("nil arguments", func(t *testing.T) {
		_, err := serverToolCallToBlockParam(&schema.ServerToolCall{
			CallID: "call_5", Name: string(ServerToolNameWebSearch),
			Arguments: &ServerToolCallArguments{},
		})
		if err == nil || !strings.Contains(err.Error(), "nil") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestTextEditorCodeExecutionToolResultToBlockParamCoverage(t *testing.T) {
	t.Run("error", func(t *testing.T) {
		_, err := textEditorCodeExecutionToolResultToBlockParam(&TextEditorCodeExecutionResult{
			Type:  TextEditorCodeExecutionResultTypeError,
			Error: &TextEditorCodeExecutionResultError{Code: "err", Message: "msg"},
		}, "call_1")
		if err != nil {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("create", func(t *testing.T) {
		_, err := textEditorCodeExecutionToolResultToBlockParam(&TextEditorCodeExecutionResult{
			Type: TextEditorCodeExecutionResultTypeCreate, Create: &TextEditorCodeExecutionCreateResult{IsFileUpdate: true},
		}, "call_2")
		if err != nil {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("str replace", func(t *testing.T) {
		_, err := textEditorCodeExecutionToolResultToBlockParam(&TextEditorCodeExecutionResult{
			Type:       TextEditorCodeExecutionResultTypeStrReplace,
			StrReplace: &TextEditorCodeExecutionStrReplaceResult{OldStart: 1, OldLines: 2, NewStart: 1, NewLines: 3, Lines: []string{"a"}},
		}, "call_3")
		if err != nil {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		_, err := textEditorCodeExecutionToolResultToBlockParam(&TextEditorCodeExecutionResult{Type: "bad"}, "call_4")
		if err == nil {
			t.Fatalf("expected error")
		}
	})
}

func TestCodeExecutionToolResultToBlockParamCoverage(t *testing.T) {
	t.Run("error", func(t *testing.T) {
		_, err := codeExecutionToolResultToBlockParam(&CodeExecutionResult{
			Type: CodeExecutionResultTypeError, Error: &CodeExecutionResultError{Code: "timeout"},
		}, "call_1")
		if err != nil {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("encrypted", func(t *testing.T) {
		_, err := codeExecutionToolResultToBlockParam(&CodeExecutionResult{
			Type: CodeExecutionResultTypeEncrypted,
			EncryptedResult: &EncryptedCodeExecutionResultBlock{
				Content: []*CodeExecutionOutput{{FileID: "f1"}}, EncryptedStdout: "enc", Stderr: "err", ReturnCode: 1,
			},
		}, "call_2")
		if err != nil {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		_, err := codeExecutionToolResultToBlockParam(&CodeExecutionResult{Type: "bad"}, "call_3")
		if err == nil {
			t.Fatalf("expected error")
		}
	})
}

func TestBashCodeExecutionToolResultToBlockParamCoverage(t *testing.T) {
	t.Run("error", func(t *testing.T) {
		_, err := bashCodeExecutionToolResultToBlockParam(&BashCodeExecutionResult{
			Type: BashCodeExecutionResultTypeError, Error: &BashCodeExecutionResultError{Code: "sandbox"},
		}, "call_1")
		if err != nil {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		_, err := bashCodeExecutionToolResultToBlockParam(&BashCodeExecutionResult{Type: "bad"}, "call_2")
		if err == nil {
			t.Fatalf("expected error")
		}
	})
}

func TestToAgenticContentBlockServerToolResults(t *testing.T) {
	t.Run("web search result block", func(t *testing.T) {
		var block anthropic.WebSearchToolResultBlock
		_ = json.Unmarshal([]byte(`{"type":"web_search_tool_result","tool_use_id":"call_1","content":[{"type":"web_search_result","title":"Go","url":"https://go.dev","encrypted_content":"e","page_age":"1d"}]}`), &block)
		cb, err := toAgenticContentBlock(block)
		if err != nil || cb == nil || cb.ServerToolResult == nil {
			t.Fatalf("error = %v, cb = %#v", err, cb)
		}
	})

	t.Run("code execution result block", func(t *testing.T) {
		var block anthropic.CodeExecutionToolResultBlock
		_ = json.Unmarshal([]byte(`{"type":"code_execution_tool_result","tool_use_id":"call_2","content":{"type":"code_execution_result","stdout":"hi","stderr":"","return_code":0,"content":[]}}`), &block)
		cb, err := toAgenticContentBlock(block)
		if err != nil || cb == nil || cb.ServerToolResult == nil {
			t.Fatalf("error = %v, cb = %#v", err, cb)
		}
	})

	t.Run("bash code execution result block", func(t *testing.T) {
		var block anthropic.BashCodeExecutionToolResultBlock
		_ = json.Unmarshal([]byte(`{"type":"bash_code_execution_tool_result","tool_use_id":"call_3","content":{"type":"bash_code_execution_result","stdout":"ok","stderr":"","return_code":0,"content":[]}}`), &block)
		cb, err := toAgenticContentBlock(block)
		if err != nil || cb == nil || cb.ServerToolResult == nil {
			t.Fatalf("error = %v, cb = %#v", err, cb)
		}
	})

	t.Run("text editor result block", func(t *testing.T) {
		var block anthropic.TextEditorCodeExecutionToolResultBlock
		_ = json.Unmarshal([]byte(`{"type":"text_editor_code_execution_tool_result","tool_use_id":"call_4","content":{"type":"text_editor_code_execution_view_result","file_type":"go","content":"pkg","num_lines":1,"start_line":1,"total_lines":1}}`), &block)
		cb, err := toAgenticContentBlock(block)
		if err != nil || cb == nil || cb.ServerToolResult == nil {
			t.Fatalf("error = %v, cb = %#v", err, cb)
		}
	})

	t.Run("web fetch result block", func(t *testing.T) {
		var block anthropic.WebFetchToolResultBlock
		_ = json.Unmarshal([]byte(`{"type":"web_fetch_tool_result","tool_use_id":"call_5","content":{"type":"web_fetch_tool_result_error","error_code":"fetch_failed"}}`), &block)
		cb, err := toAgenticContentBlock(block)
		if err != nil || cb == nil || cb.ServerToolResult == nil {
			t.Fatalf("error = %v, cb = %#v", err, cb)
		}
	})
}

func TestAssistantGenTextToBlockParamWithCitations(t *testing.T) {
	blockParam, err := assistantGenTextToBlockParam(&schema.AssistantGenText{
		Text: "answer",
		ClaudeExtension: &claudeschema.AssistantGenTextExtension{
			Citations: []*claudeschema.TextCitation{
				{Type: claudeschema.TextCitationTypeCharLocation, CharLocation: &claudeschema.CitationCharLocation{
					CitedText: "cite", DocumentIndex: 0, StartCharIndex: 0, EndCharIndex: 4,
				}},
			},
		},
	})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if !strings.Contains(mustJSON(t, blockParam), `"cited_text":"cite"`) {
		t.Fatalf("json = %s", mustJSON(t, blockParam))
	}
}

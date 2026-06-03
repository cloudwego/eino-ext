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
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestGetServerToolCallArguments(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		args := &ServerToolCallArguments{WebSearch: &WebSearchArguments{Query: "q"}}
		got, err := getServerToolCallArguments(&schema.ServerToolCall{
			Name:      string(ServerToolNameWebSearch),
			Arguments: args,
		})
		if err != nil {
			t.Fatalf("getServerToolCallArguments() error = %v", err)
		}
		if got != args {
			t.Fatalf("getServerToolCallArguments() = %p, want %p", got, args)
		}
	})

	t.Run("map[string]any success", func(t *testing.T) {
		got, err := getServerToolCallArguments(&schema.ServerToolCall{
			Name: string(ServerToolNameWebSearch),
			Arguments: map[string]any{
				"web_search": map[string]any{
					"query": "test query",
				},
			},
		})
		if err != nil {
			t.Fatalf("getServerToolCallArguments() error = %v", err)
		}
		if got.WebSearch == nil || got.WebSearch.Query != "test query" {
			t.Fatalf("getServerToolCallArguments() web_search.query = %v, want 'test query'", got.WebSearch)
		}
	})

	t.Run("unexpected type", func(t *testing.T) {
		_, err := getServerToolCallArguments(&schema.ServerToolCall{
			Name:      string(ServerToolNameWebSearch),
			Arguments: "invalid",
		})
		if err == nil || !strings.Contains(err.Error(), "unexpected type string for server tool call arguments") {
			t.Fatalf("getServerToolCallArguments() error = %v", err)
		}
	})
}

func TestGetServerToolResult(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		result := &ServerToolResult{WebFetch: &WebFetchResult{Type: WebFetchResultTypeResult}}
		got, err := getServerToolResult(&schema.ServerToolResult{
			Name:    string(ServerToolNameWebFetch),
			Content: result,
		})
		if err != nil {
			t.Fatalf("getServerToolResult() error = %v", err)
		}
		if got != result {
			t.Fatalf("getServerToolResult() = %p, want %p", got, result)
		}
	})

	t.Run("map[string]any success", func(t *testing.T) {
		got, err := getServerToolResult(&schema.ServerToolResult{
			Name: string(ServerToolNameWebFetch),
			Content: map[string]any{
				"web_fetch": map[string]any{
					"type": string(WebFetchResultTypeResult),
					"result": map[string]any{
						"url": "https://example.com",
					},
				},
			},
		})
		if err != nil {
			t.Fatalf("getServerToolResult() error = %v", err)
		}
		if got.WebFetch == nil || got.WebFetch.Type != WebFetchResultTypeResult {
			t.Fatalf("getServerToolResult() web_fetch.type = %v, want %v", got.WebFetch, WebFetchResultTypeResult)
		}
		if got.WebFetch.Result == nil || got.WebFetch.Result.URL != "https://example.com" {
			t.Fatalf("getServerToolResult() web_fetch.result.url = %v, want 'https://example.com'", got.WebFetch.Result)
		}
	})

	t.Run("unexpected type", func(t *testing.T) {
		_, err := getServerToolResult(&schema.ServerToolResult{
			Name:    string(ServerToolNameWebFetch),
			Content: "invalid",
		})
		if err == nil || !strings.Contains(err.Error(), "unexpected type string for server tool result") {
			t.Fatalf("getServerToolResult() error = %v", err)
		}
	})
}


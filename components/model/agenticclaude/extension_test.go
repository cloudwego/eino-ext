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

	t.Run("unexpected type", func(t *testing.T) {
		_, err := getServerToolCallArguments(&schema.ServerToolCall{
			Name:      string(ServerToolNameWebSearch),
			Arguments: map[string]any{"query": "q"},
		})
		if err == nil || !strings.Contains(err.Error(), "unexpected type map[string]interface {} for server tool call arguments") {
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

	t.Run("unexpected type", func(t *testing.T) {
		_, err := getServerToolResult(&schema.ServerToolResult{
			Name:    string(ServerToolNameWebFetch),
			Content: map[string]any{"url": "https://example.com"},
		})
		if err == nil || !strings.Contains(err.Error(), "unexpected type map[string]interface {} for server tool result") {
			t.Fatalf("getServerToolResult() error = %v", err)
		}
	})
}


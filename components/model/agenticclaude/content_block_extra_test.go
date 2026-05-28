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

func TestContentBlockExtraValueHelpers(t *testing.T) {
	block := &schema.ContentBlock{}
	setContentBlockExtraValue(block, "trace_id", "trace-1")

	got, ok := getContentBlockExtraValue[string](block, "trace_id")
	if !ok || got != "trace-1" {
		t.Fatalf("getContentBlockExtraValue() = (%q, %v)", got, ok)
	}

	if _, ok := getContentBlockExtraValue[int](block, "trace_id"); ok {
		t.Fatalf("getContentBlockExtraValue() should reject mismatched types")
	}
}

func TestResultCallerRoundTrip(t *testing.T) {
	t.Run("web search", func(t *testing.T) {
		var caller anthropic.WebSearchToolResultBlockCallerUnion
		if err := json.Unmarshal([]byte(`{"type":"direct"}`), &caller); err != nil {
			t.Fatalf("json.Unmarshal(web search caller) error = %v", err)
		}

		block := &schema.ContentBlock{}
		setWebSearchResultCaller(block, caller)

		param, err := toWebSearchResultCallerParam(block)
		if err != nil {
			t.Fatalf("toWebSearchResultCallerParam() error = %v", err)
		}
		if typ := param.GetType(); typ == nil || *typ != "direct" {
			t.Fatalf("caller type = %#v", typ)
		}
	})

	t.Run("web fetch", func(t *testing.T) {
		var caller anthropic.WebFetchToolResultBlockCallerUnion
		if err := json.Unmarshal([]byte(`{"type":"direct"}`), &caller); err != nil {
			t.Fatalf("json.Unmarshal(web fetch caller) error = %v", err)
		}

		block := &schema.ContentBlock{}
		setWebFetchResultCaller(block, caller)

		param, err := toWebFetchResultCallerParam(block)
		if err != nil {
			t.Fatalf("toWebFetchResultCallerParam() error = %v", err)
		}
		if typ := param.GetType(); typ == nil || *typ != "direct" {
			t.Fatalf("caller type = %#v", typ)
		}
	})
}

func TestResultCallerDecodeError(t *testing.T) {
	block := &schema.ContentBlock{Extra: map[string]any{
		keyOfWebSearchToolResultCaller: "{",
	}}

	_, err := toWebSearchResultCallerParam(block)
	if err == nil {
		t.Fatalf("toWebSearchResultCallerParam() should fail on invalid json")
	}
}

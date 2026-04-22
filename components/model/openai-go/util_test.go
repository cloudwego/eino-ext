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

	"github.com/cloudwego/eino/schema"
	jschema "github.com/eino-contrib/jsonschema"
	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/packages/param"
	"github.com/openai/openai-go/v3/responses"
)

func TestExtractAssistantTextForHistory(t *testing.T) {
	text, ok, err := extractAssistantTextForHistory(&schema.Message{Content: "hello"})
	if err != nil || !ok || text != "hello" {
		t.Fatalf("unexpected content extraction result text=%q ok=%v err=%v", text, ok, err)
	}

	text, ok, err = extractAssistantTextForHistory(&schema.Message{AssistantGenMultiContent: []schema.MessageOutputPart{{Type: schema.ChatMessagePartTypeText, Text: "one"}, {Type: schema.ChatMessagePartTypeText, Text: "two"}}})
	if err != nil || !ok || text != "one\ntwo" {
		t.Fatalf("unexpected assistant multi-content extraction: text=%q ok=%v err=%v", text, ok, err)
	}

	text, ok, err = extractAssistantTextForHistory(&schema.Message{MultiContent: []schema.ChatMessagePart{{Type: schema.ChatMessagePartTypeText, Text: "legacy1"}, {Type: schema.ChatMessagePartTypeText, Text: "legacy2"}}})
	if err != nil || !ok || text != "legacy1\nlegacy2" {
		t.Fatalf("unexpected deprecated multi-content extraction: text=%q ok=%v err=%v", text, ok, err)
	}

	text, ok, err = extractAssistantTextForHistory(nil)
	if err != nil || ok || text != "" {
		t.Fatalf("expected nil message to produce no text, got text=%q ok=%v err=%v", text, ok, err)
	}

	tests := []struct {
		name string
		msg  *schema.Message
		want string
	}{
		{name: "assistant non-text output", msg: &schema.Message{AssistantGenMultiContent: []schema.MessageOutputPart{{Type: schema.ChatMessagePartTypeImageURL}}}, want: "non-text part"},
		{name: "deprecated non-text output", msg: &schema.Message{MultiContent: []schema.ChatMessagePart{{Type: schema.ChatMessagePartTypeImageURL}}}, want: "deprecated MultiContent non-text part"},
		{name: "assistant user input content", msg: &schema.Message{UserInputMultiContent: []schema.MessageInputPart{{Type: schema.ChatMessagePartTypeText, Text: "bad"}}}, want: "UserInputMultiContent"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := extractAssistantTextForHistory(tt.msg)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error containing %q, got %v", tt.want, err)
			}
		})
	}
}

func TestToolNameExists(t *testing.T) {
	tools := []responses.ToolUnionParam{responses.ToolParamOfFunction("lookup_weather", map[string]any{"type": "object"}, true)}
	if !toolNameExists(tools, "lookup_weather") {
		t.Fatalf("expected tool to exist")
	}
	if toolNameExists(tools, "missing") {
		t.Fatalf("did not expect missing tool to exist")
	}
}

func TestEnforceOpenAIStrictJSONSchema(t *testing.T) {
	schemaMap := map[string]any{
		"properties": map[string]any{
			"name": map[string]any{"type": "string"},
			"profile": map[string]any{
				"properties": map[string]any{
					"age": map[string]any{"type": "integer"},
				},
			},
		},
		"items": map[string]any{
			"properties": map[string]any{
				"nested": map[string]any{"type": "string"},
			},
		},
	}
	enforceOpenAIStrictJSONSchema(schemaMap)
	if schemaMap["type"] != "object" || schemaMap["additionalProperties"] != false {
		t.Fatalf("expected strict object schema, got %#v", schemaMap)
	}
	if required, ok := schemaMap["required"].([]any); !ok || len(required) != 2 {
		t.Fatalf("expected required keys for all properties, got %#v", schemaMap["required"])
	}
	profile := schemaMap["properties"].(map[string]any)["profile"].(map[string]any)
	if profile["type"] != "object" || profile["additionalProperties"] != false {
		t.Fatalf("expected nested strict schema, got %#v", profile)
	}
}

func TestJSONSchemaToMap(t *testing.T) {
	m, err := jsonSchemaToMap(&jschema.Schema{Type: "object", Description: "demo"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m["type"] != "object" || m["description"] != "demo" {
		t.Fatalf("unexpected schema map: %#v", m)
	}
	m, err = jsonSchemaToMap(nil)
	if err != nil {
		t.Fatalf("unexpected error for nil schema: %v", err)
	}
	if len(m) != 0 {
		t.Fatalf("expected empty map for nil schema, got %#v", m)
	}
}

func TestCommonToDataOrURL(t *testing.T) {
	url := "https://example.com/image.png"
	got, err := commonToDataOrURL(schema.MessagePartCommon{URL: &url})
	if err != nil || got != url {
		t.Fatalf("expected url passthrough, got %q err=%v", got, err)
	}
	b64 := "SGVsbG8="
	got, err = commonToDataOrURL(schema.MessagePartCommon{Base64Data: &b64, MIMEType: "text/plain"})
	if err != nil || got != "data:text/plain;base64,SGVsbG8=" {
		t.Fatalf("unexpected data url: %q err=%v", got, err)
	}
	badPrefixed := "data:text/plain;base64,SGVsbG8="
	tests := []struct {
		name   string
		common schema.MessagePartCommon
		want   string
	}{
		{name: "missing source", common: schema.MessagePartCommon{}, want: "URL or Base64Data"},
		{name: "missing mime", common: schema.MessagePartCommon{Base64Data: &b64}, want: "MIMEType"},
		{name: "prefixed data", common: schema.MessagePartCommon{Base64Data: &badPrefixed, MIMEType: "text/plain"}, want: "raw base64"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := commonToDataOrURL(tt.common)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error containing %q, got %v", tt.want, err)
			}
		})
	}
}

func TestJoinReasoningText(t *testing.T) {
	item := responses.ResponseReasoningItem{Summary: []responses.ResponseReasoningItemSummary{{Text: "s1"}, {Text: "s2"}}}
	if got := joinReasoningText(item); got != "s1\n\ns2" {
		t.Fatalf("unexpected summary text %q", got)
	}
	item = responses.ResponseReasoningItem{Content: []responses.ResponseReasoningItemContent{{Text: "c1"}, {Text: "c2"}}}
	if got := joinReasoningText(item); got != "c1\n\nc2" {
		t.Fatalf("unexpected content text %q", got)
	}
	if got := joinReasoningText(responses.ResponseReasoningItem{}); got != "" {
		t.Fatalf("expected empty reasoning text, got %q", got)
	}
}

func TestEnsureResponseMetaAndOptHelpers(t *testing.T) {
	meta := ensureResponseMeta(nil)
	if meta == nil {
		t.Fatalf("expected response meta to be initialized")
	}
	meta2 := &schema.ResponseMeta{}
	if ensureResponseMeta(meta2) != meta2 {
		t.Fatalf("expected existing meta to be returned as-is")
	}
	if got := responsesModelFromString("gpt-4o-mini"); got != "gpt-4o-mini" {
		t.Fatalf("unexpected model conversion %q", got)
	}
	if got := optInt64(param.Opt[int64]{}); got != 0 {
		t.Fatalf("expected zero int64 opt, got %d", got)
	}
	if got := optInt64(openai.Int(7)); got != 7 {
		t.Fatalf("expected int64 opt value 7, got %d", got)
	}
	if got := optFloat64(param.Opt[float64]{}); got != 0 {
		t.Fatalf("expected zero float64 opt, got %v", got)
	}
	if got := optFloat64(openai.Float(1.5)); got != 1.5 {
		t.Fatalf("expected float64 opt value 1.5, got %v", got)
	}
}

func TestPanicAndCloneHelpers(t *testing.T) {
	err := newPanicErr("boom", []byte("stacktrace"))
	if err == nil || !strings.Contains(err.Error(), "boom") || !strings.Contains(err.Error(), "stacktrace") {
		t.Fatalf("unexpected panic error %v", err)
	}
	if got := cloneStringMap(nil); got != nil {
		t.Fatalf("expected nil clone for nil string map, got %#v", got)
	}
	if got := cloneAnyMap(nil); got != nil {
		t.Fatalf("expected nil clone for nil any map, got %#v", got)
	}
	stringMap := map[string]string{"a": "b"}
	anyMap := map[string]any{"x": 1}
	cloneS := cloneStringMap(stringMap)
	cloneA := cloneAnyMap(anyMap)
	stringMap["a"] = "changed"
	anyMap["x"] = 2
	if cloneS["a"] != "b" || cloneA["x"] != 1 {
		t.Fatalf("expected clones to be independent, got %#v %#v", cloneS, cloneA)
	}
}

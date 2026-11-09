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

package ark

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/responses"
)

const responsesProjectionJSON = `[
 {"type":"function_call","id":"fc","call_id":"call","name":"inspect","arguments":"{}","status":"completed","extension":9007199254740993},
 {"type":"reasoning","id":"r","status":"completed","summary":[{"type":"summary_text","text":"one","extension":1},{"type":"summary_text","text":"two"}],"content":[{"type":"reasoning_text","text":"raw","annotations":[]}],"encrypted_content":"cipher"},
 {"type":"message","id":"m","role":"assistant","status":"completed","phase":"final_answer","extension":true,"content":[{"type":"output_text","text":"A","extension":9007199254740993,"annotations":[{"type":"future_annotation","value":9007199254740993}]},{"type":"output_text","text":"B","extension":true}]},
 {"type":"message","id":"empty","role":"assistant","status":"completed","content":[]},
 {"type":"future_output","payload":{"value":9007199254740993}}
]`

// Only fields represented by the SDK input schema belong in the next request.
const responsesProjectionInputJSON = `[
 {"type":"function_call","id":"fc","call_id":"call","name":"inspect","arguments":"{}","status":"completed"},
 {"type":"reasoning","id":"r","status":"completed","summary":[{"type":"summary_text","text":"one"},{"type":"summary_text","text":"two"}],"content":[{"type":"reasoning_text","text":"raw","annotations":[]}],"encrypted_content":"cipher"},
 {"type":"message","id":"m","role":"assistant","content":[{"type":"output_text","text":"A","annotations":[{"type":"future_annotation","value":9007199254740993}]},{"type":"output_text","text":"B"}]},
 {"type":"message","id":"empty","role":"assistant","content":[]}
]`

func projectedResponsesMessage(t *testing.T, mode, output string) *schema.Message {
	t.Helper()
	cm := testResponsesModel(t, func(w http.ResponseWriter, r *http.Request) {
		body := decodeRequest(t, r)
		raw := fmt.Sprintf(`{"id":"projected","created_at":1,"object":"response","model":"test","status":"completed","output":%s}`, output)
		var compact bytes.Buffer
		check(t, json.Compact(&compact, []byte(raw)))
		raw = compact.String()
		if body["stream"] == true {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, responseEvent("response.completed", map[string]any{"response": json.RawMessage(raw)}))
		} else {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, raw)
		}
	}, nil)
	return callResponsesWithOptions(t, cm, mode, []*schema.Message{schema.UserMessage("hello")})
}

func TestResponsesOutputProjection(t *testing.T) {
	for _, mode := range []string{"generate", "stream"} {
		t.Run(mode, func(t *testing.T) {
			original := projectedResponsesMessage(t, mode, responsesProjectionJSON)
			equal(t, "AB", original.Content)
			equal(t, "one\ntwo", original.ReasoningContent)
			equal(t, 1, len(original.ToolCalls))
			if len(original.AssistantGenMultiContent) == 0 {
				t.Fatal("native output has no canonical parts")
			}
			if _, ok := original.Extra["ark-responses-output"]; ok {
				t.Fatal("full output shadow remains")
			}
			for _, codec := range messagePersistenceCodecs() {
				t.Run(codec.name, func(t *testing.T) {
					raw, err := codec.marshal(original)
					check(t, err)
					var msg schema.Message
					check(t, codec.unmarshal(raw, &msg))
					msg.Content = "ignored aggregate"
					msg.ReasoningContent = "ignored reasoning aggregate"
					input, err := toResponsesInput([]*schema.Message{&msg})
					check(t, err)
					raw, err = json.Marshal(input)
					check(t, err)
					equalRawJSON(t, []byte(responsesProjectionInputJSON), raw)
					foundText, foundReasoning := false, false
					for i := range msg.AssistantGenMultiContent {
						part := &msg.AssistantGenMultiContent[i]
						if part.Reasoning != nil && part.Reasoning.Signature != "" {
							part.Reasoning.Signature = "new cipher"
						}
						if part.Type == schema.ChatMessagePartTypeText && part.Text == "A" {
							part.Text = "edited"
							foundText = true
						}
						if part.Reasoning != nil && part.Reasoning.Text == "one" {
							part.Reasoning.Text = "changed"

							foundReasoning = true
						}
					}
					equal(t, true, foundText)
					equal(t, true, foundReasoning)
					msg.ToolCalls[0].ID = "changed call"
					msg.ToolCalls[0].Function.Name = "new_name"
					msg.ToolCalls[0].Function.Arguments = `{"x":1}`
					input, err = toResponsesInput([]*schema.Message{&msg})
					check(t, err)
					raw, err = json.Marshal(input)
					check(t, err)
					want := strings.NewReplacer(`"text":"A"`, `"text":"edited"`, `"text":"one"`, `"text":"changed"`, `"encrypted_content":"cipher"`, `"encrypted_content":"new cipher"`, `"call_id":"call"`, `"call_id":"changed call"`, `"name":"inspect"`, `"name":"new_name"`, `"arguments":"{}"`, `"arguments":"{\"x\":1}"`).Replace(responsesProjectionInputJSON)
					equalRawJSON(t, []byte(want), raw)
					for _, item := range input {
						if _, _, unknown := item.OneOf.GetUnknown(); unknown {
							t.Fatal("native output bypassed typed SDK inputs")
						}
					}
				})
			}
		})
	}
}

func TestResponsesReasoningPartsOverrideAggregates(t *testing.T) {
	msg := &schema.Message{Role: schema.Assistant, Content: "ignored", ReasoningContent: "ignored reasoning", AssistantGenMultiContent: []schema.MessageOutputPart{
		{Type: schema.ChatMessagePartTypeText, Text: "canonical"},
		{Type: schema.ChatMessagePartTypeReasoning, Reasoning: &schema.MessageOutputReasoning{Text: "canonical reasoning", Signature: "signature"}},
	}}
	input, err := toResponsesInput([]*schema.Message{msg})
	check(t, err)
	raw, err := json.Marshal(input)
	check(t, err)
	var items []map[string]json.RawMessage
	check(t, json.Unmarshal(raw, &items))
	equal(t, 2, len(items))
	if strings.Contains(string(raw), "ignored") {
		t.Fatal(string(raw))
	}
}

func TestResponsesProjectionEditsAndIndices(t *testing.T) {
	original := projectedResponsesMessage(t, "generate", responsesProjectionJSON)
	clone := func() *schema.Message {
		raw, err := json.Marshal(original)
		check(t, err)
		var msg schema.Message
		check(t, json.Unmarshal(raw, &msg))
		return &msg
	}
	wire := func(msg *schema.Message) []json.RawMessage {
		t.Helper()
		input, err := toResponsesInput([]*schema.Message{msg})
		check(t, err)
		raw, err := json.Marshal(input)
		check(t, err)
		var result []json.RawMessage
		check(t, json.Unmarshal(raw, &result))
		return result
	}
	var initial []json.RawMessage
	check(t, json.Unmarshal([]byte(responsesProjectionInputJSON), &initial))
	t.Run("stream merge indices do not order items", func(t *testing.T) {
		msg := clone()
		for i := range msg.AssistantGenMultiContent {
			msg.AssistantGenMultiContent[i].StreamingMeta = &schema.MessageStreamingMeta{Index: 500 + i}
		}
		mergeIndex := 999
		msg.ToolCalls[0].Index = &mergeIndex
		actual := wire(msg)
		for i := range initial {
			equalRawJSON(t, initial[i], actual[i])
		}
	})
	t.Run("delete text and tool", func(t *testing.T) {
		msg := clone()
		msg.ToolCalls = nil
		var parts []schema.MessageOutputPart
		for _, part := range msg.AssistantGenMultiContent {
			if part.Type != schema.ChatMessagePartTypeText || part.Text != "B" {
				parts = append(parts, part)
			}
		}
		msg.AssistantGenMultiContent = parts
		actual := wire(msg)
		equal(t, 3, len(actual))
		var item map[string]json.RawMessage
		check(t, json.Unmarshal(actual[1], &item))
		var content []map[string]any
		check(t, json.Unmarshal(item["content"], &content))
		equal(t, 1, len(content))
		equal(t, "A", content[0]["text"])
		equalRawJSON(t, initial[1], actual[0])
		equalRawJSON(t, initial[3], actual[2])
	})
	t.Run("move tool and preserve metadata", func(t *testing.T) {
		msg := clone()
		for i := range msg.AssistantGenMultiContent {
			part := &msg.AssistantGenMultiContent[i]
			index, exists, err := extraIndex(part.Extra, keyResponsesItemIndex)
			check(t, err)
			if exists && index == 0 {
				part.Extra[keyResponsesItemIndex] = metadataInt64(8)
			}
		}
		msg.ToolCalls[0].Extra[keyResponsesItemIndex] = metadataInt64(8)
		actual := wire(msg)
		equalRawJSON(t, initial[0], actual[3])
		equalRawJSON(t, initial[1], actual[0])
	})
	t.Run("append manual part", func(t *testing.T) {
		msg := clone()
		msg.AssistantGenMultiContent = append(msg.AssistantGenMultiContent, schema.MessageOutputPart{Type: schema.ChatMessagePartTypeText, Text: "new text"})
		actual := wire(msg)
		equal(t, 5, len(actual))
		if !strings.Contains(string(actual[4]), "new text") {
			t.Fatal(string(actual[4]))
		}
	})
	t.Run("append tool without native index", func(t *testing.T) {
		msg := clone()
		msg.ToolCalls = append(msg.ToolCalls, schema.ToolCall{ID: "new-call", Type: "function", Function: schema.FunctionCall{Name: "new_tool", Arguments: "{}"}})
		actual := wire(msg)
		equal(t, 5, len(actual))
		equalRawJSON(t, []byte(`{"type":"function_call","call_id":"new-call","name":"new_tool","arguments":"{}"}`), actual[4])
	})
	t.Run("read only", func(t *testing.T) {
		msg := clone()
		before, err := json.Marshal(msg)
		check(t, err)
		_ = wire(msg)
		after, err := json.Marshal(msg)
		check(t, err)
		equalRawJSON(t, before, after)
	})
	for _, kind := range []string{"invalid item index", "null item index", "string item index", "fractional item index", "invalid content index", "invalid tool index", "invalid scalar metadata", "invalid annotations", "duplicate item metadata", "orphan content"} {
		t.Run(kind, func(t *testing.T) {
			msg := clone()
			switch kind {
			case "invalid item index":
				msg.AssistantGenMultiContent[0].Extra[keyResponsesItemIndex] = float64(-1)
			case "null item index":
				msg.AssistantGenMultiContent[0].Extra[keyResponsesItemIndex] = nil
			case "string item index":
				msg.AssistantGenMultiContent[0].Extra[keyResponsesItemIndex] = "0"
			case "fractional item index":
				msg.AssistantGenMultiContent[0].Extra[keyResponsesItemIndex] = 0.5
			case "invalid content index":
				for i := range msg.AssistantGenMultiContent {
					if _, content := msg.AssistantGenMultiContent[i].Extra[keyResponsesContentIndex]; content {
						msg.AssistantGenMultiContent[i].Extra[keyResponsesContentIndex] = nil
						break
					}
				}
			case "invalid tool index":
				msg.ToolCalls[0].Extra[keyResponsesItemIndex] = nil
			case "invalid scalar metadata":
				msg.AssistantGenMultiContent[0].Extra[keyResponsesItemStatus] = 123
			case "invalid annotations":
				for i := range msg.AssistantGenMultiContent {
					part := &msg.AssistantGenMultiContent[i]
					if part.Type == schema.ChatMessagePartTypeText {
						part.Extra[keyResponsesAnnotations] = "{"
						break
					}
				}
			case "duplicate item metadata":
				msg.AssistantGenMultiContent = append(msg.AssistantGenMultiContent, msg.AssistantGenMultiContent[0])
			case "orphan content":
				var parts []schema.MessageOutputPart
				for _, part := range msg.AssistantGenMultiContent {
					if !hasResponsesItemMetadata(&part) {
						parts = append(parts, part)
					}
				}
				msg.AssistantGenMultiContent = parts
			}
			_, err := toResponsesInput([]*schema.Message{msg})
			if err == nil {
				t.Fatal("invalid projection accepted")
			}
		})
	}
}

func TestResponsesProjectionPresence(t *testing.T) {
	output := `[{"type":"message","id":"m","role":"assistant","status":"completed","content":[{"type":"output_text"},{"type":"output_text","text":""}]},{"type":"reasoning","status":"completed","content":[]},{"type":"future_output","id":9007199254740993,"partial":{"nested":true},"content":{"number":9007199254740993},"text":"opaque text","arguments":null}]`
	var items []responses.OutputItem
	check(t, sonic.UnmarshalString(output, &items))
	msg := &schema.Message{Role: schema.Assistant}
	check(t, projectResponsesOutput(msg, items))
	for _, codec := range messagePersistenceCodecs() {
		t.Run(codec.name, func(t *testing.T) {
			raw, err := codec.marshal(msg)
			check(t, err)
			var restored schema.Message
			check(t, codec.unmarshal(raw, &restored))
			input, err := toResponsesInput([]*schema.Message{&restored})
			check(t, err)
			raw, err = json.Marshal(input)
			check(t, err)
			equalRawJSON(t, []byte(`[{"type":"message","id":"m","role":"assistant","content":[{"type":"output_text","text":""},{"type":"output_text","text":""}]},{"type":"reasoning","status":"completed"}]`), raw)
			equalMetadataJSON(t, msg, &restored)
		})
	}
}

func TestResponsesFlatMetadata(t *testing.T) {
	original := projectedResponsesMessage(t, "generate", responsesProjectionJSON)
	for _, codec := range messagePersistenceCodecs() {
		t.Run(codec.name, func(t *testing.T) {
			raw, err := codec.marshal(original)
			check(t, err)
			var msg schema.Message
			check(t, codec.unmarshal(raw, &msg))
			foundItem, foundText := false, false
			for i := range msg.AssistantGenMultiContent {
				part := &msg.AssistantGenMultiContent[i]
				if id, _ := extraString(part.Extra, "ark-responses-item-id"); id == "fc" {
					foundItem = true
					part.Extra["ark-responses-item-id"] = "edited-fc"
					part.Extra["ark-responses-item-status"] = "in_progress"
				}
				if part.Type == schema.ChatMessagePartTypeText && part.Text == "A" {
					foundText = true
					kind, ok := extraString(part.Extra, keyResponsesContentType)
					equal(t, true, ok)
					equal(t, "output_text", kind)
					itemKind, ok := extraString(part.Extra, keyResponsesItemType)
					equal(t, true, ok)
					equal(t, "message", itemKind)
					annotations, ok := extraString(part.Extra, "ark-responses-annotations")
					equal(t, true, ok)
					equalRawJSON(t, []byte(`[{"type":"future_annotation","value":9007199254740993}]`), []byte(annotations))
				}
				for _, key := range []string{"ark-responses-item", "ark-responses-content"} {
					if _, exists := part.Extra[key]; exists {
						t.Fatalf("whole object is still stored in Extra[%q]", key)
					}
				}
			}
			equal(t, true, foundItem)
			equal(t, true, foundText)
			raw, err = codec.marshal(&msg)
			check(t, err)
			var restored schema.Message
			check(t, codec.unmarshal(raw, &restored))
			input, err := toResponsesInput([]*schema.Message{&restored})
			check(t, err)
			raw, err = json.Marshal(input)
			check(t, err)
			expected := strings.Replace(responsesProjectionInputJSON, `"id":"fc"`, `"id":"edited-fc"`, 1)
			expected = strings.Replace(expected, `"status":"completed"`, `"status":"in_progress"`, 1)
			equalRawJSON(t, []byte(expected), raw)
		})
	}
}

func TestResponsesOutputMetadataDoesNotExtendInput(t *testing.T) {
	msg := projectedResponsesMessage(t, "generate", responsesProjectionJSON)
	for i := range msg.AssistantGenMultiContent {
		part := &msg.AssistantGenMultiContent[i]
		if hasResponsesItemMetadata(part) {
			// Unrecognized application metadata cannot bypass SDK input fields.
			part.Extra["ark-responses-item-raw.extension"] = "not request JSON"
			part.Extra["ark-responses-item-raw.id"] = `"must-not-replace-id"`
		}
	}
	items, err := toResponsesInput([]*schema.Message{msg})
	check(t, err)
	raw, err := json.Marshal(items)
	check(t, err)
	equalRawJSON(t, []byte(responsesProjectionInputJSON), raw)
}

func TestResponsesReasoningUsesPublicSignature(t *testing.T) {
	original := projectedResponsesMessage(t, "generate", responsesProjectionJSON)
	clone := func() *schema.Message {
		raw, err := json.Marshal(original)
		check(t, err)
		var msg schema.Message
		check(t, json.Unmarshal(raw, &msg))
		return &msg
	}
	t.Run("no private signature marker", func(t *testing.T) {
		for _, part := range original.AssistantGenMultiContent {
			if _, exists := part.Extra["ark-responses-signature"]; exists {
				t.Fatal("signature ownership is duplicated in Extra")
			}
		}
	})
	t.Run("move signature between reasoning parts", func(t *testing.T) {
		msg := clone()
		last := -1
		for i := range msg.AssistantGenMultiContent {
			part := &msg.AssistantGenMultiContent[i]
			if part.Reasoning != nil {
				part.Reasoning.Signature = ""
				last = i
			}
		}
		if last < 0 {
			t.Fatal("no reasoning part")
		}
		msg.AssistantGenMultiContent[last].Reasoning.Signature = "moved cipher"
		input, err := toResponsesInput([]*schema.Message{msg})
		check(t, err)
		raw, err := json.Marshal(input)
		check(t, err)
		expected := strings.Replace(responsesProjectionInputJSON, `"encrypted_content":"cipher"`, `"encrypted_content":"moved cipher"`, 1)
		equalRawJSON(t, []byte(expected), raw)
	})
	t.Run("conflicting public signatures", func(t *testing.T) {
		msg := clone()
		for i := range msg.AssistantGenMultiContent {
			part := &msg.AssistantGenMultiContent[i]
			if part.Reasoning != nil && part.Reasoning.Signature == "" {
				part.Reasoning.Signature = "another cipher"
				break
			}
		}
		_, err := toResponsesInput([]*schema.Message{msg})
		if err == nil {
			t.Fatal("multiple item signatures accepted")
		}
	})
}

func TestResponsesReorderedPartsAndTools(t *testing.T) {
	output := `[
 {"type":"message","id":"a","status":"completed","role":"assistant","content":[{"type":"output_text","text":"first"},{"type":"output_text","text":"second"}]},
 {"type":"function_call","call_id":"c1","name":"f","arguments":"{}"},
 {"type":"reasoning","id":"r1","status":"completed","summary":[{"type":"summary_text","text":"summary one"},{"type":"summary_text","text":"summary two"}],"content":[{"type":"reasoning_text","text":"content one"},{"type":"reasoning_text","text":"content two"}],"encrypted_content":"cipher one"},
 {"type":"function_call","call_id":"c2","name":"g","arguments":"{}"},
 {"type":"reasoning","id":"r2","status":"completed","summary":[{"type":"summary_text","text":"another summary"}],"content":[],"encrypted_content":"cipher two"},
 {"type":"message","id":"b","status":"completed","role":"assistant","content":[{"type":"output_text","text":"last"}]}
]`
	for _, mode := range []string{"generate", "stream"} {
		t.Run(mode, func(t *testing.T) {
			msg := projectedResponsesMessage(t, mode, output)
			for i, j := 0, len(msg.AssistantGenMultiContent)-1; i < j; i, j = i+1, j-1 {
				msg.AssistantGenMultiContent[i], msg.AssistantGenMultiContent[j] = msg.AssistantGenMultiContent[j], msg.AssistantGenMultiContent[i]
			}
			msg.ToolCalls[0], msg.ToolCalls[1] = msg.ToolCalls[1], msg.ToolCalls[0]
			for _, codec := range messagePersistenceCodecs() {
				t.Run(codec.name, func(t *testing.T) {
					raw, err := codec.marshal(msg)
					check(t, err)
					var restored schema.Message
					check(t, codec.unmarshal(raw, &restored))
					input, err := toResponsesInput([]*schema.Message{&restored})
					check(t, err)
					raw, err = json.Marshal(input)
					check(t, err)
					expected := strings.ReplaceAll(output, `"status":"completed","role":"assistant",`, `"role":"assistant",`)
					expected = strings.ReplaceAll(expected, `"content":[],"encrypted_content"`, `"encrypted_content"`)
					equalRawJSON(t, []byte(expected), raw)
				})
			}
		})
	}
}

func TestResponsesItemMetadataMergesIntoFirstDataPart(t *testing.T) {
	for _, mode := range []string{"generate", "stream"} {
		t.Run(mode, func(t *testing.T) {
			msg := projectedResponsesMessage(t, mode, responsesProjectionJSON)
			equal(t, 8, len(msg.AssistantGenMultiContent))
			seen := map[int]bool{}
			for _, part := range msg.AssistantGenMultiContent {
				index, exists, err := extraIndex(part.Extra, keyResponsesItemIndex)
				check(t, err)
				equal(t, true, exists)
				equal(t, !seen[index], hasResponsesItemMetadata(&part))
				seen[index] = true
				if index == 1 || index == 2 {
					if part.Type == ResponsesOutputItem {
						t.Fatal("item with data still has a separate metadata part")
					}
				}
				if section, _ := extraString(part.Extra, keyResponsesSection); section == "summary" {
					index, exists, err := extraIndex(part.Extra, keyResponsesSummaryIndex)
					check(t, err)
					equal(t, true, exists)
					if index == 0 {
						equal(t, "one", part.Reasoning.Text)
						equal(t, "cipher", part.Reasoning.Signature)
					} else {
						equal(t, "two", part.Reasoning.Text)
					}
					if _, exists := part.Extra[keyResponsesContentIndex]; exists {
						t.Fatal("summary retains a segment index")
					}
				}
			}
		})
	}
}

func TestResponsesMoveMetadataBeforeDeletingFirstDataPart(t *testing.T) {
	msg := projectedResponsesMessage(t, "generate", responsesProjectionJSON)
	var metadata map[string]any
	for _, part := range msg.AssistantGenMultiContent {
		if part.Text == "A" {
			metadata = part.Extra
		}
	}
	var retained []schema.MessageOutputPart
	for _, part := range msg.AssistantGenMultiContent {
		if part.Text == "A" {
			continue
		}
		if part.Text == "B" {
			for _, key := range []string{keyResponsesItemType, keyResponsesItemID, keyResponsesItemStatus, keyResponsesItemRole} {
				part.Extra[key] = metadata[key]
			}
		}
		retained = append(retained, part)
	}
	msg.AssistantGenMultiContent = retained
	input, err := toResponsesInput([]*schema.Message{msg})
	check(t, err)
	raw, err := json.Marshal(input)
	check(t, err)
	expected := strings.Replace(responsesProjectionInputJSON, `{"type":"output_text","text":"A","annotations":[{"type":"future_annotation","value":9007199254740993}]},`, "", 1)
	equalRawJSON(t, []byte(expected), raw)
}

func TestResponsesSignatureOnlyItemHasOnePart(t *testing.T) {
	msg := projectedResponsesMessage(t, "generate", `[{"type":"reasoning","id":"r","status":"completed","encrypted_content":"cipher"}]`)
	equal(t, 1, len(msg.AssistantGenMultiContent))
	part := &msg.AssistantGenMultiContent[0]
	equal(t, schema.ChatMessagePartTypeReasoning, part.Type)
	equal(t, true, hasResponsesItemMetadata(part))
	equal(t, "cipher", part.Reasoning.Signature)
	input, err := toResponsesInput([]*schema.Message{msg})
	check(t, err)
	reasoning, ok := input[0].OneOf.GetItemReasoning()
	equal(t, true, ok)
	equal(t, "cipher", reasoning.EncryptedContent.Or(""))
	equal(t, 0, len(reasoning.Summary))
}

func TestResponsesProjectionUsesSDKFields(t *testing.T) {
	output := `[
 {"type":"message","id":"m","role":"assistant","status":"completed","partial":true,"phase":"final_answer","extension":"not-in-sdk","content":[{"type":"output_text","text":"answer","extension":"not-in-sdk","annotations":[]}]},
 {"type":"web_search_call","id":"search","status":"completed","action":{"type":"search","query":"sdk query","queries":["not-in-sdk"]}},
 {"type":"function_call","id":"fc","call_id":"call","name":"inspect","namespace":"sdk_namespace","arguments":"{}","status":"completed"},
 {"type":"future_output","payload":{"integer":9007199254740993}}
]`
	for _, mode := range []string{"generate", "stream"} {
		t.Run(mode, func(t *testing.T) {
			original := projectedResponsesMessage(t, mode, output)
			for _, codec := range messagePersistenceCodecs() {
				t.Run(codec.name, func(t *testing.T) {
					raw, err := codec.marshal(original)
					check(t, err)
					var msg schema.Message
					check(t, codec.unmarshal(raw, &msg))
					raw, err = json.Marshal(msg)
					check(t, err)
					for _, unsupported := range []string{"phase", "queries", "not-in-sdk", "ark-responses-item-raw.", "ark-responses-content-raw."} {
						if strings.Contains(string(raw), unsupported) {
							t.Fatalf("projected field outside SDK schema: %s", unsupported)
						}
					}
					equal(t, 4, len(msg.AssistantGenMultiContent))
					id, _ := extraString(msg.AssistantGenMultiContent[1].Extra, keyResponsesItemID)
					status, _ := extraString(msg.AssistantGenMultiContent[1].Extra, keyResponsesItemStatus)
					equal(t, "search", id)
					equal(t, "completed", status)
					search, ok := extraString(msg.AssistantGenMultiContent[1].Extra, keyResponsesItemData)
					equal(t, true, ok)
					equalRawJSON(t, []byte(`{"type":"web_search_call","id":"search","status":"completed","action":{"type":"search","query":"sdk query"}}`), []byte(search))
					unknown, ok := extraString(msg.AssistantGenMultiContent[3].Extra, keyResponsesItemData)
					equal(t, true, ok)
					equalRawJSON(t, []byte(`{"type":"future_output","payload":{"integer":9007199254740993}}`), []byte(unknown))
					input, err := toResponsesInput([]*schema.Message{&msg})
					check(t, err)
					raw, err = json.Marshal(input)
					check(t, err)
					equalRawJSON(t, []byte(`[
 {"type":"message","id":"m","role":"assistant","partial":true,"content":[{"type":"output_text","text":"answer","annotations":[]}]},
 {"type":"function_call","id":"fc","call_id":"call","name":"inspect","namespace":"sdk_namespace","arguments":"{}","status":"completed"}
]`), raw)
				})
			}
		})
	}
}

func TestResponsesStreamingPartMetadata(t *testing.T) {
	output := `[
 {"type":"reasoning","id":"r","status":"completed","summary":[{"type":"summary_text","text":"S1"},{"type":"summary_text","text":"S2"}],"content":[{"type":"reasoning_text","text":"R1"},{"type":"reasoning_text","text":"R2"}],"encrypted_content":"cipher"},
 {"type":"message","id":"m1","role":"assistant","status":"completed","content":[{"type":"output_text","text":"A"},{"type":"output_text","text":"B"}]},
 {"type":"message","id":"m2","role":"assistant","status":"completed","content":[{"type":"output_text","text":"C"}]},
 {"type":"function_call","call_id":"call","name":"inspect","arguments":"{}","status":"completed"}
]`
	var compact bytes.Buffer
	check(t, json.Compact(&compact, []byte(output)))
	output = compact.String()
	generated := projectedResponsesMessage(t, "generate", output)
	for _, part := range generated.AssistantGenMultiContent {
		if part.StreamingMeta != nil {
			t.Fatal("Generate returned part streaming metadata")
		}
	}
	wantInput, err := toResponsesInput([]*schema.Message{generated})
	check(t, err)
	wantInputJSON, err := json.Marshal(wantInput)
	check(t, err)
	for _, ending := range []string{"completed", "incomplete"} {
		t.Run(ending, func(t *testing.T) {
			cm := testResponsesModel(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				for _, delta := range []struct {
					event string
					data  map[string]any
				}{
					{"response.reasoning_text.delta", map[string]any{"output_index": 0, "content_index": 0, "item_id": "r", "delta": "R1"}},
					{"response.output_text.delta", map[string]any{"output_index": 1, "content_index": 0, "item_id": "m1", "delta": "A"}},
					{"response.output_text.delta", map[string]any{"output_index": 1, "content_index": 1, "item_id": "m1", "delta": "B"}},
				} {
					_, _ = io.WriteString(w, responseEvent(delta.event, delta.data))
				}
				response := fmt.Sprintf(`{"id":"parts","created_at":1,"object":"response","model":"test","status":%q,"output":%s}`, ending, output)
				_, _ = io.WriteString(w, responseEvent("response."+ending, map[string]any{"response": json.RawMessage(response)}))
			}, nil)
			sr, err := cm.Stream(context.Background(), []*schema.Message{schema.UserMessage("hello")})
			check(t, err)
			defer sr.Close()
			var chunks []*schema.Message
			for {
				chunk, err := sr.Recv()
				if errors.Is(err, io.EOF) {
					break
				}
				check(t, err)
				chunks = append(chunks, chunk)
			}
			equal(t, 4, len(chunks))
			equal(t, 1, len(chunks[0].AssistantGenMultiContent))
			equal(t, "R1", chunks[0].AssistantGenMultiContent[0].Reasoning.Text)
			for _, chunk := range chunks[1 : len(chunks)-1] {
				equal(t, 0, len(chunk.AssistantGenMultiContent))
			}
			terminal := chunks[len(chunks)-1]
			equal(t, 8, len(terminal.AssistantGenMultiContent))
			for index, part := range terminal.AssistantGenMultiContent {
				if part.StreamingMeta == nil {
					t.Fatal("terminal stream part has no merge identity")
				}
				equal(t, index, part.StreamingMeta.Index)
			}
			merged, err := schema.ConcatMessages(chunks)
			check(t, err)
			equal(t, "ABC", merged.Content)
			equal(t, "S1\nS2", merged.ReasoningContent)
			equal(t, 8, len(merged.AssistantGenMultiContent))
			for index, want := range []string{"R1", "R2", "S1", "S2", "A", "B", "C"} {
				part := merged.AssistantGenMultiContent[index]
				if part.Reasoning != nil {
					equal(t, want, part.Reasoning.Text)
				} else {
					equal(t, want, part.Text)
				}
			}
			equal(t, "cipher", merged.AssistantGenMultiContent[2].Reasoning.Signature)
			input, err := toResponsesInput([]*schema.Message{merged})
			check(t, err)
			raw, err := json.Marshal(input)
			check(t, err)
			equalRawJSON(t, wantInputJSON, raw)
		})
	}
}

func TestResponsesUnknownContentRemainsOpaque(t *testing.T) {
	const unknown = `{"type":"future_content","payload":{"note":"SDK unknown variant"}}`
	for _, mode := range []string{"generate", "stream"} {
		for _, kind := range []string{"message", "reasoning"} {
			t.Run(mode+"/"+kind, func(t *testing.T) {
				contentType := "output_text"
				if kind == "reasoning" {
					contentType = "reasoning_text"
				}
				output := fmt.Sprintf(`[{"type":%q,"id":"item","role":"assistant","status":"completed","content":[{"type":%q,"text":"A"},%s,{"type":%q,"text":"B"}]}]`, kind, contentType, unknown, contentType)
				msg := projectedResponsesMessage(t, mode, output)
				equal(t, 3, len(msg.AssistantGenMultiContent))
				part := msg.AssistantGenMultiContent[1]
				equal(t, ResponsesOutputItem, part.Type)
				equal(t, "", part.Text)
				if part.Reasoning != nil {
					t.Fatal("unknown SDK content was projected as reasoning text")
				}
				data, ok := extraString(part.Extra, keyResponsesContentData)
				equal(t, true, ok)
				equalRawJSON(t, []byte(unknown), []byte(data))
				_, err := toResponsesInput([]*schema.Message{msg})
				if err == nil {
					t.Fatal("unknown content was converted to supported SDK input")
				}
			})
		}
	}
}

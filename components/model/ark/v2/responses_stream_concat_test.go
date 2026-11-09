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
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/responses"
)

func TestResponsesCapturedStreamConcat(t *testing.T) {
	for _, name := range []string{"text", "reasoning", "tools"} {
		t.Run(name, func(t *testing.T) {
			wire, err := os.ReadFile("testdata/responses_stream/" + name + ".sse")
			check(t, err)
			expected, err := os.ReadFile("testdata/responses_stream/" + name + ".json")
			check(t, err)
			cm := testResponsesModel(t, func(w http.ResponseWriter, r *http.Request) {
				equal(t, true, decodeRequest(t, r)["stream"])
				w.Header().Set("Content-Type", "text/event-stream")
				w.Header().Set("X-Request-Id", "fixture-request-"+name)
				_, _ = w.Write(wire)
			}, nil)
			for _, concat := range []string{"messages", "stream reader"} {
				t.Run(concat, func(t *testing.T) {
					reader, err := cm.Stream(context.Background(), []*schema.Message{schema.UserMessage("replay fixture")})
					check(t, err)
					var merged *schema.Message
					if concat == "stream reader" {
						merged, err = schema.ConcatMessageStream(reader)
					} else {
						defer reader.Close()
						var chunks []*schema.Message
						incremental := false
						identities := make(map[string]int)
						for {
							chunk, e := reader.Recv()
							if errors.Is(e, io.EOF) {
								break
							}
							check(t, e)
							chunks = append(chunks, chunk)
							for _, part := range chunk.AssistantGenMultiContent {
								if part.StreamingMeta == nil {
									t.Fatal("part chunk has no merge index")
								}
								item, _, e := extraIndex(part.Extra, keyResponsesItemIndex)
								check(t, e)
								key := fmt.Sprint(item, "/", responsesPartIdentity(part))
								if index, exists := identities[key]; exists {
									equal(t, index, part.StreamingMeta.Index)
								} else {
									identities[key] = part.StreamingMeta.Index
								}
							}
							if chunk.ResponseMeta == nil && (chunk.Content != "" || chunk.ReasoningContent != "") && len(chunk.AssistantGenMultiContent) > 0 {
								incremental = true
							}
						}
						if name != "tools" && !incremental {
							t.Fatal("stream did not expose incremental text/reasoning parts")
						}
						merged, err = schema.ConcatMessages(chunks)
					}
					check(t, err)
					raw, err := sonic.Marshal(merged)
					check(t, err)
					equalRawJSON(t, expected, raw)
					signatures := 0
					for index, part := range merged.AssistantGenMultiContent {
						if part.StreamingMeta == nil {
							t.Fatal("merged part has no index")
						}
						equal(t, index, part.StreamingMeta.Index)
						if part.Reasoning != nil && part.Reasoning.Signature != "" {
							signatures++
						}
					}
					if name == "reasoning" {
						equal(t, 1, signatures)
					} else {
						equal(t, 0, signatures)
					}
					inputs, err := toResponsesInput([]*schema.Message{merged})
					check(t, err)
					if name == "reasoning" {
						equal(t, 2, len(inputs))
						reasoning, ok := inputs[0].OneOf.GetItemReasoning()
						equal(t, true, ok)
						equal(t, 1, len(reasoning.Summary))
						equal(t, "fixture-encrypted-reasoning", reasoning.EncryptedContent.Or(""))
					} else {
						equal(t, 1, len(inputs))
					}
				})
			}
		})
	}
}

func resolveResponsesTestEvent(t *testing.T, state *responsesStreamState, raw string) (*schema.Message, error) {
	t.Helper()
	var event responses.ResponseStreamEvent
	raw = strings.Replace(raw, "{", `{"sequence_number":0,"item_id":"r",`, 1)
	check(t, sonic.UnmarshalString(raw, &event))
	msg, _, err := state.resolve(&event, &responses.ResponsesRequest{})
	return msg, err
}

func TestResponsesEmptyAddedPartKeepsItsIndex(t *testing.T) {
	var state responsesStreamState
	var chunks []*schema.Message
	for _, raw := range []string{
		`{"type":"response.content_part.added","output_index":0,"content_index":0,"part":{"type":"reasoning_text","text":""}}`,
		`{"type":"response.reasoning_summary_text.delta","output_index":0,"summary_index":1,"delta":"CD"}`,
		`{"type":"response.reasoning_summary_text.delta","output_index":0,"summary_index":0,"delta":"AB"}`,
		`{"type":"response.reasoning_text.delta","output_index":0,"content_index":0,"delta":"R"}`,
		`{"type":"response.output_item.done","output_index":0,"item":{"type":"reasoning","id":"r","status":"completed","encrypted_content":"cipher","summary":[{"type":"summary_text","text":"AB"},{"type":"summary_text","text":"CD"}],"content":[{"type":"reasoning_text","text":"R"}]}}`,
	} {
		msg, err := resolveResponsesTestEvent(t, &state, raw)
		check(t, err)
		if msg != nil {
			chunks = append(chunks, msg)
		}
	}
	equal(t, 0, chunks[0].AssistantGenMultiContent[0].StreamingMeta.Index)
	equal(t, "", chunks[0].AssistantGenMultiContent[0].Reasoning.Text)
	merged, err := schema.ConcatMessages(chunks)
	check(t, err)
	equal(t, 3, len(merged.AssistantGenMultiContent))
	for index, text := range []string{"R", "AB", "CD"} {
		part := merged.AssistantGenMultiContent[index]
		equal(t, index, part.StreamingMeta.Index)
		equal(t, text, part.Reasoning.Text)
	}
	equal(t, "AB\nCD", merged.ReasoningContent)
	equal(t, "cipher", merged.AssistantGenMultiContent[1].Reasoning.Signature)
	equal(t, "", merged.AssistantGenMultiContent[2].Reasoning.Signature)
	kind, _ := extraString(merged.AssistantGenMultiContent[0].Extra, keyResponsesItemType)
	equal(t, "reasoning", kind)
	inputs, err := toResponsesInput([]*schema.Message{merged})
	check(t, err)
	equal(t, 1, len(inputs))
	r, ok := inputs[0].OneOf.GetItemReasoning()
	equal(t, true, ok)
	equal(t, 2, len(r.Summary))
	equal(t, "AB", r.Summary[0].Text.Or(""))
	equal(t, "CD", r.Summary[1].Text.Or(""))
	equal(t, "cipher", r.EncryptedContent.Or(""))
}

func TestResponsesContradictoryPartEvents(t *testing.T) {
	const delta = `{"type":"response.output_text.delta","output_index":0,"content_index":0,"delta":"A"}`
	const done = `{"type":"response.output_item.done","output_index":0,"item":{"type":"message","id":"m","role":"assistant","status":"completed","content":[{"type":"output_text","text":"A"}]}}`
	for _, tc := range []struct {
		name   string
		events []string
	}{
		{"snapshot prefix", []string{delta, `{"type":"response.content_part.done","output_index":0,"content_index":0,"part":{"type":"output_text","text":"B"}}`}},
		{"buffered prefix", []string{delta, `{"type":"response.output_text.delta","output_index":0,"content_index":1,"delta":"B"}`, strings.Replace(done, `"text":"A"}`, `"text":"A"},{"type":"output_text","text":"C"}`, 1)}},
		{"delta after done", []string{done, delta}},
		{"changed terminal", []string{done, `{"type":"response.completed","response":{"id":"r","object":"response","created_at":1,"model":"test","status":"completed","output":[{"type":"message","id":"m","role":"assistant","status":"completed","content":[{"type":"output_text","text":"B"}]}]}}`}},
		{"omitted item", []string{strings.Replace(delta, `"output_index":0`, `"output_index":1`, 1), `{"type":"response.incomplete","response":{"id":"r","object":"response","created_at":1,"model":"test","status":"incomplete","output":[]}}`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var state responsesStreamState
			for _, raw := range tc.events[:len(tc.events)-1] {
				_, err := resolveResponsesTestEvent(t, &state, raw)
				check(t, err)
			}
			_, err := resolveResponsesTestEvent(t, &state, tc.events[len(tc.events)-1])
			if err == nil {
				t.Fatal("contradictory stream events silently accepted")
			}
		})
	}
}

func TestResponsesContentKinds(t *testing.T) {
	for _, itemType := range []string{"message", "reasoning"} {
		contentType := "output_text"
		if itemType == "reasoning" {
			contentType = "reasoning_text"
		}
		for _, mode := range []string{"generate", "stream"} {
			t.Run(itemType+"/"+mode, func(t *testing.T) {
				output := fmt.Sprintf(`[{"type":%q,"id":"item","role":"assistant","status":"completed","summary":[],"content":[{"type":%q,"text":"AB"}]}]`, itemType, contentType)
				msg := projectedResponsesMessage(t, mode, output)
				equal(t, 1, len(msg.AssistantGenMultiContent))
				part := msg.AssistantGenMultiContent[0]
				raw, err := sonic.Marshal(part)
				check(t, err)
				var fields map[string]any
				check(t, sonic.Unmarshal(raw, &fields))
				if itemType == "reasoning" {
					equal(t, schema.ChatMessagePartTypeReasoning, part.Type)
					equal(t, "", part.Text)
					equal(t, "AB", part.Reasoning.Text)
					if _, exists := fields["text"]; exists {
						t.Fatal("reasoning JSON has a top-level text field")
					}
					equal(t, "", msg.Content)
					equal(t, "", msg.ReasoningContent)
				} else {
					equal(t, schema.ChatMessagePartTypeText, part.Type)
					equal(t, "AB", part.Text)
					if _, exists := fields["reasoning"]; exists {
						t.Fatal("text JSON has a reasoning field")
					}
					equal(t, "AB", msg.Content)
					equal(t, "", msg.ReasoningContent)
				}
			})
		}
	}
}

func TestResponsesRejectContradictoryContent(t *testing.T) {
	for _, itemType := range []string{"message", "reasoning"} {
		contentType, wrongType := "output_text", "reasoning_text"
		if itemType == "reasoning" {
			contentType, wrongType = wrongType, contentType
		}
		t.Run(itemType, func(t *testing.T) {
			item := fmt.Sprintf(`{"type":%q,"id":"item","role":"assistant","status":"completed","summary":[],"content":[{"type":%q,"text":"AB"}]}`, itemType, wrongType)
			t.Run("generate", func(t *testing.T) {
				var value responses.OutputItem
				check(t, sonic.UnmarshalString(item, &value))
				_, err := projectResponsesItem(value, 0)
				if err == nil {
					t.Fatal("contradictory item/content types accepted")
				}
			})
			for stage, event := range map[string]string{
				"part added": fmt.Sprintf(`{"type":"response.content_part.added","output_index":0,"content_index":0,"part":{"type":%q,"text":""}}`, wrongType),
				"part done":  fmt.Sprintf(`{"type":"response.content_part.done","output_index":0,"content_index":0,"part":{"type":%q,"text":"AB"}}`, wrongType),
				"delta":      fmt.Sprintf(`{"type":"response.%s.delta","output_index":0,"content_index":0,"delta":"AB"}`, wrongType),
				"item added": fmt.Sprintf(`{"type":"response.output_item.added","output_index":0,"item":%s}`, item),
				"item done":  fmt.Sprintf(`{"type":"response.output_item.done","output_index":0,"item":%s}`, item),
				"terminal":   fmt.Sprintf(`{"type":"response.completed","response":{"id":"r","object":"response","created_at":1,"model":"test","status":"completed","output":[%s]}}`, item),
			} {
				t.Run("stream/"+stage, func(t *testing.T) {
					var state responsesStreamState
					_, err := resolveResponsesTestEvent(t, &state, fmt.Sprintf(`{"type":"response.output_item.added","output_index":0,"item":{"type":%q,"id":"item","role":"assistant","status":"in_progress","summary":[],"content":[]}}`, itemType))
					check(t, err)
					_, err = resolveResponsesTestEvent(t, &state, event)
					if err == nil {
						t.Fatal("contradictory stream content accepted")
					}
				})
			}
			for _, edit := range []string{"wire type", "part type"} {
				t.Run("history/"+edit, func(t *testing.T) {
					valid := strings.Replace(item, wrongType, contentType, 1)
					msg := projectedResponsesMessage(t, "generate", "["+valid+"]")
					part := &msg.AssistantGenMultiContent[0]
					if edit == "wire type" {
						part.Extra[keyResponsesContentType] = metadataString(wrongType)
					} else if itemType == "message" {
						part.Type, part.Text = schema.ChatMessagePartTypeReasoning, ""
						part.Reasoning = &schema.MessageOutputReasoning{Text: "AB"}
					} else {
						part.Type, part.Text, part.Reasoning = schema.ChatMessagePartTypeText, "AB", nil
					}
					if _, err := toResponsesInput([]*schema.Message{msg}); err == nil {
						t.Fatal("contradictory native history accepted")
					}
				})
			}
		})
	}
}

func TestResponsesCapturedStreamMissingTerminal(t *testing.T) {
	wire, err := os.ReadFile("testdata/responses_stream/text.sse")
	check(t, err)
	prefix, _, found := strings.Cut(string(wire), "event: response.completed\n")
	equal(t, true, found)
	cm := testResponsesModel(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, prefix)
	}, nil)
	reader, err := cm.Stream(context.Background(), []*schema.Message{schema.UserMessage("truncated fixture")})
	check(t, err)
	_, err = schema.ConcatMessageStream(reader)
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("expected incomplete stream error, got %v", err)
	}
}

func TestResponsesSummaryIndicesAndOneSignature(t *testing.T) {
	for _, delivery := range []string{"item done", "terminal", "added", "item done only"} {
		t.Run(delivery, func(t *testing.T) {
			reasoning := map[string]any{"type": "reasoning", "id": "reasoning", "status": "completed", "summary": []map[string]any{{"type": "summary_text", "text": "AB"}, {"type": "summary_text", "text": "CD"}}, "content": []any{}, "encrypted_content": "cipher"}
			message := map[string]any{"type": "message", "id": "message", "status": "completed", "role": "assistant", "content": []map[string]any{{"type": "output_text", "text": "E"}, {"type": "output_text", "text": "F"}}}
			cm := testResponsesModel(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				send := func(kind string, data map[string]any) { _, _ = io.WriteString(w, responseEvent(kind, data)) }
				added := map[string]any{"type": "reasoning", "id": "reasoning", "status": "in_progress", "summary": []any{}, "content": []any{}}
				if delivery == "added" {
					added["encrypted_content"] = "cipher"
				}
				send("response.output_item.added", map[string]any{"output_index": 0, "item": added})
				for index, text := range []string{"AB", "CD"} {
					send("response.reasoning_summary_part.added", map[string]any{"output_index": 0, "summary_index": index, "item_id": "reasoning", "part": map[string]any{"type": "summary_text", "text": ""}})
					for _, delta := range text {
						send("response.reasoning_summary_text.delta", map[string]any{"output_index": 0, "summary_index": index, "item_id": "reasoning", "delta": string(delta)})
					}
					send("response.reasoning_summary_part.done", map[string]any{"output_index": 0, "summary_index": index, "item_id": "reasoning", "part": map[string]any{"type": "summary_text", "text": text}})
				}
				done := make(map[string]any)
				for key, value := range reasoning {
					done[key] = value
				}
				if delivery == "terminal" || delivery == "added" {
					delete(done, "encrypted_content")
				}
				send("response.output_item.done", map[string]any{"output_index": 0, "item": done})
				if delivery == "added" || delivery == "item done only" {
					delete(reasoning, "encrypted_content")
				}
				send("response.output_item.added", map[string]any{"output_index": 1, "item": map[string]any{"type": "message", "id": "message", "role": "assistant", "status": "in_progress", "content": []any{}}})
				for index, text := range []string{"E", "F"} {
					send("response.content_part.added", map[string]any{"output_index": 1, "content_index": index, "item_id": "message", "part": map[string]any{"type": "output_text", "text": ""}})
					send("response.output_text.delta", map[string]any{"output_index": 1, "content_index": index, "item_id": "message", "delta": text})
					send("response.content_part.done", map[string]any{"output_index": 1, "content_index": index, "item_id": "message", "part": map[string]any{"type": "output_text", "text": text}})
				}
				send("response.output_item.done", map[string]any{"output_index": 1, "item": message})
				send("response.completed", map[string]any{"response": map[string]any{"id": "response", "created_at": 1, "object": "response", "model": "test", "status": "completed", "output": []any{reasoning, message}}})
			}, nil)
			reader, err := cm.Stream(context.Background(), []*schema.Message{schema.UserMessage("multiple parts")})
			check(t, err)
			defer reader.Close()
			var chunks []*schema.Message
			counts := make(map[int]int)
			for {
				chunk, e := reader.Recv()
				if errors.Is(e, io.EOF) {
					break
				}
				check(t, e)
				chunks = append(chunks, chunk)
				for _, part := range chunk.AssistantGenMultiContent {
					item, _, e := extraIndex(part.Extra, keyResponsesItemIndex)
					check(t, e)
					key := responsesPartIdentity(part)
					expected := key.index
					if item == 1 {
						expected += 2
					}
					equal(t, expected, part.StreamingMeta.Index)
					counts[expected]++
				}
			}
			if counts[0] < 4 {
				t.Fatal("added, delta and done did not share the summary part index")
			}
			merged, err := schema.ConcatMessages(chunks)
			check(t, err)
			equal(t, "AB\nCD", merged.ReasoningContent)
			equal(t, 4, len(merged.AssistantGenMultiContent))
			equal(t, "AB", merged.AssistantGenMultiContent[0].Reasoning.Text)
			equal(t, "CD", merged.AssistantGenMultiContent[1].Reasoning.Text)
			signatures := 0
			for _, part := range merged.AssistantGenMultiContent {
				if part.Reasoning != nil && part.Reasoning.Signature != "" {
					signatures++
				}
			}
			equal(t, 1, signatures)
			equal(t, "cipher", merged.AssistantGenMultiContent[0].Reasoning.Signature)
			inputs, err := toResponsesInput([]*schema.Message{merged})
			check(t, err)
			equal(t, 2, len(inputs))
			r, ok := inputs[0].OneOf.GetItemReasoning()
			equal(t, true, ok)
			equal(t, 2, len(r.Summary))
			equal(t, "AB", r.Summary[0].Text.Or(""))
			equal(t, "CD", r.Summary[1].Text.Or(""))
			equal(t, "cipher", r.EncryptedContent.Or(""))
		})
	}
}

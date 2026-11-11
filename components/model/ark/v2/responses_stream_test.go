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

func TestResponsesStreamConcat(t *testing.T) {
	for _, name := range []string{"text", "reasoning", "tools"} {
		t.Run(name, func(t *testing.T) {
			const message = `{"type":"message","id":"m","role":"assistant","status":"completed","content":[{"type":"output_text","text":"hello"}]}`
			const reasoning = `{"type":"reasoning","id":"r","status":"completed","summary":[{"type":"summary_text","text":"think"}],"encrypted_content":"fixture-encrypted-reasoning"}`
			const call = `{"type":"function_call","id":"fc","call_id":"call","name":"add","arguments":"{\"a\":8,\"b\":13}","status":"completed"}`
			content, summary, tools := "hello", "", 0
			wire := responseEvent("response.output_text.delta", map[string]any{"output_index": 0, "content_index": 0, "item_id": "m", "delta": "hello"}) + terminalEvent("response.completed", []byte("["+message+"]"))
			switch name {
			case "reasoning":
				summary = "think"
				wire = responseEvent("response.reasoning_summary_text.delta", map[string]any{"output_index": 0, "summary_index": 0, "item_id": "r", "delta": summary}) +
					responseEvent("response.output_text.delta", map[string]any{"output_index": 1, "content_index": 0, "item_id": "m", "delta": content}) +
					terminalEvent("response.completed", []byte("["+reasoning+","+message+"]"))
			case "tools":
				content, tools = "", 1
				wire = responseEvent("response.output_item.added", map[string]any{"output_index": 0, "item": map[string]any{"type": "function_call", "id": "fc", "call_id": "call", "name": "add", "arguments": "", "status": "in_progress"}}) +
					responseEvent("response.function_call_arguments.delta", map[string]any{"output_index": 0, "item_id": "fc", "delta": `{"a":8,`}) +
					responseEvent("response.function_call_arguments.delta", map[string]any{"output_index": 0, "item_id": "fc", "delta": `"b":13}`}) +
					terminalEvent("response.completed", []byte("["+call+"]"))
			}
			cm := testResponsesModel(t, func(w http.ResponseWriter, r *http.Request) {
				equal(t, true, decodeRequest(t, r)["stream"])
				w.Header().Set("Content-Type", "text/event-stream")
				w.Header().Set("X-Request-Id", "fixture-request-"+name)
				_, _ = io.WriteString(w, wire)
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
					equal(t, content, merged.Content)
					equal(t, summary, merged.ReasoningContent)
					equal(t, tools, len(merged.ToolCalls))
					equal(t, 11, merged.ResponseMeta.Usage.TotalTokens)
					if tools > 0 {
						equal(t, 0, *merged.ToolCalls[0].Index)
						equal(t, "call", merged.ToolCalls[0].ID)
						equal(t, `{"a":8,"b":13}`, merged.ToolCalls[0].Function.Arguments)
					}
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

func TestResponsesStreamMissingTerminal(t *testing.T) {
	prefix := responseEvent("response.output_text.delta", map[string]any{"output_index": 0, "content_index": 0, "item_id": "m", "delta": "partial"})
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

// This hand-authored protocol fixture deliberately interleaves two functions
// with reasoning and message deltas. It is not a captured model response.
func TestResponsesMixedStreamConcat(t *testing.T) {
	wire := mixedResponsesTestStream(t)
	prefix, last, ok := strings.Cut(wire, "event: response.completed\n")
	equal(t, true, ok)
	var terminal struct {
		Response responses.Response `json:"response"`
	}
	check(t, sonic.UnmarshalString(strings.TrimSpace(strings.TrimPrefix(last, "data: ")), &terminal))
	for _, status := range []string{"completed", "incomplete"} {
		for _, concat := range []string{"messages", "reader"} {
			t.Run(status+"/"+concat, func(t *testing.T) {
				response := terminal.Response
				if status == "incomplete" {
					response.Status = responses.NewOptResponseStatus(responses.ResponseStatusIncomplete)
				}
				responseJSON, err := sonic.Marshal(&response)
				check(t, err)
				streamWire := wire
				if status == "incomplete" {
					streamWire = prefix + responseEvent("response.incomplete", map[string]any{"response": json.RawMessage(responseJSON)})
				}
				cm := testResponsesModel(t, func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("X-Request-Id", "fixture-mixed-request")
					if decodeRequest(t, r)["stream"] == true {
						w.Header().Set("Content-Type", "text/event-stream")
						_, _ = io.WriteString(w, streamWire)
					} else {
						w.Header().Set("Content-Type", "application/json")
						_, _ = w.Write(responseJSON)
					}
				}, nil)
				input := []*schema.Message{schema.UserMessage("Replay the mixed stream fixture.")}
				reader, err := cm.Stream(context.Background(), input)
				check(t, err)
				var merged *schema.Message
				if concat == "reader" {
					merged, err = schema.ConcatMessageStream(reader)
				} else {
					defer reader.Close()
					var chunks []*schema.Message
					identities := make(map[string]int)
					var lastIndex int
					seenIndex := false
					partialTools := map[int]bool{}
					for {
						msg, e := reader.Recv()
						if errors.Is(e, io.EOF) {
							break
						}
						check(t, e)
						chunks = append(chunks, msg)
						for _, part := range msg.AssistantGenMultiContent {
							if part.StreamingMeta == nil {
								t.Fatal("part has no merge identity")
							}
							native, _, e := extraIndex(part.Extra, keyResponsesItemIndex)
							check(t, e)
							key := fmt.Sprint(native, "/", responsesPartIdentity(part))
							index := part.StreamingMeta.Index
							if previous, exists := identities[key]; exists {
								equal(t, previous, index)
							}
							identities[key] = index
							if seenIndex && index < lastIndex {
								t.Fatal("part indices returned to a closed part")
							}
							lastIndex, seenIndex = index, true
						}
						for _, call := range msg.ToolCalls {
							if msg.ResponseMeta == nil && call.Function.Arguments != "" {
								partialTools[*call.Index] = true
							}
						}
						if msg.ResponseMeta == nil {
							for _, part := range msg.AssistantGenMultiContent {
								if part.Reasoning != nil && part.Reasoning.Signature != "" {
									t.Fatal("fixture signature arrived before its terminal event")
								}
							}
						}
					}
					equal(t, 6, len(identities))
					equal(t, true, partialTools[0])
					equal(t, true, partialTools[1])
					merged, err = schema.ConcatMessages(chunks)
				}
				check(t, err)
				equal(t, "I will call both tools.\nWaiting for results.", merged.Content)
				equal(t, "Add first.\nMultiply second.", merged.ReasoningContent)
				equal(t, status, merged.ResponseMeta.FinishReason)
				equal(t, 130, merged.ResponseMeta.Usage.TotalTokens)
				equal(t, 6, len(merged.AssistantGenMultiContent))
				wantNative := []string{"0/summary/0", "0/summary/1", "0/content/0", "1/content/0", "1/content/1", "4/content/0"}
				wantText := []string{"Add first.", "Multiply second.", "Check independent operations.", "I will ", "call both tools.", "\nWaiting for results."}
				signatures := 0
				for index, part := range merged.AssistantGenMultiContent {
					native, _, e := extraIndex(part.Extra, keyResponsesItemIndex)
					check(t, e)
					key := responsesPartIdentity(part)
					equal(t, wantNative[index], fmt.Sprintf("%d/%s/%d", native, key.section, key.index))
					equal(t, index, part.StreamingMeta.Index)
					equal(t, wantText[index], responsesPartText(part))
					if part.Reasoning != nil && part.Reasoning.Signature != "" {
						signatures++
						equal(t, 0, index)
						equal(t, "fixture-encrypted-reasoning", part.Reasoning.Signature)
					}
				}
				equal(t, 1, signatures)
				equal(t, 2, len(merged.ToolCalls))
				for i, want := range []struct {
					index      int
					name, args string
				}{{2, "add", `{"a":8,"b":13}`}, {3, "multiply", `{"a":6,"b":7}`}} {
					call := merged.ToolCalls[i]
					equal(t, i, *call.Index)
					native, _, err := extraIndex(call.Extra, keyResponsesItemIndex)
					check(t, err)
					equal(t, want.index, native)
					equal(t, want.name, call.Function.Name)
					equal(t, want.args, call.Function.Arguments)
					equal(t, "completed", fmt.Sprint(call.Extra[keyResponsesItemStatus]))
				}
				equal(t, "math", fmt.Sprint(merged.ToolCalls[0].Extra[keyResponsesNamespace]))
				// Generate uses the same native identities but no stream indices.
				full, err := cm.Generate(context.Background(), input)
				check(t, err)
				for _, part := range full.AssistantGenMultiContent {
					if part.StreamingMeta != nil {
						t.Fatal("Generate has a stream index")
					}
				}
				want, err := toResponsesInput([]*schema.Message{full})
				check(t, err)
				wantJSON, err := json.Marshal(want)
				check(t, err)
				equal(t, 5, len(want))
				for _, codec := range messagePersistenceCodecs() {
					restored := roundTripHistory(t, codec, []*schema.Message{merged})
					actual, err := toResponsesInput(restored)
					check(t, err)
					actualJSON, err := json.Marshal(actual)
					check(t, err)
					equalRawJSON(t, wantJSON, actualJSON)
					reasoning := actual[0].OneOf.ItemReasoning
					equal(t, 2, len(reasoning.Summary))
					equal(t, 1, len(reasoning.Content))
					equal(t, "fixture-encrypted-reasoning", reasoning.EncryptedContent.Or(""))
					equal(t, "call-add", actual[2].OneOf.ItemFunctionToolCall.CallID.Or(""))
					equal(t, "call-multiply", actual[3].OneOf.ItemFunctionToolCall.CallID.Or(""))
				}
			})
		}
	}
}

// Build only the protocol events needed by this regression, without recorded
// model responses or external test data files.
func mixedResponsesTestStream(t *testing.T) string {
	t.Helper()
	const reasoning = `{"type":"reasoning","id":"fixture-reason","status":"completed","summary":[{"type":"summary_text","text":"Add first."},{"type":"summary_text","text":"Multiply second."}],"content":[{"type":"reasoning_text","text":"Check independent operations.","annotations":[]}]}`
	const message = `{"type":"message","id":"fixture-message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"I will ","annotations":[]},{"type":"output_text","text":"call both tools.","annotations":[]}]}`
	const add = `{"type":"function_call","id":"fixture-add","call_id":"call-add","name":"add","arguments":"{\"a\":8,\"b\":13}","status":"completed","namespace":"math"}`
	const multiply = `{"type":"function_call","id":"fixture-multiply","call_id":"call-multiply","name":"multiply","arguments":"{\"a\":6,\"b\":7}","status":"completed"}`
	const tail = `{"type":"message","id":"fixture-tail","role":"assistant","status":"completed","content":[{"type":"output_text","text":"\nWaiting for results.","annotations":[]}]}`
	var wire strings.Builder
	sequence := 0
	emit := func(kind string, fields map[string]any) {
		fields["type"], fields["sequence_number"] = kind, sequence
		sequence++
		raw, err := sonic.Marshal(fields)
		check(t, err)
		fmt.Fprintf(&wire, "event: %s\ndata: %s\n\n", kind, raw)
	}
	ids := map[int]string{0: "fixture-reason", 1: "fixture-message", 2: "fixture-add", 3: "fixture-multiply", 4: "fixture-tail"}
	added := func(index int, full string) {
		var item map[string]any
		check(t, sonic.UnmarshalString(full, &item))
		item["status"] = "in_progress"
		switch item["type"] {
		case "reasoning":
			item["summary"], item["content"] = []any{}, []any{}
		case "message":
			item["content"] = []any{}
		case "function_call":
			item["arguments"] = ""
			delete(item, "namespace")
		}
		emit("response.output_item.added", map[string]any{"output_index": index, "item": item})
	}
	delta := func(kind string, index, subindex int, text string) {
		fields := map[string]any{"output_index": index, "item_id": ids[index], "delta": text}
		if subindex >= 0 {
			key := "content_index"
			if strings.Contains(kind, "summary") {
				key = "summary_index"
			}
			fields[key] = subindex
		}
		emit(kind, fields)
	}
	part := func(kind string, index, subindex int, value string) {
		key := "content_index"
		if strings.Contains(kind, "summary") {
			key = "summary_index"
		}
		emit(kind, map[string]any{"output_index": index, "item_id": ids[index], key: subindex, "part": json.RawMessage(value)})
	}
	done := func(index int, full string) {
		emit("response.output_item.done", map[string]any{"output_index": index, "item": json.RawMessage(full)})
	}
	added(0, reasoning)
	part("response.reasoning_summary_part.added", 0, 0, `{"type":"summary_text","text":""}`)
	delta("response.reasoning_summary_text.delta", 0, 0, "Add ")
	added(1, message)
	added(2, add)
	added(3, multiply)
	delta("response.output_text.delta", 1, 0, "I ")
	delta("response.function_call_arguments.delta", 2, -1, `{"a":`)
	delta("response.reasoning_summary_text.delta", 0, 0, "first.")
	delta("response.function_call_arguments.delta", 3, -1, `{"a":6,`)
	part("response.content_part.added", 0, 0, `{"type":"reasoning_text","text":""}`)
	delta("response.reasoning_text.delta", 0, 0, "Check ")
	part("response.reasoning_summary_part.added", 0, 1, `{"type":"summary_text","text":""}`)
	delta("response.reasoning_summary_text.delta", 0, 1, "Multiply second.")
	delta("response.output_text.delta", 1, 0, "will ")
	delta("response.function_call_arguments.delta", 2, -1, `8,"b":`)
	part("response.content_part.added", 1, 1, `{"type":"output_text","text":""}`)
	delta("response.output_text.delta", 1, 1, "call both ")
	delta("response.function_call_arguments.delta", 3, -1, `"b":7}`)
	part("response.reasoning_summary_part.done", 0, 0, `{"type":"summary_text","text":"Add first."}`)
	delta("response.reasoning_text.delta", 0, 0, "independent operations.")
	part("response.content_part.done", 0, 0, `{"type":"reasoning_text","text":"Check independent operations.","annotations":[]}`)
	part("response.reasoning_summary_part.done", 0, 1, `{"type":"summary_text","text":"Multiply second."}`)
	done(0, reasoning)
	done(2, add)
	delta("response.output_text.delta", 1, 1, "tools.")
	part("response.content_part.done", 1, 0, `{"type":"output_text","text":"I will ","annotations":[]}`)
	part("response.content_part.done", 1, 1, `{"type":"output_text","text":"call both tools.","annotations":[]}`)
	done(3, multiply)
	done(1, message)
	added(4, tail)
	delta("response.output_text.delta", 4, 0, "\nWaiting for results.")
	done(4, tail)
	var encrypted map[string]any
	check(t, sonic.UnmarshalString(reasoning, &encrypted))
	encrypted["encrypted_content"] = "fixture-encrypted-reasoning"
	emit("response.completed", map[string]any{"response": map[string]any{
		"id": "fixture-mixed-response", "object": "response", "created_at": 1, "model": "fixture-model", "status": "completed",
		"output": []any{encrypted, json.RawMessage(message), json.RawMessage(add), json.RawMessage(multiply), json.RawMessage(tail)},
		"usage": map[string]any{"input_tokens": 100, "input_tokens_details": map[string]any{"cached_tokens": 20}, "output_tokens": 30,
			"output_tokens_details": map[string]any{"reasoning_tokens": 10}, "total_tokens": 130},
	}})
	return wire.String()
}

func TestResponsesTerminalOutputReconciliation(t *testing.T) {
	for _, ending := range []string{"completed", "incomplete"} {
		for _, events := range []string{"terminal only", "done only", "prefix", "all deltas"} {
			for _, codec := range messagePersistenceCodecs() {
				t.Run(ending+"/"+events+"/"+codec.name, func(t *testing.T) {
					cm := testResponsesModel(t, func(w http.ResponseWriter, r *http.Request) {
						body := decodeRequest(t, r)
						if body["stream"] != true {
							input := body["input"].([]any)
							equal(t, 3, len(input))
							equal(t, "reasoning", input[0].(map[string]any)["type"])
							equal(t, "cipher", input[0].(map[string]any)["encrypted_content"])
							equal(t, "call-1", input[2].(map[string]any)["call_id"])
							writeResponses(w)
							return
						}
						w.Header().Set("Content-Type", "text/event-stream")
						if events == "prefix" || events == "all deltas" {
							text, args := "do", "{"
							if events == "all deltas" {
								text, args = "done", "{}"
							}
							_, _ = io.WriteString(w, responseEvent("response.output_item.added", map[string]any{"output_index": 2, "item": map[string]any{"type": "function_call", "id": "fc-1", "call_id": "call-1", "name": "inspect", "arguments": args}}))
							_, _ = io.WriteString(w, responseEvent("response.output_text.delta", map[string]any{"output_index": 1, "content_index": 0, "item_id": "msg-1", "delta": text}))
						}
						if events == "done only" {
							_, _ = io.WriteString(w, responseEvent("response.output_item.done", map[string]any{"output_index": 2, "item": map[string]any{"type": "function_call", "id": "fc-1", "call_id": "call-1", "name": "inspect", "arguments": "{}", "status": "completed"}}))
						}
						response := strings.ReplaceAll(responsesJSON, `"status":"completed"`, fmt.Sprintf(`"status":%q`, ending))
						_, _ = io.WriteString(w, responseEvent("response."+ending, map[string]any{"response": json.RawMessage(response)}))
					}, nil)
					first := callResponsesWithOptions(t, cm, "stream", []*schema.Message{schema.UserMessage("first")})
					equal(t, "done", first.Content)
					equal(t, "summary", first.ReasoningContent)
					equal(t, 1, len(first.ToolCalls))
					equal(t, "inspect", first.ToolCalls[0].Function.Name)
					equal(t, "{}", first.ToolCalls[0].Function.Arguments)
					history := roundTripHistory(t, codec, []*schema.Message{first})
					_, err := cm.Generate(context.Background(), history)
					check(t, err)
				})
			}
		}
	}
}

func TestResponsesEmptyCompletedStream(t *testing.T) {
	cm := testResponsesModel(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, responseEvent("response.completed", map[string]any{"response": json.RawMessage(`{"id":"empty","created_at":1,"object":"response","model":"test","status":"completed","output":[]}`)}))
	}, nil)
	sr, err := cm.Stream(context.Background(), []*schema.Message{schema.UserMessage("first")})
	check(t, err)
	_, err = readResponsesStream(t, sr)
	if !errors.Is(err, ErrEmptyResponse) {
		t.Fatalf("expected ErrEmptyResponse, got %v", err)
	}
}

func TestResponsesSummaryDisplay(t *testing.T) {
	for _, ending := range []string{"completed", "incomplete"} {
		for _, tc := range []struct {
			name, output, want string
			rawDeltas          []string
			summary            string
		}{
			{"raw prefix", `{"summary":[],"content":[{"type":"reasoning_text","text":"raw thought"}]}`, "", []string{"raw"}, ""},
			{"summary prefix", `{"summary":[{"type":"summary_text","text":"one"},{"type":"summary_text","text":"two"}]}`, "one\ntwo", nil, "one"},
			{"complete", `{"content":[{"type":"reasoning_text","text":"raw thought"}]}`, "", []string{"raw thought"}, ""},
			{"mixed", `{"summary":[{"type":"summary_text","text":"summary"}],"content":[{"type":"reasoning_text","text":"raw thought"}]}`, "summary", []string{"raw"}, "sum"},
			{"identical summary and raw", `{"summary":[{"type":"summary_text","text":"same"}],"content":[{"type":"reasoning_text","text":"same"}]}`, "same", []string{"same"}, "sa"},
			{"summary only at terminal", `{"summary":[{"type":"summary_text","text":"one"},{"type":"summary_text","text":"two"}],"content":[{"type":"reasoning_text","text":"raw"}]}`, "one\ntwo", []string{"raw"}, ""},
			{"empty summary segments", `{"summary":[{"type":"summary_text","text":""},{"type":"summary_text","text":"one"},{"type":"summary_text","text":""},{"type":"summary_text","text":"two"}]}`, "one\ntwo", nil, ""},
			{"summary absent at terminal", `{"summary":[],"encrypted_content":"cipher"}`, "sum", []string{"raw"}, "sum"},
			{"no terminal plaintext", `{"summary":[],"encrypted_content":"cipher"}`, "", []string{"raw"}, ""},
			{"different flattened order", `{"content":[{"type":"reasoning_text","text":"A1"},{"type":"reasoning_text","text":"B"}]}`, "", []string{"A", "B"}, ""},
		} {
			t.Run(ending+"/"+tc.name, func(t *testing.T) {
				cm := testResponsesModel(t, func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "text/event-stream")
					if tc.summary != "" {
						_, _ = io.WriteString(w, responseEvent("response.reasoning_summary_text.delta", map[string]any{"item_id": "rs-1", "output_index": 0, "summary_index": 0, "delta": tc.summary}))
					}
					for index, delta := range tc.rawDeltas {
						_, _ = io.WriteString(w, responseEvent("response.reasoning_text.delta", map[string]any{"item_id": "rs-1", "output_index": 0, "content_index": index, "delta": delta}))
					}
					var reasoning map[string]json.RawMessage
					check(t, json.Unmarshal([]byte(tc.output), &reasoning))
					reasoning["type"] = json.RawMessage(`"reasoning"`)
					reasoning["id"] = json.RawMessage(`"rs-1"`)
					reasoning["status"] = json.RawMessage(`"completed"`)
					_, _ = io.WriteString(w, responseEvent("response."+ending, map[string]any{"response": map[string]any{
						"id": "resp-1", "created_at": 1, "object": "response", "model": "test", "status": ending, "output": []any{reasoning},
					}}))
				}, nil)
				msg := callResponsesWithOptions(t, cm, "stream", []*schema.Message{schema.UserMessage("first")})
				equal(t, tc.want, msg.ReasoningContent)
				if len(msg.AssistantGenMultiContent) == 0 {
					t.Fatal("missing canonical terminal parts")
				}
				inputs, err := toResponsesInput([]*schema.Message{msg})
				check(t, err)
				reasoning, ok := inputs[0].OneOf.GetItemReasoning()
				equal(t, true, ok)
				if len(tc.rawDeltas) > 0 && len(reasoning.Content) == 0 {
					t.Fatal("raw reasoning was lost from canonical history")
				}
			})
		}
	}
}

func TestResponsesRawReasoningDeltasDoNotPopulateDisplay(t *testing.T) {
	for _, kind := range []string{"reasoning_text", "reasoning_raw_text"} {
		t.Run(kind, func(t *testing.T) {
			var state responsesStreamState
			msg, err := resolveResponsesTestEvent(t, &state, fmt.Sprintf(`{"type":"response.%s.delta","output_index":0,"content_index":0,"delta":"raw thought"}`, kind))
			check(t, err)
			equal(t, "", msg.ReasoningContent)
			equal(t, 1, len(msg.AssistantGenMultiContent))
			equal(t, "raw thought", msg.AssistantGenMultiContent[0].Reasoning.Text)
		})
	}
}

func TestResponsesChunkModifierRetainsRaw(t *testing.T) {
	var saved [][]byte
	cm := testResponsesModel(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for _, delta := range []string{"a", "b"} {
			_, _ = io.WriteString(w, responseEvent("response.output_text.delta", map[string]any{"output_index": 1, "content_index": 0, "item_id": "msg-1", "delta": delta}))
		}
		response := strings.ReplaceAll(responsesJSON, `"done"`, `"ab"`)
		_, _ = io.WriteString(w, responseEvent("response.completed", map[string]any{"response": json.RawMessage(response)}))
	}, &ResponsesAPIConfig{ResponseChunkMessageModifier: func(_ context.Context, msg *schema.Message, raw []byte, end bool) (*schema.Message, error) {
		if !end {
			saved = append(saved, raw)
		}
		return msg, nil
	}})
	callResponsesWithOptions(t, cm, "stream", []*schema.Message{schema.UserMessage("first")})
	for i, delta := range []string{"a", "b"} {
		var event struct{ Delta string }
		check(t, json.Unmarshal(saved[i], &event))
		equal(t, delta, event.Delta)
	}
}

func TestResponsesStreamToolIdentityIsAtomic(t *testing.T) {
	for _, field := range []string{"call_id", "name"} {
		t.Run(field, func(t *testing.T) {
			cm := testResponsesModel(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				call := map[string]any{"type": "function_call", "id": "fc-1", "call_id": "call-1", "name": "inspect", "arguments": "{}"}
				call[field] = "prefix"
				_, _ = io.WriteString(w, responseEvent("response.output_item.added", map[string]any{"output_index": 2, "item": call}))
				_, _ = io.WriteString(w, responseEvent("response.completed", map[string]any{"response": json.RawMessage(responsesJSON)}))
			}, nil)
			sr, err := cm.Stream(context.Background(), []*schema.Message{schema.UserMessage("first")})
			check(t, err)
			_, err = readResponsesStream(t, sr)
			if err == nil || !strings.Contains(err.Error(), "differs from terminal output") {
				t.Fatalf("expected inconsistent identity error, got %v", err)
			}
		})
	}
}

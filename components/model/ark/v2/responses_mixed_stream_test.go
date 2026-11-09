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

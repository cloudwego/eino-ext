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
	"os"
	"strings"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/responses"
)

// This hand-authored protocol fixture deliberately interleaves two functions
// with reasoning and message deltas. It is not a captured model response.
func TestResponsesMixedStreamConcat(t *testing.T) {
	raw, err := os.ReadFile("testdata/responses_stream/mixed.sse")
	check(t, err)
	wire := string(raw)
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
					equal(t, true, partialTools[2])
					equal(t, true, partialTools[3])
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
					equal(t, want.index, *call.Index)
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

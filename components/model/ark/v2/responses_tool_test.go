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
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/responses"
)

func TestResponsesToolDefinitionsWire(t *testing.T) {
	var parameters jsonschema.Schema
	check(t, json.Unmarshal([]byte(`{"type":"object","properties":{"value":{"anyOf":[{"type":"integer"},{"type":"null"}]}},"required":["value"],"additionalProperties":false,"$defs":{"label":{"type":"string","pattern":"^[a-z]+$"}}}`), &parameters))
	parameters.Extras = map[string]any{"x-sequence": json.Number("9007199254740993")}
	expectedParameters, err := json.Marshal(&parameters)
	check(t, err)
	for _, mode := range []string{"generate", "stream"} {
		for _, source := range []string{"bound", "option"} {
			for _, modified := range []bool{false, true} {
				for _, strict := range []struct {
					name  string
					value *bool
				}{{"unset", nil}, {"true", ptr(true)}, {"false", ptr(false)}} {
					name := mode + "/" + source + "/" + strict.name
					if modified {
						name += "/modifier"
					}
					t.Run(name, func(t *testing.T) {
						base := &schema.ToolInfo{Name: "inspect", Desc: "Inspect a value", ParamsOneOf: schema.NewParamsOneOfByJSONSchema(&parameters), Extra: map[string]any{"local-only": "metadata"}}
						tools := []*schema.ToolInfo{base, WithToolStrict(base, true), WithToolStrict(base, false), {Name: "no_args"}}
						tools[1].Name, tools[2].Name = "strict", "flexible"
						before, err := json.Marshal(tools)
						check(t, err)
						config := &ResponsesAPIConfig{ToolStrict: strict.value, EnableToolWebSearch: &responses.WebSearchTool{Limit: responses.NewOptInt64(2)}}
						if modified {
							config.RequestPayloadModifier = func(_ context.Context, _ []*schema.Message, raw []byte) ([]byte, error) { return raw, nil }
						}
						cm := testResponsesModel(t, func(w http.ResponseWriter, r *http.Request) {
							var body struct {
								Tools  []map[string]json.RawMessage
								Stream bool
							}
							check(t, json.NewDecoder(r.Body).Decode(&body))
							equal(t, 5, len(body.Tools))
							expectedStrict := []*bool{strict.value, ptr(true), ptr(false), strict.value}
							for i, tool := range body.Tools[:4] {
								equal(t, `"function"`, string(tool["type"]))
								var name string
								check(t, json.Unmarshal(tool["name"], &name))
								equal(t, tools[i].Name, name)
								if expectedStrict[i] == nil {
									equal(t, json.RawMessage(nil), tool["strict"])
								} else {
									raw, err := json.Marshal(*expectedStrict[i])
									check(t, err)
									equalRawJSON(t, raw, tool["strict"])
								}
								if i == 3 {
									equal(t, json.RawMessage(nil), tool["parameters"])
									equal(t, json.RawMessage(nil), tool["description"])
								} else {
									equalRawJSON(t, expectedParameters, tool["parameters"])
									equal(t, `"Inspect a value"`, string(tool["description"]))
								}
								equal(t, json.RawMessage(nil), tool["extra"])
								equal(t, json.RawMessage(nil), tool["local-only"])
							}
							equal(t, `"web_search"`, string(body.Tools[4]["type"]))
							equal(t, "2", string(body.Tools[4]["limit"]))
							writeResponsesHistoryReply(w, false, body.Stream)
						}, config)
						var opts []model.Option
						if source == "bound" {
							check(t, cm.BindTools(tools))
						} else {
							opts = append(opts, model.WithTools(tools))
						}
						callResponsesWithOptions(t, cm, mode, []*schema.Message{schema.UserMessage("hello")}, opts...)
						after, err := json.Marshal(tools)
						check(t, err)
						equalRawJSON(t, before, after)
					})
				}
			}
		}
	}
}

func TestResponsesToolDefinitionsPresence(t *testing.T) {
	for _, mode := range []string{"generate", "stream"} {
		for _, kind := range []string{"omitted", "cleared", "search", "cleared with search"} {
			t.Run(mode+"/"+kind, func(t *testing.T) {
				config := &ResponsesAPIConfig{}
				if kind == "search" || kind == "cleared with search" {
					config.EnableToolWebSearch = &responses.WebSearchTool{}
				}
				cm := testResponsesModel(t, func(w http.ResponseWriter, r *http.Request) {
					body := decodeRequest(t, r)
					switch kind {
					case "omitted":
						if _, exists := body["tools"]; exists {
							t.Error("unset tools must be omitted")
						}
					case "cleared":
						equal(t, []any{}, body["tools"])
					default:
						tools := body["tools"].([]any)
						equal(t, 1, len(tools))
						equal(t, "web_search", tools[0].(map[string]any)["type"])
					}
					writeResponsesHistoryReply(w, false, body["stream"] == true)
				}, config)
				var opts []model.Option
				if kind == "cleared" || kind == "cleared with search" {
					check(t, cm.BindTools([]*schema.ToolInfo{toolInfo("inspect")}))
					opts = append(opts, model.WithTools(nil))
				}
				callResponsesWithOptions(t, cm, mode, []*schema.Message{schema.UserMessage("hello")}, opts...)
			})
		}
	}
}

func TestResponsesInvalidToolsBeforeHTTP(t *testing.T) {
	cm := testResponsesModel(t, func(http.ResponseWriter, *http.Request) { t.Error("invalid tools reached HTTP") }, nil)
	for _, tools := range [][]*schema.ToolInfo{
		{nil},
		{{Name: "invalid", Extra: map[string]any{toolStrictKey: "true"}}},
		{{Name: "invalid", ParamsOneOf: schema.NewParamsOneOfByJSONSchema(jsonschema.FalseSchema)}},
	} {
		if _, err := cm.WithTools(tools); err == nil {
			t.Error("expected binding validation error")
		}
		for _, mode := range []string{"generate", "stream"} {
			var err error
			if mode == "generate" {
				_, err = cm.Generate(context.Background(), []*schema.Message{schema.UserMessage("hello")}, model.WithTools(tools))
			} else {
				var reader *schema.StreamReader[*schema.Message]
				reader, err = cm.Stream(context.Background(), []*schema.Message{schema.UserMessage("hello")}, model.WithTools(tools))
				if reader != nil {
					reader.Close()
				}
			}
			if err == nil {
				t.Error("expected request validation error")
			}
		}
	}
	_, err := cm.Generate(context.Background(), []*schema.Message{schema.UserMessage("hello")}, model.WithToolChoice(schema.ToolChoiceForced))
	if err == nil {
		t.Error("forced selection without tools must fail")
	}
}

func TestResponsesFunctionOnlyHistory(t *testing.T) {
	const output = `[
 {"type":"function_call","id":"fc-0","call_id":"call-0","name":"add","arguments":"{}","status":"completed","namespace":"math"},
 {"type":"function_call","id":"fc-1","call_id":"call-1","name":"inspect","arguments":"{}","status":"completed"}
]`
	for _, mode := range []string{"generate", "stream"} {
		original := projectedResponsesMessage(t, mode, output)
		equal(t, 0, len(original.AssistantGenMultiContent))
		equal(t, 2, len(original.ToolCalls))
		for _, codec := range messagePersistenceCodecs() {
			t.Run(mode+"/"+codec.name, func(t *testing.T) {
				messages := roundTripHistory(t, codec, []*schema.Message{original})
				msg := messages[0]
				input, err := toResponsesInput(messages)
				check(t, err)
				raw, err := json.Marshal(input)
				check(t, err)
				equalRawJSON(t, []byte(output), raw)
				// Reorder by native item index across both public arrays.
				msg.ToolCalls[0].Extra[keyResponsesItemIndex] = metadataInt64(3)
				input, err = toResponsesInput(messages)
				check(t, err)
				equal(t, "call-1", input[0].OneOf.ItemFunctionToolCall.CallID.Or(""))
				equal(t, "fc-0", input[1].OneOf.ItemFunctionToolCall.ID.Or(""))
				msg.ToolCalls = msg.ToolCalls[:1]
				input, err = toResponsesInput(messages)
				check(t, err)
				equal(t, 1, len(input))
				equal(t, "call-0", input[0].OneOf.ItemFunctionToolCall.CallID.Or(""))
			})
		}
	}
}

func TestResponsesToolCallMetadataValidation(t *testing.T) {
	const output = `[{"type":"function_call","id":"fc","call_id":"call","name":"add","arguments":"{}","status":"completed"}]`
	for _, edit := range []string{"missing index", "invalid index", "invalid type", "invalid id", "invalid status", "invalid namespace", "duplicate call", "overlapping part"} {
		t.Run(edit, func(t *testing.T) {
			msg := projectedResponsesMessage(t, "generate", output)
			extra := msg.ToolCalls[0].Extra
			switch edit {
			case "missing index":
				delete(extra, keyResponsesItemIndex)
			case "invalid index":
				extra[keyResponsesItemIndex] = -1
			case "invalid type":
				extra[keyResponsesItemType] = "message"
			case "invalid id":
				extra[keyResponsesItemID] = true
			case "invalid status":
				extra[keyResponsesItemStatus] = 1
			case "invalid namespace":
				extra[keyResponsesNamespace] = false
			case "duplicate call":
				msg.ToolCalls = append(msg.ToolCalls, msg.ToolCalls[0])
			case "overlapping part":
				msg.AssistantGenMultiContent = []schema.MessageOutputPart{{Type: schema.ChatMessagePartTypeText, Text: "overlap", Extra: extra}}
			}
			if _, err := toResponsesInput([]*schema.Message{msg}); err == nil {
				t.Fatal("invalid tool metadata accepted")
			}
		})
	}
}

func TestResponsesToolSnapshotsAndContentSlots(t *testing.T) {
	for _, terminalArgs := range []bool{true, false} {
		t.Run(fmt.Sprint("terminal arguments=", terminalArgs), func(t *testing.T) {
			call0 := `{"type":"function_call","id":"fc-0","call_id":"call-0","name":"add","arguments":"{}","namespace":"math","status":"completed"}`
			call2 := `{"type":"function_call","id":"fc-2","call_id":"call-2","name":"inspect","arguments":"[]","status":"completed"}`
			message := `{"type":"message","id":"m","role":"assistant","status":"completed","content":[{"type":"output_text","text":"A"}]}`
			output := "[" + call0 + "," + message + "," + call2 + "]"
			terminal := output
			if !terminalArgs {
				terminal = strings.ReplaceAll(terminal, `,"arguments":"{}"`, "")
				terminal = strings.ReplaceAll(terminal, `,"arguments":"[]"`, "")
			}
			var state responsesStreamState
			var chunks []*schema.Message
			for _, event := range []string{
				`{"type":"response.output_item.added","output_index":0,"item":{"type":"function_call","call_id":"call-0","name":"add","arguments":"","status":"in_progress"}}`,
				`{"type":"response.output_item.added","output_index":1,"item":{"type":"message","id":"m","role":"assistant","status":"in_progress","content":[]}}`,
				`{"type":"response.output_item.added","output_index":2,"item":{"type":"function_call","call_id":"call-2","name":"inspect","arguments":"","status":"in_progress"}}`,
				`{"type":"response.function_call_arguments.delta","output_index":2,"delta":"["}`,
				`{"type":"response.function_call_arguments.delta","output_index":0,"delta":"{"}`,
				`{"type":"response.output_text.delta","output_index":1,"content_index":0,"delta":"A"}`,
				`{"type":"response.output_item.done","output_index":2,"item":` + call2 + `}`,
				`{"type":"response.output_item.done","output_index":0,"item":` + call0 + `}`,
				`{"type":"response.output_item.done","output_index":1,"item":` + message + `}`,
				`{"type":"response.completed","response":{"id":"r","object":"response","created_at":1,"model":"test","status":"completed","output":` + terminal + `}}`,
			} {
				msg, err := resolveResponsesTestEvent(t, &state, event)
				check(t, err)
				if msg == nil {
					continue
				}
				for _, part := range msg.AssistantGenMultiContent {
					kind, _ := extraString(part.Extra, keyResponsesItemType)
					if kind == "function_call" {
						t.Fatal("function call created a content part")
					}
				}
				chunks = append(chunks, msg)
			}
			merged, err := schema.ConcatMessages(chunks)
			check(t, err)
			equal(t, "A", merged.Content)
			equal(t, 1, len(merged.AssistantGenMultiContent))
			equal(t, 0, merged.AssistantGenMultiContent[0].StreamingMeta.Index)
			equal(t, 2, len(merged.ToolCalls))
			for i, index := range []int{0, 2} {
				call := merged.ToolCalls[i]
				equal(t, i, *call.Index)
				native, _, err := extraIndex(call.Extra, keyResponsesItemIndex)
				check(t, err)
				equal(t, index, native)
				equal(t, "completed", fmt.Sprint(call.Extra[keyResponsesItemStatus]))
				equal(t, fmt.Sprint("fc-", index), fmt.Sprint(call.Extra[keyResponsesItemID]))
			}
			input, err := toResponsesInput([]*schema.Message{merged})
			check(t, err)
			raw, err := json.Marshal(input)
			check(t, err)
			expected := strings.Replace(output, `,"status":"completed","content"`, `,"content"`, 1)
			equalRawJSON(t, []byte(expected), raw)
		})
	}
}

func TestResponsesToolIndicesFollowFirstOccurrence(t *testing.T) {
	const add = `{"type":"function_call","id":"fc-add","call_id":"call-add","name":"add","arguments":"{\"a\":8,\"b\":13}","status":"completed"}`
	const multiply = `{"type":"function_call","id":"fc-multiply","call_id":"call-multiply","name":"multiply","arguments":"{\"a\":6,\"b\":7}","status":"completed"}`
	message := func(id string) string {
		return fmt.Sprintf(`{"type":"message","id":%q,"role":"assistant","status":"completed","content":[]}`, id)
	}
	output := "[" + message("m0") + "," + message("m1") + "," + add + "," + message("m3") + "," + multiply + "]"
	var state responsesStreamState
	var chunks []*schema.Message
	for _, event := range []string{
		// The second native function arrives first, before its item metadata.
		`{"type":"response.function_call_arguments.delta","output_index":4,"delta":"{\"a\":6,"}`,
		`{"type":"response.output_item.added","output_index":2,"item":{"type":"function_call","id":"fc-add","call_id":"call-add","name":"add","arguments":"","status":"in_progress"}}`,
		`{"type":"response.output_item.added","output_index":4,"item":{"type":"function_call","id":"fc-multiply","call_id":"call-multiply","name":"multiply","status":"in_progress"}}`,
		`{"type":"response.function_call_arguments.delta","output_index":2,"delta":"{\"a\":8,\"b\":"}`,
		`{"type":"response.function_call_arguments.delta","output_index":4,"delta":"\"b\":7}"}`,
		`{"type":"response.output_item.done","output_index":2,"item":` + add + `}`,
		`{"type":"response.output_item.done","output_index":4,"item":` + multiply + `}`,
		`{"type":"response.completed","response":{"id":"r","object":"response","created_at":1,"model":"test","status":"completed","output":` + output + `}}`,
	} {
		msg, err := resolveResponsesTestEvent(t, &state, event)
		check(t, err)
		if msg == nil {
			continue
		}
		for _, call := range msg.ToolCalls {
			native, _, err := extraIndex(call.Extra, keyResponsesItemIndex)
			check(t, err)
			equal(t, map[int]int{4: 0, 2: 1}[native], *call.Index)
		}
		chunks = append(chunks, msg)
	}
	merged, err := schema.ConcatMessages(chunks)
	check(t, err)
	equal(t, 2, len(merged.ToolCalls))
	for index, want := range []struct{ id, args string }{{"call-multiply", `{"a":6,"b":7}`}, {"call-add", `{"a":8,"b":13}`}} {
		call := merged.ToolCalls[index]
		equal(t, index, *call.Index)
		equal(t, want.id, call.ID)
		equal(t, want.args, call.Function.Arguments)
	}
	// Native item indices restore SDK order, independently of Eino tool order.
	for _, codec := range messagePersistenceCodecs() {
		restored := roundTripHistory(t, codec, []*schema.Message{merged})
		input, err := toResponsesInput(restored)
		check(t, err)
		equal(t, 5, len(input))
		equal(t, "call-add", input[2].OneOf.ItemFunctionToolCall.CallID.Or(""))
		equal(t, "call-multiply", input[4].OneOf.ItemFunctionToolCall.CallID.Or(""))
	}
}

func TestResponsesToolIndicesFromTerminalOnly(t *testing.T) {
	const output = `[
 {"type":"message","id":"m0","role":"assistant","status":"completed","content":[{"type":"output_text","text":"before"}]},
 {"type":"function_call","id":"fc-a","call_id":"call-a","name":"add","arguments":"{}","status":"completed"},
 {"type":"message","id":"m2","role":"assistant","status":"completed","content":[{"type":"output_text","text":"between"}]},
 {"type":"function_call","id":"fc-b","call_id":"call-b","name":"multiply","arguments":"{}","status":"completed"}
]`
	stream := projectedResponsesMessage(t, "stream", output)
	full := projectedResponsesMessage(t, "generate", output)
	for index, native := range []int{1, 3} {
		equal(t, index, *stream.ToolCalls[index].Index)
		actual, _, err := extraIndex(stream.ToolCalls[index].Extra, keyResponsesItemIndex)
		check(t, err)
		equal(t, native, actual)
		if full.ToolCalls[index].Index != nil {
			t.Fatal("Generate has a tool stream index")
		}
	}
}

func TestResponsesForcedBuiltInTools(t *testing.T) {
	for _, mode := range []string{"generate", "stream"} {
		for _, kind := range []string{"function only", "web only", "web and function", "named function", "cached", "cached named"} {
			t.Run(mode+"/"+kind, func(t *testing.T) {
				config := &ResponsesAPIConfig{EnableToolWebSearch: &responses.WebSearchTool{}}
				if kind == "function only" {
					config.EnableToolWebSearch = nil
				}
				cm := testResponsesModel(t, func(w http.ResponseWriter, r *http.Request) {
					body := decodeRequest(t, r)
					if strings.HasPrefix(kind, "cached") {
						equal(t, nil, body["tool_choice"])
					} else if kind == "named function" {
						equal(t, map[string]any{"type": "function", "name": "inspect"}, body["tool_choice"])
					} else {
						equal(t, "required", body["tool_choice"])
					}
					if strings.HasPrefix(kind, "cached") {
						if _, exists := body["tools"]; exists {
							t.Error("cached request resent definitions")
						}
					}
					writeResponsesHistoryReply(w, false, body["stream"] == true)
				}, config)
				opts := []model.Option{model.WithToolChoice(schema.ToolChoiceForced)}
				if kind == "function only" || kind == "web and function" || kind == "named function" {
					check(t, cm.BindTools([]*schema.ToolInfo{toolInfo("inspect")}))
				}
				if kind == "named function" {
					opts = []model.Option{model.WithToolChoice(schema.ToolChoiceForced, "inspect")}
				}
				if kind == "cached" {
					// These local definitions are not sent; the server already
					// owns the definitions associated with the cached response.
					opts = append(opts, WithCache(&CacheOption{HeadPreviousResponseID: ptr("cached")}), model.WithTools([]*schema.ToolInfo{{Name: "ignored", Extra: map[string]any{toolStrictKey: "unused"}}}))
				}
				if kind == "cached named" {
					opts = []model.Option{WithCache(&CacheOption{HeadPreviousResponseID: ptr("cached")}), model.WithToolChoice(schema.ToolChoiceForced, "inspect")}
				}
				callResponsesWithOptions(t, cm, mode, []*schema.Message{schema.UserMessage("first")}, opts...)
			})
		}
	}
}

func TestResponsesAllowedNamesWithBuiltInAndCache(t *testing.T) {
	for _, mode := range []string{"generate", "stream"} {
		for _, choice := range []schema.ToolChoice{schema.ToolChoiceAllowed, schema.ToolChoiceForced} {
			t.Run(mode+"/"+string(choice), func(t *testing.T) {
				requests := make(chan map[string]any, 1)
				cm := testResponsesModel(t, func(w http.ResponseWriter, r *http.Request) {
					body := decodeRequest(t, r)
					requests <- body
					writeResponsesHistoryReply(w, false, body["stream"] == true)
				}, &ResponsesAPIConfig{EnableToolWebSearch: &responses.WebSearchTool{}})
				check(t, cm.BindTools([]*schema.ToolInfo{toolInfo("inspect"), toolInfo("other")}))
				callResponsesWithOptions(t, cm, mode, []*schema.Message{schema.UserMessage("first")}, model.WithToolChoice(choice, "inspect", "other"))
				body := <-requests
				tools := body["tools"].([]any)
				equal(t, 2, len(tools))
				for _, tool := range tools {
					equal(t, "function", tool.(map[string]any)["type"])
				}
				// Cached definitions belong to the server; even unusable local
				// schemas must not be converted when continuing the same subset.
				opts := []model.Option{
					WithCache(&CacheOption{HeadPreviousResponseID: ptr("cached")}),
					model.WithToolChoice(choice, "inspect", "other"),
					model.WithTools([]*schema.ToolInfo{
						{Name: "inspect", Extra: map[string]any{toolStrictKey: "unused"}},
						{Name: "other", Extra: map[string]any{toolStrictKey: "unused"}},
					}),
				}
				callResponsesWithOptions(t, cm, mode, []*schema.Message{schema.UserMessage("next")}, opts...)
				cached := <-requests
				equal(t, "cached", cached["previous_response_id"])
				if _, exists := cached["tools"]; exists {
					t.Fatal("cached request resent definitions")
				}
				equal(t, nil, cached["tool_choice"])
			})
		}
	}
}

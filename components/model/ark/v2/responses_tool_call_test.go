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
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
)

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

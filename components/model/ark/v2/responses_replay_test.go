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
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/cloudwego/eino/schema"
)

const responsesReplayOutputJSON = `[
  {"type":"function_call","id":"fc-1","call_id":"call-1","name":"inspect","arguments":"{}","status":"completed","extension":{"sequence":9007199254740993}},
  {"type":"reasoning","id":"rs-1","status":"completed","summary":[{"type":"summary_text","text":"thought"}],"encrypted_content":"cipher","extension":"reasoning-extension"},
  {"type":"message","id":"msg-1","role":"assistant","status":"completed","content":[{"type":"output_text","text":"done","annotations":[{"type":"url_citation","url":"https://example.com","title":"source","start_index":0,"end_index":4}],"extension":"text-extension"}],"extension":"message-extension"},
  {"type":"future_output","id":"future-1","payload":{"sequence":9007199254740993}}
]`

func equalRawJSON(t *testing.T, want, got []byte) {
	t.Helper()
	values := make([]any, 2)
	for i, raw := range [][]byte{want, got} {
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		check(t, decoder.Decode(&values[i]))
	}
	equal(t, values[0], values[1])
}

func TestResponsesReplayOriginalOutput(t *testing.T) {
	for _, mode := range []string{"generate", "stream"} {
		for _, codec := range messagePersistenceCodecs() {
			for _, modifier := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/modifier=%t", mode, codec.name, modifier), func(t *testing.T) {
					var calls atomic.Int32
					requests := make(chan []json.RawMessage, 2)
					config := &ResponsesAPIConfig{ToolStrict: ptr(true)}
					if modifier {
						config.RequestPayloadModifier = func(_ context.Context, _ []*schema.Message, raw []byte) ([]byte, error) { return raw, nil }
					}
					cm := testResponsesModel(t, func(w http.ResponseWriter, r *http.Request) {
						var body struct {
							Input  []json.RawMessage
							Stream bool
						}
						if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
							t.Error(err)
							return
						}
						requests <- body.Input
						if calls.Add(1) != 1 {
							writeResponsesHistoryReply(w, false, body.Stream)
							return
						}
						response := fmt.Sprintf(`{"id":"replay-1","created_at":1,"object":"response","model":"test","status":"completed","output":%s}`, responsesReplayOutputJSON)
						var compact bytes.Buffer
						if err := json.Compact(&compact, []byte(response)); err != nil {
							t.Error(err)
							return
						}
						response = compact.String()
						if !body.Stream {
							w.Header().Set("Content-Type", "application/json")
							_, _ = io.WriteString(w, response)
							return
						}
						w.Header().Set("Content-Type", "text/event-stream")
						_, _ = io.WriteString(w, responseEvent("response.output_item.added", map[string]any{"output_index": 0, "item": map[string]any{"type": "function_call", "id": "fc-1", "call_id": "call-1", "name": "inspect", "arguments": ""}}))
						_, _ = io.WriteString(w, responseEvent("response.function_call_arguments.delta", map[string]any{"output_index": 0, "item_id": "fc-1", "delta": "{}"}))
						_, _ = io.WriteString(w, responseEvent("response.reasoning_summary_text.delta", map[string]any{"output_index": 1, "summary_index": 0, "item_id": "rs-1", "delta": "thought"}))
						_, _ = io.WriteString(w, responseEvent("response.output_text.delta", map[string]any{"output_index": 2, "content_index": 0, "item_id": "msg-1", "delta": "done"}))
						_, _ = io.WriteString(w, responseEvent("response.completed", map[string]any{"response": json.RawMessage(response)}))
					}, config)
					check(t, cm.BindTools([]*schema.ToolInfo{toolInfo("inspect")}))
					user := schema.UserMessage("inspect")
					first := callResponsesWithOptions(t, cm, mode, []*schema.Message{user})
					<-requests
					history := roundTripHistory(t, codec, []*schema.Message{user, first, {Role: schema.Tool, ToolCallID: "call-1", Content: "tool result"}, schema.UserMessage("follow-up")})
					reply := callResponsesWithOptions(t, cm, mode, history)
					equal(t, "continued", reply.Content)
					input := <-requests
					var output []json.RawMessage
					check(t, json.Unmarshal([]byte(responsesReplayOutputJSON), &output))
					equal(t, len(output)+3, len(input))
					for i := range output {
						equalRawJSON(t, output[i], input[i+1])
					}
					saved, ok := extraString(history[1], keyResponsesOutput)
					equal(t, true, ok)
					equalRawJSON(t, []byte(responsesReplayOutputJSON), []byte(saved))
				})
			}
		}
	}
}

func TestResponsesOutputEdits(t *testing.T) {
	for _, field := range []string{"content", "reasoning", "tool"} {
		t.Run(field, func(t *testing.T) {
			requests := make(chan map[string]any, 2)
			cm := testResponsesModel(t, func(w http.ResponseWriter, r *http.Request) {
				requests <- decodeRequest(t, r)
				writeResponses(w)
			}, &ResponsesAPIConfig{EnableReasoningContentPassback: true})
			first, err := cm.Generate(context.Background(), []*schema.Message{schema.UserMessage("first")})
			check(t, err)
			<-requests
			switch field {
			case "content":
				first.Content = "edited content"
			case "reasoning":
				first.ReasoningContent = "edited reasoning"
			case "tool":
				first.ToolCalls[0].Function.Arguments = `{"path":"edited"}`
			}
			_, err = cm.Generate(context.Background(), []*schema.Message{first})
			check(t, err)
			input := (<-requests)["input"].([]any)
			switch field {
			case "content":
				equal(t, "edited content", input[1].(map[string]any)["content"])
			case "reasoning":
				reasoning := input[0].(map[string]any)
				equal(t, "edited reasoning", reasoning["summary"].([]any)[0].(map[string]any)["text"])
				if _, exists := reasoning["encrypted_content"]; exists {
					t.Fatal("stale encrypted reasoning restored")
				}
			case "tool":
				equal(t, `{"path":"edited"}`, input[2].(map[string]any)["arguments"])
			}
		})
	}
}

func TestResponsesReplayByDefault(t *testing.T) {
	requests := make(chan []json.RawMessage, 2)
	cm := testResponsesModel(t, func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Input []json.RawMessage }
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			return
		}
		requests <- body.Input
		writeResponses(w)
	}, nil)
	first, err := cm.Generate(context.Background(), []*schema.Message{schema.UserMessage("first")})
	check(t, err)
	<-requests
	_, err = cm.Generate(context.Background(), []*schema.Message{first})
	check(t, err)
	input := <-requests
	saved, ok := extraString(first, keyResponsesOutput)
	equal(t, true, ok)
	var output []json.RawMessage
	check(t, json.Unmarshal([]byte(saved), &output))
	equal(t, len(output), len(input))
	for i := range output {
		equalRawJSON(t, output[i], input[i])
	}
}

func TestAssistantReasoningOutputParts(t *testing.T) {
	message := &schema.Message{Role: schema.Assistant, AssistantGenMultiContent: []schema.MessageOutputPart{
		{Type: schema.ChatMessagePartTypeText, Text: "before"},
		{Type: schema.ChatMessagePartTypeReasoning, Reasoning: &schema.MessageOutputReasoning{Text: "thought", Signature: "cipher"}},
		{Type: schema.ChatMessagePartTypeText, Text: "after"},
	}}
	t.Run("responses", func(t *testing.T) {
		requests := make(chan map[string]any, 1)
		cm := testResponsesModel(t, func(w http.ResponseWriter, r *http.Request) { requests <- decodeRequest(t, r); writeResponses(w) }, nil)
		_, err := cm.Generate(context.Background(), []*schema.Message{message})
		check(t, err)
		input := (<-requests)["input"].([]any)
		equal(t, 3, len(input))
		equal(t, "before", input[0].(map[string]any)["content"].([]any)[0].(map[string]any)["text"])
		equal(t, "reasoning", input[1].(map[string]any)["type"])
		equal(t, "cipher", input[1].(map[string]any)["encrypted_content"])
		equal(t, "after", input[2].(map[string]any)["content"].([]any)[0].(map[string]any)["text"])
	})
	t.Run("chat", func(t *testing.T) {
		requests := make(chan map[string]any, 1)
		cm := testModel(t, func(w http.ResponseWriter, r *http.Request) {
			requests <- decodeRequest(t, r)
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, responseJSON)
		}, nil)
		_, err := cm.Generate(context.Background(), []*schema.Message{message})
		check(t, err)
		assistant := (<-requests)["messages"].([]any)[0].(map[string]any)
		equal(t, "thought", assistant["reasoning_content"])
		equal(t, "cipher", assistant["encrypted_content"])
		equal(t, 2, len(assistant["content"].([]any)))
	})
}

func TestResponsesReplayWithSDKFilePreprocessing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "image.png")
	check(t, os.WriteFile(path, []byte("image fixture"), 0600))
	for _, modifier := range []bool{false, true} {
		t.Run(fmt.Sprint(modifier), func(t *testing.T) {
			var calls, uploads, retrieves atomic.Int32
			requests := make(chan map[string]any, 2)
			config := &ResponsesAPIConfig{EnableReasoningContentPassback: true}
			if modifier {
				config.RequestPayloadModifier = func(_ context.Context, _ []*schema.Message, raw []byte) ([]byte, error) { return raw, nil }
			}
			cm := testResponsesModel(t, func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/v3/files", "/api/v3/files/file-1":
					if r.Method == http.MethodPost {
						uploads.Add(1)
					} else {
						retrieves.Add(1)
					}
					w.Header().Set("Content-Type", "application/json")
					_, _ = io.WriteString(w, `{"id":"file-1","object":"file","bytes":13,"created_at":1,"expire_at":-1,"filename":"image.png","purpose":"user_data","status":"active"}`)
				case "/api/v3/responses":
					requests <- decodeRequest(t, r)
					calls.Add(1)
					writeResponses(w)
				default:
					t.Error("unexpected SDK request", r.URL.Path)
					http.NotFound(w, r)
				}
			}, config)
			first, err := cm.Generate(context.Background(), []*schema.Message{schema.UserMessage("first")})
			check(t, err)
			<-requests
			_, err = cm.Generate(context.Background(), []*schema.Message{first, {Role: schema.User, UserInputMultiContent: []schema.MessageInputPart{
				{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{URL: ptr("file://" + path)}}},
			}}})
			check(t, err)
			input := (<-requests)["input"].([]any)
			image := input[len(input)-1].(map[string]any)["content"].([]any)[0].(map[string]any)
			equal(t, "file-1", image["file_id"])
			if _, exists := image["image_url"]; exists {
				t.Fatal("SDK-preprocessed file URL restored")
			}
			equal(t, int32(1), uploads.Load())
			equal(t, int32(1), retrieves.Load())
			equal(t, int32(2), calls.Load())
		})
	}
}

func TestResponsesManualReasoningPassback(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprint(enabled), func(t *testing.T) {
			requests := make(chan map[string]any, 1)
			cm := testResponsesModel(t, func(w http.ResponseWriter, r *http.Request) { requests <- decodeRequest(t, r); writeResponses(w) }, &ResponsesAPIConfig{EnableReasoningContentPassback: enabled})
			_, err := cm.Generate(context.Background(), []*schema.Message{{Role: schema.Assistant, Content: "answer", ReasoningContent: "manual thought"}})
			check(t, err)
			input := (<-requests)["input"].([]any)
			if enabled {
				equal(t, 2, len(input))
				equal(t, "reasoning", input[0].(map[string]any)["type"])
				equal(t, "manual thought", input[0].(map[string]any)["summary"].([]any)[0].(map[string]any)["text"])
			} else {
				equal(t, 1, len(input))
			}
			equal(t, "answer", input[len(input)-1].(map[string]any)["content"])
		})
	}
}

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
	"strings"
	"sync/atomic"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/schema"
)

const responsesReplayOutputJSON = `[
  {"type":"function_call","id":"fc-1","call_id":"call-1","name":"inspect","arguments":"{}","status":"completed","extension":{"sequence":9007199254740993}},
  {"type":"reasoning","id":"rs-1","status":"completed","summary":[{"type":"summary_text","text":"thought"}],"encrypted_content":"cipher","extension":"reasoning-extension"},
  {"type":"message","id":"msg-1","role":"assistant","status":"completed","content":[{"type":"output_text","text":"done","annotations":[{"type":"url_citation","url":"https://example.com","title":"source","start_index":0,"end_index":4}],"extension":"text-extension"}],"extension":"message-extension"},
  {"type":"future_output","id":"future-1","payload":{"sequence":9007199254740993}}
]`

const responsesReplayInputJSON = `[
 {"type":"function_call","id":"fc-1","call_id":"call-1","name":"inspect","arguments":"{}","status":"completed"},
 {"type":"reasoning","id":"rs-1","status":"completed","summary":[{"type":"summary_text","text":"thought"}],"encrypted_content":"cipher"},
 {"type":"message","id":"msg-1","role":"assistant","content":[{"type":"output_text","text":"done","annotations":[{"type":"url_citation","url":"https://example.com","title":"source"}]}]}
]`

func responsesInputJSON(msg *schema.Message) (string, bool) {
	items, err := toResponsesInput([]*schema.Message{msg})
	if err != nil {
		return "", false
	}
	raw, err := json.Marshal(items)
	return string(raw), err == nil
}

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

func TestResponsesHistoryUsesSDKInputs(t *testing.T) {
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
					check(t, json.Unmarshal([]byte(responsesReplayInputJSON), &output))
					equal(t, len(output)+3, len(input))
					for i := range output {
						equalRawJSON(t, output[i], input[i+1])
					}
					saved, ok := responsesInputJSON(history[1])
					equal(t, true, ok)
					equalRawJSON(t, []byte(responsesReplayInputJSON), []byte(saved))
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
			}, &ResponsesAPIConfig{})
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
				equal(t, "done", input[1].(map[string]any)["content"].([]any)[0].(map[string]any)["text"])
			case "reasoning":
				reasoning := input[0].(map[string]any)
				equal(t, "summary", reasoning["summary"].([]any)[0].(map[string]any)["text"])
				equal(t, "cipher", reasoning["encrypted_content"])
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
	saved, ok := responsesInputJSON(first)
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
			config := &ResponsesAPIConfig{}
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

func TestResponsesReasoningInputDefaults(t *testing.T) {
	for _, mode := range []string{"generate", "stream"} {
		for _, structured := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/structured=%t", mode, structured), func(t *testing.T) {
				requests := make(chan map[string]any, 1)
				cm := testResponsesModel(t, func(w http.ResponseWriter, r *http.Request) {
					body := decodeRequest(t, r)
					requests <- body
					writeResponsesHistoryReply(w, false, body["stream"] == true)
				}, nil)
				msg := &schema.Message{Role: schema.Assistant, Content: "answer", ReasoningContent: "display thought"}
				if structured {
					msg.AssistantGenMultiContent = []schema.MessageOutputPart{
						{Type: schema.ChatMessagePartTypeReasoning, Reasoning: &schema.MessageOutputReasoning{Text: "explicit thought", Signature: "cipher"}},
						{Type: schema.ChatMessagePartTypeText, Text: "answer"},
					}
				}
				callResponsesWithOptions(t, cm, mode, []*schema.Message{msg})
				input := (<-requests)["input"].([]any)
				if structured {
					equal(t, 2, len(input))
					reasoning := input[0].(map[string]any)
					equal(t, "reasoning", reasoning["type"])
					equal(t, "cipher", reasoning["encrypted_content"])
					equal(t, "explicit thought", reasoning["summary"].([]any)[0].(map[string]any)["text"])
					equal(t, "answer", input[1].(map[string]any)["content"].([]any)[0].(map[string]any)["text"])
				} else {
					equal(t, 2, len(input))
					equal(t, "display thought", input[0].(map[string]any)["summary"].([]any)[0].(map[string]any)["text"])
					equal(t, "answer", input[1].(map[string]any)["content"])
				}
			})
		}
	}
}

func TestChatCannotMergeReasoningSignatures(t *testing.T) {
	cm := testModel(t, func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("multiple encrypted reasoning values reached Chat HTTP")
	}, nil)
	_, err := cm.Generate(context.Background(), []*schema.Message{{Role: schema.Assistant, AssistantGenMultiContent: []schema.MessageOutputPart{
		{Type: schema.ChatMessagePartTypeReasoning, Reasoning: &schema.MessageOutputReasoning{Text: "first", Signature: "first-cipher"}},
		{Type: schema.ChatMessagePartTypeReasoning, Reasoning: &schema.MessageOutputReasoning{Text: "second", Signature: "second-cipher"}},
	}}})
	if err == nil {
		t.Fatal("encrypted reasoning values must not be concatenated")
	}
}

func TestResponsesStandardAssistantHistory(t *testing.T) {
	for _, tc := range []struct {
		name string
		msg  *schema.Message
		want string
	}{
		{
			name: "flat reasoning and tools",
			msg: &schema.Message{Role: schema.Assistant, Content: "answer", ReasoningContent: "thought", ToolCalls: []schema.ToolCall{
				{Index: ptr(8), ID: "first", Function: schema.FunctionCall{Name: "inspect", Arguments: `{"number":9007199254740993}`}},
				{Index: ptr(1), ID: "second", Type: "function", Function: schema.FunctionCall{Name: "finish", Arguments: `{}`}},
			}},
			want: `[{"type":"reasoning","status":"completed","summary":[{"type":"summary_text","text":"thought"}]},
			 {"type":"message","role":"assistant","content":"answer"},
			 {"type":"function_call","call_id":"first","name":"inspect","arguments":"{\"number\":9007199254740993}"},
			 {"type":"function_call","call_id":"second","name":"finish","arguments":"{}"}]`,
		},
		{
			name: "ordered reasoning parts",
			msg: &schema.Message{Role: schema.Assistant, Content: "duplicate answer", ReasoningContent: "duplicate reasoning", AssistantGenMultiContent: []schema.MessageOutputPart{
				{Type: schema.ChatMessagePartTypeText, Text: "before", StreamingMeta: &schema.MessageStreamingMeta{Index: 9}},
				{Type: schema.ChatMessagePartTypeReasoning, Reasoning: &schema.MessageOutputReasoning{Text: "first thought", Signature: "first signature"}},
				{Type: schema.ChatMessagePartTypeText, Text: "between"},
				{Type: schema.ChatMessagePartTypeReasoning, Reasoning: &schema.MessageOutputReasoning{Text: "second thought", Signature: "second signature"}},
				{Type: schema.ChatMessagePartTypeText, Text: "after", StreamingMeta: &schema.MessageStreamingMeta{Index: 0}},
			}},
			want: `[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"before"}]},
			 {"type":"reasoning","status":"completed","summary":[{"type":"summary_text","text":"first thought"}],"encrypted_content":"first signature"},
			 {"type":"message","role":"assistant","content":[{"type":"output_text","text":"between"}]},
			 {"type":"reasoning","status":"completed","summary":[{"type":"summary_text","text":"second thought"}],"encrypted_content":"second signature"},
			 {"type":"message","role":"assistant","content":[{"type":"output_text","text":"after"}]}]`,
		},
		{
			name: "media with separate reasoning",
			msg: &schema.Message{Role: schema.Assistant, ReasoningContent: "media thought", AssistantGenMultiContent: []schema.MessageOutputPart{
				{Type: schema.ChatMessagePartTypeText, Text: "caption"},
				{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageOutputImage{MessagePartCommon: schema.MessagePartCommon{FileID: ptr("image-1")}}},
				{Type: schema.ChatMessagePartTypeAudioURL, Audio: &schema.MessageOutputAudio{MessagePartCommon: schema.MessagePartCommon{Base64Data: ptr("YXVkaW8="), MIMEType: "audio/wav"}}},
				{Type: schema.ChatMessagePartTypeVideoURL, Video: &schema.MessageOutputVideo{MessagePartCommon: schema.MessagePartCommon{URL: ptr("https://example.com/video")}}},
			}},
			want: `[{"type":"reasoning","status":"completed","summary":[{"type":"summary_text","text":"media thought"}]},
			 {"type":"message","role":"assistant","content":[{"type":"output_text","text":"caption"},
			 {"type":"input_image","file_id":"image-1"},
			 {"type":"input_audio","audio_url":"data:audio/wav;base64,YXVkaW8="},
			 {"type":"input_video","video_url":"https://example.com/video"}]}]`,
		},
		{
			name: "legacy content",
			msg: &schema.Message{Role: schema.Assistant, Content: "duplicate answer", ReasoningContent: "legacy thought", MultiContent: []schema.ChatMessagePart{
				{Type: schema.ChatMessagePartTypeText, Text: "caption"},
				{Type: schema.ChatMessagePartTypeImageURL, ImageURL: &schema.ChatMessageImageURL{URL: "https://example.com/image", Detail: schema.ImageURLDetailHigh}},
			}},
			want: `[{"type":"reasoning","status":"completed","summary":[{"type":"summary_text","text":"legacy thought"}]},
			 {"type":"message","role":"assistant","content":[{"type":"output_text","text":"caption"},{"type":"input_image","image_url":"https://example.com/image","detail":"high"}]}]`,
		},
		{
			name: "whitespace content",
			msg:  &schema.Message{Role: schema.Assistant, Content: " \n"},
			want: `[{"type":"message","role":"assistant","content":" \n"}]`,
		},
	} {
		for _, mode := range []string{"generate", "stream"} {
			for _, codec := range messagePersistenceCodecs() {
				t.Run(tc.name+"/"+mode+"/"+codec.name, func(t *testing.T) {
					messages := roundTripHistory(t, codec, []*schema.Message{tc.msg})
					msg := messages[0]
					// Arbitrary model-specific state must not influence the standard
					// mapping, even if a field resembles a target provider setting.
					extra := map[string]any{"id": "foreign-id", "status": "foreign-status", "signature": "foreign signature", keyVideoFPS: float64(9)}
					msg.Extra = extra
					for i := range msg.AssistantGenMultiContent {
						msg.AssistantGenMultiContent[i].Extra = extra
					}
					for i := range msg.ToolCalls {
						msg.ToolCalls[i].Extra = extra
					}
					before, err := json.Marshal(messages)
					check(t, err)
					cm := testResponsesModel(t, func(w http.ResponseWriter, r *http.Request) {
						var body struct {
							Input  json.RawMessage `json:"input"`
							Stream bool            `json:"stream"`
						}
						if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
							t.Error(err)
							return
						}
						equalRawJSON(t, []byte(tc.want), body.Input)
						writeResponsesHistoryReply(w, false, body.Stream)
					}, nil)
					callResponsesWithOptions(t, cm, mode, messages)
					after, err := json.Marshal(messages)
					check(t, err)
					equal(t, string(before), string(after))
				})
			}
		}
	}
}

func TestResponsesStandardAssistantInvalidToolType(t *testing.T) {
	cm := testResponsesModel(t, func(http.ResponseWriter, *http.Request) { t.Error("unsupported tool type reached HTTP") }, nil)
	_, err := cm.Generate(context.Background(), []*schema.Message{{Role: schema.Assistant, ToolCalls: []schema.ToolCall{
		{ID: "call", Type: "custom", Function: schema.FunctionCall{Name: "inspect", Arguments: `{}`}},
	}}})
	if err == nil || !strings.Contains(err.Error(), "unsupported tool call type") {
		t.Fatalf("expected unsupported tool type error, got %v", err)
	}
}

func TestMidConversationSystemContent(t *testing.T) {
	for _, role := range []schema.RoleType{schema.System, Developer} {
		for _, mode := range []string{"generate", "stream"} {
			t.Run(string(role)+"/"+mode, func(t *testing.T) {
				messages := []*schema.Message{schema.UserMessage("first"), schema.AssistantMessage("reply", nil), {
					Role: role, UserInputMultiContent: []schema.MessageInputPart{{Type: schema.ChatMessagePartTypeText, Text: "reminder"}},
				}, schema.UserMessage("continue")}
				cm := testResponsesModel(t, func(w http.ResponseWriter, r *http.Request) {
					body := decodeRequest(t, r)
					input := body["input"].([]any)
					equal(t, 4, len(input))
					equal(t, string(role), input[2].(map[string]any)["role"])
					equal(t, "reminder", input[2].(map[string]any)["content"].([]any)[0].(map[string]any)["text"])
					writeResponsesHistoryReply(w, false, body["stream"].(bool))
				}, nil)
				callResponsesWithOptions(t, cm, mode, messages)
				chat := testModel(t, func(w http.ResponseWriter, r *http.Request) {
					input := decodeRequest(t, r)["messages"].([]any)
					equal(t, string(role), input[2].(map[string]any)["role"])
					equal(t, "reminder", input[2].(map[string]any)["content"].([]any)[0].(map[string]any)["text"])
					writeResponse(w)
				}, nil)
				_, err := chat.Generate(context.Background(), messages)
				check(t, err)
			})
		}
	}
}

func TestResponsesCachedSystemInput(t *testing.T) {
	for _, role := range []schema.RoleType{schema.System, Developer} {
		for _, mode := range []string{"generate", "stream"} {
			t.Run(string(role)+"/"+mode, func(t *testing.T) {
				cm := testResponsesModel(t, func(w http.ResponseWriter, r *http.Request) {
					body := decodeRequest(t, r)
					equal(t, "cached-response", body["previous_response_id"])
					input := body["input"].([]any)
					equal(t, 2, len(input))
					equal(t, string(role), input[0].(map[string]any)["role"])
					equal(t, "reminder", input[0].(map[string]any)["content"].([]any)[0].(map[string]any)["text"])
					writeResponsesHistoryReply(w, true, body["stream"].(bool))
				}, &ResponsesAPIConfig{SessionCache: &SessionCacheConfig{EnableCache: true}})
				callResponsesWithOptions(t, cm, mode, []*schema.Message{
					schema.UserMessage("first"),
					{Role: schema.Assistant, Content: "previous", Extra: map[string]any{keyResponseID: metadataString("cached-response"), keyCacheExpiration: metadataInt64(4102444800)}},
					{Role: role, UserInputMultiContent: []schema.MessageInputPart{{Type: schema.ChatMessagePartTypeText, Text: "reminder"}}},
					schema.UserMessage("continue"),
				})
			})
		}
	}
}

func TestResponsesReplayAdjacentMessages(t *testing.T) {
	var requests [][]json.RawMessage
	cm := testResponsesModel(t, func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Input []json.RawMessage }
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		requests = append(requests, body.Input)
		writeResponses(w)
	}, nil)
	first, err := cm.Generate(context.Background(), []*schema.Message{schema.UserMessage("first")})
	check(t, err)
	_, err = cm.Generate(context.Background(), []*schema.Message{first, first})
	check(t, err)
	var output []json.RawMessage
	saved, _ := responsesInputJSON(first)
	check(t, json.Unmarshal([]byte(saved), &output))
	equal(t, 2*len(output), len(requests[1]))
	for i, raw := range requests[1] {
		equalRawJSON(t, output[i%len(output)], raw)
	}
}

func TestResponsesModifiedInputFilePreprocessing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fixture")
	check(t, os.WriteFile(path, []byte("fixture"), 0600))
	for _, mode := range []string{"generate", "stream"} {
		for _, kind := range []string{"image", "video", "file"} {
			for _, edit := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/edit=%t", mode, kind, edit), func(t *testing.T) {
					modifier := func(_ context.Context, _ []*schema.Message, raw []byte) ([]byte, error) {
						if !edit {
							return raw, nil
						}
						var fields map[string]json.RawMessage
						if err := sonic.Unmarshal(raw, &fields); err != nil {
							return nil, err
						}
						// Preserve unknown fields and exact numbers while the SDK
						// replaces the local media URL in this edited input.
						fields["input"] = json.RawMessage(fmt.Sprintf(`[{"type":"message","role":"user","extension":9007199254740993,"content":[{"type":"input_%s","%s_url":%q,"extension":9007199254740993}]}]`, kind, kind, "file://"+path))
						return sonic.Marshal(fields)
					}
					var uploads atomic.Int32
					cm := testResponsesModel(t, func(w http.ResponseWriter, r *http.Request) {
						switch r.URL.Path {
						case "/api/v3/files", "/api/v3/files/file-1":
							if r.Method == http.MethodPost {
								uploads.Add(1)
								if err := r.ParseMultipartForm(1024); err != nil {
									t.Error(err)
								}
								if r.MultipartForm != nil {
									defer r.MultipartForm.RemoveAll()
								}
							}
							w.Header().Set("Content-Type", "application/json")
							_, _ = io.WriteString(w, `{"id":"file-1","object":"file","bytes":7,"created_at":1,"expire_at":-1,"filename":"fixture","purpose":"user_data","status":"active"}`)
						case "/api/v3/responses":
							var body struct {
								Input []struct {
									Extension json.Number
									Content   []map[string]json.RawMessage
								}
								Stream bool
							}
							if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
								t.Error(err)
							}
							for _, part := range body.Input[len(body.Input)-1].Content {
								equal(t, `"file-1"`, string(part["file_id"]))
								if _, exists := part[kind+"_url"]; exists {
									t.Error("modifier restored the original media URL")
								}
								if edit {
									equal(t, "9007199254740993", string(part["extension"]))
								}
							}
							if edit {
								equal(t, "9007199254740993", body.Input[0].Extension.String())
							}
							writeResponsesHistoryReply(w, false, body.Stream)
						default:
							t.Error("unexpected path", r.URL.Path)
							http.NotFound(w, r)
						}
					}, &ResponsesAPIConfig{RequestPayloadModifier: modifier})
					part := schema.MessageInputPart{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{URL: ptr("file://" + path)}}}
					if kind == "video" {
						part = schema.MessageInputPart{Type: schema.ChatMessagePartTypeVideoURL, Video: &schema.MessageInputVideo{MessagePartCommon: schema.MessagePartCommon{URL: ptr("file://" + path)}}}
					} else if kind == "file" {
						part = schema.MessageInputPart{Type: schema.ChatMessagePartTypeFileURL, File: &schema.MessageInputFile{MessagePartCommon: schema.MessagePartCommon{URL: ptr("file://" + path)}}}
					}
					callResponsesWithOptions(t, cm, mode, []*schema.Message{{Role: schema.User, UserInputMultiContent: []schema.MessageInputPart{part, part}}})
					wantUploads := int32(2)
					if edit {
						wantUploads = 1
					}
					equal(t, wantUploads, uploads.Load())
				})
			}
		}
	}
}

func TestResponsesReplayStreamedReasoning(t *testing.T) {
	for _, codec := range messagePersistenceCodecs() {
		for _, withRaw := range []bool{false, true} {
			for _, edited := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/raw=%t/edited=%t", codec.name, withRaw, edited), func(t *testing.T) {
					output := `[{"type":"reasoning","id":"rs-1","status":"completed","summary":[{"type":"summary_text","text":"first"},{"type":"summary_text","text":"second"}],"encrypted_content":"cipher"CONTENT},{"type":"message","id":"msg-1","role":"assistant","status":"completed","content":[{"type":"output_text","text":"done","annotations":[]}]}]`
					content := ""
					if withRaw {
						content = `,"content":[{"type":"reasoning_text","text":"raw"}]`
					}
					output = strings.ReplaceAll(output, "CONTENT", content)
					cm := testResponsesModel(t, func(w http.ResponseWriter, r *http.Request) {
						body := decodeRequest(t, r)
						if body["stream"].(bool) {
							w.Header().Set("Content-Type", "text/event-stream")
							for i, text := range []string{"first", "second"} {
								_, _ = io.WriteString(w, responseEvent("response.reasoning_summary_text.delta", map[string]any{"output_index": 0, "summary_index": i, "item_id": "rs-1", "delta": text}))
							}
							if withRaw {
								_, _ = io.WriteString(w, responseEvent("response.reasoning_text.delta", map[string]any{"output_index": 0, "content_index": 0, "item_id": "rs-1", "delta": "raw"}))
							}
							_, _ = io.WriteString(w, responseEvent("response.output_text.delta", map[string]any{"output_index": 1, "content_index": 0, "item_id": "msg-1", "delta": "done"}))
							_, _ = io.WriteString(w, responseEvent("response.completed", map[string]any{"response": json.RawMessage(fmt.Sprintf(`{"id":"resp-1","created_at":1,"object":"response","model":"test","status":"completed","output":%s}`, output))}))
							return
						}
						input := body["input"].([]any)
						equal(t, 2, len(input))
						equal(t, "cipher", input[0].(map[string]any)["encrypted_content"])
						equal(t, "rs-1", input[0].(map[string]any)["id"])
						writeResponses(w)
					}, nil)
					first := callResponsesWithOptions(t, cm, "stream", []*schema.Message{schema.UserMessage("first")})
					if edited {
						first.ReasoningContent = "edited"
					}
					history := roundTripHistory(t, codec, []*schema.Message{first})
					_, err := cm.Generate(context.Background(), history)
					check(t, err)
				})
			}
		}
	}
}

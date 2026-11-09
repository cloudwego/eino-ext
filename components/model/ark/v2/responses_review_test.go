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

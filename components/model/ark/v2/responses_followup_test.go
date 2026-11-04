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

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/responses"
)

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
					equal(t, "raw thought", first.ReasoningContent)
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

func TestResponsesEffectiveCallbackConfig(t *testing.T) {
	for _, mode := range []string{"generate", "stream"} {
		t.Run(mode, func(t *testing.T) {
			var started []*schema.ToolInfo
			var configs []*model.Config
			handler := callbacks.NewHandlerBuilder().OnStartFn(func(ctx context.Context, _ *callbacks.RunInfo, in callbacks.CallbackInput) context.Context {
				started = in.(*model.CallbackInput).Tools
				return ctx
			}).OnEndFn(func(ctx context.Context, _ *callbacks.RunInfo, out callbacks.CallbackOutput) context.Context {
				configs = append(configs, out.(*model.CallbackOutput).Config)
				return ctx
			}).OnEndWithStreamOutputFn(func(ctx context.Context, _ *callbacks.RunInfo, sr *schema.StreamReader[callbacks.CallbackOutput]) context.Context {
				defer sr.Close()
				for {
					out, err := sr.Recv()
					if err != nil {
						return ctx
					}
					configs = append(configs, out.(*model.CallbackOutput).Config)
				}
			}).Build()
			cm := testResponsesModel(t, func(w http.ResponseWriter, r *http.Request) {
				body := decodeRequest(t, r)
				equal(t, "modified", body["model"])
				writeResponsesHistoryReply(w, false, body["stream"] == true)
			}, &ResponsesAPIConfig{RequestPayloadModifier: func(_ context.Context, _ []*schema.Message, raw []byte) ([]byte, error) {
				var fields map[string]json.RawMessage
				if err := json.Unmarshal(raw, &fields); err != nil {
					return nil, err
				}
				fields["model"] = json.RawMessage(`"modified"`)
				fields["max_output_tokens"] = json.RawMessage(`123`)
				fields["temperature"] = json.RawMessage(`0.25`)
				return json.Marshal(fields)
			}})
			check(t, cm.BindTools([]*schema.ToolInfo{toolInfo("inspect"), toolInfo("other")}))
			ctx := callbacks.InitCallbacks(context.Background(), nil, handler)
			callResponsesWithOptionsContext(t, ctx, cm, mode, model.WithToolChoice(schema.ToolChoiceAllowed, "other"))
			equal(t, 1, len(started))
			equal(t, "other", started[0].Name)
			if len(configs) == 0 {
				t.Fatal("missing callback output")
			}
			for _, config := range configs {
				equal(t, "modified", config.Model)
				equal(t, 123, config.MaxTokens)
				equal(t, float32(0.25), config.Temperature)
			}
		})
	}
}

func callResponsesWithOptionsContext(t *testing.T, ctx context.Context, cm *ResponsesAPIChatModel, mode string, opts ...model.Option) {
	t.Helper()
	if mode == "stream" {
		sr, err := cm.Stream(ctx, []*schema.Message{schema.UserMessage("first")}, opts...)
		check(t, err)
		_, err = readResponsesStream(t, sr)
		check(t, err)
		return
	}
	_, err := cm.Generate(ctx, []*schema.Message{schema.UserMessage("first")}, opts...)
	check(t, err)
}

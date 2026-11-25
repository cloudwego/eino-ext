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
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/responses"
)

const responsesJSON = `{"id":"resp-1","created_at":1,"object":"response","model":"actual-model","status":"completed","output":[{"type":"reasoning","id":"rs-1","status":"completed","summary":[{"type":"summary_text","text":"summary"}],"content":[{"type":"reasoning_text","text":"raw thought"}],"encrypted_content":"cipher"},{"type":"message","id":"msg-1","role":"assistant","status":"completed","content":[{"type":"output_text","text":"done","annotations":[]}]},{"type":"function_call","id":"fc-1","call_id":"call-1","name":"inspect","arguments":"{}","status":"completed"}],"usage":{"input_tokens":8,"output_tokens":3,"total_tokens":11,"input_tokens_details":{"cached_tokens":2},"output_tokens_details":{"reasoning_tokens":1}},"service_tier":"default","expire_at":4102444800}`

func testResponsesModel(t *testing.T, handler http.HandlerFunc, config *ResponsesAPIConfig) *ResponsesAPIChatModel {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	if config == nil {
		config = &ResponsesAPIConfig{}
	}
	if config.Model == "" {
		config.Model = "requested-model"
	}
	config.APIKey, config.BaseURL, config.RetryTimes = "test-key", server.URL+"/api/v3", ptr(0)
	cm, err := NewResponsesAPIChatModel(context.Background(), config)
	check(t, err)
	return cm
}
func writeResponses(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, responsesJSON)
}
func responseEvent(kind string, fields map[string]any) string {
	fields["type"] = kind
	fields["sequence_number"] = 0
	raw, _ := sonic.Marshal(fields)
	return "event: " + kind + "\ndata: " + string(raw) + "\n\n"
}
func terminalEvent(kind string, output ...json.RawMessage) string {
	var resp map[string]any
	_ = sonic.UnmarshalString(responsesJSON, &resp)
	resp["status"] = strings.TrimPrefix(kind, "response.")
	if len(output) > 0 {
		var compact bytes.Buffer
		if err := json.Compact(&compact, output[0]); err != nil {
			panic(err)
		}
		resp["output"] = json.RawMessage(compact.Bytes())
	}
	if kind == "response.incomplete" {
		resp["incomplete_details"] = map[string]any{"reason": "max_output_tokens"}
	}
	if kind == "response.failed" {
		resp["error"] = map[string]any{"code": "server_error", "message": "failed deliberately"}
	}
	return responseEvent(kind, map[string]any{"response": resp})
}
func readResponsesStream(t *testing.T, sr *schema.StreamReader[*schema.Message]) (*schema.Message, error) {
	t.Helper()
	defer sr.Close()
	var messages []*schema.Message
	for {
		msg, err := sr.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	return schema.ConcatMessages(messages)
}

func TestResponsesGenerate(t *testing.T) {
	var callbackContent string
	config := &ResponsesAPIConfig{Model: "requested-model", MaxOutputTokens: ptr(1024), Temperature: ptr(float32(0.7)), TopP: ptr(float32(0.9)),
		Thinking: &responses.Thinking{Type: responses.NewOptThinkingMode(responses.ThinkingModeDisabled)}, ReasoningEffort: ptr(responses.ReasoningEffortLow),
		SessionCache:        &SessionCacheConfig{EnableCache: true, TTL: 3600},
		EnableToolWebSearch: &responses.WebSearchTool{Limit: responses.NewOptInt64(2)}, MaxToolCalls: ptr(int64(3)),
		CustomHeader:   map[string]string{"X-Test": "configured"},
		ResponseFormat: &responses.TextFormat{Type: responses.TextFormatTypeJSONObject},
	}
	cm := testResponsesModel(t, func(w http.ResponseWriter, r *http.Request) {
		equal(t, "/api/v3/responses", r.URL.Path)
		equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		equal(t, "override", r.Header.Get("X-Test"))
		equal(t, "true", r.Header.Get("ark-beta-web-search"))
		body := decodeRequest(t, r)
		equal(t, "requested-model", body["model"])
		equal(t, false, body["stream"])
		equal(t, float64(1024), body["max_output_tokens"])
		equal(t, float64(0.7), body["temperature"])
		equal(t, true, body["store"])
		equal(t, "enabled", body["caching"].(map[string]any)["type"])
		equal(t, "disabled", body["thinking"].(map[string]any)["type"])
		equal(t, "low", body["reasoning"].(map[string]any)["effort"])
		equal(t, "json_object", body["text"].(map[string]any)["format"].(map[string]any)["type"])
		tools := body["tools"].([]any)
		equal(t, 2, len(tools))
		function := tools[0].(map[string]any)
		equal(t, "inspect", function["name"])
		if _, ok := function["strict"]; ok {
			t.Error("Responses function tool contains Chat strict setting")
		}
		if _, ok := body["parallel_tool_calls"]; ok {
			t.Error("Responses request contains Chat parallel tool setting")
		}
		equal(t, "function", function["type"])
		equal(t, "web_search", tools[1].(map[string]any)["type"])
		writeResponses(w)
	}, config)
	check(t, cm.BindTools([]*schema.ToolInfo{WithToolStrict(toolInfo("inspect"), false)}))
	ctx := callbacks.InitCallbacks(context.Background(), nil, callbacks.NewHandlerBuilder().OnEndFn(func(_ context.Context, _ *callbacks.RunInfo, out callbacks.CallbackOutput) context.Context {
		callbackContent = out.(*model.CallbackOutput).Message.Content
		return context.Background()
	}).Build())
	out, err := cm.Generate(ctx, []*schema.Message{schema.UserMessage("hello")}, WithCustomHeader(map[string]string{"X-Test": "override"}), WithParallelToolCalls(false))
	check(t, err)
	equal(t, "done", out.Content)
	equal(t, "done", callbackContent)
	equal(t, "summary", out.ReasoningContent)
	equal(t, "function", out.ToolCalls[0].Type)
	equal(t, "call-1", out.ToolCalls[0].ID)
	equal(t, 11, out.ResponseMeta.Usage.TotalTokens)
	equal(t, 1, out.ResponseMeta.Usage.CompletionTokensDetails.ReasoningTokens)
	meta, ok := GetResponsesMetadata(out)
	equal(t, true, ok)
	equal(t, "resp-1", meta.ResponseID)
	ref, ok := GetResponseCacheRef(out)
	equal(t, true, ok)
	equal(t, ResponseCacheRef{ResponseID: meta.ResponseID, ExpireAt: 4102444800}, ref)
	equal(t, int64(2), meta.UsageDetails.InputTokensDetails.CachedTokens)
	itemCount := 0
	for i := range out.AssistantGenMultiContent {
		if hasResponsesItemMetadata(&out.AssistantGenMultiContent[i]) {
			itemCount++
		}
	}
	equal(t, 2, itemCount)
	equal(t, "function_call", fmt.Sprint(out.ToolCalls[0].Extra[keyResponsesItemType]))
}

func TestResponsesToolsAndPayload(t *testing.T) {
	for _, choice := range []schema.ToolChoice{schema.ToolChoiceAllowed, schema.ToolChoiceForced} {
		for _, names := range [][]string{{"inspect"}, {"inspect", "other"}} {
			t.Run(string(choice)+strings.Join(names, "-"), func(t *testing.T) {
				cm := testResponsesModel(t, func(w http.ResponseWriter, r *http.Request) {
					body := decodeRequest(t, r)
					equal(t, len(names), len(body["tools"].([]any)))
					equal(t, false, body["stream"])
					if choice == schema.ToolChoiceForced && len(names) == 1 {
						equal(t, "inspect", body["tool_choice"].(map[string]any)["name"])
					}
					writeResponses(w)
				}, &ResponsesAPIConfig{Temperature: ptr(float32(1))})
				check(t, cm.BindTools([]*schema.ToolInfo{toolInfo("inspect"), toolInfo("other"), toolInfo("excluded")}))
				_, err := cm.Generate(context.Background(), []*schema.Message{schema.UserMessage("hi")}, model.WithToolChoice(choice, names...))
				check(t, err)
			})
		}
	}
}

func TestResponsesStream(t *testing.T) {
	for _, ending := range []string{"response.completed", "response.incomplete"} {
		t.Run(ending, func(t *testing.T) {
			cm := testResponsesModel(t, func(w http.ResponseWriter, r *http.Request) {
				equal(t, true, decodeRequest(t, r)["stream"])
				w.Header().Set("Content-Type", "text/event-stream")
				for i, name := range []string{"first", "second"} {
					_, _ = io.WriteString(w, responseEvent("response.output_item.added", map[string]any{"output_index": i, "item": map[string]any{"type": "function_call", "id": fmt.Sprint("fc-", i), "call_id": fmt.Sprint("call-", i), "name": name, "arguments": ""}}))
				}
				for _, i := range []int{1, 0, 1, 0} {
					delta := "{"
					_, _ = io.WriteString(w, responseEvent("response.function_call_arguments.delta", map[string]any{"output_index": i, "item_id": fmt.Sprint("fc-", i), "delta": delta}))
				}
				_, _ = io.WriteString(w, responseEvent("response.output_text.delta", map[string]any{"output_index": 2, "content_index": 0, "item_id": "msg-1", "delta": "hello"}))
				_, _ = io.WriteString(w, responseEvent("response.reasoning_text.delta", map[string]any{"output_index": 3, "content_index": 0, "item_id": "rs-1", "delta": "thought"}))
				_, _ = io.WriteString(w, terminalEvent(ending, json.RawMessage(`[
					{"type":"function_call","id":"fc-0","call_id":"call-0","name":"first","arguments":"{{","status":"completed"},
					{"type":"function_call","id":"fc-1","call_id":"call-1","name":"second","arguments":"{{","status":"completed"},
					{"type":"message","id":"msg-1","role":"assistant","status":"completed","content":[{"type":"output_text","text":"hello","annotations":[]}]},
					{"type":"reasoning","id":"rs-1","status":"completed","summary":[],"content":[{"type":"reasoning_text","text":"thought"}]}
				]`)))
			}, &ResponsesAPIConfig{SessionCache: &SessionCacheConfig{EnableCache: true, TTL: 3600}})
			sr, err := cm.Stream(context.Background(), []*schema.Message{schema.UserMessage("hi")})
			check(t, err)
			out, err := readResponsesStream(t, sr)
			check(t, err)
			equal(t, "hello", out.Content)
			equal(t, "", out.ReasoningContent)
			equal(t, 2, len(out.ToolCalls))
			equal(t, "call-0", out.ToolCalls[0].ID)
			equal(t, "first", out.ToolCalls[0].Function.Name)
			equal(t, "{{", out.ToolCalls[0].Function.Arguments)
			equal(t, "call-1", out.ToolCalls[1].ID)
			equal(t, 11, out.ResponseMeta.Usage.TotalTokens)
			meta, ok := GetResponsesMetadata(out)
			equal(t, true, ok)
			if ending == "response.incomplete" {
				equal(t, "incomplete", out.ResponseMeta.FinishReason)
				equal(t, &responses.IncompleteDetails{Reason: "max_output_tokens"}, meta.IncompleteDetails)
			} else {
				equal(t, "completed", out.ResponseMeta.FinishReason)
				equal(t, (*responses.IncompleteDetails)(nil), meta.IncompleteDetails)
			}
			equal(t, "resp-1", meta.ResponseID)
		})
	}
}

func TestResponsesIncompleteDetails(t *testing.T) {
	for _, tc := range []struct {
		name    string
		details *responses.IncompleteDetails
	}{
		{name: "omitted"},
		{name: "content filter", details: &responses.IncompleteDetails{
			Reason:        "content_filter",
			ContentFilter: responses.NewOptContentFilter(responses.ContentFilter{Type: "text", Details: "filtered"}),
		}},
	} {
		for _, mode := range []string{"generate", "stream"} {
			t.Run(tc.name+"/"+mode, func(t *testing.T) {
				body := map[string]any{"id": "resp-1", "object": "response", "model": "test", "created_at": 1, "status": "incomplete", "output": []any{}}
				if tc.details != nil {
					body["incomplete_details"] = tc.details
				}
				cm := testResponsesModel(t, func(w http.ResponseWriter, r *http.Request) {
					if decodeRequest(t, r)["stream"] == true {
						w.Header().Set("Content-Type", "text/event-stream")
						_, _ = io.WriteString(w, responseEvent("response.incomplete", map[string]any{"response": body}))
						return
					}
					w.Header().Set("Content-Type", "application/json")
					check(t, json.NewEncoder(w).Encode(body))
				}, nil)
				msg := callHistoryModel(t, cm, mode, []*schema.Message{schema.UserMessage("hi")})
				for _, codec := range messagePersistenceCodecs() {
					restored := roundTripHistory(t, codec, []*schema.Message{msg})[0]
					equal(t, "incomplete", restored.ResponseMeta.FinishReason)
					meta, ok := GetResponsesMetadata(restored)
					equal(t, true, ok)
					equal(t, tc.details, meta.IncompleteDetails)
				}
			})
		}
	}
}

func TestResponsesStreamFailures(t *testing.T) {
	for _, test := range []struct{ name, wire, want string }{
		{"empty", "", "unexpected EOF"},
		{"done_only", "data: [DONE]\n\n", "unexpected EOF"},
		{"failed", terminalEvent("response.failed"), "Ark response failed (server_error): failed deliberately"},
		{"error", responseEvent("error", map[string]any{"message": "bad input", "code": "bad_request", "param": "input"}), "bad input"},
		{"invalid", "data: {broken}\n\n", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			cm := testResponsesModel(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = io.WriteString(w, test.wire)
			}, nil)
			sr, err := cm.Stream(context.Background(), []*schema.Message{schema.UserMessage("hi")})
			check(t, err)
			_, err = readResponsesStream(t, sr)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected %q, got %v", test.want, err)
			}
		})
	}
}

func TestResponsesStreamFailedAfterDelta(t *testing.T) {
	delta := responseEvent("response.output_text.delta", map[string]any{
		"delta": "partial", "output_index": 0, "content_index": 0, "item_id": "msg-1",
	})
	// The event type is authoritative even with a completed snapshot and no error details.
	failed := responseEvent("response.failed", map[string]any{"response": json.RawMessage(responsesJSON)})
	cm := testResponsesModel(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, delta+failed)
	}, nil)
	sr, err := cm.Stream(context.Background(), []*schema.Message{schema.UserMessage("hi")})
	check(t, err)
	defer sr.Close()
	msg, err := sr.Recv()
	check(t, err)
	equal(t, "partial", msg.Content)
	msg, err = sr.Recv()
	if msg != nil || err == nil || err.Error() != "Ark response failed" {
		t.Fatalf("expected only the failure, got message %v and error %v", msg, err)
	}
	_, err = sr.Recv()
	equal(t, true, errors.Is(err, io.EOF))
}

func TestResponsesCacheAndReasoning(t *testing.T) {
	var call atomic.Int32
	cm := testResponsesModel(t, func(w http.ResponseWriter, r *http.Request) {
		body := decodeRequest(t, r)
		n := call.Add(1)
		input := body["input"].([]any)
		switch n {
		case 2:
			equal(t, "resp-1", body["previous_response_id"])
			equal(t, 1, len(input))
			equal(t, "user", input[0].(map[string]any)["role"])
		case 3:
			if _, ok := body["previous_response_id"]; ok {
				t.Error("cache invalidation ignored")
			}
			equal(t, "reasoning", input[1].(map[string]any)["type"])
			equal(t, "cipher", input[1].(map[string]any)["encrypted_content"])
		}
		writeResponses(w)
	}, &ResponsesAPIConfig{SessionCache: &SessionCacheConfig{EnableCache: true, TTL: 3600}})
	first, err := cm.Generate(context.Background(), []*schema.Message{schema.UserMessage("hi")})
	check(t, err)
	// A persisted message can carry both packages' cache metadata.
	legacyExpiry := float64(time.Now().Add(time.Hour).Unix())
	first.Extra["ark-response-id"] = "legacy-response"
	first.Extra["ark-response-cache-expire-at"] = legacyExpiry
	history := []*schema.Message{schema.UserMessage("hi"), first, schema.UserMessage("next")}
	_, err = cm.Generate(context.Background(), history, WithCache(CacheOption{SessionCache: &SessionCacheConfig{EnableCache: true}, HeadPreviousResponseID: ptr("prefix")}))
	check(t, err)
	check(t, InvalidateMessageCaches([]*schema.Message{first}))
	equal(t, legacyExpiry, first.Extra["ark-response-cache-expire-at"])
	equal(t, "legacy-response", first.Extra["ark-response-id"])
	_, err = cm.Generate(context.Background(), history)
	check(t, err)
}

func TestResponsesConcurrentAndCancellation(t *testing.T) {
	cm := testResponsesModel(t, func(w http.ResponseWriter, r *http.Request) { writeResponses(w) }, &ResponsesAPIConfig{})
	check(t, cm.BindTools([]*schema.ToolInfo{toolInfo("inspect")}))
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := cm.Generate(context.Background(), []*schema.Message{schema.UserMessage("hi")})
			if err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	blocked := testResponsesModel(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	sr, err := blocked.Stream(ctx, []*schema.Message{schema.UserMessage("hi")})
	check(t, err)
	_, err = readResponsesStream(t, sr)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline, got %v", err)
	}
}

func TestResponsesAssistantMediaHistory(t *testing.T) {
	requests := make(chan map[string]any, 1)
	cm := testResponsesModel(t, func(w http.ResponseWriter, r *http.Request) {
		requests <- decodeRequest(t, r)
		writeResponses(w)
	}, nil)
	_, err := cm.Generate(context.Background(), []*schema.Message{{Role: schema.Assistant, AssistantGenMultiContent: []schema.MessageOutputPart{
		{Type: schema.ChatMessagePartTypeText, Text: "caption"},
		{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageOutputImage{MessagePartCommon: schema.MessagePartCommon{FileID: ptr("image-1")}}},
		{Type: schema.ChatMessagePartTypeVideoURL, Video: &schema.MessageOutputVideo{MessagePartCommon: schema.MessagePartCommon{URL: ptr("https://example.com/video")}}, Extra: map[string]any{keyVideoFPS: 0.25}},
	}}})
	check(t, err)
	item := (<-requests)["input"].([]any)[0].(map[string]any)
	equal(t, "assistant", item["role"])
	equal(t, []any{
		map[string]any{"type": "output_text", "text": "caption"},
		map[string]any{"type": "input_image", "file_id": "image-1"},
		map[string]any{"type": "input_video", "video_url": "https://example.com/video"},
	}, item["content"])
}

func TestResponsesNativeCallOptions(t *testing.T) {
	for _, tc := range []struct {
		name     string
		thinking *responses.Thinking
		want     any
	}{
		{"disabled", &responses.Thinking{Type: responses.NewOptThinkingMode(responses.ThinkingModeDisabled)}, map[string]any{"type": "disabled"}},
		{"unset type", &responses.Thinking{}, map[string]any{}},
		{"omitted", nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := &ResponsesAPIConfig{Thinking: &responses.Thinking{Type: responses.NewOptThinkingMode(responses.ThinkingModeEnabled)}, ReasoningEffort: ptr(responses.ReasoningEffortLow)}
			requests := make(chan map[string]any, 1)
			cm := testResponsesModel(t, func(w http.ResponseWriter, r *http.Request) { requests <- decodeRequest(t, r); writeResponses(w) }, config)
			var observed any
			handler := callbacks.NewHandlerBuilder().OnStartFn(func(ctx context.Context, _ *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
				observed = input.(*model.CallbackInput).Extra["thinking"]
				return ctx
			}).Build()
			ctx := callbacks.InitCallbacks(context.Background(), nil, handler)
			_, err := cm.Generate(ctx, []*schema.Message{schema.UserMessage("hello")}, WithResponsesThinking(tc.thinking), WithResponsesReasoningEffort(responses.ReasoningEffortHigh))
			check(t, err)
			body := <-requests
			equal(t, tc.want, body["thinking"])
			equal(t, "high", body["reasoning"].(map[string]any)["effort"])
			thinking, ok := observed.(*responses.Thinking)
			equal(t, true, ok)
			equal(t, tc.thinking, thinking)
			equal(t, responses.ThinkingModeEnabled, config.Thinking.Type.Value)
			equal(t, responses.ReasoningEffortLow, *config.ReasoningEffort)
		})
	}
}

func TestResponsesNativeToolChoice(t *testing.T) {
	for _, tc := range []struct {
		name   string
		choice *schema.ToolChoice
		names  []string
		want   any
		count  int
	}{
		{"default", nil, nil, nil, 2},
		{"none", ptr(schema.ToolChoiceForbidden), nil, "none", 2},
		{"auto subset", ptr(schema.ToolChoiceAllowed), []string{"inspect"}, "auto", 1},
		{"required", ptr(schema.ToolChoiceForced), nil, "required", 2},
		{"named", ptr(schema.ToolChoiceForced), []string{"inspect"}, map[string]any{"type": "function", "name": "inspect"}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := make(chan map[string]any, 2)
			cm := testResponsesModel(t, func(w http.ResponseWriter, r *http.Request) { requests <- decodeRequest(t, r); writeResponses(w) }, nil)
			check(t, cm.BindTools([]*schema.ToolInfo{toolInfo("inspect"), toolInfo("other")}))
			var opts []model.Option
			if tc.choice != nil {
				opts = append(opts, model.WithToolChoice(*tc.choice, tc.names...))
			}
			_, err := cm.Generate(context.Background(), []*schema.Message{schema.UserMessage("hello")}, opts...)
			check(t, err)
			body := <-requests
			equal(t, tc.want, body["tool_choice"])
			equal(t, tc.count, len(body["tools"].([]any)))
			for _, value := range body["tools"].([]any) {
				function := value.(map[string]any)
				equal(t, "function", function["type"])
				if _, nested := function["function"]; nested {
					t.Fatal("Responses function uses the Chat envelope")
				}
				if _, strict := function["strict"]; strict {
					t.Fatal("unset strict must be omitted")
				}
			}
			if len(tc.names) > 0 {
				_, err = cm.Generate(context.Background(), []*schema.Message{schema.UserMessage("next")})
				check(t, err)
				equal(t, 2, len((<-requests)["tools"].([]any)))
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
				equal(t, "configured", body["model"])
				writeResponsesHistoryReply(w, false, body["stream"] == true)
			}, &ResponsesAPIConfig{Model: "configured", MaxOutputTokens: ptr(123), Temperature: ptr(float32(0.25))})
			check(t, cm.BindTools([]*schema.ToolInfo{toolInfo("inspect"), toolInfo("other")}))
			ctx := callbacks.InitCallbacks(context.Background(), nil, handler)
			callResponsesWithOptionsContext(t, ctx, cm, mode, model.WithToolChoice(schema.ToolChoiceAllowed, "other"))
			equal(t, 1, len(started))
			equal(t, "other", started[0].Name)
			if len(configs) == 0 {
				t.Fatal("missing callback output")
			}
			for _, config := range configs {
				equal(t, "configured", config.Model)
				equal(t, 123, config.MaxTokens)
				equal(t, float32(0.25), config.Temperature)
			}
		})
	}
}

func TestResponsesEffectiveCallbackThinking(t *testing.T) {
	for _, mode := range []string{"generate", "stream"} {
		for _, tc := range []struct {
			name, want string
			configured bool
		}{
			{"enabled", "enabled", true},
			{"unset", "", false},
		} {
			t.Run(mode+"/"+tc.name, func(t *testing.T) {
				var observed []string
				observe := func(out callbacks.CallbackOutput) {
					thinking, ok := out.(*model.CallbackOutput).Extra["thinking"].(*responses.Thinking)
					if !ok {
						t.Error("callback thinking must use the Responses SDK type")
					}
					value := ""
					if thinking != nil {
						value = string(thinking.Type.Or(""))
					}
					observed = append(observed, value)
				}
				handler := callbacks.NewHandlerBuilder().OnEndFn(func(ctx context.Context, _ *callbacks.RunInfo, out callbacks.CallbackOutput) context.Context {
					observe(out)
					return ctx
				}).OnEndWithStreamOutputFn(func(ctx context.Context, _ *callbacks.RunInfo, sr *schema.StreamReader[callbacks.CallbackOutput]) context.Context {
					defer sr.Close()
					for {
						out, err := sr.Recv()
						if err != nil {
							return ctx
						}
						observe(out)
					}
				}).Build()
				config := &ResponsesAPIConfig{}
				if tc.configured {
					config.Thinking = &responses.Thinking{Type: responses.NewOptThinkingMode(responses.ThinkingModeEnabled)}
				}
				cm := testResponsesModel(t, func(w http.ResponseWriter, r *http.Request) {
					body := decodeRequest(t, r)
					if tc.want == "" {
						equal(t, nil, body["thinking"])
					} else {
						equal(t, tc.want, body["thinking"].(map[string]any)["type"])
					}
					writeResponsesHistoryReply(w, false, body["stream"] == true)
				}, config)
				ctx := callbacks.InitCallbacks(context.Background(), nil, handler)
				callResponsesWithOptionsContext(t, ctx, cm, mode)
				if len(observed) == 0 {
					t.Fatal("missing callback output")
				}
				for _, value := range observed {
					equal(t, tc.want, value)
				}
			})
		}
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

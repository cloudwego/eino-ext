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

package openai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"
	"github.com/openai/openai-go/v3/responses"
)

// unmarshalResponse builds a Response the way the SDK does. Union accessors such
// as AsMessage read the raw JSON rather than the struct fields, so a Response
// assembled field by field would expose no output items.
func unmarshalResponse(t *testing.T, raw string) *responses.Response {
	t.Helper()
	var resp responses.Response
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	return &resp
}

func textResponse(status, incompleteReason, text string) string {
	return `{
		"id": "resp_test",
		"status": "` + status + `",
		"incomplete_details": {"reason": "` + incompleteReason + `"},
		"output": [{
			"type": "message",
			"id": "msg_test",
			"role": "assistant",
			"content": [{"type": "output_text", "text": "` + text + `", "annotations": []}]
		}]
	}`
}

func TestResponsesGeneratePropagatesResponseTraceID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set(responsesRequestIDHeader, " trace-generate ")
		_, _ = w.Write([]byte(textResponse("completed", "", "hello")))
	}))
	defer server.Close()

	chatModel, err := NewResponsesChatModel(context.Background(), &ResponsesChatModelConfig{
		BaseURL: server.URL + "/",
		APIKey:  "test-key",
		Model:   "test-model",
	})
	if err != nil {
		t.Fatalf("new chat model: %v", err)
	}

	msg, err := chatModel.Generate(context.Background(), []*schema.Message{schema.UserMessage("hi")})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if got := msg.Extra[ResponsesExtraKeyRequestID]; got != "trace-generate" {
		t.Fatalf("message trace ID=%v, want trace-generate", got)
	}

	callbackOutput := newCallbackOutput(msg, nil, msg.Extra[ResponsesExtraKeyRequestID].(string))
	if got := callbackOutput.Extra[ResponsesExtraKeyRequestID]; got != "trace-generate" {
		t.Fatalf("callback trace ID=%v, want trace-generate", got)
	}
}

func TestResponsesStreamPropagatesResponseTraceIDOnFinalMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set(responsesRequestIDHeader, "trace-stream")
		_, _ = io.WriteString(w, "data: {\"type\":\"response.output_item.done\",\"item\":{\"type\":\"reasoning\",\"id\":\"rs_stream\",\"summary\":[],\"encrypted_content\":\"cipher-stream\"}}\n\n")
		_, _ = io.WriteString(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\"}\n\n")
		_, _ = io.WriteString(w, "data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_stream\",\"status\":\"completed\",\"output\":[],\"usage\":{\"input_tokens\":1,\"output_tokens\":1,\"total_tokens\":2}}}\n\n")
	}))
	defer server.Close()

	chatModel, err := NewResponsesChatModel(context.Background(), &ResponsesChatModelConfig{
		BaseURL: server.URL + "/",
		APIKey:  "test-key",
		Model:   "test-model",
	})
	if err != nil {
		t.Fatalf("new chat model: %v", err)
	}

	stream, err := chatModel.Stream(context.Background(), []*schema.Message{schema.UserMessage("hi")})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	defer stream.Close()

	var chunks []*schema.Message
	for {
		chunk, recvErr := stream.Recv()
		if errors.Is(recvErr, io.EOF) {
			break
		}
		if recvErr != nil {
			t.Fatalf("receive stream: %v", recvErr)
		}
		chunks = append(chunks, chunk)
	}
	msg, err := schema.ConcatMessages(chunks)
	if err != nil {
		t.Fatalf("concat stream: %v", err)
	}
	if got := msg.Extra[ResponsesExtraKeyRequestID]; got != "trace-stream" {
		t.Fatalf("message trace ID=%v, want trace-stream", got)
	}
	items, err := reasoningInput(msg)
	if err != nil || len(items) != 1 {
		t.Fatalf("stream reasoning lost: items=%d err=%v", len(items), err)
	}
}

func TestResponsesReasoningRoundTripKeepsOnlyReasoning(t *testing.T) {
	resp := unmarshalResponse(t, `{"id":"resp_test","status":"completed","output":[
		{"type":"reasoning","id":"rs_test","summary":[],"encrypted_content":"cipher"},
		{"type":"message","role":"assistant","content":[{"type":"output_text","text":"original"}]},
		{"type":"function_call","call_id":"call_test","name":"lookup","arguments":"{\"x\":1}"}
	]}`)
	msg, err := (&ResponsesChatModel{}).convertResponse(resp)
	if err != nil {
		t.Fatal(err)
	}
	if err := saveReasoning(msg, resp.Output); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	var restored schema.Message
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	var stored []map[string]any
	if err := json.Unmarshal([]byte(restored.Extra[extraKeyReasoning].(string)), &stored); err != nil {
		t.Fatal(err)
	}
	if len(stored) != 1 || stored[0]["type"] != "reasoning" || stored[0]["encrypted_content"] != "cipher" {
		t.Fatalf("unexpected stored reasoning: %v", stored)
	}
	// Text and tool arguments must come from the current message, not a snapshot.
	restored.Content = "edited"
	restored.ToolCalls[0].Function.Arguments = `{"x":2}`
	items, err := messageToInputItems(&restored)
	if err != nil {
		t.Fatal(err)
	}
	data, err = json.Marshal(items)
	if err != nil {
		t.Fatal(err)
	}
	var input []map[string]any
	if err := json.Unmarshal(data, &input); err != nil {
		t.Fatal(err)
	}
	if len(input) != 3 || input[0]["encrypted_content"] != "cipher" || input[0]["id"] != "rs_test" || input[1]["content"] != "edited" || input[2]["arguments"] != `{"x":2}` {
		t.Fatalf("unexpected replay input: %s", data)
	}
}

func TestResponsesToResponseToolsExplicitlyDisablesStrictMode(t *testing.T) {
	toolInfos := []*schema.ToolInfo{
		{
			Name: "memo",
			ParamsOneOf: schema.NewParamsOneOfByJSONSchema(&jsonschema.Schema{
				Type: "object",
			}),
		},
	}

	tools, err := toResponseTools(toolInfos)
	if err != nil {
		t.Fatalf("convert tools: %v", err)
	}
	if len(tools) != 1 || tools[0].OfFunction == nil {
		t.Fatalf("tools=%#v, want one function tool", tools)
	}
	if strict := tools[0].OfFunction.Strict; !strict.Valid() || strict.Value {
		t.Fatalf("strict=%v (valid=%v), want explicit false", strict.Value, strict.Valid())
	}
}

func TestResponsesConvertResponseFailedStatusReturnsError(t *testing.T) {
	resp := unmarshalResponse(t, `{
		"id": "resp_1",
		"status": "failed",
		"error": {"code": "server_error", "message": "The model is overloaded."},
		"output": []
	}`)

	msg, err := (&ResponsesChatModel{}).convertResponse(resp)
	if err == nil {
		t.Fatalf("failed status must be reported as an error, got message=%#v", msg)
	}

	var failed *ResponsesFailedError
	if !errors.As(err, &failed) {
		t.Fatalf("error type=%T, want *ResponsesFailedError", err)
	}
	if failed.Code != responses.ResponseErrorCodeServerError {
		t.Errorf("code=%q, want server_error", failed.Code)
	}
	if failed.ResponseID != "resp_1" {
		t.Errorf("response id=%q, want resp_1", failed.ResponseID)
	}
}

func TestResponsesConvertResponseCancelledStatusReturnsError(t *testing.T) {
	resp := unmarshalResponse(t, `{"id": "resp_2", "status": "cancelled", "output": []}`)

	if _, err := (&ResponsesChatModel{}).convertResponse(resp); err == nil {
		t.Fatal("cancelled status must be reported as an error")
	}
}

// The retry policy in comm tells a context-window overflow apart from other
// invalid_prompt failures by matching the provider wording, so the message has
// to survive into the error string.
func TestResponsesConvertResponseFailedErrorKeepsProviderMessage(t *testing.T) {
	resp := unmarshalResponse(t, `{
		"id": "resp_3",
		"status": "failed",
		"error": {"code": "invalid_prompt", "message": "Your input exceeds the context window of this model."},
		"output": []
	}`)

	_, err := (&ResponsesChatModel{}).convertResponse(resp)
	if err == nil {
		t.Fatal("failed status must be reported as an error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "input exceeds the context window") {
		t.Fatalf("error text must keep the provider message, got %q", err)
	}
}

func TestResponsesConvertResponseFinishReason(t *testing.T) {
	tests := []struct {
		name   string
		status string
		reason string
		want   string
	}{
		{name: "completed", status: "completed", want: "completed"},
		{name: "truncated by output limit", status: "incomplete", reason: "max_output_tokens", want: "length"},
		{name: "blocked by content filter", status: "incomplete", reason: "content_filter", want: "content_filter"},
		{name: "incomplete without reason", status: "incomplete", want: "incomplete"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := unmarshalResponse(t, textResponse(tt.status, tt.reason, "已经写出的正文"))

			msg, err := (&ResponsesChatModel{}).convertResponse(resp)
			if err != nil {
				t.Fatalf("convert response: %v", err)
			}
			if msg.ResponseMeta.FinishReason != tt.want {
				t.Errorf("finish reason=%q, want %q", msg.ResponseMeta.FinishReason, tt.want)
			}
			// A truncated or filtered response still carries the text produced
			// so far; only failed runs discard it.
			if msg.Content != "已经写出的正文" {
				t.Errorf("content=%q, want the partial output to be preserved", msg.Content)
			}
		})
	}
}

// A refusal arrives on a completed response, so the status alone would mark the
// turn as usable output and let the refusal text through as chapter prose.
func TestResponsesConvertResponseRefusalMapsToContentFilter(t *testing.T) {
	resp := unmarshalResponse(t, `{
		"id": "resp_refusal",
		"status": "completed",
		"output": [{
			"type": "message",
			"id": "msg_1",
			"role": "assistant",
			"content": [{"type": "refusal", "refusal": "抱歉，我无法协助。"}]
		}]
	}`)

	msg, err := (&ResponsesChatModel{}).convertResponse(resp)
	if err != nil {
		t.Fatalf("convert response: %v", err)
	}
	if msg.ResponseMeta.FinishReason != "content_filter" {
		t.Errorf("finish reason=%q, want content_filter", msg.ResponseMeta.FinishReason)
	}
	// The text is still carried so the refusal shows up in logs and traces.
	if msg.Content != "抱歉，我无法协助。" {
		t.Errorf("content=%q, want the refusal text", msg.Content)
	}
}

// When both show up in one turn the tool call wins, so the caller keeps driving
// the task instead of discarding the call and retrying over the refusal.
func TestResponsesConvertResponseToolCallOutranksRefusal(t *testing.T) {
	resp := unmarshalResponse(t, `{
		"id": "resp_mixed",
		"status": "completed",
		"output": [
			{
				"type": "message",
				"id": "msg_1",
				"role": "assistant",
				"content": [{"type": "refusal", "refusal": "抱歉，我无法协助。"}]
			},
			{
				"type": "function_call",
				"id": "fc_1",
				"call_id": "call_1",
				"name": "write_chapter",
				"arguments": "{}"
			}
		]
	}`)

	msg, err := (&ResponsesChatModel{}).convertResponse(resp)
	if err != nil {
		t.Fatalf("convert response: %v", err)
	}
	if msg.ResponseMeta.FinishReason != "tool_calls" {
		t.Errorf("finish reason=%q, want tool_calls", msg.ResponseMeta.FinishReason)
	}
}

func TestResponsesConvertResponseFinishReasonForToolCalls(t *testing.T) {
	resp := unmarshalResponse(t, `{
		"id": "resp_4",
		"status": "completed",
		"output": [{
			"type": "function_call",
			"id": "fc_1",
			"call_id": "call_1",
			"name": "write_chapter",
			"arguments": "{}"
		}]
	}`)

	msg, err := (&ResponsesChatModel{}).convertResponse(resp)
	if err != nil {
		t.Fatalf("convert response: %v", err)
	}
	if msg.ResponseMeta.FinishReason != "tool_calls" {
		t.Errorf("finish reason=%q, want tool_calls", msg.ResponseMeta.FinishReason)
	}
}

func TestResponsesStreamBuilderFailedEventReturnsError(t *testing.T) {
	b := newStreamBuilder()

	_, _, err := b.processEvent(context.Background(), responses.ResponseStreamEventUnion{
		Type: "response.failed",
		Response: responses.Response{
			ID:     "resp_stream",
			Status: responses.ResponseStatusFailed,
			Error: responses.ResponseError{
				Code:    responses.ResponseErrorCodeRateLimitExceeded,
				Message: "Rate limit reached for gpt-5.",
			},
		},
	})
	if err == nil {
		t.Fatal("response.failed must terminate the stream with an error")
	}

	var failed *ResponsesFailedError
	if !errors.As(err, &failed) {
		t.Fatalf("error type=%T, want *ResponsesFailedError", err)
	}
	if failed.Code != responses.ResponseErrorCodeRateLimitExceeded {
		t.Errorf("code=%q, want rate_limit_exceeded", failed.Code)
	}
}

// collectStreamContent feeds events through the builder and joins whatever it
// chose to emit, mirroring how the caller concatenates the stream.
func collectStreamContent(t *testing.T, events ...responses.ResponseStreamEventUnion) string {
	t.Helper()
	b := newStreamBuilder()

	var content strings.Builder
	for _, event := range events {
		msg, emit, err := b.processEvent(context.Background(), event)
		if err != nil {
			t.Fatalf("process %s event: %v", event.Type, err)
		}
		if emit {
			content.WriteString(msg.Content)
		}
	}
	return content.String()
}

func TestResponsesStreamBuilderStreamsRefusalAsContent(t *testing.T) {
	got := collectStreamContent(t,
		responses.ResponseStreamEventUnion{Type: "response.refusal.delta", ItemID: "msg_1", Delta: "抱歉，"},
		responses.ResponseStreamEventUnion{Type: "response.refusal.delta", ItemID: "msg_1", Delta: "我无法协助。"},
		responses.ResponseStreamEventUnion{Type: "response.refusal.done", ItemID: "msg_1", Refusal: "抱歉，我无法协助。"},
	)

	// The done event repeats the full refusal, so emitting it again would double
	// the text the caller sees.
	if got != "抱歉，我无法协助。" {
		t.Errorf("streamed content=%q, want the refusal exactly once", got)
	}
}

func TestResponsesStreamBuilderEmitsRefusalWithoutDeltas(t *testing.T) {
	got := collectStreamContent(t,
		responses.ResponseStreamEventUnion{Type: "response.refusal.done", ItemID: "msg_1", Refusal: "抱歉，我无法协助。"},
	)

	if got != "抱歉，我无法协助。" {
		t.Errorf("streamed content=%q, want the refusal to fall back to the done event", got)
	}
}

func TestResponsesStreamBuilderRefusalMapsToContentFilter(t *testing.T) {
	b := newStreamBuilder()

	events := []responses.ResponseStreamEventUnion{
		{Type: "response.refusal.delta", ItemID: "msg_1", Delta: "抱歉，我无法协助。"},
		{Type: "response.refusal.done", ItemID: "msg_1", Refusal: "抱歉，我无法协助。"},
		{Type: "response.completed", Response: responses.Response{
			ID:     "resp_refusal",
			Status: responses.ResponseStatusCompleted,
		}},
	}
	for _, event := range events {
		if _, _, err := b.processEvent(context.Background(), event); err != nil {
			t.Fatalf("process %s event: %v", event.Type, err)
		}
	}

	msg, err := b.finish("")
	if err != nil {
		t.Fatalf("finish: %v", err)
	}
	if msg.ResponseMeta.FinishReason != "content_filter" {
		t.Errorf("finish reason=%q, want content_filter", msg.ResponseMeta.FinishReason)
	}
}

func TestResponsesStreamBuilderIncompleteEventMapsFinishReason(t *testing.T) {
	b := newStreamBuilder()

	if _, _, err := b.processEvent(context.Background(), responses.ResponseStreamEventUnion{
		Type: "response.incomplete",
		Response: responses.Response{
			ID:                "resp_stream",
			Status:            responses.ResponseStatusIncomplete,
			IncompleteDetails: responses.ResponseIncompleteDetails{Reason: "max_output_tokens"},
		},
	}); err != nil {
		t.Fatalf("process incomplete event: %v", err)
	}

	msg, err := b.finish("")
	if err != nil {
		t.Fatalf("finish: %v", err)
	}
	if msg.ResponseMeta.FinishReason != "length" {
		t.Errorf("finish reason=%q, want length", msg.ResponseMeta.FinishReason)
	}
}

// A stream that stops before any terminal event has been truncated in transit.
// Without an error the caller would accept the half-written text as a complete
// turn, because a concatenated stream carries no finish reason of its own.
func TestResponsesStreamBuilderFinishWithoutTerminalEventFails(t *testing.T) {
	b := newStreamBuilder()

	if _, _, err := b.processEvent(context.Background(), responses.ResponseStreamEventUnion{
		Type:  "response.output_text.delta",
		Delta: "半句话",
	}); err != nil {
		t.Fatalf("process delta event: %v", err)
	}

	if _, err := b.finish(""); err == nil {
		t.Fatal("a stream cut short before the terminal event must fail")
	}
}

func TestResponsesStreamBuilderIgnoresKnownInformationalEvents(t *testing.T) {
	b := newStreamBuilder()
	eventTypes := []string{
		"response.created",
		"response.in_progress",
		"response.queued",
		"response.content_part.added",
		"response.content_part.done",
		"response.output_text.done",
		"response.reasoning_summary_part.added",
		"response.reasoning_summary_part.done",
		"response.reasoning_summary_text.done",
		"keepalive",
		"response.metadata",
	}

	for _, eventType := range eventTypes {
		msg, emit, err := b.processEvent(context.Background(), responses.ResponseStreamEventUnion{Type: eventType})
		if err != nil {
			t.Errorf("process %s event: %v", eventType, err)
		}
		if emit || msg != nil {
			t.Errorf("event %s emitted msg=%#v, emit=%t", eventType, msg, emit)
		}
	}
}

func TestResponsesGenerateToolContinuation(t *testing.T) {
	requests := make(chan map[string]any, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			http.Error(w, "wrong endpoint", http.StatusNotFound)
			return
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		requests <- body
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"resp_tool","status":"completed","output":[
			{"id":"rs_1","type":"reasoning","summary":[],"encrypted_content":"cipher"},
			{"id":"fc_1","type":"function_call","call_id":"call_1","name":"lookup","arguments":"{\"city\":\"Paris\"}"}
		],"usage":{"input_tokens":100,"output_tokens":20,"total_tokens":120,"input_tokens_details":{"cached_tokens":50},"output_tokens_details":{"reasoning_tokens":12}}}`)
	}))
	defer server.Close()
	cm, err := NewResponsesChatModel(context.Background(), &ResponsesChatModelConfig{APIKey: "test", BaseURL: server.URL + "/v1", Model: "test-model"})
	if err != nil {
		t.Fatal(err)
	}
	bound, err := cm.WithTools([]*schema.ToolInfo{{Name: "lookup", Desc: "Find a city"}})
	if err != nil {
		t.Fatal(err)
	}
	history := []*schema.Message{schema.SystemMessage("system"), schema.UserMessage("find Paris")}
	msg, err := bound.Generate(context.Background(), history, model.WithMaxTokens(300), WithResponsesReasoningEffort(ResponsesReasoningEffortHigh))
	if err != nil {
		t.Fatal(err)
	}
	first := <-requests
	if first["model"] != "test-model" || first["max_output_tokens"] != float64(300) || first["reasoning"].(map[string]any)["effort"] != "high" {
		t.Fatalf("request=%v", first)
	}
	if _, exists := first["store"]; exists {
		t.Fatal("store default was overridden")
	}
	if !reflect.DeepEqual(first["include"], []any{"reasoning.encrypted_content"}) {
		t.Fatalf("include=%v", first["include"])
	}
	if len(first["tools"].([]any)) != 1 || len(cm.tools) != 0 {
		t.Fatal("WithTools must bind a copy")
	}
	if len(msg.ToolCalls) != 1 || msg.ToolCalls[0].ID != "call_1" || msg.ResponseMeta.FinishReason != "tool_calls" {
		t.Fatalf("message=%+v", msg)
	}
	if msg.ResponseMeta.Usage.CompletionTokensDetails.ReasoningTokens != 12 || msg.ResponseMeta.Usage.PromptTokenDetails.CachedTokens != 50 {
		t.Fatalf("usage=%+v", msg.ResponseMeta.Usage)
	}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	var restored schema.Message
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	history = append(history, &restored, schema.ToolMessage("sunny", "call_1"))
	if _, err := bound.Generate(context.Background(), history); err != nil {
		t.Fatal(err)
	}
	input := (<-requests)["input"].([]any)
	if len(input) != 5 || input[2].(map[string]any)["encrypted_content"] != "cipher" || input[3].(map[string]any)["type"] != "function_call" || input[4].(map[string]any)["type"] != "function_call_output" {
		t.Fatalf("input=%v", input)
	}
}

func TestResponsesRequestOptionsAndStoredContinuation(t *testing.T) {
	cm, err := NewResponsesChatModel(context.Background(), &ResponsesChatModelConfig{Model: "test"})
	if err != nil {
		t.Fatal(err)
	}
	previous := schema.AssistantMessage("first answer", nil)
	previous.Extra = map[string]any{ResponsesExtraKeyResponseID: "resp_previous"}
	history := []*schema.Message{schema.SystemMessage("new instructions"), schema.UserMessage("first"), previous, schema.UserMessage("second")}
	request, _, err := cm.buildRequest(history, WithResponsesUseResponseID(true), WithResponsesStore(true), WithResponsesPromptCacheKey("test-cache"), WithResponsesPromptCacheRetention(ResponsesPromptCacheRetention24h))
	if err != nil {
		t.Fatal(err)
	}
	if request.PreviousResponseID.Value != "resp_previous" || request.Instructions.Value != "new instructions" || len(request.Input.OfInputItemList) != 1 || !request.Store.Value {
		t.Fatalf("continuation=%+v", request)
	}
	if request.PromptCacheKey.Value != "test-cache" || request.PromptCacheRetention != ResponsesPromptCacheRetention24h {
		t.Fatalf("cache options=%+v", request)
	}
	request, _, err = cm.buildRequest(history)
	if err != nil {
		t.Fatal(err)
	}
	if request.PreviousResponseID.Valid() || len(request.Input.OfInputItemList) != 4 {
		t.Fatal("response IDs must be opt-in")
	}
}

func TestResponsesImageInputAndToolChoice(t *testing.T) {
	cm, err := NewResponsesChatModel(context.Background(), &ResponsesChatModelConfig{Model: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if err := cm.BindTools([]*schema.ToolInfo{{Name: "lookup"}}); err != nil {
		t.Fatal(err)
	}
	history := []*schema.Message{{Role: schema.User, MultiContent: []schema.ChatMessagePart{
		{Type: schema.ChatMessagePartTypeText, Text: "describe"},
		{Type: schema.ChatMessagePartTypeImageURL, ImageURL: &schema.ChatMessageImageURL{URL: "https://example.com/image.png"}},
	}}}
	request, _, err := cm.buildRequest(history, model.WithToolChoice(schema.ToolChoiceForced, "lookup"))
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatal(err)
	}
	content := body["input"].([]any)[0].(map[string]any)["content"].([]any)
	if len(content) != 2 || content[1].(map[string]any)["image_url"] != "https://example.com/image.png" {
		t.Fatalf("content=%v", content)
	}
	choice := body["tool_choice"].(map[string]any)
	if choice["type"] != "function" || choice["name"] != "lookup" {
		t.Fatalf("tool choice=%v", choice)
	}
}

func TestResponsesStreamToolAndReasoning(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for _, event := range []string{
			`{"type":"response.output_item.done","item":{"type":"reasoning","id":"rs_stream","summary":[],"encrypted_content":"cipher"}}`,
			`{"type":"response.output_item.added","item":{"type":"function_call","id":"fc_stream","call_id":"call_stream","name":"lookup","arguments":""}}`,
			`{"type":"response.function_call_arguments.delta","item_id":"fc_stream","delta":"{\"x\":"}`,
			`{"type":"response.function_call_arguments.delta","item_id":"fc_stream","delta":"1}"}`,
			`{"type":"response.output_item.done","item":{"type":"function_call","id":"fc_stream","call_id":"call_stream","name":"lookup","arguments":"{\"x\":1}"}}`,
			`{"type":"response.completed","response":{"id":"resp_stream","status":"completed","output":[],"usage":{"input_tokens":12,"output_tokens":7,"total_tokens":19}}}`,
		} {
			_, _ = io.WriteString(w, "data: "+event+"\n\n")
		}
	}))
	defer server.Close()
	cm, err := NewResponsesChatModel(context.Background(), &ResponsesChatModelConfig{Model: "test", APIKey: "test", BaseURL: server.URL + "/"})
	if err != nil {
		t.Fatal(err)
	}
	stream, err := cm.Stream(context.Background(), []*schema.Message{schema.UserMessage("lookup")})
	if err != nil {
		t.Fatal(err)
	}
	msg, err := schema.ConcatMessageStream(stream)
	if err != nil {
		t.Fatal(err)
	}
	if len(msg.ToolCalls) != 1 || msg.ToolCalls[0].ID != "call_stream" || msg.ToolCalls[0].Function.Arguments != `{"x":1}` || msg.ResponseMeta.FinishReason != "tool_calls" {
		t.Fatalf("stream result=%+v", msg)
	}
	input, err := messageToInputItems(msg)
	if err != nil || len(input) != 2 {
		t.Fatalf("stream continuation input=%v error=%v", input, err)
	}
}

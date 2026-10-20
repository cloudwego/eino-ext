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
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/openai/openai-go/v3/responses"
)

const streamEventCompleted = `{"type":"response.completed","response":{"id":"resp_ok","status":"completed","output":[],"usage":{"input_tokens":3,"output_tokens":2,"total_tokens":5}}}`

func TestResponsesStreamEventHandler(t *testing.T) {
	limitErr := errors.New("gateway_limit")
	proxyErr := errors.New("gateway_upstream")
	handler := func(_ context.Context, event responses.ResponseStreamEventUnion) ResponsesStreamEventResult {
		switch event.Type {
		case "gateway.limit":
			return ResponsesStreamEventResult{IncompleteError: limitErr}
		case "gateway.error":
			var payload struct {
				Code string `json:"code"`
			}
			if err := json.Unmarshal([]byte(event.RawJSON()), &payload); err != nil {
				return ResponsesStreamEventResult{Error: err}
			}
			return ResponsesStreamEventResult{Error: fmt.Errorf("%w: %s", proxyErr, payload.Code)}
		}
		return ResponsesStreamEventResult{}
	}
	text := `{"type":"response.output_text.delta","delta":"ok"}`
	limit := `{"type":"gateway.limit"}`
	for _, tc := range []struct {
		name           string
		events         []string
		hook           ResponsesStreamEventHandler
		wantErr        error
		genericErr     bool
		transportError bool
		wantText       string
	}{
		{name: "normal", events: []string{text, streamEventCompleted}, hook: handler, wantText: "ok"},
		{name: "nil hook", events: []string{limit, text, streamEventCompleted}, wantText: "ok"},
		{name: "immediate error", events: []string{`{"type":"gateway.error","code":"overloaded"}`, text, streamEventCompleted}, hook: handler, wantErr: proxyErr},
		{name: "limit without terminal", events: []string{limit}, hook: handler, wantErr: limitErr},
		{name: "limit before success", events: []string{limit, text, streamEventCompleted}, hook: handler, wantText: "ok"},
		{name: "limit after success", events: []string{text, streamEventCompleted, limit}, hook: handler, wantText: "ok"},
		{name: "plain truncated stream", events: []string{text}, hook: handler, genericErr: true},
		{name: "transport error wins", events: []string{limit}, hook: handler, genericErr: true, transportError: true},
		{name: "standard error wins", events: []string{limit, `{"type":"error","code":"invalid_request","message":"bad input"}`}, hook: handler, genericErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				if tc.transportError {
					w.Header().Set("Content-Length", "99999")
				}
				for _, event := range tc.events {
					_, _ = io.WriteString(w, "data: "+event+"\n\n")
				}
			}))
			defer server.Close()
			cm, err := NewResponsesChatModel(context.Background(), &ResponsesChatModelConfig{Model: "test", APIKey: "test", BaseURL: server.URL + "/", StreamEventHandler: tc.hook})
			if err != nil {
				t.Fatal(err)
			}
			stream, err := cm.Stream(context.Background(), []*schema.Message{schema.UserMessage("hi")})
			if err != nil {
				t.Fatal(err)
			}
			msg, err := schema.ConcatMessageStream(stream)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("error=%v, want %v", err, tc.wantErr)
				}
			} else if tc.genericErr {
				if err == nil || errors.Is(err, limitErr) {
					t.Fatalf("expected native error, got %v", err)
				}
			} else if err != nil || msg.Content != tc.wantText || msg.ResponseMeta.FinishReason != "completed" || msg.ResponseMeta.Usage.TotalTokens != 5 {
				t.Fatalf("message=%+v error=%v", msg, err)
			}
		})
	}
}

func TestResponsesStreamEventErrorClosesConnection(t *testing.T) {
	sentinel := errors.New("stop_now")
	closed := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"type\":\"gateway.error\"}\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		close(closed)
	}))
	defer server.Close()
	cm, err := NewResponsesChatModel(context.Background(), &ResponsesChatModelConfig{Model: "test", APIKey: "test", BaseURL: server.URL + "/", StreamEventHandler: func(context.Context, responses.ResponseStreamEventUnion) ResponsesStreamEventResult {
		return ResponsesStreamEventResult{Error: sentinel}
	}})
	if err != nil {
		t.Fatal(err)
	}
	stream, err := cm.Stream(context.Background(), []*schema.Message{schema.UserMessage("hi")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := schema.ConcatMessageStream(stream); !errors.Is(err, sentinel) {
		t.Fatalf("error=%v", err)
	}
	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("handler error did not close upstream connection")
	}
}

func TestResponsesStreamEventFallbackIsPerStream(t *testing.T) {
	sentinel := errors.New("limited")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Input []struct {
				Content string `json:"content"`
			} `json:"input"`
		}
		_ = json.NewDecoder(r.Body).Decode(&request)
		w.Header().Set("Content-Type", "text/event-stream")
		if request.Input[0].Content == "limited" {
			_, _ = io.WriteString(w, "data: {\"type\":\"gateway.limit\"}\n\n")
		}
		// Both streams end without a terminal response; only one has a fallback.
	}))
	defer server.Close()
	cm, err := NewResponsesChatModel(context.Background(), &ResponsesChatModelConfig{Model: "test", APIKey: "test", BaseURL: server.URL + "/", StreamEventHandler: func(_ context.Context, event responses.ResponseStreamEventUnion) ResponsesStreamEventResult {
		if event.Type == "gateway.limit" {
			return ResponsesStreamEventResult{IncompleteError: sentinel}
		}
		return ResponsesStreamEventResult{}
	}})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		limited := i%2 == 0
		wg.Add(1)
		go func() {
			defer wg.Done()
			prompt := "normal"
			if limited {
				prompt = "limited"
			}
			stream, err := cm.Stream(context.Background(), []*schema.Message{schema.UserMessage(prompt)})
			if err != nil {
				t.Error(err)
				return
			}
			_, err = schema.ConcatMessageStream(stream)
			if err == nil || errors.Is(err, sentinel) != limited {
				t.Errorf("limited=%v error=%v", limited, err)
			}
		}()
	}
	wg.Wait()
}

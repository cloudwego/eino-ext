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

package openai_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func TestReasoningEffort(t *testing.T) {
	tests := []struct {
		name       string
		config     openai.ReasoningEffortLevel
		options    []model.Option
		want       string
		wantConfig string
	}{
		{name: "unset"},
		{name: "config_none", config: openai.ReasoningEffortLevelNone, want: "none"},
		{name: "config_low", config: openai.ReasoningEffortLevelLow, want: "low"},
		{name: "config_medium", config: openai.ReasoningEffortLevelMedium, want: "medium"},
		{name: "config_high", config: openai.ReasoningEffortLevelHigh, want: "high"},
		{name: "config_xhigh", config: openai.ReasoningEffortLevelXHigh, want: "xhigh"},
		{name: "config_custom", config: "provider-specific", want: "provider-specific"},
		{
			name:       "option_none",
			config:     openai.ReasoningEffortLevelHigh,
			options:    []model.Option{openai.WithReasoningEffort(openai.ReasoningEffortLevelNone)},
			want:       "none",
			wantConfig: "high",
		},
		{
			name:       "option_xhigh",
			config:     openai.ReasoningEffortLevelNone,
			options:    []model.Option{openai.WithReasoningEffort(openai.ReasoningEffortLevelXHigh)},
			want:       "xhigh",
			wantConfig: "none",
		},
		{
			name:       "option_empty",
			config:     openai.ReasoningEffortLevelHigh,
			options:    []model.Option{openai.WithReasoningEffort("")},
			wantConfig: "high",
		},
		{
			name:       "option_custom",
			config:     openai.ReasoningEffortLevelHigh,
			options:    []model.Option{openai.WithReasoningEffort("provider-specific")},
			want:       "provider-specific",
			wantConfig: "high",
		},
	}

	for _, method := range []string{"Generate", "Stream"} {
		t.Run(method, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					requests := make(chan map[string]json.RawMessage, 1)
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if r.Method != http.MethodPost || r.URL.Path != "/chat/completions" {
							t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
							http.Error(w, "unexpected request", http.StatusNotFound)
							return
						}
						var body map[string]json.RawMessage
						if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
							t.Errorf("decode request: %v", err)
							http.Error(w, "invalid JSON", http.StatusBadRequest)
							return
						}
						requests <- body

						var response string
						if method == "Stream" {
							w.Header().Set("Content-Type", "text/event-stream")
							response = "data: {\"id\":\"test\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"ok\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n"
						} else {
							w.Header().Set("Content-Type", "application/json")
							response = `{"id":"test","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`
						}
						if _, err := io.WriteString(w, response); err != nil {
							t.Errorf("write response: %v", err)
						}
					}))
					t.Cleanup(server.Close)

					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					t.Cleanup(cancel)
					cm, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
						APIKey:          "test",
						BaseURL:         server.URL,
						HTTPClient:      server.Client(),
						Model:           "test-model",
						ReasoningEffort: tt.config,
					})
					if err != nil {
						t.Fatalf("NewChatModel: %v", err)
					}

					call := func(want string, options ...model.Option) {
						t.Helper()
						messages := []*schema.Message{schema.UserMessage("hello")}
						var content string
						if method == "Stream" {
							stream, err := cm.Stream(ctx, messages, options...)
							if err != nil {
								t.Fatalf("Stream: %v", err)
							}
							defer stream.Close()
							for {
								message, err := stream.Recv()
								if err == io.EOF {
									break
								}
								if err != nil {
									t.Fatalf("Recv: %v", err)
								}
								content += message.Content
							}
						} else {
							message, err := cm.Generate(ctx, messages, options...)
							if err != nil {
								t.Fatalf("Generate: %v", err)
							}
							content = message.Content
						}
						if content != "ok" {
							t.Fatalf("response content = %q, want %q", content, "ok")
						}

						select {
						case request := <-requests:
							raw, present := request["reasoning_effort"]
							if want == "" {
								if present {
									t.Fatalf("reasoning_effort = %s, want field omitted", raw)
								}
								return
							}
							var got string
							if err := json.Unmarshal(raw, &got); err != nil {
								t.Fatalf("decode reasoning_effort: %v", err)
							}
							if got != want {
								t.Fatalf("reasoning_effort = %q, want %q", got, want)
							}
						default:
							t.Fatal("no serialized request captured")
						}
					}

					call(tt.want, tt.options...)
					if len(tt.options) > 0 {
						call(tt.wantConfig)
					}
				})
			}
		})
	}
}

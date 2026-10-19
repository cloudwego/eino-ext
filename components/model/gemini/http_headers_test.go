/*
 * Copyright 2024 CloudWeGo Authors
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

package gemini_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cloudwego/eino-ext/components/model/gemini"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"google.golang.org/genai"
)

// Exercise the real adapter and SDK against a local endpoint so SDK validation
// and protocol conversion cannot drop the header or put it into the JSON body.
func TestWithCustomHeaders(t *testing.T) {
	for _, backend := range []genai.Backend{genai.BackendGeminiAPI, genai.BackendVertexAI} {
		t.Run(fmt.Sprint(backend), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body struct {
					GatewayFlag    bool              `json:"gateway_flag"`
					PromptCacheKey *string           `json:"prompt_cache_key"`
					Labels         map[string]string `json:"labels"`
					Contents       []*genai.Content  `json:"contents"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Errorf("decode request: %v", err)
					http.Error(w, "invalid JSON", http.StatusBadRequest)
					return
				}
				if len(body.Contents) != 1 || len(body.Contents[0].Parts) != 1 {
					t.Errorf("native contents were not preserved: %+v", body.Contents)
					http.Error(w, "missing contents", http.StatusBadRequest)
					return
				}
				projectID := body.Contents[0].Parts[0].Text
				wantKey := projectID + ":creation"
				if projectID == "without-option" {
					wantKey = ""
				}
				if got := r.Header.Get("X-Request-Key"); got != wantKey {
					t.Errorf("project %q: header = %q, want %q", projectID, got, wantKey)
				}
				if r.Header.Get("X-Client-Default") != "preserved" {
					t.Errorf("client header was not preserved")
				}
				wantOverride := "request-value"
				if projectID == "without-option" {
					wantOverride = "client-value"
				}
				if got := r.Header.Get("X-Override"); got != wantOverride {
					t.Errorf("override header = %q, want %q", got, wantOverride)
				}
				if body.GatewayFlag != (projectID != "without-option") {
					t.Errorf("header and extra body options did not coexist correctly: %+v", body)
				}
				if body.PromptCacheKey != nil || body.Labels != nil {
					t.Errorf("cache key must not be sent in the JSON body: %+v", body)
				}
				response := `{"candidates":[{"content":{"role":"model","parts":[{"text":"ok"}]},"finishReason":"STOP"}]}`
				if strings.HasSuffix(r.URL.Path, ":streamGenerateContent") {
					w.Header().Set("Content-Type", "text/event-stream")
					fmt.Fprintf(w, "data: %s\n\n", response)
				} else {
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprint(w, response)
				}
			}))
			defer server.Close()
			ctx := context.Background()
			client, err := genai.NewClient(ctx, &genai.ClientConfig{
				Backend:     backend,
				APIKey:      "local-test-key",
				HTTPOptions: genai.HTTPOptions{BaseURL: server.URL, APIVersion: "v1beta", Headers: http.Header{"X-Client-Default": {"preserved"}, "X-Override": {"client-value"}}},
			})
			if err != nil {
				t.Fatal(err)
			}
			chat, err := gemini.NewChatModel(ctx, &gemini.Config{Client: client, Model: "gemini-test"})
			if err != nil {
				t.Fatal(err)
			}
			call := func(t *testing.T, projectID string, stream bool) {
				t.Helper()
				var opts []model.Option
				if projectID != "without-option" {
					headers := http.Header{
						"X-Request-Key": {projectID + ":creation"},
						"x-override":    {"request-value"},
					}
					opts = []model.Option{
						gemini.WithCustomHeaders(headers),
						gemini.WithExtraFields(map[string]any{"gateway_flag": true}),
					}
					// The option must snapshot headers, including value slices.
					headers["X-Request-Key"][0] = "mutated-after-option"
					headers.Set("x-override", "mutated-after-option")
				}
				messages := []*schema.Message{schema.UserMessage(projectID)}
				if !stream {
					result, err := chat.Generate(ctx, messages, opts...)
					if err != nil {
						t.Fatal(err)
					}
					if result.Content != "ok" {
						t.Fatalf("unexpected response: %+v", result)
					}
					return
				}
				reader, err := chat.Stream(ctx, messages, opts...)
				if err != nil {
					t.Fatal(err)
				}
				defer reader.Close()
				var content strings.Builder
				for {
					chunk, err := reader.Recv()
					if err == io.EOF {
						break
					}
					if err != nil {
						t.Fatal(err)
					}
					content.WriteString(chunk.Content)
				}
				if content.String() != "ok" {
					t.Fatalf("unexpected streamed response: %q", content.String())
				}
			}
			call(t, "project-generate", false)
			call(t, "project-stream", true)
			t.Run("concurrent-projects", func(t *testing.T) {
				for i := range 12 {
					t.Run(fmt.Sprint(i), func(t *testing.T) {
						t.Parallel()
						call(t, fmt.Sprintf("project-%d", i), i%2 == 0)
					})
				}
			})
			call(t, "without-option", false)
			call(t, "without-option", true)
		})
	}
}

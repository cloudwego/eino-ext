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
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/components/model/openai"
)

func TestResponseID(t *testing.T) {
	const responseID = "chatcmpl-response-id"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Stream bool `json:"stream"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("X-Request-ID", "req-http-request-id")
		if !request.Stream {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"id":%q,"choices":[{"index":0,"message":{"role":"assistant","content":"hello world"},"finish_reason":"stop"}]}`, responseID)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		for _, delta := range []string{`{"role":"assistant"}`, `{"content":"hello "}`, `{"content":"world"}`} {
			fmt.Fprintf(w, "data: {\"id\":%q,\"choices\":[{\"index\":0,\"delta\":%s}]}\n\n", responseID, delta)
		}
		fmt.Fprintf(w, "data: {\"id\":%q,\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n", responseID)
		fmt.Fprintf(w, "data: {\"id\":%q,\"choices\":[],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":2,\"total_tokens\":3}}\n\n", responseID)
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	ctx := context.Background()
	model, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey: "test-key", Model: "test-model", BaseURL: server.URL + "/v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	input := []*schema.Message{schema.UserMessage("hello")}
	checkID := func(t *testing.T, msg *schema.Message) {
		t.Helper()
		if got := openai.GetResponseID(msg); got != responseID {
			t.Fatalf("response ID = %q, want %q", got, responseID)
		}
	}
	t.Run("generate", func(t *testing.T) {
		msg, err := model.Generate(ctx, input)
		if err != nil {
			t.Fatal(err)
		}
		checkID(t, msg)
	})
	t.Run("stream and concatenate", func(t *testing.T) {
		stream, err := model.Stream(ctx, input)
		if err != nil {
			t.Fatal(err)
		}
		defer stream.Close()
		var chunks []*schema.Message
		for {
			chunk, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			checkID(t, chunk)
			chunks = append(chunks, chunk)
		}
		if len(chunks) < 2 {
			t.Fatalf("expected multiple chunks, got %d", len(chunks))
		}
		msg, err := schema.ConcatMessages(chunks)
		if err != nil {
			t.Fatal(err)
		}
		checkID(t, msg)
		if msg.Content != "hello world" || msg.ResponseMeta.Usage.TotalTokens != 3 {
			t.Fatalf("unexpected concatenated message: %+v", msg)
		}
	})
}

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

package qwen

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// generateRequestBody sends one Generate call to a local server and returns
// the JSON body the client posted.
func generateRequestBody(t *testing.T, config *ChatModelConfig, opts ...model.Option) map[string]any {
	t.Helper()
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"1","object":"chat.completion","model":"qwen-plus",`+
			`"choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`)
	}))
	defer srv.Close()

	config.BaseURL = srv.URL
	config.APIKey = "test"
	config.Model = "qwen-plus"
	cm, err := NewChatModel(context.Background(), config)
	if err != nil {
		t.Fatalf("NewChatModel: %v", err)
	}
	if _, err := cm.Generate(context.Background(), []*schema.Message{schema.UserMessage("hi")}, opts...); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	return body
}

func TestExtraFieldsMergeWithEnableThinking(t *testing.T) {
	enable := true

	t.Run("config EnableThinking keeps caller extra fields", func(t *testing.T) {
		body := generateRequestBody(t, &ChatModelConfig{EnableThinking: &enable},
			WithExtraFields(map[string]any{"enable_search": true}))

		if body["enable_search"] != true {
			t.Errorf("enable_search dropped from request body: %v", body)
		}
		if body["enable_thinking"] != true {
			t.Errorf("enable_thinking missing from request body: %v", body)
		}
		if _, ok := body["chat_template_kwargs"]; !ok {
			t.Errorf("chat_template_kwargs missing from request body: %v", body)
		}
	})

	t.Run("option EnableThinking keeps caller extra fields", func(t *testing.T) {
		body := generateRequestBody(t, &ChatModelConfig{},
			WithExtraFields(map[string]any{"enable_search": true}), WithEnableThinking(false))

		if body["enable_search"] != true {
			t.Errorf("enable_search dropped from request body: %v", body)
		}
		if body["enable_thinking"] != false {
			t.Errorf("enable_thinking = %v, want false", body["enable_thinking"])
		}
	})

	t.Run("caller extra fields override thinking fields", func(t *testing.T) {
		body := generateRequestBody(t, &ChatModelConfig{EnableThinking: &enable},
			WithExtraFields(map[string]any{"enable_thinking": false}))

		if body["enable_thinking"] != false {
			t.Errorf("enable_thinking = %v, want caller's false", body["enable_thinking"])
		}
	})

	t.Run("extra fields without thinking", func(t *testing.T) {
		body := generateRequestBody(t, &ChatModelConfig{},
			WithExtraFields(map[string]any{"enable_search": true}))

		if body["enable_search"] != true {
			t.Errorf("enable_search dropped from request body: %v", body)
		}
		if _, ok := body["enable_thinking"]; ok {
			t.Errorf("enable_thinking should not be set: %v", body)
		}
	})

	t.Run("caller map is not modified", func(t *testing.T) {
		fields := map[string]any{"enable_search": true}
		_ = generateRequestBody(t, &ChatModelConfig{EnableThinking: &enable}, WithExtraFields(fields))

		if len(fields) != 1 {
			t.Errorf("caller's map was modified: %v", fields)
		}
	})
}

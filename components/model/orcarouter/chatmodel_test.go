/*
 * Copyright 2025 CloudWeGo Authors
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

package orcarouter

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloudwego/eino/schema"
)

func TestNewChatModelNilConfig(t *testing.T) {
	_, err := NewChatModel(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config cannot be nil")
}

func TestNewChatModelDefaultBaseURL(t *testing.T) {
	// defaultBaseURL must point at OrcaRouter's OpenAI-compatible endpoint.
	assert.Equal(t, "https://api.orcarouter.ai/v1", defaultBaseURL)

	cm, err := NewChatModel(context.Background(), &Config{APIKey: "sk-orca-test", Model: "anthropic/claude-haiku-4.5"})
	require.NoError(t, err)
	assert.NotNil(t, cm)
	assert.Equal(t, "OrcaRouter", cm.GetType())
}

func TestGenerate(t *testing.T) {
	var gotPath string
	var gotAuth string
	var gotBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "chatcmpl-test",
			"object": "chat.completion",
			"created": 1677652288,
			"model": "anthropic/claude-haiku-4.5",
			"choices": [{
				"index": 0,
				"message": {"role": "assistant", "content": "Hello from OrcaRouter!"},
				"finish_reason": "stop"
			}],
			"usage": {"prompt_tokens": 9, "completion_tokens": 12, "total_tokens": 21}
		}`))
	}))
	defer srv.Close()

	cm, err := NewChatModel(context.Background(), &Config{
		APIKey:  "sk-orca-test",
		BaseURL: srv.URL,
		Model:   "anthropic/claude-haiku-4.5",
	})
	require.NoError(t, err)

	out, err := cm.Generate(context.Background(), []*schema.Message{
		{Role: schema.User, Content: "hi"},
	})
	require.NoError(t, err)
	assert.Equal(t, "Hello from OrcaRouter!", out.Content)
	assert.Equal(t, schema.Assistant, out.Role)
	assert.Equal(t, "stop", out.ResponseMeta.FinishReason)
	assert.Equal(t, 21, out.ResponseMeta.Usage.TotalTokens)

	// Request shape: OpenAI-compatible chat/completions path + bearer auth + model/messages.
	assert.Equal(t, "/chat/completions", gotPath)
	assert.Equal(t, "Bearer sk-orca-test", gotAuth)
	assert.Equal(t, "anthropic/claude-haiku-4.5", gotBody["model"])
	msgs := gotBody["messages"].([]any)
	assert.Equal(t, "user", msgs[0].(map[string]any)["role"])
	assert.Equal(t, "hi", msgs[0].(map[string]any)["content"])
}

func TestGenerateReasoningContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "chatcmpl-test",
			"object": "chat.completion",
			"created": 1677652288,
			"model": "anthropic/claude-sonnet-5",
			"choices": [{
				"index": 0,
				"message": {"role": "assistant", "content": "Final answer", "reasoning_content": "Let me think step by step."},
				"finish_reason": "stop"
			}],
			"usage": {"prompt_tokens": 9, "completion_tokens": 12, "total_tokens": 21}
		}`))
	}))
	defer srv.Close()

	cm, err := NewChatModel(context.Background(), &Config{APIKey: "sk-orca-test", BaseURL: srv.URL, Model: "anthropic/claude-sonnet-5"})
	require.NoError(t, err)

	out, err := cm.Generate(context.Background(), []*schema.Message{{Role: schema.User, Content: "1+1?"}})
	require.NoError(t, err)
	assert.Equal(t, "Final answer", out.Content)
	// OrcaRouter's OpenAI-compatible endpoint surfaces thinking models' reasoning via reasoning_content.
	assert.Equal(t, "Let me think step by step.", out.ReasoningContent)
}

func TestStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(
			"data: {\"id\":\"chatcmpl-test\",\"object\":\"chat.completion.chunk\",\"created\":1677652288,\"model\":\"anthropic/claude-haiku-4.5\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"Hello\"},\"finish_reason\":null}]}\n\n" +
				"data: {\"id\":\"chatcmpl-test\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\" world\"},\"finish_reason\":null}]}\n\n" +
				"data: {\"id\":\"chatcmpl-test\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":9,\"completion_tokens\":2,\"total_tokens\":11}}\n\n" +
				"data: [DONE]\n\n",
		))
	}))
	defer srv.Close()

	cm, err := NewChatModel(context.Background(), &Config{APIKey: "sk-orca-test", BaseURL: srv.URL, Model: "anthropic/claude-haiku-4.5"})
	require.NoError(t, err)

	stream, err := cm.Stream(context.Background(), []*schema.Message{{Role: schema.User, Content: "hi"}})
	require.NoError(t, err)
	defer stream.Close()

	var contents []string
	for {
		msg, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("stream recv failed: %v", err)
		}
		if msg.Content != "" {
			contents = append(contents, msg.Content)
		}
	}

	assert.Equal(t, []string{"Hello", " world"}, contents)
}

func TestWithTools(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "chatcmpl-test",
			"object": "chat.completion",
			"created": 1677652288,
			"model": "anthropic/claude-haiku-4.5",
			"choices": [{
				"index": 0,
				"message": {
					"role": "assistant",
					"content": null,
					"tool_calls": [{
						"id": "call_1",
						"type": "function",
						"function": {"name": "get_weather", "arguments": "{\"city\":\"Shanghai\"}"}
					}]
				},
				"finish_reason": "tool_calls"
			}],
			"usage": {"prompt_tokens": 20, "completion_tokens": 10, "total_tokens": 30}
		}`))
	}))
	defer srv.Close()

	cm, err := NewChatModel(context.Background(), &Config{APIKey: "sk-orca-test", BaseURL: srv.URL, Model: "anthropic/claude-haiku-4.5"})
	require.NoError(t, err)

	toolCallee, err := cm.WithTools([]*schema.ToolInfo{
		{
			Name: "get_weather",
			Desc: "Get the weather for a city",
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"city": {Type: schema.String, Required: true},
			}),
		},
	})
	require.NoError(t, err)

	out, err := toolCallee.Generate(context.Background(), []*schema.Message{{Role: schema.User, Content: "weather in Shanghai?"}})
	require.NoError(t, err)
	require.Len(t, out.ToolCalls, 1)
	assert.Equal(t, "get_weather", out.ToolCalls[0].Function.Name)
	assert.Equal(t, "tool_calls", out.ResponseMeta.FinishReason)

	// tools must be attached to the request body.
	tools := gotBody["tools"].([]any)
	assert.Equal(t, "get_weather", tools[0].(map[string]any)["function"].(map[string]any)["name"])
}

func TestBindForcedTools(t *testing.T) {
	cm, err := NewChatModel(context.Background(), &Config{APIKey: "sk-orca-test", BaseURL: "http://127.0.0.1:1", Model: "anthropic/claude-haiku-4.5"})
	require.NoError(t, err)

	err = cm.BindForcedTools([]*schema.ToolInfo{
		{
			Name: "get_weather",
			Desc: "Get the weather for a city",
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"city": {Type: schema.String, Required: true},
			}),
		},
	})
	require.NoError(t, err)

	// empty tool list must error
	err = cm.BindForcedTools(nil)
	require.Error(t, err)
}

func TestGetType(t *testing.T) {
	cm := &ChatModel{}
	assert.Equal(t, "OrcaRouter", cm.GetType())
}

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

package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/bytedance/mockey"
	"github.com/eino-contrib/jsonschema"
	"github.com/meguminnnnnnnnn/go-openai"
	orderedmap "github.com/wk8/go-ordered-map/v2"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	protocol "github.com/cloudwego/eino-ext/libs/acl/openai"
)

func TestOpenAIGenerate(t *testing.T) {
	js := &jsonschema.Schema{
		Type: string(schema.Object),
		Properties: orderedmap.New[string, *jsonschema.Schema](
			orderedmap.WithInitialData[string, *jsonschema.Schema](
				orderedmap.Pair[string, *jsonschema.Schema]{
					Key: "a",
					Value: &jsonschema.Schema{
						Type: string(schema.String),
					},
				},
				orderedmap.Pair[string, *jsonschema.Schema]{
					Key: "b",
					Value: &jsonschema.Schema{
						Type: string(schema.Integer),
					},
				},
			),
		),
	}

	expectedSeed := 4
	mockToolCallIdx := 5
	var temperature float32 = 0.1
	expectedRequestBody := openai.ChatCompletionRequest{
		Model: "gpt-4",
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "system",
			},
			{
				Role: openai.ChatMessageRoleUser,
				MultiContent: []openai.ChatMessagePart{
					{
						Type: openai.ChatMessagePartTypeImageURL,
						ImageURL: &openai.ChatMessageImageURL{
							URL:    "http://a.b.c",
							Detail: "detail",
						},
					},
					{
						Type: openai.ChatMessagePartTypeText,
						Text: "text",
					},
				},
			},
			{
				Role:       openai.ChatMessageRoleTool,
				Content:    "tool",
				ToolCallID: "tool call id",
			},
		},
		MaxTokens:           1,
		MaxCompletionTokens: 1,
		Temperature:         &temperature,
		TopP:                0.2,
		Stream:              false,
		Stop:                []string{"stop"},
		PresencePenalty:     0.3,
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		},
		Seed:             &expectedSeed,
		FrequencyPenalty: 0.4,
		LogitBias:        map[string]int{"1024": 100},
		User:             "megumin",
		Tools: []openai.Tool{
			{
				Type: openai.ToolTypeFunction,
				Function: &openai.FunctionDefinition{
					Name:        "tool1",
					Description: "tool1",
					Parameters:  js,
				},
			},
			{
				Type: openai.ToolTypeFunction,
				Function: &openai.FunctionDefinition{
					Name:        "tool2",
					Description: "tool2",
					Parameters:  js,
				},
			},
		},
		ToolChoice: "required",
	}
	mockOpenAIResponse := openai.ChatCompletionResponse{
		ID: "request id",
		Choices: []openai.ChatCompletionChoice{
			{
				Index: 0,
				Message: openai.ChatCompletionMessage{
					Role:    openai.ChatMessageRoleAssistant,
					Content: "response content",
					Name:    "response name",
					ToolCalls: []openai.ToolCall{
						{
							Index: &mockToolCallIdx,
							ID:    "id",
							Type:  openai.ToolTypeFunction,
							Function: openai.FunctionCall{
								Name:      "name",
								Arguments: "arguments",
							},
						},
					},
				},
			},
		},
		Usage: openai.Usage{
			PromptTokens:     1,
			CompletionTokens: 2,
			TotalTokens:      3,
		},
	}
	expectedMessages := &schema.Message{
		Role:    schema.Assistant,
		Content: "response content",
		Name:    "response name",
		ToolCalls: []schema.ToolCall{
			{
				Index: &mockToolCallIdx,
				ID:    "id",
				Type:  "function",
				Function: schema.FunctionCall{
					Name:      "name",
					Arguments: "arguments",
				},
			},
		},
		ResponseMeta: &schema.ResponseMeta{
			Usage: &schema.TokenUsage{
				PromptTokens:     1,
				CompletionTokens: 2,
				TotalTokens:      3,
			},
		},
		Extra: map[string]any{
			"openai-request-id": "request id",
		},
	}
	config := &ChatModelConfig{
		ByAzure:             false,
		BaseURL:             "",
		APIVersion:          "",
		APIKey:              "",
		Timeout:             0,
		Model:               "gpt-4",
		MaxTokens:           &expectedRequestBody.MaxTokens,
		MaxCompletionTokens: &expectedRequestBody.MaxCompletionTokens,
		Temperature:         expectedRequestBody.Temperature,
		TopP:                &expectedRequestBody.TopP,
		Stop:                expectedRequestBody.Stop,
		PresencePenalty:     &expectedRequestBody.PresencePenalty,
		ResponseFormat: &protocol.ChatCompletionResponseFormat{
			Type: protocol.ChatCompletionResponseFormatTypeJSONObject,
		},
		Seed:             expectedRequestBody.Seed,
		FrequencyPenalty: &expectedRequestBody.FrequencyPenalty,
		LogitBias:        expectedRequestBody.LogitBias,
		User:             &expectedRequestBody.User,
	}

	t.Run("all param", func(t *testing.T) {
		defer mockey.Mock((*openai.Client).CreateChatCompletion).To(func(ctx context.Context,
			request openai.ChatCompletionRequest, opts ...openai.ChatCompletionRequestOption) (response openai.ChatCompletionResponse, err error) {
			return mockOpenAIResponse, nil
		}).Build().UnPatch()
		ctx := context.Background()
		m, err := NewChatModel(ctx, config)
		if err != nil {
			t.Fatal(err)
		}
		err = m.BindForcedTools([]*schema.ToolInfo{
			{
				Name:        "tool1",
				Desc:        "tool1",
				ParamsOneOf: schema.NewParamsOneOfByJSONSchema(js),
			},
			{
				Name:        "tool2",
				Desc:        "tool2",
				ParamsOneOf: schema.NewParamsOneOfByJSONSchema(js),
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		handler := callbacks.NewHandlerBuilder().OnEndFn(func(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
			nOutput := model.ConvCallbackOutput(output)
			if nOutput.TokenUsage.PromptTokens != 1 {
				t.Fatal("invalid token usage")
			}
			if nOutput.TokenUsage.CompletionTokens != 2 {
				t.Fatal("invalid token usage")
			}
			if nOutput.TokenUsage.TotalTokens != 3 {
				t.Fatal("invalid token usage")
			}
			return ctx
		})
		ctx = callbacks.InitCallbacks(ctx, &callbacks.RunInfo{}, handler.Build())

		result, err := m.Generate(ctx, []*schema.Message{
			schema.SystemMessage("system"),
			{
				Role: schema.User,
				MultiContent: []schema.ChatMessagePart{
					{
						Type: schema.ChatMessagePartTypeImageURL,
						ImageURL: &schema.ChatMessageImageURL{
							URL:    "http://a.b.c",
							Detail: "detail",
						},
					},
					{
						Type: schema.ChatMessagePartTypeText,
						Text: "text",
					},
				},
			},
			schema.ToolMessage("tool", "tool call id"),
		})
		if err != nil {
			t.Fatal(err)
		}
		// 手动处理result中的openaiResultID类型
		result.Extra["openai-request-id"] = protocol.GetRequestID(result)
		if !reflect.DeepEqual(result, expectedMessages) {
			resultData, _ := json.Marshal(result)
			expectMsgData, _ := json.Marshal(expectedMessages)
			t.Fatalf("result is unexpected, given=%v, expected=%v", string(resultData), string(expectMsgData))
		}
	})
	t.Run("stream all param", func(t *testing.T) {
		defer mockey.Mock((*openai.Client).CreateChatCompletionStream).To(func(ctx context.Context,
			request openai.ChatCompletionRequest, opts ...openai.ChatCompletionRequestOption) (response *openai.ChatCompletionStream, err error) {
			expectedRequestBody := expectedRequestBody
			expectedRequestBody.Stream = true
			expectedRequestBody.StreamOptions = &openai.StreamOptions{IncludeUsage: true}
			return nil, fmt.Errorf("expected error")
		}).Build().UnPatch()
		ctx := context.Background()
		m, err := NewChatModel(ctx, config)
		if err != nil {
			t.Fatal(err)
		}
		err = m.BindForcedTools([]*schema.ToolInfo{
			{
				Name:        "tool1",
				Desc:        "tool1",
				ParamsOneOf: schema.NewParamsOneOfByJSONSchema(js),
			},
			{
				Name:        "tool2",
				Desc:        "tool2",
				ParamsOneOf: schema.NewParamsOneOfByJSONSchema(js),
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		_, err = m.Stream(ctx, []*schema.Message{
			schema.SystemMessage("system"),
			{
				Role: schema.User,
				MultiContent: []schema.ChatMessagePart{
					{
						Type: schema.ChatMessagePartTypeImageURL,
						ImageURL: &schema.ChatMessageImageURL{
							URL:    "http://a.b.c",
							Detail: "detail",
						},
					},
					{
						Type: schema.ChatMessagePartTypeText,
						Text: "text",
					},
				},
			},
			schema.ToolMessage("tool", "tool call id"),
		})
		if strings.Contains(err.Error(), "request is unexpected") {
			t.Fatal(err)
		}
	})
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestChatModelCustomPath(t *testing.T) {
	const completionJSON = `{"id":"resp-1","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`

	tests := []struct {
		name          string
		baseURLSuffix string
		byAzure       bool
		apiVersion    string
		path          string
		expectedPath  string
		expectedQuery url.Values
	}{
		{
			name:          "default path when unset",
			baseURLSuffix: "/v1",
			path:          "",
			expectedPath:  "/v1/chat/completions",
		},
		{
			name:          "custom path with leading slash",
			baseURLSuffix: "/v1",
			path:          "/custom/completions",
			expectedPath:  "/v1/custom/completions",
		},
		{
			name:          "custom path without leading slash",
			baseURLSuffix: "",
			path:          "gateway/v2/invoke",
			expectedPath:  "/gateway/v2/invoke",
		},
		{
			name:          "custom path with trailing slash in baseURL",
			baseURLSuffix: "/proxy/",
			path:          "/llm/chat",
			expectedPath:  "/proxy/llm/chat",
		},
		{
			name:          "custom path with query parameters",
			baseURLSuffix: "/v1",
			path:          "/custom/chat?app=demo&mode=fast",
			expectedPath:  "/v1/custom/chat",
			expectedQuery: url.Values{"app": {"demo"}, "mode": {"fast"}},
		},
		{
			name:          "query only keeps default path",
			baseURLSuffix: "/v1",
			path:          "?app=demo",
			expectedPath:  "/v1/chat/completions",
			expectedQuery: url.Values{"app": {"demo"}},
		},
		{
			name:          "azure mode with custom path preserves api-version and merges query",
			baseURLSuffix: "",
			byAzure:       true,
			apiVersion:    "2024-06-01",
			path:          "/custom/completions?extra=1",
			expectedPath:  "/openai/deployments/gpt-4/custom/completions",
			expectedQuery: url.Values{"api-version": {"2024-06-01"}, "extra": {"1"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gotPath string
			var gotQuery url.Values
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				gotQuery = r.URL.Query()
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(completionJSON))
			}))
			defer srv.Close()

			cm, err := NewChatModel(context.Background(), &ChatModelConfig{
				APIKey:     "test-key",
				BaseURL:    srv.URL + tc.baseURLSuffix,
				ByAzure:    tc.byAzure,
				APIVersion: tc.apiVersion,
				Path:       tc.path,
				Model:      "gpt-4",
			})
			if err != nil {
				t.Fatalf("NewChatModel() error = %v", err)
			}

			msg, err := cm.Generate(context.Background(), []*schema.Message{schema.UserMessage("hello")})
			if err != nil {
				t.Fatalf("Generate() error = %v", err)
			}
			if msg.Content != "ok" {
				t.Fatalf("unexpected content: %q", msg.Content)
			}
			if gotPath != tc.expectedPath {
				t.Fatalf("request path = %q, want %q", gotPath, tc.expectedPath)
			}
			if len(tc.expectedQuery) > 0 && !reflect.DeepEqual(gotQuery, tc.expectedQuery) {
				t.Fatalf("request query = %v, want %v", gotQuery, tc.expectedQuery)
			}
		})
	}
}

func TestChatModelCustomPathStream(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		_, _ = fmt.Fprint(w, "data: {\"id\":\"s-1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"hello\"}}]}\n\n")
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
		if flusher != nil {
			flusher.Flush()
		}
	}))
	defer srv.Close()

	cm, err := NewChatModel(context.Background(), &ChatModelConfig{
		APIKey:  "test-key",
		BaseURL: srv.URL + "/api",
		Path:    "/v2/stream-chat",
		Model:   "gpt-4",
	})
	if err != nil {
		t.Fatalf("NewChatModel() error = %v", err)
	}

	sr, err := cm.Stream(context.Background(), []*schema.Message{schema.UserMessage("hi")})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}
	defer sr.Close()

	var content strings.Builder
	for {
		chunk, recvErr := sr.Recv()
		if recvErr == io.EOF {
			break
		}
		if recvErr != nil {
			t.Fatalf("Stream Recv() error = %v", recvErr)
		}
		content.WriteString(chunk.Content)
	}

	if gotPath != "/api/v2/stream-chat" {
		t.Fatalf("stream request path = %q, want %q", gotPath, "/api/v2/stream-chat")
	}
	if content.String() != "hello" {
		t.Fatalf("stream content = %q, want %q", content.String(), "hello")
	}
}

func TestChatModelCustomPathPreservesHTTPClientAndConcurrency(t *testing.T) {
	const completionJSON = `{"id":"resp-1","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/gw/custom" {
			http.Error(w, "unexpected path: "+r.URL.Path, http.StatusNotFound)
			return
		}
		if r.Header.Get("X-Custom-Transport") != "true" {
			http.Error(w, "missing custom transport header", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(completionJSON))
	}))
	defer srv.Close()

	baseRT := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		cloned := req.Clone(req.Context())
		cloned.Header.Set("X-Custom-Transport", "true")
		return http.DefaultTransport.RoundTrip(cloned)
	})
	sharedClient := &http.Client{Transport: baseRT}

	cm, err := NewChatModel(context.Background(), &ChatModelConfig{
		APIKey:     "test-key",
		BaseURL:    srv.URL + "/gw",
		Path:       "/custom",
		Model:      "gpt-4",
		HTTPClient: sharedClient,
	})
	if err != nil {
		t.Fatalf("NewChatModel() error = %v", err)
	}

	if reflect.ValueOf(sharedClient.Transport).Pointer() != reflect.ValueOf(baseRT).Pointer() {
		t.Fatalf("caller HTTPClient.Transport was mutated")
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, genErr := cm.Generate(context.Background(), []*schema.Message{schema.UserMessage("ping")})
			if genErr != nil {
				errCh <- genErr
			}
		}()
	}
	wg.Wait()
	close(errCh)

	for genErr := range errCh {
		t.Fatalf("concurrent Generate() error = %v", genErr)
	}
}

func TestChatModelCustomPathInvalid(t *testing.T) {
	invalidPaths := []string{
		"https://example.com/v1/chat",
		"//example.com/v1/chat",
		"/bad/%zz",
	}

	for _, p := range invalidPaths {
		t.Run(p, func(t *testing.T) {
			_, err := NewChatModel(context.Background(), &ChatModelConfig{
				APIKey: "test-key",
				Model:  "gpt-4",
				Path:   p,
			})
			if err == nil {
				t.Fatalf("NewChatModel() expected error for invalid path %q, got nil", p)
			}
		})
	}
}

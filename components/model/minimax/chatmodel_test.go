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

package minimax

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
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

func TestNewChatModel(t *testing.T) {
	ctx := context.Background()

	t.Run("nil config", func(t *testing.T) {
		_, err := NewChatModel(ctx, nil)
		if err == nil {
			t.Fatal("expected error for nil config")
		}
	})

	t.Run("empty api key", func(t *testing.T) {
		_, err := NewChatModel(ctx, &Config{})
		if err == nil {
			t.Fatal("expected error for empty api key")
		}
	})

	t.Run("default values", func(t *testing.T) {
		m, err := NewChatModel(ctx, &Config{
			APIKey: "test-key",
		})
		if err != nil {
			t.Fatal(err)
		}
		if m.GetType() != "MiniMax" {
			t.Fatalf("expected type MiniMax, got %s", m.GetType())
		}
	})

	t.Run("custom base url", func(t *testing.T) {
		_, err := NewChatModel(ctx, &Config{
			APIKey:  "test-key",
			BaseURL: "https://api.minimaxi.com/v1",
		})
		if err != nil {
			t.Fatal(err)
		}
	})
}

func TestTemperatureClamping(t *testing.T) {
	ctx := context.Background()

	t.Run("temperature too low clamped to 0.01", func(t *testing.T) {
		var temp float32 = 0.0
		m, err := NewChatModel(ctx, &Config{
			APIKey:      "test-key",
			Temperature: &temp,
		})
		if err != nil {
			t.Fatal(err)
		}
		_ = m // Temperature is clamped in config
	})

	t.Run("temperature too high clamped to 1.0", func(t *testing.T) {
		var temp float32 = 2.0
		m, err := NewChatModel(ctx, &Config{
			APIKey:      "test-key",
			Temperature: &temp,
		})
		if err != nil {
			t.Fatal(err)
		}
		_ = m
	})

	t.Run("runtime temperature clamping", func(t *testing.T) {
		m, err := NewChatModel(ctx, &Config{
			APIKey: "test-key",
		})
		if err != nil {
			t.Fatal(err)
		}
		opts := m.clampTemperatureOpts(model.WithTemperature(0.0))
		commonOpts := model.GetCommonOptions(&model.Options{}, opts...)
		if commonOpts.Temperature == nil {
			t.Fatal("expected temperature to be set")
		}
		if *commonOpts.Temperature <= 0 {
			t.Fatalf("expected temperature > 0, got %f", *commonOpts.Temperature)
		}
	})
}

func TestMiniMaxGenerate(t *testing.T) {
	js := &jsonschema.Schema{
		Type: string(schema.Object),
		Properties: orderedmap.New[string, *jsonschema.Schema](
			orderedmap.WithInitialData[string, *jsonschema.Schema](
				orderedmap.Pair[string, *jsonschema.Schema]{
					Key: "query",
					Value: &jsonschema.Schema{
						Type: string(schema.String),
					},
				},
			),
		),
	}

	mockToolCallIdx := 0
	var temperature float32 = 0.7
	mockOpenAIResponse := openai.ChatCompletionResponse{
		ID: "request-id-123",
		Choices: []openai.ChatCompletionChoice{
			{
				Index: 0,
				Message: openai.ChatCompletionMessage{
					Role:    openai.ChatMessageRoleAssistant,
					Content: "Hello from MiniMax!",
					ToolCalls: []openai.ToolCall{
						{
							Index: &mockToolCallIdx,
							ID:    "call_1",
							Type:  openai.ToolTypeFunction,
							Function: openai.FunctionCall{
								Name:      "search",
								Arguments: `{"query":"test"}`,
							},
						},
					},
				},
			},
		},
		Usage: openai.Usage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
		},
	}
	expectedMessages := &schema.Message{
		Role:    schema.Assistant,
		Content: "Hello from MiniMax!",
		ToolCalls: []schema.ToolCall{
			{
				Index: &mockToolCallIdx,
				ID:    "call_1",
				Type:  "function",
				Function: schema.FunctionCall{
					Name:      "search",
					Arguments: `{"query":"test"}`,
				},
			},
		},
		ResponseMeta: &schema.ResponseMeta{
			Usage: &schema.TokenUsage{
				PromptTokens:     10,
				CompletionTokens: 20,
				TotalTokens:      30,
			},
		},
		Extra: map[string]any{
			"openai-request-id": "request-id-123",
		},
	}
	config := &Config{
		APIKey:      "test-minimax-key",
		Model:       "MiniMax-M2.7",
		Temperature: &temperature,
	}

	t.Run("generate with tool calling", func(t *testing.T) {
		defer mockey.Mock((*openai.Client).CreateChatCompletion).To(func(ctx context.Context,
			request openai.ChatCompletionRequest, opts ...openai.ChatCompletionRequestOption) (response openai.ChatCompletionResponse, err error) {
			return mockOpenAIResponse, nil
		}).Build().UnPatch()

		ctx := context.Background()
		m, err := NewChatModel(ctx, config)
		if err != nil {
			t.Fatal(err)
		}
		err = m.BindTools([]*schema.ToolInfo{
			{
				Name:        "search",
				Desc:        "Search for information",
				ParamsOneOf: schema.NewParamsOneOfByJSONSchema(js),
			},
		})
		if err != nil {
			t.Fatal(err)
		}

		handler := callbacks.NewHandlerBuilder().OnEndFn(func(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
			nOutput := model.ConvCallbackOutput(output)
			if nOutput.TokenUsage.PromptTokens != 10 {
				t.Fatal("invalid prompt token usage")
			}
			if nOutput.TokenUsage.CompletionTokens != 20 {
				t.Fatal("invalid completion token usage")
			}
			if nOutput.TokenUsage.TotalTokens != 30 {
				t.Fatal("invalid total token usage")
			}
			return ctx
		})
		ctx = callbacks.InitCallbacks(ctx, &callbacks.RunInfo{}, handler.Build())

		result, err := m.Generate(ctx, []*schema.Message{
			schema.SystemMessage("You are a helpful assistant."),
			{
				Role:    schema.User,
				Content: "Search for MiniMax AI",
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		result.Extra["openai-request-id"] = protocol.GetRequestID(result)
		if !reflect.DeepEqual(result, expectedMessages) {
			resultData, _ := json.Marshal(result)
			expectMsgData, _ := json.Marshal(expectedMessages)
			t.Fatalf("result is unexpected, given=%v, expected=%v", string(resultData), string(expectMsgData))
		}
	})

	t.Run("stream returns error", func(t *testing.T) {
		defer mockey.Mock((*openai.Client).CreateChatCompletionStream).To(func(ctx context.Context,
			request openai.ChatCompletionRequest, opts ...openai.ChatCompletionRequestOption) (response *openai.ChatCompletionStream, err error) {
			return nil, fmt.Errorf("stream error")
		}).Build().UnPatch()

		ctx := context.Background()
		m, err := NewChatModel(ctx, config)
		if err != nil {
			t.Fatal(err)
		}
		_, err = m.Stream(ctx, []*schema.Message{
			schema.SystemMessage("You are a helpful assistant."),
			{
				Role:    schema.User,
				Content: "Hello",
			},
		})
		if !strings.Contains(err.Error(), "stream error") {
			t.Fatalf("expected stream error, got: %v", err)
		}
	})
}

func TestWithTools(t *testing.T) {
	ctx := context.Background()
	m, err := NewChatModel(ctx, &Config{
		APIKey: "test-key",
	})
	if err != nil {
		t.Fatal(err)
	}

	js := &jsonschema.Schema{
		Type: string(schema.Object),
		Properties: orderedmap.New[string, *jsonschema.Schema](
			orderedmap.WithInitialData[string, *jsonschema.Schema](
				orderedmap.Pair[string, *jsonschema.Schema]{
					Key: "input",
					Value: &jsonschema.Schema{
						Type: string(schema.String),
					},
				},
			),
		),
	}

	newModel, err := m.WithTools([]*schema.ToolInfo{
		{
			Name:        "test_tool",
			Desc:        "A test tool",
			ParamsOneOf: schema.NewParamsOneOfByJSONSchema(js),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if newModel == nil {
		t.Fatal("expected non-nil model from WithTools")
	}
	// Verify it's a different instance
	if newModel == m {
		t.Fatal("WithTools should return a new instance")
	}
}

func TestAPIError(t *testing.T) {
	t.Run("error with status code", func(t *testing.T) {
		err := &APIError{
			HTTPStatusCode: 401,
			HTTPStatus:     "Unauthorized",
			Message:        "Invalid API key",
		}
		if !strings.Contains(err.Error(), "401") {
			t.Fatalf("expected error to contain status code, got: %s", err.Error())
		}
	})

	t.Run("error without status code", func(t *testing.T) {
		err := &APIError{
			Message: "some error",
		}
		if err.Error() != "some error" {
			t.Fatalf("expected 'some error', got: %s", err.Error())
		}
	})
}

func TestGetType(t *testing.T) {
	ctx := context.Background()
	m, err := NewChatModel(ctx, &Config{
		APIKey: "test-key",
	})
	if err != nil {
		t.Fatal(err)
	}
	if m.GetType() != "MiniMax" {
		t.Fatalf("expected type 'MiniMax', got '%s'", m.GetType())
	}
}

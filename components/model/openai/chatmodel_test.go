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

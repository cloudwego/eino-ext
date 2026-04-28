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

package deepseek

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/bytedance/mockey"
	"github.com/cohesion-org/deepseek-go"
	"github.com/eino-contrib/jsonschema"
	"github.com/stretchr/testify/assert"
	orderedmap "github.com/wk8/go-ordered-map/v2"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func TestChatModelGenerate(t *testing.T) {
	defer mockey.Mock((*deepseek.Client).CreateChatCompletion).To(func(ctx context.Context, request *deepseek.ChatCompletionRequest) (*deepseek.ChatCompletionResponse, error) {
		return &deepseek.ChatCompletionResponse{
			Choices: []deepseek.Choice{
				{
					Index: 0,
					Message: deepseek.Message{
						Role:             "assistant",
						Content:          "hello world",
						ReasoningContent: "reasoning content",
						ToolCalls: []deepseek.ToolCall{
							{
								Index: 1,
								ID:    "id",
								Type:  "type",
								Function: deepseek.ToolCallFunction{
									Name:      "name",
									Arguments: "arguments",
								},
							},
						},
					},
					Logprobs: nil,
				},
			},
			Usage: deepseek.Usage{
				PromptTokens:     1,
				CompletionTokens: 2,
				TotalTokens:      3,
			},
		}, nil
	}).Build().UnPatch()

	ctx := context.Background()
	cm, err := NewChatModel(ctx, &ChatModelConfig{
		APIKey:  "my-api-key",
		Timeout: time.Second,
		Model:   "deepseek-chat",
	})
	assert.Nil(t, err)
	err = cm.BindForcedTools([]*schema.ToolInfo{
		{
			Name: "deepseek-tool",
			ParamsOneOf: schema.NewParamsOneOfByJSONSchema(
				&jsonschema.Schema{
					Type: string(schema.Object),
					Properties: orderedmap.New[string, *jsonschema.Schema](orderedmap.WithInitialData[string, *jsonschema.Schema](
						orderedmap.Pair[string, *jsonschema.Schema]{
							Key:   "field1",
							Value: &jsonschema.Schema{Type: string(schema.String)},
						},
					)),
				},
			),
		},
	})
	assert.Nil(t, err)
	result, err := cm.Generate(ctx, []*schema.Message{schema.SystemMessage("system"), schema.UserMessage("hello"), schema.AssistantMessage("assistant", nil), schema.UserMessage("hello")})
	assert.Nil(t, err)
	index := 1
	expected := &schema.Message{
		Role:             schema.Assistant,
		Content:          "hello world",
		ReasoningContent: "reasoning content",
		ToolCalls: []schema.ToolCall{
			{
				Index: &index,
				ID:    "id",
				Type:  "type",
				Function: schema.FunctionCall{
					Name:      "name",
					Arguments: "arguments",
				},
			},
		},
		ResponseMeta: &schema.ResponseMeta{Usage: &schema.TokenUsage{
			PromptTokens:     1,
			CompletionTokens: 2,
			TotalTokens:      3,
		}},
	}
	SetReasoningContent(expected, "reasoning content")
	assert.Equal(t, expected, result)
}

func TestChatModelStream(t *testing.T) {
	responses := []*deepseek.StreamChatCompletionResponse{
		{
			Choices: []deepseek.StreamChoices{
				{
					Index: 0,
					Delta: deepseek.StreamDelta{
						Role:    "assistant",
						Content: "Hello",
					},
				},
			},
		},
		{
			Choices: []deepseek.StreamChoices{
				{
					Index: 0,
					Delta: deepseek.StreamDelta{
						Role:    "assistant",
						Content: " World",
						ToolCalls: []deepseek.ToolCall{
							{
								Index: 1,
								ID:    "id",
								Type:  "type",
								Function: deepseek.ToolCallFunction{
									Name:      "name",
									Arguments: "arguments",
								},
							},
						},
					},
				},
			},
		},
		{
			Usage: &deepseek.StreamUsage{
				PromptTokens:     1,
				CompletionTokens: 2,
				TotalTokens:      3,
			},
		},
	}

	defer mockey.Mock((*deepseek.Client).CreateChatCompletionStream).To(func(ctx context.Context, request *deepseek.StreamChatCompletionRequest) (deepseek.ChatCompletionStream, error) {
		return &mockStream{
			responses: responses,
			idx:       0,
		}, nil
	}).Build().UnPatch()

	ctx := context.Background()
	cm, err := NewChatModel(ctx, &ChatModelConfig{
		APIKey:             "my-api-key",
		Timeout:            time.Second,
		Model:              "deepseek-chat",
		ResponseFormatType: ResponseFormatTypeJSONObject,
	})
	assert.Nil(t, err)
	err = cm.BindTools([]*schema.ToolInfo{
		{
			Name: "deepseek-tool",
			ParamsOneOf: schema.NewParamsOneOfByJSONSchema(
				&jsonschema.Schema{
					Type: string(schema.Object),
					Properties: orderedmap.New[string, *jsonschema.Schema](
						orderedmap.WithInitialData[string, *jsonschema.Schema](
							orderedmap.Pair[string, *jsonschema.Schema]{
								Key:   "field1",
								Value: &jsonschema.Schema{Type: string(schema.String)},
							},
						),
					),
				},
			),
		},
	})
	assert.Nil(t, err)
	result, err := cm.Stream(ctx, []*schema.Message{schema.UserMessage("hello")})
	assert.Nil(t, err)

	var msgs []*schema.Message
	for {
		chunk, err := result.Recv()
		if err == io.EOF {
			break
		}
		assert.Nil(t, err)
		msgs = append(msgs, chunk)
	}

	msg, err := schema.ConcatMessages(msgs)
	assert.Nil(t, err)
	index := 1
	assert.Equal(t, &schema.Message{
		Role:    schema.Assistant,
		Content: "Hello World",
		ToolCalls: []schema.ToolCall{
			{
				Index: &index,
				ID:    "id",
				Type:  "type",
				Function: schema.FunctionCall{
					Name:      "name",
					Arguments: "arguments",
				},
			},
		},
		ResponseMeta: &schema.ResponseMeta{Usage: &schema.TokenUsage{
			PromptTokens:     1,
			CompletionTokens: 2,
			TotalTokens:      3,
		},
			LogProbs: nil,
		},
	}, msg)
}

type mockStream struct {
	responses []*deepseek.StreamChatCompletionResponse
	idx       int
}

func (m *mockStream) Recv() (*deepseek.StreamChatCompletionResponse, error) {
	if m.idx >= len(m.responses) {
		return nil, io.EOF
	}
	res := m.responses[m.idx]
	m.idx++
	return res, nil
}

func (m *mockStream) Close() error {
	return nil
}

func TestPanicErr(t *testing.T) {
	err := newPanicErr("info", []byte("stack"))
	assert.Equal(t, "panic error: info, \nstack: stack", err.Error())
}

func TestWithTools(t *testing.T) {
	cm := &ChatModel{conf: &ChatModelConfig{Model: "test model"}}
	ncm, err := cm.WithTools([]*schema.ToolInfo{{Name: "test tool name"}})
	assert.Nil(t, err)
	assert.Equal(t, "test model", ncm.(*ChatModel).conf.Model)
	assert.Equal(t, "test tool name", ncm.(*ChatModel).rawTools[0].Name)
}

func TestLogProbs(t *testing.T) {
	assert.Equal(t, &schema.LogProbs{Content: []schema.LogProb{
		{
			Token:   "1",
			LogProb: 1,
			Bytes:   []int64{'a'},
			TopLogProbs: []schema.TopLogProb{
				{
					Token:   "2",
					LogProb: 2,
					Bytes:   []int64{'b'},
				},
			},
		},
	}}, toLogProbs(&deepseek.Logprobs{Content: []deepseek.ContentToken{
		{
			Token:   "1",
			Logprob: 1,
			Bytes:   []int{'a'},
			TopLogprobs: []deepseek.TopLogprobToken{
				{
					Token:   "2",
					Logprob: 2,
					Bytes:   []int{'b'},
				},
			},
		},
	}}))
}

func TestPopulateToolChoice(t *testing.T) {
	toolChoiceForbidden := schema.ToolChoiceForbidden
	toolChoiceAllowed := schema.ToolChoiceAllowed
	toolChoiceForced := schema.ToolChoiceForced
	unsupportedToolChoice := schema.ToolChoice("unsupported")

	tool1 := deepseek.Tool{Type: "function", Function: deepseek.Function{Name: "tool1"}}
	tool2 := deepseek.Tool{Type: "function", Function: deepseek.Function{Name: "tool2"}}

	testCases := []struct {
		name        string
		options     *model.Options
		req         deepseek.ChatCompletionRequest
		expectedReq deepseek.ChatCompletionRequest
		expectErr   bool
		errContains string
	}{
		{
			name:        "nil tool choice",
			options:     &model.Options{},
			req:         deepseek.ChatCompletionRequest{},
			expectedReq: deepseek.ChatCompletionRequest{},
			expectErr:   false,
		},
		{
			name:        "tool choice forbidden",
			options:     &model.Options{ToolChoice: &toolChoiceForbidden},
			req:         deepseek.ChatCompletionRequest{},
			expectedReq: deepseek.ChatCompletionRequest{ToolChoice: "none"},
			expectErr:   false,
		},
		{
			name:        "tool choice allowed",
			options:     &model.Options{ToolChoice: &toolChoiceAllowed},
			req:         deepseek.ChatCompletionRequest{},
			expectedReq: deepseek.ChatCompletionRequest{ToolChoice: "auto"},
			expectErr:   false,
		},
		{
			name:        "tool choice forced with no tools",
			options:     &model.Options{ToolChoice: &toolChoiceForced},
			req:         deepseek.ChatCompletionRequest{},
			expectErr:   true,
			errContains: "tool choice is forced but tool is not provided",
		},
		{
			name:        "tool choice forced with multiple allowed tool names",
			options:     &model.Options{ToolChoice: &toolChoiceForced, AllowedToolNames: []string{"tool1", "tool2"}},
			req:         deepseek.ChatCompletionRequest{Tools: []deepseek.Tool{tool1, tool2}},
			expectErr:   true,
			errContains: "only one allowed tool name can be configured",
		},
		{
			name:        "tool choice forced with allowed tool name not in tools list",
			options:     &model.Options{ToolChoice: &toolChoiceForced, AllowedToolNames: []string{"tool3"}},
			req:         deepseek.ChatCompletionRequest{Tools: []deepseek.Tool{tool1, tool2}},
			expectErr:   true,
			errContains: "allowed tool name 'tool3' not found in tools list",
		},
		{
			name:    "tool choice forced with one allowed tool name",
			options: &model.Options{ToolChoice: &toolChoiceForced, AllowedToolNames: []string{"tool1"}},
			req:     deepseek.ChatCompletionRequest{Tools: []deepseek.Tool{tool1, tool2}},
			expectedReq: deepseek.ChatCompletionRequest{
				Tools: []deepseek.Tool{tool1, tool2},
				ToolChoice: deepseek.ToolChoice{
					Type:     "function",
					Function: deepseek.ToolChoiceFunction{Name: "tool1"},
				},
			},
			expectErr: false,
		},
		{
			name:    "tool choice forced with one tool",
			options: &model.Options{ToolChoice: &toolChoiceForced},
			req:     deepseek.ChatCompletionRequest{Tools: []deepseek.Tool{tool1}},
			expectedReq: deepseek.ChatCompletionRequest{
				Tools: []deepseek.Tool{tool1},
				ToolChoice: deepseek.ToolChoice{
					Type:     "function",
					Function: deepseek.ToolChoiceFunction{Name: "tool1"},
				},
			},
			expectErr: false,
		},
		{
			name:        "tool choice forced with multiple tools and no allowed tool names",
			options:     &model.Options{ToolChoice: &toolChoiceForced},
			req:         deepseek.ChatCompletionRequest{Tools: []deepseek.Tool{tool1, tool2}},
			expectedReq: deepseek.ChatCompletionRequest{Tools: []deepseek.Tool{tool1, tool2}, ToolChoice: "required"},
			expectErr:   false,
		},
		{
			name:        "unsupported tool choice",
			options:     &model.Options{ToolChoice: &unsupportedToolChoice},
			req:         deepseek.ChatCompletionRequest{},
			expectErr:   true,
			errContains: "tool choice=unsupported not support",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := populateToolChoice(&tc.req, tc.options.ToolChoice, tc.options.AllowedToolNames)

			if tc.expectErr {
				assert.Error(t, err)
				if tc.errContains != "" {
					assert.Contains(t, err.Error(), tc.errContains)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedReq, tc.req)
			}
		})
	}
}

func TestThinkingConfig(t *testing.T) {
	var capturedReq *deepseek.ChatCompletionRequest
	defer mockey.Mock((*deepseek.Client).CreateChatCompletion).To(func(ctx context.Context, request *deepseek.ChatCompletionRequest) (*deepseek.ChatCompletionResponse, error) {
		capturedReq = request
		return &deepseek.ChatCompletionResponse{
			Choices: []deepseek.Choice{
				{
					Index:   0,
					Message: deepseek.Message{Role: "assistant", Content: "hello"},
				},
			},
		}, nil
	}).Build().UnPatch()

	ctx := context.Background()

	t.Run("with thinking config", func(t *testing.T) {
		cm, err := NewChatModel(ctx, &ChatModelConfig{
			APIKey: "test-key",
			Model:  "deepseek-reasoner",
			ThinkingConfig: &ThinkingConfig{
				Type: "enabled",
			},
		})
		assert.Nil(t, err)

		_, err = cm.Generate(ctx, []*schema.Message{schema.UserMessage("hello")})
		assert.Nil(t, err)
		assert.NotNil(t, capturedReq.Thinking)
		assert.Equal(t, "enabled", capturedReq.Thinking.Type)
	})

	t.Run("without thinking config", func(t *testing.T) {
		cm, err := NewChatModel(ctx, &ChatModelConfig{
			APIKey: "test-key",
			Model:  "deepseek-chat",
		})
		assert.Nil(t, err)

		_, err = cm.Generate(ctx, []*schema.Message{schema.UserMessage("hello")})
		assert.Nil(t, err)
		assert.Nil(t, capturedReq.Thinking)
	})
}

func TestThinkingConfigStream(t *testing.T) {
	var capturedReq *deepseek.StreamChatCompletionRequest
	defer mockey.Mock((*deepseek.Client).CreateChatCompletionStream).To(func(ctx context.Context, request *deepseek.StreamChatCompletionRequest) (deepseek.ChatCompletionStream, error) {
		capturedReq = request
		return &mockStream{}, nil
	}).Build().UnPatch()

	ctx := context.Background()
	cm, err := NewChatModel(ctx, &ChatModelConfig{
		APIKey: "test-key",
		Model:  "deepseek-reasoner",
		ThinkingConfig: &ThinkingConfig{
			Type: "enabled",
		},
	})
	assert.Nil(t, err)

	stream, err := cm.Stream(ctx, []*schema.Message{schema.UserMessage("hello")})
	assert.Nil(t, err)
	if assert.NotNil(t, stream) &&
		assert.NotNil(t, capturedReq) &&
		assert.NotNil(t, capturedReq.Thinking) {
		assert.Equal(t, "enabled", capturedReq.Thinking.Type)
	}
}

func TestNewChatModel(t *testing.T) {
	ctx := context.Background()

	t.Run("empty model returns error", func(t *testing.T) {
		_, err := NewChatModel(ctx, &ChatModelConfig{APIKey: "key"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "model is required")
	})

	t.Run("base url without trailing slash", func(t *testing.T) {
		cm, err := NewChatModel(ctx, &ChatModelConfig{
			APIKey:  "key",
			Model:   "deepseek-chat",
			BaseURL: "https://custom.api.com",
		})
		assert.Nil(t, err)
		assert.NotNil(t, cm)
	})

	t.Run("with all options", func(t *testing.T) {
		cm, err := NewChatModel(ctx, &ChatModelConfig{
			APIKey:  "key",
			Model:   "deepseek-chat",
			Timeout: 10 * time.Second,
			BaseURL: "https://custom.api.com/",
			Path:    "/v1/chat",
		})
		assert.Nil(t, err)
		assert.NotNil(t, cm)
	})
}

func TestToDeepSeekMessage(t *testing.T) {
	t.Run("role mapping", func(t *testing.T) {
		cases := []struct {
			role     schema.RoleType
			expected string
		}{
			{schema.System, "system"},
			{schema.User, "user"},
			{schema.Assistant, "assistant"},
			{schema.Tool, "tool"},
		}
		for _, tc := range cases {
			msg := &schema.Message{Role: tc.role, Content: "hi"}
			result, err := toDeepSeekMessage(msg)
			assert.Nil(t, err)
			assert.Equal(t, tc.expected, result.Role)
		}
	})

	t.Run("unknown role returns error", func(t *testing.T) {
		msg := &schema.Message{Role: schema.RoleType("unknown"), Content: "hi"}
		_, err := toDeepSeekMessage(msg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown role type")
	})

	t.Run("multi content not supported", func(t *testing.T) {
		msg := &schema.Message{
			Role: schema.User,
			MultiContent: []schema.ChatMessagePart{
				{Type: schema.ChatMessagePartTypeText, Text: "hello"},
			},
		}
		_, err := toDeepSeekMessage(msg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "multi content is not supported")
	})

	t.Run("user input multi content text only", func(t *testing.T) {
		msg := &schema.Message{
			Role: schema.User,
			UserInputMultiContent: []schema.MessageInputPart{
				{Type: schema.ChatMessagePartTypeText, Text: "hello"},
				{Type: schema.ChatMessagePartTypeText, Text: "world"},
			},
		}
		result, err := toDeepSeekMessage(msg)
		assert.Nil(t, err)
		assert.Equal(t, "hello\n\nworld", result.Content)
	})

	t.Run("user input multi content unsupported type", func(t *testing.T) {
		msg := &schema.Message{
			Role: schema.User,
			UserInputMultiContent: []schema.MessageInputPart{
				{Type: schema.ChatMessagePartTypeImageURL, Text: "img"},
			},
		}
		_, err := toDeepSeekMessage(msg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "user input multi content")
	})

	t.Run("assistant gen multi content text only", func(t *testing.T) {
		msg := &schema.Message{
			Role: schema.Assistant,
			AssistantGenMultiContent: []schema.MessageOutputPart{
				{Type: schema.ChatMessagePartTypeText, Text: "foo"},
				{Type: schema.ChatMessagePartTypeText, Text: "bar"},
			},
		}
		result, err := toDeepSeekMessage(msg)
		assert.Nil(t, err)
		assert.Equal(t, "foo\n\nbar", result.Content)
	})

	t.Run("assistant gen multi content unsupported type", func(t *testing.T) {
		msg := &schema.Message{
			Role: schema.Assistant,
			AssistantGenMultiContent: []schema.MessageOutputPart{
				{Type: schema.ChatMessagePartTypeImageURL, Text: "img"},
			},
		}
		_, err := toDeepSeekMessage(msg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "assistant gen multi content")
	})

	t.Run("prefix on non-assistant returns error", func(t *testing.T) {
		msg := &schema.Message{Role: schema.User, Content: "hi"}
		SetPrefix(msg)
		_, err := toDeepSeekMessage(msg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "prefix only supported for assistant message")
	})

	t.Run("prefix on assistant is ok", func(t *testing.T) {
		msg := &schema.Message{Role: schema.Assistant, Content: "hi"}
		SetPrefix(msg)
		result, err := toDeepSeekMessage(msg)
		assert.Nil(t, err)
		assert.True(t, result.Prefix)
	})

	t.Run("reasoning content from field", func(t *testing.T) {
		msg := &schema.Message{Role: schema.Assistant, Content: "hi", ReasoningContent: "think"}
		result, err := toDeepSeekMessage(msg)
		assert.Nil(t, err)
		assert.Equal(t, "think", result.ReasoningContent)
	})

	t.Run("reasoning content fallback from extra", func(t *testing.T) {
		msg := &schema.Message{Role: schema.Assistant, Content: "hi"}
		SetReasoningContent(msg, "from extra")
		result, err := toDeepSeekMessage(msg)
		assert.Nil(t, err)
		assert.Equal(t, "from extra", result.ReasoningContent)
	})

	t.Run("tool role with tool call id", func(t *testing.T) {
		msg := &schema.Message{Role: schema.Tool, Content: "result", ToolCallID: "call-123"}
		result, err := toDeepSeekMessage(msg)
		assert.Nil(t, err)
		assert.Equal(t, "call-123", result.ToolCallID)
	})

	t.Run("assistant with tool calls nil index", func(t *testing.T) {
		msg := &schema.Message{
			Role: schema.Assistant,
			ToolCalls: []schema.ToolCall{
				{ID: "id1", Type: "function", Function: schema.FunctionCall{Name: "fn", Arguments: "{}"}},
			},
		}
		result, err := toDeepSeekMessage(msg)
		assert.Nil(t, err)
		assert.Len(t, result.ToolCalls, 1)
		assert.Equal(t, 0, result.ToolCalls[0].Index)
		assert.Equal(t, "id1", result.ToolCalls[0].ID)
	})
}

func TestResolveStreamResponse(t *testing.T) {
	t.Run("normal choice index 0", func(t *testing.T) {
		resp := &deepseek.StreamChatCompletionResponse{
			Choices: []deepseek.StreamChoices{
				{Index: 0, Delta: deepseek.StreamDelta{Role: "assistant", Content: "hello"}},
			},
		}
		msg, found, err := resolveStreamResponse(resp)
		assert.Nil(t, err)
		assert.True(t, found)
		assert.Equal(t, "hello", msg.Content)
	})

	t.Run("skip non-zero index", func(t *testing.T) {
		resp := &deepseek.StreamChatCompletionResponse{
			Choices: []deepseek.StreamChoices{
				{Index: 1, Delta: deepseek.StreamDelta{Role: "assistant", Content: "skip"}},
			},
		}
		msg, found, err := resolveStreamResponse(resp)
		assert.Nil(t, err)
		assert.False(t, found)
		assert.Nil(t, msg)
	})

	t.Run("usage only response", func(t *testing.T) {
		resp := &deepseek.StreamChatCompletionResponse{
			Usage: &deepseek.StreamUsage{
				PromptTokens:     10,
				CompletionTokens: 20,
				TotalTokens:      30,
			},
		}
		msg, found, err := resolveStreamResponse(resp)
		assert.Nil(t, err)
		assert.True(t, found)
		assert.NotNil(t, msg.ResponseMeta)
		assert.Equal(t, 10, msg.ResponseMeta.Usage.PromptTokens)
	})

	t.Run("reasoning content in delta", func(t *testing.T) {
		resp := &deepseek.StreamChatCompletionResponse{
			Choices: []deepseek.StreamChoices{
				{Index: 0, Delta: deepseek.StreamDelta{Role: "assistant", ReasoningContent: "thinking..."}},
			},
		}
		msg, found, err := resolveStreamResponse(resp)
		assert.Nil(t, err)
		assert.True(t, found)
		rc, ok := GetReasoningContent(msg)
		assert.True(t, ok)
		assert.Equal(t, "thinking...", rc)
		assert.Equal(t, "thinking...", msg.ReasoningContent)
	})
}

func TestConcatTextParts(t *testing.T) {
	t.Run("text parts joined", func(t *testing.T) {
		parts := []schema.MessageInputPart{
			{Type: schema.ChatMessagePartTypeText, Text: "a"},
			{Type: schema.ChatMessagePartTypeText, Text: "b"},
		}
		result, err := concatTextParts(parts, func(p schema.MessageInputPart) (schema.ChatMessagePartType, string) {
			return p.Type, p.Text
		})
		assert.Nil(t, err)
		assert.Equal(t, "a\n\nb", result)
	})

	t.Run("unsupported type returns error", func(t *testing.T) {
		parts := []schema.MessageInputPart{
			{Type: schema.ChatMessagePartTypeImageURL, Text: "img"},
		}
		_, err := concatTextParts(parts, func(p schema.MessageInputPart) (schema.ChatMessagePartType, string) {
			return p.Type, p.Text
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "does not support")
	})
}

func TestExtractLogProbs(t *testing.T) {
	t.Run("nil returns nil", func(t *testing.T) {
		result, err := extractLogProbs(nil)
		assert.Nil(t, err)
		assert.Nil(t, result)
	})

	t.Run("non-map type returns error", func(t *testing.T) {
		_, err := extractLogProbs("invalid")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expected to map[string]any")
	})

	t.Run("valid map", func(t *testing.T) {
		input := map[string]any{
			"content": []any{
				map[string]any{
					"token":   "hello",
					"logprob": 0.5,
				},
			},
		}
		result, err := extractLogProbs(input)
		assert.Nil(t, err)
		assert.NotNil(t, result)
		assert.Len(t, result.Content, 1)
	})
}

func TestToToolParam(t *testing.T) {
	t.Run("nil schema returns nil", func(t *testing.T) {
		result := toToolParam(nil)
		assert.Nil(t, result)
	})

	t.Run("with properties and required", func(t *testing.T) {
		s := &jsonschema.Schema{
			Type:     "object",
			Required: []string{"name"},
			Properties: orderedmap.New[string, *jsonschema.Schema](
				orderedmap.WithInitialData[string, *jsonschema.Schema](
					orderedmap.Pair[string, *jsonschema.Schema]{
						Key:   "name",
						Value: &jsonschema.Schema{Type: "string"},
					},
				),
			),
		}
		result := toToolParam(s)
		assert.NotNil(t, result)
		assert.Equal(t, "object", result.Type)
		assert.Equal(t, []string{"name"}, result.Required)
		assert.Contains(t, result.Properties, "name")
	})
}

func TestTokenUsageConversions(t *testing.T) {
	t.Run("toEinoTokenUsage nil", func(t *testing.T) {
		assert.Nil(t, toEinoTokenUsage(nil))
	})

	t.Run("toCallbackUsage nil", func(t *testing.T) {
		assert.Nil(t, toCallbackUsage(nil))
	})

	t.Run("toModelCallbackUsage nil respMeta", func(t *testing.T) {
		assert.Nil(t, toModelCallbackUsage(nil))
	})

	t.Run("toModelCallbackUsage nil usage", func(t *testing.T) {
		assert.Nil(t, toModelCallbackUsage(&schema.ResponseMeta{}))
	})

	t.Run("toModelCallbackUsage with usage", func(t *testing.T) {
		result := toModelCallbackUsage(&schema.ResponseMeta{
			Usage: &schema.TokenUsage{
				PromptTokens:     1,
				CompletionTokens: 2,
				TotalTokens:      3,
			},
		})
		assert.NotNil(t, result)
		assert.Equal(t, 1, result.PromptTokens)
	})

	t.Run("streamToEinoTokenUsage nil", func(t *testing.T) {
		assert.Nil(t, streamToEinoTokenUsage(nil))
	})

	t.Run("streamToEinoTokenUsage all zero", func(t *testing.T) {
		assert.Nil(t, streamToEinoTokenUsage(&deepseek.StreamUsage{}))
	})

	t.Run("streamToEinoTokenUsage with values", func(t *testing.T) {
		result := streamToEinoTokenUsage(&deepseek.StreamUsage{
			PromptTokens:     5,
			CompletionTokens: 10,
			TotalTokens:      15,
		})
		assert.NotNil(t, result)
		assert.Equal(t, 5, result.PromptTokens)
		assert.Equal(t, 15, result.TotalTokens)
	})

	t.Run("toEinoTokenUsage with cache hit", func(t *testing.T) {
		result := toEinoTokenUsage(&deepseek.Usage{
			PromptTokens:         10,
			PromptCacheHitTokens: 5,
			CompletionTokens:     20,
			TotalTokens:          30,
		})
		assert.NotNil(t, result)
		assert.Equal(t, 5, result.PromptTokenDetails.CachedTokens)
	})

	t.Run("toCallbackUsage with values", func(t *testing.T) {
		result := toCallbackUsage(&schema.TokenUsage{
			PromptTokens:       1,
			CompletionTokens:   2,
			TotalTokens:        3,
			PromptTokenDetails: schema.PromptTokenDetails{CachedTokens: 1},
		})
		assert.NotNil(t, result)
		assert.Equal(t, 1, result.PromptTokenDetails.CachedTokens)
	})
}

func TestBindToolsError(t *testing.T) {
	cm := &ChatModel{conf: &ChatModelConfig{Model: "test"}}

	t.Run("BindTools empty", func(t *testing.T) {
		err := cm.BindTools(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no tools to bind")
	})

	t.Run("BindForcedTools empty", func(t *testing.T) {
		err := cm.BindForcedTools(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no tools to bind")
	})

	t.Run("WithTools empty", func(t *testing.T) {
		_, err := cm.WithTools(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no tools to bind")
	})

	t.Run("BindTools nil tool info", func(t *testing.T) {
		err := cm.BindTools([]*schema.ToolInfo{nil})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "tool info cannot be nil")
	})
}

func TestToMessageRole(t *testing.T) {
	assert.Equal(t, schema.User, toMessageRole("user"))
	assert.Equal(t, schema.Assistant, toMessageRole("assistant"))
	assert.Equal(t, schema.System, toMessageRole("system"))
	assert.Equal(t, schema.Tool, toMessageRole("tool"))
	assert.Equal(t, schema.RoleType("custom"), toMessageRole("custom"))
}

func TestToMessageToolCalls(t *testing.T) {
	t.Run("empty returns nil", func(t *testing.T) {
		assert.Nil(t, toMessageToolCalls(nil))
		assert.Nil(t, toMessageToolCalls([]deepseek.ToolCall{}))
	})

	t.Run("converts correctly", func(t *testing.T) {
		calls := []deepseek.ToolCall{
			{Index: 0, ID: "c1", Type: "function", Function: deepseek.ToolCallFunction{Name: "fn1", Arguments: "{}"}},
		}
		result := toMessageToolCalls(calls)
		assert.Len(t, result, 1)
		assert.Equal(t, "c1", result[0].ID)
		assert.Equal(t, 0, *result[0].Index)
	})
}

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

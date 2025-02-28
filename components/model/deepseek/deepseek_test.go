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
	"github.com/cloudwego/eino/schema"
	"github.com/cohesion-org/deepseek-go"
	"github.com/stretchr/testify/assert"
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
						ToolCalls:        nil,
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
	result, err := cm.Generate(ctx, []*schema.Message{schema.SystemMessage("system"), schema.UserMessage("hello"), schema.AssistantMessage("assistant", nil), schema.UserMessage("hello")})
	assert.Nil(t, err)
	expected := &schema.Message{
		Role:    schema.Assistant,
		Content: "hello world",
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
		APIKey:  "my-api-key",
		Timeout: time.Second,
		Model:   "deepseek-chat",
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
	assert.Equal(t, &schema.Message{
		Role:    schema.Assistant,
		Content: "Hello World",
		ResponseMeta: &schema.ResponseMeta{Usage: &schema.TokenUsage{
			PromptTokens:     1,
			CompletionTokens: 2,
			TotalTokens:      3,
		}},
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

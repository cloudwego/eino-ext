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
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/shared/constant"
	"github.com/bytedance/mockey"
	"github.com/stretchr/testify/assert"

	"github.com/cloudwego/eino/schema"
)

func TestNewChatModel(t *testing.T) {
	ctx := context.Background()

	t.Run("missing APIKey", func(t *testing.T) {
		_, err := NewChatModel(ctx, &Config{Model: "test"})
		assert.ErrorContains(t, err, "APIKey is required")
	})

	t.Run("missing model", func(t *testing.T) {
		_, err := NewChatModel(ctx, &Config{APIKey: "test"})
		assert.ErrorContains(t, err, "model is required")
	})

	t.Run("missing maxTokens", func(t *testing.T) {
		_, err := NewChatModel(ctx, &Config{APIKey: "test", Model: "test"})
		assert.ErrorContains(t, err, "maxTokens is required")
	})

	t.Run("valid config", func(t *testing.T) {
		cm, err := NewChatModel(ctx, &Config{
			APIKey:    "test-key",
			Model:     "MiniMax-M2.7",
			MaxTokens: 1024,
		})
		assert.NoError(t, err)
		assert.NotNil(t, cm)
		assert.Equal(t, "minimax", cm.GetType())
		assert.True(t, cm.IsCallbacksEnabled())
	})
}

func TestGenerate(t *testing.T) {
	ctx := context.Background()

	mockey.PatchConvey("basic chat", t, func() {
		cm := &ChatModel{
			model:     "MiniMax-M2.7",
			maxTokens: 1024,
		}

		content := anthropic.ContentBlockUnion{
			Type: "text",
			Text: "Hello, I'm MiniMax!",
		}
		defer mockey.Mock(anthropic.ContentBlockUnion.AsAny).Return(anthropic.TextBlock{
			Type: constant.Text(content.Type),
			Text: content.Text,
		}).Build().UnPatch()
		defer mockey.Mock((*anthropic.MessageService).New).Return(&anthropic.Message{
			Content: []anthropic.ContentBlockUnion{content},
			Usage: anthropic.Usage{
				InputTokens:  10,
				OutputTokens: 5,
			},
		}, nil).Build().UnPatch()

		resp, err := cm.Generate(ctx, []*schema.Message{
			schema.UserMessage("Hi, who are you?"),
		})

		assert.NoError(t, err)
		assert.Equal(t, "Hello, I'm MiniMax!", resp.Content)
		assert.Equal(t, schema.Assistant, resp.Role)
		assert.Equal(t, 10, resp.ResponseMeta.Usage.PromptTokens)
		assert.Equal(t, 5, resp.ResponseMeta.Usage.CompletionTokens)
	})
}

func TestWithTools(t *testing.T) {
	cm := &ChatModel{model: "test model"}
	ncm, err := cm.WithTools([]*schema.ToolInfo{{Name: "test tool"}})
	assert.Nil(t, err)
	assert.Equal(t, "test model", ncm.(*ChatModel).model)
}

func TestBindTools(t *testing.T) {
	cm := &ChatModel{model: "test model"}
	err := cm.BindTools([]*schema.ToolInfo{{Name: "test tool"}})
	assert.NoError(t, err)
	assert.Equal(t, "test tool", cm.origTools[0].Name)
}

func TestBindForcedTools(t *testing.T) {
	cm := &ChatModel{model: "test model"}
	err := cm.BindForcedTools([]*schema.ToolInfo{{Name: "test tool"}})
	assert.NoError(t, err)
	assert.Equal(t, "test tool", cm.origTools[0].Name)
}

func TestPanicErr(t *testing.T) {
	err := newPanicErr("info", []byte("stack"))
	assert.Equal(t, "panic error: info, \nstack: stack", err.Error())
}

func TestAPIError(t *testing.T) {
	err := &APIError{
		Type:       "invalid_request",
		Message:    "test error",
		HTTPStatus: 400,
	}
	assert.Equal(t, "minimax error, status code: 400, type: invalid_request, message: test error", err.Error())
}

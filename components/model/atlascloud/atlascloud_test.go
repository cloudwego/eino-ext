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

package atlascloud

import (
	"context"
	"errors"
	"testing"

	"github.com/bytedance/mockey"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"

	modelopenai "github.com/cloudwego/eino-ext/components/model/openai"
)

func TestNewChatModel(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		chatModel, err := NewChatModel(context.Background(), nil)
		assert.Error(t, err)
		assert.Nil(t, chatModel)
	})

	t.Run("default base url", func(t *testing.T) {
		var captured *modelopenai.ChatModelConfig

		patch := mockey.Mock(modelopenai.NewChatModel).
			To(func(_ context.Context, config *modelopenai.ChatModelConfig) (*modelopenai.ChatModel, error) {
				captured = config
				return &modelopenai.ChatModel{}, nil
			}).Build()
		defer patch.UnPatch()

		chatModel, err := NewChatModel(context.Background(), &ChatModelConfig{
			APIKey: "test-key",
			Model:  "deepseek-v3",
		})
		assert.NoError(t, err)
		assert.NotNil(t, chatModel)
		assert.Equal(t, DefaultBaseURL, captured.BaseURL)
	})

	t.Run("custom base url preserved", func(t *testing.T) {
		var captured *modelopenai.ChatModelConfig

		patch := mockey.Mock(modelopenai.NewChatModel).
			To(func(_ context.Context, config *modelopenai.ChatModelConfig) (*modelopenai.ChatModel, error) {
				captured = config
				return &modelopenai.ChatModel{}, nil
			}).Build()
		defer patch.UnPatch()

		chatModel, err := NewChatModel(context.Background(), &ChatModelConfig{
			APIKey:  "test-key",
			Model:   "deepseek-v3",
			BaseURL: "https://custom.example/v1",
		})
		assert.NoError(t, err)
		assert.NotNil(t, chatModel)
		assert.Equal(t, "https://custom.example/v1", captured.BaseURL)
	})
}

func TestChatModelDelegates(t *testing.T) {
	t.Run("generate and stream", func(t *testing.T) {
		patchGenerate := mockey.Mock((*modelopenai.ChatModel).Generate).
			To(func(_ *modelopenai.ChatModel, _ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
				return schema.AssistantMessage("ok"), nil
			}).Build()
		defer patchGenerate.UnPatch()

		patchStream := mockey.Mock((*modelopenai.ChatModel).Stream).
			To(func(_ *modelopenai.ChatModel, _ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
				return &schema.StreamReader[*schema.Message]{}, nil
			}).Build()
		defer patchStream.UnPatch()

		cm := &ChatModel{inner: &modelopenai.ChatModel{}}

		msg, err := cm.Generate(t.Context(), []*schema.Message{schema.UserMessage("hello")})
		assert.NoError(t, err)
		assert.Equal(t, "ok", msg.Content)

		stream, err := cm.Stream(t.Context(), []*schema.Message{schema.UserMessage("hello")})
		assert.NoError(t, err)
		assert.NotNil(t, stream)
	})

	t.Run("with tools wraps result", func(t *testing.T) {
		patch := mockey.Mock((*modelopenai.ChatModel).WithTools).
			To(func(_ *modelopenai.ChatModel, _ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
				return &modelopenai.ChatModel{}, nil
			}).Build()
		defer patch.UnPatch()

		cm := &ChatModel{inner: &modelopenai.ChatModel{}}
		toolModel, err := cm.WithTools([]*schema.ToolInfo{{Name: "test-tool"}})
		assert.NoError(t, err)
		assert.IsType(t, &ChatModel{}, toolModel)
	})

	t.Run("with tools returns underlying error", func(t *testing.T) {
		patch := mockey.Mock((*modelopenai.ChatModel).WithTools).
			To(func(_ *modelopenai.ChatModel, _ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
				return nil, errors.New("boom")
			}).Build()
		defer patch.UnPatch()

		cm := &ChatModel{inner: &modelopenai.ChatModel{}}
		toolModel, err := cm.WithTools([]*schema.ToolInfo{{Name: "test-tool"}})
		assert.Error(t, err)
		assert.Nil(t, toolModel)
	})
}

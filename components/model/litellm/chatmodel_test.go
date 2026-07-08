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

package litellm

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/bytedance/mockey"
	"github.com/cloudwego/eino-ext/libs/acl/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
)

func TestNewChatModel(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		config := &Config{
			APIKey:  "test-api-key",
			BaseURL: "http://localhost:4000",
			Timeout: 30 * time.Second,
			Model:   "openai/gpt-4o",
		}
		chatModel, err := NewChatModel(context.Background(), config)
		assert.NoError(t, err)
		assert.NotNil(t, chatModel)
		assert.NotNil(t, chatModel.cli)
		assert.Equal(t, typ, chatModel.GetType())
		assert.True(t, chatModel.IsCallbacksEnabled())
	})

	t.Run("nil config", func(t *testing.T) {
		chatModel, err := NewChatModel(context.Background(), nil)
		assert.Error(t, err)
		assert.Nil(t, chatModel)
	})

	t.Run("missing base_url", func(t *testing.T) {
		config := &Config{
			APIKey: "test-api-key",
			Model:  "openai/gpt-4o",
		}
		chatModel, err := NewChatModel(context.Background(), config)
		assert.Error(t, err)
		assert.Nil(t, chatModel)
		assert.Contains(t, err.Error(), "base_url is required")
	})

	t.Run("custom http client", func(t *testing.T) {
		customClient := &http.Client{
			Timeout: 60 * time.Second,
		}
		config := &Config{
			APIKey:     "test-api-key",
			BaseURL:    "http://localhost:4000",
			HTTPClient: customClient,
			Model:      "anthropic/claude-sonnet-4-20250514",
		}
		chatModel, err := NewChatModel(context.Background(), config)
		assert.NoError(t, err)
		assert.NotNil(t, chatModel)
	})

	t.Run("with extra fields", func(t *testing.T) {
		config := &Config{
			APIKey:  "test-api-key",
			BaseURL: "http://localhost:4000",
			Model:   "openai/gpt-4o",
			ExtraFields: map[string]any{
				"drop_params": true,
				"metadata": map[string]string{
					"team": "engineering",
				},
			},
		}
		chatModel, err := NewChatModel(context.Background(), config)
		assert.NoError(t, err)
		assert.NotNil(t, chatModel)
	})

	t.Run("with extra fields for drop_params", func(t *testing.T) {
		config := &Config{
			APIKey:  "test-api-key",
			BaseURL: "http://localhost:4000",
			Model:   "openai/gpt-4o",
			ExtraFields: map[string]any{
				"drop_params": true,
			},
		}
		chatModel, err := NewChatModel(context.Background(), config)
		assert.NoError(t, err)
		assert.NotNil(t, chatModel)
	})
}

func TestChatModel_Generate(t *testing.T) {
	mockey.PatchConvey("Generate delegates to openai client", t, func() {
		expectedMsg := &schema.Message{
			Role:    schema.Assistant,
			Content: "4",
		}

		mockey.Mock((*openai.Client).Generate).Return(expectedMsg, nil).Build()

		config := &Config{
			APIKey:  "test-api-key",
			BaseURL: "http://localhost:4000",
			Model:   "openai/gpt-4o",
		}
		chatModel, err := NewChatModel(context.Background(), config)
		assert.NoError(t, err)

		messages := []*schema.Message{
			{Role: schema.User, Content: "What is 2+2?"},
		}

		result, err := chatModel.Generate(context.Background(), messages)
		assert.NoError(t, err)
		assert.Equal(t, "4", result.Content)
	})

	mockey.PatchConvey("Generate returns error from client", t, func() {
		mockey.Mock((*openai.Client).Generate).Return(nil, errors.New("api error")).Build()

		config := &Config{
			APIKey:  "test-api-key",
			BaseURL: "http://localhost:4000",
			Model:   "openai/gpt-4o",
		}
		chatModel, err := NewChatModel(context.Background(), config)
		assert.NoError(t, err)

		messages := []*schema.Message{
			{Role: schema.User, Content: "Hello"},
		}

		result, err := chatModel.Generate(context.Background(), messages)
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestChatModel_Stream(t *testing.T) {
	mockey.PatchConvey("Stream delegates to openai client", t, func() {
		r, w := schema.Pipe[*schema.Message](1)
		_ = w.Send(&schema.Message{Role: schema.Assistant, Content: "hello"}, nil)
		w.Close()

		mockey.Mock((*openai.Client).Stream).Return(r, nil).Build()

		config := &Config{
			APIKey:  "test-api-key",
			BaseURL: "http://localhost:4000",
			Model:   "openai/gpt-4o",
		}
		chatModel, err := NewChatModel(context.Background(), config)
		assert.NoError(t, err)

		messages := []*schema.Message{
			{Role: schema.User, Content: "Hello"},
		}

		stream, err := chatModel.Stream(context.Background(), messages)
		assert.NoError(t, err)
		assert.NotNil(t, stream)
	})
}

func TestChatModel_WithTools(t *testing.T) {
	mockey.PatchConvey("WithTools returns new instance", t, func() {
		mockey.Mock((*openai.Client).WithToolsForClient).Return(&openai.Client{}, nil).Build()

		config := &Config{
			APIKey:  "test-api-key",
			BaseURL: "http://localhost:4000",
			Model:   "openai/gpt-4o",
		}
		chatModel, err := NewChatModel(context.Background(), config)
		assert.NoError(t, err)

		tools := []*schema.ToolInfo{
			{
				Name: "get_weather",
				Desc: "Get current weather",
				ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
					"city": {Type: "string", Desc: "City name"},
				}),
			},
		}

		newModel, err := chatModel.WithTools(tools)
		assert.NoError(t, err)
		assert.NotNil(t, newModel)

		litellmModel, ok := newModel.(*ChatModel)
		assert.True(t, ok)
		assert.NotNil(t, litellmModel.cli)
	})
}

func TestChatModel_GetType(t *testing.T) {
	config := &Config{
		APIKey:  "test-api-key",
		BaseURL: "http://localhost:4000",
		Model:   "openai/gpt-4o",
	}
	chatModel, err := NewChatModel(context.Background(), config)
	assert.NoError(t, err)
	assert.Equal(t, "LiteLLM", chatModel.GetType())
}

func TestChatModel_WithModelOption(t *testing.T) {
	mockey.PatchConvey("Generate with model option override", t, func() {
		expectedMsg := &schema.Message{
			Role:    schema.Assistant,
			Content: "response",
		}

		mockey.Mock((*openai.Client).Generate).Return(expectedMsg, nil).Build()

		config := &Config{
			APIKey:  "test-api-key",
			BaseURL: "http://localhost:4000",
			Model:   "openai/gpt-4o",
		}
		chatModel, err := NewChatModel(context.Background(), config)
		assert.NoError(t, err)

		messages := []*schema.Message{
			{Role: schema.User, Content: "Hello"},
		}

		result, err := chatModel.Generate(context.Background(), messages, model.WithModel("anthropic/claude-sonnet-4-20250514"))
		assert.NoError(t, err)
		assert.Equal(t, "response", result.Content)
	})
}

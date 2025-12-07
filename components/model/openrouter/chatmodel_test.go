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

package openrouter

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
	jsoniter "github.com/json-iterator/go"
	"github.com/stretchr/testify/assert"
)

func TestNewChatModel(t *testing.T) {
	// 1. Test with a valid configuration
	t.Run("success", func(t *testing.T) {
		config := &Config{
			APIKey:    "test-api-key",
			Timeout:   30 * time.Second,
			Model:     "test-model",
			BaseURL:   "https://example.com",
			User:      new(string),
			LogitBias: map[string]int{"test": 1},
		}
		chatModel, err := NewChatModel(context.Background(), config)
		assert.Equal(t, chatModel.GetType(), typ)
		assert.Equal(t, chatModel.IsCallbacksEnabled(), true)
		assert.NoError(t, err)
		assert.NotNil(t, chatModel)
		assert.NotNil(t, chatModel.cli)
	})

	// 2. Test with a nil configuration
	t.Run("nil config", func(t *testing.T) {
		chatModel, err := NewChatModel(context.Background(), nil)
		assert.Error(t, err)
		assert.Nil(t, chatModel)
	})

	// 3. Test with a custom HTTP client
	t.Run("custom http client", func(t *testing.T) {
		customClient := &http.Client{
			Timeout: 60 * time.Second,
		}
		config := &Config{
			APIKey:     "test-api-key",
			HTTPClient: customClient,
			Model:      "test-model",
		}
		chatModel, err := NewChatModel(context.Background(), config)
		assert.NoError(t, err)
		assert.NotNil(t, chatModel)
		assert.NotNil(t, chatModel.cli)
	})

	// 4. Test with default BaseURL
	t.Run("default base url", func(t *testing.T) {
		config := &Config{
			APIKey: "test-api-key",
			Model:  "test-model",
		}
		chatModel, err := NewChatModel(context.Background(), config)
		assert.NoError(t, err)
		assert.NotNil(t, chatModel)
		assert.NotNil(t, chatModel.cli)
	})

	// 5. Test with all possible fields
	t.Run("all fields", func(t *testing.T) {
		maxTokens := 100
		maxCompletionTokens := 200
		seed := 123
		topP := float32(0.9)
		temperature := float32(0.8)
		presencePenalty := float32(0.1)
		frequencyPenalty := float32(0.2)
		user := "test-user"
		config := &Config{
			APIKey:              "test-api-key",
			Timeout:             30 * time.Second,
			BaseURL:             "https://example.com",
			Model:               "test-model",
			Models:              []string{"test-model-1", "test-model-2"},
			MaxTokens:           &maxTokens,
			MaxCompletionTokens: &maxCompletionTokens,
			Seed:                &seed,
			Stop:                []string{"stop1", "stop2"},
			TopP:                &topP,
			Temperature:         &temperature,
			ResponseFormat: &ChatCompletionResponseFormat{
				Type: "json_object",
			},
			PresencePenalty:  &presencePenalty,
			FrequencyPenalty: &frequencyPenalty,
			LogitBias:        map[string]int{"test": 1},
			LogProbs:         true,
			TopLogProbs:      5,
			Reasoning: &Reasoning{
				Effort:  "auto",
				Summary: "auto",
			},
			User:     &user,
			Metadata: map[string]string{"key": "value"},
			ExtraFields: map[string]any{
				"extra": "field",
			},
		}
		chatModel, err := NewChatModel(context.Background(), config)
		assert.NoError(t, err)
		assert.NotNil(t, chatModel)
		assert.NotNil(t, chatModel.cli)
		assert.Equal(t, config.Models, chatModel.models)
		assert.Equal(t, config.ResponseFormat, chatModel.responseFormat)
		assert.Equal(t, config.Reasoning, chatModel.reasoning)
		assert.Equal(t, config.Metadata, chatModel.metadata)
	})
}

func TestChatModel_Generate_Stream(t *testing.T) {
	ctx := t.Context()
	cm := &ChatModel{}
	mockey.PatchConvey("normal", t, func() {
		mockey.Mock((*ChatModel).buildOptions).Return([]model.Option{}).Build()
		mockey.Mock((*openai.Client).Generate).Return(&schema.Message{Role: schema.User, Content: "ok"}, nil).Build()
		ret, err := cm.Generate(ctx, []*schema.Message{{Role: schema.User, Content: "ok"}})
		assert.Nil(t, err)
		assert.Equal(t, &schema.Message{Role: schema.User, Content: "ok"}, ret)

		mockey.Mock((*openai.Client).Stream).Return(&schema.StreamReader[*schema.Message]{}, nil).Build()

		_, err = cm.Stream(ctx, []*schema.Message{{Role: schema.User, Content: "ok"}})
		assert.Nil(t, err)

	})
	mockey.PatchConvey("error", t, func() {
		mockey.Mock((*ChatModel).buildOptions).Return([]model.Option{}).Build()
		mockey.Mock((*openai.Client).Generate).Return(nil, errors.New("error")).Build()
		_, err := cm.Generate(ctx, []*schema.Message{{Role: schema.User, Content: "ok"}})
		assert.NotNil(t, err)
		mockey.Mock((*openai.Client).Stream).Return(nil, errors.New("error")).Build()

		_, err = cm.Stream(ctx, []*schema.Message{{Role: schema.User, Content: "ok"}})
		assert.NotNil(t, err)
	})

}

func TestChatModel_buildRequestModifier(t *testing.T) {
	cm := &ChatModel{
		models:    []string{"mode1", "mode2"},
		reasoning: &Reasoning{Effort: EffortOfNone},
		responseFormat: &ChatCompletionResponseFormat{
			Type: "json_object",
		},
	}
	modifier := cm.buildRequestModifier(&openrouterOption{
		models: []string{"option_v1", "option_v2"},
	})

	inMsg := []*schema.Message{
		schema.UserMessage("hello"),
	}
	setReasoningDetails(inMsg[0], []*reasoningDetails{
		{Format: "reasoning.text", Text: "ok"},
	})
	body, err := modifier(t.Context(), inMsg, []byte(`{"messages":[{"role":"user"}]}`))
	assert.Nil(t, err)
	models := make([]string, 0, 2)
	jsoniter.Get(body, "models").ToVal(&models)
	assert.Equal(t, models, []string{"option_v1", "option_v2"})

	responseFormat := &ChatCompletionResponseFormat{}
	jsoniter.Get(body, "response_format").ToVal(responseFormat)

	assert.Equal(t, *responseFormat, ChatCompletionResponseFormat{Type: "json_object"})

}

func TestChatModel_buildResponseMessageModifier(t *testing.T) {
	cm := &ChatModel{}
	modifier := cm.buildResponseMessageModifier()
	ctx := context.Background()

	t.Run("success with reasoning details", func(t *testing.T) {
		msg := &schema.Message{}
		rawBody := []byte(`{"choices":[{"index":0,"message":{"reasoning":"test reasoning","reasoning_details":[{"format":"text","text":"detail"}]}}]}`)
		modifiedMsg, err := modifier(ctx, msg, rawBody)
		assert.NoError(t, err)
		details, ok := getReasoningDetails(modifiedMsg)
		assert.True(t, ok)
		assert.Len(t, details, 1)
		assert.Equal(t, "text", details[0].Format)
		assert.Equal(t, "detail", details[0].Text)
	})

	t.Run("success with images", func(t *testing.T) {
		msg := &schema.Message{}
		rawBody := []byte(`{
		  "choices": [
			{
			  "index": 0,
			  "message": {
				"reasoning": "test reasoning",
				"reasoning_details": [
				  {
					"format": "text",
					"text": "detail"
				  }
				],
				"images": [
				  {
					"type": "image_url",
					"image_url": {
					  "url": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAABAAAAAQACA"
					}
				  }
				]
			  }
			}
		  ]
		}`)
		modifiedMsg, err := modifier(ctx, msg, rawBody)
		assert.NoError(t, err)
		details, ok := getReasoningDetails(modifiedMsg)
		assert.True(t, ok)
		assert.Len(t, details, 1)
		assert.Len(t, msg.AssistantGenMultiContent, 1)

	})

	t.Run("no choices", func(t *testing.T) {
		msg := &schema.Message{}
		rawBody := []byte(`{}`)
		modifiedMsg, err := modifier(ctx, msg, rawBody)
		assert.NoError(t, err)
		assert.Equal(t, msg, modifiedMsg)
	})

	t.Run("no choice with index 0", func(t *testing.T) {
		msg := &schema.Message{}
		rawBody := []byte(`{"choices":[{"index":1,"message":{"reasoning":"test reasoning"}}]}`)
		modifiedMsg, err := modifier(ctx, msg, rawBody)
		assert.NoError(t, err)
		assert.Equal(t, msg, modifiedMsg)
	})

}

func TestChatModel_buildResponseChunkMessageModifier(t *testing.T) {
	cm := &ChatModel{}
	modifier := cm.buildResponseChunkMessageModifier()
	ctx := context.Background()

	t.Run("success with reasoning details", func(t *testing.T) {
		msg := &schema.Message{}
		rawBody := []byte(`{
		  "choices": [
			{
			  "index": 0,
			  "delta": {
				"reasoning": "test reasoning",
				"reasoning_details": [
				  {
					"format": "text",
					"text": "detail"
				  }
				],
				"images": [
				  {
					"type": "image_url",
					"image_url": {
					  "url": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAABAAAAAQACA"
					}
				  }
				]
			  }
			}
		  ]
		}`)
		modifiedMsg, err := modifier(ctx, msg, rawBody, false)
		assert.NoError(t, err)
		details, ok := getReasoningDetails(modifiedMsg)
		assert.True(t, ok)
		assert.Len(t, details, 1)
		assert.Equal(t, "text", details[0].Format)
		assert.Equal(t, "detail", details[0].Text)
		assert.Len(t, msg.AssistantGenMultiContent, 1)
	})

	t.Run("success with images", func(t *testing.T) {
		msg := &schema.Message{}
		rawBody := []byte(`{"choices":[{"index":0,"delta":{"reasoning":"test reasoning","reasoning_details":[{"format":"text","text":"detail"}]}}]}`)
		modifiedMsg, err := modifier(ctx, msg, rawBody, false)
		assert.NoError(t, err)
		details, ok := getReasoningDetails(modifiedMsg)
		assert.True(t, ok)
		assert.Len(t, details, 1)
		assert.Equal(t, "text", details[0].Format)
		assert.Equal(t, "detail", details[0].Text)
	})

	t.Run("error finish reason", func(t *testing.T) {
		msg := &schema.Message{
			ResponseMeta: &schema.ResponseMeta{
				FinishReason: "error",
			},
		}
		rawBody := []byte(`{"error":{"message":"test error"}}`)
		modifiedMsg, err := modifier(ctx, msg, rawBody, true)
		assert.NoError(t, err)
		terminatedError, ok := GetStreamTerminatedError(modifiedMsg)
		assert.True(t, ok)
		assert.NotNil(t, terminatedError)
		assert.Contains(t, terminatedError.Message, "test error")
	})

	t.Run("no choices", func(t *testing.T) {
		msg := &schema.Message{}
		rawBody := []byte(`{}`)
		modifiedMsg, err := modifier(ctx, msg, rawBody, false)
		assert.NoError(t, err)
		assert.Equal(t, msg, modifiedMsg)
	})

	t.Run("no choice with index 0", func(t *testing.T) {
		msg := &schema.Message{}
		rawBody := []byte(`{"choices":[{"index":1,"delta":{"reasoning":"test reasoning"}}]}`)
		modifiedMsg, err := modifier(ctx, msg, rawBody, false)
		assert.NoError(t, err)
		assert.Equal(t, msg, modifiedMsg)
	})

	t.Run("invalid choices json", func(t *testing.T) {
		msg := &schema.Message{}
		rawBody := []byte(`{"choices":"invalid"}`)
		_, err := modifier(ctx, msg, rawBody, false)
		assert.Error(t, err)
	})
}

func TestChatModel_Tools(t *testing.T) {
	config := &Config{
		APIKey:    "test-api-key",
		Timeout:   30 * time.Second,
		Model:     "test-model",
		BaseURL:   "https://example.com",
		User:      new(string),
		LogitBias: map[string]int{"test": 1},
	}
	chatModel, err := NewChatModel(context.Background(), config)
	assert.NoError(t, err)

	_, err = chatModel.WithTools([]*schema.ToolInfo{
		{Name: "test-tool", Desc: "test tool description", ParamsOneOf: &schema.ParamsOneOf{}},
	})

	assert.NoError(t, err)

}

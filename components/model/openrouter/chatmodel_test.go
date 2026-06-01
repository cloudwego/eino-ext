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
		responseFormat: &ChatCompletionResponseFormat{
			Type: "json_object",
		},
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

func TestChatModel_buildRequestModifier_CacheControl(t *testing.T) {
	cm := &ChatModel{}

	t.Run("UserInputMultiContent text part with cache control", func(t *testing.T) {
		part := schema.MessageInputPart{
			Type: schema.ChatMessagePartTypeText,
			Text: "hello",
		}
		EnableMessageInputPartCacheControl(&part)

		msg := &schema.Message{
			Role:                  schema.User,
			UserInputMultiContent: []schema.MessageInputPart{part},
		}

		modifier := cm.buildRequestModifier(&openrouterOption{})
		body, err := modifier(t.Context(), []*schema.Message{msg}, []byte(`{"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]}`))
		assert.NoError(t, err)

		cacheCtrl := jsoniter.Get(body, "messages", 0, "content", 0, "cache_control")
		assert.Equal(t, "ephemeral", cacheCtrl.Get("type").ToString())
	})

	t.Run("UserInputMultiContent text part without cache control skipped", func(t *testing.T) {
		part := schema.MessageInputPart{
			Type: schema.ChatMessagePartTypeText,
			Text: "hello",
		}

		msg := &schema.Message{
			Role:                  schema.User,
			UserInputMultiContent: []schema.MessageInputPart{part},
		}

		modifier := cm.buildRequestModifier(&openrouterOption{})
		body, err := modifier(t.Context(), []*schema.Message{msg}, []byte(`{"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]}`))
		assert.NoError(t, err)

		cacheCtrl := jsoniter.Get(body, "messages", 0, "content", 0, "cache_control")
		assert.Equal(t, jsoniter.InvalidValue, cacheCtrl.ValueType())
	})

	t.Run("UserInputMultiContent non-text part without cache control skipped", func(t *testing.T) {
		textPart := schema.MessageInputPart{
			Type: schema.ChatMessagePartTypeText,
			Text: "hello",
		}
		imgPart := schema.MessageInputPart{
			Type: schema.ChatMessagePartTypeImageURL,
		}

		msg := &schema.Message{
			Role:                  schema.User,
			UserInputMultiContent: []schema.MessageInputPart{textPart, imgPart},
		}

		modifier := cm.buildRequestModifier(&openrouterOption{})
		body, err := modifier(t.Context(), []*schema.Message{msg}, []byte(`{"messages":[{"role":"user","content":[{"type":"text","text":"hello"},{"type":"image_url"}]}]}`))
		assert.NoError(t, err)
		assert.NotNil(t, body)
	})

	t.Run("UserInputMultiContent non-text part with cache control injects cache_control", func(t *testing.T) {
		imgPart := schema.MessageInputPart{
			Type: schema.ChatMessagePartTypeImageURL,
		}
		EnableMessageInputPartCacheControl(&imgPart)

		msg := &schema.Message{
			Role:                  schema.User,
			UserInputMultiContent: []schema.MessageInputPart{imgPart},
		}

		modifier := cm.buildRequestModifier(&openrouterOption{})
		body, err := modifier(t.Context(), []*schema.Message{msg}, []byte(`{"messages":[{"role":"user","content":[{"type":"image_url"}]}]}`))
		assert.NoError(t, err)

		cacheCtrl := jsoniter.Get(body, "messages", 0, "content", 0, "cache_control")
		assert.Equal(t, "ephemeral", cacheCtrl.Get("type").ToString())
	})

	t.Run("AssistantGenMultiContent text part with cache control", func(t *testing.T) {
		part := schema.MessageOutputPart{
			Type: schema.ChatMessagePartTypeText,
			Text: "response",
		}
		EnableMessageOutputPartCacheControl(&part)

		msg := &schema.Message{
			Role:                     schema.Assistant,
			AssistantGenMultiContent: []schema.MessageOutputPart{part},
		}

		modifier := cm.buildRequestModifier(&openrouterOption{})
		body, err := modifier(t.Context(), []*schema.Message{msg}, []byte(`{"messages":[{"role":"assistant","content":[{"type":"text","text":"response"}]}]}`))
		assert.NoError(t, err)

		cacheCtrl := jsoniter.Get(body, "messages", 0, "content", 0, "cache_control")
		assert.Equal(t, "ephemeral", cacheCtrl.Get("type").ToString())
	})

	t.Run("AssistantGenMultiContent non-text part without cache control skipped", func(t *testing.T) {
		textPart := schema.MessageOutputPart{
			Type: schema.ChatMessagePartTypeText,
			Text: "response",
		}
		imgPart := schema.MessageOutputPart{
			Type: schema.ChatMessagePartTypeImageURL,
		}

		msg := &schema.Message{
			Role:                     schema.Assistant,
			AssistantGenMultiContent: []schema.MessageOutputPart{textPart, imgPart},
		}

		modifier := cm.buildRequestModifier(&openrouterOption{})
		body, err := modifier(t.Context(), []*schema.Message{msg}, []byte(`{"messages":[{"role":"assistant","content":[{"type":"text","text":"response"},{"type":"image_url"}]}]}`))
		assert.NoError(t, err)
		assert.NotNil(t, body)
	})

	t.Run("AssistantGenMultiContent non-text part with cache control injects cache_control", func(t *testing.T) {
		imgPart := schema.MessageOutputPart{
			Type: schema.ChatMessagePartTypeImageURL,
		}
		EnableMessageOutputPartCacheControl(&imgPart)

		msg := &schema.Message{
			Role:                     schema.Assistant,
			AssistantGenMultiContent: []schema.MessageOutputPart{imgPart},
		}

		modifier := cm.buildRequestModifier(&openrouterOption{})
		body, err := modifier(t.Context(), []*schema.Message{msg}, []byte(`{"messages":[{"role":"assistant","content":[{"type":"image_url"}]}]}`))
		assert.NoError(t, err)

		cacheCtrl := jsoniter.Get(body, "messages", 0, "content", 0, "cache_control")
		assert.Equal(t, "ephemeral", cacheCtrl.Get("type").ToString())
	})

	t.Run("message content with cache control", func(t *testing.T) {
		msg := &schema.Message{
			Role:    schema.User,
			Content: "hello",
		}
		EnableMessageContentCacheControl(msg)

		modifier := cm.buildRequestModifier(&openrouterOption{})
		body, err := modifier(t.Context(), []*schema.Message{msg}, []byte(`{"messages":[{"role":"user","content":"hello"}]}`))
		assert.NoError(t, err)

		cacheCtrl := jsoniter.Get(body, "messages", 0, "cache_control")
		assert.Equal(t, "ephemeral", cacheCtrl.Get("type").ToString())
	})

	t.Run("top-level cache_control via option", func(t *testing.T) {
		msg := &schema.Message{Role: schema.User, Content: "hello"}

		ctrl := CacheControl{TTL: CacheControlTTL1Hour}
		modifier := cm.buildRequestModifier(&openrouterOption{cacheControl: ctrl.toInternal()})
		body, err := modifier(t.Context(), []*schema.Message{msg}, []byte(`{"messages":[{"role":"user","content":"hello"}]}`))
		assert.NoError(t, err)

		cacheCtrl := jsoniter.Get(body, "cache_control")
		assert.Equal(t, "ephemeral", cacheCtrl.Get("type").ToString())
		assert.Equal(t, "1h", cacheCtrl.Get("ttl").ToString())
	})

	t.Run("top-level cache_control not set when option is nil", func(t *testing.T) {
		msg := &schema.Message{Role: schema.User, Content: "hello"}

		modifier := cm.buildRequestModifier(&openrouterOption{})
		body, err := modifier(t.Context(), []*schema.Message{msg}, []byte(`{"messages":[{"role":"user","content":"hello"}]}`))
		assert.NoError(t, err)

		cacheCtrl := jsoniter.Get(body, "cache_control")
		assert.Equal(t, jsoniter.InvalidValue, cacheCtrl.ValueType())
	})
}

func TestChatModel_buildOptions_CacheControlFallback(t *testing.T) {
	configCtrl := &CacheControl{TTL: CacheControlTTL5Minutes}
	cm := &ChatModel{
		cacheControl: configCtrl.toInternal(),
	}

	t.Run("config-level cacheControl used when no option override", func(t *testing.T) {
		modifier := cm.buildRequestModifier(&openrouterOption{cacheControl: cm.cacheControl})
		body, err := modifier(t.Context(), []*schema.Message{{Role: schema.User, Content: "hi"}}, []byte(`{"messages":[{"role":"user","content":"hi"}]}`))
		assert.NoError(t, err)

		cacheCtrl := jsoniter.Get(body, "cache_control")
		assert.Equal(t, "ephemeral", cacheCtrl.Get("type").ToString())
		assert.Equal(t, "5m", cacheCtrl.Get("ttl").ToString())
	})

	t.Run("option-level cacheControl overrides config", func(t *testing.T) {
		optCtrl := CacheControl{TTL: CacheControlTTL1Hour}
		modifier := cm.buildRequestModifier(&openrouterOption{cacheControl: optCtrl.toInternal()})
		body, err := modifier(t.Context(), []*schema.Message{{Role: schema.User, Content: "hi"}}, []byte(`{"messages":[{"role":"user","content":"hi"}]}`))
		assert.NoError(t, err)

		cacheCtrl := jsoniter.Get(body, "cache_control")
		assert.Equal(t, "1h", cacheCtrl.Get("ttl").ToString())
	})
}

func TestChatModel_WithTools_PreservesFields(t *testing.T) {
	ctrl := &CacheControl{TTL: CacheControlTTL1Hour}
	rf := &ChatCompletionResponseFormat{Type: ChatCompletionResponseFormatTypeText}
	cm := &ChatModel{
		models:         []string{"model-a", "model-b"},
		reasoning:      &Reasoning{Effort: "high"},
		metadata:       map[string]string{"key": "value"},
		cacheControl:   ctrl.toInternal(),
		responseFormat: rf,
	}

	mockey.PatchConvey("WithTools preserves all fields", t, func() {
		mockey.Mock((*openai.Client).WithToolsForClient).Return(&openai.Client{}, nil).Build()

		toolModel, err := cm.WithTools([]*schema.ToolInfo{{Name: "test"}})
		assert.NoError(t, err)

		newCM := toolModel.(*ChatModel)
		assert.Equal(t, cm.models, newCM.models)
		assert.Equal(t, cm.reasoning, newCM.reasoning)
		assert.Equal(t, cm.metadata, newCM.metadata)
		assert.Equal(t, cm.cacheControl, newCM.cacheControl)
		assert.Equal(t, cm.responseFormat, newCM.responseFormat)
	})
}

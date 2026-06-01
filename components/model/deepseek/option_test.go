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
	"testing"

	"github.com/bytedance/mockey"
	"github.com/cohesion-org/deepseek-go"
	"github.com/stretchr/testify/assert"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func TestWithExtraFields(t *testing.T) {
	t.Run("option merges into specific options", func(t *testing.T) {
		extra := map[string]interface{}{
			"chat_template_kwargs": map[string]interface{}{
				"thinking": true,
			},
			"custom_flag": "value",
		}

		specOpts := model.GetImplSpecificOptions(&deepseekOptions{}, WithExtraFields(extra))
		assert.Equal(t, extra, specOpts.extraFields)
	})

	t.Run("nil extra fields is a no-op", func(t *testing.T) {
		specOpts := model.GetImplSpecificOptions(&deepseekOptions{}, WithExtraFields(nil))
		assert.Nil(t, specOpts.extraFields)
	})

	t.Run("generate forwards extra fields into request", func(t *testing.T) {
		var capturedReq *deepseek.ChatCompletionRequest
		defer mockey.Mock((*deepseek.Client).CreateChatCompletion).To(func(ctx context.Context, request *deepseek.ChatCompletionRequest) (*deepseek.ChatCompletionResponse, error) {
			capturedReq = request
			return &deepseek.ChatCompletionResponse{
				Choices: []deepseek.Choice{
					{Index: 0, Message: deepseek.Message{Role: "assistant", Content: "ok"}},
				},
			}, nil
		}).Build().UnPatch()

		ctx := context.Background()
		cm, err := NewChatModel(ctx, &ChatModelConfig{
			APIKey: "test-key",
			Model:  "deepseek-chat",
		})
		assert.Nil(t, err)

		extra := map[string]interface{}{
			"chat_template_kwargs": map[string]interface{}{
				"thinking": true,
			},
		}

		_, err = cm.Generate(ctx, []*schema.Message{schema.UserMessage("hello")}, WithExtraFields(extra))
		assert.Nil(t, err)
		assert.NotNil(t, capturedReq)
		assert.Equal(t, extra, capturedReq.ExtraFields)
	})

	t.Run("stream forwards extra fields into stream request", func(t *testing.T) {
		var capturedReq *deepseek.StreamChatCompletionRequest
		defer mockey.Mock((*deepseek.Client).CreateChatCompletionStream).To(func(ctx context.Context, request *deepseek.StreamChatCompletionRequest) (deepseek.ChatCompletionStream, error) {
			capturedReq = request
			return &mockStream{responses: nil, idx: 0}, nil
		}).Build().UnPatch()

		ctx := context.Background()
		cm, err := NewChatModel(ctx, &ChatModelConfig{
			APIKey: "test-key",
			Model:  "deepseek-chat",
		})
		assert.Nil(t, err)

		extra := map[string]interface{}{
			"top_k": 10,
		}

		_, err = cm.Stream(ctx, []*schema.Message{schema.UserMessage("hello")}, WithExtraFields(extra))
		assert.Nil(t, err)
		assert.NotNil(t, capturedReq)
		assert.Equal(t, extra, capturedReq.ExtraFields)
	})

	t.Run("without option leaves extra fields empty", func(t *testing.T) {
		var capturedReq *deepseek.ChatCompletionRequest
		defer mockey.Mock((*deepseek.Client).CreateChatCompletion).To(func(ctx context.Context, request *deepseek.ChatCompletionRequest) (*deepseek.ChatCompletionResponse, error) {
			capturedReq = request
			return &deepseek.ChatCompletionResponse{
				Choices: []deepseek.Choice{
					{Index: 0, Message: deepseek.Message{Role: "assistant", Content: "ok"}},
				},
			}, nil
		}).Build().UnPatch()

		ctx := context.Background()
		cm, err := NewChatModel(ctx, &ChatModelConfig{
			APIKey: "test-key",
			Model:  "deepseek-chat",
		})
		assert.Nil(t, err)

		_, err = cm.Generate(ctx, []*schema.Message{schema.UserMessage("hello")})
		assert.Nil(t, err)
		assert.NotNil(t, capturedReq)
		assert.Nil(t, capturedReq.ExtraFields)
	})
}

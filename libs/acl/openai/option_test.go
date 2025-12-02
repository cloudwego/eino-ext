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

package openai

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	goopenai "github.com/meguminnnnnnnnn/go-openai"
	"github.com/stretchr/testify/assert"
)

func Test_OpenAIOptions_Setters(t *testing.T) {
	t.Run("WithExtraFields", func(t *testing.T) {
		fields := map[string]any{"a": 1, "b": "x"}
		opts := []model.Option{WithExtraFields(fields)}
		spec := model.GetImplSpecificOptions(&openaiOptions{}, opts...)
		assert.Equal(t, fields, spec.ExtraFields)
	})

	t.Run("WithReasoningEffort", func(t *testing.T) {
		opts := []model.Option{WithReasoningEffort(ReasoningEffortLevelHigh)}
		spec := model.GetImplSpecificOptions(&openaiOptions{}, opts...)
		assert.Equal(t, ReasoningEffortLevelHigh, spec.ReasoningEffort)
	})

	t.Run("WithRequestPayloadModifier", func(t *testing.T) {
		mod := func(ctx context.Context, msgs []*schema.Message, rawBody []byte) ([]byte, error) {
			return append(rawBody, 'x'), nil
		}
		opts := []model.Option{WithRequestPayloadModifier(mod)}
		spec := model.GetImplSpecificOptions(&openaiOptions{}, opts...)
		if assert.NotNil(t, spec.RequestPayloadModifier) {
			out, err := spec.RequestPayloadModifier(t.Context(), []*schema.Message{{Role: schema.User}}, []byte("body"))
			assert.NoError(t, err)
			assert.Equal(t, []byte("bodyx"), out)
		}
	})

	t.Run("WithResponseMessageModifier", func(t *testing.T) {
		mod := func(ctx context.Context, msg *schema.Message, rawBody []byte) (*schema.Message, error) {
			return &schema.Message{Role: msg.Role, Content: msg.Content + "|m"}, nil
		}
		opts := []model.Option{WithResponseMessageModifier(mod)}
		spec := model.GetImplSpecificOptions(&openaiOptions{}, opts...)
		if assert.NotNil(t, spec.ResponseMessageModifier) {
			outMsg, err := spec.ResponseMessageModifier(t.Context(), &schema.Message{Role: schema.Assistant, Content: "resp"}, []byte("raw"))
			assert.NoError(t, err)
			assert.Equal(t, "resp|m", outMsg.Content)
			assert.Equal(t, schema.Assistant, outMsg.Role)
		}
	})

	t.Run("WithResponseChunkMessageModifier", func(t *testing.T) {
		mod := func(ctx context.Context, msg *schema.Message, rawBody []byte, end bool) (*schema.Message, error) {
			if end {
				return msg, nil
			}
			return &schema.Message{Role: msg.Role, Content: msg.Content + "|chunk"}, nil
		}
		opts := []model.Option{WithResponseChunkMessageModifier(mod)}
		spec := model.GetImplSpecificOptions(&openaiOptions{}, opts...)
		if assert.NotNil(t, spec.ResponseChunkMessageModifier) {
			outMsg, err := spec.ResponseChunkMessageModifier(t.Context(), &schema.Message{Role: schema.Assistant, Content: "resp"}, []byte("raw"), false)
			assert.NoError(t, err)
			assert.Equal(t, "resp|chunk", outMsg.Content)
			outMsg2, err2 := spec.ResponseChunkMessageModifier(t.Context(), &schema.Message{Role: schema.Assistant, Content: "resp"}, []byte("raw"), true)
			assert.NoError(t, err2)
			assert.Equal(t, "resp", outMsg2.Content)
		}
	})

	t.Run("WithRequestBodyModifier (deprecated)", func(t *testing.T) {
		mod := func(rawBody []byte) ([]byte, error) { return append(rawBody, 'y'), nil }
		opts := []model.Option{WithRequestBodyModifier(goopenai.RequestBodyModifier(mod))}
		spec := model.GetImplSpecificOptions(&openaiOptions{}, opts...)
		if assert.NotNil(t, spec.RequestBodyModifier) {
			out, err := spec.RequestBodyModifier([]byte("body"))
			assert.NoError(t, err)
			assert.Equal(t, []byte("bodyy"), out)
		}
	})

	t.Run("WithExtraHeader", func(t *testing.T) {
		hdr := map[string]string{"x": "1"}
		opts := []model.Option{WithExtraHeader(hdr)}
		spec := model.GetImplSpecificOptions(&openaiOptions{}, opts...)
		assert.Equal(t, hdr, spec.ExtraHeader)
	})

	t.Run("WithMaxCompletionTokens", func(t *testing.T) {
		opts := []model.Option{WithMaxCompletionTokens(123)}
		spec := model.GetImplSpecificOptions(&openaiOptions{}, opts...)
		if assert.NotNil(t, spec.MaxCompletionTokens) {
			assert.Equal(t, 123, *spec.MaxCompletionTokens)
		}
	})
}

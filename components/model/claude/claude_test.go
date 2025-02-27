/*
 * Copyright 2024 CloudWeGo Authors
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

package claude

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/bytedance/mockey"
	"github.com/cloudwego/eino/schema"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
)

func TestClaude(t *testing.T) {
	ctx := context.Background()
	model, err := NewChatModel(ctx, &Config{
		APIKey: "test-key",
		Model:  "claude-3-opus-20240229",
	})
	assert.NoError(t, err)

	mockey.PatchConvey("basic chat", t, func() {
		// Mock API response
		defer mockey.Mock((*anthropic.MessageService).New).Return(&anthropic.Message{
			Content: []anthropic.ContentBlock{
				{
					Type: anthropic.ContentBlockTypeText,
					Text: "Hello, I'm Claude!",
				},
			},
			Usage: anthropic.Usage{
				InputTokens:  10,
				OutputTokens: 5,
			},
		}, nil).Build().UnPatch()

		resp, err := model.Generate(ctx, []*schema.Message{
			{
				Role:    schema.User,
				Content: "Hi, who are you?",
			},
		}, WithTopK(5))

		assert.NoError(t, err)
		assert.Equal(t, "Hello, I'm Claude!", resp.Content)
		assert.Equal(t, schema.Assistant, resp.Role)
		assert.Equal(t, 10, resp.ResponseMeta.Usage.PromptTokens)
		assert.Equal(t, 5, resp.ResponseMeta.Usage.CompletionTokens)
	})

	mockey.PatchConvey("function calling", t, func() {
		// Bind tool
		err := model.BindTools([]*schema.ToolInfo{
			{
				Name: "get_weather",
				Desc: "Get weather information",
				ParamsOneOf: schema.NewParamsOneOfByOpenAPIV3(&openapi3.Schema{
					Type: "object",
					Properties: map[string]*openapi3.SchemaRef{
						"city": {
							Value: &openapi3.Schema{
								Type: "string",
							},
						},
					},
				}),
			},
		})
		assert.NoError(t, err)

		// Mock function call response
		defer mockey.Mock((*anthropic.MessageService).New).Return(&anthropic.Message{
			Content: []anthropic.ContentBlock{
				{
					Type:  anthropic.ContentBlockTypeToolUse,
					ID:    "call_1",
					Name:  "get_weather",
					Input: []byte(`{"city":"Paris"}`),
				},
			},
		}, nil).Build().UnPatch()

		resp, err := model.Generate(ctx, []*schema.Message{
			{
				Role:    schema.User,
				Content: "What's the weather in Paris?",
			},
		})

		assert.NoError(t, err)
		assert.Len(t, resp.ToolCalls, 1)
		assert.Equal(t, "get_weather", resp.ToolCalls[0].Function.Name)
		assert.Equal(t, `{"city":"Paris"}`, resp.ToolCalls[0].Function.Arguments)
	})

	mockey.PatchConvey("image processing", t, func() {
		// Mock image response
		defer mockey.Mock((*anthropic.MessageService).New).Return(&anthropic.Message{
			Content: []anthropic.ContentBlock{
				{
					Type: anthropic.ContentBlockTypeText,
					Text: "I see a beautiful sunset image",
				},
			},
		}, nil).Build().UnPatch()

		resp, err := model.Generate(ctx, []*schema.Message{
			{
				Role: schema.User,
				MultiContent: []schema.ChatMessagePart{
					{
						Type: schema.ChatMessagePartTypeText,
						Text: "What's in this image?",
					},
					{
						Type: schema.ChatMessagePartTypeImageURL,
						ImageURL: &schema.ChatMessageImageURL{
							URL:      "data:image/jpeg;base64,/9j/4AAQSkZ...",
							MIMEType: "image/jpeg",
						},
					},
				},
			},
		})

		assert.NoError(t, err)
		assert.Equal(t, "I see a beautiful sunset image", resp.Content)
	})
}

func TestConvStreamEvent(t *testing.T) {
	streamCtx := &streamContext{}

	mockey.PatchConvey("message start event", t, func() {
		event := anthropic.MessageStreamEvent{}
		defer mockey.Mock(anthropic.MessageStreamEvent.AsUnion).Return(anthropic.MessageStartEvent{
			Message: anthropic.Message{
				Content: []anthropic.ContentBlock{
					{
						Type: anthropic.ContentBlockTypeText,
						Text: "Initial message",
					},
				},
				Usage: anthropic.Usage{
					InputTokens:  5,
					OutputTokens: 2,
				},
			},
		}).Build().UnPatch()

		message, err := convStreamEvent(event, streamCtx)
		assert.NoError(t, err)
		assert.Equal(t, "Initial message", message.Content)
		assert.Equal(t, schema.Assistant, message.Role)
		assert.Equal(t, 5, message.ResponseMeta.Usage.PromptTokens)
		assert.Equal(t, 2, message.ResponseMeta.Usage.CompletionTokens)
	})

	mockey.PatchConvey("content block delta event - text", t, func() {
		event := anthropic.MessageStreamEvent{}
		delta := anthropic.ContentBlockDeltaEventDelta{}
		defer mockey.Mock(anthropic.ContentBlockDeltaEventDelta.AsUnion).Return(anthropic.TextDelta{
			Text: " world",
		}).Build().UnPatch()
		defer mockey.Mock(anthropic.MessageStreamEvent.AsUnion).Return(anthropic.ContentBlockDeltaEvent{
			Delta: delta,
			Index: 0,
			Type:  "",
		}).Build().UnPatch()

		message, err := convStreamEvent(event, streamCtx)
		assert.NoError(t, err)
		assert.Equal(t, " world", message.Content)
	})

	mockey.PatchConvey("content block delta event - tool input", t, func() {
		streamCtx.toolIndex = new(int)
		*streamCtx.toolIndex = 0

		event := anthropic.MessageStreamEvent{}
		delta := anthropic.ContentBlockDeltaEventDelta{}
		defer mockey.Mock(anthropic.ContentBlockDeltaEventDelta.AsUnion).Return(anthropic.InputJSONDelta{
			PartialJSON: `,"temp":25`,
		}).Build().UnPatch()
		defer mockey.Mock(anthropic.MessageStreamEvent.AsUnion).Return(anthropic.ContentBlockDeltaEvent{
			Delta: delta,
			Index: 0,
			Type:  "",
		}).Build().UnPatch()

		message, err := convStreamEvent(event, streamCtx)
		assert.NoError(t, err)
		assert.Len(t, message.ToolCalls, 1)
		assert.Equal(t, 0, *message.ToolCalls[0].Index)
		assert.Equal(t, `,"temp":25`, message.ToolCalls[0].Function.Arguments)
	})

	mockey.PatchConvey("message delta event", t, func() {
		event := anthropic.MessageStreamEvent{}
		defer mockey.Mock(anthropic.MessageStreamEvent.AsUnion).Return(anthropic.MessageDeltaEvent{
			Delta: anthropic.MessageDeltaEventDelta{
				StopReason: "end_turn",
			},
			Usage: anthropic.MessageDeltaUsage{
				OutputTokens: 10,
			},
		}).Build().UnPatch()

		message, err := convStreamEvent(event, streamCtx)
		assert.NoError(t, err)
		assert.Equal(t, "end_turn", message.ResponseMeta.FinishReason)
		assert.Equal(t, 10, message.ResponseMeta.Usage.CompletionTokens)
	})

	mockey.PatchConvey("content block start event", t, func() {
		event := anthropic.MessageStreamEvent{}
		defer mockey.Mock(anthropic.MessageStreamEvent.AsUnion).Return(anthropic.ContentBlockStartEvent{}).Build().UnPatch()
		defer mockey.Mock((*anthropic.ContentBlock).UnmarshalJSON).When(func(r *anthropic.ContentBlock, data []byte) bool {
			r.Type = anthropic.ContentBlockTypeToolUse
			r.Name = "tool"
			r.Input = json.RawMessage("")
			return true
		}).Return(nil).Build().UnPatch()

		message, err := convStreamEvent(event, streamCtx)
		assert.NoError(t, err)
		assert.Equal(t, len(message.ToolCalls), 1)
		assert.Equal(t, *message.ToolCalls[0].Index, 1)
		assert.Equal(t, message.ToolCalls[0].Function.Name, "tool")
		assert.Equal(t, message.ToolCalls[0].Function.Arguments, "")
	})
}

func TestPanicErr(t *testing.T) {
	err := newPanicErr("info", []byte("stack"))
	assert.Equal(t, "panic error: info, \nstack: stack", err.Error())
}

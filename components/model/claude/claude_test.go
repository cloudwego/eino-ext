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
	"github.com/anthropics/anthropic-sdk-go/shared/constant"
	"github.com/bytedance/mockey"
	"github.com/eino-contrib/jsonschema"
	"github.com/stretchr/testify/assert"
	orderedmap "github.com/wk8/go-ordered-map/v2"

	"github.com/cloudwego/eino/schema"
)

func TestClaude(t *testing.T) {
	ctx := context.Background()
	model, err := NewChatModel(ctx, &Config{
		APIKey: "test-key",
		Model:  "claude-3-opus-20240229",
	})
	assert.NoError(t, err)

	mockey.PatchConvey("requires at least 1 user msg", t, func() {
		_, err := model.genMessageNewParams([]*schema.Message{
			schema.SystemMessage("hello"),
		})
		assert.Error(t, err)
		assert.ErrorContains(t, err, "only system message in input, require at least 1 user message")
	})

	mockey.PatchConvey("first non system msg should be user", t, func() {
		_, err := model.genMessageNewParams([]*schema.Message{
			schema.SystemMessage("hello"),
			schema.AssistantMessage("world", nil),
		})
		assert.Error(t, err)
		assert.ErrorContains(t, err, "first non-system message should be user message")
	})

	mockey.PatchConvey("multiple system msg", t, func() {
		resp, err := model.genMessageNewParams([]*schema.Message{
			schema.SystemMessage("hello"),
			schema.SystemMessage("world"),
			schema.UserMessage("again"),
		})
		assert.NoError(t, err)
		assert.Equal(t, anthropic.MessageNewParams{
			Model: "claude-3-opus-20240229",
			System: []anthropic.TextBlockParam{
				{
					Text: "hello",
				},
				{
					Text: "world",
				},
			},
			Messages: []anthropic.MessageParam{
				{
					Content: []anthropic.ContentBlockParamUnion{
						{
							OfText: &anthropic.TextBlockParam{
								Text: "again",
							},
						},
					},
					Role: anthropic.MessageParamRoleUser,
				},
			},
		}, resp)
	})

	mockey.PatchConvey("basic chat", t, func() {
		// Mock API response
		content := anthropic.ContentBlockUnion{
			Type: "text",
			Text: "Hello, I'm Claude!",
		}
		defer mockey.Mock(anthropic.ContentBlockUnion.AsAny).Return(anthropic.TextBlock{
			Type: constant.Text(content.Type),
			Text: content.Text,
		}).Build().UnPatch()
		defer mockey.Mock((*anthropic.MessageService).New).Return(&anthropic.Message{
			Content: []anthropic.ContentBlockUnion{
				content,
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
				ParamsOneOf: schema.NewParamsOneOfByJSONSchema(&jsonschema.Schema{
					Type: "object",
					Properties: orderedmap.New[string, *jsonschema.Schema](orderedmap.WithInitialData[string, *jsonschema.Schema](
						orderedmap.Pair[string, *jsonschema.Schema]{
							Key: "city",
							Value: &jsonschema.Schema{
								Type: "string",
							},
						}),
					),
				}),
			},
		})
		assert.NoError(t, err)

		content := anthropic.ContentBlockUnion{
			Type:  "tool_use",
			ID:    "call_1",
			Name:  "get_weather",
			Input: []byte(`{"city":"Paris"}`),
		}
		defer mockey.Mock(anthropic.ContentBlockUnion.AsAny).Return(anthropic.ToolUseBlock{
			Type:  constant.ToolUse(content.Type),
			ID:    content.ID,
			Name:  content.Name,
			Input: content.Input,
		}).Build().UnPatch()
		// Mock function call response
		defer mockey.Mock((*anthropic.MessageService).New).Return(&anthropic.Message{
			Content: []anthropic.ContentBlockUnion{
				content,
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
		content := anthropic.ContentBlockUnion{
			Type: "text",
			Text: "I see a beautiful sunset image",
		}
		defer mockey.Mock(anthropic.ContentBlockUnion.AsAny).Return(anthropic.TextBlock{
			Type: constant.Text(content.Text),
			Text: content.Text,
		}).Build().UnPatch()
		defer mockey.Mock((*anthropic.MessageService).New).Return(&anthropic.Message{
			Content: []anthropic.ContentBlockUnion{
				content,
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
		event := anthropic.MessageStreamEventUnion{}
		content := anthropic.ContentBlockUnion{
			Type: "text",
			Text: "Initial message",
		}
		defer mockey.Mock(anthropic.ContentBlockUnion.AsAny).Return(anthropic.TextBlock{
			Type: constant.Text(content.Type),
			Text: content.Text,
		}).Build().UnPatch()

		defer mockey.Mock(anthropic.MessageStreamEventUnion.AsAny).Return(anthropic.MessageStartEvent{
			Message: anthropic.Message{
				Content: []anthropic.ContentBlockUnion{
					content,
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
		event := anthropic.MessageStreamEventUnion{}
		delta := anthropic.RawContentBlockDeltaUnion{
			Text: " world",
		}
		defer mockey.Mock(anthropic.RawContentBlockDeltaUnion.AsAny).Return(anthropic.TextDelta{
			Text: delta.Text,
		}).Build().UnPatch()

		defer mockey.Mock(anthropic.MessageStreamEventUnion.AsAny).Return(anthropic.ContentBlockDeltaEvent{
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

		event := anthropic.MessageStreamEventUnion{}
		delta := anthropic.RawContentBlockDeltaUnion{}
		defer mockey.Mock(anthropic.RawContentBlockDeltaUnion.AsAny).Return(anthropic.InputJSONDelta{
			PartialJSON: `,"temp":25`,
		}).Build().UnPatch()
		defer mockey.Mock(anthropic.MessageStreamEventUnion.AsAny).Return(anthropic.ContentBlockDeltaEvent{
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
		event := anthropic.MessageStreamEventUnion{}
		defer mockey.Mock(anthropic.MessageStreamEventUnion.AsAny).Return(anthropic.MessageDeltaEvent{
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
		event := anthropic.MessageStreamEventUnion{}
		defer mockey.Mock(anthropic.MessageStreamEventUnion.AsAny).
			Return(anthropic.ContentBlockStartEvent{}).Build().UnPatch()
		defer mockey.Mock(anthropic.ContentBlockStartEventContentBlockUnion.AsAny).
			Return(anthropic.ToolUseBlock{
				Type:  "tool_use",
				Name:  "tool",
				Input: json.RawMessage("xxx"),
			}).Build().UnPatch()

		message, err := convStreamEvent(event, streamCtx)
		assert.NoError(t, err)
		assert.Equal(t, len(message.ToolCalls), 1)
		assert.Equal(t, *message.ToolCalls[0].Index, 1)
		assert.Equal(t, message.ToolCalls[0].Function.Name, "tool")
		assert.Equal(t, message.ToolCalls[0].Function.Arguments, "xxx")
	})
}

func TestPanicErr(t *testing.T) {
	err := newPanicErr("info", []byte("stack"))
	assert.Equal(t, "panic error: info, \nstack: stack", err.Error())
}

func TestWithTools(t *testing.T) {
	cm := &ChatModel{model: "test model"}
	ncm, err := cm.WithTools([]*schema.ToolInfo{{Name: "test tool name"}})
	assert.Nil(t, err)
	assert.Equal(t, "test model", ncm.(*ChatModel).model)
	assert.Equal(t, "test tool name", ncm.(*ChatModel).origTools[0].Name)
}

func TestPopulateContentBlockBreakPoint(t *testing.T) {
	block := anthropic.NewTextBlock("input")
	populateContentBlockBreakPoint(block)
	assert.NotEmpty(t, block.OfText.CacheControl.Type)

	block = anthropic.NewImageBlock[anthropic.URLImageSourceParam](anthropic.URLImageSourceParam{})
	populateContentBlockBreakPoint(block)
	assert.NotEmpty(t, block.OfImage.CacheControl.Type)

	block = anthropic.NewToolResultBlock("userID", "input", false)
	populateContentBlockBreakPoint(block)
	assert.NotEmpty(t, block.OfToolResult.CacheControl.Type)

	block = anthropic.NewToolUseBlock("123", "input", "test_tool")
	populateContentBlockBreakPoint(block)
	assert.NotEmpty(t, block.OfToolUse.CacheControl.Type)
}

func Test_convSchemaMessage_MultiContent(t *testing.T) {
	rawBase64 := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII="
	invalidDataURL := "data:image/png;base64," + rawBase64
	httpURL := "https://example.com/image.png"

	t.Run("UserInputMultiContent", func(t *testing.T) {
		t.Run("success with base64", func(t *testing.T) {
			msg := &schema.Message{
				Role: schema.User,
				UserInputMultiContent: []schema.MessageInputPart{
					{Type: schema.ChatMessagePartTypeText, Text: "hello"},
					{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{Base64Data: &rawBase64, MIMEType: "image/png"}}},
				},
			}
			result, err := convSchemaMessage(msg)
			assert.NoError(t, err)
			assert.Len(t, result.Content, 2)
			assert.Equal(t, "hello", result.Content[0].OfText.Text)
			assert.Equal(t, anthropic.Base64ImageSourceMediaType("image/png"), result.Content[1].OfImage.Source.OfBase64.MediaType)
		})

		t.Run("success with url", func(t *testing.T) {
			msg := &schema.Message{
				Role: schema.User,
				UserInputMultiContent: []schema.MessageInputPart{
					{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{URL: &httpURL}}},
				},
			}
			result, err := convSchemaMessage(msg)
			assert.NoError(t, err)
			assert.Len(t, result.Content, 1)
			assert.Equal(t, httpURL, result.Content[0].OfImage.Source.OfURL.URL)
		})

		t.Run("error with data url prefix", func(t *testing.T) {
			msg := &schema.Message{
				Role: schema.User,
				UserInputMultiContent: []schema.MessageInputPart{
					{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{Base64Data: &invalidDataURL, MIMEType: "image/png"}}},
				},
			}
			_, err := convSchemaMessage(msg)
			assert.Error(t, err)
			assert.ErrorContains(t, err, "Base64Data should be a raw base64 string")
		})

		t.Run("error with no mime type for base64", func(t *testing.T) {
			msg := &schema.Message{
				Role: schema.User,
				UserInputMultiContent: []schema.MessageInputPart{
					{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{Base64Data: &rawBase64}}},
				},
			}
			_, err := convSchemaMessage(msg)
			assert.Error(t, err)
			assert.ErrorContains(t, err, "image part must have MIMEType when use Base64Data")
		})

		t.Run("error with no url or base64", func(t *testing.T) {
			msg := &schema.Message{
				Role: schema.User,
				UserInputMultiContent: []schema.MessageInputPart{
					{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{}},
				},
			}
			_, err := convSchemaMessage(msg)
			assert.Error(t, err)
			assert.ErrorContains(t, err, "image part must have either a URL or Base64Data")
		})
	})

	t.Run("AssistantGenMultiContent", func(t *testing.T) {
		t.Run("success with image", func(t *testing.T) {
			msg := &schema.Message{
				Role: schema.Assistant,
				AssistantGenMultiContent: []schema.MessageOutputPart{
					{
						Type: schema.ChatMessagePartTypeImageURL,
						Image: &schema.MessageOutputImage{
							MessagePartCommon: schema.MessagePartCommon{
								Base64Data: &rawBase64,
								MIMEType:   "image/png",
							},
						},
					},
					{Type: schema.ChatMessagePartTypeText, Text: "some text"},
				},
			}
			result, err := convSchemaMessage(msg)
			assert.NoError(t, err)
			assert.Len(t, result.Content, 2)
			assert.Equal(t, anthropic.Base64ImageSourceMediaType("image/png"), result.Content[0].OfImage.Source.OfBase64.MediaType)
			assert.Equal(t, "some text", result.Content[1].OfText.Text)
		})

		t.Run("success with url", func(t *testing.T) {
			msg := &schema.Message{
				Role: schema.Assistant,
				AssistantGenMultiContent: []schema.MessageOutputPart{
					{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageOutputImage{MessagePartCommon: schema.MessagePartCommon{URL: &httpURL}}},
				},
			}
			result, err := convSchemaMessage(msg)
			assert.NoError(t, err)
			assert.Len(t, result.Content, 1)
			assert.Equal(t, httpURL, result.Content[0].OfImage.Source.OfURL.URL)
		})

		t.Run("error with wrong role", func(t *testing.T) {
			msg := &schema.Message{
				Role: schema.User,
				AssistantGenMultiContent: []schema.MessageOutputPart{
					{Type: schema.ChatMessagePartTypeText, Text: "some text"},
				},
			}
			_, err := convSchemaMessage(msg)
			assert.Error(t, err)
			assert.ErrorContains(t, err, "assistant gen multi content only support assistant role")
		})

		t.Run("error with data url prefix", func(t *testing.T) {
			msg := &schema.Message{
				Role: schema.Assistant,
				AssistantGenMultiContent: []schema.MessageOutputPart{
					{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageOutputImage{MessagePartCommon: schema.MessagePartCommon{Base64Data: &invalidDataURL, MIMEType: "image/png"}}},
				},
			}
			_, err := convSchemaMessage(msg)
			assert.Error(t, err)
			assert.ErrorContains(t, err, "Base64Data should be a raw base64 string")
		})

		t.Run("error with no mime type for base64", func(t *testing.T) {
			msg := &schema.Message{
				Role: schema.Assistant,
				AssistantGenMultiContent: []schema.MessageOutputPart{
					{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageOutputImage{MessagePartCommon: schema.MessagePartCommon{Base64Data: &rawBase64}}},
				},
			}
			_, err := convSchemaMessage(msg)
			assert.Error(t, err)
			assert.ErrorContains(t, err, "image part must have MIMEType when use Base64Data")
		})

		t.Run("error with no url or base64", func(t *testing.T) {
			msg := &schema.Message{
				Role: schema.Assistant,
				AssistantGenMultiContent: []schema.MessageOutputPart{
					{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageOutputImage{}},
				},
			}
			_, err := convSchemaMessage(msg)
			assert.Error(t, err)
			assert.ErrorContains(t, err, "image part must have either a URL or Base64Data")
		})
	})

	t.Run("MultiContent backward compatibility", func(t *testing.T) {
		msg := &schema.Message{
			Role: schema.User,
			MultiContent: []schema.ChatMessagePart{
				{Type: schema.ChatMessagePartTypeText, Text: "legacy"},
				{Type: schema.ChatMessagePartTypeImageURL, ImageURL: &schema.ChatMessageImageURL{URL: invalidDataURL}},
			},
		}
		result, err := convSchemaMessage(msg)
		assert.NoError(t, err)
		assert.Len(t, result.Content, 2)
		assert.Equal(t, "legacy", result.Content[0].OfText.Text)
		assert.Equal(t, anthropic.Base64ImageSourceMediaType("image/png"), result.Content[1].OfImage.Source.OfBase64.MediaType)
	})

	t.Run("MultiContent backward compatibility with http url", func(t *testing.T) {
		msg := &schema.Message{
			Role: schema.User,
			MultiContent: []schema.ChatMessagePart{
				{Type: schema.ChatMessagePartTypeImageURL, ImageURL: &schema.ChatMessageImageURL{URL: httpURL}},
			},
		}
		result, err := convSchemaMessage(msg)
		assert.NoError(t, err)
		assert.Len(t, result.Content, 1)
		assert.Equal(t, httpURL, result.Content[0].OfImage.Source.OfURL.URL)
	})

	t.Run("error with both UserInputMultiContent and AssistantGenMultiContent", func(t *testing.T) {
		msg := &schema.Message{
			Role:                     schema.User,
			UserInputMultiContent:    []schema.MessageInputPart{{Type: schema.ChatMessagePartTypeText, Text: "user"}},
			AssistantGenMultiContent: []schema.MessageOutputPart{{Type: schema.ChatMessagePartTypeText, Text: "assistant"}},
		}
		_, err := convSchemaMessage(msg)
		assert.Error(t, err)
		assert.ErrorContains(t, err, "a message cannot contain both UserInputMultiContent and AssistantGenMultiContent")
	})

	t.Run("error with nil image in UserInputMultiContent", func(t *testing.T) {
		msg := &schema.Message{
			Role: schema.User,
			UserInputMultiContent: []schema.MessageInputPart{
				{Type: schema.ChatMessagePartTypeImageURL, Image: nil},
			},
		}
		_, err := convSchemaMessage(msg)
		assert.Error(t, err)
		assert.ErrorContains(t, err, "image field must not be nil")
	})

	t.Run("error with nil image in AssistantGenMultiContent", func(t *testing.T) {
		msg := &schema.Message{
			Role: schema.Assistant,
			AssistantGenMultiContent: []schema.MessageOutputPart{
				{Type: schema.ChatMessagePartTypeImageURL, Image: nil},
			},
		}
		_, err := convSchemaMessage(msg)
		assert.Error(t, err)
		assert.ErrorContains(t, err, "image field must not be nil")
	})
}

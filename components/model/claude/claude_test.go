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
	"os"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	"github.com/anthropics/anthropic-sdk-go/shared/constant"
	"github.com/anthropics/anthropic-sdk-go/vertex"
	"github.com/bytedance/mockey"
	"github.com/cloudwego/eino/components/model"
	"github.com/eino-contrib/jsonschema"
	"github.com/stretchr/testify/assert"
	orderedmap "github.com/wk8/go-ordered-map/v2"

	"github.com/cloudwego/eino/schema"
	"golang.org/x/oauth2/google"
)

func TestDirectAnthropicAuthSelection(t *testing.T) {
	t.Run("config auth exists", func(t *testing.T) {
		clearAnthropicAuthEnv(t)
		assert.True(t, hasDirectAnthropicConfigAuth(&Config{APIKey: "api-key"}))
		assert.True(t, hasDirectAnthropicConfigAuth(&Config{AuthToken: "auth-token"}))
		assert.True(t, hasDirectAnthropicConfigAuth(&Config{APIKey: "api-key", AuthToken: "auth-token"}))
	})

	t.Run("env auth exists", func(t *testing.T) {
		clearAnthropicAuthEnv(t)
		t.Setenv("ANTHROPIC_API_KEY", "env-api-key")
		model, err := NewChatModel(context.Background(), &Config{
			Model: "claude-3-opus-20240229",
		})
		assert.NoError(t, err)
		assert.NotNil(t, model)
	})

	t.Run("missing auth still allows creation", func(t *testing.T) {
		clearAnthropicAuthEnv(t)
		model, err := NewChatModel(context.Background(), &Config{
			Model: "claude-3-opus-20240229",
		})
		assert.NoError(t, err)
		assert.NotNil(t, model)
	})
}

func TestClaude(t *testing.T) {
	clearAnthropicAuthEnv(t)

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

func clearAnthropicAuthEnv(t *testing.T) {
	t.Helper()
	// Use os.Unsetenv to truly remove the env vars, since the SDK uses
	// os.LookupEnv — an empty string is still "present".
	for _, key := range []string{"ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN"} {
		prev, existed := os.LookupEnv(key)
		os.Unsetenv(key)
		if existed {
			t.Cleanup(func() { os.Setenv(key, prev) })
		} else {
			t.Cleanup(func() { os.Unsetenv(key) })
		}
	}
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
	populateContentBlockBreakPoint(block, nil)
	assert.NotEmpty(t, block.OfText.CacheControl.Type)

	block = anthropic.NewImageBlock[anthropic.URLImageSourceParam](anthropic.URLImageSourceParam{})
	populateContentBlockBreakPoint(block, nil)
	assert.NotEmpty(t, block.OfImage.CacheControl.Type)

	block = anthropic.NewToolResultBlock("userID", "input", false)
	populateContentBlockBreakPoint(block, nil)
	assert.NotEmpty(t, block.OfToolResult.CacheControl.Type)

	block = anthropic.NewToolUseBlock("123", "input", "test_tool")
	populateContentBlockBreakPoint(block, nil)
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

func TestPopulateToolChoice(t *testing.T) {
	t.Run("nil tool choice", func(t *testing.T) {
		params := &anthropic.MessageNewParams{}
		options := &model.Options{}
		err := populateToolChoice(params, options.ToolChoice, options.AllowedToolNames, nil)
		assert.NoError(t, err)

	})

	t.Run("tool choice forbidden", func(t *testing.T) {
		params := &anthropic.MessageNewParams{}
		toolChoice := schema.ToolChoiceForbidden
		options := &model.Options{
			ToolChoice: &toolChoice,
		}
		err := populateToolChoice(params, options.ToolChoice, options.AllowedToolNames, nil)
		assert.NoError(t, err)
		assert.NotNil(t, params.ToolChoice)
		assert.NotNil(t, params.ToolChoice.OfNone)
	})

	t.Run("tool choice allowed", func(t *testing.T) {
		params := &anthropic.MessageNewParams{}
		toolChoice := schema.ToolChoiceAllowed
		options := &model.Options{
			ToolChoice: &toolChoice,
		}
		err := populateToolChoice(params, options.ToolChoice, options.AllowedToolNames, nil)
		assert.NoError(t, err)
		assert.NotNil(t, params.ToolChoice)
		assert.NotNil(t, params.ToolChoice.OfAuto)
	})

	t.Run("tool choice allowed with disable parallel tool use", func(t *testing.T) {
		params := &anthropic.MessageNewParams{}
		toolChoice := schema.ToolChoiceAllowed
		options := &model.Options{
			ToolChoice: &toolChoice,
		}
		disableParallelToolUse := true
		err := populateToolChoice(params, options.ToolChoice, options.AllowedToolNames, &disableParallelToolUse)
		assert.NoError(t, err)
		assert.NotNil(t, params.ToolChoice)
		assert.NotNil(t, params.ToolChoice.OfAuto)
		assert.Equal(t, param.NewOpt(true), params.ToolChoice.OfAuto.DisableParallelToolUse)
	})

	t.Run("tool choice forced with no tools", func(t *testing.T) {
		params := &anthropic.MessageNewParams{}
		toolChoice := schema.ToolChoiceForced
		options := &model.Options{
			ToolChoice: &toolChoice,
		}
		err := populateToolChoice(params, options.ToolChoice, options.AllowedToolNames, nil)
		assert.Error(t, err)
		assert.Equal(t, "tool choice is forced but tool is not provided", err.Error())
	})

	t.Run("tool choice forced with one tool", func(t *testing.T) {
		params := &anthropic.MessageNewParams{
			Tools: []anthropic.ToolUnionParam{
				{
					OfTool: &anthropic.ToolParam{
						Name: "test_tool",
					},
				},
			},
		}
		toolChoice := schema.ToolChoiceForced
		options := &model.Options{
			ToolChoice: &toolChoice,
		}
		err := populateToolChoice(params, options.ToolChoice, options.AllowedToolNames, nil)
		assert.NoError(t, err)
		assert.NotNil(t, params.ToolChoice)
		assert.Equal(t, "test_tool", params.ToolChoice.OfTool.Name)
	})

	t.Run("tool choice forced with multiple tools", func(t *testing.T) {
		params := &anthropic.MessageNewParams{
			Tools: []anthropic.ToolUnionParam{
				{
					OfTool: &anthropic.ToolParam{
						Name: "test_tool_1",
					},
				},
				{
					OfTool: &anthropic.ToolParam{
						Name: "test_tool_2",
					},
				},
			},
		}
		toolChoice := schema.ToolChoiceForced
		options := &model.Options{
			ToolChoice: &toolChoice,
		}
		err := populateToolChoice(params, options.ToolChoice, options.AllowedToolNames, nil)
		assert.NoError(t, err)
		assert.NotNil(t, params.ToolChoice)
		assert.NotNil(t, params.ToolChoice.OfAny)
	})

	t.Run("tool choice forced with allowed tool name", func(t *testing.T) {
		params := &anthropic.MessageNewParams{
			Tools: []anthropic.ToolUnionParam{
				{
					OfTool: &anthropic.ToolParam{
						Name: "test_tool_1",
					},
				},
				{
					OfTool: &anthropic.ToolParam{
						Name: "test_tool_2",
					},
				},
			},
		}
		toolChoice := schema.ToolChoiceForced
		options := &model.Options{
			ToolChoice:       &toolChoice,
			AllowedToolNames: []string{"test_tool_1"},
		}
		err := populateToolChoice(params, options.ToolChoice, options.AllowedToolNames, nil)
		assert.NoError(t, err)
		assert.NotNil(t, params.ToolChoice)
		assert.Equal(t, "test_tool_1", params.ToolChoice.OfTool.Name)
	})

	t.Run("tool choice forced with non-existent allowed tool name", func(t *testing.T) {
		params := &anthropic.MessageNewParams{
			Tools: []anthropic.ToolUnionParam{
				{
					OfTool: &anthropic.ToolParam{
						Name: "test_tool_1",
					},
				},
			},
		}
		toolChoice := schema.ToolChoiceForced
		options := &model.Options{
			ToolChoice:       &toolChoice,
			AllowedToolNames: []string{"non_existent_tool"},
		}
		err := populateToolChoice(params, options.ToolChoice, options.AllowedToolNames, nil)
		assert.Error(t, err)
		assert.Equal(t, "allowed tool name 'non_existent_tool' not found in tools list", err.Error())
	})

	t.Run("tool choice forced with multiple allowed tool names", func(t *testing.T) {
		params := &anthropic.MessageNewParams{
			Tools: []anthropic.ToolUnionParam{
				{
					OfTool: &anthropic.ToolParam{
						Name: "test_tool_1",
					},
				},
			},
		}
		toolChoice := schema.ToolChoiceForced
		options := &model.Options{
			ToolChoice:       &toolChoice,
			AllowedToolNames: []string{"test_tool_1", "test_tool_2"},
		}
		err := populateToolChoice(params, options.ToolChoice, options.AllowedToolNames, nil)
		assert.Error(t, err)
		assert.Equal(t, "only one allowed tool name can be configured", err.Error())
	})

	t.Run("unsupported tool choice", func(t *testing.T) {
		params := &anthropic.MessageNewParams{}
		unsupportedToolChoice := schema.ToolChoice("unsupported")
		options := &model.Options{
			ToolChoice: &unsupportedToolChoice,
		}
		err := populateToolChoice(params, options.ToolChoice, options.AllowedToolNames, nil)
		assert.Error(t, err)
		assert.Equal(t, "tool choice=unsupported not support", err.Error())
	})
}

func TestCacheTTL(t *testing.T) {
	t.Run("SetMessageBreakpoint without TTL", func(t *testing.T) {
		msg := schema.UserMessage("hello")
		bp := SetMessageBreakpoint(msg)
		assert.True(t, isBreakpointMessage(bp))
		ctrl := getMessageBreakpointCacheControl(bp)
		assert.Nil(t, ctrl)
	})

	t.Run("SetMessageCacheControl with TTL", func(t *testing.T) {
		msg := schema.UserMessage("hello")
		bp := SetMessageCacheControl(msg, &CacheControl{TTL: CacheTTL1h})
		assert.True(t, isBreakpointMessage(bp))
		ctrl := getMessageBreakpointCacheControl(bp)
		assert.Equal(t, CacheTTL1h, ctrl.TTL)
	})

	t.Run("SetToolInfoBreakpoint without TTL", func(t *testing.T) {
		tool := &schema.ToolInfo{Name: "test"}
		bp := SetToolInfoBreakpoint(tool)
		assert.True(t, isBreakpointTool(bp))
		ctrl := getToolBreakpointCacheControl(bp)
		assert.Nil(t, ctrl)
	})

	t.Run("SetToolInfoCacheControl with TTL", func(t *testing.T) {
		tool := &schema.ToolInfo{Name: "test"}
		bp := SetToolInfoCacheControl(tool, &CacheControl{TTL: CacheTTL5m})
		assert.True(t, isBreakpointTool(bp))
		ctrl := getToolBreakpointCacheControl(bp)
		assert.Equal(t, CacheTTL5m, ctrl.TTL)
	})

	t.Run("newCacheControlParam without TTL", func(t *testing.T) {
		p := newCacheControlParam(nil)
		assert.Equal(t, CacheTTL(""), p.TTL)
		assert.NotEmpty(t, p.Type)
	})

	t.Run("newCacheControlParam with TTL", func(t *testing.T) {
		p := newCacheControlParam(&CacheControl{TTL: CacheTTL1h})
		assert.Equal(t, CacheTTL1h, p.TTL)
		assert.NotEmpty(t, p.Type)
	})

	t.Run("newCacheControlParam with invalid TTL passthrough", func(t *testing.T) {
		p := newCacheControlParam(&CacheControl{TTL: "invalid_ttl"})
		assert.Equal(t, CacheTTL("invalid_ttl"), p.TTL)
		assert.NotEmpty(t, p.Type)
	})

	t.Run("populateContentBlockBreakPoint with TTL", func(t *testing.T) {
		block := anthropic.NewTextBlock("input")
		populateContentBlockBreakPoint(block, &CacheControl{TTL: CacheTTL1h})
		assert.NotEmpty(t, block.OfText.CacheControl.Type)
		assert.Equal(t, CacheTTL1h, block.OfText.CacheControl.TTL)
	})

	t.Run("manual breakpoint TTL flows to params", func(t *testing.T) {
		cm := &ChatModel{model: "test", maxTokens: 100}
		msg := schema.UserMessage("hello")
		sysMsg := schema.SystemMessage("system")
		bpSys := SetMessageCacheControl(sysMsg, &CacheControl{TTL: CacheTTL1h})

		params, err := cm.genMessageNewParams([]*schema.Message{bpSys, msg})
		assert.NoError(t, err)
		assert.Equal(t, CacheTTL1h, params.System[0].CacheControl.TTL)
	})

	t.Run("WithAutoCacheControl with TTL", func(t *testing.T) {
		cm := &ChatModel{model: "test", maxTokens: 100}
		msg := schema.UserMessage("hello")
		sysMsg := schema.SystemMessage("system")

		params, err := cm.genMessageNewParams(
			[]*schema.Message{sysMsg, msg},
			WithAutoCacheControl(&CacheControl{TTL: CacheTTL1h}),
		)
		assert.NoError(t, err)
		// auto cache should set TTL on last system message
		assert.Equal(t, CacheTTL1h, params.System[0].CacheControl.TTL)
	})
}

func TestToolSearchResultInput(t *testing.T) {
	t.Run("tool search result in tool message", func(t *testing.T) {
		msg := &schema.Message{
			Role:       schema.Tool,
			ToolCallID: "tool_search_call_1",
			UserInputMultiContent: []schema.MessageInputPart{
				{
					Type: schema.ChatMessagePartTypeToolSearchResult,
					ToolSearchResult: &schema.ToolSearchResult{
						Tools: []*schema.ToolInfo{
							{Name: "found_tool_1"},
							{Name: "found_tool_2"},
						},
					},
				},
			},
		}

		result, err := convSchemaMessage(msg)
		assert.NoError(t, err)
		assert.Len(t, result.Content, 1)
		assert.NotNil(t, result.Content[0].OfToolResult)
		assert.Equal(t, "tool_search_call_1", result.Content[0].OfToolResult.ToolUseID)
		assert.Len(t, result.Content[0].OfToolResult.Content, 2)
		assert.Equal(t, "found_tool_1", result.Content[0].OfToolResult.Content[0].OfToolReference.ToolName)
		assert.Equal(t, "found_tool_2", result.Content[0].OfToolResult.Content[1].OfToolReference.ToolName)
	})

	t.Run("nil ToolSearchResult errors", func(t *testing.T) {
		msg := &schema.Message{
			Role:       schema.Tool,
			ToolCallID: "call_1",
			UserInputMultiContent: []schema.MessageInputPart{
				{
					Type:             schema.ChatMessagePartTypeToolSearchResult,
					ToolSearchResult: nil,
				},
			},
		}
		_, err := convSchemaMessage(msg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ToolSearchResult field must not be nil")
	})
}

func TestDuplicateToolError(t *testing.T) {
	params := &anthropic.MessageNewParams{
		Tools: []anthropic.ToolUnionParam{
			{OfTool: &anthropic.ToolParam{Name: "existing_tool"}},
		},
	}

	msgs := []*schema.Message{
		{
			Role:       schema.Tool,
			ToolCallID: "call_1",
			UserInputMultiContent: []schema.MessageInputPart{
				{
					Type: schema.ChatMessagePartTypeToolSearchResult,
					ToolSearchResult: &schema.ToolSearchResult{
						Tools: []*schema.ToolInfo{
							{Name: "existing_tool"},
						},
					},
				},
			},
		},
	}

	err := populateToolSearchResultTools(params, msgs)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tool 'existing_tool' from ToolSearchResult already exists in tool list")
}

func TestToolSearchEventsRoundTrip(t *testing.T) {
	t.Run("server tool use event round trip", func(t *testing.T) {
		msg := &schema.Message{
			Role:  schema.Assistant,
			Extra: make(map[string]any),
		}

		// Simulate storing a server_tool_use event
		appendToolSearchEvent(msg, ToolSearchEvent{
			Type:  "server_tool_use",
			ID:    "stu_1",
			Name:  "tool_search_tool_bm25",
			Input: json.RawMessage(`{"query":"search query"}`),
		})

		// Simulate storing a tool_search_tool_result event
		appendToolSearchEvent(msg, ToolSearchEvent{
			Type:      "tool_search_tool_result",
			ToolUseID: "stu_1",
			Content: &ToolSearchEventContent{
				Type: "tool_search_tool_search_result",
				ToolReferences: []ToolSearchEventToolReference{
					{ToolName: "matched_tool"},
				},
			},
		})

		// Verify events were stored
		events := getToolSearchEvents(msg)
		assert.Len(t, events, 2)

		// Convert to message param (round-trip)
		result, err := convSchemaMessage(msg)
		assert.NoError(t, err)

		// Should have the two tool search event blocks in the content
		hasServerToolUse := false
		hasToolSearchResult := false
		for _, block := range result.Content {
			if block.OfServerToolUse != nil {
				hasServerToolUse = true
				assert.Equal(t, "stu_1", block.OfServerToolUse.ID)
				assert.Equal(t, anthropic.ServerToolUseBlockParamName("tool_search_tool_bm25"), block.OfServerToolUse.Name)
			}
			if block.OfToolSearchToolResult != nil {
				hasToolSearchResult = true
				assert.Equal(t, "stu_1", block.OfToolSearchToolResult.ToolUseID)
				searchResult := block.OfToolSearchToolResult.Content.OfRequestToolSearchToolSearchResultBlock
				assert.NotNil(t, searchResult)
				assert.Len(t, searchResult.ToolReferences, 1)
				assert.Equal(t, "matched_tool", searchResult.ToolReferences[0].ToolName)
			}
		}
		assert.True(t, hasServerToolUse, "should have server_tool_use block")
		assert.True(t, hasToolSearchResult, "should have tool_search_tool_result block")
	})

	t.Run("error event round trip", func(t *testing.T) {
		msg := &schema.Message{
			Role:  schema.Assistant,
			Extra: make(map[string]any),
		}

		appendToolSearchEvent(msg, ToolSearchEvent{
			Type:      "tool_search_tool_result",
			ToolUseID: "stu_2",
			Content: &ToolSearchEventContent{
				Type:      "tool_search_tool_result_error",
				ErrorCode: "unavailable",
			},
		})

		result, err := convSchemaMessage(msg)
		assert.NoError(t, err)

		hasErrorResult := false
		for _, block := range result.Content {
			if block.OfToolSearchToolResult != nil {
				hasErrorResult = true
				errParam := block.OfToolSearchToolResult.Content.OfRequestToolSearchToolResultError
				assert.NotNil(t, errParam)
				assert.Equal(t, anthropic.ToolSearchToolResultErrorCode("unavailable"), errParam.ErrorCode)
			}
		}
		assert.True(t, hasErrorResult, "should have error result block")
	})
}

// TestPopulateInputMergeConsecutiveTools verifies that consecutive
// Tool messages are merged into a single Anthropic UserMessage.
// Anthropic API requires all tool_result blocks for the same assistant
// turn to be in the same user message.
func TestPopulateInputMergeConsecutiveTools(t *testing.T) {
	clearAnthropicAuthEnv(t)
	ctx := context.Background()

	cm, err := NewChatModel(ctx, &Config{
		APIKey: "test-key",
		Model:  "claude-3-opus-20240229",
	})
	assert.NoError(t, err)

	toolMsg := func(id, name, content string) *schema.Message {
		return &schema.Message{
			Role:       schema.Tool,
			Content:    content,
			ToolCallID: id,
			ToolName:   name,
		}
	}

	t.Run("single tool message unchanged", func(t *testing.T) {
		params, err := cm.genMessageNewParams([]*schema.Message{
			schema.UserMessage("hello"),
			toolMsg("call_1", "tool1", "result1"),
		})
		assert.NoError(t, err)
		assert.Len(t, params.Messages, 2)
		// The tool message becomes a UserMessage with one tool_result block
		assert.Len(t, params.Messages[1].Content, 1)
		assert.NotNil(t, params.Messages[1].Content[0].OfToolResult)
	})

	t.Run("two consecutive tool messages merged", func(t *testing.T) {
		params, err := cm.genMessageNewParams([]*schema.Message{
			schema.UserMessage("hello"),
			toolMsg("call_1", "tool1", "result1"),
			toolMsg("call_2", "tool2", "result2"),
		})
		assert.NoError(t, err)
		// User + merged tool = 2 messages
		assert.Len(t, params.Messages, 2)
		// Merged message has 2 content blocks
		assert.Len(t, params.Messages[1].Content, 2)
	})

	t.Run("three consecutive tools merged", func(t *testing.T) {
		params, err := cm.genMessageNewParams([]*schema.Message{
			schema.UserMessage("hello"),
			toolMsg("call_1", "t1", "r1"),
			toolMsg("call_2", "t2", "r2"),
			toolMsg("call_3", "t3", "r3"),
		})
		assert.NoError(t, err)
		assert.Len(t, params.Messages, 2)
		assert.Len(t, params.Messages[1].Content, 3)
	})

	t.Run("tool messages separated by non-tool are not merged", func(t *testing.T) {
		params, err := cm.genMessageNewParams([]*schema.Message{
			schema.UserMessage("hello"),
			toolMsg("call_1", "t1", "r1"),
			schema.AssistantMessage("ok", nil),
			toolMsg("call_2", "t2", "r2"),
		})
		assert.NoError(t, err)
		// User + Tool(r1) + Assistant + Tool(r2) = 4 messages
		assert.Len(t, params.Messages, 4)
		assert.Len(t, params.Messages[1].Content, 1)
		assert.Len(t, params.Messages[3].Content, 1)
	})

	t.Run("no tool messages unchanged", func(t *testing.T) {
		params, err := cm.genMessageNewParams([]*schema.Message{
			schema.UserMessage("hello"),
			schema.AssistantMessage("hi", nil),
		})
		assert.NoError(t, err)
		assert.Len(t, params.Messages, 2)
	})

	t.Run("real world pattern: user assistant(tool_calls) tool tool", func(t *testing.T) {
		params, err := cm.genMessageNewParams([]*schema.Message{
			schema.UserMessage("what's the weather in Paris and London?"),
			{
				Role:    schema.Assistant,
				Content: "",
				ToolCalls: []schema.ToolCall{
					{ID: "call_1", Function: schema.FunctionCall{Name: "get_weather", Arguments: `{"city":"Paris"}`}},
					{ID: "call_2", Function: schema.FunctionCall{Name: "get_weather", Arguments: `{"city":"London"}`}},
				},
			},
			toolMsg("call_1", "get_weather", "Paris: sunny, 22°C"),
			toolMsg("call_2", "get_weather", "London: cloudy, 15°C"),
		})
		assert.NoError(t, err)
		// User + Assistant + merged_tool = 3 messages
		assert.Len(t, params.Messages, 3)
		// Assistant has tool_use blocks
		assert.Equal(t, anthropic.MessageParamRoleAssistant, params.Messages[1].Role)
		// Merged tool message has 2 tool_result blocks
		assert.Equal(t, anthropic.MessageParamRoleUser, params.Messages[2].Role)
		assert.Len(t, params.Messages[2].Content, 2)
	})
}

func TestVertexServiceAccountJSON(t *testing.T) {
	ctx := context.Background()
	base := &Config{
		ByVertex:        true,
		VertexProjectID: "test-project",
		VertexRegion:    "us-east5",
		Model:           "claude-3-opus-20240229",
		MaxTokens:       1,
	}

	t.Run("invalid JSON", func(t *testing.T) {
		cfg := *base
		cfg.VertexServiceAccountJSON = []byte(`not-json`)
		_, err := NewChatModel(ctx, &cfg)
		assert.Error(t, err)
		assert.ErrorContains(t, err, "create vertex credentials from service account JSON")
	})

	t.Run("ADC path when service account JSON empty", func(t *testing.T) {
		mockey.PatchConvey("", t, func() {
			var calledWithGoogleAuth bool
			mockey.Mock(vertex.WithGoogleAuth).To(func(ctx context.Context, region, projectID string, scopes ...string) option.RequestOption {
				calledWithGoogleAuth = true
				assert.Equal(t, "us-east5", region)
				assert.Equal(t, "test-project", projectID)
				return option.WithAPIKey("adc-test")
			}).Build()

			model, err := NewChatModel(ctx, base)
			assert.NoError(t, err)
			assert.NotNil(t, model)
			assert.True(t, calledWithGoogleAuth, "expected vertex.WithGoogleAuth to be called for ADC path")
		})
	})

	t.Run("service account JSON uses WithCredentials path", func(t *testing.T) {
		mockey.PatchConvey("", t, func() {
			var calledCredentialsFromJSON bool
			var calledWithCredentials bool
			mockey.Mock(google.CredentialsFromJSON).To(func(ctx context.Context, jsonData []byte, scopes ...string) (*google.Credentials, error) {
				calledCredentialsFromJSON = true
				assert.Equal(t, []string{"https://www.googleapis.com/auth/cloud-platform"}, scopes)
				return &google.Credentials{ProjectID: "from-sa"}, nil
			}).Build()
			mockey.Mock(vertex.WithCredentials).To(func(ctx context.Context, region, projectID string, creds *google.Credentials) option.RequestOption {
				calledWithCredentials = true
				assert.Equal(t, "us-east5", region)
				assert.Equal(t, "test-project", projectID)
				assert.Equal(t, "from-sa", creds.ProjectID)
				return option.WithAPIKey("sa-test")
			}).Build()

			cfg := *base
			cfg.VertexServiceAccountJSON = []byte(`{"type":"service_account","project_id":"from-sa"}`)
			model, err := NewChatModel(ctx, &cfg)
			assert.NoError(t, err)
			assert.NotNil(t, model)
			assert.True(t, calledCredentialsFromJSON, "expected google.CredentialsFromJSON to be called for service account path")
			assert.True(t, calledWithCredentials, "expected vertex.WithCredentials to be called for service account path")
		})
	})
}

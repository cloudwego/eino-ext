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

package ollama

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	. "github.com/bytedance/mockey"
	"github.com/eino-contrib/ollama/api"
	"github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/assert"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func MockChatInvoke(ctx context.Context, req *api.ChatRequest, fn api.ChatResponseFunc) error {
	res := api.ChatResponse{
		Model:     req.Model,
		CreatedAt: time.Now(),
		Message: api.Message{
			Content: "test",
			Role:    "assistant",
		},
		Done:       true,
		DoneReason: "stop",
		Metrics: api.Metrics{
			EvalCount:          1,
			EvalDuration:       2,
			LoadDuration:       3,
			PromptEvalCount:    4,
			PromptEvalDuration: 5,
			TotalDuration:      6,
		},
	}

	if len(req.Tools) > 0 {
		res.Message.ToolCalls = []api.ToolCall{
			{
				Function: api.ToolCallFunction{
					Name:      req.Tools[0].Function.Name,
					Arguments: api.ToolCallFunctionArguments(map[string]any{}),
				},
			},
		}
	}
	return fn(res)
}

func MockChatInvokeError(ctx context.Context, req *api.ChatRequest, fn api.ChatResponseFunc) error {
	return errors.New("test invoke error")
}

func MockChatStreamError(ctx context.Context, req *api.ChatRequest, fn api.ChatResponseFunc) error {
	return errors.New("test stream error")
}

func MockChatStream(ctx context.Context, req *api.ChatRequest, fn api.ChatResponseFunc) error {
	res := api.ChatResponse{
		Model:     req.Model,
		CreatedAt: time.Now(),
		Message: api.Message{
			Content: "test",
			Role:    "assistant",
		},
	}

	for i := 0; i < 5; i++ {
		res.Message.Content = fmt.Sprintf("test_%03d", i)
		err := fn(res)
		if err != nil {
			return err
		}
	}

	res.DoneReason = "stop"
	res.Done = true
	res.DoneReason = "stop"
	res.Metrics = api.Metrics{
		EvalCount:          1,
		EvalDuration:       2,
		LoadDuration:       3,
		PromptEvalCount:    4,
		PromptEvalDuration: 5,
		TotalDuration:      6,
	}

	return fn(res)
}

func Test_Generate(t *testing.T) {
	PatchConvey("test Generate", t, func() {
		ctx := callbacks.InitCallbacks(context.Background(), &callbacks.RunInfo{}, callbacks.NewHandlerBuilder().Build())
		m, err := NewChatModel(ctx, &ChatModelConfig{
			Model: "asd",
		})
		convey.So(err, convey.ShouldBeNil)

		cli := m.cli
		idx := 1
		msgs := []*schema.Message{
			{
				Role:    schema.User,
				Content: "test",
				ToolCalls: []schema.ToolCall{
					{
						Index: &idx,
						ID:    "asd",
						Function: schema.FunctionCall{
							Name:      "qwe",
							Arguments: "{}",
						},
					},
				},
			},
		}

		convey.So(m.BindTools([]*schema.ToolInfo{
			{
				Name: "get_current_weather",
				Desc: "Get the current weather in a given location",
				ParamsOneOf: schema.NewParamsOneOfByParams(
					map[string]*schema.ParameterInfo{
						"location": {
							Type:     schema.String,
							Desc:     "The city and state, e.g. San Francisco, CA",
							Required: true,
						},
						"unit": {
							Type:     schema.String,
							Enum:     []string{"celsius", "fahrenheit"},
							Required: true,
						},
					}),
			},
			{
				Name: "get_current_stock_price",
				Desc: "Get the current stock price given the name of the stock",
				ParamsOneOf: schema.NewParamsOneOfByParams(
					map[string]*schema.ParameterInfo{
						"name": {
							Type:     schema.String,
							Desc:     "The name of the stock",
							Required: true,
						},
					}),
			},
		}), convey.ShouldBeNil)

		PatchConvey("test chat error", func() {
			mocker := Mock(GetMethod(cli, "Chat")).To(MockChatInvokeError).Build()
			defer mocker.UnPatch()

			outMsg, err := m.Generate(ctx, msgs)

			convey.So(err, convey.ShouldNotBeNil)
			convey.So(outMsg, convey.ShouldBeNil)
		})

		PatchConvey("test resolveChatResponse error", func() {
			mocker := Mock(GetMethod(cli, "Chat")).To(MockChatInvokeError).Build()
			defer mocker.UnPatch()

			outMsg, err := m.Generate(ctx, msgs)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(outMsg, convey.ShouldBeNil)
		})

		PatchConvey("test success", func() {
			mocker := Mock(GetMethod(cli, "Chat")).To(MockChatInvoke).Build()
			defer mocker.UnPatch()
			outMsg, err := m.Generate(ctx, msgs,
				model.WithTemperature(1),
				model.WithMaxTokens(321),
				model.WithModel("asd"),
				model.WithTopP(123), WithSeed(111))
			convey.So(err, convey.ShouldBeNil)
			convey.So(outMsg, convey.ShouldNotBeNil)
			convey.So(outMsg.Role, convey.ShouldEqual, schema.Assistant)
			convey.So(len(outMsg.ToolCalls), convey.ShouldEqual, 1)
		})

	})

}

func Test_Stream(t *testing.T) {
	PatchConvey("test Stream", t, func() {
		ctx := callbacks.InitCallbacks(context.Background(), &callbacks.RunInfo{}, callbacks.NewHandlerBuilder().Build())
		m, err := NewChatModel(ctx, &ChatModelConfig{
			Model: "asd",
		})
		convey.So(err, convey.ShouldBeNil)

		cli := m.cli
		idx := 1
		msgs := []*schema.Message{
			{
				Role:    schema.User,
				Content: "test",
				ToolCalls: []schema.ToolCall{
					{
						Index: &idx,
						ID:    "asd",
						Function: schema.FunctionCall{
							Name:      "qwe",
							Arguments: `{"hello":"world"}`,
						},
					},
				},
			},
		}

		PatchConvey("test chan err", func() {
			mocker := Mock(GetMethod(cli, "Chat")).To(MockChatStreamError).Build()
			defer mocker.UnPatch()

			outStream, err := m.Stream(ctx, msgs)
			convey.So(err, convey.ShouldBeNil)
			convey.So(outStream, convey.ShouldNotBeNil)
		})

		PatchConvey("test chan success", func() {
			mocker := Mock(GetMethod(cli, "Chat")).Return(MockChatStream).Build()
			defer mocker.UnPatch()

			outStream, err := m.Stream(ctx, msgs)
			convey.So(err, convey.ShouldBeNil)
			convey.So(outStream, convey.ShouldNotBeNil)
		})

	})
}

func TestBindTools(t *testing.T) {

	t.Run("chat model bind tools", func(t *testing.T) {
		ctx := context.Background()

		chatModel, err := NewChatModel(ctx, &ChatModelConfig{Model: "llama3"})
		assert.NoError(t, err)

		doNothingParams := map[string]*schema.ParameterInfo{
			"test": {
				Type:     schema.String,
				Desc:     "no meaning",
				Required: true,
			},
		}

		stockParams := map[string]*schema.ParameterInfo{
			"name": {
				Type:     schema.String,
				Desc:     "The name of the stock",
				Required: true,
			},
		}

		tools := []*schema.ToolInfo{
			{
				Name:        "do_nothing",
				Desc:        "do nothing",
				ParamsOneOf: schema.NewParamsOneOfByParams(doNothingParams),
			},
			{
				Name:        "get_current_stock_price",
				Desc:        "Get the current stock price given the name of the stock",
				ParamsOneOf: schema.NewParamsOneOfByParams(stockParams),
			},
		}

		err = chatModel.BindTools([]*schema.ToolInfo{tools[0]})
		assert.Nil(t, err)

	})
}

func TestPanicErr(t *testing.T) {
	err := newPanicErr("info", []byte("stack"))
	assert.Equal(t, "panic error: info, \nstack: stack", err.Error())
}

func TestWithTools(t *testing.T) {
	cm := &ChatModel{config: &ChatModelConfig{Model: "test model"}}
	ncm, err := cm.WithTools([]*schema.ToolInfo{{Name: "test tool name"}})
	assert.Nil(t, err)
	assert.Equal(t, "test model", ncm.(*ChatModel).config.Model)
	assert.Equal(t, "test tool name", ncm.(*ChatModel).tools[0].Name)
}

func Test_toOllamaMessage(t *testing.T) {
	base64Data := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII="
	PatchConvey("test toOllamaMessage", t, func() {
		PatchConvey("test simple message", func() {
			msg := &schema.Message{
				Role:    schema.User,
				Content: "test content",
			}
			ollamaMsg, err := toOllamaMessage(msg)
			convey.So(err, convey.ShouldBeNil)
			convey.So(ollamaMsg.Role, convey.ShouldEqual, string(schema.User))
			convey.So(ollamaMsg.Content, convey.ShouldEqual, "test content")
			convey.So(ollamaMsg.Images, convey.ShouldBeEmpty)
		})

		PatchConvey("test UserInputMultiContent", func() {
			msg := &schema.Message{
				Role: schema.User,
				UserInputMultiContent: []schema.MessageInputPart{
					{
						Type: schema.ChatMessagePartTypeText,
						Text: "hello",
					},
					{
						Type: schema.ChatMessagePartTypeImageURL,
						Image: &schema.MessageInputImage{
							MessagePartCommon: schema.MessagePartCommon{
								Base64Data: &base64Data,
							},
						},
					},
				},
			}
			ollamaMsg, err := toOllamaMessage(msg)
			convey.So(err, convey.ShouldBeNil)
			convey.So(ollamaMsg.Role, convey.ShouldEqual, string(schema.User))
			convey.So(ollamaMsg.Content, convey.ShouldEqual, "hello")
			convey.So(len(ollamaMsg.Images), convey.ShouldEqual, 1)
			convey.So(string(ollamaMsg.Images[0]), convey.ShouldEqual, base64Data)
		})

		PatchConvey("test AssistantGenMultiContent", func() {
			msg := &schema.Message{
				Role: schema.Assistant,
				AssistantGenMultiContent: []schema.MessageOutputPart{
					{
						Type: schema.ChatMessagePartTypeText,
						Text: "hello",
					},
					{
						Type: schema.ChatMessagePartTypeImageURL,
						Image: &schema.MessageOutputImage{
							MessagePartCommon: schema.MessagePartCommon{
								Base64Data: &base64Data,
							},
						},
					},
				},
			}
			ollamaMsg, err := toOllamaMessage(msg)
			convey.So(err, convey.ShouldBeNil)
			convey.So(ollamaMsg.Role, convey.ShouldEqual, string(schema.Assistant))
			convey.So(ollamaMsg.Content, convey.ShouldEqual, "hello")
			convey.So(len(ollamaMsg.Images), convey.ShouldEqual, 1)
			convey.So(string(ollamaMsg.Images[0]), convey.ShouldEqual, base64Data)
		})

		PatchConvey("test AssistantGenMultiContent with correct role", func() {
			msg := &schema.Message{
				Role: schema.Assistant,
				AssistantGenMultiContent: []schema.MessageOutputPart{
					{
						Type: schema.ChatMessagePartTypeText,
						Text: "world",
					},
				},
			}
			ollamaMsg, err := toOllamaMessage(msg)
			convey.So(err, convey.ShouldBeNil)
			convey.So(ollamaMsg.Role, convey.ShouldEqual, string(schema.Assistant))
			convey.So(ollamaMsg.Content, convey.ShouldEqual, "world")
		})

		PatchConvey("test AssistantGenMultiContent with incorrect role", func() {
			msg := &schema.Message{
				Role: schema.User, // Incorrect role
				AssistantGenMultiContent: []schema.MessageOutputPart{
					{
						Type: schema.ChatMessagePartTypeText,
						Text: "world",
					},
				},
			}
			_, err := toOllamaMessage(msg)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "assistant gen multi content only support assistant role")
		})

		PatchConvey("test MultiContent compatibility", func() {
			msg := &schema.Message{
				Role: schema.User,
				MultiContent: []schema.ChatMessagePart{
					{
						Type: schema.ChatMessagePartTypeText,
						Text: "legacy content",
					},
					{
						Type: schema.ChatMessagePartTypeImageURL,
						ImageURL: &schema.ChatMessageImageURL{
							URL: base64Data,
						},
					},
				},
			}
			ollamaMsg, err := toOllamaMessage(msg)
			convey.So(err, convey.ShouldBeNil)
			convey.So(ollamaMsg.Content, convey.ShouldEqual, "legacy content")
			convey.So(len(ollamaMsg.Images), convey.ShouldEqual, 1)
			convey.So(string(ollamaMsg.Images[0]), convey.ShouldEqual, base64Data)
		})

		PatchConvey("test error on http URL in MultiContent", func() {
			msg := &schema.Message{
				Role: schema.User,
				MultiContent: []schema.ChatMessagePart{
					{
						Type: schema.ChatMessagePartTypeImageURL,
						ImageURL: &schema.ChatMessageImageURL{
							URL: "http://example.com/image.png",
						},
					},
				},
			}
			_, err := toOllamaMessage(msg)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "ollama model only supports base64-encoded strings")
		})

		PatchConvey("test error on nil image in UserInputMultiContent", func() {
			msg := &schema.Message{
				Role: schema.User,
				UserInputMultiContent: []schema.MessageInputPart{
					{
						Type:  schema.ChatMessagePartTypeImageURL,
						Image: nil, // Nil image
					},
				},
			}
			_, err := toOllamaMessage(msg)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldEqual, "image is required in UserInputMultiContent, but got nil")
		})

		PatchConvey("test error on nil image in AssistantGenMultiContent", func() {
			msg := &schema.Message{
				Role: schema.Assistant,
				AssistantGenMultiContent: []schema.MessageOutputPart{
					{
						Type:  schema.ChatMessagePartTypeImageURL,
						Image: nil, // Nil image
					},
				},
			}
			_, err := toOllamaMessage(msg)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldEqual, "image is required in AssistantGenMultiContent, but got nil")
		})

		PatchConvey("test error on nil ImageURL in MultiContent", func() {
			msg := &schema.Message{
				Role: schema.User,
				MultiContent: []schema.ChatMessagePart{
					{
						Type:     schema.ChatMessagePartTypeImageURL,
						ImageURL: nil, // Nil ImageURL
					},
				},
			}
			_, err := toOllamaMessage(msg)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldEqual, "image url is required")
		})

		PatchConvey("test error on URL in UserInputMultiContent", func() {
			url := "http://example.com/image.png"
			msg := &schema.Message{
				Role: schema.User,
				UserInputMultiContent: []schema.MessageInputPart{
					{
						Type:  schema.ChatMessagePartTypeImageURL,
						Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{URL: &url}},
					},
				},
			}
			_, err := toOllamaMessage(msg)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "ollama model only supports base64-encoded strings")
		})

		PatchConvey("test error on nil Base64Data in UserInputMultiContent", func() {
			msg := &schema.Message{
				Role: schema.User,
				UserInputMultiContent: []schema.MessageInputPart{
					{
						Type:  schema.ChatMessagePartTypeImageURL,
						Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{Base64Data: nil}},
					},
				},
			}
			_, err := toOllamaMessage(msg)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldEqual, "image is required in UserInputMultiContent, but got nil Base64Data")
		})

		PatchConvey("test error on URL in AssistantGenMultiContent", func() {
			url := "http://example.com/image.png"
			msg := &schema.Message{
				Role: schema.Assistant,
				AssistantGenMultiContent: []schema.MessageOutputPart{
					{
						Type:  schema.ChatMessagePartTypeImageURL,
						Image: &schema.MessageOutputImage{MessagePartCommon: schema.MessagePartCommon{URL: &url}},
					},
				},
			}
			_, err := toOllamaMessage(msg)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "ollama model only supports base64-encoded strings")
		})

		PatchConvey("test error on nil Base64Data in AssistantGenMultiContent", func() {
			msg := &schema.Message{
				Role: schema.Assistant,
				AssistantGenMultiContent: []schema.MessageOutputPart{
					{
						Type:  schema.ChatMessagePartTypeImageURL,
						Image: &schema.MessageOutputImage{MessagePartCommon: schema.MessagePartCommon{Base64Data: nil}},
					},
				},
			}
			_, err := toOllamaMessage(msg)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldEqual, "image is required in AssistantGenMultiContent, but got nil Base64Data")
		})

		PatchConvey("test error on both UserInputMultiContent and AssistantGenMultiContent", func() {
			msg := &schema.Message{
				Role:                     schema.User,
				UserInputMultiContent:    []schema.MessageInputPart{{Type: schema.ChatMessagePartTypeText, Text: "user"}},
				AssistantGenMultiContent: []schema.MessageOutputPart{{Type: schema.ChatMessagePartTypeText, Text: "assistant"}},
			}
			_, err := toOllamaMessage(msg)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "a message cannot contain both UserInputMultiContent and AssistantGenMultiContent")
		})
	})
}

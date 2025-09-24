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

package arkbot

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"

	. "github.com/bytedance/mockey"
	"github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/assert"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/utils"

	fmodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func Test_Generate(t *testing.T) {
	PatchConvey("test Generate", t, func() {
		ctx := context.Background()
		m, err := NewChatModel(ctx, &Config{
			APIKey: "asd",
			Model:  "asd",
		})
		convey.So(err, convey.ShouldBeNil)

		cli := m.client
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
							Arguments: "zxc",
						},
					},
				},
			},
		}

		_, err = m.WithTools([]*schema.ToolInfo{
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
		})
		convey.So(err, convey.ShouldBeNil)

		PatchConvey("test chat error", func() {
			Mock(GetMethod(cli, "CreateBotChatCompletion")).Return(
				nil, errors.New("test for error")).Build()

			outMsg, err := m.Generate(ctx, msgs)

			convey.So(err, convey.ShouldNotBeNil)
			convey.So(outMsg, convey.ShouldBeNil)
		})

		PatchConvey("test resolveChatResponse error", func() {
			Mock(GetMethod(cli, "CreateBotChatCompletion")).Return(
				model.BotChatCompletionResponse{
					ChatCompletionResponse: model.ChatCompletionResponse{
						ID:      "123",
						Choices: []*model.ChatCompletionChoice{},
					},
				}, nil).Build()

			outMsg, err := m.Generate(ctx, msgs)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(outMsg, convey.ShouldBeNil)
		})

		PatchConvey("test success", func() {
			Mock(GetMethod(cli, "CreateBotChatCompletion")).Return(
				model.BotChatCompletionResponse{
					ChatCompletionResponse: model.ChatCompletionResponse{
						Usage: model.Usage{
							CompletionTokens: 1,
							PromptTokens:     2,
							TotalTokens:      3,
						},
						Choices: []*model.ChatCompletionChoice{
							{
								Message: model.ChatCompletionMessage{
									Content:    &model.ChatCompletionMessageContent{StringValue: ptrOf("test_content")},
									Role:       model.ChatMessageRoleAssistant,
									ToolCallID: "",
									ToolCalls: []*model.ToolCall{
										{
											Function: model.FunctionCall{
												Arguments: "ccc",
												Name:      "qqq",
											},
											ID:   "123",
											Type: model.ToolTypeFunction,
										},
									},
								},
							},
						},
					}}, nil).Build()

			outMsg, err := m.Generate(ctx, msgs,
				fmodel.WithTemperature(1),
				fmodel.WithMaxTokens(321),
				fmodel.WithModel("asd"),
				fmodel.WithTopP(123))
			convey.So(err, convey.ShouldBeNil)
			convey.So(outMsg, convey.ShouldNotBeNil)
			convey.So(outMsg.Role, convey.ShouldEqual, schema.Assistant)
			convey.So(len(outMsg.ToolCalls), convey.ShouldEqual, 1)
		})

		PatchConvey("generate_with_image_success", func() {

			multiModalMsg := schema.UserMessage("")
			multiModalMsg.MultiContent = []schema.ChatMessagePart{
				{
					Type: schema.ChatMessagePartTypeText,
					Text: "image_desc",
				},
				{
					Type: schema.ChatMessagePartTypeImageURL,
					ImageURL: &schema.ChatMessageImageURL{
						URL:    "https://{RL_ADDRESS}",
						Detail: schema.ImageURLDetailAuto,
					},
				},
			}

			req, err := toArkContent(multiModalMsg)
			convey.So(err, convey.ShouldBeNil)
			convey.So(req.StringValue, convey.ShouldBeNil)
			convey.So(req.ListValue, convey.ShouldHaveLength, 2)
			convey.So(req.ListValue[0], convey.ShouldEqual, &model.ChatCompletionMessageContentPart{
				Type: model.ChatCompletionMessageContentPartTypeText,
				Text: "image_desc",
			})
			convey.So(req.ListValue[1], convey.ShouldEqual, &model.ChatCompletionMessageContentPart{
				Type: model.ChatCompletionMessageContentPartTypeImageURL,
				ImageURL: &model.ChatMessageImageURL{
					URL:    "https://{RL_ADDRESS}",
					Detail: model.ImageURLDetailAuto,
				},
			})
		})
	})

}

func Test_Stream(t *testing.T) {
	PatchConvey("test Stream", t, func() {
		ctx := context.Background()
		m, err := NewChatModel(ctx, &Config{
			APIKey: "asd",
			Model:  "asd",
		})
		convey.So(err, convey.ShouldBeNil)

		cli := m.client
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
							Arguments: "zxc",
						},
					},
				},
			},
		}

		PatchConvey("test chan err", func() {
			Mock(GetMethod(cli, "CreateBotChatCompletionStream")).Return(
				nil, errors.New("test stream error")).Build()

			outStream, err := m.Stream(ctx, msgs)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(outStream, convey.ShouldBeNil)
		})

		PatchConvey("test native recv parse err", func() {
			sr := utils.BotChatCompletionStreamReader{}

			Mock(GetMethod(cli, "CreateBotChatCompletionStream")).Return(
				&sr, nil).Build()

			times := 0
			Mock((*utils.BotChatCompletionStreamReader).Recv).To(
				func() (response model.BotChatCompletionStreamResponse, err error) {
					if times >= 2 {
						return model.BotChatCompletionStreamResponse{}, io.EOF
					}

					times++
					index := times
					return model.BotChatCompletionStreamResponse{
						BotUsage: &model.BotUsage{
							ModelUsage:  []*model.BotModelUsage{{Usage: model.Usage{TotalTokens: 10}}},
							ActionUsage: []*model.BotActionUsage{{TotalTokens: 10}},
						},
						References: []*model.BotChatResultReference{
							{
								Url: "test",
							},
						},
						ChatCompletionStreamResponse: model.ChatCompletionStreamResponse{
							Usage: &model.Usage{
								CompletionTokens: 1,
								PromptTokens:     2,
								TotalTokens:      3,
							},
							Choices: []*model.ChatCompletionStreamChoice{
								{
									Delta: model.ChatCompletionStreamChoiceDelta{
										Content: fmt.Sprintf("test_content_%03d\n", times),
										Role:    model.ChatMessageRoleAssistant,
										ToolCalls: []*model.ToolCall{
											{
												ID:   "123",
												Type: model.ToolTypeFunction,
												Function: model.FunctionCall{
													Arguments: "ccc",
													Name:      "qqq",
												},
												Index: &index,
											},
										},
									},
								},
							}},
					}, nil
				},
			).Build()

			outStreamReader, err := m.Stream(ctx, msgs)
			convey.So(err, convey.ShouldBeNil)
			convey.So(outStreamReader, convey.ShouldNotBeNil)

			defer outStreamReader.Close()

			var msgs []*schema.Message
			for {
				item, e := outStreamReader.Recv()
				if e != nil {
					convey.ShouldBeError(e, io.EOF)

					break
				}

				msgs = append(msgs, item)
			}

			msg, err := schema.ConcatMessages(msgs)
			convey.So(err, convey.ShouldBeNil)
			convey.So(msg.Role, convey.ShouldEqual, schema.Assistant)
			convey.So(msg.Content, convey.ShouldEqual, "test_content_001\ntest_content_002\n")
			convey.So(len(msg.ToolCalls), convey.ShouldEqual, 2)

			bu, existed := GetBotUsage(msg)
			convey.So(existed, convey.ShouldBeTrue)
			convey.So(bu, convey.ShouldEqual, &model.BotUsage{
				ModelUsage:  []*model.BotModelUsage{{Usage: model.Usage{TotalTokens: 10}}, {Usage: model.Usage{TotalTokens: 10}}},
				ActionUsage: []*model.BotActionUsage{{TotalTokens: 10}, {TotalTokens: 10}},
			})

			ref, existed := GetBotChatResultReference(msg)
			convey.So(existed, convey.ShouldBeTrue)
			convey.So(ref, convey.ShouldEqual, []*model.BotChatResultReference{
				{Url: "test"}, {Url: "test"},
			})
		})

	})
}

func TestPanicErr(t *testing.T) {
	err := newPanicErr("info", []byte("stack"))
	assert.Equal(t, "panic error: info, \nstack: stack", err.Error())
}

func TestWithTools(t *testing.T) {
	cm := &ChatModel{config: &Config{Model: "test model"}}
	ncm, err := cm.WithTools([]*schema.ToolInfo{{Name: "test tool name"}})
	assert.Nil(t, err)
	assert.Equal(t, "test model", ncm.(*ChatModel).config.Model)
	assert.Equal(t, "test tool name", ncm.(*ChatModel).rawTools[0].Name)
}

func TestLogProbs(t *testing.T) {
	assert.Equal(t, &schema.LogProbs{Content: []schema.LogProb{
		{
			Token:   "1",
			LogProb: 1,
			Bytes:   []int64{'a'},
			TopLogProbs: []schema.TopLogProb{
				{
					Token:   "2",
					LogProb: 2,
					Bytes:   []int64{'b'},
				},
			},
		},
	}}, toLogProbs(&model.LogProbs{Content: []*model.LogProb{
		{
			Token:   "1",
			LogProb: 1,
			Bytes:   []rune{'a'},
			TopLogProbs: []*model.TopLogProbs{
				{
					Token:   "2",
					LogProb: 2,
					Bytes:   []rune{'b'},
				},
			},
		},
	}}))
}

func Test_toArkContent(t *testing.T) {
	PatchConvey("test toArkContent", t, func() {
		base64Data := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII="
		httpURL := "https://example.com/image.png"

		PatchConvey("UserInputMultiContent", func() {
			PatchConvey("success", func() {
				msg := &schema.Message{
					Role: schema.User,
					UserInputMultiContent: []schema.MessageInputPart{
						{Type: schema.ChatMessagePartTypeText, Text: "hello"},
						{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{Base64Data: &base64Data, MIMEType: "image/png"}}},
						{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{URL: &httpURL}}},
					},
				}
				content, err := toArkContent(msg)
				convey.So(err, convey.ShouldBeNil)
				convey.So(content.ListValue, convey.ShouldHaveLength, 3)
				convey.So(content.ListValue[0].Text, convey.ShouldEqual, "hello")
				convey.So(content.ListValue[1].ImageURL.URL, convey.ShouldContainSubstring, "data:image/png;base64,")
				convey.So(content.ListValue[2].ImageURL.URL, convey.ShouldEqual, httpURL)
			})

			PatchConvey("nil image", func() {
				msg := &schema.Message{
					UserInputMultiContent: []schema.MessageInputPart{
						{Type: schema.ChatMessagePartTypeImageURL, Image: nil},
					},
				}
				_, err := toArkContent(msg)
				convey.So(err, convey.ShouldNotBeNil)
			})

			PatchConvey("empty image", func() {
				msg := &schema.Message{
					UserInputMultiContent: []schema.MessageInputPart{
						{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{}},
					},
				}
				_, err := toArkContent(msg)
				convey.So(err, convey.ShouldNotBeNil)
			})

			PatchConvey("no mime type", func() {
				msg := &schema.Message{
					UserInputMultiContent: []schema.MessageInputPart{
						{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{Base64Data: &base64Data}}},
					},
				}
				_, err := toArkContent(msg)
				convey.So(err, convey.ShouldNotBeNil)
			})
		})

		PatchConvey("AssistantGenMultiContent", func() {
			PatchConvey("success", func() {
				msg := &schema.Message{
					Role: schema.Assistant,
					AssistantGenMultiContent: []schema.MessageOutputPart{
						{Type: schema.ChatMessagePartTypeText, Text: "assistant response"},
					},
				}
				content, err := toArkContent(msg)
				convey.So(err, convey.ShouldBeNil)
				convey.So(content.ListValue, convey.ShouldHaveLength, 1)
			})

			PatchConvey("success with image", func() {
				msg := &schema.Message{
					Role: schema.Assistant,
					AssistantGenMultiContent: []schema.MessageOutputPart{
						{
							Type: schema.ChatMessagePartTypeImageURL,
							Image: &schema.MessageOutputImage{
								MessagePartCommon: schema.MessagePartCommon{
									URL: &httpURL,
								},
							},
						},
						{
							Type: schema.ChatMessagePartTypeImageURL,
							Image: &schema.MessageOutputImage{
								MessagePartCommon: schema.MessagePartCommon{
									Base64Data: &base64Data,
									MIMEType:   "image/png",
								},
							},
						},
					},
				}
				content, err := toArkContent(msg)
				convey.So(err, convey.ShouldBeNil)
				convey.So(content.ListValue, convey.ShouldHaveLength, 2)
				convey.So(content.ListValue[0].ImageURL.URL, convey.ShouldEqual, httpURL)
				convey.So(content.ListValue[1].ImageURL.URL, convey.ShouldContainSubstring, "data:image/png;base64,")
			})

			PatchConvey("wrong role", func() {
				msg := &schema.Message{
					Role:                     schema.User,
					AssistantGenMultiContent: []schema.MessageOutputPart{{}},
				}
				_, err := toArkContent(msg)
				convey.So(err, convey.ShouldNotBeNil)
			})

			PatchConvey("nil image", func() {
				msg := &schema.Message{
					Role: schema.Assistant,
					AssistantGenMultiContent: []schema.MessageOutputPart{
						{Type: schema.ChatMessagePartTypeImageURL, Image: nil},
					},
				}
				_, err := toArkContent(msg)
				convey.So(err, convey.ShouldNotBeNil)
			})

			PatchConvey("empty image", func() {
				msg := &schema.Message{
					Role: schema.Assistant,
					AssistantGenMultiContent: []schema.MessageOutputPart{
						{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageOutputImage{}},
					},
				}
				_, err := toArkContent(msg)
				convey.So(err, convey.ShouldNotBeNil)
			})

			PatchConvey("no mime type", func() {
				msg := &schema.Message{
					Role: schema.Assistant,
					AssistantGenMultiContent: []schema.MessageOutputPart{
						{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageOutputImage{MessagePartCommon: schema.MessagePartCommon{Base64Data: &base64Data}}},
					},
				}
				_, err := toArkContent(msg)
				convey.So(err, convey.ShouldNotBeNil)
			})
		})

		PatchConvey("MultiContent", func() {
			msg := &schema.Message{
				MultiContent: []schema.ChatMessagePart{
					{Type: schema.ChatMessagePartTypeText, Text: "legacy"},
					{Type: schema.ChatMessagePartTypeImageURL, ImageURL: &schema.ChatMessageImageURL{URL: httpURL}},
				},
			}
			content, err := toArkContent(msg)
			convey.So(err, convey.ShouldBeNil)
			convey.So(content.ListValue, convey.ShouldHaveLength, 2)
		})

		PatchConvey("Text Content", func() {
			msg := &schema.Message{Content: "just text"}
			content, err := toArkContent(msg)
			convey.So(err, convey.ShouldBeNil)
			convey.So(*content.StringValue, convey.ShouldEqual, "just text")
		})

		PatchConvey("both UserInputMultiContent and AssistantGenMultiContent", func() {
			msg := &schema.Message{
				UserInputMultiContent:    []schema.MessageInputPart{{Type: schema.ChatMessagePartTypeText, Text: "user"}},
				AssistantGenMultiContent: []schema.MessageOutputPart{{Type: schema.ChatMessagePartTypeText, Text: "assistant"}},
			}
			_, err := toArkContent(msg)
			convey.So(err, convey.ShouldNotBeNil)
		})
	})
}

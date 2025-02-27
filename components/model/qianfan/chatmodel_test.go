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

package qianfan

import (
	"context"
	"errors"
	"testing"

	"github.com/baidubce/bce-qianfan-sdk/go/qianfan"
	. "github.com/bytedance/mockey"
	"github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/assert"

	fmodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func Test_Generate(t *testing.T) {
	PatchConvey("test Generate", t, func() {
		ctx := context.Background()
		m, err := NewChatModel(ctx, &ChatModelConfig{
			Model: "asd",
		})
		convey.So(err, convey.ShouldBeNil)

		cli := m.cc
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
			Mock(GetMethod(cli, "Do")).Return(
				nil, errors.New("test for error")).Build()

			outMsg, err := m.Generate(ctx, msgs)

			convey.So(err, convey.ShouldNotBeNil)
			convey.So(outMsg, convey.ShouldBeNil)
		})

		PatchConvey("test ChatCompletionV2Response error", func() {
			Mock(GetMethod(cli, "Do")).Return(
				&qianfan.ChatCompletionV2Response{
					Error: &qianfan.ChatCompletionV2Error{
						Code:    "123",
						Message: "asd",
						Type:    "qwe",
					},
				}, nil).Build()

			outMsg, err := m.Generate(ctx, msgs)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(outMsg, convey.ShouldBeNil)
		})

		PatchConvey("test Choices empty", func() {
			Mock(GetMethod(cli, "Do")).Return(
				&qianfan.ChatCompletionV2Response{
					Choices: nil,
				}, nil).Build()

			outMsg, err := m.Generate(ctx, msgs)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(outMsg, convey.ShouldBeNil)
		})

		PatchConvey("test choice not found", func() {
			Mock(GetMethod(cli, "Do")).Return(
				&qianfan.ChatCompletionV2Response{
					Choices: []qianfan.ChatCompletionV2Choice{
						{
							Index: 1,
						},
					},
				}, nil).Build()

			outMsg, err := m.Generate(ctx, msgs)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(outMsg, convey.ShouldBeNil)
		})

		PatchConvey("test success", func() {
			Mock(GetMethod(cli, "Do")).Return(
				&qianfan.ChatCompletionV2Response{
					Choices: []qianfan.ChatCompletionV2Choice{
						{
							Index: 0,
							Message: qianfan.ChatCompletionV2Message{
								Role:    "assistant",
								Content: "test_content",
								Name:    "test_name",
								ToolCalls: []qianfan.ToolCall{
									{
										Function: qianfan.FunctionCallV2{
											Arguments: "ccc",
											Name:      "qqq",
										},
										Id:       "123",
										ToolType: "function",
									},
								},
								ToolCallId: "",
							},
						},
					},
					Usage: &qianfan.ModelUsage{
						PromptTokens:     1,
						CompletionTokens: 2,
						TotalTokens:      3,
					},
				}, nil).Build()

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
	})
}

func Test_Stream(t *testing.T) {
	PatchConvey("test Stream", t, func() {
		ctx := context.Background()
		m, err := NewChatModel(ctx, &ChatModelConfig{Model: "asd"})
		convey.So(err, convey.ShouldBeNil)

		cli := m.cc
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

		PatchConvey("test Stream err", func() {
			Mock(GetMethod(cli, "Stream")).Return(
				nil, errors.New("test stream error")).Build()

			outStream, err := m.Stream(ctx, msgs)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(outStream, convey.ShouldBeNil)
		})

		// ChatCompletionV2ResponseStream not able to mock
		// so can't test stream recv
		PatchConvey("test resolveQianfanStreamResponse", func() {
			rawMsgs := []*qianfan.ChatCompletionV2Response{
				{
					Choices: []qianfan.ChatCompletionV2Choice{
						{
							Index: 0,
							Delta: qianfan.ChatCompletionV2Delta{
								Content:   "test_content_001",
								ToolCalls: nil,
							},
						},
					},
				},
				{
					Choices: []qianfan.ChatCompletionV2Choice{
						{
							Index: 0,
							Delta: qianfan.ChatCompletionV2Delta{
								Content:   "test_content_002",
								ToolCalls: nil,
							},
						},
					},
				},
				{
					Usage: &qianfan.ModelUsage{
						PromptTokens:     1,
						CompletionTokens: 2,
						TotalTokens:      3,
					},
				},
				{},
			}

			var mm []*schema.Message
			for i := range rawMsgs {
				msg, found, err := resolveQianfanStreamResponse(rawMsgs[i])
				convey.So(err, convey.ShouldBeNil)

				if i == 3 {
					convey.So(found, convey.ShouldBeFalse)
				} else {
					convey.So(found, convey.ShouldBeTrue)
				}

				if msg == nil {
					continue
				}

				mm = append(mm, msg)
			}

			msg, err := schema.ConcatMessages(mm)
			convey.So(err, convey.ShouldBeNil)
			convey.So(msg.Role, convey.ShouldEqual, schema.Assistant)
			convey.So(msg.Content, convey.ShouldEqual, "test_content_001test_content_002")
			convey.So(msg.ResponseMeta.Usage, convey.ShouldEqual, &schema.TokenUsage{
				PromptTokens:     1,
				CompletionTokens: 2,
				TotalTokens:      3,
			})
		})
	})
}

func TestBindTools(t *testing.T) {
	PatchConvey("chat model force tool call", t, func() {
		ctx := context.Background()

		chatModel, err := NewChatModel(ctx, &ChatModelConfig{Model: "test"})
		convey.So(err, convey.ShouldBeNil)

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
		convey.So(err, convey.ShouldBeNil)

	})
}
func TestPanicErr(t *testing.T) {
	err := newPanicErr("info", []byte("stack"))
	assert.Equal(t, "panic error: info, \nstack: stack", err.Error())
}

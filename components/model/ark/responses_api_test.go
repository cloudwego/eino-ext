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

package ark

import (
	"context"
	"io"
	"testing"
	"time"

	. "github.com/bytedance/mockey"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime"
	arkModel "github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model/responses"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/utils"
)

func TestResponsesAPIChatModelGenerate(t *testing.T) {
	PatchConvey("test Generate", t, func() {
		Mock(callbacks.OnError).Return(context.Background()).Build()
		Mock((*responsesAPIChatModel).genRequestAndOptions).
			Return(&responses.ResponsesRequest{}, nil).Build()
		Mock((*responsesAPIChatModel).toCallbackConfig).
			Return(&model.Config{}).Build()
		MockGeneric(callbacks.OnStart[*callbacks.CallbackInput]).Return(context.Background()).Build()

		Mock((*arkruntime.Client).CreateResponses).
			Return(&responses.ResponseObject{
				Usage: &responses.Usage{InputTokensDetails: &responses.InputTokensDetails{},
					OutputTokensDetails: &responses.OutputTokensDetails{ReasoningTokens: 100}},
			}, nil).Build()

		Mock((*responsesAPIChatModel).toOutputMessage).To(func(resp *responses.ResponseObject, cache *cacheConfig) (*schema.Message, error) {
			cm := &responsesAPIChatModel{}
			return &schema.Message{
				Role:    schema.Assistant,
				Content: "assistant",
				ResponseMeta: &schema.ResponseMeta{
					Usage: cm.toEinoTokenUsage(resp.Usage),
				},
			}, nil
		}).Build()

		MockGeneric(callbacks.OnEnd[*callbacks.CallbackOutput]).Return(context.Background()).Build()

		cm := &responsesAPIChatModel{}
		msg, err := cm.Generate(context.Background(), []*schema.Message{
			{
				Role:    schema.User,
				Content: "user",
			},
		})
		assert.Nil(t, err)
		assert.Equal(t, 100, msg.ResponseMeta.Usage.CompletionTokensDetails.ReasoningTokens)
		assert.Equal(t, "assistant", msg.Content)
	})
}

func TestResponsesAPIChatModelStream(t *testing.T) {
	PatchConvey("test Stream", t, func() {
		ctx := context.Background()
		sr, sw := schema.Pipe[*model.CallbackOutput](1)

		Mock(callbacks.OnError).Return(ctx).Build()

		Mock((*responsesAPIChatModel).genRequestAndOptions).
			Return(&responses.ResponsesRequest{}, nil).Build()

		Mock((*responsesAPIChatModel).toCallbackConfig).
			Return(&model.Config{}).Build()
		MockGeneric(callbacks.OnStart[*callbacks.CallbackInput]).Return(context.Background()).Build()

		Mock((*arkruntime.Client).CreateResponsesStream).
			Return(&utils.ResponsesStreamReader{}, nil).Build()

		Mock((*utils.ChatCompletionStreamReader).Close).Return(nil).Build()

		MockGeneric(schema.Pipe[*model.CallbackOutput]).
			Return(sr, sw).Build()

		Mock((*responsesAPIChatModel).receivedStreamResponse).Return().Build()

		cm := &responsesAPIChatModel{}
		stream, err := cm.Stream(context.Background(), []*schema.Message{
			{
				Role:    schema.User,
				Content: "user",
			},
		})
		assert.Nil(t, err)

		for {
			msg, err := stream.Recv()
			if err == io.EOF {
				break
			}
			assert.Nil(t, err)
			assert.Equal(t, "1", msg.Content)
		}
	})
}

func TestResponsesAPIChatModelInjectInput(t *testing.T) {
	cm := &responsesAPIChatModel{}

	PatchConvey("empty input message", t, func() {
		req := &responses.ResponsesRequest{
			Model: "test-model",
		}
		var in []*schema.Message
		err := cm.populateInput(in, req)
		assert.Nil(t, err)
	})

	PatchConvey("user message", t, func() {
		req := &responses.ResponsesRequest{
			Model: "test-model",
		}
		in := []*schema.Message{
			{
				Role:    schema.User,
				Content: "Hello",
			},
		}

		err := cm.populateInput(in, req)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(req.GetInput().GetListValue().GetListValue()))
		item := req.GetInput().GetListValue().GetListValue()[0].GetInputMessage()
		assert.Equal(t, responses.MessageRole_user, item.Role)
		assert.Equal(t, "Hello", item.Content[0].GetText().GetText())
	})

	PatchConvey("assistant message", t, func() {
		req := &responses.ResponsesRequest{
			Model: "test-model",
		}
		in := []*schema.Message{
			{
				Role:    schema.Assistant,
				Content: "Hi there",
			},
		}

		err := cm.populateInput(in, req)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(req.GetInput().GetListValue().GetListValue()))

		item := req.GetInput().GetListValue().GetListValue()[0].GetInputMessage()
		assert.Equal(t, responses.MessageRole_assistant, item.Role)
		assert.Equal(t, "Hi there", item.Content[0].GetText().GetText())
	})

	PatchConvey("system message", t, func() {
		req := &responses.ResponsesRequest{
			Model: "test-model",
		}
		in := []*schema.Message{
			{
				Role:    schema.System,
				Content: "You are a helpful assistant.",
			},
		}

		err := cm.populateInput(in, req)
		assert.Nil(t, err)

		assert.Nil(t, err)
		assert.Equal(t, 1, len(req.GetInput().GetListValue().GetListValue()))

		item := req.GetInput().GetListValue().GetListValue()[0].GetInputMessage()
		assert.Equal(t, responses.MessageRole_system, item.Role)
		assert.Equal(t, "You are a helpful assistant.", item.Content[0].GetText().GetText())

	})
	//
	PatchConvey("tool call", t, func() {
		req := &responses.ResponsesRequest{
			Model: "test-model",
		}
		in := []*schema.Message{
			{
				Role:       schema.Tool,
				ToolCallID: "call_123",
				Content:    "tool output",
			},
		}

		err := cm.populateInput(in, req)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(req.GetInput().GetListValue().GetListValue()))

		item := req.GetInput().GetListValue().GetListValue()[0].GetFunctionToolCallOutput()
		assert.Equal(t, "call_123", item.CallId)
		assert.Equal(t, "tool output", item.Output)
	})

	PatchConvey("unknown role", t, func() {
		req := &responses.ResponsesRequest{
			Model: "test-model",
		}
		in := []*schema.Message{
			{
				Role:    "unknown_role",
				Content: "some content",
			},
		}
		err := cm.populateInput(in, req)
		assert.NotNil(t, err)
	})
}

func TestResponsesAPIChatModelToOpenaiMultiModalContent(t *testing.T) {
	cm := &responsesAPIChatModel{}

	PatchConvey("image message", t, func() {
		msg := &schema.Message{
			Role: schema.User,
			UserInputMultiContent: []schema.MessageInputPart{
				{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{
					MessagePartCommon: schema.MessagePartCommon{
						URL: ptrOf("https://example.com/image.png"),
					},
				}},
			},
		}

		content, err := cm.toArkUserRoleItemInputMessage(msg)
		assert.Nil(t, err)

		contentList := content.Content
		assert.Equal(t, 1, len(contentList))
		assert.Equal(t, "https://example.com/image.png", *contentList[0].GetImage().ImageUrl)
	})

	PatchConvey("unknown modal type", t, func() {
		msg := &schema.Message{
			Role: schema.User,
			MultiContent: []schema.ChatMessagePart{
				{
					Type: "unsupported_type",
				},
			},
		}
		_, err := cm.toArkUserRoleItemInputMessage(msg)
		assert.NotNil(t, err)
	})
}

func TestResponsesAPIChatModelToTools(t *testing.T) {
	cm := &responsesAPIChatModel{}

	PatchConvey("empty tools", t, func() {
		tools := []*schema.ToolInfo{}
		openAITools, err := cm.toTools(tools)
		assert.Nil(t, err)
		assert.Equal(t, 0, len(openAITools))
	})

	PatchConvey("single tool", t, func() {
		tools := []*schema.ToolInfo{
			{
				Name: "test tool",
				Desc: "description of test tool",
				ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
					"param": {
						Type:     schema.String,
						Desc:     "description of param1",
						Required: true,
					},
				}),
			},
		}
		responsesTools, err := cm.toTools(tools)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(responsesTools))
		assert.Equal(t, tools[0].Name, responsesTools[0].GetToolFunction().Name)
		assert.Equal(t, "description of test tool", *responsesTools[0].GetToolFunction().Description)
		assert.NotNil(t, responsesTools[0].GetToolFunction().Parameters.GetValue())
	})
}

func TestResponsesAPIChatModelInjectCache(t *testing.T) {
	PatchConvey("not configure", t, func() {
		var (
			cm = &responsesAPIChatModel{}
		)
		arkOpts := &arkOptions{}
		msgs := []*schema.Message{
			{
				Role:    schema.User,
				Content: "Hello",
			},
		}

		reqParams := &responses.ResponsesRequest{}

		in_, err := cm.populateCache(msgs, reqParams, arkOpts)
		assert.Nil(t, err)
		assert.Equal(t, false, *reqParams.Store)
		assert.Len(t, in_, 1)
	})

	PatchConvey("enable cache", t, func() {
		cm := &responsesAPIChatModel{
			cache: &CacheConfig{
				SessionCache: &SessionCacheConfig{
					EnableCache: true,
				},
			},
		}
		arkOpts := &arkOptions{}
		msgs := []*schema.Message{
			{
				Role:    schema.User,
				Content: "Hello",
				Extra: map[string]any{
					keyOfResponseID:            "test-response-id",
					keyOfResponseCacheExpireAt: time.Now().Unix() + 259200,
				},
			},
			{
				Role:    schema.User,
				Content: "World",
			},
		}
		reqParams := &responses.ResponsesRequest{}
		in_, err := cm.populateCache(msgs, reqParams, arkOpts)
		assert.Nil(t, err)
		assert.Equal(t, true, *reqParams.Store)
		assert.Equal(t, "test-response-id", *reqParams.PreviousResponseId)
		assert.Len(t, in_, 1)
		assert.Equal(t, "World", in_[0].Content)
		assert.NotNil(t, reqParams.ExpireAt)
	})
	PatchConvey("option overridden config", t, func() {
		cm := &responsesAPIChatModel{
			cache: &CacheConfig{
				SessionCache: &SessionCacheConfig{
					EnableCache: false,
				},
			},
		}

		contextID := "test-context"
		arkOpts := &arkOptions{
			cache: &CacheOption{
				ContextID: &contextID,
				SessionCache: &SessionCacheConfig{
					EnableCache: true,
				},
			},
		}
		msgs := []*schema.Message{
			{
				Role:    schema.User,
				Content: "Hello",
				Extra: map[string]any{
					keyOfResponseID:            "test-response-id",
					keyOfResponseCacheExpireAt: time.Now().Unix() + 259200,
				},
			},
			{
				Role:    schema.User,
				Content: "World",
			},
		}

		reqParams := &responses.ResponsesRequest{}
		in_, err := cm.populateCache(msgs, reqParams, arkOpts)
		assert.Nil(t, err)
		//assert.Equal(t, initialReqOptsLen+2, len(reqParams.opts))
		assert.Equal(t, true, *reqParams.Store)
		assert.Equal(t, "test-context", *reqParams.PreviousResponseId)
		assert.Len(t, in_, 2)
		assert.NotNil(t, reqParams.ExpireAt)
	})
}

func TestResponsesAPIChatModelReceivedStreamResponse_ResponseCreatedEvent(t *testing.T) {
	cm := &responsesAPIChatModel{}

	PatchConvey("ResponseCreatedEvent", t, func() {
		Mock((*utils.ResponsesStreamReader).Recv).Return(Sequence(&responses.Event{
			Event: &responses.Event_Response{
				Response: &responses.ResponseEvent{
					Response: &responses.ResponseObject{},
				},
			},
		}, nil).Then(nil, io.EOF)).Build()
		mocker := Mock((*responsesAPIChatModel).sendCallbackOutput).Return().Build()
		streamReader := &utils.ResponsesStreamReader{}
		cm.receivedStreamResponse(streamReader, nil, &cacheConfig{Enabled: true}, nil)
		assert.Equal(t, 1, mocker.Times())
	})
}

func TestResponsesAPIChatModelReceivedStreamResponse_ResponseCompletedEvent(t *testing.T) {
	cm := &responsesAPIChatModel{}
	PatchConvey("ResponseCompletedEvent", t, func() {
		Mock((*utils.ResponsesStreamReader).Recv).Return(Sequence(&responses.Event{
			Event: &responses.Event_ResponseCompleted{
				ResponseCompleted: &responses.ResponseCompletedEvent{
					Response: &responses.ResponseObject{
						Usage: &responses.Usage{InputTokensDetails: &responses.InputTokensDetails{}},
					},
				},
			},
		}, nil).Then(nil, io.EOF)).Build()
		mocker := Mock((*responsesAPIChatModel).sendCallbackOutput).Return().Build()
		streamReader := &utils.ResponsesStreamReader{}
		cm.receivedStreamResponse(streamReader, nil, &cacheConfig{Enabled: true}, nil)
		assert.Equal(t, 1, mocker.Times())
	})
}

func TestResponsesAPIChatModelReceivedStreamResponse_ResponseErrorEvent(t *testing.T) {
	cm := &responsesAPIChatModel{}
	PatchConvey("ResponseErrorEvent", t, func() {
		Mock((*utils.ResponsesStreamReader).Recv).Return(Sequence(&responses.Event{
			Event: &responses.Event_Error{
				Error: &responses.ErrorEvent{
					Message: "error msg",
				},
			},
		}, nil).Then(nil, io.EOF)).Build()
		sr, sw := schema.Pipe[*model.CallbackOutput](1)
		streamReader := &utils.ResponsesStreamReader{}
		cm.receivedStreamResponse(streamReader, nil, &cacheConfig{Enabled: true}, sw)

		_, err := sr.Recv()
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "error msg")
	})
}

func TestResponsesAPIChatModelReceivedStreamResponse_ResponseIncompleteEvent(t *testing.T) {

	cm := &responsesAPIChatModel{}
	PatchConvey("ResponseIncompleteEvent", t, func() {
		Mock((*utils.ResponsesStreamReader).Recv).Return(Sequence(&responses.Event{
			Event: &responses.Event_ResponseIncomplete{
				ResponseIncomplete: &responses.ResponseIncompleteEvent{
					Response: &responses.ResponseObject{
						IncompleteDetails: &responses.IncompleteDetails{},
						Usage:             &responses.Usage{InputTokensDetails: &responses.InputTokensDetails{}},
					},
				},
			},
		}, nil).Then(nil, io.EOF)).Build()
		streamReader := &utils.ResponsesStreamReader{}
		mocker := Mock((*responsesAPIChatModel).sendCallbackOutput).Return().Build()

		cm.receivedStreamResponse(streamReader, nil, &cacheConfig{Enabled: true}, nil)

		assert.Equal(t, 1, mocker.Times())
	})

}

func TestResponsesAPIChatModelReceivedStreamResponse_ResponseFailedEvent(t *testing.T) {
	cm := &responsesAPIChatModel{}
	PatchConvey("ResponseFailedEvent", t, func() {
		Mock((*utils.ResponsesStreamReader).Recv).Return(Sequence(&responses.Event{
			Event: &responses.Event_ResponseFailed{
				ResponseFailed: &responses.ResponseFailedEvent{
					Response: &responses.ResponseObject{
						Usage: &responses.Usage{
							InputTokensDetails: &responses.InputTokensDetails{},
						},
					},
				},
			},
		}, nil).Then(nil, io.EOF)).Build()
		streamReader := &utils.ResponsesStreamReader{}
		mocker := Mock((*responsesAPIChatModel).sendCallbackOutput).Return().Build()

		cm.receivedStreamResponse(streamReader, nil, &cacheConfig{Enabled: true}, nil)

		assert.Equal(t, 1, mocker.Times())
	})
}

func TestResponsesAPIChatModelReceivedStreamResponse_Default(t *testing.T) {
	cm := &responsesAPIChatModel{}
	PatchConvey("Default", t, func() {
		Mock((*utils.ResponsesStreamReader).Recv).Return(Sequence(&responses.Event{
			Event: &responses.Event_Text{
				Text: &responses.OutputTextEvent{
					Delta: ptrOf("ok"),
				},
			},
		}, nil).Then(nil, io.EOF)).Build()
		streamReader := &utils.ResponsesStreamReader{}
		mocker := Mock((*responsesAPIChatModel).sendCallbackOutput).Return().Build()

		cm.receivedStreamResponse(streamReader, nil, &cacheConfig{Enabled: true}, nil)

		assert.Equal(t, 1, mocker.Times())

	})
}

func TestResponsesAPIChatModelReceivedStreamResponse_ToolCallMetaMsg(t *testing.T) {
	cm := &responsesAPIChatModel{}
	PatchConvey("ToolCallMetaMsg", t, func() {
		Mock((*utils.ResponsesStreamReader).Recv).Return(Sequence(&responses.Event{
			Event: &responses.Event_Item{
				Item: &responses.ItemEvent{
					Item: &responses.OutputItem{
						Union: &responses.OutputItem_FunctionToolCall{
							FunctionToolCall: &responses.ItemFunctionToolCall{
								Id:     ptrOf("123"),
								CallId: "123",
								Name:   "test",
								Type:   responses.ItemType_function_call,
							},
						},
					},
				},
			},
		}, nil).Then(&responses.Event{
			Event: &responses.Event_FunctionCallArguments{
				FunctionCallArguments: &responses.FunctionCallArgumentsEvent{
					Delta:  ptrOf("arguments"),
					ItemId: "123",
				},
			},
		}, nil).Then(nil, io.EOF)).Build()
		streamReader := &utils.ResponsesStreamReader{}

		mocker := Mock((*responsesAPIChatModel).sendCallbackOutput).To(
			func(sw *schema.StreamWriter[*model.CallbackOutput], reqConf *model.Config, modelName string,
				msg *schema.Message) {
				assert.Equal(t, "123", msg.ToolCalls[0].ID)
				assert.Equal(t, "test", msg.ToolCalls[0].Function.Name)
				assert.Equal(t, "arguments", msg.ToolCalls[0].Function.Arguments)
				assert.Equal(t, "function_call", msg.ToolCalls[0].Type)
			}).Build()

		cache := &cacheConfig{Enabled: true}

		cm.receivedStreamResponse(streamReader, nil, cache, nil)

		assert.Equal(t, 1, mocker.Times())

	})
}

func TestResponsesAPIChatModelHandleGenRequestAndOptions(t *testing.T) {
	cm := &responsesAPIChatModel{
		temperature: ptrOf(float32(1.0)),
		maxTokens:   ptrOf(1),
		model:       "model",
		topP:        ptrOf(float32(1.0)),
		thinking: &arkModel.Thinking{
			Type: arkModel.ThinkingTypeDisabled,
		},
		customHeader: map[string]string{
			"h1": "v1",
		},
		responseFormat: &ResponseFormat{
			Type: arkModel.ResponseFormatJSONSchema,
			JSONSchema: &arkModel.ResponseFormatJSONSchemaJSONSchemaParam{
				Name: "json_schema",
			},
		},
	}

	PatchConvey("vv", t, func() {
		Mock((*responsesAPIChatModel).checkOptions).To(func(mOpts *model.Options, arkOpts *arkOptions) error {
			assert.Equal(t, int(float32(2.0)), int(*mOpts.Temperature))
			assert.Equal(t, 2, *mOpts.MaxTokens)
			assert.Equal(t, int(float32(2.0)), int(*mOpts.TopP))
			assert.Equal(t, "model2", *mOpts.Model)

			assert.Equal(t, arkModel.ThinkingTypeAuto, arkOpts.thinking.Type)
			assert.Len(t, arkOpts.customHeaders, 2)
			assert.Equal(t, "v2", arkOpts.customHeaders["h2"])
			assert.Equal(t, "v3", arkOpts.customHeaders["h3"])

			return nil
		}).Build()

		Mock((*responsesAPIChatModel).populateCache).To(func(in []*schema.Message, respRequest *responses.ResponsesRequest, arkOpts *arkOptions,
		) ([]*schema.Message, error) {
			return in, nil
		}).Build()

		in := []*schema.Message{
			{
				Role:    schema.User,
				Content: "user",
			},
		}

		opts := []model.Option{
			model.WithTemperature(2.0),
			model.WithMaxTokens(2),
			model.WithTopP(2.0),
			model.WithModel("model2"),
			model.WithTools([]*schema.ToolInfo{
				{
					Name: "test tool",
					Desc: "description of test tool",
					ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
						"param": {
							Type:     schema.String,
							Desc:     "description of param1",
							Required: true,
						},
					}),
				},
			}),
			WithThinking(&arkModel.Thinking{Type: arkModel.ThinkingTypeAuto}),
			WithCustomHeader(map[string]string{
				"h2": "v2",
				"h3": "v3",
			}),
		}

		options, specOptions, err := cm.getOptions(opts)
		assert.NoError(t, err)

		reqParams, err := cm.genRequestAndOptions(in, options, specOptions)
		assert.Nil(t, err)
		assert.Equal(t, "model2", reqParams.Model)
		assert.Len(t, reqParams.Input.GetListValue().GetListValue(), 1)
		assert.Equal(t, "user", reqParams.Input.GetListValue().ListValue[0].GetInputMessage().GetContent()[0].GetText().GetText())
		assert.Len(t, reqParams.Tools, 1)
		assert.Equal(t, "test tool", reqParams.Tools[0].GetToolFunction().Name)

		assert.Equal(t, "json_schema", reqParams.Text.Format.GetName())
	})
}

func TestResponsesAPIChatModel_toOpenaiMultiModalContent(t *testing.T) {
	cm := &responsesAPIChatModel{}
	base64Data := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII="
	httpURL := "https://example.com/image.png"

	PatchConvey("Test toOpenaiMultiModalContent Comprehensive", t, func() {
		PatchConvey("Pure Text Content", func() {
			msg := &schema.Message{Role: schema.User, Content: "just text"}
			inputMessage, err := cm.toArkUserRoleItemInputMessage(msg)
			assert.Nil(t, err)
			assert.Equal(t, "just text", inputMessage.Content[0].GetText().GetText())
		})

		PatchConvey("UserInputMultiContent", func() {
			PatchConvey("Success with all types", func() {
				msg := &schema.Message{
					Role:    schema.User,
					Content: "initial text",
					UserInputMultiContent: []schema.MessageInputPart{
						{Type: schema.ChatMessagePartTypeText, Text: " more text"},
						{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{URL: &httpURL}}},
						{Type: schema.ChatMessagePartTypeFileURL, File: &schema.MessageInputFile{MessagePartCommon: schema.MessagePartCommon{URL: &httpURL}}},
						{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{Base64Data: &base64Data, MIMEType: "image/png"}}},
						{Type: schema.ChatMessagePartTypeFileURL, File: &schema.MessageInputFile{MessagePartCommon: schema.MessagePartCommon{Base64Data: &base64Data, MIMEType: "application/pdf"}}},
					},
				}
				inputMessage, err := cm.toArkUserRoleItemInputMessage(msg)
				assert.Nil(t, err)
				assert.Len(t, inputMessage.Content, 5)
			})

			PatchConvey("Error on missing MIMEType for Base64", func() {
				msg := &schema.Message{
					Role: schema.User,
					UserInputMultiContent: []schema.MessageInputPart{
						{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{Base64Data: &base64Data}}},
					},
				}
				_, err := cm.toArkUserRoleItemInputMessage(msg)
				assert.NotNil(t, err)
				assert.ErrorContains(t, err, "message part must have MIMEType when use Base64Data")
			})

			PatchConvey("Error on nil Image", func() {
				msg := &schema.Message{
					Role: schema.User,
					UserInputMultiContent: []schema.MessageInputPart{
						{Type: schema.ChatMessagePartTypeImageURL, Image: nil},
					},
				}
				_, err := cm.toArkUserRoleItemInputMessage(msg)
				assert.NotNil(t, err)
				assert.ErrorContains(t, err, "image field must not be nil")
			})

		})

		PatchConvey("AssistantGenMultiContent", func() {
			PatchConvey("Success with all types", func() {
				msg := &schema.Message{
					Role:    schema.Assistant,
					Content: "assistant text",
					AssistantGenMultiContent: []schema.MessageOutputPart{
						{Type: schema.ChatMessagePartTypeText, Text: " more assistant text"},
					},
				}
				inputMessage, err := cm.toArkAssistantRoleItemInputMessage(msg)
				assert.Nil(t, err)
				assert.Len(t, inputMessage.Content, 1)
			})

			PatchConvey("Error on wrong role", func() {
				msg := &schema.Message{
					Role: schema.User,
					AssistantGenMultiContent: []schema.MessageOutputPart{{
						Type: schema.ChatMessagePartTypeText,
						Text: " more assistant text",
					}},
				}
				_, err := cm.toArkUserRoleItemInputMessage(msg)
				assert.NotNil(t, err)
				assert.ErrorContains(t, err, "if user role, AssistantGenMultiContent cannot be set")
			})

			PatchConvey("Error on nil Image", func() {
				msg := &schema.Message{
					Role: schema.Assistant,
					AssistantGenMultiContent: []schema.MessageOutputPart{
						{Type: schema.ChatMessagePartTypeImageURL, Image: nil},
					},
				}
				_, err := cm.toArkAssistantRoleItemInputMessage(msg)
				assert.NotNil(t, err)

			})

		})

		PatchConvey("MultiContent (Legacy 1)", func() {
			msg := &schema.Message{
				Content: "legacy text",
			}
			inputMessage, err := cm.toArkAssistantRoleItemInputMessage(msg)
			assert.Nil(t, err)
			assert.Len(t, inputMessage.Content, 1)
		})

		PatchConvey("MultiContent (Legacy 2", func() {
			msg := &schema.Message{
				MultiContent: []schema.ChatMessagePart{
					{Type: schema.ChatMessagePartTypeText, Text: " more legacy text"},
					{Type: schema.ChatMessagePartTypeImageURL, ImageURL: &schema.ChatMessageImageURL{URL: httpURL}},
					{Type: schema.ChatMessagePartTypeFileURL, FileURL: &schema.ChatMessageFileURL{URL: httpURL}},
				},
			}
			inputMessage, err := cm.toArkUserRoleItemInputMessage(msg)
			assert.Nil(t, err)
			assert.Len(t, inputMessage.Content, 3)
		})

		PatchConvey("Error on both UserInputMultiContent and AssistantGenMultiContent", func() {
			msg := &schema.Message{
				UserInputMultiContent:    []schema.MessageInputPart{{Type: schema.ChatMessagePartTypeText, Text: "user"}},
				AssistantGenMultiContent: []schema.MessageOutputPart{{Type: schema.ChatMessagePartTypeText, Text: "assistant"}},
			}
			_, err := cm.toArkAssistantRoleItemInputMessage(msg)
			assert.NotNil(t, err)
			assert.ErrorContains(t, err, "if assistant role, UserInputMultiContent cannot be set")
		})
	})
}

func Test_responsesAPIChatModel_handleCompletedStreamEvent(t *testing.T) {
	cm := &responsesAPIChatModel{}
	msg := cm.handleCompletedStreamEvent(&responses.ResponseObject{
		Status: responses.ResponseStatus_completed,
		Usage:  &responses.Usage{InputTokensDetails: &responses.InputTokensDetails{}},
	})
	assert.Equal(t, responses.ResponseStatus_completed.String(), msg.ResponseMeta.FinishReason)

}

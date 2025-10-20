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

	. "github.com/bytedance/mockey"
	openaiOption "github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/packages/ssestream"
	"github.com/openai/openai-go/responses"
	"github.com/stretchr/testify/assert"
	arkModel "github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func TestResponsesAPIChatModelGenerate(t *testing.T) {
	PatchConvey("test Generate", t, func() {
		Mock(callbacks.OnError).Return(context.Background()).Build()
		Mock((*responsesAPIChatModel).genRequestAndOptions).
			Return(responses.ResponseNewParams{}, nil, nil).Build()
		Mock((*responsesAPIChatModel).toCallbackConfig).
			Return(&model.Config{}).Build()
		MockGeneric(callbacks.OnStart[*callbacks.CallbackInput]).Return(context.Background()).Build()
		Mock((*responses.ResponseService).New).
			Return(&responses.Response{}, nil).Build()
		Mock((*responsesAPIChatModel).toOutputMessage).
			Return(&schema.Message{
				Role:    schema.Assistant,
				Content: "assistant",
			}, nil).Build()
		MockGeneric(callbacks.OnEnd[*callbacks.CallbackOutput]).Return(context.Background()).Build()

		cm := &responsesAPIChatModel{}
		msg, err := cm.Generate(context.Background(), []*schema.Message{
			{
				Role:    schema.User,
				Content: "user",
			},
		})
		assert.Nil(t, err)
		assert.Equal(t, "assistant", msg.Content)
	})
}

func TestResponsesAPIChatModelStream(t *testing.T) {
	PatchConvey("test Stream", t, func() {
		ctx := context.Background()
		sr, sw := schema.Pipe[*model.CallbackOutput](1)

		Mock(callbacks.OnError).Return(ctx).Build()
		Mock((*responsesAPIChatModel).genRequestAndOptions).
			Return(responses.ResponseNewParams{}, nil, nil).Build()
		Mock((*responsesAPIChatModel).toCallbackConfig).
			Return(&model.Config{}).Build()
		MockGeneric(callbacks.OnStart[*callbacks.CallbackInput]).Return(context.Background()).Build()
		Mock((*responses.ResponseService).NewStreaming).
			Return(&ssestream.Stream[responses.ResponseStreamEventUnion]{}).Build()
		MockGeneric(schema.Pipe[*model.CallbackOutput]).
			Return(sr, sw).Build()
		Mock((*responsesAPIChatModel).receivedStreamResponse).Return().Build()
		MockGeneric((*ssestream.Stream[responses.ResponseStreamEventUnion]).Err).
			Return(nil).Build()

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
		req := &responses.ResponseNewParams{
			Model: "test-model",
		}
		in := []*schema.Message{}
		err := cm.populateInput(req, in)
		assert.Nil(t, err)
	})

	PatchConvey("user message", t, func() {
		req := &responses.ResponseNewParams{
			Model: "test-model",
		}
		in := []*schema.Message{
			{
				Role:    schema.User,
				Content: "Hello",
			},
		}

		err := cm.populateInput(req, in)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(req.Input.OfInputItemList))

		item := req.Input.OfInputItemList[0]
		assert.Equal(t, responses.EasyInputMessageRoleUser, item.OfMessage.Role)
		assert.Equal(t, "Hello", item.OfMessage.Content.OfString.Value)
	})

	PatchConvey("assistant message", t, func() {
		req := &responses.ResponseNewParams{
			Model: "test-model",
		}
		in := []*schema.Message{
			{
				Role:    schema.Assistant,
				Content: "Hi there",
			},
		}

		err := cm.populateInput(req, in)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(req.Input.OfInputItemList))

		item := req.Input.OfInputItemList[0]
		assert.Equal(t, responses.EasyInputMessageRoleAssistant, item.OfMessage.Role)
		assert.Equal(t, "Hi there", item.OfMessage.Content.OfString.Value)
	})

	PatchConvey("system message", t, func() {
		req := &responses.ResponseNewParams{
			Model: "test-model",
		}
		in := []*schema.Message{
			{
				Role:    schema.System,
				Content: "You are a helpful assistant.",
			},
		}

		err := cm.populateInput(req, in)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(req.Input.OfInputItemList))

		item := req.Input.OfInputItemList[0]
		assert.Equal(t, responses.EasyInputMessageRoleSystem, item.OfMessage.Role)
		assert.Equal(t, "You are a helpful assistant.", item.OfMessage.Content.OfString.Value)
	})

	PatchConvey("tool call", t, func() {
		req := &responses.ResponseNewParams{
			Model: "test-model",
		}
		in := []*schema.Message{
			{
				Role:       schema.Tool,
				ToolCallID: "call_123",
				Content:    "tool output",
			},
		}

		err := cm.populateInput(req, in)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(req.Input.OfInputItemList))

		item := req.Input.OfInputItemList[0]
		assert.Equal(t, "call_123", item.OfFunctionCallOutput.CallID)
		assert.Equal(t, "tool output", item.OfFunctionCallOutput.Output)
	})

	PatchConvey("unknown role", t, func() {
		req := &responses.ResponseNewParams{
			Model: "test-model",
		}
		in := []*schema.Message{
			{
				Role:    "unknown_role",
				Content: "some content",
			},
		}

		err := cm.populateInput(req, in)
		assert.NotNil(t, err)
	})
}

func TestResponsesAPIChatModelToOpenaiMultiModalContent(t *testing.T) {
	cm := &responsesAPIChatModel{}

	PatchConvey("image message", t, func() {
		msg := &schema.Message{
			Role: schema.User,
			MultiContent: []schema.ChatMessagePart{
				{
					Type: schema.ChatMessagePartTypeImageURL,
					ImageURL: &schema.ChatMessageImageURL{
						URL: "https://example.com/image.png",
					},
				},
			},
		}

		content, err := cm.toOpenaiMultiModalContent(msg)
		assert.Nil(t, err)

		contentList := content.OfInputItemContentList
		assert.Equal(t, 1, len(contentList))
		assert.Equal(t, "https://example.com/image.png", contentList[0].OfInputImage.ImageURL.Value)
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

		_, err := cm.toOpenaiMultiModalContent(msg)
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
		openAITools, err := cm.toTools(tools)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(openAITools))
		assert.Equal(t, tools[0].Name, openAITools[0].OfFunction.Name)
		assert.Equal(t, param.NewOpt(tools[0].Desc), openAITools[0].OfFunction.Description)
		assert.NotNil(t, openAITools[0].OfFunction.Parameters["properties"].(map[string]any)["param"])
	})
}

func TestResponsesAPIChatModelInjectCache(t *testing.T) {
	PatchConvey("not configure", t, func() {
		var (
			req     = responses.ResponseNewParams{}
			cm      = &responsesAPIChatModel{}
			reqOpts []openaiOption.RequestOption
		)

		arkOpts := &arkOptions{}
		initialReqOptsLen := len(reqOpts)
		msgs := []*schema.Message{
			{
				Role:    schema.User,
				Content: "Hello",
			},
		}

		in_, newReq, newReqOpts, err := cm.populateCache(msgs, req, arkOpts, reqOpts)
		assert.Nil(t, err)
		assert.Equal(t, param.NewOpt(false), newReq.Store)
		assert.Equal(t, initialReqOptsLen+1, len(newReqOpts))
		assert.Len(t, in_, 1)
	})

	PatchConvey("enable cache and set ttl", t, func() {
		var (
			req     = responses.ResponseNewParams{}
			reqOpts []openaiOption.RequestOption
		)

		cm := &responsesAPIChatModel{
			cache: &CacheConfig{
				SessionCache: &SessionCacheConfig{
					EnableCache: true,
				},
			},
		}

		arkOpts := &arkOptions{}
		initialReqOptsLen := len(reqOpts)
		msgs := []*schema.Message{
			{
				Role:    schema.User,
				Content: "Hello",
				Extra: map[string]any{
					keyOfResponseID:      "test-response-id",
					keyOfResponseCaching: cachingEnabled,
				},
			},
			{
				Role:    schema.User,
				Content: "World",
			},
		}

		in_, newReq, newReqOpts, err := cm.populateCache(msgs, req, arkOpts, reqOpts)
		assert.Nil(t, err)
		assert.Equal(t, initialReqOptsLen+2, len(newReqOpts))
		assert.Equal(t, param.NewOpt(true), newReq.Store)
		assert.Equal(t, "test-response-id", newReq.PreviousResponseID.Value)
		assert.Len(t, in_, 1)
		assert.Equal(t, "World", in_[0].Content)
	})

	PatchConvey("option overridden config", t, func() {
		var (
			req     = responses.ResponseNewParams{}
			reqOpts []openaiOption.RequestOption
		)

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

		initialReqOptsLen := len(reqOpts)
		msgs := []*schema.Message{
			{
				Role:    schema.User,
				Content: "Hello",
				Extra: map[string]any{
					keyOfResponseID:      "test-response-id",
					keyOfResponseCaching: cachingEnabled,
				},
			},
			{
				Role:    schema.User,
				Content: "World",
			},
		}

		in_, newReq, newReqOpts, err := cm.populateCache(msgs, req, arkOpts, reqOpts)
		assert.Nil(t, err)
		assert.Equal(t, initialReqOptsLen+2, len(newReqOpts))
		assert.Equal(t, param.NewOpt(true), newReq.Store)
		assert.Equal(t, "test-context", newReq.PreviousResponseID.Value)
		assert.Len(t, in_, 2)
	})
}

func TestResponsesAPIChatModelReceivedStreamResponse_ResponseCreatedEvent(t *testing.T) {
	cm := &responsesAPIChatModel{}
	streamResp := &ssestream.Stream[responses.ResponseStreamEventUnion]{}
	PatchConvey("ResponseCreatedEvent", t, func() {
		MockGeneric((*ssestream.Stream[responses.ResponseStreamEventUnion]).Next).
			Return(Sequence(true).Then(false)).Build()
		MockGeneric((*ssestream.Stream[responses.ResponseStreamEventUnion]).Current).
			Return(responses.ResponseStreamEventUnion{}).Build()
		Mock((*responsesAPIChatModel).isAddedToolCall).Return(nil, false).Build()
		Mock(responses.ResponseStreamEventUnion.AsAny).
			Return(responses.ResponseCreatedEvent{}).Build()
		mocker := Mock((*responsesAPIChatModel).sendCallbackOutput).Return().Build()

		cm.receivedStreamResponse(streamResp, nil, true, nil)
		assert.Equal(t, 1, mocker.Times())
	})
}

func TestResponsesAPIChatModelReceivedStreamResponse_ResponseCompletedEvent(t *testing.T) {
	cm := &responsesAPIChatModel{}
	streamResp := &ssestream.Stream[responses.ResponseStreamEventUnion]{}
	PatchConvey("ResponseCompletedEvent", t, func() {
		MockGeneric((*ssestream.Stream[responses.ResponseStreamEventUnion]).Next).
			Return(true).Build()
		MockGeneric((*ssestream.Stream[responses.ResponseStreamEventUnion]).Current).
			Return(responses.ResponseStreamEventUnion{}).Build()
		Mock((*responsesAPIChatModel).isAddedToolCall).Return(nil, false).Build()
		mocker := Mock((*responsesAPIChatModel).sendCallbackOutput).Return().Build()
		Mock(responses.ResponseStreamEventUnion.AsAny).
			Return(responses.ResponseCompletedEvent{}).Build()
		Mock((*responsesAPIChatModel).handleCompletedStreamEvent).Return(&schema.Message{}).Build()

		cm.receivedStreamResponse(streamResp, nil, true, nil)
		assert.Equal(t, 1, mocker.Times())
	})
}

func TestResponsesAPIChatModelReceivedStreamResponse_ResponseErrorEvent(t *testing.T) {
	cm := &responsesAPIChatModel{}
	streamResp := &ssestream.Stream[responses.ResponseStreamEventUnion]{}
	PatchConvey("ResponseErrorEvent", t, func() {
		MockGeneric((*ssestream.Stream[responses.ResponseStreamEventUnion]).Next).
			Return(true).Build()
		MockGeneric((*ssestream.Stream[responses.ResponseStreamEventUnion]).Current).
			Return(responses.ResponseStreamEventUnion{}).Build()
		Mock((*responsesAPIChatModel).isAddedToolCall).Return(nil, false).Build()
		mocker := MockGeneric((*schema.StreamWriter[*model.CallbackOutput]).Send).Return(false).Build()
		Mock(responses.ResponseStreamEventUnion.AsAny).
			Return(responses.ResponseErrorEvent{}).Build()

		Mock((*responsesAPIChatModel).handleCompletedStreamEvent).Return(&schema.Message{}).Build()

		cm.receivedStreamResponse(streamResp, nil, true, nil)
		assert.Equal(t, 1, mocker.Times())
	})
}

func TestResponsesAPIChatModelReceivedStreamResponse_ResponseIncompleteEvent(t *testing.T) {
	cm := &responsesAPIChatModel{}
	streamResp := &ssestream.Stream[responses.ResponseStreamEventUnion]{}
	PatchConvey("ResponseIncompleteEvent", t, func() {
		MockGeneric((*ssestream.Stream[responses.ResponseStreamEventUnion]).Next).
			Return(Sequence(true).Then(false)).Build()
		MockGeneric((*ssestream.Stream[responses.ResponseStreamEventUnion]).Current).
			Return(responses.ResponseStreamEventUnion{}).Build()
		Mock((*responsesAPIChatModel).isAddedToolCall).Return(nil, false).Build()
		mocker := Mock((*responsesAPIChatModel).sendCallbackOutput).Return().Build()
		Mock(responses.ResponseStreamEventUnion.AsAny).
			Return(responses.ResponseIncompleteEvent{}).Build()
		Mock((*responsesAPIChatModel).handleIncompleteStreamEvent).Return(&schema.Message{}).Build()

		cm.receivedStreamResponse(streamResp, nil, true, nil)
		assert.Equal(t, 1, mocker.Times())
	})
}

func TestResponsesAPIChatModelReceivedStreamResponse_ResponseFailedEvent(t *testing.T) {
	cm := &responsesAPIChatModel{}
	streamResp := &ssestream.Stream[responses.ResponseStreamEventUnion]{}
	PatchConvey("ResponseFailedEvent", t, func() {
		MockGeneric((*ssestream.Stream[responses.ResponseStreamEventUnion]).Next).
			Return(true).Build()
		MockGeneric((*ssestream.Stream[responses.ResponseStreamEventUnion]).Current).
			Return(responses.ResponseStreamEventUnion{}).Build()
		Mock((*responsesAPIChatModel).isAddedToolCall).Return(nil, false).Build()
		mocker := Mock((*responsesAPIChatModel).sendCallbackOutput).Return().Build()
		Mock(responses.ResponseStreamEventUnion.AsAny).
			Return(responses.ResponseFailedEvent{}).Build()
		Mock((*responsesAPIChatModel).handleFailedStreamEvent).Return(&schema.Message{}).Build()

		cm.receivedStreamResponse(streamResp, nil, true, nil)
		assert.Equal(t, 1, mocker.Times())
	})
}

func TestResponsesAPIChatModelReceivedStreamResponse_Default(t *testing.T) {
	cm := &responsesAPIChatModel{}
	streamResp := &ssestream.Stream[responses.ResponseStreamEventUnion]{}
	PatchConvey("Default", t, func() {
		MockGeneric((*ssestream.Stream[responses.ResponseStreamEventUnion]).Next).
			Return(Sequence(true).Then(false)).Build()
		MockGeneric((*ssestream.Stream[responses.ResponseStreamEventUnion]).Current).
			Return(responses.ResponseStreamEventUnion{}).Build()
		Mock((*responsesAPIChatModel).isAddedToolCall).Return(nil, false).Build()
		Mock(responses.ResponseStreamEventUnion.AsAny).
			Return(responses.ResponseTextDeltaEvent{}).Build()
		mocker := Mock((*responsesAPIChatModel).sendCallbackOutput).Return().Build()
		Mock((*responsesAPIChatModel).handleDeltaStreamEvent).Return(&schema.Message{}).Build()

		cm.receivedStreamResponse(streamResp, nil, true, nil)
		assert.Equal(t, 1, mocker.Times())
	})
}

func TestResponsesAPIChatModelReceivedStreamResponse_ToolCallMetaMsg(t *testing.T) {
	cm := &responsesAPIChatModel{}
	streamResp := &ssestream.Stream[responses.ResponseStreamEventUnion]{}
	PatchConvey("toolCallMetaMsg", t, func() {
		MockGeneric((*ssestream.Stream[responses.ResponseStreamEventUnion]).Next).
			Return(Sequence(true).Then(true).Then(false)).Build()
		MockGeneric((*ssestream.Stream[responses.ResponseStreamEventUnion]).Current).
			Return(responses.ResponseStreamEventUnion{}).Build()
		Mock((*responsesAPIChatModel).isAddedToolCall).Return(
			Sequence(
				&schema.Message{
					Role: schema.Assistant,
					ToolCalls: []schema.ToolCall{
						{
							ID:   "123",
							Type: "function",
							Function: schema.FunctionCall{
								Name:      "test",
								Arguments: "test",
							},
						},
					},
				}, true).
				Then(nil, false)).Build()
		Mock(responses.ResponseStreamEventUnion.AsAny).
			Return(responses.ResponseTextDeltaEvent{}).Build()
		Mock((*responsesAPIChatModel).handleDeltaStreamEvent).Return(&schema.Message{
			ToolCalls: []schema.ToolCall{
				{
					Function: schema.FunctionCall{
						Arguments: "arguments",
					},
				},
			},
		}).Build()
		mocker := Mock((*responsesAPIChatModel).sendCallbackOutput).To(
			func(sw *schema.StreamWriter[*model.CallbackOutput], reqConf *model.Config,
				msg *schema.Message) {
				assert.Equal(t, "123", msg.ToolCalls[0].ID)
				assert.Equal(t, "test", msg.ToolCalls[0].Function.Name)
				assert.Equal(t, "arguments", msg.ToolCalls[0].Function.Arguments)
				assert.Equal(t, "function", msg.ToolCalls[0].Type)
			}).Build()

		cm.receivedStreamResponse(streamResp, nil, true, nil)
		assert.Equal(t, 1, mocker.Times())
	})
}

func TestResponsesAPIChatModelHandleDeltaStreamEvent(t *testing.T) {
	cm := &responsesAPIChatModel{}

	PatchConvey("ResponseTextDeltaEvent", t, func() {
		chunk := responses.ResponseTextDeltaEvent{
			Delta: "test",
		}
		msg := cm.handleDeltaStreamEvent(chunk)
		assert.Equal(t, chunk.Delta, msg.Content)
	})

	PatchConvey("ResponseFunctionCallArgumentsDeltaEvent", t, func() {
		chunk := responses.ResponseFunctionCallArgumentsDeltaEvent{
			Delta: "test",
		}
		msg := cm.handleDeltaStreamEvent(chunk)
		assert.Equal(t, chunk.Delta, msg.ToolCalls[0].Function.Arguments)
	})

	PatchConvey("ResponseReasoningSummaryTextDeltaEvent", t, func() {
		chunk := responses.ResponseReasoningSummaryTextDeltaEvent{
			Delta: "test",
		}
		msg := cm.handleDeltaStreamEvent(chunk)
		assert.Equal(t, chunk.Delta, msg.ReasoningContent)
		assert.Equal(t, chunk.Delta, msg.Extra[keyOfReasoningContent])
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
	}

	PatchConvey("", t, func() {
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

		Mock((*responsesAPIChatModel).populateCache).To(func(in []*schema.Message, req responses.ResponseNewParams, arkOpts *arkOptions,
			reqOpts []openaiOption.RequestOption) ([]*schema.Message, responses.ResponseNewParams, []openaiOption.RequestOption, error) {
			return in, req, reqOpts, nil
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

		req, reqOpts, err := cm.genRequestAndOptions(in, options, specOptions)
		assert.Nil(t, err)
		assert.Equal(t, "model2", req.Model)
		assert.Len(t, req.Input.OfInputItemList, 1)
		assert.Equal(t, "user", req.Input.OfInputItemList[0].OfMessage.Content.OfString.Value)
		assert.Len(t, req.Tools, 1)
		assert.Equal(t, "test tool", req.Tools[0].OfFunction.Name)
		assert.Len(t, reqOpts, 3)
	})
}

func TestResponsesAPIChatModelIsAddedToolCall(t *testing.T) {
	cm := &responsesAPIChatModel{}
	PatchConvey("", t, func() {
		Mock(responses.ResponseStreamEventUnion.AsAny).Return(
			responses.ResponseOutputItemAddedEvent{},
		).Build()
		Mock(responses.ResponseOutputItemUnion.AsAny).Return(
			responses.ResponseFunctionToolCall{
				CallID: "123",
				Type:   "function_call",
				Name:   "name",
			},
		).Build()

		msg, ok := cm.isAddedToolCall(responses.ResponseStreamEventUnion{})
		assert.True(t, ok)
		assert.Equal(t, "123", msg.ToolCalls[0].ID)
		assert.Equal(t, "function_call", msg.ToolCalls[0].Type)
		assert.Equal(t, "name", msg.ToolCalls[0].Function.Name)
	})
}

func TestGetArkRequestID(t *testing.T) {
	item := responses.EasyInputMessageContentUnionParam{}
	if item.OfString.Valid() {
		t.Log("eq")
	}
}

func TestResponsesAPIChatModel_toOpenaiMultiModalContent(t *testing.T) {
	cm := &responsesAPIChatModel{}
	base64Data := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII="
	httpURL := "https://example.com/image.png"

	PatchConvey("Test toOpenaiMultiModalContent Comprehensive", t, func() {
		PatchConvey("Pure Text Content", func() {
			msg := &schema.Message{Role: schema.User, Content: "just text"}
			content, err := cm.toOpenaiMultiModalContent(msg)
			assert.Nil(t, err)
			assert.Equal(t, "just text", content.OfString.Value)
		})

		PatchConvey("UserInputMultiContent", func() {
			PatchConvey("Success with all types", func() {
				msg := &schema.Message{
					Role:    schema.User,
					Content: "initial text",
					UserInputMultiContent: []schema.MessageInputPart{
						{Type: schema.ChatMessagePartTypeText, Text: " more text"},
						{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{URL: &httpURL}}},
						{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{Base64Data: &base64Data, MIMEType: "image/png"}}},
					},
				}
				content, err := cm.toOpenaiMultiModalContent(msg)
				assert.Nil(t, err)
				assert.Len(t, content.OfInputItemContentList, 4)
			})

			PatchConvey("Error on missing MIMEType for Base64", func() {
				msg := &schema.Message{
					Role: schema.User,
					UserInputMultiContent: []schema.MessageInputPart{
						{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{Base64Data: &base64Data}}},
					},
				}
				_, err := cm.toOpenaiMultiModalContent(msg)
				assert.NotNil(t, err)
				assert.ErrorContains(t, err, "image part must have MIMEType when use Base64Data")
			})

			PatchConvey("Error on nil Image", func() {
				msg := &schema.Message{
					Role: schema.User,
					UserInputMultiContent: []schema.MessageInputPart{
						{Type: schema.ChatMessagePartTypeImageURL, Image: nil},
					},
				}
				_, err := cm.toOpenaiMultiModalContent(msg)
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
						{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageOutputImage{MessagePartCommon: schema.MessagePartCommon{URL: &httpURL}}},
						{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageOutputImage{MessagePartCommon: schema.MessagePartCommon{Base64Data: &base64Data, MIMEType: "image/png"}}},
					},
				}
				content, err := cm.toOpenaiMultiModalContent(msg)
				assert.Nil(t, err)
				assert.Len(t, content.OfInputItemContentList, 4)
			})

			PatchConvey("Error on wrong role", func() {
				msg := &schema.Message{
					Role:                     schema.User,
					AssistantGenMultiContent: []schema.MessageOutputPart{{}},
				}
				_, err := cm.toOpenaiMultiModalContent(msg)
				assert.NotNil(t, err)
				assert.ErrorContains(t, err, "assistant gen multi content only support assistant role")
			})

			PatchConvey("Error on nil Image", func() {
				msg := &schema.Message{
					Role: schema.Assistant,
					AssistantGenMultiContent: []schema.MessageOutputPart{
						{Type: schema.ChatMessagePartTypeImageURL, Image: nil},
					},
				}
				_, err := cm.toOpenaiMultiModalContent(msg)
				assert.NotNil(t, err)
				assert.ErrorContains(t, err, "image field must not be nil")
			})

			PatchConvey("Error on missing MIMEType for Base64", func() {
				msg := &schema.Message{
					Role: schema.Assistant,
					AssistantGenMultiContent: []schema.MessageOutputPart{
						{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageOutputImage{MessagePartCommon: schema.MessagePartCommon{Base64Data: &base64Data}}},
					},
				}
				_, err := cm.toOpenaiMultiModalContent(msg)
				assert.NotNil(t, err)
				assert.ErrorContains(t, err, "image part must have MIMEType when use Base64Data")
			})
		})

		PatchConvey("MultiContent (Legacy)", func() {
			msg := &schema.Message{
				Content: "legacy text",
				MultiContent: []schema.ChatMessagePart{
					{Type: schema.ChatMessagePartTypeText, Text: " more legacy text"},
					{Type: schema.ChatMessagePartTypeImageURL, ImageURL: &schema.ChatMessageImageURL{URL: httpURL}},
				},
			}
			content, err := cm.toOpenaiMultiModalContent(msg)
			assert.Nil(t, err)
			assert.Len(t, content.OfInputItemContentList, 3)
		})

		PatchConvey("Error on both UserInputMultiContent and AssistantGenMultiContent", func() {
			msg := &schema.Message{
				UserInputMultiContent:    []schema.MessageInputPart{{Type: schema.ChatMessagePartTypeText, Text: "user"}},
				AssistantGenMultiContent: []schema.MessageOutputPart{{Type: schema.ChatMessagePartTypeText, Text: "assistant"}},
			}
			_, err := cm.toOpenaiMultiModalContent(msg)
			assert.NotNil(t, err)
			assert.ErrorContains(t, err, "a message cannot contain both UserInputMultiContent and AssistantGenMultiContent")
		})
	})
}

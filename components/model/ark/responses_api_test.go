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
		MockGeneric((*ssestream.Stream[responses.ResponseStreamEventUnion]).Next).
			Return(Sequence(true).Then(true).Then(false)).Build()
		MockGeneric((*ssestream.Stream[responses.ResponseStreamEventUnion]).Current).
			Return(responses.ResponseStreamEventUnion{}).Build()
		Mock((*responsesAPIChatModel).handleStreamEvent).
			To(func(eventUnion responses.ResponseStreamEventUnion, mConf *model.Config,
				sw *schema.StreamWriter[*model.CallbackOutput]) bool {
				sw.Send(&model.CallbackOutput{
					Message: &schema.Message{
						Role:    schema.Assistant,
						Content: "1",
					},
				}, nil)
				return true
			}).Build()
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
	initialReq := responses.ResponseNewParams{
		Model: "test-model",
	}

	PatchConvey("empty input message", t, func() {
		in := []*schema.Message{}
		req, err := cm.injectInput(initialReq, in)
		assert.Nil(t, err)
		assert.Equal(t, initialReq, req)
	})

	PatchConvey("user message", t, func() {
		in := []*schema.Message{
			{
				Role:    schema.User,
				Content: "Hello",
			},
		}

		req, err := cm.injectInput(initialReq, in)
		assert.Nil(t, err)
		assert.Equal(t, initialReq.Model, req.Model)
		assert.Equal(t, 1, len(req.Input.OfInputItemList))

		item := req.Input.OfInputItemList[0]
		assert.Equal(t, responses.EasyInputMessageRoleUser, item.OfMessage.Role)
		assert.Equal(t, "Hello", item.OfMessage.Content.OfString.Value)
	})

	PatchConvey("assistant message", t, func() {
		in := []*schema.Message{
			{
				Role:    schema.Assistant,
				Content: "Hi there",
			},
		}

		req, err := cm.injectInput(initialReq, in)
		assert.Nil(t, err)
		assert.Equal(t, initialReq.Model, req.Model)
		assert.Equal(t, 1, len(req.Input.OfInputItemList))

		item := req.Input.OfInputItemList[0]
		assert.Equal(t, responses.EasyInputMessageRoleAssistant, item.OfMessage.Role)
		assert.Equal(t, "Hi there", item.OfMessage.Content.OfString.Value)
	})

	PatchConvey("system message", t, func() {
		in := []*schema.Message{
			{
				Role:    schema.System,
				Content: "You are a helpful assistant.",
			},
		}

		req, err := cm.injectInput(initialReq, in)
		assert.Nil(t, err)
		assert.Equal(t, initialReq.Model, req.Model)
		assert.Equal(t, 1, len(req.Input.OfInputItemList))

		item := req.Input.OfInputItemList[0]
		assert.Equal(t, responses.EasyInputMessageRoleDeveloper, item.OfMessage.Role)
		assert.Equal(t, "You are a helpful assistant.", item.OfMessage.Content.OfString.Value)
	})

	PatchConvey("tool call", t, func() {
		in := []*schema.Message{
			{
				Role:       schema.Tool,
				ToolCallID: "call_123",
				Content:    "tool output",
			},
		}

		req, err := cm.injectInput(initialReq, in)
		assert.Nil(t, err)
		assert.Equal(t, initialReq.Model, req.Model)
		assert.Equal(t, 1, len(req.Input.OfInputItemList))

		item := req.Input.OfInputItemList[0]
		assert.Equal(t, "call_123", item.OfFunctionCallOutput.CallID)
		assert.Equal(t, "tool output", item.OfFunctionCallOutput.Output)
	})

	PatchConvey("unknown role", t, func() {
		in := []*schema.Message{
			{
				Role:    "unknown_role",
				Content: "some content",
			},
		}

		_, err := cm.injectInput(initialReq, in)
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
						URL: "http://example.com/image.png",
					},
				},
			},
		}

		content, err := cm.toOpenaiMultiModalContent(msg)
		assert.Nil(t, err)

		contentList := content.OfInputItemContentList
		assert.Equal(t, 1, len(contentList))
		assert.Equal(t, "http://example.com/image.png", contentList[0].OfInputImage.ImageURL.Value)
	})

	PatchConvey("text and file message", t, func() {
		msg := &schema.Message{
			Role:    schema.User,
			Content: "Here is the file.",
			MultiContent: []schema.ChatMessagePart{
				{
					Type: schema.ChatMessagePartTypeFileURL,
					FileURL: &schema.ChatMessageFileURL{
						URL: "http://example.com/file.pdf",
					},
				},
			},
		}

		content, err := cm.toOpenaiMultiModalContent(msg)
		assert.Nil(t, err)

		contentList := content.OfInputItemContentList
		assert.Equal(t, 2, len(contentList))
		assert.Equal(t, "Here is the file.", contentList[0].OfInputText.Text)
		assert.Equal(t, "http://example.com/file.pdf", contentList[1].OfInputFile.FileURL.Value)
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

		newReq, newReqOpts, err := cm.injectCache(req, arkOpts, reqOpts)
		assert.Nil(t, err)
		assert.Equal(t, param.NewOpt(false), newReq.Store)
		assert.Equal(t, initialReqOptsLen+1, len(newReqOpts))
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
					TTL:         ptrOf(3600),
				},
			},
		}

		arkOpts := &arkOptions{}
		initialReqOptsLen := len(reqOpts)

		newReq, newReqOpts, err := cm.injectCache(req, arkOpts, reqOpts)
		assert.Nil(t, err)
		assert.Equal(t, initialReqOptsLen+2, len(newReqOpts))
		assert.Equal(t, param.NewOpt(true), newReq.Store)
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

		optTTL := 7200
		contextID := "test-context"
		persist := true
		arkOpts := &arkOptions{
			cache: &CacheOption{
				ContextID: &contextID,
				SessionCacheOption: &SessionCacheOption{
					PersistCurrentContext: &persist,
					TTL:                   &optTTL,
				},
			},
		}

		initialReqOptsLen := len(reqOpts)

		newReq, newReqOpts, err := cm.injectCache(req, arkOpts, reqOpts)
		assert.Nil(t, err)
		assert.Equal(t, initialReqOptsLen+2, len(newReqOpts))
		assert.Equal(t, param.NewOpt(true), newReq.Store)
		assert.Equal(t, contextID, newReq.PreviousResponseID.Value)
	})
}

func TestResponsesAPIChatModelHandleStreamEvent(t *testing.T) {
	cm := &responsesAPIChatModel{}

	PatchConvey("ResponseCreatedEvent", t, func() {
		Mock((*responsesAPIChatModel).sendCallbackOutput).Return().Build()
		needContinue := cm.handleStreamEvent(responses.ResponseStreamEventUnion{
			Type: "response.created",
		}, nil, nil)
		assert.True(t, needContinue)
	})

	PatchConvey("ResponseOutputItemDoneEvent", t, func() {
		Mock((*responsesAPIChatModel).sendCallbackOutput).Return().Build()
		Mock((*responsesAPIChatModel).handleOutputItemDoneStreamEvent).Return(&schema.Message{}).Build()
		needContinue := cm.handleStreamEvent(responses.ResponseStreamEventUnion{
			Type: "response.output_item.done",
		}, nil, nil)
		assert.True(t, needContinue)
	})

	PatchConvey("ResponseCompletedEvent", t, func() {
		Mock((*responsesAPIChatModel).sendCallbackOutput).Return().Build()
		Mock((*responsesAPIChatModel).handleOutputItemDoneStreamEvent).Return(&schema.Message{}).Build()
		needContinue := cm.handleStreamEvent(responses.ResponseStreamEventUnion{
			Type: "response.output_item.done",
		}, nil, nil)
		assert.True(t, needContinue)
	})

	PatchConvey("ResponseErrorEvent", t, func() {
		MockGeneric((*schema.StreamWriter[*model.CallbackOutput]).Send).Return(false).Build()
		needContinue := cm.handleStreamEvent(responses.ResponseStreamEventUnion{
			Type: "error",
		}, nil, nil)
		assert.False(t, needContinue)
	})

	PatchConvey("ResponseIncompleteEvent", t, func() {
		Mock((*responsesAPIChatModel).sendCallbackOutput).Return().Build()
		Mock((*responsesAPIChatModel).handleIncompleteStreamEvent).Return(&schema.Message{}).Build()
		needContinue := cm.handleStreamEvent(responses.ResponseStreamEventUnion{
			Type: "response.incomplete",
		}, nil, nil)
		assert.False(t, needContinue)
	})

	PatchConvey("ResponseFailedEvent", t, func() {
		Mock((*responsesAPIChatModel).sendCallbackOutput).Return().Build()
		Mock((*responsesAPIChatModel).handleFailedStreamEvent).Return(&schema.Message{}).Build()
		needContinue := cm.handleStreamEvent(responses.ResponseStreamEventUnion{
			Type: "response.failed",
		}, nil, nil)
		assert.False(t, needContinue)
	})

	PatchConvey("Default", t, func() {
		Mock((*responsesAPIChatModel).sendCallbackOutput).Return().Build()
		Mock((*responsesAPIChatModel).handleDeltaStreamEvent).Return(&schema.Message{}).Build()
		needContinue := cm.handleStreamEvent(responses.ResponseStreamEventUnion{
			Type: "response.output_text.delta",
		}, nil, nil)
		assert.True(t, needContinue)
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
		assert.Equal(t, chunk.Delta, msg.Content)
	})

	PatchConvey("ResponseReasoningSummaryTextDeltaEvent", t, func() {
		chunk := responses.ResponseReasoningSummaryTextDeltaEvent{
			Delta: "test",
		}
		msg := cm.handleDeltaStreamEvent(chunk)
		assert.Equal(t, chunk.Delta, msg.Content)
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

		Mock((*responsesAPIChatModel).injectCache).To(func(req responses.ResponseNewParams, arkOpts *arkOptions,
			reqOpts []openaiOption.RequestOption) (responses.ResponseNewParams, []openaiOption.RequestOption, error) {
			return req, reqOpts, nil
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

		req, reqOpts, err := cm.genRequestAndOptions(in, opts...)
		assert.Nil(t, err)
		assert.Equal(t, "model2", req.Model)
		assert.Len(t, req.Input.OfInputItemList, 1)
		assert.Equal(t, "user", req.Input.OfInputItemList[0].OfMessage.Content.OfString.Value)
		assert.Len(t, req.Tools, 1)
		assert.Equal(t, "test tool", req.Tools[0].OfFunction.Name)
		assert.Len(t, reqOpts, 3)
	})
}

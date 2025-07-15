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
	"github.com/openai/openai-go/packages/ssestream"
	"github.com/openai/openai-go/responses"
	"github.com/stretchr/testify/assert"

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

func Test_responsesAPIChatModel_injectInput(t *testing.T) {
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

	PatchConvey("user message", func() {
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

func TestToOpenaiMultiModalContent(t *testing.T) {
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

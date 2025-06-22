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

package gemini

import (
	"context"
	"io"
	"iter"
	"testing"

	"github.com/bytedance/mockey"
	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/schema"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"google.golang.org/genai"
)

func TestGemini(t *testing.T) {
	ctx := context.Background()
	mockClient := &genai.Client{}
	model, err := NewChatModel(ctx, &Config{
		Client: mockClient,
		Model:  "gemini-pro",
	})
	assert.Nil(t, err)
	mockey.PatchConvey("common", t, func() {
		mockChat := &genai.Chat{}
		defer mockey.Mock((*genai.Chats).Create).Return(mockChat, nil).Build().UnPatch()

		defer mockey.Mock((*genai.Chat).SendMessage).Return(&genai.GenerateContentResponse{
			Candidates: []*genai.Candidate{
				{
					Content: &genai.Content{
						Role: "model",
						Parts: []*genai.Part{
							{Text: "Hello, how can I help you?"},
						},
					},
				},
			},
		}, nil).Build().UnPatch()

		resp, err := model.Generate(ctx, []*schema.Message{
			{
				Role:    schema.User,
				Content: "Hi",
			},
		})
		assert.NoError(t, err)
		assert.Equal(t, "Hello, how can I help you?", resp.Content)
		assert.Equal(t, schema.Assistant, resp.Role)
	})
	mockey.PatchConvey("stream", t, func() {
		mockChat := &genai.Chat{}
		defer mockey.Mock((*genai.Chats).Create).Return(mockChat, nil).Build().UnPatch()

		respList := []*genai.GenerateContentResponse{
			{Candidates: []*genai.Candidate{{
				Content: &genai.Content{
					Role: "model",
					Parts: []*genai.Part{
						{Text: "Hello,"},
					},
				},
			}}},
			{Candidates: []*genai.Candidate{{
				Content: &genai.Content{
					Role: "model",
					Parts: []*genai.Part{
						{Text: " how can I "},
					},
				},
			}}},
			{Candidates: []*genai.Candidate{{
				Content: &genai.Content{
					Role: "model",
					Parts: []*genai.Part{
						{Text: "help you?"},
					},
				},
			}}},
		}

		// Mock SendMessageStream to return an iterator function
		defer mockey.Mock((*genai.Chat).SendMessageStream).To(func(ctx context.Context, parts ...genai.Part) iter.Seq2[*genai.GenerateContentResponse, error] {
			return func(yield func(*genai.GenerateContentResponse, error) bool) {
				for _, resp := range respList {
					if !yield(resp, nil) {
						return
					}
				}
				// Signal completion
				yield(nil, io.EOF)
			}
		}).Build().UnPatch()

		streamResp, err := model.Stream(ctx, []*schema.Message{
			{
				Role:    schema.User,
				Content: "Hi",
			},
		}, WithTopK(0), WithResponseSchema(&openapi3.Schema{
			Type: openapi3.TypeString,
			Enum: []any{"1", "2"},
		}))
		assert.NoError(t, err)
		var respContent string
		for {
			resp, err := streamResp.Recv()
			if err == io.EOF {
				break
			}
			assert.NoError(t, err)
			respContent += resp.Content
		}
		assert.Equal(t, "Hello, how can I help you?", respContent)
	})

	mockey.PatchConvey("structure", t, func() {
		mockChat := &genai.Chat{}
		defer mockey.Mock((*genai.Chats).Create).Return(mockChat, nil).Build().UnPatch()

		responseSchema := &openapi3.Schema{
			Type: openapi3.TypeObject,
			Properties: map[string]*openapi3.SchemaRef{
				"name": {
					Value: &openapi3.Schema{
						Type: openapi3.TypeString,
					},
				},
				"age": {
					Value: &openapi3.Schema{
						Type: openapi3.TypeInteger,
					},
				},
			},
		}
		model.responseSchema = responseSchema

		defer mockey.Mock((*genai.Chat).SendMessage).Return(&genai.GenerateContentResponse{
			Candidates: []*genai.Candidate{
				{
					Content: &genai.Content{
						Role: "model",
						Parts: []*genai.Part{
							{Text: `{"name":"John","age":25}`},
						},
					},
				},
			},
		}, nil).Build().UnPatch()

		resp, err := model.Generate(ctx, []*schema.Message{
			{
				Role:    schema.User,
				Content: "Get user info",
			},
		})

		assert.NoError(t, err)
		assert.Equal(t, `{"name":"John","age":25}`, resp.Content)
	})

	mockey.PatchConvey("function", t, func() {
		mockChat := &genai.Chat{}
		defer mockey.Mock((*genai.Chats).Create).Return(mockChat, nil).Build().UnPatch()

		err = model.BindTools([]*schema.ToolInfo{
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

		defer mockey.Mock((*genai.Chat).SendMessage).Return(&genai.GenerateContentResponse{
			Candidates: []*genai.Candidate{
				{
					Content: &genai.Content{
						Role: "model",
						Parts: []*genai.Part{
							{
								FunctionCall: &genai.FunctionCall{
									Name: "get_weather",
									Args: map[string]interface{}{
										"city": "Beijing",
									},
								},
							},
						},
					},
				},
			},
		}, nil).Build().UnPatch()

		resp, err := model.Generate(ctx, []*schema.Message{
			{
				Role:    schema.User,
				Content: "What's the weather in Beijing?",
			},
		})

		assert.NoError(t, err)
		assert.Len(t, resp.ToolCalls, 1)
		assert.Equal(t, "get_weather", resp.ToolCalls[0].Function.Name)

		var args map[string]interface{}
		err = sonic.UnmarshalString(resp.ToolCalls[0].Function.Arguments, &args)
		assert.NoError(t, err)
		assert.Equal(t, "Beijing", args["city"])
	})

	mockey.PatchConvey("media", t, func() {
		mockChat := &genai.Chat{}
		defer mockey.Mock((*genai.Chats).Create).Return(mockChat, nil).Build().UnPatch()

		defer mockey.Mock((*genai.Chat).SendMessage).Return(&genai.GenerateContentResponse{
			Candidates: []*genai.Candidate{
				{
					Content: &genai.Content{
						Role: "model",
						Parts: []*genai.Part{
							{Text: "I see a beautiful sunset image"},
						},
					},
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
							URI:      "https://example.com/sunset.jpg",
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

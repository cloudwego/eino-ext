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
	"testing"

	"github.com/bytedance/mockey"
	"github.com/bytedance/sonic"
	"github.com/eino-contrib/jsonschema"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	orderedmap "github.com/wk8/go-ordered-map/v2"
	"google.golang.org/genai"

	"github.com/cloudwego/eino/schema"
)

func TestGemini(t *testing.T) {
	ctx := context.Background()
	model, err := NewChatModel(ctx, &Config{Client: &genai.Client{Models: &genai.Models{}}})
	assert.Nil(t, err)
	mockey.PatchConvey("common", t, func() {
		// Mock Gemini API 响应
		defer mockey.Mock(genai.Models.GenerateContent).Return(&genai.GenerateContentResponse{
			Candidates: []*genai.Candidate{
				{
					Content: &genai.Content{
						Role: "model",
						Parts: []*genai.Part{
							genai.NewPartFromText("Hello, how can I help you?"),
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
		respList := []*genai.GenerateContentResponse{
			{Candidates: []*genai.Candidate{{
				Content: &genai.Content{
					Role: "model",
					Parts: []*genai.Part{
						genai.NewPartFromText("Hello,"),
					},
				},
			}}},
			{Candidates: []*genai.Candidate{{
				Content: &genai.Content{
					Role: "model",
					Parts: []*genai.Part{
						genai.NewPartFromText(" how can I "),
					},
				},
			}}},
			{Candidates: []*genai.Candidate{{
				Content: &genai.Content{
					Role: "model",
					Parts: []*genai.Part{
						genai.NewPartFromText("help you?"),
					},
				},
			}}},
		}
		defer mockey.Mock(genai.Models.GenerateContentStream).Return(func(yield func(*genai.GenerateContentResponse, error) bool) {
			for i := 0; i < 3; i++ {
				if !yield(respList[i], nil) {
					return
				}
			}
			return
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
		responseSchema := &openapi3.Schema{
			Type: "object",
			Properties: map[string]*openapi3.SchemaRef{
				"name": {
					Value: &openapi3.Schema{
						Type: "string",
					},
				},
				"age": {
					Value: &openapi3.Schema{
						Type: "integer",
					},
				},
			},
		}
		model.responseSchema = responseSchema

		// Mock Gemini API 响应
		defer mockey.Mock(genai.Models.GenerateContent).Return(&genai.GenerateContentResponse{
			Candidates: []*genai.Candidate{
				{
					Content: &genai.Content{
						Role: "model",
						Parts: []*genai.Part{
							genai.NewPartFromText(`{"name":"John","age":25}`),
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
		err = model.BindTools([]*schema.ToolInfo{
			{
				Name: "get_weather",
				Desc: "Get weather information",
				ParamsOneOf: schema.NewParamsOneOfByJSONSchema(
					&jsonschema.Schema{
						Type: string(schema.Object),
						Properties: orderedmap.New[string, *jsonschema.Schema](
							orderedmap.WithInitialData[string, *jsonschema.Schema](
								orderedmap.Pair[string, *jsonschema.Schema]{
									Key: "city",
									Value: &jsonschema.Schema{
										Type: string(schema.String),
									},
								},
							),
						),
					},
				),
			},
		})
		assert.NoError(t, err)

		defer mockey.Mock(genai.Models.GenerateContent).Return(&genai.GenerateContentResponse{
			Candidates: []*genai.Candidate{
				{
					Content: &genai.Content{
						Role: "model",
						Parts: []*genai.Part{
							genai.NewPartFromFunctionCall("get_weather", map[string]interface{}{
								"city": "Beijing",
							}),
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
		defer mockey.Mock(genai.Models.GenerateContent).Return(&genai.GenerateContentResponse{
			Candidates: []*genai.Candidate{
				{
					Content: &genai.Content{
						Role: "model",
						Parts: []*genai.Part{
							genai.NewPartFromText("I see a beautiful sunset image"),
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

func TestChatModelConvMedia(t *testing.T) {
	cm := &ChatModel{model: "test model"}
	contents := []schema.ChatMessagePart{
		{
			Type: schema.ChatMessagePartTypeText,
			Text: "test text",
		},
		{
			Type: schema.ChatMessagePartTypeImageURL,
			ImageURL: &schema.ChatMessageImageURL{
				URI:      "test uri",
				MIMEType: "test mime type",
			},
		},
		{
			Type: schema.ChatMessagePartTypeFileURL,
			FileURL: &schema.ChatMessageFileURL{
				URI:      "test uri",
				MIMEType: "test mime type",
			},
		},
		{
			Type: schema.ChatMessagePartTypeAudioURL,
			AudioURL: &schema.ChatMessageAudioURL{
				URI:      "test uri",
				MIMEType: "test mime type",
			},
		},
		{
			Type: schema.ChatMessagePartTypeVideoURL,
			VideoURL: &schema.ChatMessageVideoURL{
				URI:      "test uri",
				MIMEType: "test mime type",
			},
		},
	}

	parts := cm.convMedia(contents)
	assert.Equal(t, 5, len(parts))
	assert.Equal(t, "test text", parts[0].Text)

	for i := 1; i < len(parts); i++ {
		assert.Equal(t, "test uri", parts[i].FileData.FileURI)
		assert.Equal(t, "test mime type", parts[i].FileData.MIMEType)
	}
}

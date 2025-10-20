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
	"encoding/base64"
	"io"
	"testing"
	"time"

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

func Test_toMultiOutPart(t *testing.T) {
	t.Run("nil part", func(t *testing.T) {
		part, err := toMultiOutPart(nil)
		assert.NoError(t, err)
		assert.Empty(t, part)
	})

	t.Run("nil inline data", func(t *testing.T) {
		part, err := toMultiOutPart(&genai.Part{InlineData: nil})
		assert.NoError(t, err)
		assert.Empty(t, part)
	})

	t.Run("image part", func(t *testing.T) {
		data := []byte("fake-image-data")
		encoded := base64.StdEncoding.EncodeToString(data)
		part, err := toMultiOutPart(&genai.Part{
			InlineData: &genai.Blob{
				MIMEType: "image/png",
				Data:     data,
			},
		})
		assert.NoError(t, err)
		assert.Equal(t, schema.ChatMessagePartTypeImageURL, part.Type)
		assert.NotNil(t, part.Image)
		assert.Equal(t, "image/png", part.Image.MIMEType)
		assert.Equal(t, encoded, *part.Image.Base64Data)
	})

	t.Run("unsupported type", func(t *testing.T) {
		part, err := toMultiOutPart(&genai.Part{
			InlineData: &genai.Blob{
				MIMEType: "application/pdf",
				Data:     []byte("fake-pdf-data"),
			},
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported media type")
		assert.Empty(t, part)
	})
}

func TestChatModel_convMedia(t *testing.T) {
	t.Run("convMedia", func(t *testing.T) {
		cm := &ChatModel{model: "test model"}
		base64Data := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII="
		dataURL := "data:image/png;base64," + base64Data
		t.Run("success", func(t *testing.T) {
			contents := []schema.ChatMessagePart{
				{
					Type: schema.ChatMessagePartTypeText,
					Text: "test text",
				},
				{
					Type:     schema.ChatMessagePartTypeImageURL,
					ImageURL: &schema.ChatMessageImageURL{URL: dataURL, MIMEType: "image/png"},
				},
				{
					Type:    schema.ChatMessagePartTypeFileURL,
					FileURL: &schema.ChatMessageFileURL{URL: dataURL, MIMEType: "application/pdf"},
				},
				{
					Type:     schema.ChatMessagePartTypeAudioURL,
					AudioURL: &schema.ChatMessageAudioURL{URL: dataURL, MIMEType: "audio/mp3"},
				},
				{
					Type:     schema.ChatMessagePartTypeVideoURL,
					VideoURL: &schema.ChatMessageVideoURL{URL: dataURL, MIMEType: "video/mp4"},
				},
			}

			parts, err := cm.convMedia(contents)
			assert.NoError(t, err)
			assert.Len(t, parts, 5)
			assert.Equal(t, "test text", parts[0].Text)

			decodedData, err := base64.StdEncoding.DecodeString(base64Data)
			assert.NoError(t, err)

			assert.Equal(t, "image/png", parts[1].InlineData.MIMEType)
			assert.Equal(t, decodedData, parts[1].InlineData.Data)
			assert.Equal(t, "application/pdf", parts[2].InlineData.MIMEType)
			assert.Equal(t, decodedData, parts[2].InlineData.Data)
			assert.Equal(t, "audio/mp3", parts[3].InlineData.MIMEType)
			assert.Equal(t, decodedData, parts[3].InlineData.Data)
			assert.Equal(t, "video/mp4", parts[4].InlineData.MIMEType)
			assert.Equal(t, decodedData, parts[4].InlineData.Data)
		})

		t.Run("with video metadata", func(t *testing.T) {
			videoPart := &schema.ChatMessageVideoURL{URL: dataURL, MIMEType: "video/mp4"}
			SetVideoMetaData(videoPart, &genai.VideoMetadata{
				StartOffset: time.Second,
				EndOffset:   time.Second * 5,
			})
			contents := []schema.ChatMessagePart{
				{
					Type:     schema.ChatMessagePartTypeVideoURL,
					VideoURL: videoPart,
				},
			}
			parts, err := cm.convMedia(contents)
			assert.NoError(t, err)
			assert.Len(t, parts, 2)
			assert.NotNil(t, parts[0].VideoMetadata)
			assert.Equal(t, time.Second, parts[0].VideoMetadata.StartOffset)
			assert.Equal(t, time.Second*5, parts[0].VideoMetadata.EndOffset)
		})

		t.Run("with invalid data url", func(t *testing.T) {
			contents := []schema.ChatMessagePart{
				{
					Type:     schema.ChatMessagePartTypeImageURL,
					ImageURL: &schema.ChatMessageImageURL{URL: "data:image/png;base64,invalid"},
				},
			}
			_, err := cm.convMedia(contents)
			assert.Error(t, err)
		})
	})
	cm := &ChatModel{model: "test model"}
	base64Data := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII="

	t.Run("convInputMedia", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			contents := []schema.MessageInputPart{
				{Type: schema.ChatMessagePartTypeText, Text: "hello"},
				{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{Base64Data: &base64Data, MIMEType: "image/png"}}},
				{Type: schema.ChatMessagePartTypeAudioURL, Audio: &schema.MessageInputAudio{MessagePartCommon: schema.MessagePartCommon{Base64Data: &base64Data, MIMEType: "audio/mp3"}}},
				{Type: schema.ChatMessagePartTypeVideoURL, Video: &schema.MessageInputVideo{MessagePartCommon: schema.MessagePartCommon{Base64Data: &base64Data, MIMEType: "video/mp4"}}},
				{Type: schema.ChatMessagePartTypeFileURL, File: &schema.MessageInputFile{MessagePartCommon: schema.MessagePartCommon{Base64Data: &base64Data, MIMEType: "application/pdf"}}},
			}
			parts, err := cm.convInputMedia(contents)
			assert.NoError(t, err)
			assert.Len(t, parts, 5)
			assert.Equal(t, "hello", parts[0].Text)
			assert.Equal(t, "image/png", parts[1].InlineData.MIMEType)
			assert.Equal(t, "audio/mp3", parts[2].InlineData.MIMEType)
			assert.Equal(t, "video/mp4", parts[3].InlineData.MIMEType)
			assert.Equal(t, "application/pdf", parts[4].InlineData.MIMEType)
			// check data
			decodedData, err := base64.StdEncoding.DecodeString(base64Data)
			assert.NoError(t, err)
			assert.Equal(t, decodedData, parts[1].InlineData.Data)
		})

		t.Run("with video metadata", func(t *testing.T) {
			videoPart := &schema.MessageInputVideo{MessagePartCommon: schema.MessagePartCommon{Base64Data: &base64Data, MIMEType: "video/mp4"}}
			setInputVideoMetaData(videoPart, &genai.VideoMetadata{
				StartOffset: time.Second,
				EndOffset:   time.Second * 5,
			})
			contents := []schema.MessageInputPart{
				{
					Type:  schema.ChatMessagePartTypeVideoURL,
					Video: videoPart,
				},
			}
			parts, err := cm.convInputMedia(contents)
			assert.NoError(t, err)
			assert.Len(t, parts, 2)
			assert.NotNil(t, parts[0].VideoMetadata)
			assert.Equal(t, time.Second, parts[0].VideoMetadata.StartOffset)
			assert.Equal(t, time.Second*5, parts[0].VideoMetadata.EndOffset)
		})

		t.Run("error cases", func(t *testing.T) {
			url := "https://example.com/image.png"
			invalidBase64 := "invalid-base64"
			testCases := []struct {
				name    string
				content schema.MessageInputPart
			}{
				{name: "Image with URL", content: schema.MessageInputPart{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{URL: &url}}}},
				{name: "Audio with URL", content: schema.MessageInputPart{Type: schema.ChatMessagePartTypeAudioURL, Audio: &schema.MessageInputAudio{MessagePartCommon: schema.MessagePartCommon{URL: &url}}}},
				{name: "Video with URL", content: schema.MessageInputPart{Type: schema.ChatMessagePartTypeVideoURL, Video: &schema.MessageInputVideo{MessagePartCommon: schema.MessagePartCommon{URL: &url}}}},
				{name: "File with URL", content: schema.MessageInputPart{Type: schema.ChatMessagePartTypeFileURL, File: &schema.MessageInputFile{MessagePartCommon: schema.MessagePartCommon{URL: &url}}}},
				{name: "Image with invalid base64", content: schema.MessageInputPart{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{Base64Data: &invalidBase64}}}},
				{name: "Image without MIMEType", content: schema.MessageInputPart{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{Base64Data: &base64Data}}}},
				{name: "Audio with invalid base64", content: schema.MessageInputPart{Type: schema.ChatMessagePartTypeAudioURL, Audio: &schema.MessageInputAudio{MessagePartCommon: schema.MessagePartCommon{Base64Data: &invalidBase64}}}},
				{name: "Audio without MIMEType", content: schema.MessageInputPart{Type: schema.ChatMessagePartTypeAudioURL, Audio: &schema.MessageInputAudio{MessagePartCommon: schema.MessagePartCommon{Base64Data: &base64Data}}}},
				{name: "Video with invalid base64", content: schema.MessageInputPart{Type: schema.ChatMessagePartTypeVideoURL, Video: &schema.MessageInputVideo{MessagePartCommon: schema.MessagePartCommon{Base64Data: &invalidBase64}}}},
				{name: "Video without MIMEType", content: schema.MessageInputPart{Type: schema.ChatMessagePartTypeVideoURL, Video: &schema.MessageInputVideo{MessagePartCommon: schema.MessagePartCommon{Base64Data: &base64Data}}}},
				{name: "File with invalid base64", content: schema.MessageInputPart{Type: schema.ChatMessagePartTypeFileURL, File: &schema.MessageInputFile{MessagePartCommon: schema.MessagePartCommon{Base64Data: &invalidBase64}}}},
				{name: "File without MIMEType", content: schema.MessageInputPart{Type: schema.ChatMessagePartTypeFileURL, File: &schema.MessageInputFile{MessagePartCommon: schema.MessagePartCommon{Base64Data: &base64Data}}}},
				{name: "Image with nil media", content: schema.MessageInputPart{Type: schema.ChatMessagePartTypeImageURL, Image: nil}},
				{name: "Audio with nil media", content: schema.MessageInputPart{Type: schema.ChatMessagePartTypeAudioURL, Audio: nil}},
				{name: "Video with nil media", content: schema.MessageInputPart{Type: schema.ChatMessagePartTypeVideoURL, Video: nil}},
				{name: "File with nil media", content: schema.MessageInputPart{Type: schema.ChatMessagePartTypeFileURL, File: nil}},
			}

			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					_, err := cm.convInputMedia([]schema.MessageInputPart{tc.content})
					assert.Error(t, err)
				})
			}
		})
	})

	t.Run("convOutputMedia", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			contents := []schema.MessageOutputPart{
				{Type: schema.ChatMessagePartTypeText, Text: "hello"},
				{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageOutputImage{MessagePartCommon: schema.MessagePartCommon{Base64Data: &base64Data, MIMEType: "image/png"}}},
				{Type: schema.ChatMessagePartTypeAudioURL, Audio: &schema.MessageOutputAudio{MessagePartCommon: schema.MessagePartCommon{Base64Data: &base64Data, MIMEType: "audio/mp3"}}},
				{Type: schema.ChatMessagePartTypeVideoURL, Video: &schema.MessageOutputVideo{MessagePartCommon: schema.MessagePartCommon{Base64Data: &base64Data, MIMEType: "video/mp4"}}},
			}
			parts, err := cm.convOutputMedia(contents)
			assert.NoError(t, err)
			assert.Len(t, parts, 4)
			assert.Equal(t, "hello", parts[0].Text)
			assert.Equal(t, "image/png", parts[1].InlineData.MIMEType)
			assert.Equal(t, "audio/mp3", parts[2].InlineData.MIMEType)
			assert.Equal(t, "video/mp4", parts[3].InlineData.MIMEType)
			// check data
			decodedData, err := base64.StdEncoding.DecodeString(base64Data)
			assert.NoError(t, err)
			assert.Equal(t, decodedData, parts[1].InlineData.Data)
		})

		t.Run("error cases", func(t *testing.T) {
			url := "https://example.com/image.png"
			invalidBase64 := "invalid-base64"
			testCases := []struct {
				name    string
				content schema.MessageOutputPart
			}{
				{name: "Image with URL", content: schema.MessageOutputPart{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageOutputImage{MessagePartCommon: schema.MessagePartCommon{URL: &url}}}},
				{name: "Audio with URL", content: schema.MessageOutputPart{Type: schema.ChatMessagePartTypeAudioURL, Audio: &schema.MessageOutputAudio{MessagePartCommon: schema.MessagePartCommon{URL: &url}}}},
				{name: "Video with URL", content: schema.MessageOutputPart{Type: schema.ChatMessagePartTypeVideoURL, Video: &schema.MessageOutputVideo{MessagePartCommon: schema.MessagePartCommon{URL: &url}}}},
				{name: "Image with invalid base64", content: schema.MessageOutputPart{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageOutputImage{MessagePartCommon: schema.MessagePartCommon{Base64Data: &invalidBase64}}}},
				{name: "Image without MIMEType", content: schema.MessageOutputPart{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageOutputImage{MessagePartCommon: schema.MessagePartCommon{Base64Data: &base64Data}}}},
				{name: "Audio with invalid base64", content: schema.MessageOutputPart{Type: schema.ChatMessagePartTypeAudioURL, Audio: &schema.MessageOutputAudio{MessagePartCommon: schema.MessagePartCommon{Base64Data: &invalidBase64}}}},
				{name: "Audio without MIMEType", content: schema.MessageOutputPart{Type: schema.ChatMessagePartTypeAudioURL, Audio: &schema.MessageOutputAudio{MessagePartCommon: schema.MessagePartCommon{Base64Data: &base64Data}}}},
				{name: "Video with invalid base64", content: schema.MessageOutputPart{Type: schema.ChatMessagePartTypeVideoURL, Video: &schema.MessageOutputVideo{MessagePartCommon: schema.MessagePartCommon{Base64Data: &invalidBase64}}}},
				{name: "Video without MIMEType", content: schema.MessageOutputPart{Type: schema.ChatMessagePartTypeVideoURL, Video: &schema.MessageOutputVideo{MessagePartCommon: schema.MessagePartCommon{Base64Data: &base64Data}}}},
				{name: "Image with nil media", content: schema.MessageOutputPart{Type: schema.ChatMessagePartTypeImageURL, Image: nil}},
				{name: "Audio with nil media", content: schema.MessageOutputPart{Type: schema.ChatMessagePartTypeAudioURL, Audio: nil}},
				{name: "Video with nil media", content: schema.MessageOutputPart{Type: schema.ChatMessagePartTypeVideoURL, Video: nil}},
			}

			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					_, err := cm.convOutputMedia([]schema.MessageOutputPart{tc.content})
					assert.Error(t, err)
				})
			}
		})
	})
}

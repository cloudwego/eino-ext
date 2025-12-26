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
	"github.com/cloudwego/eino/components/model"
	"github.com/eino-contrib/jsonschema"
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
			UsageMetadata: &genai.GenerateContentResponseUsageMetadata{
				TotalTokenCount:      100,
				ThoughtsTokenCount:   50,
				CandidatesTokenCount: 50,
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
		assert.Equal(t, 100, resp.ResponseMeta.Usage.TotalTokens)
		assert.Equal(t, 50, resp.ResponseMeta.Usage.CompletionTokensDetails.ReasoningTokens)
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
		}, WithTopK(0), WithResponseJSONSchema(&jsonschema.Schema{
			Type: "string",
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
		responseSchema := &jsonschema.Schema{
			Type: "object",
			Properties: orderedmap.New[string, *jsonschema.Schema](
				orderedmap.WithInitialData[string, *jsonschema.Schema](
					orderedmap.Pair[string, *jsonschema.Schema]{
						Key: "name",
						Value: &jsonschema.Schema{
							Type: string(schema.String),
						},
					},
					orderedmap.Pair[string, *jsonschema.Schema]{
						Key: "age",
						Value: &jsonschema.Schema{
							Type: string(schema.Integer),
						},
					},
				),
			),
		}
		model.responseJSONSchema = responseSchema

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

			parts, err := convMedia(contents)
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
			parts, err := convMedia(contents)
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
			_, err := convMedia(contents)
			assert.Error(t, err)
		})
	})
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
			parts, err := convInputMedia(contents)
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
			SetInputVideoMetaData(videoPart, &genai.VideoMetadata{
				StartOffset: time.Second,
				EndOffset:   time.Second * 5,
			})
			contents := []schema.MessageInputPart{
				{
					Type:  schema.ChatMessagePartTypeVideoURL,
					Video: videoPart,
				},
			}
			parts, err := convInputMedia(contents)
			assert.NoError(t, err)
			assert.Len(t, parts, 2)
			assert.NotNil(t, parts[0].VideoMetadata)
			assert.Equal(t, time.Second, parts[0].VideoMetadata.StartOffset)
			assert.Equal(t, time.Second*5, parts[0].VideoMetadata.EndOffset)
		})

		t.Run("error cases", func(t *testing.T) {
			invalidBase64 := "invalid-base64"
			testCases := []struct {
				name    string
				content schema.MessageInputPart
			}{
				{name: "Image with invalid base64", content: schema.MessageInputPart{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{Base64Data: &invalidBase64}}}},
				{name: "Audio with invalid base64", content: schema.MessageInputPart{Type: schema.ChatMessagePartTypeAudioURL, Audio: &schema.MessageInputAudio{MessagePartCommon: schema.MessagePartCommon{Base64Data: &invalidBase64}}}},
				{name: "Video with invalid base64", content: schema.MessageInputPart{Type: schema.ChatMessagePartTypeVideoURL, Video: &schema.MessageInputVideo{MessagePartCommon: schema.MessagePartCommon{Base64Data: &invalidBase64}}}},
				{name: "File with invalid base64", content: schema.MessageInputPart{Type: schema.ChatMessagePartTypeFileURL, File: &schema.MessageInputFile{MessagePartCommon: schema.MessagePartCommon{Base64Data: &invalidBase64}}}},
				{name: "Image with nil media", content: schema.MessageInputPart{Type: schema.ChatMessagePartTypeImageURL, Image: nil}},
				{name: "Audio with nil media", content: schema.MessageInputPart{Type: schema.ChatMessagePartTypeAudioURL, Audio: nil}},
				{name: "Video with nil media", content: schema.MessageInputPart{Type: schema.ChatMessagePartTypeVideoURL, Video: nil}},
				{name: "File with nil media", content: schema.MessageInputPart{Type: schema.ChatMessagePartTypeFileURL, File: nil}},
			}

			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					_, err := convInputMedia([]schema.MessageInputPart{tc.content})
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
			parts, err := convOutputMedia(contents)
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
			invalidBase64 := "invalid-base64"
			testCases := []struct {
				name    string
				content schema.MessageOutputPart
			}{
				{name: "Image with invalid base64", content: schema.MessageOutputPart{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageOutputImage{MessagePartCommon: schema.MessagePartCommon{Base64Data: &invalidBase64}}}},
				{name: "Audio with invalid base64", content: schema.MessageOutputPart{Type: schema.ChatMessagePartTypeAudioURL, Audio: &schema.MessageOutputAudio{MessagePartCommon: schema.MessagePartCommon{Base64Data: &invalidBase64}}}},
				{name: "Video with invalid base64", content: schema.MessageOutputPart{Type: schema.ChatMessagePartTypeVideoURL, Video: &schema.MessageOutputVideo{MessagePartCommon: schema.MessagePartCommon{Base64Data: &invalidBase64}}}},
				{name: "Image with nil media", content: schema.MessageOutputPart{Type: schema.ChatMessagePartTypeImageURL, Image: nil}},
				{name: "Audio with nil media", content: schema.MessageOutputPart{Type: schema.ChatMessagePartTypeAudioURL, Audio: nil}},
				{name: "Video with nil media", content: schema.MessageOutputPart{Type: schema.ChatMessagePartTypeVideoURL, Video: nil}},
			}

			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					_, err := convOutputMedia([]schema.MessageOutputPart{tc.content})
					assert.Error(t, err)
				})
			}
		})
	})
}

func TestThoughtSignatureRoundTrip(t *testing.T) {
	t.Run("convToolMessageToPart", func(t *testing.T) {
		part, err := convToolMessageToPart("tool_1", `{"result":"ok"}`)
		assert.NoError(t, err)
		assert.NotNil(t, part.FunctionResponse)
		assert.Equal(t, "tool_1", part.FunctionResponse.Name)
		assert.Equal(t, "ok", part.FunctionResponse.Response["result"])
	})

	t.Run("convToolMessageToPart fallback to output", func(t *testing.T) {
		part, err := convToolMessageToPart("tool_2", "raw-response")
		assert.NoError(t, err)
		assert.NotNil(t, part.FunctionResponse)
		assert.Equal(t, "tool_2", part.FunctionResponse.Name)
		assert.Equal(t, "raw-response", part.FunctionResponse.Response["output"])
	})

	t.Run("convSchemaMessages merges consecutive tool responses", func(t *testing.T) {
		messages := []*schema.Message{
			{
				Role: schema.Assistant,
				ToolCalls: []schema.ToolCall{
					{
						ID: "call_a",
						Function: schema.FunctionCall{
							Name:      "fn_a",
							Arguments: `{"x":1}`,
						},
					},
					{
						ID: "call_b",
						Function: schema.FunctionCall{
							Name:      "fn_b",
							Arguments: `{"y":2}`,
						},
					},
				},
			},
			{Role: schema.Tool, ToolCallID: "call_a", Content: `{"res":"A"}`},
			{Role: schema.Tool, ToolCallID: "call_b", Content: `{"res":"B"}`},
		}

		contents, err := convSchemaMessages(messages)
		assert.NoError(t, err)
		assert.Len(t, contents, 2)
		assert.Equal(t, roleModel, contents[0].Role)
		assert.Equal(t, roleUser, contents[1].Role)
		if assert.Len(t, contents[1].Parts, 2) {
			assert.Equal(t, "call_a", contents[1].Parts[0].FunctionResponse.Name)
			assert.Equal(t, "A", contents[1].Parts[0].FunctionResponse.Response["res"])
			assert.Equal(t, "call_b", contents[1].Parts[1].FunctionResponse.Name)
			assert.Equal(t, "B", contents[1].Parts[1].FunctionResponse.Response["res"])
		}
	})

	t.Run("convSchemaMessage without thought signature", func(t *testing.T) {
		toolCall := &schema.ToolCall{
			ID: "test_call",
			Function: schema.FunctionCall{
				Name:      "test_function",
				Arguments: `{"param":"value"}`,
			},
		}

		message := &schema.Message{
			Role:      schema.Assistant,
			ToolCalls: []schema.ToolCall{*toolCall},
		}

		content, err := convSchemaMessage(message)
		assert.NoError(t, err)
		assert.NotNil(t, content)
		assert.Len(t, content.Parts, 1)

		// Verify no thought signature in the Part when none was stored
		assert.Nil(t, content.Parts[0].ThoughtSignature)
		assert.NotNil(t, content.Parts[0].FunctionCall)
	})

	// Test that reasoning content thought signature is preserved through the round-trip
	// Per Gemini docs, signature should be on the final part (text), not the thought part
	t.Run("convSchemaMessage restores reasoning content with thought signature on final part", func(t *testing.T) {
		signature := []byte("reasoning_thought_signature")
		message := &schema.Message{
			Role:             schema.Assistant,
			Content:          "final answer",
			ReasoningContent: "thinking process",
		}
		setMessageThoughtSignature(message, signature)

		content, err := convSchemaMessage(message)
		assert.NoError(t, err)
		assert.NotNil(t, content)
		// Should have 2 parts: thought part + text part
		assert.Len(t, content.Parts, 2)

		// First part should be the thought (without signature)
		assert.True(t, content.Parts[0].Thought)
		assert.Equal(t, "thinking process", content.Parts[0].Text)
		assert.Nil(t, content.Parts[0].ThoughtSignature)

		// Second part should be the text content with signature (final part per Gemini docs)
		assert.False(t, content.Parts[1].Thought)
		assert.Equal(t, "final answer", content.Parts[1].Text)
		assert.Equal(t, signature, content.Parts[1].ThoughtSignature)
	})

	t.Run("convSchemaMessage restores reasoning content without thought signature", func(t *testing.T) {
		message := &schema.Message{
			Role:             schema.Assistant,
			Content:          "final answer",
			ReasoningContent: "thinking process",
		}

		content, err := convSchemaMessage(message)
		assert.NoError(t, err)
		assert.NotNil(t, content)
		// Should have 2 parts: thought part + text part
		assert.Len(t, content.Parts, 2)

		// First part should be the thought without signature
		assert.True(t, content.Parts[0].Thought)
		assert.Equal(t, "thinking process", content.Parts[0].Text)
		assert.Nil(t, content.Parts[0].ThoughtSignature)
	})

	t.Run("convSchemaMessage with reasoning content and tool calls with signature on functionCall", func(t *testing.T) {
		fcSignature := []byte("function_call_signature")

		toolCall := schema.ToolCall{
			ID: "test_call",
			Function: schema.FunctionCall{
				Name:      "test_function",
				Arguments: `{"param":"value"}`,
			},
		}
		// Per Gemini docs, signature should be on the functionCall part
		setToolCallThoughtSignature(&toolCall, fcSignature)

		message := &schema.Message{
			Role:             schema.Assistant,
			ReasoningContent: "thinking before calling tool",
			ToolCalls:        []schema.ToolCall{toolCall},
		}

		content, err := convSchemaMessage(message)
		assert.NoError(t, err)
		assert.NotNil(t, content)
		// Should have 2 parts: thought part + function call part
		assert.Len(t, content.Parts, 2)

		// First part should be the thought (without signature)
		assert.True(t, content.Parts[0].Thought)
		assert.Equal(t, "thinking before calling tool", content.Parts[0].Text)
		assert.Nil(t, content.Parts[0].ThoughtSignature)

		// Second part should be the function call with signature
		assert.NotNil(t, content.Parts[1].FunctionCall)
		assert.Equal(t, "test_function", content.Parts[1].FunctionCall.Name)
		assert.Equal(t, fcSignature, content.Parts[1].ThoughtSignature)
	})

	// Test functionCall part with thought signature (per Gemini 3 Pro docs)
	t.Run("convSchemaMessage with tool call signature on functionCall part", func(t *testing.T) {
		signature := []byte("function_call_signature")

		toolCall := schema.ToolCall{
			ID: "test_call",
			Function: schema.FunctionCall{
				Name:      "check_flight",
				Arguments: `{"flight":"AA100"}`,
			},
		}
		setToolCallThoughtSignature(&toolCall, signature)

		message := &schema.Message{
			Role:      schema.Assistant,
			ToolCalls: []schema.ToolCall{toolCall},
		}

		content, err := convSchemaMessage(message)
		assert.NoError(t, err)
		assert.NotNil(t, content)
		assert.Len(t, content.Parts, 1)

		// The functionCall part should have the signature attached
		assert.NotNil(t, content.Parts[0].FunctionCall)
		assert.Equal(t, "check_flight", content.Parts[0].FunctionCall.Name)
		assert.Equal(t, signature, content.Parts[0].ThoughtSignature)
	})

	// Test parallel function calls - only first has signature
	t.Run("convSchemaMessage with parallel tool calls (first has signature)", func(t *testing.T) {
		signature := []byte("parallel_signature")

		toolCall1 := schema.ToolCall{
			ID: "call_1",
			Function: schema.FunctionCall{
				Name:      "get_weather",
				Arguments: `{"location":"Paris"}`,
			},
		}
		setToolCallThoughtSignature(&toolCall1, signature)

		toolCall2 := schema.ToolCall{
			ID: "call_2",
			Function: schema.FunctionCall{
				Name:      "get_weather",
				Arguments: `{"location":"London"}`,
			},
		}
		// Second tool call has no signature (per Gemini docs for parallel calls)

		message := &schema.Message{
			Role:      schema.Assistant,
			ToolCalls: []schema.ToolCall{toolCall1, toolCall2},
		}

		content, err := convSchemaMessage(message)
		assert.NoError(t, err)
		assert.NotNil(t, content)
		assert.Len(t, content.Parts, 2)

		// First functionCall should have signature
		assert.NotNil(t, content.Parts[0].FunctionCall)
		assert.Equal(t, "get_weather", content.Parts[0].FunctionCall.Name)
		assert.Equal(t, signature, content.Parts[0].ThoughtSignature)

		// Second functionCall should not have signature
		assert.NotNil(t, content.Parts[1].FunctionCall)
		assert.Equal(t, "get_weather", content.Parts[1].FunctionCall.Name)
		assert.Nil(t, content.Parts[1].ThoughtSignature)
	})

	// Test text part with signature (non-function-call response)
	t.Run("convSchemaMessage with text part signature", func(t *testing.T) {
		signature := []byte("text_signature")

		message := &schema.Message{
			Role:    schema.Assistant,
			Content: "This is the response",
		}
		setMessageThoughtSignature(message, signature)

		content, err := convSchemaMessage(message)
		assert.NoError(t, err)
		assert.NotNil(t, content)
		assert.Len(t, content.Parts, 1)

		// Text part should have signature for non-function-call response
		assert.Equal(t, "This is the response", content.Parts[0].Text)
		assert.Equal(t, signature, content.Parts[0].ThoughtSignature)
	})

	// Test convCandidate extracts signature from functionCall part
	t.Run("convCandidate extracts signature from functionCall part", func(t *testing.T) {
		signature := []byte("extracted_signature")

		candidate := &genai.Candidate{
			Content: &genai.Content{
				Role: roleModel,
				Parts: []*genai.Part{
					{
						FunctionCall: &genai.FunctionCall{
							Name: "check_flight",
							Args: map[string]any{"flight": "AA100"},
						},
						ThoughtSignature: signature,
					},
				},
			},
		}

		message, err := convCandidate(candidate)
		assert.NoError(t, err)
		assert.NotNil(t, message)
		assert.Len(t, message.ToolCalls, 1)

		// Signature should be stored on the tool call
		assert.Equal(t, signature, getToolCallThoughtSignature(&message.ToolCalls[0]))
		// Message-level signature should be nil (signature is on functionCall)
		assert.Nil(t, getMessageThoughtSignature(message))
	})

	// Test convCandidate extracts signature from text part (non-function-call)
	t.Run("convCandidate extracts signature from text part", func(t *testing.T) {
		signature := []byte("text_part_signature")

		candidate := &genai.Candidate{
			Content: &genai.Content{
				Role: roleModel,
				Parts: []*genai.Part{
					{
						Text:             "Final response",
						ThoughtSignature: signature,
					},
				},
			},
		}

		message, err := convCandidate(candidate)
		assert.NoError(t, err)
		assert.NotNil(t, message)
		assert.Equal(t, "Final response", message.Content)

		// Signature should be stored at message level for non-functionCall parts
		assert.Equal(t, signature, getMessageThoughtSignature(message))
	})

	// Test sequential function calls - each step has its own signature
	t.Run("sequential function call signatures are preserved separately", func(t *testing.T) {
		sigA := []byte("signature_A")
		sigB := []byte("signature_B")

		// Simulate step 1 response
		candidate1 := &genai.Candidate{
			Content: &genai.Content{
				Role: roleModel,
				Parts: []*genai.Part{
					{
						FunctionCall: &genai.FunctionCall{
							Name: "check_flight",
							Args: map[string]any{"flight": "AA100"},
						},
						ThoughtSignature: sigA,
					},
				},
			},
		}

		msg1, err := convCandidate(candidate1)
		assert.NoError(t, err)
		assert.Equal(t, sigA, getToolCallThoughtSignature(&msg1.ToolCalls[0]))

		// Simulate step 2 response
		candidate2 := &genai.Candidate{
			Content: &genai.Content{
				Role: roleModel,
				Parts: []*genai.Part{
					{
						FunctionCall: &genai.FunctionCall{
							Name: "book_taxi",
							Args: map[string]any{"time": "10 AM"},
						},
						ThoughtSignature: sigB,
					},
				},
			},
		}

		msg2, err := convCandidate(candidate2)
		assert.NoError(t, err)
		assert.Equal(t, sigB, getToolCallThoughtSignature(&msg2.ToolCalls[0]))

		// Verify both signatures can be restored correctly
		content1, err := convSchemaMessage(msg1)
		assert.NoError(t, err)
		assert.Equal(t, sigA, content1.Parts[0].ThoughtSignature)

		content2, err := convSchemaMessage(msg2)
		assert.NoError(t, err)
		assert.Equal(t, sigB, content2.Parts[0].ThoughtSignature)
	})
}

func TestCreatePrefixCache(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		ctx := context.Background()
		cm, err := NewChatModel(ctx, &Config{Client: &genai.Client{Caches: &genai.Caches{}}, Model: "test-model"})
		assert.Nil(t, err)

		defer mockey.Mock(genai.Caches.Create).Return(&genai.CachedContent{Name: "cached/basic"}, nil).Build().UnPatch()

		prefixMsgs := []*schema.Message{{Role: schema.User, Content: "Hello"}}
		cache, err := cm.CreatePrefixCache(ctx, prefixMsgs)
		assert.NoError(t, err)
		assert.NotNil(t, cache)
	})

	t.Run("cache_instruction_tools_messages", func(t *testing.T) {
		ctx := context.Background()
		cm, err := NewChatModel(ctx, &Config{Client: &genai.Client{Caches: &genai.Caches{}}, Model: "test-model"})
		assert.Nil(t, err)

		cm.cache = &CacheConfig{TTL: time.Minute}
		cm.enableCodeExecution = true

		var cacheConfig *genai.CreateCachedContentConfig
		defer mockey.Mock(genai.Caches.Create).
			To(func(ctx context.Context, model string, config *genai.CreateCachedContentConfig) (*genai.CachedContent, error) {
				cacheConfig = config
				return &genai.CachedContent{Name: "cached/cache_instruction_tools_messages"}, nil
			}).Build().Patch().UnPatch()

		prefixMsgs := []*schema.Message{
			{Role: schema.System, Content: "sys"},
			{Role: schema.User, Content: "hello"},
		}

		cache, err := cm.CreatePrefixCache(ctx, prefixMsgs,
			model.WithTools([]*schema.ToolInfo{
				{
					Name:        "tool_a",
					Desc:        "desc",
					ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{}),
				},
			}))
		assert.NoError(t, err)
		assert.NotNil(t, cache)
		assert.Equal(t, "cached/cache_instruction_tools_messages", cache.Name)
		assert.Equal(t, time.Minute, cacheConfig.TTL)

		assert.Equal(t, "hello", cacheConfig.Contents[0].Parts[0].Text)

		assert.NotNil(t, cacheConfig.SystemInstruction)
		assert.Equal(t, "sys", cacheConfig.SystemInstruction.Parts[0].Text)

		assert.NotNil(t, cacheConfig.Tools)
		assert.Len(t, cacheConfig.Tools, 2)
		assert.Equal(t, "tool_a", cacheConfig.Tools[0].FunctionDeclarations[0].Name)
		assert.NotNil(t, cacheConfig.Tools[1].CodeExecution)

	})

	t.Run("cache_and_generate", func(t *testing.T) {
		ctx := context.Background()
		cm, err := NewChatModel(ctx, &Config{Client: &genai.Client{
			Models: &genai.Models{},
			Caches: &genai.Caches{}},
			Model: "test-model"})
		assert.Nil(t, err)

		cm.cache = &CacheConfig{TTL: time.Minute}
		cm.enableCodeExecution = true

		defer mockey.Mock(genai.Caches.Create).
			To(func(ctx context.Context, model string, config *genai.CreateCachedContentConfig) (*genai.CachedContent, error) {
				return &genai.CachedContent{Name: "cached/cache_and_generate"}, nil
			}).Build().Patch().UnPatch()

		var generateConf *genai.GenerateContentConfig
		defer mockey.Mock(genai.Models.GenerateContent).
			To(func(ctx context.Context, model string, contents []*genai.Content, config *genai.GenerateContentConfig) (
				*genai.GenerateContentResponse, error) {
				generateConf = config
				return &genai.GenerateContentResponse{
					Candidates: []*genai.Candidate{
						{
							Content: &genai.Content{
								Role: "model",
								Parts: []*genai.Part{
									genai.NewPartFromText("bye too"),
								},
							},
						},
					},
				}, nil
			}).Build().Patch().UnPatch()

		prefixMsgs := []*schema.Message{
			{Role: schema.System, Content: "sys"},
			{Role: schema.User, Content: "hello"},
		}

		cache, err := cm.CreatePrefixCache(ctx, prefixMsgs,
			model.WithTools([]*schema.ToolInfo{
				{
					Name:        "tool_a",
					Desc:        "desc",
					ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{}),
				},
			}))
		assert.NoError(t, err)
		assert.NotNil(t, cache)

		_, err = cm.Generate(ctx, []*schema.Message{schema.UserMessage("bye")},
			WithCachedContentName(cache.Name))
		assert.NoError(t, err)

		assert.Equal(t, "cached/cache_and_generate", generateConf.CachedContent)
		assert.Nil(t, generateConf.SystemInstruction)
		assert.Len(t, generateConf.Tools, 0)
	})
}

func TestSpecialPart(t *testing.T) {
	msg, err := convCandidate(&genai.Candidate{
		Content: &genai.Content{
			Parts: []*genai.Part{
				genai.NewPartFromExecutableCode("code", "language"),
				genai.NewPartFromCodeExecutionResult("outcome", "output"),
			},
			Role: genai.RoleModel,
		},
	})
	assert.Nil(t, err)

	content, err := convSchemaMessage(msg)
	assert.Nil(t, err)
	assert.Equal(t, content.Parts[0].ExecutableCode.Code, "code")
	assert.Equal(t, content.Parts[0].ExecutableCode.Language, genai.Language("language"))
	assert.Equal(t, content.Parts[1].CodeExecutionResult.Output, "output")
	assert.Equal(t, content.Parts[1].CodeExecutionResult.Outcome, genai.Outcome("outcome"))
}

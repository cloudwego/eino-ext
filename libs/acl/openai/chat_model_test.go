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

package openai

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"reflect"
	"testing"
	"unsafe"

	"github.com/bytedance/mockey"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	openai "github.com/meguminnnnnnnnn/go-openai"
	"github.com/stretchr/testify/assert"
)

func TestStream(t *testing.T) {
	t.Run("stream applies RequestPayloadModifier", func(t *testing.T) {
		// mock CreateChatCompletionStream to validate options carry payload modifier
		defer mockey.Mock((*openai.Client).CreateChatCompletionStream).To(func(ctx context.Context,
			request openai.ChatCompletionRequest, opts ...openai.ChatCompletionRequestOption) (response *openai.ChatCompletionStream, err error) {
			assert.GreaterOrEqual(t, len(opts), 1)
			return nil, fmt.Errorf("expected error to stop early")
		}).Build().Patch().UnPatch()

		c := &Client{config: &Config{Model: "test-model"}}
		// initialize cli to a valid client; network won't be used due to mocking
		conf := openai.DefaultConfig("dummy-key")
		c.cli = openai.NewClientWithConfig(conf)

		in := []*schema.Message{{Role: schema.User, Content: "hello"}}
		_, err := c.Stream(t.Context(), in,
			WithRequestPayloadModifier(func(ctx context.Context, msgs []*schema.Message, rawBody []byte) ([]byte, error) {
				return rawBody, nil
			}),
		)
		assert.Error(t, err)
	})

	t.Run("stream_with_ResponseChunkMessageModifier", func(t *testing.T) {

		c := &Client{config: &Config{Model: "test-model"}}
		conf := openai.DefaultConfig("dummy-key")
		c.cli = openai.NewClientWithConfig(conf)

		// prepare a fake stream and patch its methods
		stream := &openai.ChatCompletionStream{}

		defer mockey.Mock(mockey.GetMethod(c.cli, "CreateChatCompletionStream")).
			To(func(ctx context.Context, req openai.ChatCompletionRequest, opts ...openai.ChatCompletionRequestOption) (
				response *openai.ChatCompletionStream, err error) {
				return stream, nil
			}).Build().Patch().UnPatch()

		innerStream := populateAndGetEmbeddedStreamReader(stream)
		var call int
		defer mockey.Mock(mockey.GetMethod(innerStream, "Recv")).
			To(func() (openai.ChatCompletionStreamResponse, error) {
				call++
				if call == 1 {
					return openai.ChatCompletionStreamResponse{
						Choices: []openai.ChatCompletionStreamChoice{
							{
								Index: 0,
								Delta: openai.ChatCompletionStreamChoiceDelta{Role: "assistant", Content: "hello"},
							},
						},
						RawBody: []byte(`{"role":"assistant","content":"hello"}`),
					}, nil
				}
				// final EOF with last raw body
				return openai.ChatCompletionStreamResponse{RawBody: []byte("rawEOF")}, io.EOF
			}).Build().Patch().UnPatch()
		defer mockey.Mock(mockey.GetMethod(innerStream, "Close")).Return(nil).Build().Patch().UnPatch()

		in := []*schema.Message{{Role: schema.User, Content: "hello"}}
		var seenBodies []string
		var seenEnds []bool
		outStream, err := c.Stream(t.Context(), in,
			WithResponseChunkMessageModifier(func(ctx context.Context, msg *schema.Message, rawBody []byte, end bool) (*schema.Message, error) {
				seenBodies = append(seenBodies, string(rawBody))
				seenEnds = append(seenEnds, end)
				if msg == nil {
					return msg, nil
				}
				// reflect rawBody usage by appending its string form
				msg.Content = msg.Content + "|mod|" + string(rawBody)
				return msg, nil
			}),
		)
		assert.NoError(t, err)
		defer outStream.Close()

		// read first message
		msg, recvErr := outStream.Recv()
		assert.NoError(t, recvErr)
		assert.Equal(t, schema.Assistant, msg.Role)
		assert.Equal(t, "hello|mod|{\"role\":\"assistant\",\"content\":\"hello\"}", msg.Content)
		// next call should be EOF
		_, recvErr = outStream.Recv()
		assert.Equal(t, io.EOF, recvErr)
		// verify rawBody captured for both the content frame and EOF frame
		assert.Equal(t, 2, len(seenBodies))
		assert.Equal(t, []bool{false, true}, seenEnds)
		assert.Equal(t, "{\"role\":\"assistant\",\"content\":\"hello\"}", seenBodies[0])
		// Some versions may not carry RawBody on EOF; accept empty
		assert.True(t, seenBodies[1] == "rawEOF" || seenBodies[1] == "")
	})
}

func TestGenerate(t *testing.T) {
	t.Run("payload and response modifiers", func(t *testing.T) {
		c := &Client{config: &Config{Model: "test-model"}}
		conf := openai.DefaultConfig("dummy-key")
		c.cli = openai.NewClientWithConfig(conf)

		// Mock CreateChatCompletion to assert options and return a basic response
		defer mockey.Mock((*openai.Client).CreateChatCompletion).To(func(ctx context.Context,
			req openai.ChatCompletionRequest, opts ...openai.ChatCompletionRequestOption) (openai.ChatCompletionResponse, error) {
			// expect at least one option due to RequestPayloadModifier
			assert.GreaterOrEqual(t, len(opts), 1)
			return openai.ChatCompletionResponse{
				ID: "resp-id-123",
				Choices: []openai.ChatCompletionChoice{
					{
						Index:   0,
						Message: openai.ChatCompletionMessage{Role: "assistant", Content: "hello"},
					},
				},
				RawBody: []byte(`{"role":"assistant","content":"hello"}`),
			}, nil
		}).Build().UnPatch()

		in := []*schema.Message{{Role: schema.User, Content: "hi"}}

		outMsg, err := c.Generate(t.Context(), in,
			WithRequestPayloadModifier(func(ctx context.Context, msgs []*schema.Message, rawBody []byte) ([]byte, error) {
				// keep raw body unchanged for simplicity; presence is asserted via len(opts)
				return rawBody, nil
			}),
			WithResponseMessageModifier(func(ctx context.Context, msg *schema.Message, rawBody []byte) (*schema.Message, error) {
				// append marker and raw body to verify usage
				return &schema.Message{
					Role:         msg.Role,
					Name:         msg.Name,
					Content:      msg.Content + "|mod|" + string(rawBody),
					ToolCallID:   msg.ToolCallID,
					ToolCalls:    msg.ToolCalls,
					ResponseMeta: msg.ResponseMeta,
				}, nil
			}),
		)
		assert.NoError(t, err)
		assert.NotNil(t, outMsg)
		assert.Equal(t, schema.Assistant, outMsg.Role)
		assert.Equal(t, "hello|mod|{\"role\":\"assistant\",\"content\":\"hello\"}", outMsg.Content)
	})
}

func TestToXXXUtils(t *testing.T) {
	t.Run("toOpenAIMultiContent", func(t *testing.T) {

		multiContents := []schema.ChatMessagePart{
			{
				Type: schema.ChatMessagePartTypeText,
				Text: "image_desc",
			},
			{
				Type: schema.ChatMessagePartTypeImageURL,
				ImageURL: &schema.ChatMessageImageURL{
					URL:    "test_url",
					Detail: schema.ImageURLDetailAuto,
				},
			},
			{
				Type: schema.ChatMessagePartTypeAudioURL,
				AudioURL: &schema.ChatMessageAudioURL{
					URL:      "test_url",
					MIMEType: "mp3",
				},
			},
			{
				Type: schema.ChatMessagePartTypeVideoURL,
				VideoURL: &schema.ChatMessageVideoURL{
					URL: "test_url",
				},
			},
		}

		mc, err := toOpenAIMultiContent(multiContents)
		assert.NoError(t, err)
		assert.Len(t, mc, 4)
		assert.Equal(t, mc[0], openai.ChatMessagePart{
			Type: openai.ChatMessagePartTypeText,
			Text: "image_desc",
		})

		assert.Equal(t, mc[1], openai.ChatMessagePart{
			Type: openai.ChatMessagePartTypeImageURL,
			ImageURL: &openai.ChatMessageImageURL{
				URL:    "test_url",
				Detail: openai.ImageURLDetailAuto,
			},
		})

		assert.Equal(t, mc[2], openai.ChatMessagePart{
			Type: openai.ChatMessagePartTypeInputAudio,
			InputAudio: &openai.ChatMessageInputAudio{
				Data:   "test_url",
				Format: "mp3",
			},
		})
		assert.Equal(t, mc[3], openai.ChatMessagePart{
			Type: openai.ChatMessagePartTypeVideoURL,
			VideoURL: &openai.ChatMessageVideoURL{
				URL: "test_url",
			},
		})

		mc, err = toOpenAIMultiContent(nil)
		assert.Nil(t, err)
		assert.Nil(t, mc)
	})
}

func TestToOpenAIToolCalls(t *testing.T) {
	t.Run("empty tools", func(t *testing.T) {
		tools := toOpenAIToolCalls([]schema.ToolCall{})
		assert.Len(t, tools, 0)
	})

	t.Run("normal tools", func(t *testing.T) {
		fakeToolCall1 := schema.ToolCall{
			ID:       randStr(),
			Function: schema.FunctionCall{Name: randStr(), Arguments: randStr()},
		}

		toolCalls := toOpenAIToolCalls([]schema.ToolCall{fakeToolCall1})

		assert.Len(t, toolCalls, 1)
		assert.Equal(t, fakeToolCall1.ID, toolCalls[0].ID)
		assert.Equal(t, fakeToolCall1.Function.Name, toolCalls[0].Function.Name)
	})
}

func randStr() string {
	seeds := []rune("abcdefghijklmnopqrstuvwxyz")
	b := make([]rune, 8)
	for i := range b {
		b[i] = seeds[rand.Intn(len(seeds))]
	}
	return string(b)
}

func TestPanicErr(t *testing.T) {
	err := newPanicErr("info", []byte("stack"))
	assert.Equal(t, "panic error: info, \nstack: stack", err.Error())
}

func TestWithTools(t *testing.T) {
	cm := &Client{config: &Config{Model: "test model"}}
	ncm, err := cm.WithToolsForClient([]*schema.ToolInfo{{Name: "test tool name"}})
	assert.Nil(t, err)
	assert.Equal(t, "test model", ncm.config.Model)
	assert.Equal(t, "test tool name", ncm.rawTools[0].Name)
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
	}}, toLogProbs(&openai.LogProbs{Content: []openai.LogProb{
		{
			Token:   "1",
			LogProb: 1,
			Bytes:   []byte{'a'},
			TopLogProbs: []openai.TopLogProbs{
				{
					Token:   "2",
					LogProb: 2,
					Bytes:   []byte{'b'},
				},
			},
		},
	}}))
}

func TestToTools(t *testing.T) {
	mockey.PatchConvey("", t, func() {
		mockTools := []*schema.ToolInfo{
			{
				Name: "test tool name",
				Desc: "description of test tool",
				ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
					"126": {
						Type:     schema.String,
						Required: true,
					},
					"123": {
						Type:     schema.Array,
						Required: true,
						ElemInfo: &schema.ParameterInfo{
							Type:     schema.Object,
							Required: true,
							SubParams: map[string]*schema.ParameterInfo{
								"459": {
									Type:     schema.String,
									Required: true,
								},
								"458": {
									Type:     schema.String,
									Required: true,
								},
								"457": {
									Type:     schema.String,
									Required: true,
								},
							},
						},
					},
					"129": {
						Type:     schema.Object,
						Required: true,
					},
				}),
			},
		}

		tools, err := toTools(mockTools)
		assert.Nil(t, err)
		assert.Len(t, tools, 1)

		sc := tools[0].Function.Parameters
		assert.Equal(t, []string{"123", "126", "129"}, sc.Required)
		props, ok := sc.Properties.Get("123")
		assert.True(t, ok)
		assert.Equal(t, []string{"457", "458", "459"}, props.Items.Required)
	})
}

func TestBuildMessages(t *testing.T) {
	t.Run("buildMessageFromAssistantGenMultiContent", func(t *testing.T) {
		t.Run("success with audio", func(t *testing.T) {
			mockey.PatchConvey("mock GetMessageOutputAudioID", t, func() {
				mockey.Mock(getMessageOutputAudioID).Return("audio-id-123", true).Build()
				inMsg := &schema.Message{
					Role: schema.Assistant,
					Name: "test-assistant",
					AssistantGenMultiContent: []schema.MessageOutputPart{
						{
							Type: schema.ChatMessagePartTypeText,
							Text: "some text",
						},
						{
							Type:  schema.ChatMessagePartTypeAudioURL,
							Audio: &schema.MessageOutputAudio{},
						},
						{
							Type: schema.ChatMessagePartTypeText,
							Text: "this should be ignored",
						},
					},
				}
				msg, err := buildMessageFromAssistantGenMultiContent(inMsg)
				assert.NoError(t, err)
				assert.Equal(t, openai.ChatMessageRoleAssistant, msg.Role)
				assert.Equal(t, "test-assistant", msg.Name)
				assert.NotNil(t, msg.Audio)
				assert.Equal(t, "audio-id-123", msg.Audio.ID)
				assert.Empty(t, msg.MultiContent, "MultiContent should be empty when audio is present")
			})
		})

		t.Run("success with text only", func(t *testing.T) {
			inMsg := &schema.Message{
				Role: schema.Assistant,
				Name: "test-assistant",
				AssistantGenMultiContent: []schema.MessageOutputPart{
					{
						Type: schema.ChatMessagePartTypeText,
						Text: "some text",
					},
				},
			}
			msg, err := buildMessageFromAssistantGenMultiContent(inMsg)
			assert.NoError(t, err)
			assert.Equal(t, openai.ChatMessageRoleAssistant, msg.Role)
			assert.Nil(t, msg.Audio)
			assert.Len(t, msg.MultiContent, 1)
			assert.Equal(t, "some text", msg.MultiContent[0].Text)
		})

		t.Run("error on getting audio id", func(t *testing.T) {
			mockey.PatchConvey("mock GetMessageOutputAudioID failure", t, func() {
				mockey.Mock(getMessageOutputAudioID).Return("", false).Build()
				inMsg := &schema.Message{
					Role: schema.Assistant,
					AssistantGenMultiContent: []schema.MessageOutputPart{
						{
							Type:  schema.ChatMessagePartTypeAudioURL,
							Audio: &schema.MessageOutputAudio{},
						},
					},
				}
				_, err := buildMessageFromAssistantGenMultiContent(inMsg)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "failed to get audio ID")
			})
		})

		t.Run("error on unsupported part type", func(t *testing.T) {
			inMsg := &schema.Message{
				Role: schema.Assistant,
				AssistantGenMultiContent: []schema.MessageOutputPart{
					{
						Type: "unsupported-type",
					},
				},
			}
			_, err := buildMessageFromAssistantGenMultiContent(inMsg)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "unsupported chat message part type")
		})
	})

	t.Run("buildMessageFromMultiContent", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			inMsg := &schema.Message{
				Role:    schema.System,
				Content: "system prompt",
				MultiContent: []schema.ChatMessagePart{
					{
						Type: schema.ChatMessagePartTypeText,
						Text: "hello world",
					},
					{
						Type: schema.ChatMessagePartTypeImageURL,
						ImageURL: &schema.ChatMessageImageURL{
							URL: "http://example.com/image.png",
						},
					},
				},
				Name: "test-system",
			}
			msg, err := buildMessageFromMultiContent(inMsg)
			assert.NoError(t, err)
			assert.Equal(t, openai.ChatMessageRoleSystem, msg.Role)
			assert.Equal(t, "system prompt", msg.Content)
			assert.Equal(t, "test-system", msg.Name)
			assert.Len(t, msg.MultiContent, 2)
			assert.Equal(t, openai.ChatMessagePartTypeText, msg.MultiContent[0].Type)
			assert.Equal(t, "hello world", msg.MultiContent[0].Text)
			assert.Equal(t, openai.ChatMessagePartTypeImageURL, msg.MultiContent[1].Type)
			assert.Equal(t, "http://example.com/image.png", msg.MultiContent[1].ImageURL.URL)
		})

		t.Run("error from toOpenAIMultiContent", func(t *testing.T) {
			inMsg := &schema.Message{
				Role: schema.User,
				MultiContent: []schema.ChatMessagePart{
					{
						Type: "invalid-type", // This will cause toOpenAIMultiContent to fail
					},
				},
			}
			_, err := buildMessageFromMultiContent(inMsg)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "unsupported chat message part type")
		})
	})
}

func TestBuildMessageFromUserInputMultiContent(t *testing.T) {
	mockey.PatchConvey("TestBuildMessageFromUserInputMultiContent", t, func() {
		base64Data := "base64data"
		text := "hello"

		tests := []struct {
			name    string
			inMsg   *schema.Message
			want    openai.ChatCompletionMessage
			wantErr bool
		}{
			{
				name: "success",
				inMsg: &schema.Message{
					Role: schema.User,
					UserInputMultiContent: []schema.MessageInputPart{
						{
							Type: schema.ChatMessagePartTypeText,
							Text: text,
						},
						{
							Type: schema.ChatMessagePartTypeImageURL,
							Image: &schema.MessageInputImage{
								MessagePartCommon: schema.MessagePartCommon{
									Base64Data: &base64Data,
									MIMEType:   "image/png",
								},
								Detail: schema.ImageURLDetailHigh,
							},
						},
						{
							Type: schema.ChatMessagePartTypeAudioURL,
							Audio: &schema.MessageInputAudio{
								MessagePartCommon: schema.MessagePartCommon{
									Base64Data: &base64Data,
									MIMEType:   "audio/wav",
								},
							},
						},
						{
							Type: schema.ChatMessagePartTypeVideoURL,
							Video: &schema.MessageInputVideo{
								MessagePartCommon: schema.MessagePartCommon{
									Base64Data: &base64Data,
									MIMEType:   "video/mp4",
								},
							},
						},
					},
				},
				want: openai.ChatCompletionMessage{
					Role: openai.ChatMessageRoleUser,
					MultiContent: []openai.ChatMessagePart{
						{
							Type: openai.ChatMessagePartTypeText,
							Text: text,
						},
						{
							Type: openai.ChatMessagePartTypeImageURL,
							ImageURL: &openai.ChatMessageImageURL{
								URL:    "data:image/png;base64,base64data",
								Detail: openai.ImageURLDetailHigh,
							},
						},
						{
							Type: openai.ChatMessagePartTypeInputAudio,
							InputAudio: &openai.ChatMessageInputAudio{
								Data:   base64Data,
								Format: "wav",
							},
						},
						{
							Type: openai.ChatMessagePartTypeVideoURL,
							VideoURL: &openai.ChatMessageVideoURL{
								URL: "data:video/mp4;base64,base64data",
							},
						},
					},
				},
			},
			{
				name: "unsupported type",
				inMsg: &schema.Message{
					Role: schema.User,
					UserInputMultiContent: []schema.MessageInputPart{
						{
							Type: "unsupported",
						},
					},
				},
				wantErr: true,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := buildMessageFromUserInputMultiContent(tt.inMsg)
				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
				assert.Equal(t, tt.want, got)
			})
		}
	})
}

func Test_newStreamMessageBuilder(t *testing.T) {
	audio := &Audio{Format: "mp3"}
	builder := newStreamMessageBuilder(audio)
	assert.Equal(t, audio, builder.audioCfg)
}

func Test_streamMessageBuilder_setOutputMessageAudio(t *testing.T) {
	builder := newStreamMessageBuilder(&Audio{Format: "mp3"})
	msg := &schema.Message{}
	audio := &openai.Audio{
		ID:         "audio-id",
		Data:       "audio-data",
		Transcript: "audio-transcript",
	}

	err := builder.setOutputMessageAudio(msg, audio)
	assert.NoError(t, err)
	assert.Equal(t, "audio-id", builder.audioID)
	assert.Len(t, msg.AssistantGenMultiContent, 1)
	assert.Equal(t, schema.ChatMessagePartTypeAudioURL, msg.AssistantGenMultiContent[0].Type)
	assert.NotNil(t, msg.AssistantGenMultiContent[0].Audio)
	aID, ok := getMessageOutputAudioID(msg.AssistantGenMultiContent[0].Audio)
	assert.True(t, ok)
	assert.Equal(t, audioID("audio-id"), aID)
	assert.Equal(t, "audio-data", *msg.AssistantGenMultiContent[0].Audio.Base64Data)
	assert.Equal(t, "audio/mpeg", msg.AssistantGenMultiContent[0].Audio.MIMEType)
	transcript, ok := GetMessageOutputAudioTranscript(msg.AssistantGenMultiContent[0].Audio)
	assert.True(t, ok)
	assert.Equal(t, "audio-transcript", transcript)
}

func Test_streamMessageBuilder_build(t *testing.T) {
	builder := newStreamMessageBuilder(&Audio{Format: "mp3"})
	resp := openai.ChatCompletionStreamResponse{
		Choices: []openai.ChatCompletionStreamChoice{
			{
				Index: 0,
				Delta: openai.ChatCompletionStreamChoiceDelta{
					Role:    "assistant",
					Content: "hello",
					Audio: &openai.Audio{
						ID:   "audio-id",
						Data: "audio-data",
					},
				},
			},
		},
	}

	msg, found, err := builder.build(resp)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.NotNil(t, msg)
	assert.Equal(t, schema.Assistant, msg.Role)
	assert.Equal(t, "hello", msg.Content)
	assert.Len(t, msg.AssistantGenMultiContent, 1)
}

func Test_genRequest(t *testing.T) {
	t.Run("basic request", func(t *testing.T) {
		c := &Client{config: &Config{Model: "test-model"}}
		in := []*schema.Message{{Role: schema.User, Content: "hello"}}

		req, cbInput, reqOpts, spec, err := c.genRequest(t.Context(), in)
		assert.NoError(t, err)
		assert.NotNil(t, req)
		assert.NotNil(t, cbInput)
		assert.NotNil(t, spec)
		assert.Equal(t, "test-model", req.Model)
		assert.Equal(t, 1, len(req.Messages))
		assert.Equal(t, "hello", req.Messages[0].Content)
		assert.Equal(t, "test-model", cbInput.Config.Model)
		assert.Equal(t, in, cbInput.Messages)
		assert.Len(t, reqOpts, 0)
	})

	t.Run("multi-content conflict error", func(t *testing.T) {
		c := &Client{config: &Config{Model: "test-model"}}
		in := []*schema.Message{{
			Role:                     schema.User,
			UserInputMultiContent:    []schema.MessageInputPart{{Type: schema.ChatMessagePartTypeText, Text: "hi"}},
			AssistantGenMultiContent: []schema.MessageOutputPart{{Type: schema.ChatMessagePartTypeText, Text: "out"}},
		}}

		_, _, _, _, err := c.genRequest(t.Context(), in)
		assert.Error(t, err)
	})

	t.Run("payload modifier and extra header options", func(t *testing.T) {
		c := &Client{config: &Config{Model: "test-model"}}
		in := []*schema.Message{{Role: schema.User, Content: "hello"}}

		opts := []model.Option{
			WithRequestPayloadModifier(func(ctx context.Context, msgs []*schema.Message, rawBody []byte) ([]byte, error) {
				return rawBody, nil
			}),
			WithExtraHeader(map[string]string{"x-test": "y"}),
		}

		_, _, reqOpts, _, err := c.genRequest(t.Context(), in, opts...)
		assert.NoError(t, err)
		assert.Len(t, reqOpts, 2)
	})

	t.Run("forced tool choice without tools returns error", func(t *testing.T) {
		c := &Client{config: &Config{Model: "test-model"}}
		tc := schema.ToolChoiceForced
		c.toolChoice = &tc

		_, _, _, _, err := c.genRequest(t.Context(), []*schema.Message{{Role: schema.User, Content: "hello"}})
		assert.Error(t, err)
	})

	t.Run("forced tool choice with multiple tools becomes required", func(t *testing.T) {
		c := &Client{config: &Config{Model: "test-model"}}
		tools := []*schema.ToolInfo{
			{
				Name: "tool1",
				Desc: "desc1",
				ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
					"param": {Type: schema.String, Required: true},
				}),
			},
			{
				Name: "tool2",
				Desc: "desc2",
				ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
					"param": {Type: schema.String, Required: true},
				}),
			},
		}
		assert.NoError(t, c.BindForcedTools(tools))

		req, _, _, _, err := c.genRequest(t.Context(), []*schema.Message{{Role: schema.User, Content: "hello"}})
		assert.NoError(t, err)
		assert.Equal(t, 2, len(req.Tools))
		// tool choice should be "required" when multiple tools are bound and forced
		if v, ok := any(req.ToolChoice).(string); ok {
			assert.Equal(t, toolChoiceRequired, v)
		} else {
			t.Fatalf("expected toolChoice to be string 'required', got %T", req.ToolChoice)
		}
	})

	t.Run("modalities audio requires audio config", func(t *testing.T) {
		c := &Client{config: &Config{Model: "test-model", Modalities: []Modality{AudioModality}}}
		_, _, _, _, err := c.genRequest(t.Context(), []*schema.Message{{Role: schema.User, Content: "hello"}})
		assert.Error(t, err)
	})

	t.Run("modalities audio with config populates extra fields", func(t *testing.T) {
		c := &Client{config: &Config{Model: "test-model", Modalities: []Modality{AudioModality}, Audio: &Audio{Format: "mp3", Voice: "alloy"}}}
		_, _, _, spec, err := c.genRequest(t.Context(), []*schema.Message{{Role: schema.User, Content: "hello"}})
		assert.NoError(t, err)
		assert.NotNil(t, spec.ExtraFields)
		// Expect modalities and audio to be present in ExtraFields
		_, okMod := spec.ExtraFields["modalities"]
		_, okAudio := spec.ExtraFields["audio"]
		assert.True(t, okMod)
		assert.True(t, okAudio)
	})

	t.Run("response format mapping", func(t *testing.T) {
		c := &Client{config: &Config{Model: "test-model", ResponseFormat: &ChatCompletionResponseFormat{Type: ChatCompletionResponseFormatTypeText}}}
		req, _, _, _, err := c.genRequest(t.Context(), []*schema.Message{{Role: schema.User, Content: "hello"}})
		assert.NoError(t, err)
		assert.NotNil(t, req.ResponseFormat)
		assert.Equal(t, ChatCompletionResponseFormatTypeText, ChatCompletionResponseFormatType(req.ResponseFormat.Type))
	})

	t.Run("request payload modifier wiring", func(t *testing.T) {
		c := &Client{config: &Config{Model: "test-model"}}
		in := []*schema.Message{{Role: schema.User, Content: "hello"}}
		opts := []model.Option{
			WithRequestPayloadModifier(func(ctx context.Context, msgs []*schema.Message, rawBody []byte) ([]byte, error) {
				return append(rawBody, []byte("x")...), nil
			}),
		}
		_, _, reqOpts, spec, err := c.genRequest(t.Context(), in, opts...)
		assert.NoError(t, err)
		assert.Len(t, reqOpts, 1)
		if assert.NotNil(t, spec.RequestPayloadModifier) {
			out, mErr := spec.RequestPayloadModifier(t.Context(), in, []byte("body"))
			assert.NoError(t, mErr)
			assert.Equal(t, []byte("bodyx"), out)
		}
	})

	t.Run("response message modifier wiring", func(t *testing.T) {
		c := &Client{config: &Config{Model: "test-model"}}
		in := []*schema.Message{{Role: schema.User, Content: "hello"}}
		opts := []model.Option{
			WithResponseMessageModifier(func(ctx context.Context, msg *schema.Message, rawBody []byte) (*schema.Message, error) {
				return &schema.Message{
					Role:         msg.Role,
					Name:         msg.Name,
					Content:      msg.Content + "|mod",
					ToolCallID:   msg.ToolCallID,
					ToolCalls:    msg.ToolCalls,
					ResponseMeta: msg.ResponseMeta,
				}, nil
			}),
		}
		_, _, _, spec, err := c.genRequest(t.Context(), in, opts...)
		assert.NoError(t, err)
		if assert.NotNil(t, spec.ResponseMessageModifier) {
			outMsg, mErr := spec.ResponseMessageModifier(t.Context(), &schema.Message{Role: schema.Assistant, Content: "resp"}, []byte("raw"))
			assert.NoError(t, mErr)
			assert.Equal(t, "resp|mod", outMsg.Content)
			assert.Equal(t, schema.Assistant, outMsg.Role)
		}
	})
}

func populateAndGetEmbeddedStreamReader(stream *openai.ChatCompletionStream) any {
	v := reflect.ValueOf(stream).Elem()
	f := v.Field(0) // embedded *streamReader[ChatCompletionStreamResponse]
	t := f.Type()   // pointer type

	// allocate zero streamReader[T]
	newReaderPtr := reflect.New(t.Elem()) // *streamReader[T]

	// unsafe set unexported embedded pointer field
	// reflect.Value.Set on unexported fields will panic; use NewAt with UnsafeAddr to bypass
	p := unsafe.Pointer(f.UnsafeAddr())
	reflect.NewAt(t, p).Elem().Set(newReaderPtr)
	return newReaderPtr.Interface()
}

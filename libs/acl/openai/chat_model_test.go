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
	"math/rand"
	"testing"

	"github.com/bytedance/mockey"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	openai "github.com/meguminnnnnnnnn/go-openai"
	"github.com/stretchr/testify/assert"
)

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

func TestClientGetChatCompletionRequestOptions(t *testing.T) {
	cli := &Client{
		config: &Config{},
	}

	assert.Len(t, cli.getChatCompletionRequestOptions([]model.Option{
		WithRequestBodyModifier(func(rawBody []byte) ([]byte, error) {
			return rawBody, nil
		}),
	}), 1)
}

func TestClientWithExtraHeader(t *testing.T) {
	cli := &Client{
		config: &Config{},
	}

	assert.Len(t, cli.getChatCompletionRequestOptions([]model.Option{
		WithExtraHeader(map[string]string{"test": "test"}),
	}), 1)
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

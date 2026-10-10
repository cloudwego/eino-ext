/*
 * Copyright 2026 CloudWeGo Authors
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

package agenticopenai

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	. "github.com/bytedance/mockey"
	"github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/assert"

	"github.com/cloudwego/eino/schema"
)

func TestModel(t *testing.T) {
	PatchConvey("test Model", t, func() {
		ctx := context.Background()
		m, err := NewChatModel(ctx, nil)
		convey.So(err, convey.ShouldNotBeNil)

		m, err = NewChatModel(ctx, &ChatConfig{
			BaseURL: "https://api.openai.com/v1",
			APIKey:  "test-key",
			Model:   "gpt-4",
		})
		convey.So(err, convey.ShouldBeNil)
		convey.So(m, convey.ShouldNotBeNil)

		cli := m.cli

		PatchConvey("test Generate error", func() {
			Mock(GetMethod(cli, "Generate")).Return(nil, fmt.Errorf("mock err")).Build()
			msg, err := m.Generate(ctx, []*schema.AgenticMessage{
				{
					Role: schema.AgenticRoleTypeUser,
					ContentBlocks: []*schema.ContentBlock{
						schema.NewContentBlock(&schema.UserInputText{Text: "hello"}),
					},
				},
			})
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(msg, convey.ShouldBeNil)
		})

		PatchConvey("test Generate success", func() {
			extension := &ChatResponseMetaExtension{FinishReason: "stop"}
			mockResp := &schema.AgenticMessage{
				Role: schema.AgenticRoleTypeAssistant,
				ContentBlocks: []*schema.ContentBlock{
					schema.NewContentBlock(&schema.AssistantGenText{Text: "hi there"}),
				},
				Extra: map[string]any{
					extraKeyChatResponseMetaExtension: extension,
				},
				ResponseMeta: &schema.AgenticResponseMeta{Extension: extension},
			}
			Mock(GetMethod(cli, "Generate")).Return(mockResp, nil).Build()
			msg, err := m.Generate(ctx, []*schema.AgenticMessage{
				{
					Role: schema.AgenticRoleTypeUser,
					ContentBlocks: []*schema.ContentBlock{
						schema.NewContentBlock(&schema.UserInputText{Text: "hello"}),
					},
				},
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(msg, convey.ShouldNotBeNil)
			convey.So(msg.ResponseMeta, convey.ShouldNotBeNil)
			convey.So(msg.ResponseMeta.Extension, convey.ShouldNotBeNil)
			ext, ok := msg.ResponseMeta.Extension.(*ChatResponseMetaExtension)
			convey.So(ok, convey.ShouldBeTrue)
			convey.So(ext.FinishReason, convey.ShouldEqual, "stop")
		})

		PatchConvey("test Generate success without extension", func() {
			mockResp := &schema.AgenticMessage{
				Role: schema.AgenticRoleTypeAssistant,
				ContentBlocks: []*schema.ContentBlock{
					schema.NewContentBlock(&schema.AssistantGenText{Text: "hi there"}),
				},
			}
			Mock(GetMethod(cli, "Generate")).Return(mockResp, nil).Build()
			msg, err := m.Generate(ctx, []*schema.AgenticMessage{
				{
					Role: schema.AgenticRoleTypeUser,
					ContentBlocks: []*schema.ContentBlock{
						schema.NewContentBlock(&schema.UserInputText{Text: "hello"}),
					},
				},
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(msg, convey.ShouldNotBeNil)
		})

		PatchConvey("test Generate with custom headers option", func() {
			mockResp := &schema.AgenticMessage{
				Role: schema.AgenticRoleTypeAssistant,
				ContentBlocks: []*schema.ContentBlock{
					schema.NewContentBlock(&schema.AssistantGenText{Text: "reply"}),
				},
			}
			Mock(GetMethod(cli, "Generate")).Return(mockResp, nil).Build()
			msg, err := m.Generate(ctx, []*schema.AgenticMessage{
				{
					Role: schema.AgenticRoleTypeUser,
					ContentBlocks: []*schema.ContentBlock{
						schema.NewContentBlock(&schema.UserInputText{Text: "hello"}),
					},
				},
			}, WithCustomHeaders(map[string]string{"X-Custom": "value"}))
			convey.So(err, convey.ShouldBeNil)
			convey.So(msg, convey.ShouldNotBeNil)
		})

		PatchConvey("test Stream error", func() {
			Mock(GetMethod(cli, "Stream")).Return(nil, fmt.Errorf("mock err")).Build()
			sr, err := m.Stream(ctx, []*schema.AgenticMessage{
				{
					Role: schema.AgenticRoleTypeUser,
					ContentBlocks: []*schema.ContentBlock{
						schema.NewContentBlock(&schema.UserInputText{Text: "hello"}),
					},
				},
			})
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(sr, convey.ShouldBeNil)
		})

		PatchConvey("test Stream success", func() {
			extension := &ChatResponseMetaExtension{FinishReason: "stop"}
			chunks := []*schema.AgenticMessage{
				{
					Role: schema.AgenticRoleTypeAssistant,
					ContentBlocks: []*schema.ContentBlock{
						schema.NewContentBlock(&schema.AssistantGenText{Text: "hello"}),
					},
					Extra: map[string]any{
						extraKeyChatResponseMetaExtension: extension,
					},
					ResponseMeta: &schema.AgenticResponseMeta{Extension: extension},
				},
			}
			mockStream := schema.StreamReaderFromArray(chunks)
			Mock(GetMethod(cli, "Stream")).Return(mockStream, nil).Build()
			sr, err := m.Stream(ctx, []*schema.AgenticMessage{
				{
					Role: schema.AgenticRoleTypeUser,
					ContentBlocks: []*schema.ContentBlock{
						schema.NewContentBlock(&schema.UserInputText{Text: "hello"}),
					},
				},
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(sr, convey.ShouldNotBeNil)

			msg, err := sr.Recv()
			convey.So(err, convey.ShouldBeNil)
			convey.So(msg, convey.ShouldNotBeNil)
			convey.So(msg.ResponseMeta, convey.ShouldNotBeNil)
			ext, ok := msg.ResponseMeta.Extension.(*ChatResponseMetaExtension)
			convey.So(ok, convey.ShouldBeTrue)
			convey.So(ext.FinishReason, convey.ShouldEqual, "stop")

			_, err = sr.Recv()
			convey.So(err, convey.ShouldEqual, io.EOF)
		})

		PatchConvey("test GetType", func() {
			convey.So(m.GetType(), convey.ShouldEqual, "AgenticOpenAI/Chat")
		})

		PatchConvey("test IsCallbacksEnabled", func() {
			convey.So(m.IsCallbacksEnabled(), convey.ShouldBeTrue)
		})
	})
}

func TestNewModel(t *testing.T) {
	PatchConvey("test New with various configs", t, func() {
		ctx := context.Background()

		PatchConvey("default BaseURL", func() {
			m, err := NewChatModel(ctx, &ChatConfig{
				APIKey: "key",
				Model:  "gpt-4",
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(m, convey.ShouldNotBeNil)
		})

		PatchConvey("with Azure", func() {
			m, err := NewChatModel(ctx, &ChatConfig{
				APIKey:     "key",
				Model:      "gpt-4",
				ByAzure:    true,
				BaseURL:    "https://myresource.openai.azure.com",
				APIVersion: "2024-02-01",
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(m, convey.ShouldNotBeNil)
		})
	})
}

func TestParseCustomOptions(t *testing.T) {
	PatchConvey("test parseCustomOptions", t, func() {
		ctx := context.Background()

		PatchConvey("with custom headers", func() {
			m, err := NewChatModel(ctx, &ChatConfig{
				APIKey: "key",
				Model:  "gpt-4",
			})
			convey.So(err, convey.ShouldBeNil)
			opts := m.parseCustomOptions(WithCustomHeaders(map[string]string{"X-Key": "val"}))
			convey.So(len(opts), convey.ShouldBeGreaterThan, 0)
		})

		PatchConvey("with extra fields", func() {
			m, err := NewChatModel(ctx, &ChatConfig{
				APIKey: "key",
				Model:  "gpt-4",
			})
			convey.So(err, convey.ShouldBeNil)
			opts := m.parseCustomOptions(WithExtraFields(map[string]any{"key": "value"}))
			convey.So(len(opts), convey.ShouldBeGreaterThan, 0)
		})

		PatchConvey("with empty extra fields", func() {
			m, err := NewChatModel(ctx, &ChatConfig{
				APIKey:      "key",
				Model:       "gpt-4",
				ExtraFields: map[string]any{"default": "value"},
			})
			convey.So(err, convey.ShouldBeNil)
			emptyFields := map[string]any{}
			opts := m.parseCustomOptions(WithExtraFields(emptyFields))
			convey.So(len(opts), convey.ShouldEqual, 2)
		})

		PatchConvey("no custom options", func() {
			m, err := NewChatModel(ctx, &ChatConfig{
				APIKey: "key",
				Model:  "gpt-4",
			})
			convey.So(err, convey.ShouldBeNil)
			opts := m.parseCustomOptions()
			convey.So(len(opts), convey.ShouldEqual, 0)
		})
	})
}

type chatRoundTripFunc func(*http.Request) (*http.Response, error)

func (f chatRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestEmptyCustomHeadersReplaceDefaults(t *testing.T) {
	const responseBody = `{"id":"response-id","object":"chat.completion","created":1,` +
		`"model":"gpt-4","choices":[{"index":0,"message":{"role":"assistant",` +
		`"content":"ok"},"finish_reason":"stop"}]}`

	var requestHeaders http.Header
	transport := chatRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestHeaders = req.Header.Clone()
		body := strings.NewReader(responseBody)
		response := &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(body),
			Request:    req,
		}
		return response, nil
	})
	client := &http.Client{Transport: transport}
	config := &ChatConfig{
		BaseURL:       "https://example.com/v1",
		APIKey:        "key",
		Model:         "gpt-4",
		HTTPClient:    client,
		CustomHeaders: map[string]string{"X-Default": "value"},
	}
	m, err := NewChatModel(t.Context(), config)
	assert.NoError(t, err)
	input := []*schema.AgenticMessage{
		{
			Role: schema.AgenticRoleTypeUser,
			ContentBlocks: []*schema.ContentBlock{
				schema.NewContentBlock(&schema.UserInputText{Text: "hello"}),
			},
		},
	}
	emptyHeaders := map[string]string{}

	_, err = m.Generate(t.Context(), input, WithCustomHeaders(emptyHeaders))
	assert.NoError(t, err)
	assert.Empty(t, requestHeaders.Values("X-Default"))
}

func TestExtractChatResponseMetaExtension(t *testing.T) {
	PatchConvey("test applyAgenticResponseMetaExtension", t, func() {
		PatchConvey("nil Extra", func() {
			msg := &schema.AgenticMessage{}
			applyAgenticResponseMetaExtension(msg)
			convey.So(msg.ResponseMeta, convey.ShouldBeNil)
		})

		PatchConvey("Extra without extension key", func() {
			msg := &schema.AgenticMessage{
				Extra: map[string]any{"other_key": "value"},
			}
			applyAgenticResponseMetaExtension(msg)
			convey.So(msg.ResponseMeta, convey.ShouldBeNil)
		})

		PatchConvey("Extra with wrong type", func() {
			msg := &schema.AgenticMessage{
				Extra: map[string]any{extraKeyChatResponseMetaExtension: "wrong_type"},
			}
			applyAgenticResponseMetaExtension(msg)
			convey.So(msg.ResponseMeta, convey.ShouldBeNil)
		})

		PatchConvey("Extra with valid extension and nil ResponseMeta", func() {
			ext := &ChatResponseMetaExtension{FinishReason: "stop"}
			msg := &schema.AgenticMessage{
				Extra: map[string]any{extraKeyChatResponseMetaExtension: ext},
			}
			applyAgenticResponseMetaExtension(msg)
			convey.So(msg.ResponseMeta, convey.ShouldNotBeNil)
			convey.So(msg.ResponseMeta.Extension, convey.ShouldEqual, ext)
		})

		PatchConvey("Extra with valid extension and existing ResponseMeta", func() {
			ext := &ChatResponseMetaExtension{FinishReason: "length"}
			msg := &schema.AgenticMessage{
				Extra:        map[string]any{extraKeyChatResponseMetaExtension: ext},
				ResponseMeta: &schema.AgenticResponseMeta{},
			}
			applyAgenticResponseMetaExtension(msg)
			convey.So(msg.ResponseMeta.Extension, convey.ShouldEqual, ext)
		})
	})
}

func TestConcatChatResponseMetaExtensions(t *testing.T) {
	PatchConvey("test concatChatResponseMetaExtensions", t, func() {
		PatchConvey("empty chunks", func() {
			result, err := concatChatResponseMetaExtensions(nil)
			convey.So(err, convey.ShouldBeNil)
			convey.So(result, convey.ShouldBeNil)
		})

		PatchConvey("single chunk", func() {
			ext := &ChatResponseMetaExtension{FinishReason: "stop"}
			result, err := concatChatResponseMetaExtensions([]*ChatResponseMetaExtension{ext})
			convey.So(err, convey.ShouldBeNil)
			convey.So(result, convey.ShouldEqual, ext)
		})

		PatchConvey("multiple chunks", func() {
			logProbs := &schema.LogProbs{Content: []schema.LogProb{{Token: "a"}}}
			chunks := []*ChatResponseMetaExtension{
				{FinishReason: ""},
				{FinishReason: "stop", LogProbs: logProbs},
			}
			result, err := concatChatResponseMetaExtensions(chunks)
			convey.So(err, convey.ShouldBeNil)
			convey.So(result.FinishReason, convey.ShouldEqual, "stop")
			convey.So(result.LogProbs, convey.ShouldEqual, logProbs)
		})

		PatchConvey("multiple chunks with overwrite", func() {
			chunks := []*ChatResponseMetaExtension{
				{FinishReason: "length"},
				{FinishReason: "stop"},
			}
			result, err := concatChatResponseMetaExtensions(chunks)
			convey.So(err, convey.ShouldBeNil)
			convey.So(result.FinishReason, convey.ShouldEqual, "stop")
		})

		PatchConvey("multiple chunks append logprobs", func() {
			chunks := []*ChatResponseMetaExtension{
				{LogProbs: &schema.LogProbs{Content: []schema.LogProb{{Token: "a"}}}},
				{LogProbs: &schema.LogProbs{Content: []schema.LogProb{{Token: "b"}}}},
			}
			result, err := concatChatResponseMetaExtensions(chunks)
			convey.So(err, convey.ShouldBeNil)
			convey.So(result.LogProbs, convey.ShouldNotBeNil)
			convey.So(result.LogProbs.Content, convey.ShouldResemble, []schema.LogProb{
				{Token: "a"},
				{Token: "b"},
			})
		})
	})
}

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
	"errors"
	"fmt"
	"io"
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	. "github.com/bytedance/mockey"
	"github.com/smartystreets/goconvey/convey"

	"github.com/cloudwego/eino/schema"
)

func TestChatModelStreamTimeoutDoesNotInterruptActiveStream(t *testing.T) {
	server := httptest.NewServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(nethttp.Flusher)
		if !ok {
			nethttp.Error(w, "streaming unsupported", nethttp.StatusInternalServerError)
			return
		}

		fmt.Fprint(w, `data: {"id":"chatcmpl-test","object":"chat.completion.chunk","created":1,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"hello"},"finish_reason":null}]}`+"\n\n")
		flusher.Flush()

		time.Sleep(50 * time.Millisecond)

		fmt.Fprint(w, `data: {"id":"chatcmpl-test","object":"chat.completion.chunk","created":1,"model":"gpt-4","choices":[{"index":0,"delta":{"content":" world"},"finish_reason":null}]}`+"\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	ctx := context.Background()
	m, err := NewChatModel(ctx, &ChatConfig{
		BaseURL:               server.URL,
		APIKey:                "test-key",
		Model:                 "gpt-4",
		ResponseHeaderTimeout: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}

	stream, err := m.Stream(ctx, []*schema.AgenticMessage{
		{
			Role: schema.AgenticRoleTypeUser,
			ContentBlocks: []*schema.ContentBlock{
				schema.NewContentBlock(&schema.UserInputText{Text: "hello"}),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()

	var got strings.Builder
	for {
		msg, recvErr := stream.Recv()
		if errors.Is(recvErr, io.EOF) {
			break
		}
		if recvErr != nil {
			t.Fatalf("stream should not be interrupted by response header timeout: %v", recvErr)
		}
		for _, block := range msg.ContentBlocks {
			if block.AssistantGenText != nil {
				got.WriteString(block.AssistantGenText.Text)
			}
		}
	}

	if got.String() != "hello world" {
		t.Fatalf("unexpected stream content: %q", got.String())
	}
}

func TestChatModelTimeoutConfig(t *testing.T) {
	timeout := time.Second
	responseHeaderTimeout := 3 * time.Second

	client := newHTTPClient(timeout, responseHeaderTimeout)
	if client.Timeout != timeout {
		t.Fatalf("expected http.Client.Timeout to keep Timeout config, got %s", client.Timeout)
	}
	transport, ok := client.Transport.(*nethttp.Transport)
	if !ok {
		t.Fatalf("expected *http.Transport, got %T", client.Transport)
	}
	if transport.ResponseHeaderTimeout != responseHeaderTimeout {
		t.Fatalf("unexpected ResponseHeaderTimeout: %s", transport.ResponseHeaderTimeout)
	}
}

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
			mockResp := &schema.AgenticMessage{
				Role: schema.AgenticRoleTypeAssistant,
				ContentBlocks: []*schema.ContentBlock{
					schema.NewContentBlock(&schema.AssistantGenText{Text: "hi there"}),
				},
				Extra: map[string]any{
					extraKeyChatResponseMetaExtension: &ChatResponseMetaExtension{
						FinishReason: "stop",
					},
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
			chunks := []*schema.AgenticMessage{
				{
					Role: schema.AgenticRoleTypeAssistant,
					ContentBlocks: []*schema.ContentBlock{
						schema.NewContentBlock(&schema.AssistantGenText{Text: "hello"}),
					},
					Extra: map[string]any{
						extraKeyChatResponseMetaExtension: &ChatResponseMetaExtension{
							FinishReason: "stop",
						},
					},
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

func TestExtractChatResponseMetaExtension(t *testing.T) {
	PatchConvey("test extractChatResponseMetaExtension", t, func() {
		PatchConvey("nil Extra", func() {
			msg := &schema.AgenticMessage{}
			extractChatResponseMetaExtension(msg)
			convey.So(msg.ResponseMeta, convey.ShouldBeNil)
		})

		PatchConvey("Extra without extension key", func() {
			msg := &schema.AgenticMessage{
				Extra: map[string]any{"other_key": "value"},
			}
			extractChatResponseMetaExtension(msg)
			convey.So(msg.ResponseMeta, convey.ShouldBeNil)
		})

		PatchConvey("Extra with wrong type", func() {
			msg := &schema.AgenticMessage{
				Extra: map[string]any{extraKeyChatResponseMetaExtension: "wrong_type"},
			}
			extractChatResponseMetaExtension(msg)
			convey.So(msg.ResponseMeta, convey.ShouldBeNil)
		})

		PatchConvey("Extra with valid extension and nil ResponseMeta", func() {
			ext := &ChatResponseMetaExtension{FinishReason: "stop"}
			msg := &schema.AgenticMessage{
				Extra: map[string]any{extraKeyChatResponseMetaExtension: ext},
			}
			extractChatResponseMetaExtension(msg)
			convey.So(msg.ResponseMeta, convey.ShouldNotBeNil)
			convey.So(msg.ResponseMeta.Extension, convey.ShouldEqual, ext)
		})

		PatchConvey("Extra with valid extension and existing ResponseMeta", func() {
			ext := &ChatResponseMetaExtension{FinishReason: "length"}
			msg := &schema.AgenticMessage{
				Extra:        map[string]any{extraKeyChatResponseMetaExtension: ext},
				ResponseMeta: &schema.AgenticResponseMeta{},
			}
			extractChatResponseMetaExtension(msg)
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

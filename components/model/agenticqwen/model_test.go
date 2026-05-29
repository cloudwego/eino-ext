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

package agenticqwen

import (
	"context"
	"fmt"
	"io"
	"testing"

	. "github.com/bytedance/mockey"
	"github.com/smartystreets/goconvey/convey"

	"github.com/cloudwego/eino/schema"
)

func TestModel(t *testing.T) {
	PatchConvey("test Model", t, func() {
		ctx := context.Background()
		m, err := New(ctx, nil)
		convey.So(err, convey.ShouldNotBeNil)

		m, err = New(ctx, &Config{
			BaseURL: "asd",
			APIKey:  "qwe",
			Model:   "zxc",
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
					extraKeyResponseMetaExtension: &ResponseMetaExtension{
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
			ext, ok := msg.ResponseMeta.Extension.(*ResponseMetaExtension)
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

		PatchConvey("test Generate with thinking options", func() {
			mockResp := &schema.AgenticMessage{
				Role: schema.AgenticRoleTypeAssistant,
				ContentBlocks: []*schema.ContentBlock{
					schema.NewContentBlock(&schema.AssistantGenText{Text: "thinking reply"}),
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
			}, WithEnableThinking(true), WithPreserveThinking(true))
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
						extraKeyResponseMetaExtension: &ResponseMetaExtension{
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
			ext, ok := msg.ResponseMeta.Extension.(*ResponseMetaExtension)
			convey.So(ok, convey.ShouldBeTrue)
			convey.So(ext.FinishReason, convey.ShouldEqual, "stop")

			_, err = sr.Recv()
			convey.So(err, convey.ShouldEqual, io.EOF)
		})

		PatchConvey("test GetType", func() {
			convey.So(m.GetType(), convey.ShouldEqual, "AgenticQwen")
		})

		PatchConvey("test IsCallbacksEnabled", func() {
			convey.So(m.IsCallbacksEnabled(), convey.ShouldBeTrue)
		})
	})
}

func TestNew(t *testing.T) {
	PatchConvey("test New with various configs", t, func() {
		ctx := context.Background()

		PatchConvey("default BaseURL", func() {
			m, err := New(ctx, &Config{
				APIKey: "key",
				Model:  "model",
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(m, convey.ShouldNotBeNil)
		})

		PatchConvey("with Modalities", func() {
			m, err := New(ctx, &Config{
				APIKey:     "key",
				Model:      "model",
				Modalities: []Modality{ModalityText, ModalityAudio},
				Audio:      &AudioConfig{Format: AudioFormatWav, Voice: AudioVoiceCherry},
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(m, convey.ShouldNotBeNil)
		})

		PatchConvey("with EnableThinking", func() {
			enable := true
			preserve := false
			m, err := New(ctx, &Config{
				APIKey:           "key",
				Model:            "model",
				EnableThinking:   &enable,
				PreserveThinking: &preserve,
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(m, convey.ShouldNotBeNil)
		})
	})
}

func TestParseCustomOptions(t *testing.T) {
	PatchConvey("test parseCustomOptions", t, func() {
		ctx := context.Background()

		PatchConvey("with enable thinking from config", func() {
			enable := true
			m, err := New(ctx, &Config{
				APIKey:         "key",
				Model:          "model",
				EnableThinking: &enable,
			})
			convey.So(err, convey.ShouldBeNil)
			opts := m.parseCustomOptions()
			convey.So(len(opts), convey.ShouldBeGreaterThan, 0)
		})

		PatchConvey("with preserve thinking from config", func() {
			preserve := true
			m, err := New(ctx, &Config{
				APIKey:           "key",
				Model:            "model",
				PreserveThinking: &preserve,
			})
			convey.So(err, convey.ShouldBeNil)
			opts := m.parseCustomOptions()
			convey.So(len(opts), convey.ShouldBeGreaterThan, 0)
		})

		PatchConvey("with option overrides", func() {
			m, err := New(ctx, &Config{
				APIKey: "key",
				Model:  "model",
			})
			convey.So(err, convey.ShouldBeNil)
			opts := m.parseCustomOptions(WithEnableThinking(true), WithPreserveThinking(true))
			convey.So(len(opts), convey.ShouldBeGreaterThan, 0)
		})

		PatchConvey("no custom options", func() {
			m, err := New(ctx, &Config{
				APIKey: "key",
				Model:  "model",
			})
			convey.So(err, convey.ShouldBeNil)
			opts := m.parseCustomOptions()
			convey.So(len(opts), convey.ShouldEqual, 0)
		})
	})
}

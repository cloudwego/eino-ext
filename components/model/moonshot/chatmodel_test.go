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

package moonshot

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	. "github.com/bytedance/mockey"
	"github.com/smartystreets/goconvey/convey"

	"github.com/cloudwego/eino/schema"
)

func TestChatModel(t *testing.T) {
	PatchConvey("test ChatModel", t, func() {
		ctx := context.Background()

		PatchConvey("nil config", func() {
			cm, err := NewChatModel(ctx, nil)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(cm, convey.ShouldBeNil)
		})

		PatchConvey("default base url applied", func() {
			cm, err := NewChatModel(ctx, &ChatModelConfig{
				APIKey: "k",
				Model:  "m",
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(cm, convey.ShouldNotBeNil)
			convey.So(cm.GetType(), convey.ShouldEqual, "Moonshot")
		})

		PatchConvey("custom http client and base url", func() {
			cm, err := NewChatModel(ctx, &ChatModelConfig{
				BaseURL:    "https://example.com/v1",
				APIKey:     "k",
				Model:      "m",
				HTTPClient: &http.Client{Timeout: time.Second},
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(cm, convey.ShouldNotBeNil)
			convey.So(cm.IsCallbacksEnabled(), convey.ShouldEqual, cm.cli.IsCallbacksEnabled())
		})

		PatchConvey("with all standard options", func() {
			cm, err := NewChatModel(ctx, &ChatModelConfig{
				APIKey:           "k",
				Model:            "m",
				MaxTokens:        ptr(128),
				Temperature:      ptr(float32(0.5)),
				TopP:             ptr(float32(0.9)),
				Stop:             []string{"\n"},
				PresencePenalty:  ptr(float32(0.1)),
				FrequencyPenalty: ptr(float32(0.1)),
				User:             ptr("u"),
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(cm, convey.ShouldNotBeNil)
		})
	})
}

func TestChatModelDelegation(t *testing.T) {
	PatchConvey("test Generate / Stream / Bind delegation", t, func() {
		ctx := context.Background()
		cm, err := NewChatModel(ctx, &ChatModelConfig{
			APIKey: "k",
			Model:  "m",
		})
		convey.So(err, convey.ShouldBeNil)
		cli := cm.cli

		PatchConvey("Generate forwards error", func() {
			Mock(GetMethod(cli, "Generate")).Return(nil, fmt.Errorf("mock err")).Build()
			msg, err := cm.Generate(ctx, []*schema.Message{schema.UserMessage("hi")})
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(msg, convey.ShouldBeNil)
		})

		PatchConvey("Stream forwards error", func() {
			Mock(GetMethod(cli, "Stream")).Return(nil, fmt.Errorf("mock err")).Build()
			sr, err := cm.Stream(ctx, []*schema.Message{schema.UserMessage("hi")})
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(sr, convey.ShouldBeNil)
		})

		PatchConvey("BindTools forwards", func() {
			err := cm.BindTools([]*schema.ToolInfo{{Name: "t"}})
			convey.So(err, convey.ShouldBeNil)
		})

		PatchConvey("BindForcedTools forwards", func() {
			err := cm.BindForcedTools([]*schema.ToolInfo{{Name: "t"}})
			convey.So(err, convey.ShouldBeNil)
		})

		PatchConvey("WithTools returns a new model with the bound client", func() {
			tcm, err := cm.WithTools([]*schema.ToolInfo{{Name: "t"}})
			convey.So(err, convey.ShouldBeNil)
			convey.So(tcm, convey.ShouldNotBeNil)
		})

		PatchConvey("WithTools forwards error", func() {
			Mock(GetMethod(cli, "WithToolsForClient")).Return(nil, fmt.Errorf("mock err")).Build()
			tcm, err := cm.WithTools([]*schema.ToolInfo{{Name: "t"}})
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(tcm, convey.ShouldBeNil)
		})
	})
}

func TestOptions(t *testing.T) {
	convey.Convey("WithExtraHeader returns option", t, func() {
		opt := WithExtraHeader(map[string]string{"X-A": "1"})
		convey.So(opt, convey.ShouldNotBeNil)
	})
	convey.Convey("WithExtraFields returns option", t, func() {
		opt := WithExtraFields(map[string]any{"partial": true})
		convey.So(opt, convey.ShouldNotBeNil)
	})
}

func ptr[T any](v T) *T { return &v }

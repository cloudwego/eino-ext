/*
 * Copyright 2025 CloudWeGo Authors
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

package spark

import (
	"context"
	"fmt"
	"testing"

	. "github.com/bytedance/mockey"
	"github.com/smartystreets/goconvey/convey"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func TestChatModel(t *testing.T) {
	PatchConvey("test ChatModel", t, func() {
		ctx := context.Background()

		cm, err := NewChatModel(ctx, nil)
		convey.So(err, convey.ShouldNotBeNil)
		convey.So(cm, convey.ShouldBeNil)

		cm, err = NewChatModel(ctx, &ChatModelConfig{
			APIKey: "qwe",
		})
		convey.So(err, convey.ShouldBeNil)
		convey.So(cm, convey.ShouldNotBeNil)
		convey.So(cm.GetType(), convey.ShouldEqual, "Spark")

		cli := cm.cli

		PatchConvey("test Generate success", func() {
			Mock(GetMethod(cli, "Generate")).Return(schema.UserMessage("hi"), nil).Build()
			msg, err := cm.Generate(ctx, []*schema.Message{
				schema.UserMessage("hello"),
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(msg, convey.ShouldNotBeNil)
		})

		PatchConvey("test Generate error", func() {
			Mock(GetMethod(cli, "Generate")).Return(nil, fmt.Errorf("mock err")).Build()
			msg, err := cm.Generate(ctx, []*schema.Message{
				schema.UserMessage("hello"),
			})
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(msg, convey.ShouldBeNil)
		})

		PatchConvey("test Generate validate error", func() {
			_, err := cm.Generate(ctx,
				[]*schema.Message{schema.UserMessage("hello")},
				model.WithToolChoice(schema.ToolChoiceForced, "t1", "t2"),
				model.WithTools([]*schema.ToolInfo{{Name: "t1"}, {Name: "t2"}}),
			)
			convey.So(err, convey.ShouldNotBeNil)
		})

		PatchConvey("test Stream error", func() {
			Mock(GetMethod(cli, "Stream")).Return(nil, fmt.Errorf("mock err")).Build()
			sr, err := cm.Stream(ctx, []*schema.Message{
				schema.UserMessage("hello"),
			})
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(sr, convey.ShouldBeNil)
		})

		PatchConvey("test Stream validate error", func() {
			_, err := cm.Stream(ctx,
				[]*schema.Message{schema.UserMessage("hello")},
				model.WithToolChoice(schema.ToolChoiceForced, "t1", "t2"),
				model.WithTools([]*schema.ToolInfo{{Name: "t1"}, {Name: "t2"}}),
			)
			convey.So(err, convey.ShouldNotBeNil)
		})

		PatchConvey("test WithTools", func() {
			Mock(GetMethod(cli, "WithToolsForClient")).Return(cli, nil).Build()
			ncm, err := cm.WithTools([]*schema.ToolInfo{{Name: "t1"}})
			convey.So(err, convey.ShouldBeNil)
			convey.So(ncm, convey.ShouldNotBeNil)
		})

		PatchConvey("test WithTools error", func() {
			Mock(GetMethod(cli, "WithToolsForClient")).Return(nil, fmt.Errorf("mock err")).Build()
			ncm, err := cm.WithTools([]*schema.ToolInfo{{Name: "t1"}})
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(ncm, convey.ShouldBeNil)
		})

		PatchConvey("test BindTools", func() {
			Mock(GetMethod(cli, "BindTools")).Return(nil).Build()
			convey.So(cm.BindTools([]*schema.ToolInfo{{Name: "t1"}}), convey.ShouldBeNil)
		})

		PatchConvey("test BindForcedTools", func() {
			Mock(GetMethod(cli, "BindForcedTools")).Return(nil).Build()
			convey.So(cm.BindForcedTools([]*schema.ToolInfo{{Name: "t1"}}), convey.ShouldBeNil)
		})

		PatchConvey("test IsCallbacksEnabled", func() {
			Mock(GetMethod(cli, "IsCallbacksEnabled")).Return(true).Build()
			convey.So(cm.IsCallbacksEnabled(), convey.ShouldBeTrue)
		})
	})
}

func TestDefaults(t *testing.T) {
	PatchConvey("test config defaults", t, func() {
		ctx := context.Background()

		convey.Convey("empty base url and model fall back to defaults", func() {
			cm, err := NewChatModel(ctx, &ChatModelConfig{APIKey: "qwe"})
			convey.So(err, convey.ShouldBeNil)
			convey.So(cm, convey.ShouldNotBeNil)
		})

		convey.Convey("explicit base url and model are accepted", func() {
			cm, err := NewChatModel(ctx, &ChatModelConfig{
				APIKey:  "qwe",
				BaseURL: "https://spark-api-open.xf-yun.com/v1",
				Model:   "generalv3.5",
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(cm, convey.ShouldNotBeNil)
		})
	})
}

func TestOptions(t *testing.T) {
	PatchConvey("test options", t, func() {
		convey.So(WithExtraHeader(map[string]string{"k": "v"}), convey.ShouldNotBeNil)
		convey.So(WithExtraFields(map[string]any{"k": "v"}), convey.ShouldNotBeNil)
	})
}

func TestValidateToolOptions(t *testing.T) {
	PatchConvey("test validateToolOptions", t, func() {
		convey.Convey("no options", func() {
			err := validateToolOptions()
			convey.So(err, convey.ShouldBeNil)
		})

		convey.Convey("tool_choice 'allowed' with allowed_tools", func() {
			toolChoice := schema.ToolChoiceAllowed
			err := validateToolOptions(
				model.WithToolChoice(toolChoice, "tool1"),
				model.WithTools([]*schema.ToolInfo{{Name: "tool1"}}),
			)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldEqual, "tool_choice 'allowed' is not supported when allowed tool names are present")
		})

		convey.Convey("tool_choice 'forced' with more than one allowed_tool", func() {
			toolChoice := schema.ToolChoiceForced
			err := validateToolOptions(
				model.WithToolChoice(toolChoice, "tool1", "tool2"),
				model.WithTools([]*schema.ToolInfo{
					{Name: "tool1"},
					{Name: "tool2"},
				}),
			)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldEqual, "only one allowed tool name can be configured for tool_choice 'forced'")
		})

		convey.Convey("tool_choice 'forced' with one allowed_tool", func() {
			toolChoice := schema.ToolChoiceForced
			err := validateToolOptions(
				model.WithToolChoice(toolChoice),
				model.WithTools([]*schema.ToolInfo{{Name: "tool1"}}),
			)
			convey.So(err, convey.ShouldBeNil)
		})

		convey.Convey("tool_choice not set", func() {
			err := validateToolOptions(model.WithTools([]*schema.ToolInfo{{Name: "tool1"}}))
			convey.So(err, convey.ShouldBeNil)
		})
	})
}

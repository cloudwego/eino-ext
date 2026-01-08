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

package qwen

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

		cm, err = NewChatModel(ctx, &ChatModelConfig{
			BaseURL: "asd",
			APIKey:  "qwe",
			Model:   "zxc",
		})
		convey.So(err, convey.ShouldBeNil)
		convey.So(cm, convey.ShouldNotBeNil)

		cli := cm.cli

		PatchConvey("test Generate", func() {
			Mock(GetMethod(cli, "Generate")).Return(nil, fmt.Errorf("mock err")).Build()
			msg, err := cm.Generate(ctx, []*schema.Message{
				schema.UserMessage("hello"),
			})
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(msg, convey.ShouldBeNil)
		})

		PatchConvey("test Stream", func() {
			Mock(GetMethod(cli, "Stream")).Return(nil, fmt.Errorf("mock err")).Build()
			sr, err := cm.Stream(ctx, []*schema.Message{
				schema.UserMessage("hello"),
			})
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(sr, convey.ShouldBeNil)
		})
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

		convey.Convey("tool_choice 'allowed' without allowed_tools", func() {
			toolChoice := schema.ToolChoiceAllowed
			err := validateToolOptions(model.WithToolChoice(toolChoice))
			convey.So(err, convey.ShouldBeNil)
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

		convey.Convey("tool_choice 'forced' without allowed_tools", func() {
			toolChoice := schema.ToolChoiceForced
			err := validateToolOptions(model.WithToolChoice(toolChoice))
			convey.So(err, convey.ShouldBeNil)
		})

		convey.Convey("tool_choice not set", func() {
			err := validateToolOptions(model.WithTools([]*schema.ToolInfo{{Name: "tool1"}}))
			convey.So(err, convey.ShouldBeNil)
		})
	})
}

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

package agenticdeepseek

import (
	"context"
	"fmt"
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
			BaseURL: "https://api.deepseek.com/v1",
			APIKey:  "test-key",
			Model:   "deepseek-chat",
		})
		convey.So(err, convey.ShouldBeNil)
		convey.So(m, convey.ShouldNotBeNil)

		cli := m.cli

		PatchConvey("test Generate", func() {
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

		PatchConvey("test Stream", func() {
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
	})
}

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

package glm

import (
	"testing"

	"github.com/smartystreets/goconvey/convey"

	"github.com/cloudwego/eino/components/model"
)

func TestOptions(t *testing.T) {
	convey.Convey("test glm impl-specific options", t, func() {
		convey.Convey("WithThinking enabled", func() {
			o := model.GetImplSpecificOptions(&options{}, WithThinking(true))
			convey.So(o.EnableThinking, convey.ShouldNotBeNil)
			convey.So(*o.EnableThinking, convey.ShouldBeTrue)
		})

		convey.Convey("WithThinking disabled", func() {
			o := model.GetImplSpecificOptions(&options{}, WithThinking(false))
			convey.So(o.EnableThinking, convey.ShouldNotBeNil)
			convey.So(*o.EnableThinking, convey.ShouldBeFalse)
		})

		convey.Convey("WithWebSearch injects web_search tool", func() {
			o := model.GetImplSpecificOptions(&options{}, WithWebSearch(true))
			convey.So(o.extraFields, convey.ShouldContainKey, "tools")
		})

		convey.Convey("WithExtraFields merges fields", func() {
			o := model.GetImplSpecificOptions(&options{},
				WithExtraFields(map[string]any{"foo": "bar"}))
			convey.So(o.extraFields["foo"], convey.ShouldEqual, "bar")
		})
	})
}

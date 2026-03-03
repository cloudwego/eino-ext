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

package cozeloop

import (
	"context"
	"testing"

	"github.com/bytedance/mockey"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/smartystreets/goconvey/convey"
)

// Test_spanTags_setTags 为 spanTags 的 setTags 方法编写单元测试
func Test_spanTags_setTags(t *testing.T) {
	mockey.PatchConvey("测试 spanTags 的 setTags 方法", t, func() {
		mockey.PatchConvey("传入非空的 kv 映射", func() {
			// 初始化 spanTags
			tags := spanTags{}
			// 定义要设置的键值对
			kv := map[string]any{
				"key1": "value1",
				"key2": 2,
			}
			// 调用 setTags 方法
			result := tags.setTags(kv)
			// 断言结果类型正确
			convey.So(result, convey.ShouldHaveSameTypeAs, spanTags{})
			// 断言结果包含传入的键值对
			for k, v := range kv {
				convey.So(result, convey.ShouldContainKey, k)
				convey.So(result[k], convey.ShouldEqual, v)
			}
		})

		mockey.PatchConvey("传入空的 kv 映射", func() {
			// 初始化 spanTags
			tags := spanTags{
				"existingKey": "existingValue",
			}
			// 定义空的键值对
			kv := map[string]any{}
			// 调用 setTags 方法
			result := tags.setTags(kv)
			// 断言结果类型正确
			convey.So(result, convey.ShouldHaveSameTypeAs, spanTags{})
			// 断言结果保持不变
			convey.So(result, convey.ShouldResemble, tags)
		})
	})
}

func Test_spanTags_set(t *testing.T) {
	mockey.PatchConvey("测试spanTags的set方法", t, func() {
		mockey.PatchConvey("当spanTags为nil时", func() {
			// Arrange
			var tags spanTags
			key := "testKey"
			value := "testValue"

			// Act
			result := tags.set(key, value)

			// Assert
			convey.So(result, convey.ShouldBeNil)
		})

		mockey.PatchConvey("当value为nil时", func() {
			// Arrange
			tags := spanTags{}
			key := "testKey"
			var value any = nil

			// Act
			result := tags.set(key, value)

			// Assert
			convey.So(result, convey.ShouldResemble, tags)
		})

		mockey.PatchConvey("当key已经存在时", func() {
			// Arrange
			tags := spanTags{"testKey": "oldValue"}
			key := "testKey"
			value := "newValue"

			// Act
			result := tags.set(key, value)

			// Assert
			convey.So(result, convey.ShouldResemble, tags)
			convey.So(result[key], convey.ShouldEqual, "oldValue")
		})

		mockey.PatchConvey("当value为复杂类型时，调用toJson转换", func() {
			// Arrange
			tags := spanTags{}
			key := "testKey"
			value := map[string]string{"innerKey": "innerValue"}
			expectedJson := `{"innerKey": "innerValue"}`
			// Mock toJson函数
			mockToJson := mockey.Mock(toJson).Return(expectedJson).Build()
			defer mockToJson.UnPatch()

			// Act
			result := tags.set(key, value)

			// Assert
			convey.So(result[key], convey.ShouldEqual, expectedJson)
		})

		mockey.PatchConvey("当value为简单类型时，直接设置", func() {
			// Arrange
			tags := spanTags{}
			key := "testKey"
			value := "testValue"

			// Act
			result := tags.set(key, value)

			// Assert
			convey.So(result[key], convey.ShouldEqual, value)
		})
	})
}

func Test_injectToolIDNameMapToCtx(t *testing.T) {
	mockey.PatchConvey("测试injectToolIDNameMapToCtx函数", t, func() {
		mockey.PatchConvey("当ctx为nil时", func() {
			ctx := context.Background()
			var input callbacks.CallbackInput
			input = &schema.Message{
				ToolCalls: []schema.ToolCall{
					{ID: "tool1", Function: schema.FunctionCall{Name: "name1"}},
					{ID: "tool2", Function: schema.FunctionCall{Name: "name2"}},
				},
			}
			result := injectToolIDNameMapToCtx(ctx, &callbacks.RunInfo{
				Component: compose.ComponentOfToolsNode,
			}, input)

			m := getToolIDNameMapFromCtx(result)
			convey.So(m, convey.ShouldNotBeNil)
			convey.So(m["tool1"], convey.ShouldEqual, "name1")
			convey.So(m["tool2"], convey.ShouldEqual, "name2")
		})
	})
}

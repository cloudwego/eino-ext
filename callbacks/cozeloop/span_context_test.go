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
	"github.com/smartystreets/goconvey/convey"
)

// 覆盖 spanContext 的 Getter 方法
func Test_spanContext_Getters(t *testing.T) {
	mockey.PatchConvey("spanContext Getter 方法", t, func() {
		mockey.PatchConvey("正常返回 spanID/traceID/baggage", func() {
			s := &spanContext{
				spanID:  "sid-001",
				traceID: "tid-001",
				baggage: map[string]string{"k": "v"},
			}
			convey.So(s.GetSpanID(), convey.ShouldEqual, "sid-001")
			convey.So(s.GetTraceID(), convey.ShouldEqual, "tid-001")
			convey.So(s.GetBaggage(), convey.ShouldResemble, map[string]string{"k": "v"})
		})

		mockey.PatchConvey("baggage 为 nil 时返回 nil", func() {
			s := &spanContext{}
			convey.So(s.GetBaggage(), convey.ShouldBeNil)
		})
	})
}

// 覆盖 WithSpanContext 及 getSpanContextImpl 的所有分支
func Test_WithSpanContext_and_getSpanContextImpl(t *testing.T) {
	mockey.PatchConvey("WithSpanContext 与 getSpanContextImpl", t, func() {
		mockey.PatchConvey("WithSpanContext 正常注入并可取回", func() {
			ctx := context.Background()
			newCtx := WithSpanContext(ctx)
			convey.So(newCtx, convey.ShouldNotBeNil)

			sc := getSpanContextImpl(newCtx)
			convey.So(sc, convey.ShouldNotBeNil)

			s := GetSpanContext(newCtx)
			convey.So(s, convey.ShouldNotBeNil)
		})

		mockey.PatchConvey("getSpanContextImpl: ctx 未设置时返回 nil", func() {
			ctx := context.Background()
			sc := getSpanContextImpl(ctx)
			convey.So(sc, convey.ShouldBeNil)
		})

		mockey.PatchConvey("getSpanContextImpl: ctx 中类型不匹配时返回 nil", func() {
			ctx := context.WithValue(context.Background(), spanContextKey{}, "not-span-context")
			sc := getSpanContextImpl(ctx)
			convey.So(sc, convey.ShouldBeNil)
		})

		mockey.PatchConvey("getSpanContextImpl: ctx 中为 *spanContext 时返回实例", func() {
			ctx := context.Background()
			// 手动注入 *spanContext
			expected := &spanContext{spanID: "sid", traceID: "tid", baggage: map[string]string{"a": "b"}}
			ctx = context.WithValue(ctx, spanContextKey{}, expected)
			sc := getSpanContextImpl(ctx)
			convey.So(sc, convey.ShouldNotBeNil)
			convey.So(sc, convey.ShouldEqual, expected)
			convey.So(sc.GetSpanID(), convey.ShouldEqual, "sid")
			convey.So(sc.GetTraceID(), convey.ShouldEqual, "tid")
			convey.So(sc.GetBaggage()["a"], convey.ShouldEqual, "b")
		})
	})
}

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
	"errors"
	"os"
	"testing"

	"github.com/bytedance/mockey"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/coze-dev/cozeloop-go"
	"github.com/coze-dev/cozeloop-go/spec/tracespec"
	"github.com/smartystreets/goconvey/convey"
)

// 定义空实现结构体
//type ClientImpl struct{ Client }
//type CallbackDataParserImpl struct{ CallbackDataParser }
//type SpanImpl struct{ Span }

func Test_einoTracer_OnStart(t *testing.T) {
	os.Setenv(cozeloop.EnvWorkspaceID, "1234567890")
	os.Setenv(cozeloop.EnvApiToken, "xxxx")
	mockey.PatchConvey("测试einoTracer的OnStart方法", t, func() {
		client, err := cozeloop.NewClient()
		if err != nil {
			return
		}
		runtime := &tracespec.Runtime{}
		l := &einoTracer{
			client:  client,
			runtime: runtime,
		}

		ctx := context.Background()
		info := &callbacks.RunInfo{
			Name:      "testName",
			Type:      "testType",
			Component: components.Component("testComponent"),
		}
		var input callbacks.CallbackInput

		mockey.PatchConvey("info不为空，parser不为空的场景", func() {
			result := l.OnStart(ctx, info, input)
			convey.So(result, convey.ShouldNotBeNil)
		})

		mockey.PatchConvey("info为空的场景", func() {
			result := l.OnStart(ctx, nil, input)
			convey.So(result, convey.ShouldEqual, ctx)
		})
	})
}

func Test_einoTracer_OnEnd(t *testing.T) {
	os.Setenv(cozeloop.EnvWorkspaceID, "1234567890")
	os.Setenv(cozeloop.EnvApiToken, "xxxx")
	mockey.PatchConvey("测试einoTracer的OnEnd方法", t, func() {
		client, err := cozeloop.NewClient()
		if err != nil {
			return
		}
		runtime := &tracespec.Runtime{}
		l := &einoTracer{
			client:  client,
			runtime: runtime,
		}

		ctx := context.Background()
		info := &callbacks.RunInfo{
			Name:      "testName",
			Type:      "testType",
			Component: components.Component("testComponent"),
		}
		var input callbacks.CallbackInput

		mockey.PatchConvey("info不为空，parser不为空的场景", func() {
			result := l.OnEnd(ctx, info, input)
			convey.So(result, convey.ShouldNotBeNil)
		})

		mockey.PatchConvey("info为空的场景", func() {
			result := l.OnStart(ctx, nil, input)
			convey.So(result, convey.ShouldEqual, ctx)
		})
	})
}

func Test_einoTracer_OnError(t *testing.T) {
	os.Setenv(cozeloop.EnvWorkspaceID, "1234567890")
	os.Setenv(cozeloop.EnvApiToken, "xxxx")
	mockey.PatchConvey("测试einoTracer的OnError方法", t, func() {
		client, err := cozeloop.NewClient()
		if err != nil {
			return
		}
		runtime := &tracespec.Runtime{}
		l := &einoTracer{
			client:  client,
			runtime: runtime,
		}

		ctx := context.Background()
		info := &callbacks.RunInfo{
			Name:      "testName",
			Type:      "testType",
			Component: components.Component("testComponent"),
		}

		mockey.PatchConvey("info不为空，parser不为空的场景", func() {
			result := l.OnError(ctx, info, errors.New("err"))
			convey.So(result, convey.ShouldNotBeNil)
		})

		mockey.PatchConvey("info为空的场景", func() {
			result := l.OnError(ctx, nil, errors.New("err"))
			convey.So(result, convey.ShouldEqual, ctx)
		})
	})
}

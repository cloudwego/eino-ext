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
	"runtime/debug"
	"testing"

	"github.com/bytedance/mockey"
	"github.com/smartystreets/goconvey/convey"
)

func Test_readVersionByGoMod(t *testing.T) {
	mockey.PatchConvey("测试 addToolName 函数", t, func() {
		mockey.PatchConvey("输入的 message 为 nil", func() {
			mock := mockey.Mock(debug.ReadBuildInfo).Return(&debug.BuildInfo{
				GoVersion: "1.18",
				Path:      "github.com/cloudwego/eino",
				Deps: []*debug.Module{
					{
						Path:    "github.com/cloudwego/eino",
						Version: "v0.1.0",
					},
				},
			}, true).Build()
			defer mock.UnPatch()

			res := readBuildVersion()
			convey.So(res, convey.ShouldEqual, "v0.1.0")
		})
	})
}

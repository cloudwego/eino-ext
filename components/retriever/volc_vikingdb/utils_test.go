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

package volc_vikingdb

import (
	"encoding/json"
	"testing"

	. "github.com/bytedance/mockey"
	"github.com/smartystreets/goconvey/convey"
)

func TestInterfaceTof64Slice(t *testing.T) {
	PatchConvey("test interfaceTof64Slice", t, func() {
		PatchConvey("test invalid raw", func() {
			r, err := interfaceTof64Slice("asd")
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(r, convey.ShouldBeNil)
		})

		PatchConvey("test float64 item", func() {
			r, err := interfaceTof64Slice([]any{1.1})
			convey.So(err, convey.ShouldBeNil)
			convey.So(r, convey.ShouldEqual, []float64{1.1})
		})

		PatchConvey("test json number parse failed", func() {
			r, err := interfaceTof64Slice([]any{json.Number("asd")})
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(r, convey.ShouldBeNil)
		})

		PatchConvey("test json number parse success", func() {
			r, err := interfaceTof64Slice([]any{json.Number("1.1")})
			convey.So(err, convey.ShouldBeNil)
			convey.So(r, convey.ShouldEqual, []float64{1.1})
		})

		PatchConvey("test type invalid", func() {
			r, err := interfaceTof64Slice([]any{"asd"})
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(r, convey.ShouldBeNil)
		})
	})
}

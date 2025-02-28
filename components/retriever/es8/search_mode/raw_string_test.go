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

package search_mode

import (
	"context"
	"testing"

	. "github.com/bytedance/mockey"
	"github.com/cloudwego/eino-ext/components/retriever/es8"
	"github.com/smartystreets/goconvey/convey"
)

func TestSearchModeRawStringRequest(t *testing.T) {
	PatchConvey("test SearchModeRawStringRequest", t, func() {
		ctx := context.Background()
		conf := &es8.RetrieverConfig{}
		searchMode := SearchModeRawStringRequest()

		PatchConvey("test from json error", func() {
			r, err := searchMode.BuildRequest(ctx, conf, "test_query")
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(r, convey.ShouldBeNil)
		})

		PatchConvey("test success", func() {
			q := `{"query":{"match":{"test_field":{"query":"test_query"}}}}`
			r, err := searchMode.BuildRequest(ctx, conf, q)
			convey.So(err, convey.ShouldBeNil)
			convey.So(r, convey.ShouldNotBeNil)
			convey.So(r.Query.Match["test_field"].Query, convey.ShouldEqual, "test_query")
		})
	})
}

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
	"encoding/json"
	"testing"

	. "github.com/bytedance/mockey"
	"github.com/cloudwego/eino-ext/components/retriever/es8"
	"github.com/smartystreets/goconvey/convey"
)

func TestSearchModeExactMatch(t *testing.T) {
	PatchConvey("test SearchModeExactMatch", t, func() {
		ctx := context.Background()
		conf := &es8.RetrieverConfig{}
		searchMode := SearchModeExactMatch("test_field")
		req, err := searchMode.BuildRequest(ctx, conf, "test_query")
		convey.So(err, convey.ShouldBeNil)
		b, err := json.Marshal(req)
		convey.So(err, convey.ShouldBeNil)
		convey.So(string(b), convey.ShouldEqual, `{"query":{"match":{"test_field":{"query":"test_query"}}}}`)
	})

}

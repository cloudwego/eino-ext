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
	"fmt"
	"testing"

	. "github.com/bytedance/mockey"
	"github.com/cloudwego/eino-ext/components/retriever/es8"
	"github.com/smartystreets/goconvey/convey"
)

func TestSearchModeSparseVectorQuery(t *testing.T) {
	PatchConvey("test SearchModeSparseVectorQuery", t, func() {
		ctx := context.Background()

		PatchConvey("test with inference id", func() {
			mode := SearchModeSparseVectorQuery(&SparseVectorQueryConfig{
				Field:       "test_field",
				Boost:       ptrWithoutZero(float32(1.2)),
				InferenceID: ptrWithoutZero("test_inference_id"),
			})

			r, err := mode.BuildRequest(ctx, &es8.RetrieverConfig{}, "test_query")
			convey.So(err, convey.ShouldBeNil)
			b, err := json.Marshal(r)
			convey.So(err, convey.ShouldBeNil)
			convey.So(string(b), convey.ShouldEqual,
				`{"query":{"bool":{"should":[{"sparse_vector":{"boost":1.2,"field":"test_field","inference_id":"test_inference_id","query":"test_query"}}]}}}`)
		})

		PatchConvey("test with sparse vector", func() {
			mode := SearchModeSparseVectorQuery(&SparseVectorQueryConfig{
				Field: "test_field",
				Boost: ptrWithoutZero(float32(1.2)),
			})

			r, err := mode.BuildRequest(ctx, &es8.RetrieverConfig{}, "test_query",
				es8.WithSparseVector(map[string]float32{
					"tk1": 1.23,
				}))
			convey.So(err, convey.ShouldBeNil)
			b, err := json.Marshal(r)
			convey.So(err, convey.ShouldBeNil)
			convey.So(string(b), convey.ShouldEqual,
				`{"query":{"bool":{"should":[{"sparse_vector":{"boost":1.2,"field":"test_field","query_vector":{"tk1":1.23}}}]}}}`)
		})

		PatchConvey("test neither provided", func() {
			mode := SearchModeSparseVectorQuery(&SparseVectorQueryConfig{
				Field: "test_field",
				Boost: ptrWithoutZero(float32(1.2)),
			})

			r, err := mode.BuildRequest(ctx, &es8.RetrieverConfig{}, "test_query")
			convey.So(err, convey.ShouldBeError, fmt.Errorf("[sparseVectorQuery] neither inference id or query sparse vector is provided"))
			convey.So(r, convey.ShouldBeNil)
		})

	})
}

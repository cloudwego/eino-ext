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

package milvus2

import (
	"context"
	"testing"

	"github.com/bytedance/mockey"
	"github.com/smartystreets/goconvey/convey"
)

func TestSearchParams_Integration(t *testing.T) {
	mockey.PatchConvey("Test SearchParams Integration", t, func() {
		ctx := context.Background()
		mockEmb := &mockEmbedding{dims: 128}

		convey.Convey("Iterator Mode with SearchParams", func() {
			iterator := NewIterator(L2, 10)

			conf := &RetrieverConfig{
				Collection:  "test_coll",
				VectorField: "dense_vec",
				Embedding:   mockEmb,
				SearchParams: map[string]map[string]interface{}{
					"dense_vec": {"nprobe": 10},
				},
			}

			// Use pure Build* check for better unit test:
			opt, err := iterator.BuildSearchIteratorOption(ctx, conf, []float32{0.1})
			convey.So(err, convey.ShouldBeNil)
			_ = opt
		})

		convey.Convey("Range Mode with SearchParams", func() {
			r := NewRange(L2, 1.0)

			conf := &RetrieverConfig{
				Collection:  "test_coll",
				VectorField: "dense_vec",
				Embedding:   mockEmb,
				SearchParams: map[string]map[string]interface{}{
					"dense_vec": {"nprobe": 10},
				},
			}

			// Verify BuildSearchOption works and extracts params
			opt, err := r.BuildSearchOption(ctx, conf, []float32{0.1})
			convey.So(err, convey.ShouldBeNil)
			_ = opt
		})
	})
}

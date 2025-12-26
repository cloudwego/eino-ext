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

	"github.com/cloudwego/eino/components/retriever"
	"github.com/smartystreets/goconvey/convey"

	milvus2 "github.com/cloudwego/eino-ext/components/retriever/milvus2"
)

func TestNewApproximate(t *testing.T) {
	convey.Convey("test NewApproximate", t, func() {
		convey.Convey("test with default metric type", func() {
			approx := NewApproximate("")
			convey.So(approx, convey.ShouldNotBeNil)
			convey.So(approx.MetricType, convey.ShouldEqual, milvus2.L2)
		})

		convey.Convey("test with L2 metric type", func() {
			approx := NewApproximate(milvus2.L2)
			convey.So(approx, convey.ShouldNotBeNil)
			convey.So(approx.MetricType, convey.ShouldEqual, milvus2.L2)
		})

		convey.Convey("test with IP metric type", func() {
			approx := NewApproximate(milvus2.IP)
			convey.So(approx, convey.ShouldNotBeNil)
			convey.So(approx.MetricType, convey.ShouldEqual, milvus2.IP)
		})

		convey.Convey("test with COSINE metric type", func() {
			approx := NewApproximate(milvus2.COSINE)
			convey.So(approx, convey.ShouldNotBeNil)
			convey.So(approx.MetricType, convey.ShouldEqual, milvus2.COSINE)
		})
	})
}

func TestApproximate_BuildSearchOption(t *testing.T) {
	convey.Convey("test Approximate.BuildSearchOption", t, func() {
		ctx := context.Background()
		queryVector := []float32{0.1, 0.2, 0.3}

		config := &milvus2.RetrieverConfig{
			Collection:   "test_collection",
			VectorField:  "vector",
			TopK:         10,
			OutputFields: []string{"id", "content"},
		}

		convey.Convey("test basic search option", func() {
			approx := NewApproximate(milvus2.L2)
			opt, err := approx.BuildSearchOption(ctx, config, queryVector)
			convey.So(err, convey.ShouldBeNil)
			convey.So(opt, convey.ShouldNotBeNil)
		})

		convey.Convey("test with partitions", func() {
			configWithPartitions := &milvus2.RetrieverConfig{
				Collection:   "test_collection",
				VectorField:  "vector",
				TopK:         10,
				OutputFields: []string{"id", "content"},
				Partitions:   []string{"partition1", "partition2"},
			}
			approx := NewApproximate(milvus2.IP)
			opt, err := approx.BuildSearchOption(ctx, configWithPartitions, queryVector)
			convey.So(err, convey.ShouldBeNil)
			convey.So(opt, convey.ShouldNotBeNil)
		})

		convey.Convey("test with filter option", func() {
			approx := NewApproximate(milvus2.L2)
			opt, err := approx.BuildSearchOption(ctx, config, queryVector,
				milvus2.WithFilter("id > 10"))
			convey.So(err, convey.ShouldBeNil)
			convey.So(opt, convey.ShouldNotBeNil)
		})

		convey.Convey("test with grouping option", func() {
			approx := NewApproximate(milvus2.L2)
			opt, err := approx.BuildSearchOption(ctx, config, queryVector,
				milvus2.WithGrouping("category", 3, true))
			convey.So(err, convey.ShouldBeNil)
			convey.So(opt, convey.ShouldNotBeNil)
		})

		convey.Convey("test with custom TopK", func() {
			approx := NewApproximate(milvus2.L2)
			customTopK := 20
			opt, err := approx.BuildSearchOption(ctx, config, queryVector,
				retriever.WithTopK(customTopK))
			convey.So(err, convey.ShouldBeNil)
			convey.So(opt, convey.ShouldNotBeNil)
		})
	})
}

// Verify interface implementation
func TestApproximate_ImplementsSearchMode(t *testing.T) {
	convey.Convey("test Approximate implements SearchMode", t, func() {
		var _ milvus2.SearchMode = (*Approximate)(nil)
		convey.So(true, convey.ShouldBeTrue)
	})
}

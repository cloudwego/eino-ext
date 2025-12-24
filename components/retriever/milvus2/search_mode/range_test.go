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

func TestNewRange(t *testing.T) {
	convey.Convey("test NewRange", t, func() {
		convey.Convey("test with default metric type", func() {
			r := NewRange("", 0.5)
			convey.So(r, convey.ShouldNotBeNil)
			convey.So(r.MetricType, convey.ShouldEqual, milvus2.L2)
			convey.So(r.Radius, convey.ShouldEqual, 0.5)
		})

		convey.Convey("test with L2 metric type", func() {
			r := NewRange(milvus2.L2, 1.0)
			convey.So(r, convey.ShouldNotBeNil)
			convey.So(r.MetricType, convey.ShouldEqual, milvus2.L2)
			convey.So(r.Radius, convey.ShouldEqual, 1.0)
		})

		convey.Convey("test with IP metric type", func() {
			r := NewRange(milvus2.IP, 0.8)
			convey.So(r, convey.ShouldNotBeNil)
			convey.So(r.MetricType, convey.ShouldEqual, milvus2.IP)
			convey.So(r.Radius, convey.ShouldEqual, 0.8)
		})
	})
}

func TestRange_WithRangeFilter(t *testing.T) {
	convey.Convey("test Range.WithRangeFilter", t, func() {
		r := NewRange(milvus2.L2, 1.0)
		result := r.WithRangeFilter(0.5)
		convey.So(result, convey.ShouldEqual, r)
		convey.So(r.RangeFilter, convey.ShouldNotBeNil)
		convey.So(*r.RangeFilter, convey.ShouldEqual, 0.5)
	})
}

func TestRange_BuildSearchOption(t *testing.T) {
	convey.Convey("test Range.BuildSearchOption", t, func() {
		ctx := context.Background()
		queryVector := []float32{0.1, 0.2, 0.3}

		config := &milvus2.RetrieverConfig{
			Collection:   "test_collection",
			VectorField:  "vector",
			TopK:         10,
			OutputFields: []string{"id", "content"},
		}

		convey.Convey("test basic range search option", func() {
			r := NewRange(milvus2.L2, 1.0)
			opt, err := r.BuildSearchOption(ctx, config, queryVector)
			convey.So(err, convey.ShouldBeNil)
			convey.So(opt, convey.ShouldNotBeNil)
		})

		convey.Convey("test with range filter (ring search)", func() {
			r := NewRange(milvus2.L2, 1.0).WithRangeFilter(0.5)
			opt, err := r.BuildSearchOption(ctx, config, queryVector)
			convey.So(err, convey.ShouldBeNil)
			convey.So(opt, convey.ShouldNotBeNil)
		})

		convey.Convey("test with partitions", func() {
			configWithPartitions := &milvus2.RetrieverConfig{
				Collection:   "test_collection",
				VectorField:  "vector",
				TopK:         10,
				OutputFields: []string{"id", "content"},
				Partitions:   []string{"partition1"},
			}
			r := NewRange(milvus2.L2, 1.0)
			opt, err := r.BuildSearchOption(ctx, configWithPartitions, queryVector)
			convey.So(err, convey.ShouldBeNil)
			convey.So(opt, convey.ShouldNotBeNil)
		})

		convey.Convey("test with filter option", func() {
			r := NewRange(milvus2.L2, 1.0)
			opt, err := r.BuildSearchOption(ctx, config, queryVector,
				milvus2.WithFilter("id > 10"))
			convey.So(err, convey.ShouldBeNil)
			convey.So(opt, convey.ShouldNotBeNil)
		})

		convey.Convey("test with grouping option", func() {
			r := NewRange(milvus2.L2, 1.0)
			opt, err := r.BuildSearchOption(ctx, config, queryVector,
				milvus2.WithGrouping("category", 3, true))
			convey.So(err, convey.ShouldBeNil)
			convey.So(opt, convey.ShouldNotBeNil)
		})

		convey.Convey("test with custom TopK", func() {
			r := NewRange(milvus2.L2, 1.0)
			customTopK := 20
			opt, err := r.BuildSearchOption(ctx, config, queryVector,
				retriever.WithTopK(customTopK))
			convey.So(err, convey.ShouldBeNil)
			convey.So(opt, convey.ShouldNotBeNil)
		})

		convey.Convey("test with IP metric type for similarity search", func() {
			r := NewRange(milvus2.IP, 0.8)
			opt, err := r.BuildSearchOption(ctx, config, queryVector)
			convey.So(err, convey.ShouldBeNil)
			convey.So(opt, convey.ShouldNotBeNil)
		})
	})
}

// Verify interface implementation
func TestRange_ImplementsSearchMode(t *testing.T) {
	convey.Convey("test Range implements SearchMode", t, func() {
		var _ milvus2.SearchMode = (*Range)(nil)
		convey.So(true, convey.ShouldBeTrue)
	})
}

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
	"github.com/milvus-io/milvus/client/v2/milvusclient"
	"github.com/smartystreets/goconvey/convey"

	milvus2 "github.com/cloudwego/eino-ext/components/retriever/milvus2"
)

func TestNewHybrid(t *testing.T) {
	convey.Convey("test NewHybrid", t, func() {
		convey.Convey("test with reranker and no sub-requests", func() {
			reranker := milvusclient.NewRRFReranker()
			hybrid := NewHybrid(reranker)
			convey.So(hybrid, convey.ShouldNotBeNil)
			convey.So(hybrid.Reranker, convey.ShouldNotBeNil)
			convey.So(len(hybrid.SubRequests), convey.ShouldEqual, 0)
		})

		convey.Convey("test with reranker and sub-requests", func() {
			reranker := milvusclient.NewRRFReranker()
			subReq1 := &SubRequest{
				VectorField: "vector1",
				MetricType:  milvus2.L2,
				TopK:        10,
			}
			subReq2 := &SubRequest{
				VectorField: "vector2",
				MetricType:  milvus2.IP,
				TopK:        5,
			}
			hybrid := NewHybrid(reranker, subReq1, subReq2)
			convey.So(hybrid, convey.ShouldNotBeNil)
			convey.So(len(hybrid.SubRequests), convey.ShouldEqual, 2)
		})
	})
}

func TestHybrid_BuildSearchOption(t *testing.T) {
	convey.Convey("test Hybrid.BuildSearchOption returns error", t, func() {
		ctx := context.Background()
		queryVector := []float32{0.1, 0.2, 0.3}
		config := &milvus2.RetrieverConfig{
			Collection:  "test_collection",
			VectorField: "vector",
			TopK:        10,
		}

		reranker := milvusclient.NewRRFReranker()
		hybrid := NewHybrid(reranker)

		opt, err := hybrid.BuildSearchOption(ctx, config, queryVector)
		convey.So(opt, convey.ShouldBeNil)
		convey.So(err, convey.ShouldNotBeNil)
		convey.So(err.Error(), convey.ShouldContainSubstring, "BuildHybridSearchOption")
	})
}

func TestHybrid_BuildHybridSearchOption(t *testing.T) {
	convey.Convey("test Hybrid.BuildHybridSearchOption", t, func() {
		ctx := context.Background()
		queryVector := []float32{0.1, 0.2, 0.3}

		config := &milvus2.RetrieverConfig{
			Collection:   "test_collection",
			VectorField:  "vector",
			TopK:         10,
			OutputFields: []string{"id", "content"},
		}

		convey.Convey("test with single sub-request", func() {
			reranker := milvusclient.NewRRFReranker()
			subReq := &SubRequest{
				VectorField: "vector",
				MetricType:  milvus2.L2,
				TopK:        10,
			}
			hybrid := NewHybrid(reranker, subReq)

			opt, err := hybrid.BuildHybridSearchOption(ctx, config, queryVector, nil)
			convey.So(err, convey.ShouldBeNil)
			convey.So(opt, convey.ShouldNotBeNil)
		})

		convey.Convey("test with multiple sub-requests", func() {
			reranker := milvusclient.NewRRFReranker()
			subReq1 := &SubRequest{
				VectorField: "vector1",
				MetricType:  milvus2.L2,
				TopK:        10,
			}
			subReq2 := &SubRequest{
				VectorField: "vector2",
				MetricType:  milvus2.IP,
				TopK:        5,
			}
			hybrid := NewHybrid(reranker, subReq1, subReq2)

			opt, err := hybrid.BuildHybridSearchOption(ctx, config, queryVector, nil)
			convey.So(err, convey.ShouldBeNil)
			convey.So(opt, convey.ShouldNotBeNil)
		})

		convey.Convey("test with sub-request using default vector field", func() {
			reranker := milvusclient.NewRRFReranker()
			subReq := &SubRequest{
				VectorField: "", // Should use config.VectorField
				MetricType:  milvus2.L2,
				TopK:        0, // Should default to 10
			}
			hybrid := NewHybrid(reranker, subReq)

			opt, err := hybrid.BuildHybridSearchOption(ctx, config, queryVector, nil)
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
			reranker := milvusclient.NewRRFReranker()
			subReq := &SubRequest{
				VectorField: "vector",
				MetricType:  milvus2.L2,
				TopK:        10,
			}
			hybrid := NewHybrid(reranker, subReq)

			opt, err := hybrid.BuildHybridSearchOption(ctx, configWithPartitions, queryVector, nil)
			convey.So(err, convey.ShouldBeNil)
			convey.So(opt, convey.ShouldNotBeNil)
		})

		convey.Convey("test with filter option", func() {
			reranker := milvusclient.NewRRFReranker()
			subReq := &SubRequest{
				VectorField: "vector",
				MetricType:  milvus2.L2,
				TopK:        10,
			}
			hybrid := NewHybrid(reranker, subReq)

			opt, err := hybrid.BuildHybridSearchOption(ctx, config, queryVector, nil,
				milvus2.WithFilter("id > 10"))
			convey.So(err, convey.ShouldBeNil)
			convey.So(opt, convey.ShouldNotBeNil)
		})

		convey.Convey("test with custom TopK override", func() {
			reranker := milvusclient.NewRRFReranker()
			subReq := &SubRequest{
				VectorField: "vector",
				MetricType:  milvus2.L2,
				TopK:        10,
			}
			hybrid := NewHybrid(reranker, subReq)
			hybrid.TopK = 50

			opt, err := hybrid.BuildHybridSearchOption(ctx, config, queryVector, nil)
			convey.So(err, convey.ShouldBeNil)
			convey.So(opt, convey.ShouldNotBeNil)
		})

		convey.Convey("test with search params", func() {
			reranker := milvusclient.NewRRFReranker()
			subReq := &SubRequest{
				VectorField:  "vector",
				MetricType:   milvus2.L2,
				TopK:         10,
				SearchParams: map[string]string{"nprobe": "16", "ef": "64"},
			}
			hybrid := NewHybrid(reranker, subReq)

			opt, err := hybrid.BuildHybridSearchOption(ctx, config, queryVector, nil)
			convey.So(err, convey.ShouldBeNil)
			convey.So(opt, convey.ShouldNotBeNil)
		})

		convey.Convey("test with grouping", func() {
			reranker := milvusclient.NewRRFReranker()
			subReq := &SubRequest{
				VectorField: "vector",
				MetricType:  milvus2.L2,
				TopK:        10,
			}
			hybrid := NewHybrid(reranker, subReq)

			opt, err := hybrid.BuildHybridSearchOption(ctx, config, queryVector, nil,
				milvus2.WithGrouping("category", 3, true))
			convey.So(err, convey.ShouldBeNil)
			convey.So(opt, convey.ShouldNotBeNil)
		})

		convey.Convey("test with common TopK option", func() {
			reranker := milvusclient.NewRRFReranker()
			subReq := &SubRequest{
				VectorField: "vector",
				MetricType:  milvus2.L2,
				TopK:        10,
			}
			hybrid := NewHybrid(reranker, subReq)

			opt, err := hybrid.BuildHybridSearchOption(ctx, config, queryVector, nil,
				retriever.WithTopK(100))
			convey.So(err, convey.ShouldBeNil)
			convey.So(opt, convey.ShouldNotBeNil)
		})

		convey.Convey("test with sparse sub-request", func() {
			reranker := milvusclient.NewRRFReranker()
			subReq := &SubRequest{
				VectorField: "sparse_vector",
				VectorType:  milvus2.SparseVector,
				TopK:        10,
			}
			hybrid := NewHybrid(reranker, subReq)

			querySparseVector := map[int]float64{1: 0.5, 2: 0.8}
			opt, err := hybrid.BuildHybridSearchOption(ctx, config, nil, querySparseVector)
			convey.So(err, convey.ShouldBeNil)
			convey.So(opt, convey.ShouldNotBeNil)
		})
	})
}

// Verify interface implementation
func TestHybrid_ImplementsHybridSearchMode(t *testing.T) {
	convey.Convey("test Hybrid implements HybridSearchMode", t, func() {
		var _ milvus2.HybridSearchMode = (*Hybrid)(nil)
		convey.So(true, convey.ShouldBeTrue)
	})
}

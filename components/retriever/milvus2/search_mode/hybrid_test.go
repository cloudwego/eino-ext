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
	"fmt"
	"testing"

	. "github.com/bytedance/mockey"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus/client/v2/entity"
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
			hybrid := NewHybrid(reranker, subReq, &SubRequest{
				VectorField: "vector2",
				MetricType:  milvus2.IP,
				TopK:        5,
			})

			opt, err := hybrid.BuildHybridSearchOption(ctx, config, queryVector, "")
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

			opt, err := hybrid.BuildHybridSearchOption(ctx, config, queryVector, "")
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
			hybrid := NewHybrid(reranker, subReq, &SubRequest{
				VectorField: "vector2",
				MetricType:  milvus2.IP,
				TopK:        5,
			})

			opt, err := hybrid.BuildHybridSearchOption(ctx, config, queryVector, "")
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
			hybrid := NewHybrid(reranker, subReq, &SubRequest{
				VectorField: "vector2",
				MetricType:  milvus2.IP,
				TopK:        5,
			})

			opt, err := hybrid.BuildHybridSearchOption(ctx, configWithPartitions, queryVector, "")
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
			hybrid := NewHybrid(reranker, subReq, &SubRequest{
				VectorField: "vector2",
				MetricType:  milvus2.IP,
				TopK:        5,
			})

			opt, err := hybrid.BuildHybridSearchOption(ctx, config, queryVector, "",
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
			hybrid := NewHybrid(reranker, subReq, &SubRequest{
				VectorField: "vector2",
				MetricType:  milvus2.IP,
				TopK:        5,
			})
			hybrid.TopK = 50

			opt, err := hybrid.BuildHybridSearchOption(ctx, config, queryVector, "")
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
			hybrid := NewHybrid(reranker, subReq, &SubRequest{
				VectorField: "vector2",
				MetricType:  milvus2.IP,
				TopK:        5,
			})

			opt, err := hybrid.BuildHybridSearchOption(ctx, config, queryVector, "")
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
			hybrid := NewHybrid(reranker, subReq, &SubRequest{
				VectorField: "vector2",
				MetricType:  milvus2.IP,
				TopK:        5,
			})

			opt, err := hybrid.BuildHybridSearchOption(ctx, config, queryVector, "",
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
			hybrid := NewHybrid(reranker, subReq, &SubRequest{
				VectorField: "vector2",
				MetricType:  milvus2.IP,
				TopK:        5,
			})

			opt, err := hybrid.BuildHybridSearchOption(ctx, config, queryVector, "",
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
			hybrid := NewHybrid(reranker, subReq, &SubRequest{
				VectorField: "vector2_sparse",
				VectorType:  milvus2.SparseVector,
				TopK:        5,
			})

			query := "test query"
			opt, err := hybrid.BuildHybridSearchOption(ctx, config, nil, query)
			convey.So(err, convey.ShouldBeNil)
			convey.So(opt, convey.ShouldNotBeNil)
		})
	})
}

// Verify interface implementation
func TestHybrid_ImplementsSearchMode(t *testing.T) {
	convey.Convey("test Hybrid implements SearchMode", t, func() {
		var _ milvus2.SearchMode = (*Hybrid)(nil)
	})
}

func TestHybridSearchTopK(t *testing.T) {
	PatchConvey("Test Hybrid Search TopK Default", t, func() {
		ctx := context.Background()
		mockClient := &milvusclient.Client{}

		// Mock Reranker
		mockReranker := milvusclient.NewRRFReranker()

		// Hybrid mode with no explicit TopK in SubRequest
		hybridMode := NewHybrid(
			mockReranker,
			&SubRequest{
				VectorField: "vector",
				MetricType:  milvus2.L2,
				// TopK is unset (0)
			},
			&SubRequest{
				VectorField: "vector_sparse",
				MetricType:  milvus2.IP,
				// TopK is unset (0)
			},
		)

		// We just need a config object to pass to BuildHybridSearchOption
		// No need for a full Retriever instance since we are testing the SearchMode method directly
		config := &milvus2.RetrieverConfig{
			Collection:  "test_collection",
			VectorField: "vector",
			TopK:        50, // Global TopK > 10
			SearchMode:  hybridMode,
		}

		convey.Convey("should use global TopK when SubRequest TopK is missing", func() {
			Mock(GetMethod(mockClient, "HybridSearch")).Return([]milvusclient.ResultSet{}, nil).Build()

			queryVector := make([]float32, 128)
			opt, err := hybridMode.BuildHybridSearchOption(ctx, config, queryVector, "query")
			convey.So(err, convey.ShouldBeNil)
			convey.So(opt, convey.ShouldNotBeNil)

			var capturedLimits []int
			Mock(milvusclient.NewAnnRequest).To(func(fieldName string, limit int, vectors ...entity.Vector) *milvusclient.AnnRequest {
				capturedLimits = append(capturedLimits, limit)
				return &milvusclient.AnnRequest{}
			}).Build()

			Mock((*milvusclient.AnnRequest).WithSearchParam).To(func(r *milvusclient.AnnRequest, key string, value string) *milvusclient.AnnRequest {
				return r
			}).Build()

			_, _ = hybridMode.BuildHybridSearchOption(ctx, config, queryVector, "query")

			convey.So(capturedLimits, convey.ShouldContain, 50)
		})

		convey.Convey("should use explicit TopK when SubRequest TopK is set", func() {
			hybridMode.SubRequests[0].TopK = 5

			var capturedLimits []int
			Mock(milvusclient.NewAnnRequest).To(func(fieldName string, limit int, vectors ...entity.Vector) *milvusclient.AnnRequest {
				capturedLimits = append(capturedLimits, limit)
				return &milvusclient.AnnRequest{}
			}).Build()

			Mock((*milvusclient.AnnRequest).WithSearchParam).To(func(r *milvusclient.AnnRequest, key string, value string) *milvusclient.AnnRequest {
				return r
			}).Build()

			queryVector := make([]float32, 128)
			_, _ = hybridMode.BuildHybridSearchOption(ctx, config, queryVector, "query")

			convey.So(capturedLimits, convey.ShouldContain, 5)
		})
	})
}

func TestHybrid_Retrieve(t *testing.T) {
	PatchConvey("test Hybrid.Retrieve", t, func() {
		ctx := context.Background()
		mockClient := &milvusclient.Client{}
		mockEmb := &mockHybridEmbedding{}

		config := &milvus2.RetrieverConfig{
			Collection:   "test_collection",
			VectorField:  "vector",
			TopK:         10,
			OutputFields: []string{"id", "content"},
			Embedding:    mockEmb,
		}

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

		PatchConvey("success", func() {
			// Mock Client.HybridSearch
			Mock(GetMethod(mockClient, "HybridSearch")).Return([]milvusclient.ResultSet{
				{
					ResultCount: 1,
					IDs:         nil,
				},
			}, nil).Build()

			mockConverter := func(ctx context.Context, result milvusclient.ResultSet) ([]*schema.Document, error) {
				return []*schema.Document{{ID: "1"}}, nil
			}
			config.DocumentConverter = mockConverter

			docs, err := hybrid.Retrieve(ctx, mockClient, config, "query")
			convey.So(err, convey.ShouldBeNil)
			convey.So(len(docs), convey.ShouldEqual, 1)
		})

		PatchConvey("embedding error", func() {
			mockEmb.err = fmt.Errorf("embed error")
			docs, err := hybrid.Retrieve(ctx, mockClient, config, "query")
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(docs, convey.ShouldBeNil)
		})

		PatchConvey("search error", func() {
			mockEmb.err = nil
			Mock(GetMethod(mockClient, "HybridSearch")).Return(nil, fmt.Errorf("search error")).Build()

			docs, err := hybrid.Retrieve(ctx, mockClient, config, "query")
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(docs, convey.ShouldBeNil)
		})
	})
}

// mockHybridEmbedding implements embedding.Embedder for testing
type mockHybridEmbedding struct {
	err error
}

func (m *mockHybridEmbedding) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	if m.err != nil {
		return nil, m.err
	}
	return [][]float64{make([]float64, 128)}, nil
}

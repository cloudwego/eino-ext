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
	"fmt"
	"testing"

	. "github.com/bytedance/mockey"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus/client/v2/column"
	"github.com/milvus-io/milvus/client/v2/entity"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
	"github.com/smartystreets/goconvey/convey"
)

// mockEmbedding implements embedding.Embedder for testing
type mockEmbedding struct {
	err  error
	dims int
}

func (m *mockEmbedding) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	if m.err != nil {
		return nil, m.err
	}
	result := make([][]float64, len(texts))
	dims := m.dims
	if dims == 0 {
		dims = 128
	}
	for i := range texts {
		result[i] = make([]float64, dims)
		for j := 0; j < dims; j++ {
			result[i][j] = 0.1
		}
	}
	return result, nil
}

func TestIndexerConfig_validate(t *testing.T) {
	PatchConvey("test IndexerConfig.validate", t, func() {
		mockEmb := &mockEmbedding{}

		PatchConvey("test missing client and client config", func() {
			config := &IndexerConfig{
				Client:       nil,
				ClientConfig: nil,
				Collection:   "test_collection",
				Embedding:    mockEmb,
			}
			err := config.validate()
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "client")
		})

		PatchConvey("test optional embedding", func() {
			config := &IndexerConfig{
				ClientConfig: &milvusclient.ClientConfig{Address: "localhost:19530"},
				Collection:   "test_collection",
				Embedding:    nil,
				Vector: &VectorConfig{
					Dimension: 128,
				},
			}
			err := config.validate()
			convey.So(err, convey.ShouldBeNil)
		})

		PatchConvey("test valid config sets defaults", func() {
			config := &IndexerConfig{
				ClientConfig: &milvusclient.ClientConfig{Address: "localhost:19530"},
				Collection:   "",
				Embedding:    mockEmb,
				Vector: &VectorConfig{
					Dimension: 128,
				},
			}
			err := config.validate()
			convey.So(err, convey.ShouldBeNil)
			// Check defaults are set
			convey.So(config.Collection, convey.ShouldEqual, defaultCollection)
			convey.So(config.Description, convey.ShouldEqual, defaultDescription)
			convey.So(config.Vector.MetricType, convey.ShouldEqual, L2)
			convey.So(config.Vector.VectorField, convey.ShouldEqual, defaultVectorField)
			convey.So(config.DocumentConverter, convey.ShouldNotBeNil)
		})

		PatchConvey("test valid config sets defaults (sparse)", func() {
			config := &IndexerConfig{
				ClientConfig: &milvusclient.ClientConfig{Address: "localhost:19530"},
				Collection:   "default_sparse",
				Embedding:    mockEmb,
				Vector: &VectorConfig{
					Dimension: 256,
				},
				Sparse: &SparseVectorConfig{}, // Empty config
			}
			err := config.validate()
			convey.So(err, convey.ShouldBeNil)
			convey.So(config.Sparse.VectorField, convey.ShouldEqual, defaultSparseVectorField)
			convey.So(config.Sparse.MetricType, convey.ShouldEqual, BM25)
		})

		PatchConvey("test valid config preserves custom sparse vector field", func() {
			config := &IndexerConfig{
				ClientConfig: &milvusclient.ClientConfig{Address: "localhost:19530"},
				Collection:   "my_collection",
				Embedding:    mockEmb,
				Vector: &VectorConfig{
					Dimension: 256,
				},
				Sparse: &SparseVectorConfig{
					VectorField: "my_sparse_vector",
				},
			}
			err := config.validate()
			convey.So(err, convey.ShouldBeNil)
			convey.So(config.Sparse.VectorField, convey.ShouldEqual, "my_sparse_vector")
			convey.So(config.Sparse.MetricType, convey.ShouldEqual, BM25) // Default to BM25
			convey.So(len(config.Functions), convey.ShouldEqual, 1)       // Auto-generated function
		})

		PatchConvey("test explicit IP metric type", func() {
			config := &IndexerConfig{
				ClientConfig: &milvusclient.ClientConfig{Address: "localhost:19530"},
				Collection:   "my_collection",
				Embedding:    mockEmb,
				Vector: &VectorConfig{
					Dimension: 256,
				},
				Sparse: &SparseVectorConfig{
					VectorField: "my_sparse_vector",
					MetricType:  IP,
				},
			}
			err := config.validate()
			convey.So(err, convey.ShouldBeNil)
			convey.So(config.Sparse.MetricType, convey.ShouldEqual, IP)
			convey.So(len(config.Functions), convey.ShouldEqual, 0) // No auto-generated function
		})

		PatchConvey("test valid config preserves custom values", func() {
			config := &IndexerConfig{
				ClientConfig:  &milvusclient.ClientConfig{Address: "localhost:19530"},
				Collection:    "my_collection",
				Description:   "my description",
				Embedding:     mockEmb,
				PartitionName: "my_partition",
				Vector: &VectorConfig{
					Dimension:    256,
					MetricType:   IP,
					VectorField:  "my_vector",
					IndexBuilder: NewHNSWIndexBuilder(),
				},
			}
			err := config.validate()
			convey.So(err, convey.ShouldBeNil)
			convey.So(config.Collection, convey.ShouldEqual, "my_collection")
			convey.So(config.Description, convey.ShouldEqual, "my description")
			convey.So(config.Vector.VectorField, convey.ShouldEqual, "my_vector")
			convey.So(config.Vector.MetricType, convey.ShouldEqual, IP)
		})

		PatchConvey("test sparse-only config", func() {
			config := &IndexerConfig{
				ClientConfig: &milvusclient.ClientConfig{Address: "localhost:19530"},
				Collection:   "sparse_only",
				Embedding:    mockEmb,
				// Vector is nil implies no dense vector
				Sparse: &SparseVectorConfig{
					VectorField: "s_vec",
				},
			}
			err := config.validate()
			convey.So(err, convey.ShouldBeNil)
			convey.So(config.Vector, convey.ShouldBeNil)
			convey.So(config.Sparse.VectorField, convey.ShouldEqual, "s_vec")
		})

		PatchConvey("test auto-generate BM25 function (Default)", func() {
			mockEmb := &mockEmbedding{}
			config := &IndexerConfig{
				ClientConfig: &milvusclient.ClientConfig{Address: "localhost:19530"},
				Collection:   "auto_bm25",
				Embedding:    mockEmb,
				Vector: &VectorConfig{
					Dimension: 128,
				},
				Sparse: &SparseVectorConfig{
					VectorField: "sparse",
					MetricType:  BM25,
					// Method defaults to BM25 (Auto)
				},
			}

			err := config.validate()
			convey.So(err, convey.ShouldBeNil)
			convey.So(config.Sparse.Method, convey.ShouldEqual, SparseMethodAuto)
			convey.So(len(config.Functions), convey.ShouldEqual, 1)
			fn := config.Functions[0]
			convey.So(fn.Name, convey.ShouldEqual, "bm25_auto")
			convey.So(fn.Type, convey.ShouldEqual, entity.FunctionTypeBM25)
			convey.So(fn.OutputFieldNames[0], convey.ShouldEqual, "sparse")
		})

		PatchConvey("test explicit SparseMethodPrecomputed (No Auto-Gen)", func() {
			mockEmb := &mockEmbedding{}
			config := &IndexerConfig{
				ClientConfig: &milvusclient.ClientConfig{Address: "localhost:19530"},
				Collection:   "client_sparse",
				Embedding:    mockEmb,
				Vector: &VectorConfig{
					Dimension: 128,
				},
				Sparse: &SparseVectorConfig{
					VectorField: "sparse",
					MetricType:  BM25,
					Method:      SparseMethodPrecomputed,
				},
			}

			err := config.validate()
			convey.So(err, convey.ShouldBeNil)
			convey.So(config.Sparse.Method, convey.ShouldEqual, SparseMethodPrecomputed)
			// Should NOT generate function
			convey.So(len(config.Functions), convey.ShouldEqual, 0)
		})

		PatchConvey("test custom BM25 function (Suppresses Auto-Gen)", func() {
			mockEmb := &mockEmbedding{}
			customFn := entity.NewFunction().
				WithName("bm25_custom").
				WithType(entity.FunctionTypeBM25).
				WithInputFields(defaultContentField).
				WithOutputFields("sparse") // Targets the sparse field

			config := &IndexerConfig{
				ClientConfig: &milvusclient.ClientConfig{Address: "localhost:19530"},
				Collection:   "custom_bm25",
				Embedding:    mockEmb,
				Vector: &VectorConfig{
					Dimension: 128,
				},
				Sparse: &SparseVectorConfig{
					VectorField: "sparse",
					Method:      SparseMethodAuto,
				},
				Functions: []*entity.Function{customFn},
			}

			err := config.validate()
			convey.So(err, convey.ShouldBeNil)
			// Should NOT generate auto function, so count remains 1
			convey.So(len(config.Functions), convey.ShouldEqual, 1)
			convey.So(config.Functions[0].Name, convey.ShouldEqual, "bm25_custom")
		})

		PatchConvey("test unrelated function (Allows Auto-Gen)", func() {
			mockEmb := &mockEmbedding{}
			otherFn := entity.NewFunction().
				WithName("other_fn").
				WithType(entity.FunctionTypeUnknown).
				WithInputFields("other_col").
				WithOutputFields("other_out")

			config := &IndexerConfig{
				ClientConfig: &milvusclient.ClientConfig{Address: "localhost:19530"},
				Collection:   "mixed_functions",
				Embedding:    mockEmb,
				Vector: &VectorConfig{
					Dimension: 128,
				},
				Sparse: &SparseVectorConfig{
					VectorField: "sparse",
					Method:      SparseMethodAuto,
				},
				Functions: []*entity.Function{otherFn},
			}

			err := config.validate()
			convey.So(err, convey.ShouldBeNil)
			// Should generate auto function, so count becomes 2
			convey.So(len(config.Functions), convey.ShouldEqual, 2)
			// First is custom, second is auto
			convey.So(config.Functions[0].Name, convey.ShouldEqual, "other_fn")
			convey.So(config.Functions[1].Name, convey.ShouldEqual, "bm25_auto")
		})
	})
}

func TestNewIndexer(t *testing.T) {
	PatchConvey("test NewIndexer", t, func() {
		ctx := context.Background()
		mockEmb := &mockEmbedding{dims: 128}
		mockClient := &milvusclient.Client{}

		// Mock milvusclient.New
		Mock(milvusclient.New).Return(mockClient, nil).Build()

		PatchConvey("test missing client and client config", func() {
			_, err := NewIndexer(ctx, &IndexerConfig{
				Client:       nil,
				ClientConfig: nil,
				Collection:   "test_collection",
				Embedding:    mockEmb,
				Vector: &VectorConfig{
					Dimension: 128,
				},
			})
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "client")
		})

		PatchConvey("test with collection already exists and loaded", func() {
			Mock(GetMethod(mockClient, "HasCollection")).Return(true, nil).Build()
			Mock(GetMethod(mockClient, "GetLoadState")).Return(entity.LoadState{State: entity.LoadStateLoaded}, nil).Build()

			indexer, err := NewIndexer(ctx, &IndexerConfig{
				ClientConfig: &milvusclient.ClientConfig{
					Address: "localhost:19530",
				},
				Collection: "test_collection",
				Embedding:  mockEmb,
				Vector: &VectorConfig{
					Dimension: 128,
				},
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(indexer, convey.ShouldNotBeNil)
		})

		PatchConvey("test with collection not loaded, needs index and load", func() {
			Mock(GetMethod(mockClient, "HasCollection")).Return(true, nil).Build()
			Mock(GetMethod(mockClient, "GetLoadState")).Return(entity.LoadState{State: entity.LoadStateNotLoad}, nil).Build()
			Mock(GetMethod(mockClient, "ListIndexes")).Return([]string{}, nil).Build()
			Mock(GetMethod(mockClient, "DescribeIndex")).Return(milvusclient.IndexDescription{}, fmt.Errorf("index not found")).Build()

			mockTask := &milvusclient.CreateIndexTask{}
			Mock(GetMethod(mockClient, "CreateIndex")).Return(mockTask, nil).Build()
			Mock(GetMethod(mockTask, "Await")).Return(nil).Build()

			mockLoadTask := milvusclient.LoadTask{}
			Mock(GetMethod(mockClient, "LoadCollection")).Return(mockLoadTask, nil).Build()
			Mock(GetMethod(&mockLoadTask, "Await")).Return(nil).Build()

			indexer, err := NewIndexer(ctx, &IndexerConfig{
				ClientConfig: &milvusclient.ClientConfig{
					Address: "localhost:19530",
				},
				Collection: "test_collection",
				Embedding:  mockEmb,
				Vector: &VectorConfig{
					Dimension: 128,
				},
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(indexer, convey.ShouldNotBeNil)
		})

		PatchConvey("test collection does not exist, needs creation", func() {
			Mock(GetMethod(mockClient, "HasCollection")).Return(false, nil).Build()
			Mock(GetMethod(mockClient, "CreateCollection")).Return(nil).Build()
			Mock(GetMethod(mockClient, "GetLoadState")).Return(entity.LoadState{State: entity.LoadStateNotLoad}, nil).Build()
			Mock(GetMethod(mockClient, "ListIndexes")).Return([]string{}, nil).Build()
			Mock(GetMethod(mockClient, "DescribeIndex")).Return(milvusclient.IndexDescription{}, fmt.Errorf("index not found")).Build()

			mockTask := &milvusclient.CreateIndexTask{}
			Mock(GetMethod(mockClient, "CreateIndex")).Return(mockTask, nil).Build()
			Mock(GetMethod(mockTask, "Await")).Return(nil).Build()

			mockLoadTask := milvusclient.LoadTask{}
			Mock(GetMethod(mockClient, "LoadCollection")).Return(mockLoadTask, nil).Build()
			Mock(GetMethod(&mockLoadTask, "Await")).Return(nil).Build()

			indexer, err := NewIndexer(ctx, &IndexerConfig{
				ClientConfig: &milvusclient.ClientConfig{
					Address: "localhost:19530",
				},
				Collection: "test_collection",
				Embedding:  mockEmb,
				Vector: &VectorConfig{
					Dimension: 128,
				},
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(indexer, convey.ShouldNotBeNil)
		})

		PatchConvey("test collection does not exist but dimension not provided", func() {
			Mock(GetMethod(mockClient, "HasCollection")).Return(false, nil).Build()

			_, err := NewIndexer(ctx, &IndexerConfig{
				ClientConfig: &milvusclient.ClientConfig{
					Address: "localhost:19530",
				},
				Collection: "test_collection",
				Embedding:  mockEmb,
				Vector: &VectorConfig{
					Dimension: 0, // No dimension
				},
			})
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "dimension")
		})
	})
}

func TestIndexer_GetType(t *testing.T) {
	convey.Convey("test Indexer.GetType", t, func() {
		indexer := &Indexer{}
		convey.So(indexer.GetType(), convey.ShouldNotBeEmpty)
	})
}

func TestIndexer_IsCallbacksEnabled(t *testing.T) {
	convey.Convey("test Indexer.IsCallbacksEnabled", t, func() {
		indexer := &Indexer{
			config: &IndexerConfig{},
		}
		convey.So(indexer.IsCallbacksEnabled(), convey.ShouldBeTrue)
	})
}

func TestIndexer_Store(t *testing.T) {
	PatchConvey("test Indexer.Store", t, func() {
		ctx := context.Background()
		mockEmb := &mockEmbedding{dims: 128}
		mockClient := &milvusclient.Client{}

		indexer := &Indexer{
			client: mockClient,
			config: &IndexerConfig{
				Collection: "test_collection",
				Vector: &VectorConfig{
					Dimension: 128,
				},
				Embedding: mockEmb,
				DocumentConverter: defaultDocumentConverter(&VectorConfig{
					VectorField: defaultVectorField,
				}, nil),
			},
		}

		docs := []*schema.Document{
			{
				ID:       "doc1",
				Content:  "Test document 1",
				MetaData: map[string]interface{}{"key": "value"},
			},
			{
				ID:       "doc2",
				Content:  "Test document 2",
				MetaData: map[string]interface{}{"key2": "value2"},
			},
		}

		PatchConvey("test store with embedding error", func() {
			indexer.config.Embedding = &mockEmbedding{err: fmt.Errorf("embedding error")}
			ids, err := indexer.Store(ctx, docs)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "embed")
			convey.So(ids, convey.ShouldBeNil)
		})

		PatchConvey("test store with upsert error", func() {
			indexer.config.Embedding = mockEmb
			Mock(GetMethod(mockClient, "Upsert")).Return(nil, fmt.Errorf("upsert error")).Build()

			ids, err := indexer.Store(ctx, docs)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(ids, convey.ShouldBeNil)
		})

		PatchConvey("test store success", func() {
			indexer.config.Embedding = mockEmb

			// Create mock ID column
			mockIDColumn := column.NewColumnVarChar("id", []string{"doc1", "doc2"})
			mockResult := milvusclient.UpsertResult{
				IDs: mockIDColumn,
			}
			Mock(GetMethod(mockClient, "Upsert")).Return(mockResult, nil).Build()

			ids, err := indexer.Store(ctx, docs)
			convey.So(err, convey.ShouldBeNil)
			convey.So(ids, convey.ShouldNotBeNil)
			convey.So(len(ids), convey.ShouldEqual, 2)
		})
	})
}

func TestDefaultDocumentConverter(t *testing.T) {
	convey.Convey("test defaultDocumentConverter", t, func() {
		convey.Convey("test conversion (dense only)", func() {
			converter := defaultDocumentConverter(&VectorConfig{
				VectorField: defaultVectorField,
			}, nil)

			ctx := context.Background()
			docs := []*schema.Document{
				{
					ID:       "doc1",
					Content:  "content1",
					MetaData: map[string]interface{}{"key": "value"},
				},
			}
			vectors := [][]float64{{0.1, 0.2, 0.3}}

			columns, err := converter(ctx, docs, vectors)
			convey.So(err, convey.ShouldBeNil)
			convey.So(len(columns), convey.ShouldEqual, 4) // id, content, vector, metadata
			convey.So(columns[3].Name(), convey.ShouldEqual, defaultVectorField)
			convey.So(columns[2].Name(), convey.ShouldEqual, defaultMetadataField)
		})

		convey.Convey("test conversion with sparse vector", func() {
			converter := defaultDocumentConverter(&VectorConfig{
				VectorField: defaultVectorField,
			}, &SparseVectorConfig{
				VectorField: "sparse_vector",
				Method:      SparseMethodPrecomputed,
			})

			ctx := context.Background()
			docs := []*schema.Document{
				{
					ID:       "doc1",
					Content:  "content1",
					MetaData: map[string]interface{}{"key": "value"},
				},
			}
			docs[0].WithSparseVector(map[int]float64{1: 0.5})
			vectors := [][]float64{{0.1, 0.2, 0.3}}

			columns, err := converter(ctx, docs, vectors)
			convey.So(err, convey.ShouldBeNil)
			// Now returns 5 columns: id, content, metadata, dense_vector, sparse_vector
			convey.So(len(columns), convey.ShouldEqual, 5)
			convey.So(columns[4].Name(), convey.ShouldEqual, "sparse_vector")
		})

	})
}

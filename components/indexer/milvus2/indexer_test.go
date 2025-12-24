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
			}
			err := config.validate()
			convey.So(err, convey.ShouldBeNil)
		})

		PatchConvey("test valid config sets defaults", func() {
			config := &IndexerConfig{
				ClientConfig: &milvusclient.ClientConfig{Address: "localhost:19530"},
				Collection:   "",
				Embedding:    mockEmb,
				Dimension:    128,
			}
			err := config.validate()
			convey.So(err, convey.ShouldBeNil)
			// Check defaults are set
			convey.So(config.Collection, convey.ShouldEqual, defaultCollection)
			convey.So(config.Description, convey.ShouldEqual, defaultDescription)
			convey.So(config.MetricType, convey.ShouldEqual, L2)
			convey.So(config.VectorField, convey.ShouldEqual, defaultVectorField)
			convey.So(config.DocumentConverter, convey.ShouldNotBeNil)
		})

		PatchConvey("test valid config preserves custom sparse vector field", func() {
			config := &IndexerConfig{
				ClientConfig:      &milvusclient.ClientConfig{Address: "localhost:19530"},
				Collection:        "my_collection",
				Embedding:         mockEmb,
				Dimension:         256,
				SparseVectorField: "my_sparse_vector",
			}
			err := config.validate()
			convey.So(err, convey.ShouldBeNil)
			convey.So(config.SparseVectorField, convey.ShouldEqual, "my_sparse_vector")
		})

		PatchConvey("test valid config preserves custom values", func() {
			config := &IndexerConfig{
				ClientConfig:  &milvusclient.ClientConfig{Address: "localhost:19530"},
				Collection:    "my_collection",
				Description:   "my description",
				MetricType:    IP,
				Embedding:     mockEmb,
				Dimension:     256,
				PartitionName: "my_partition",
				VectorField:   "my_vector",
				IndexBuilder:  NewHNSWIndexBuilder(),
			}
			err := config.validate()
			convey.So(err, convey.ShouldBeNil)
			convey.So(config.Collection, convey.ShouldEqual, "my_collection")
			convey.So(config.Description, convey.ShouldEqual, "my description")
			convey.So(config.VectorField, convey.ShouldEqual, "my_vector")
			convey.So(config.MetricType, convey.ShouldEqual, IP)
		})

		PatchConvey("test sparse-only config", func() {
			config := &IndexerConfig{
				ClientConfig:      &milvusclient.ClientConfig{Address: "localhost:19530"},
				Collection:        "sparse_only",
				Embedding:         mockEmb,
				Dimension:         0, // No dense dimension
				SparseVectorField: "s_vec",
			}
			err := config.validate()
			convey.So(err, convey.ShouldBeNil)
			convey.So(config.VectorField, convey.ShouldBeEmpty) // Should NOT be defaulted
			convey.So(config.SparseVectorField, convey.ShouldEqual, "s_vec")
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
				Dimension:    128,
				Embedding:    mockEmb,
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
				Dimension:  128,
				Embedding:  mockEmb,
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(indexer, convey.ShouldNotBeNil)
		})

		PatchConvey("test with collection not loaded, needs index and load", func() {
			Mock(GetMethod(mockClient, "HasCollection")).Return(true, nil).Build()
			Mock(GetMethod(mockClient, "GetLoadState")).Return(entity.LoadState{State: entity.LoadStateNotLoad}, nil).Build()
			Mock(GetMethod(mockClient, "ListIndexes")).Return([]string{}, nil).Build()

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
				Dimension:  128,
				Embedding:  mockEmb,
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(indexer, convey.ShouldNotBeNil)
		})

		PatchConvey("test collection does not exist, needs creation", func() {
			Mock(GetMethod(mockClient, "HasCollection")).Return(false, nil).Build()
			Mock(GetMethod(mockClient, "CreateCollection")).Return(nil).Build()
			Mock(GetMethod(mockClient, "GetLoadState")).Return(entity.LoadState{State: entity.LoadStateNotLoad}, nil).Build()
			Mock(GetMethod(mockClient, "ListIndexes")).Return([]string{}, nil).Build()

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
				Dimension:  128,
				Embedding:  mockEmb,
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
				Dimension:  0, // No dimension
				Embedding:  mockEmb,
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
				Collection:        "test_collection",
				Dimension:         128,
				Embedding:         mockEmb,
				DocumentConverter: defaultDocumentConverter(defaultVectorField, ""),
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

		PatchConvey("test store with insert error", func() {
			indexer.config.Embedding = mockEmb
			Mock(GetMethod(mockClient, "Insert")).Return(nil, fmt.Errorf("insert error")).Build()

			ids, err := indexer.Store(ctx, docs)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(ids, convey.ShouldBeNil)
		})

		PatchConvey("test store success", func() {
			indexer.config.Embedding = mockEmb

			// Create mock ID column
			mockIDColumn := column.NewColumnVarChar("id", []string{"doc1", "doc2"})
			mockResult := milvusclient.InsertResult{
				IDs: mockIDColumn,
			}
			Mock(GetMethod(mockClient, "Insert")).Return(mockResult, nil).Build()

			// Mock flush
			mockFlushTask := &milvusclient.FlushTask{}
			Mock(GetMethod(mockClient, "Flush")).Return(mockFlushTask, nil).Build()
			Mock(GetMethod(mockFlushTask, "Await")).Return(nil).Build()

			ids, err := indexer.Store(ctx, docs)
			convey.So(err, convey.ShouldBeNil)
			convey.So(ids, convey.ShouldNotBeNil)
			convey.So(len(ids), convey.ShouldEqual, 2)
		})
	})
}

func TestIsIndexNotFoundError(t *testing.T) {
	convey.Convey("test isIndexNotFoundError", t, func() {
		convey.Convey("test with index not found error", func() {
			err := fmt.Errorf("index not found")
			result := isIndexNotFoundError(err)
			convey.So(result, convey.ShouldBeTrue)
		})

		convey.Convey("test with index doesn't exist error", func() {
			err := fmt.Errorf("index doesn't exist")
			result := isIndexNotFoundError(err)
			convey.So(result, convey.ShouldBeTrue)
		})

		convey.Convey("test with other error", func() {
			err := fmt.Errorf("some other error")
			result := isIndexNotFoundError(err)
			convey.So(result, convey.ShouldBeFalse)
		})

		convey.Convey("test with nil error", func() {
			result := isIndexNotFoundError(nil)
			convey.So(result, convey.ShouldBeFalse)
		})
	})
}

func TestDefaultDocumentConverter(t *testing.T) {
	convey.Convey("test defaultDocumentConverter", t, func() {
		convey.Convey("test conversion (dense only)", func() {
			converter := defaultDocumentConverter(defaultVectorField, "")

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
			converter := defaultDocumentConverter(defaultVectorField, "sparse_vector")

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
			convey.So(len(columns), convey.ShouldEqual, 5) // id, content, metadata, vector, sparse_vector
			convey.So(columns[4].Name(), convey.ShouldEqual, "sparse_vector")
			convey.So(columns[3].Name(), convey.ShouldEqual, defaultVectorField)
		})
	})
}

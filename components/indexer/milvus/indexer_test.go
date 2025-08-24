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

package milvus

import (
	"context"
	"fmt"
	"testing"

	. "github.com/bytedance/mockey"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus/client/v2/column"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
	"github.com/smartystreets/goconvey/convey"
)

// mockEmbedding mocks the embedding component
type mockEmbedding struct {
	err         error
	cnt         int
	sizeForCall []int
	dims        int
}

func (m *mockEmbedding) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	if m.err != nil {
		return nil, m.err
	}

	if m.cnt >= len(m.sizeForCall) {
		return nil, fmt.Errorf("unexpected call count")
	}

	slice := make([]float64, m.dims)
	for i := range slice {
		slice[i] = 1.1
	}

	r := make([][]float64, m.sizeForCall[m.cnt])
	m.cnt++
	for i := range r {
		r[i] = slice
	}

	return r, nil
}

func TestNewIndexer(t *testing.T) {
	PatchConvey("test NewIndexer", t, func() {
		ctx := context.Background()
		mockClient := &milvusclient.Client{}

		PatchConvey("test indexer config validation", func() {
			PatchConvey("test client not provided", func() {
				idx, err := NewIndexer(ctx, &IndexerConfig{
					Client:            nil,
					Collection:        "test_collection",
					Partition:         "test_partition",
					Dim:               128,
					DocumentConverter: defaultDocumentConverter,
					Embedding:         &mockEmbedding{},
				})
				convey.So(err, convey.ShouldNotBeNil)
				convey.So(err.Error(), convey.ShouldContainSubstring, "client is nil")
				convey.So(idx, convey.ShouldBeNil)
			})

			PatchConvey("test collection not provided", func() {
				Mock((*milvusclient.Client).HasCollection).Return(true, nil).Build()
				idx, err := NewIndexer(ctx, &IndexerConfig{
					Client:            mockClient,
					Collection:        "",
					Partition:         "test_partition",
					Dim:               128,
					DocumentConverter: defaultDocumentConverter,
					Embedding:         &mockEmbedding{},
				})
				// Should succeed because it will use the default collection name
				convey.So(err, convey.ShouldBeNil)
				convey.So(idx, convey.ShouldNotBeNil)
			})

			PatchConvey("test dim not provided", func() {
				idx, err := NewIndexer(ctx, &IndexerConfig{
					Client:            mockClient,
					Collection:        "test_collection",
					Partition:         "test_partition",
					Dim:               0,
					DocumentConverter: defaultDocumentConverter,
					Embedding:         &mockEmbedding{},
				})
				convey.So(err, convey.ShouldNotBeNil)
				convey.So(err.Error(), convey.ShouldContainSubstring, "dimension of the vector must be greater than 0")
				convey.So(idx, convey.ShouldBeNil)
			})

			PatchConvey("test document converter not provided", func() {
				Mock((*milvusclient.Client).HasCollection).Return(true, nil).Build()
				idx, err := NewIndexer(ctx, &IndexerConfig{
					Client:            mockClient,
					Collection:        "test_collection",
					Partition:         "test_partition",
					Dim:               128,
					DocumentConverter: nil,
					Embedding:         &mockEmbedding{},
				})
				// Should succeed because it will use the default converter
				convey.So(err, convey.ShouldBeNil)
				convey.So(idx, convey.ShouldNotBeNil)
			})
		})

		PatchConvey("test collection operations", func() {
			PatchConvey("test has collection error", func() {
				Mock((*milvusclient.Client).HasCollection).Return(false, fmt.Errorf("has collection error")).Build()
				idx, err := NewIndexer(ctx, &IndexerConfig{
					Client:            mockClient,
					Collection:        "test_collection",
					Partition:         "test_partition",
					Dim:               128,
					DocumentConverter: defaultDocumentConverter,
					Embedding:         &mockEmbedding{},
				})
				convey.So(err, convey.ShouldNotBeNil)
				convey.So(err.Error(), convey.ShouldContainSubstring, "failed to validate collection")
				convey.So(idx, convey.ShouldBeNil)
			})

			PatchConvey("test successful creation", func() {
				Mock((*milvusclient.Client).HasCollection).Return(true, nil).Build()
				idx, err := NewIndexer(ctx, &IndexerConfig{
					Client:            mockClient,
					Collection:        "test_collection",
					Partition:         "test_partition",
					Dim:               128,
					DocumentConverter: defaultDocumentConverter,
					Embedding:         &mockEmbedding{},
				})
				convey.So(err, convey.ShouldBeNil)
				convey.So(idx, convey.ShouldNotBeNil)
				convey.So(idx.GetType(), convey.ShouldEqual, typ)
			})
		})
	})
}

func TestIndexer_Store(t *testing.T) {
	PatchConvey("test Indexer.Store", t, func() {
		ctx := context.Background()
		mockClient := &milvusclient.Client{}

		// Create test documents
		testDocs := []*schema.Document{
			{
				ID:      "doc1",
				Content: "test content 1",
				MetaData: map[string]interface{}{
					"key1": "value1",
				},
			},
			{
				ID:      "doc2",
				Content: "test content 2",
				MetaData: map[string]interface{}{
					"key2": "value2",
				},
			},
		}

		PatchConvey("test embedding not set", func() {
			idx := &Indexer{
				conf: &IndexerConfig{
					Client:            mockClient,
					Collection:        "test_collection",
					Partition:         "test_partition",
					Dim:               128,
					DocumentConverter: defaultDocumentConverter,
					Embedding:         nil,
				},
			}

			ids, err := idx.Store(ctx, testDocs)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "embedding is not set")
			convey.So(ids, convey.ShouldBeNil)
		})

		PatchConvey("test embedding error", func() {
			idx := &Indexer{
				conf: &IndexerConfig{
					Client:            mockClient,
					Collection:        "test_collection",
					Partition:         "test_partition",
					Dim:               128,
					DocumentConverter: defaultDocumentConverter,
					Embedding:         &mockEmbedding{err: fmt.Errorf("embedding error")},
				},
			}

			ids, err := idx.Store(ctx, testDocs)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "failed to embed documents")
			convey.So(ids, convey.ShouldBeNil)
		})

		PatchConvey("test embedding vector count mismatch", func() {
			idx := &Indexer{
				conf: &IndexerConfig{
					Client:            mockClient,
					Collection:        "test_collection",
					Partition:         "test_partition",
					Dim:               128,
					DocumentConverter: defaultDocumentConverter,
					Embedding:         &mockEmbedding{sizeForCall: []int{1}, dims: 128}, // Returns 1 vector but has 2 documents
				},
			}

			ids, err := idx.Store(ctx, testDocs)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "converter returned 1 rows, expected 2")
			convey.So(ids, convey.ShouldBeNil)
		})

		PatchConvey("test document converter error", func() {
			idx := &Indexer{
				conf: &IndexerConfig{
					Client:            mockClient,
					Collection:        "test_collection",
					Partition:         "test_partition",
					Dim:               128,
					DocumentConverter: func(docs []*schema.Document, dim int, vectors [][]float64) ([]column.Column, error) {
						return nil, fmt.Errorf("converter error")
					},
					Embedding: &mockEmbedding{sizeForCall: []int{2}, dims: 128},
				},
			}

			ids, err := idx.Store(ctx, testDocs)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "failed to convert documents")
			convey.So(ids, convey.ShouldBeNil)
		})

		PatchConvey("test insert error", func() {
			Mock((*milvusclient.Client).Insert).Return(milvusclient.InsertResult{}, fmt.Errorf("insert error")).Build()
			idx := &Indexer{
				conf: &IndexerConfig{
					Client:            mockClient,
					Collection:        "test_collection",
					Partition:         "test_partition",
					Dim:               128,
					DocumentConverter: defaultDocumentConverter,
					Embedding:         &mockEmbedding{sizeForCall: []int{2}, dims: 128},
				},
			}

			ids, err := idx.Store(ctx, testDocs)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "failed to insert documents")
			convey.So(ids, convey.ShouldBeNil)
		})

		PatchConvey("test upsert error", func() {
			Mock((*milvusclient.Client).Upsert).Return(milvusclient.UpsertResult{}, fmt.Errorf("upsert error")).Build()
			idx := &Indexer{
				conf: &IndexerConfig{
					Client:            mockClient,
					Collection:        "test_collection",
					Partition:         "test_partition",
					Dim:               128,
					DocumentConverter: defaultDocumentConverter,
					Embedding:         &mockEmbedding{sizeForCall: []int{2}, dims: 128},
				},
			}

			ids, err := idx.Store(ctx, testDocs, WithUpsert())
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "failed to upsert documents")
			convey.So(ids, convey.ShouldBeNil)
		})

		PatchConvey("test successful insert", func() {
			Mock((*milvusclient.Client).Insert).Return(milvusclient.InsertResult{
				IDs: column.NewColumnVarChar("id", []string{"1", "2"}),
			}, nil).Build()
			idx := &Indexer{
				conf: &IndexerConfig{
					Client:            mockClient,
					Collection:        "test_collection",
					Partition:         "test_partition",
					Dim:               128,
					DocumentConverter: defaultDocumentConverter,
					Embedding:         &mockEmbedding{sizeForCall: []int{2}, dims: 128},
				},
			}

			ids, err := idx.Store(ctx, testDocs)
			convey.So(err, convey.ShouldBeNil)
			convey.So(ids, convey.ShouldNotBeNil)
			convey.So(len(ids), convey.ShouldEqual, 2)
			convey.So(ids[0], convey.ShouldEqual, "doc1")
			convey.So(ids[1], convey.ShouldEqual, "doc2")
		})

		PatchConvey("test successful upsert", func() {
			Mock((*milvusclient.Client).Upsert).Return(milvusclient.UpsertResult{
				IDs: column.NewColumnVarChar("id", []string{"1", "2"}),
			}, nil).Build()
			idx := &Indexer{
				conf: &IndexerConfig{
					Client:            mockClient,
					Collection:        "test_collection",
					Partition:         "test_partition",
					Dim:               128,
					DocumentConverter: defaultDocumentConverter,
					Embedding:         &mockEmbedding{sizeForCall: []int{2}, dims: 128},
				},
			}

			ids, err := idx.Store(ctx, testDocs, WithUpsert(), WithPartition("custom_partition"))
			convey.So(err, convey.ShouldBeNil)
			convey.So(ids, convey.ShouldNotBeNil)
			convey.So(len(ids), convey.ShouldEqual, 2)
			convey.So(ids[0], convey.ShouldEqual, "doc1")
			convey.So(ids[1], convey.ShouldEqual, "doc2")
		})
	})
}
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

package pinecone

import (
	"context"
	"fmt"
	"github.com/cloudwego/eino/schema"
	"google.golang.org/grpc"
	"testing"

	. "github.com/bytedance/mockey"
	"github.com/cloudwego/eino/components/embedding"
	pc "github.com/pinecone-io/go-pinecone/v3/pinecone"
	"github.com/smartystreets/goconvey/convey"
)

type mockEmbedding struct{}

func (m *mockEmbedding) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	result := make([][]float64, len(texts))
	for i := range texts {
		result[i] = []float64{0.1, 0.2, 0.3, 0.4}
	}
	return result, nil
}

func TestNewIndexer(t *testing.T) {
	PatchConvey("test NewIndexer", t, func() {
		ctx := context.Background()
		Mock(pc.NewClient).Return(&pc.Client{}, nil).Build()

		mockClient, _ := pc.NewClient(pc.NewClientParams{})
		mockEmb := &mockEmbedding{}
		mockDim := defaultDimension

		PatchConvey("test indexer config check", func() {
			PatchConvey("test client not provided", func() {
				indexer, err := NewIndexer(ctx, &IndexerConfig{
					Client:    nil,
					Embedding: mockEmb,
				})
				convey.So(err, convey.ShouldBeError, fmt.Errorf("[NewIndexer] pinecone client not provided"))
				convey.So(indexer, convey.ShouldBeNil)
			})

			PatchConvey("test embedding not provided", func() {
				indexer, err := NewIndexer(ctx, &IndexerConfig{
					Client:    mockClient,
					Embedding: nil,
				})
				convey.So(err, convey.ShouldBeError, fmt.Errorf("[NewIndexer] embedding not provided"))
				convey.So(indexer, convey.ShouldBeNil)
			})

			PatchConvey("test dimension must be positive", func() {
				indexer, err := NewIndexer(ctx, &IndexerConfig{
					Client:    mockClient,
					Embedding: mockEmb,
					Dimension: -1,
				})
				convey.So(err, convey.ShouldBeError, fmt.Errorf("[NewIndexer] dimension must be positive"))
				convey.So(indexer, convey.ShouldBeNil)
			})
		})

		PatchConvey("test create index pre-check", func() {
			Mock(GetMethod(mockClient, "ListIndexes")).To(func(ctx context.Context) ([]*pc.Index, error) {
				list := make([]*pc.Index, 0)
				list = append(list, &pc.Index{
					Name:               defaultIndexName,
					Metric:             defaultMetric,
					VectorType:         defaultVectorType,
					DeletionProtection: defaultDeletionProtection,
					Dimension:          &mockDim,
				})
				return list, nil
			}).Build()

			PatchConvey("test create index failed", func() {
				Mock(GetMethod(mockClient, "CreateServerlessIndex")).To(func(ctx context.Context, in *pc.CreateServerlessIndexRequest) (*pc.Index, error) {
					return nil, fmt.Errorf("[CreateServerlessIndex] mock failed")
				}).Build()

				indexer, err := NewIndexer(ctx, &IndexerConfig{
					Client:    mockClient,
					Embedding: mockEmb,
					IndexName: "test-create",
				})

				convey.So(err, convey.ShouldBeError, fmt.Errorf("[NewIndexer] failed to create index: [CreateServerlessIndex] mock failed"))
				convey.So(indexer, convey.ShouldBeNil)
			})

			PatchConvey("test validate index failed", func() {
				mockIndex := &pc.Index{
					Name:               "test-validate",
					Metric:             defaultMetric,
					VectorType:         defaultVectorType,
					DeletionProtection: defaultDeletionProtection,
					Dimension:          &mockDim,
					Tags: &pc.IndexTags{
						"mockTag": "mockValue",
					},
				}

				Mock(GetMethod(mockClient, "CreateServerlessIndex")).To(func(ctx context.Context, in *pc.CreateServerlessIndexRequest) (*pc.Index, error) {
					return mockIndex, nil
				}).Build()

				Mock(GetMethod(mockClient, "DescribeIndex")).To(func(ctx context.Context, idxName string) (*pc.Index, error) {
					return mockIndex, nil
				}).Build()

				PatchConvey("test describe index failed", func() {
					Mock(GetMethod(mockClient, "DescribeIndex")).To(func(ctx context.Context, idxName string) (*pc.Index, error) {
						return nil, fmt.Errorf("[DescribeIndex] mock failed")
					}).Build()

					indexer, err := NewIndexer(ctx, &IndexerConfig{
						Client:    mockClient,
						Embedding: mockEmb,
						IndexName: "test-describe",
					})

					convey.So(err, convey.ShouldBeError, fmt.Errorf("[NewIndexer] failed to describe index: [DescribeIndex] mock failed"))
					convey.So(indexer, convey.ShouldBeNil)
				})

				PatchConvey("test validate index dimension", func() {
					otherDim := defaultDimension + 1
					indexer, err := NewIndexer(ctx, &IndexerConfig{
						Client:    mockClient,
						Embedding: mockEmb,
						IndexName: "test-validate",
						Dimension: otherDim,
					})

					expectedErr := fmt.Errorf("index dimension mismatch: expected %d, got %d", otherDim, mockDim)
					convey.So(err, convey.ShouldBeError, fmt.Errorf("[NewIndexer] index schema validation failed: %w", expectedErr))
					convey.So(indexer, convey.ShouldBeNil)
				})

				PatchConvey("test validate index metric", func() {
					otherMetric := pc.Euclidean
					indexer, err := NewIndexer(ctx, &IndexerConfig{
						Client:    mockClient,
						Embedding: mockEmb,
						IndexName: "test-validate",
						Metric:    otherMetric,
					})

					expectedErr := fmt.Errorf("index metric mismatch: expected %s, got %s", otherMetric, defaultMetric)
					convey.So(err, convey.ShouldBeError, fmt.Errorf("[NewIndexer] index schema validation failed: %w", expectedErr))
					convey.So(indexer, convey.ShouldBeNil)
				})

				PatchConvey("test validate index deletion protection", func() {
					otherDeletionProtection := pc.DeletionProtectionEnabled
					indexer, err := NewIndexer(ctx, &IndexerConfig{
						Client:             mockClient,
						Embedding:          mockEmb,
						IndexName:          "test-validate",
						DeletionProtection: otherDeletionProtection,
					})

					expectedErr := fmt.Errorf("index deletion protection mismatch: expected %s, got %s", otherDeletionProtection, defaultDeletionProtection)
					convey.So(err, convey.ShouldBeError, fmt.Errorf("[NewIndexer] index schema validation failed: %w", expectedErr))
					convey.So(indexer, convey.ShouldBeNil)
				})

				PatchConvey("test validate index tags", func() {
					otherTags := &pc.IndexTags{
						"mockTag": "failedValue",
					}
					indexer, err := NewIndexer(ctx, &IndexerConfig{
						Client:    mockClient,
						Embedding: mockEmb,
						IndexName: "test-validate",
						Tags:      otherTags,
					})

					expectedErr := fmt.Errorf("index tag mismatch for key mockTag: expected failedValue, got mockValue")
					convey.So(err, convey.ShouldBeError, fmt.Errorf("[NewIndexer] index schema validation failed: %w", expectedErr))
					convey.So(indexer, convey.ShouldBeNil)
				})

			})
		})
	})
}

func TestIndexerStore(t *testing.T) {
	PatchConvey("test Indexer.Store", t, func() {
		ctx := context.Background()
		Mock(pc.NewClient).Return(&pc.Client{}, nil).Build()

		mockClient, _ := pc.NewClient(pc.NewClientParams{})
		mockDim := int32(4)
		mockName := "test-store"
		mockIndex := &pc.Index{
			Name:               mockName,
			Metric:             defaultMetric,
			VectorType:         defaultVectorType,
			DeletionProtection: defaultDeletionProtection,
			Dimension:          &mockDim,
			Tags: &pc.IndexTags{
				"mockTag": "mockValue",
			},
		}
		mockEmb := &mockEmbedding{}
		mockDocs := []*schema.Document{
			{
				ID:       "doc1",
				Content:  "This is a test document",
				MetaData: map[string]interface{}{"key": "value"},
			},
			{
				ID:       "doc2",
				Content:  "This is another test document",
				MetaData: map[string]interface{}{"key2": "value2"},
			},
		}

		Mock(GetMethod(mockClient, "ListIndexes")).To(func(ctx context.Context) ([]*pc.Index, error) {
			list := make([]*pc.Index, 0)
			list = append(list, &pc.Index{
				Name:               defaultIndexName,
				Metric:             defaultMetric,
				VectorType:         defaultVectorType,
				DeletionProtection: defaultDeletionProtection,
				Dimension:          &mockDim,
			})
			return list, nil
		}).Build()
		Mock(GetMethod(mockClient, "CreateServerlessIndex")).To(func(ctx context.Context, in *pc.CreateServerlessIndexRequest) (*pc.Index, error) {
			return mockIndex, nil
		}).Build()

		Mock(GetMethod(mockClient, "DescribeIndex")).To(func(ctx context.Context, idxName string) (*pc.Index, error) {
			if idxName == mockName {
				return mockIndex, nil
			} else {
				return nil, fmt.Errorf("mockDescribeIndex not found index: %s", idxName)
			}
		}).Build()

		PatchConvey("test store index embedding is nil", func() {
			indexer, err := NewIndexer(ctx, &IndexerConfig{
				Client:    mockClient,
				IndexName: mockName,
				Embedding: mockEmb,
				Dimension: mockDim,
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(indexer, convey.ShouldNotBeNil)

			indexer.config.Embedding = nil

			ids, err := indexer.Store(ctx, mockDocs)

			convey.So(err, convey.ShouldBeError, fmt.Errorf("[Store] embedding not provided"))
			convey.So(ids, convey.ShouldBeNil)
		})

		PatchConvey("test store with insert error", func() {
			indexer, err := NewIndexer(ctx, &IndexerConfig{
				Client:    mockClient,
				Embedding: mockEmb,
				IndexName: mockName,
				Dimension: mockDim,
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(indexer, convey.ShouldNotBeNil)

			mockIndexConn := &pc.IndexConnection{Namespace: indexer.config.Namespace}

			Mock(GetMethod(mockClient, "Index")).To(func(in pc.NewIndexConnParams, dialOpts ...grpc.DialOption) (*pc.IndexConnection, error) {
				return mockIndexConn, nil
			}).Build()

			Mock(GetMethod(mockIndexConn, "UpsertVectors")).To(func(ctx context.Context, in []*pc.Vector) (uint32, error) {
				return 0, fmt.Errorf("mock upsert error")
			}).Build()

			ids, err := indexer.Store(ctx, mockDocs)

			convey.So(err, convey.ShouldBeError, fmt.Errorf("[Store] failed to insert document: [Parallel] failed to insert documents: batch 0 failed: mock upsert error"))
			convey.So(ids, convey.ShouldBeEmpty)
		})

		PatchConvey("test store index success", func() {
			indexer, err := NewIndexer(ctx, &IndexerConfig{
				Client:    mockClient,
				Embedding: mockEmb,
				IndexName: mockName,
				Dimension: mockDim,
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(indexer, convey.ShouldNotBeNil)

			mockIndexConn := &pc.IndexConnection{Namespace: indexer.config.Namespace}

			Mock(GetMethod(mockClient, "Index")).To(func(in pc.NewIndexConnParams, dialOpts ...grpc.DialOption) (*pc.IndexConnection, error) {
				return mockIndexConn, nil
			}).Build()

			Mock(GetMethod(mockIndexConn, "UpsertVectors")).To(func(ctx context.Context, in []*pc.Vector) (uint32, error) {
				return uint32(len(in)), nil
			}).Build()

			ids, err := indexer.Store(ctx, mockDocs)

			expectedIds := make([]string, 0, len(mockDocs))
			for _, doc := range mockDocs {
				expectedIds = append(expectedIds, doc.ID)
			}
			convey.So(err, convey.ShouldBeNil)
			convey.So(ids, convey.ShouldResemble, expectedIds)
		})
	})
}

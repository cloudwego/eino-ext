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

package milvus_new

import (
	"context"
	"fmt"
	"log"
	"testing"

	. "github.com/bytedance/mockey"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/milvus-io/milvus/client/v2/column"
	"github.com/milvus-io/milvus/client/v2/entity"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
	"github.com/smartystreets/goconvey/convey"
	"google.golang.org/grpc"
)

func TestNewRetriever(t *testing.T) {
	PatchConvey("test NewRetriever", t, func() {
		ctx := context.Background()

		mockClient := &milvusclient.Client{}

		PatchConvey("test retriever config check", func() {
			PatchConvey("test client not provided", func() {
				r, err := NewRetriever(ctx, &RetrieverConfig{
					Client:            nil,
					Collection:        "",
					Partition:         "",
					VectorField:       "",
					OutputFields:      nil,
					DocumentConverter: nil,
					VectorConverter:   nil,
					MetricType:        "",
					TopK:              0,
					ScoreThreshold:    0,
					Embedding:         &mockEmbedding{},
				})
				convey.So(err, convey.ShouldBeError, fmt.Errorf("[NewRetriever] milvus client not provided"))
				convey.So(r, convey.ShouldBeNil)
			})

			PatchConvey("test embedding not provided", func() {
				r, err := NewRetriever(ctx, &RetrieverConfig{
					Client:            mockClient,
					Collection:        "",
					Partition:         "",
					VectorField:       "",
					OutputFields:      nil,
					DocumentConverter: nil,
					VectorConverter:   nil,
					MetricType:        "",
					TopK:              0,
					ScoreThreshold:    0,
					Embedding:         nil,
				})
				convey.So(err, convey.ShouldBeError, fmt.Errorf("[NewRetriever] embedding not provided"))
				convey.So(r, convey.ShouldBeNil)
			})

			PatchConvey("test search params not provided and score threshold is out of range", func() {
				r, err := NewRetriever(ctx, &RetrieverConfig{
					Client:            mockClient,
					Collection:        "",
					Partition:         "",
					VectorField:       "",
					OutputFields:      nil,
					DocumentConverter: nil,
					VectorConverter:   nil,
					MetricType:        "",
					TopK:              0,
					ScoreThreshold:    -1,
					Embedding:         &mockEmbedding{},
				})
				convey.So(err, convey.ShouldBeError, fmt.Errorf("[NewRetriever] invalid search params"))
				convey.So(r, convey.ShouldBeNil)
			})
		})

		PatchConvey("test pre-check", func() {
			Mock((*milvusclient.Client).HasCollection).To(func(client *milvusclient.Client, ctx context.Context, option milvusclient.HasCollectionOption, opts ...grpc.CallOption) (bool, error) {
				req := option.Request()
				if req.GetCollectionName() != defaultCollection {
					return false, nil
				}
				return true, nil
			}).Build()

			PatchConvey("test collection not found", func() {
				r, err := NewRetriever(ctx, &RetrieverConfig{
					Client:            mockClient,
					Collection:        "test_collection",
					Partition:         "",
					VectorField:       "",
					OutputFields:      nil,
					DocumentConverter: nil,
					VectorConverter:   nil,
					MetricType:        "",
					TopK:              0,
					ScoreThreshold:    0,
					Embedding:         &mockEmbedding{},
				})
				convey.So(err, convey.ShouldBeError, fmt.Errorf("[NewRetriever] collection not found"))
				convey.So(r, convey.ShouldBeNil)

				Mock((*milvusclient.Client).DescribeCollection).To(func(client *milvusclient.Client, ctx context.Context, option milvusclient.DescribeCollectionOption, opts ...grpc.CallOption) (*entity.Collection, error) {
					req := option.Request()
					if req.GetCollectionName() != defaultCollection {
						return nil, fmt.Errorf("collection not found")
					}
					return &entity.Collection{
						Schema: &entity.Schema{
							Fields: []*entity.Field{
								{
									Name:     defaultVectorField,
									DataType: entity.FieldTypeBinaryVector,
									TypeParams: map[string]string{
										"dim": "128",
									},
								},
							},
						},
						// Loaded is not set (defaults to false), so it will try to load
					}, nil
				}).Build()

				PatchConvey("test collection schema not match", func() {
					r, err := NewRetriever(ctx, &RetrieverConfig{
						Client:            mockClient,
						Collection:        defaultCollection,
						Partition:         "",
						VectorField:       "test_vector",
						OutputFields:      nil,
						DocumentConverter: nil,
						VectorConverter:   nil,
						MetricType:        "",
						TopK:              0,
						ScoreThreshold:    0,
						Embedding:         &mockEmbedding{},
					})
					convey.So(err, convey.ShouldBeError, fmt.Errorf("[NewRetriever] collection schema not match: vector field not found"))
					convey.So(r, convey.ShouldBeNil)

					PatchConvey("test collection schema match", func() {
						// Mock GetLoadState to return NotLoad state (will cause error)
						Mock((*milvusclient.Client).GetLoadState).Return(entity.LoadState{State: entity.LoadStateNotLoad}, nil).Build()

						r, err := NewRetriever(ctx, &RetrieverConfig{
							Client:            mockClient,
							Collection:        "",
							Partition:         "",
							VectorField:       defaultVectorField,
							OutputFields:      nil,
							DocumentConverter: nil,
							VectorConverter:   nil,
							MetricType:        "",
							TopK:              0,
							ScoreThreshold:    0,
							Embedding:         &mockEmbedding{},
						})
						convey.So(err, convey.ShouldNotBeNil)
						convey.So(r, convey.ShouldBeNil)

						Mock((*milvusclient.Client).GetLoadState).Return(entity.LoadState{State: entity.LoadStateLoaded}, nil).Build()

						PatchConvey("test create retriever", func() {
							r, err := NewRetriever(ctx, &RetrieverConfig{
								Client:            mockClient,
								Collection:        "",
								Partition:         "",
								VectorField:       defaultVectorField,
								OutputFields:      nil,
								DocumentConverter: nil,
								VectorConverter:   nil,
								MetricType:        "",
								TopK:              0,
								ScoreThreshold:    0,
								Embedding:         &mockEmbedding{},
							})
							convey.So(err, convey.ShouldBeNil)
							convey.So(r, convey.ShouldNotBeNil)
						})
					})
				})
			})
		})
	})
}

const docMetaData = `
{
	"id": "1",
	"content": "test",
	"vector": [1, 2, 3],
	"meta": {
		"key": "value"
	}
}
`

func TestRetriever_Retrieve(t *testing.T) {
	PatchConvey("test Retriever.Retrieve", t, func() {
		ctx := context.Background()
		mockClient := &milvusclient.Client{}

		Mock((*milvusclient.Client).HasCollection).To(func(client *milvusclient.Client, ctx context.Context, option milvusclient.HasCollectionOption, opts ...grpc.CallOption) (bool, error) {
			req := option.Request()
			if req.GetCollectionName() != defaultCollection {
				return false, nil
			}
			return true, nil
		}).Build()

		Mock((*milvusclient.Client).DescribeCollection).To(func(client *milvusclient.Client, ctx context.Context, option milvusclient.DescribeCollectionOption, opts ...grpc.CallOption) (*entity.Collection, error) {
			req := option.Request()
			if req.GetCollectionName() != defaultCollection {
				return nil, fmt.Errorf("collection not found")
			}
			return &entity.Collection{
				Schema: &entity.Schema{
					Fields: []*entity.Field{
						{
							Name:     defaultVectorField,
							DataType: entity.FieldTypeBinaryVector,
							TypeParams: map[string]string{
								"dim": "128",
							},
						},
					},
				},
				Loaded: true,
			}, nil
		}).Build()

		Mock((*milvusclient.Client).GetLoadState).Return(entity.LoadState{State: entity.LoadStateLoaded}, nil).Build()

		PatchConvey("test embedding error", func() {
			r, _ := NewRetriever(ctx, &RetrieverConfig{
				Client:            mockClient,
				Collection:        "",
				Partition:         "",
				VectorField:       "",
				OutputFields:      nil,
				DocumentConverter: nil,
				VectorConverter:   nil,
				MetricType:        "",
				TopK:              0,
				ScoreThreshold:    0,
				Embedding:         &mockEmbedding{err: fmt.Errorf("embedding error")},
			})
			documents, err := r.Retrieve(ctx, "test")

			convey.So(err, convey.ShouldBeError, fmt.Errorf("[milvus retriever] embedding has error: embedding error"))
			convey.So(documents, convey.ShouldBeNil)
		})

		PatchConvey("test embedding vector size not match", func() {
			r, _ := NewRetriever(ctx, &RetrieverConfig{
				Client:            mockClient,
				Collection:        "",
				Partition:         "",
				VectorField:       "",
				OutputFields:      nil,
				DocumentConverter: nil,
				VectorConverter:   nil,
				MetricType:        "",
				TopK:              0,
				ScoreThreshold:    0,
				Embedding:         &mockEmbedding{sizeForCall: []int{2}},
			})
			documents, err := r.Retrieve(ctx, "test")

			convey.So(err, convey.ShouldBeError, fmt.Errorf("[milvus retriever] invalid return length of vector, got=2, expected=1"))
			convey.So(documents, convey.ShouldBeNil)
		})

		PatchConvey("test embedding success", func() {
			Mock((*milvusclient.Client).Search).To(func(client *milvusclient.Client, ctx context.Context, option milvusclient.SearchOption, callOptions ...grpc.CallOption) ([]milvusclient.ResultSet, error) {
				req, err := option.Request()
				if err != nil {
					return nil, err
				}
				collName := req.GetCollectionName()

				// Test collection not found
				if collName == "test_collection" {
					return nil, fmt.Errorf("collection not found")
				}

				// Test filter expression - return empty results (ResultCount = 0)
				if req.GetDsl() != "" {
					return []milvusclient.ResultSet{
						{
							ResultCount: 0,
							Fields:      []column.Column{},
						},
					}, nil
				}

				// Test output fields not supported - simulate result error
				if len(req.GetOutputFields()) > 0 && len(req.GetOutputFields()) == 2 {
					return []milvusclient.ResultSet{
						{
							ResultCount: 0,
							Err:         fmt.Errorf("output fields not supported"),
							Fields:      []column.Column{},
						},
					}, nil
				}

				// Normal successful case
				return []milvusclient.ResultSet{
					{
						Fields: []column.Column{
							column.NewColumnVarChar("id", []string{"1", "2"}),
							column.NewColumnVarChar("content", []string{"test", "test"}),
							column.NewColumnBinaryVector("vector", 128, [][]byte{{1, 2, 3}, {4, 5, 6}}),
							column.NewColumnJSONBytes("meta", [][]byte{[]byte(docMetaData), []byte(docMetaData)}),
						},
						Scores:      []float32{1, 2},
						ResultCount: 2,
					},
				}, nil
			}).Build()

			PatchConvey("test search error", func() {
				r, _ := NewRetriever(ctx, &RetrieverConfig{
					Client:            mockClient,
					Collection:        "",
					Partition:         "",
					VectorField:       "",
					OutputFields:      nil,
					DocumentConverter: nil,
					VectorConverter:   nil,
					MetricType:        "",
					TopK:              0,
					ScoreThreshold:    0,
					Embedding:         &mockEmbedding{sizeForCall: []int{1}},
				})
				r.config.Collection = "test_collection"
				documents, err := r.Retrieve(ctx, "test")

				convey.So(err, convey.ShouldBeError, fmt.Errorf("[milvus retriever] search has error: collection not found"))
				convey.So(documents, convey.ShouldBeNil)
			})

			PatchConvey("test search result count is 0", func() {
				r, _ := NewRetriever(ctx, &RetrieverConfig{
					Client:            mockClient,
					Collection:        "",
					Partition:         "",
					VectorField:       "",
					OutputFields:      nil,
					DocumentConverter: nil,
					VectorConverter:   nil,
					MetricType:        "",
					TopK:              0,
					ScoreThreshold:    0,
					Embedding:         &mockEmbedding{sizeForCall: []int{1}},
				})
				documents, err := r.Retrieve(ctx, "test", WithFilter("test"))

				convey.So(err, convey.ShouldBeError, fmt.Errorf("[milvus retriever] no results found"))
				convey.So(documents, convey.ShouldBeNil)
			})

			PatchConvey("test search results has error", func() {
				r, _ := NewRetriever(ctx, &RetrieverConfig{
					Client:            mockClient,
					Collection:        "",
					Partition:         "",
					VectorField:       "",
					OutputFields:      []string{"1", "2"},
					DocumentConverter: nil,
					VectorConverter:   nil,
					MetricType:        "",
					TopK:              0,
					ScoreThreshold:    0,
					Embedding:         &mockEmbedding{sizeForCall: []int{1}},
				})
				documents, err := r.Retrieve(ctx, "test")

				convey.So(err, convey.ShouldBeError, fmt.Errorf("[milvus retriever] search result has error: output fields not supported"))
				convey.So(documents, convey.ShouldBeNil)
			})

			PatchConvey("test search results success", func() {
				r, _ := NewRetriever(ctx, &RetrieverConfig{
					Client:            mockClient,
					Collection:        "",
					Partition:         "",
					VectorField:       "",
					OutputFields:      nil,
					DocumentConverter: nil,
					VectorConverter:   nil,
					MetricType:        "",
					TopK:              0,
					ScoreThreshold:    0,
					Embedding:         &mockEmbedding{sizeForCall: []int{1}},
				})
				documents, err := r.Retrieve(ctx, "test")

				convey.So(err, convey.ShouldBeNil)
				convey.So(documents, convey.ShouldNotBeNil)
			})
		})
	})
}

type mockEmbedding struct {
	err         error
	cnt         int
	sizeForCall []int
	dims        int
}

func (m *mockEmbedding) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	if m.cnt > len(m.sizeForCall) {
		log.Fatal("unexpected")
	}

	if m.err != nil {
		return nil, m.err
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

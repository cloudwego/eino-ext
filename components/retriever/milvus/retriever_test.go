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
	"log"
	"testing"

	. "github.com/bytedance/mockey"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
	"github.com/smartystreets/goconvey/convey"
)

func TestNewRetriever(t *testing.T) {
	PatchConvey("test NewRetriever", t, func() {
		ctx := context.Background()
		Mock(client.NewClient).Return(&client.GrpcClient{}, nil).Build()
		mockClient, _ := client.NewClient(ctx, client.Config{})

		PatchConvey("test retriever config check", func() {
			PatchConvey("test client not provided", func() {
				r, err := NewRetriever(ctx, &RetrieverConfig{
					Client:            nil,
					Collection:        "",
					Partition:         nil,
					VectorField:       "",
					OutputFields:      nil,
					DocumentConverter: nil,
					MetricType:        "",
					TopK:              0,
					ScoreThreshold:    0,
					Sp:                nil,
					Embedding:         &mockEmbedding{},
				})
				convey.So(err, convey.ShouldBeError, fmt.Errorf("[NewRetriever] milvus client not provided"))
				convey.So(r, convey.ShouldBeNil)
			})

			PatchConvey("test embedding not provided", func() {
				r, err := NewRetriever(ctx, &RetrieverConfig{
					Client:            mockClient,
					Collection:        "",
					Partition:         nil,
					VectorField:       "",
					OutputFields:      nil,
					DocumentConverter: nil,
					MetricType:        "",
					TopK:              0,
					ScoreThreshold:    0,
					Sp:                nil,
					Embedding:         nil,
				})
				convey.So(err, convey.ShouldBeError, fmt.Errorf("[NewRetriever] embedding not provided"))
				convey.So(r, convey.ShouldBeNil)
			})

			PatchConvey("test search params not provided and score threshold is out of range", func() {
				r, err := NewRetriever(ctx, &RetrieverConfig{
					Client:            mockClient,
					Collection:        "",
					Partition:         nil,
					VectorField:       "",
					OutputFields:      nil,
					DocumentConverter: nil,
					MetricType:        "",
					TopK:              0,
					ScoreThreshold:    -1,
					Sp:                nil,
					Embedding:         &mockEmbedding{},
				})
				convey.So(err, convey.ShouldBeError, fmt.Errorf("[NewRetriever] invalid search params"))
				convey.So(r, convey.ShouldBeNil)
			})
		})

		PatchConvey("test pre-check", func() {

			Mock(GetMethod(mockClient, "HasCollection")).To(func(ctx context.Context, collName string) (bool, error) {
				if collName != defaultCollection {
					return false, nil
				}
				return true, nil
			}).Build()

			PatchConvey("test collection not found", func() {
				r, err := NewRetriever(ctx, &RetrieverConfig{
					Client:            mockClient,
					Collection:        "test_collection",
					Partition:         nil,
					VectorField:       "",
					OutputFields:      nil,
					DocumentConverter: nil,
					MetricType:        "",
					TopK:              0,
					ScoreThreshold:    0,
					Sp:                nil,
					Embedding:         &mockEmbedding{},
				})
				convey.So(err, convey.ShouldBeError, fmt.Errorf("[NewRetriever] collection not found"))
				convey.So(r, convey.ShouldBeNil)

				Mock(GetMethod(mockClient, "DescribeCollection")).To(func(ctx context.Context, collName string) (*entity.Collection, error) {
					if collName != defaultCollection {
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
					}, nil
				}).Build()

				PatchConvey("test collection schema not match", func() {
					r, err := NewRetriever(ctx, &RetrieverConfig{
						Client:            mockClient,
						Collection:        defaultCollection,
						Partition:         nil,
						VectorField:       "test_vector",
						OutputFields:      nil,
						DocumentConverter: nil,
						MetricType:        "",
						TopK:              0,
						ScoreThreshold:    0,
						Sp:                nil,
						Embedding:         &mockEmbedding{},
					})
					convey.So(err, convey.ShouldBeError, fmt.Errorf("[NewRetriever] collection schema not match: vector field not found"))
					convey.So(r, convey.ShouldBeNil)

					PatchConvey("test collection schema match", func() {
						r, err := NewRetriever(ctx, &RetrieverConfig{
							Client:            mockClient,
							Collection:        "",
							Partition:         nil,
							VectorField:       defaultVectorField,
							OutputFields:      nil,
							DocumentConverter: nil,
							MetricType:        "",
							TopK:              0,
							ScoreThreshold:    0,
							Sp:                nil,
							Embedding:         &mockEmbedding{},
						})
						convey.So(err, convey.ShouldNotBeNil)
						convey.So(r, convey.ShouldBeNil)

						Mock(GetMethod(mockClient, "GetLoadState")).Return(entity.LoadStateLoaded, nil).Build()

						PatchConvey("test create retriever", func() {
							r, err := NewRetriever(ctx, &RetrieverConfig{
								Client:            mockClient,
								Collection:        "",
								Partition:         nil,
								VectorField:       defaultVectorField,
								OutputFields:      nil,
								DocumentConverter: nil,
								MetricType:        "",
								TopK:              0,
								ScoreThreshold:    0,
								Sp:                nil,
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
		Mock(client.NewClient).Return(&client.GrpcClient{}, nil).Build()
		mockClient, _ := client.NewClient(ctx, client.Config{})

		Mock(GetMethod(mockClient, "HasCollection")).To(func(ctx context.Context, collName string) (bool, error) {
			if collName != defaultCollection {
				return false, nil
			}
			return true, nil
		}).Build()

		Mock(GetMethod(mockClient, "DescribeCollection")).To(func(ctx context.Context, collName string) (*entity.Collection, error) {
			if collName != defaultCollection {
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
			}, nil
		}).Build()

		Mock(GetMethod(mockClient, "GetLoadState")).Return(entity.LoadStateLoaded, nil).Build()

		PatchConvey("test embedding error", func() {
			r, _ := NewRetriever(ctx, &RetrieverConfig{
				Client:            mockClient,
				Collection:        "",
				Partition:         nil,
				VectorField:       "",
				OutputFields:      nil,
				DocumentConverter: nil,
				MetricType:        "",
				TopK:              0,
				ScoreThreshold:    0,
				Sp:                nil,
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
				Partition:         nil,
				VectorField:       "",
				OutputFields:      nil,
				DocumentConverter: nil,
				MetricType:        "",
				TopK:              0,
				ScoreThreshold:    0,
				Sp:                nil,
				Embedding:         &mockEmbedding{sizeForCall: []int{2}},
			})
			documents, err := r.Retrieve(ctx, "test")

			convey.So(err, convey.ShouldBeError, fmt.Errorf("[milvus retriever] invalid return length of vector, got=2, expected=1"))
			convey.So(documents, convey.ShouldBeNil)
		})
		PatchConvey("test embedding success", func() {

			Mock(GetMethod(mockClient, "Search")).To(func(ctx context.Context, collName string, partitions []string, expr string, outputFields []string, vectors []entity.Vector, vectorField string, metricType entity.MetricType, topK int, sp entity.SearchParam, opts ...client.SearchQueryOptionFunc) ([]client.SearchResult, error) {
				if collName != defaultCollection {
					return nil, fmt.Errorf("collection not found")
				}
				if expr != "" {
					return []client.SearchResult{}, nil
				}
				if len(outputFields) > 0 {
					return []client.SearchResult{
						{
							ResultCount:  0,
							GroupByValue: nil,
							IDs:          nil,
							Fields:       nil,
							Scores:       nil,
							Err:          fmt.Errorf("output fields not supported"),
						},
					}, nil
				}
				return []client.SearchResult{
					{
						ResultCount:  0,
						GroupByValue: nil,
						IDs:          entity.NewColumnVarChar("id", []string{"1", "2"}),
						Fields: []entity.Column{
							entity.NewColumnVarChar("id", []string{"1", "2"}),
							entity.NewColumnVarChar("content", []string{"test", "test"}),
							entity.NewColumnBinaryVector("vector", 128, [][]byte{{1, 2, 3}, {4, 5, 6}}),
							entity.NewColumnJSONBytes("meta", [][]byte{[]byte(docMetaData)}),
						},
						Scores: []float32{1, 2},
						Err:    nil,
					},
				}, nil
			}).Build()

			PatchConvey("test search error", func() {
				r, _ := NewRetriever(ctx, &RetrieverConfig{
					Client:            mockClient,
					Collection:        "",
					Partition:         nil,
					VectorField:       "",
					OutputFields:      nil,
					DocumentConverter: nil,
					MetricType:        "",
					TopK:              0,
					ScoreThreshold:    0,
					Sp:                nil,
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
					Partition:         nil,
					VectorField:       "",
					OutputFields:      nil,
					DocumentConverter: nil,
					MetricType:        "",
					TopK:              0,
					ScoreThreshold:    0,
					Sp:                nil,
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
					Partition:         nil,
					VectorField:       "",
					OutputFields:      []string{"1", "2"},
					DocumentConverter: nil,
					MetricType:        "",
					TopK:              0,
					ScoreThreshold:    0,
					Sp:                nil,
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
					Partition:         nil,
					VectorField:       "",
					OutputFields:      nil,
					DocumentConverter: nil,
					MetricType:        "",
					TopK:              0,
					ScoreThreshold:    0,
					Sp:                nil,
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

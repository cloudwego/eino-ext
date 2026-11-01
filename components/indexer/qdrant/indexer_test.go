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

package qdrant

import (
	"context"
	"math/rand"
	"testing"

	. "github.com/bytedance/mockey"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/schema"
	qdrant "github.com/qdrant/go-client/qdrant"
	. "github.com/smartystreets/goconvey/convey"
)

const CollectionName string = "test_collection"

func TestIndexer(t *testing.T) {
	ctx := context.Background()

	var collectionExistsCalled, createCollectionCalled, upsertCalled bool
	var upsertCallCount int

	PatchConvey("TestIndexer", t, func() {
		mockClient := &qdrant.Client{}

		Mock(qdrant.NewClient).Return(mockClient, nil).Build()

		Mock((*qdrant.Client).CollectionExists).To(func(c *qdrant.Client, ctx context.Context, collectionName string) (bool, error) {
			collectionExistsCalled = true
			return false, nil
		}).Build()

		Mock((*qdrant.Client).CreateCollection).To(func(c *qdrant.Client, ctx context.Context, req *qdrant.CreateCollection) error {
			createCollectionCalled = true
			return nil
		}).Build()

		Mock((*qdrant.Client).Upsert).To(func(c *qdrant.Client, ctx context.Context, req *qdrant.UpsertPoints) (*qdrant.UpdateResult, error) {
			upsertCalled = true
			upsertCallCount++
			return &qdrant.UpdateResult{}, nil
		}).Build()

		Mock((*qdrant.Client).Count).To(func(c *qdrant.Client, ctx context.Context, req *qdrant.CountPoints) (uint64, error) {
			return uint64(upsertCallCount), nil
		}).Build()

		Convey("Given a Qdrant indexer with mocked client", func() {
			Convey("When creating documents to store", func() {
				d1 := &schema.Document{ID: "c60df334-dbbe-49b8-82d8-a2bd668602f6", Content: "asd"}
				d2 := &schema.Document{ID: "7b83aca0-5f6c-4491-8dd4-22e15e9d582e", Content: "qwe", MetaData: map[string]any{
					"mock_field_1": map[string]any{"extra_field_1": "asd"},
					"mock_field_2": int64(123),
				}}
				docs := []*schema.Document{d1, d2}

				Convey("And creating an indexer", func() {
					i, err := NewIndexer(ctx, &Config{
						Client:     mockClient,
						Collection: CollectionName,
						BatchSize:  10,
						Embedding:  &mockEmbeddingQdrant{dims: 4},
						VectorDim:  4,
						Distance:   qdrant.Distance_Cosine,
					})

					Convey("Then the indexer should be created successfully", func() {
						So(err, ShouldBeNil)
						So(i, ShouldNotBeNil)
					})

					Convey("When storing documents", func() {
						upsertCallCount = 0

						ids, err := i.Store(ctx, docs)

						Convey("Then the store operation should succeed", func() {
							So(err, ShouldBeNil)
							So(ids, ShouldNotBeNil)
							So(len(ids), ShouldEqual, len(docs))
						})

						Convey("And the returned IDs should match the document IDs", func() {
							expectedIDs := []string{d1.ID, d2.ID}
							So(ids, ShouldResemble, expectedIDs)
						})

						Convey("And the mock methods should be called correctly", func() {
							So(collectionExistsCalled, ShouldBeTrue)
							So(createCollectionCalled, ShouldBeTrue)
							So(upsertCalled, ShouldBeTrue)
							So(upsertCallCount, ShouldEqual, 1)
						})
					})
				})
			})
		})
	})
}

type mockEmbeddingQdrant struct {
	err  error
	dims int
}

func (m *mockEmbeddingQdrant) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	if m.err != nil {
		return nil, m.err
	}

	result := make([][]float64, len(texts))
	for i := range texts {
		vec := make([]float64, m.dims)
		for j := range vec {
			vec[j] = rand.Float64()
		}
		result[i] = vec
	}
	return result, nil
}

// mockCollectionInfo builds the CollectionInfo an existing collection returns.
func mockCollectionInfo(params map[string]*qdrant.VectorParams) *qdrant.CollectionInfo {
	return &qdrant.CollectionInfo{
		Config: &qdrant.CollectionConfig{
			Params: &qdrant.CollectionParams{
				VectorsConfig: qdrant.NewVectorsConfigMap(params),
			},
		},
	}
}

func TestIndexerMultipleVectors(t *testing.T) {
	ctx := context.Background()

	const (
		nameA = "model-a"
		nameB = "model-b"
	)

	vectors := []VectorConfig{
		{Name: nameA, Dimension: 4, Distance: qdrant.Distance_Cosine, Embedding: &mockEmbeddingQdrant{dims: 4}},
		{Name: nameB, Dimension: 8, Distance: qdrant.Distance_Dot, Embedding: &mockEmbeddingQdrant{dims: 8}},
	}

	PatchConvey("TestIndexerMultipleVectors", t, func() {
		mockClient := &qdrant.Client{}

		var createCollectionReq *qdrant.CreateCollection
		var createCollectionCalled bool
		var upsertReq *qdrant.UpsertPoints
		var upsertCallCount int

		Mock((*qdrant.Client).Upsert).To(func(c *qdrant.Client, ctx context.Context, req *qdrant.UpsertPoints) (*qdrant.UpdateResult, error) {
			upsertReq = req
			upsertCallCount++
			return &qdrant.UpdateResult{}, nil
		}).Build()

		Mock((*qdrant.Client).CreateCollection).To(func(c *qdrant.Client, ctx context.Context, req *qdrant.CreateCollection) error {
			createCollectionReq = req
			createCollectionCalled = true
			return nil
		}).Build()

		Convey("When the collection does not exist, it is created with every named vector", func() {
			Mock((*qdrant.Client).CollectionExists).To(func(c *qdrant.Client, ctx context.Context, collectionName string) (bool, error) {
				return false, nil
			}).Build()

			i, err := NewIndexer(ctx, &Config{
				Client:     mockClient,
				Collection: CollectionName,
				Vectors:    vectors,
			})
			So(err, ShouldBeNil)
			So(i, ShouldNotBeNil)

			paramsMap := createCollectionReq.GetVectorsConfig().GetParamsMap().GetMap()
			So(paramsMap, ShouldHaveLength, 2)
			So(paramsMap[nameA].GetSize(), ShouldEqual, 4)
			So(paramsMap[nameA].GetDistance(), ShouldEqual, qdrant.Distance_Cosine)
			So(paramsMap[nameB].GetSize(), ShouldEqual, 8)
			So(paramsMap[nameB].GetDistance(), ShouldEqual, qdrant.Distance_Dot)

			Convey("And one upsert carries all spaces of every point", func() {
				docs := []*schema.Document{
					{ID: "doc-1", Content: "hello"},
					{ID: "doc-2", Content: "world"},
				}
				_, err := i.Store(ctx, docs)
				So(err, ShouldBeNil)

				// A single request: writing one space at a time would null the
				// vectors of the other spaces.
				So(upsertCallCount, ShouldEqual, 1)
				So(upsertReq.GetPoints(), ShouldHaveLength, len(docs))

				for idx, point := range upsertReq.GetPoints() {
					So(point.GetId().GetUuid(), ShouldEqual, docs[idx].ID)
					vecMap := point.GetVectors().GetVectors().GetVectors()
					So(vecMap, ShouldHaveLength, 2)
					So(vecMap[nameA].GetData(), ShouldHaveLength, 4)
					So(vecMap[nameB].GetData(), ShouldHaveLength, 8)
				}
			})
		})

		Convey("When the collection exists and carries all spaces, no creation happens", func() {
			Mock((*qdrant.Client).CollectionExists).To(func(c *qdrant.Client, ctx context.Context, collectionName string) (bool, error) {
				return true, nil
			}).Build()

			Mock((*qdrant.Client).GetCollectionInfo).To(func(c *qdrant.Client, ctx context.Context, collectionName string) (*qdrant.CollectionInfo, error) {
				return mockCollectionInfo(map[string]*qdrant.VectorParams{
					nameA: {Size: 4, Distance: qdrant.Distance_Cosine},
					nameB: {Size: 8, Distance: qdrant.Distance_Dot},
				}), nil
			}).Build()

			i, err := NewIndexer(ctx, &Config{
				Client:     mockClient,
				Collection: CollectionName,
				Vectors:    vectors,
			})
			So(err, ShouldBeNil)
			So(i, ShouldNotBeNil)
			So(createCollectionCalled, ShouldBeFalse)
		})

		Convey("When the existing collection misses one space, creation fails with a clear error", func() {
			Mock((*qdrant.Client).CollectionExists).To(func(c *qdrant.Client, ctx context.Context, collectionName string) (bool, error) {
				return true, nil
			}).Build()

			Mock((*qdrant.Client).GetCollectionInfo).To(func(c *qdrant.Client, ctx context.Context, collectionName string) (*qdrant.CollectionInfo, error) {
				return mockCollectionInfo(map[string]*qdrant.VectorParams{
					nameA: {Size: 4, Distance: qdrant.Distance_Cosine},
				}), nil
			}).Build()

			_, err := NewIndexer(ctx, &Config{
				Client:     mockClient,
				Collection: CollectionName,
				Vectors:    vectors,
			})
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, nameB)
			So(err.Error(), ShouldContainSubstring, "named vector")
		})

		Convey("When the existing collection has a space with a different dimension, creation fails", func() {
			Mock((*qdrant.Client).CollectionExists).To(func(c *qdrant.Client, ctx context.Context, collectionName string) (bool, error) {
				return true, nil
			}).Build()

			Mock((*qdrant.Client).GetCollectionInfo).To(func(c *qdrant.Client, ctx context.Context, collectionName string) (*qdrant.CollectionInfo, error) {
				return mockCollectionInfo(map[string]*qdrant.VectorParams{
					nameA: {Size: 4, Distance: qdrant.Distance_Cosine},
					nameB: {Size: 16, Distance: qdrant.Distance_Dot},
				}), nil
			}).Build()

			_, err := NewIndexer(ctx, &Config{
				Client:     mockClient,
				Collection: CollectionName,
				Vectors:    vectors,
			})
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "size 16")
			So(err.Error(), ShouldContainSubstring, "Dimension is 8")
		})

		Convey("When the existing collection has a space with a different distance, creation fails", func() {
			Mock((*qdrant.Client).CollectionExists).To(func(c *qdrant.Client, ctx context.Context, collectionName string) (bool, error) {
				return true, nil
			}).Build()

			Mock((*qdrant.Client).GetCollectionInfo).To(func(c *qdrant.Client, ctx context.Context, collectionName string) (*qdrant.CollectionInfo, error) {
				return mockCollectionInfo(map[string]*qdrant.VectorParams{
					nameA: {Size: 4, Distance: qdrant.Distance_Euclid},
					nameB: {Size: 8, Distance: qdrant.Distance_Dot},
				}), nil
			}).Build()

			_, err := NewIndexer(ctx, &Config{
				Client:     mockClient,
				Collection: CollectionName,
				Vectors:    vectors,
			})
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "distance")
		})

		Convey("When the embedder returns a vector of the wrong dimension, storing fails", func() {
			Mock((*qdrant.Client).CollectionExists).To(func(c *qdrant.Client, ctx context.Context, collectionName string) (bool, error) {
				return false, nil
			}).Build()

			i, err := NewIndexer(ctx, &Config{
				Client:     mockClient,
				Collection: CollectionName,
				Vectors: []VectorConfig{
					{Name: nameA, Dimension: 4, Distance: qdrant.Distance_Cosine, Embedding: &mockEmbeddingQdrant{dims: 3}},
				},
			})
			So(err, ShouldBeNil)

			_, err = i.Store(ctx, []*schema.Document{{ID: "doc-1", Content: "hello"}})
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "expected dimension 4")
		})

		Convey("When Vectors is combined with the unnamed single-vector fields, creation fails", func() {
			_, err := NewIndexer(ctx, &Config{
				Client:     mockClient,
				Collection: CollectionName,
				VectorDim:  4,
				Distance:   qdrant.Distance_Cosine,
				Vectors:    vectors,
			})
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "cannot be combined")
		})

		Convey("When Vectors is combined with Embedding, creation fails", func() {
			_, err := NewIndexer(ctx, &Config{
				Client:     mockClient,
				Collection: CollectionName,
				Embedding:  &mockEmbeddingQdrant{dims: 4},
				Vectors:    vectors,
			})
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "cannot be combined")
		})

		Convey("When Vectors is combined with the unnamed Distance, creation fails", func() {
			_, err := NewIndexer(ctx, &Config{
				Client:     mockClient,
				Collection: CollectionName,
				Distance:   qdrant.Distance_Euclid,
				Vectors:    vectors,
			})
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "cannot be combined")
		})

		Convey("When a space leaves Distance unset, creation fails", func() {
			_, err := NewIndexer(ctx, &Config{
				Client:     mockClient,
				Collection: CollectionName,
				Vectors: []VectorConfig{
					{Name: nameA, Dimension: 4, Embedding: &mockEmbeddingQdrant{dims: 4}},
				},
			})
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "Distance is required")
		})

		Convey("When two spaces share a name, creation fails", func() {
			_, err := NewIndexer(ctx, &Config{
				Client:     mockClient,
				Collection: CollectionName,
				Vectors: []VectorConfig{
					{Name: nameA, Dimension: 4, Distance: qdrant.Distance_Cosine, Embedding: &mockEmbeddingQdrant{dims: 4}},
					{Name: nameA, Dimension: 8, Distance: qdrant.Distance_Dot, Embedding: &mockEmbeddingQdrant{dims: 8}},
				},
			})
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "duplicate Name")
		})

		Convey("When a space has no name or no embedder, creation fails", func() {
			_, err := NewIndexer(ctx, &Config{
				Client:     mockClient,
				Collection: CollectionName,
				Vectors: []VectorConfig{
					{Dimension: 4, Distance: qdrant.Distance_Cosine, Embedding: &mockEmbeddingQdrant{dims: 4}},
				},
			})
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "Name is required")

			_, err = NewIndexer(ctx, &Config{
				Client:     mockClient,
				Collection: CollectionName,
				Vectors: []VectorConfig{
					{Name: nameA, Dimension: 4, Distance: qdrant.Distance_Cosine},
				},
			})
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "Embedding is required")
		})

		Convey("When an embedding option is passed to a multi-vector indexer, storing fails", func() {
			Mock((*qdrant.Client).CollectionExists).To(func(c *qdrant.Client, ctx context.Context, collectionName string) (bool, error) {
				return false, nil
			}).Build()

			i, err := NewIndexer(ctx, &Config{
				Client:     mockClient,
				Collection: CollectionName,
				Vectors: []VectorConfig{
					{Name: nameA, Dimension: 4, Distance: qdrant.Distance_Cosine, Embedding: &mockEmbeddingQdrant{dims: 4}},
				},
			})
			So(err, ShouldBeNil)

			_, err = i.Store(ctx, []*schema.Document{{ID: "doc-1", Content: "hello"}},
				indexer.WithEmbedding(&mockEmbeddingQdrant{dims: 4}))
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "embedding option is not supported")
		})

		Convey("When Vectors is empty, the unnamed vector API is used (backward compatible)", func() {
			Mock((*qdrant.Client).CollectionExists).To(func(c *qdrant.Client, ctx context.Context, collectionName string) (bool, error) {
				return false, nil
			}).Build()

			i, err := NewIndexer(ctx, &Config{
				Client:     mockClient,
				Collection: CollectionName,
				VectorDim:  4,
				Distance:   qdrant.Distance_Cosine,
				Embedding:  &mockEmbeddingQdrant{dims: 4},
			})
			So(err, ShouldBeNil)

			_, err = i.Store(ctx, []*schema.Document{{ID: "doc-1", Content: "hello"}})
			So(err, ShouldBeNil)

			So(createCollectionReq.GetVectorsConfig().GetParams(), ShouldNotBeNil)
			So(createCollectionReq.GetVectorsConfig().GetParamsMap(), ShouldBeNil)

			So(upsertReq, ShouldNotBeNil)
			So(upsertReq.GetPoints()[0].GetVectors().GetVector(), ShouldNotBeNil)
		})
	})
}

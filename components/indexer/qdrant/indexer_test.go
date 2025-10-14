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

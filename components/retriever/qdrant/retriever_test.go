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
	"fmt"
	"math/rand"
	"testing"

	. "github.com/bytedance/mockey"
	"github.com/cloudwego/eino/components/embedding"
	qdrant "github.com/qdrant/go-client/qdrant"
	. "github.com/smartystreets/goconvey/convey"
)

const CollectionName string = "test_collection"

func TestNewRetrieverValidation(t *testing.T) {
	PatchConvey("TestNewRetrieverValidation", t, func() {
		mockEmbedding := &mockEmbeddingQdrant{dims: 4}

		Convey("Given nil config", func() {
			retriever, err := NewRetriever(context.Background(), nil)

			Convey("Then it should return an error", func() {
				So(err, ShouldNotBeNil)
				So(retriever, ShouldBeNil)
				So(err.Error(), ShouldContainSubstring, "config is nil")
			})
		})

		Convey("Given config with nil embedding", func() {
			retriever, err := NewRetriever(context.Background(), &Config{
				Client:     nil,
				Collection: CollectionName,
				Embedding:  nil,
			})

			Convey("Then it should return an error", func() {
				So(err, ShouldNotBeNil)
				So(retriever, ShouldBeNil)
				So(err.Error(), ShouldContainSubstring, "embedding not provided")
			})
		})

		Convey("Given config with empty collection", func() {
			retriever, err := NewRetriever(context.Background(), &Config{
				Client:     nil,
				Collection: "",
				Embedding:  mockEmbedding,
			})

			Convey("Then it should return an error", func() {
				So(err, ShouldNotBeNil)
				So(retriever, ShouldBeNil)
				So(err.Error(), ShouldContainSubstring, "collection not provided")
			})
		})

		Convey("Given config with nil client", func() {
			retriever, err := NewRetriever(context.Background(), &Config{
				Client:     nil,
				Collection: CollectionName,
				Embedding:  mockEmbedding,
			})

			Convey("Then it should return an error", func() {
				So(err, ShouldNotBeNil)
				So(retriever, ShouldBeNil)
				So(err.Error(), ShouldContainSubstring, "client not provided")
			})
		})
	})
}

func TestRetrieverRetrieve(t *testing.T) {
	PatchConvey("TestRetrieverRetrieve", t, func() {
		ctx := context.Background()
		mockEmbedding := &mockEmbeddingQdrant{dims: 4}
		mockClient := &qdrant.Client{}

		var queryCalled bool

		Mock((*qdrant.Client).Query).To(func(c *qdrant.Client, ctx context.Context, req *qdrant.QueryPoints) ([]*qdrant.ScoredPoint, error) {
			queryCalled = true

			return []*qdrant.ScoredPoint{
				{
					Id:    qdrant.NewID("fba95545-ef38-4880-bf4a-b98174554103"),
					Score: 0.95,
					Payload: map[string]*qdrant.Value{
						defaultContentKey: qdrant.NewValueString("Test content 1"),
					},
				},
			}, nil
		}).Build()

		Convey("Given a valid retriever", func() {
			retriever, err := NewRetriever(ctx, &Config{
				Client:     mockClient,
				Collection: CollectionName,
				Embedding:  mockEmbedding,
				TopK:       5,
			})

			Convey("Then the retriever should be created successfully", func() {
				So(err, ShouldBeNil)
				So(retriever, ShouldNotBeNil)
			})

			Convey("When retrieving documents", func() {
				docs, err := retriever.Retrieve(ctx, "test query")

				Convey("Then the retrieve operation should succeed", func() {
					So(err, ShouldBeNil)
					So(docs, ShouldNotBeNil)
					So(len(docs), ShouldEqual, 1)
				})

				Convey("And the document should have correct content", func() {
					So(docs[0].Content, ShouldEqual, "Test content 1")
					So(docs[0].ID, ShouldEqual, "fba95545-ef38-4880-bf4a-b98174554103")
				})

				Convey("And the mock Query method should be called", func() {
					So(queryCalled, ShouldBeTrue)
				})
			})
		})
	})
}

func TestRetrieverRetrieveWithError(t *testing.T) {
	PatchConvey("TestRetrieverRetrieveWithError", t, func() {
		ctx := context.Background()
		mockEmbedding := &mockEmbeddingQdrant{
			err:  fmt.Errorf("embedding failed"),
			dims: 4,
		}
		mockClient := &qdrant.Client{}

		Convey("Given a retriever with failing embedding", func() {
			retriever, err := NewRetriever(ctx, &Config{
				Client:     mockClient,
				Collection: CollectionName,
				Embedding:  mockEmbedding,
				TopK:       5,
			})

			Convey("Then the retriever should be created successfully", func() {
				So(err, ShouldBeNil)
				So(retriever, ShouldNotBeNil)
			})

			Convey("When retrieving documents", func() {
				docs, err := retriever.Retrieve(ctx, "test query")

				Convey("Then the retrieve operation should fail", func() {
					So(err, ShouldNotBeNil)
					So(docs, ShouldBeNil)
					So(err.Error(), ShouldContainSubstring, "embedding failed")
				})
			})
		})
	})
}

func TestRetrieverTypeAndCallbacks(t *testing.T) {
	PatchConvey("TestRetrieverTypeAndCallbacks", t, func() {
		mockEmbedding := &mockEmbeddingQdrant{dims: 4}
		mockClient := &qdrant.Client{}

		Convey("Given a valid retriever", func() {
			retriever, err := NewRetriever(context.Background(), &Config{
				Client:     mockClient,
				Collection: CollectionName,
				Embedding:  mockEmbedding,
				TopK:       5,
			})

			Convey("Then the retriever should be created successfully", func() {
				So(err, ShouldBeNil)
				So(retriever, ShouldNotBeNil)
			})

			Convey("And GetType should return 'Qdrant'", func() {
				So(retriever.GetType(), ShouldEqual, "Qdrant")
			})

			Convey("And IsCallbacksEnabled should return true", func() {
				So(retriever.IsCallbacksEnabled(), ShouldBeTrue)
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

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
	"github.com/milvus-io/milvus/client/v2/entity"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
	"github.com/smartystreets/goconvey/convey"
)

// Mock Embedding implementation
type mockEmbedding struct{}

func (m *mockEmbedding) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	result := make([][]float64, len(texts))
	for i := range texts {
		result[i] = []float64{0.1, 0.2, 0.3}
	}
	return result, nil
}

func TestNewRetriever(t *testing.T) {
	PatchConvey("test NewRetriever", t, func() {
		mockEmb := &mockEmbedding{}
		// Create a non-nil milvusclient.Client pointer for testing
		mockClient := &milvusclient.Client{}

		PatchConvey("test retriever config validation", func() {
			PatchConvey("test client is nil", func() {
				r, err := NewRetriever(&RetrieverConfig{
					Client:    nil,
					Embedding: mockEmb,
				})
				convey.So(err, convey.ShouldNotBeNil)
				convey.So(err.Error(), convey.ShouldContainSubstring, "milvus client is nil")
				convey.So(r, convey.ShouldBeNil)
			})

			PatchConvey("test valid config", func() {
				r, err := NewRetriever(&RetrieverConfig{
					Client:    mockClient,
					Embedding: mockEmb,
				})
				convey.So(err, convey.ShouldBeNil)
				convey.So(r, convey.ShouldNotBeNil)
				convey.So(r.conf, convey.ShouldNotBeNil)
				convey.So(r.conf.Client, convey.ShouldEqual, mockClient)
				convey.So(r.conf.Embedding, convey.ShouldEqual, mockEmb)
			})

			PatchConvey("test config with default values", func() {
				r, err := NewRetriever(&RetrieverConfig{
					Client:    mockClient,
					Embedding: mockEmb,
				})
				convey.So(err, convey.ShouldBeNil)
				convey.So(r, convey.ShouldNotBeNil)
				// Verify default values
				convey.So(r.conf.Collection, convey.ShouldEqual, defaultCollection)
				convey.So(r.conf.TopK, convey.ShouldEqual, defaultTopK)
				convey.So(r.conf.DocumentConverter, convey.ShouldNotBeNil)
				convey.So(r.conf.VectorConverter, convey.ShouldNotBeNil)
			})
		})
	})
}

func TestRetriever_GetType(t *testing.T) {
	PatchConvey("test Retriever.GetType", t, func() {
		mockEmb := &mockEmbedding{}
		mockClient := &milvusclient.Client{}

		r, err := NewRetriever(&RetrieverConfig{
			Client:    mockClient,
			Embedding: mockEmb,
		})
		convey.So(err, convey.ShouldBeNil)
		convey.So(r, convey.ShouldNotBeNil)

		typeStr := r.GetType()
		convey.So(typeStr, convey.ShouldEqual, typ)
		convey.So(typeStr, convey.ShouldEqual, "Milvus")
	})
}

func TestRetriever_Retrieve(t *testing.T) {
	PatchConvey("test Retriever.Retrieve", t, func() {
		ctx := context.Background()
		mockEmb := &mockEmbedding{}
		mockClient := &milvusclient.Client{}

		PatchConvey("test retrieve with nil embedding", func() {
			r, err := NewRetriever(&RetrieverConfig{
				Client:    mockClient,
				Embedding: nil,
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(r, convey.ShouldNotBeNil)

			// Test case with nil embedding
			docs, err := r.Retrieve(ctx, "test query")
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "embedding is nil")
			convey.So(docs, convey.ShouldBeNil)
		})

		PatchConvey("test retrieve with embedding error", func() {
			// Mock embedding error
			mockEmbWithError := &mockEmbedding{}
			Mock(GetMethod(mockEmbWithError, "EmbedStrings")).Return(nil, fmt.Errorf("embedding error")).Build()

			r, err := NewRetriever(&RetrieverConfig{
				Client:    mockClient,
				Embedding: mockEmbWithError,
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(r, convey.ShouldNotBeNil)

			docs, err := r.Retrieve(ctx, "test query")
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "embed vectors has error")
			convey.So(docs, convey.ShouldBeNil)
		})

		PatchConvey("test retrieve with empty vectors", func() {
			// Mock returning empty vectors
			mockEmbEmpty := &mockEmbedding{}
			Mock(GetMethod(mockEmbEmpty, "EmbedStrings")).Return([][]float64{}, nil).Build()

			r, err := NewRetriever(&RetrieverConfig{
				Client:    mockClient,
				Embedding: mockEmbEmpty,
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(r, convey.ShouldNotBeNil)

			docs, err := r.Retrieve(ctx, "test query")
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "no vectors generated for the query")
			convey.So(docs, convey.ShouldBeNil)
		})

		PatchConvey("test retrieve with vector converter error", func() {
			// Mock vector converter error
			mockVectorConverter := func(vectors [][]float64) ([]entity.Vector, error) {
				return nil, fmt.Errorf("vector converter error")
			}

			r, err := NewRetriever(&RetrieverConfig{
				Client:          mockClient,
				Embedding:       mockEmb,
				VectorConverter: mockVectorConverter,
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(r, convey.ShouldNotBeNil)

			docs, err := r.Retrieve(ctx, "test query")
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "no vectors generated for the query")
			convey.So(docs, convey.ShouldBeNil)
		})

		PatchConvey("test retrieve with search error", func() {
			// Mock search error
			Mock((*milvusclient.Client).Search).Return(nil, fmt.Errorf("search error")).Build()

			r, err := NewRetriever(&RetrieverConfig{
				Client:    mockClient,
				Embedding: mockEmb,
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(r, convey.ShouldNotBeNil)

			docs, err := r.Retrieve(ctx, "test query")
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "query has error")
			convey.So(docs, convey.ShouldBeNil)
		})

		PatchConvey("test retrieve with document converter error", func() {
			// Mock successful search but document converter error
			mockResultSet := milvusclient.ResultSet{}
			Mock((*milvusclient.Client).Search).Return([]milvusclient.ResultSet{mockResultSet}, nil).Build()

			mockDocConverter := func(result []milvusclient.ResultSet) ([]*schema.Document, error) {
				return nil, fmt.Errorf("document converter error")
			}

			r, err := NewRetriever(&RetrieverConfig{
				Client:            mockClient,
				Embedding:         mockEmb,
				DocumentConverter: mockDocConverter,
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(r, convey.ShouldNotBeNil)

			docs, err := r.Retrieve(ctx, "test query")
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "query has error")
			convey.So(docs, convey.ShouldBeNil)
		})

		PatchConvey("test successful retrieve", func() {
			// Mock successful search and document conversion
			mockResultSet := milvusclient.ResultSet{}
			Mock((*milvusclient.Client).Search).Return([]milvusclient.ResultSet{mockResultSet}, nil).Build()

			expectedDocs := []*schema.Document{
				{
					ID:      "doc1",
					Content: "test content",
					MetaData: map[string]any{
						"score": 0.95,
					},
				},
			}

			mockDocConverter := func(result []milvusclient.ResultSet) ([]*schema.Document, error) {
				return expectedDocs, nil
			}

			r, err := NewRetriever(&RetrieverConfig{
				Client:            mockClient,
				Embedding:         mockEmb,
				DocumentConverter: mockDocConverter,
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(r, convey.ShouldNotBeNil)

			docs, err := r.Retrieve(ctx, "test query")
			convey.So(err, convey.ShouldBeNil)
			convey.So(docs, convey.ShouldNotBeNil)
			convey.So(len(docs), convey.ShouldEqual, 1)
			convey.So(docs[0].ID, convey.ShouldEqual, "doc1")
			convey.So(docs[0].Content, convey.ShouldEqual, "test content")
		})

		PatchConvey("test retrieve with hybrid search", func() {
			// Mock hybrid search
			mockResultSet := milvusclient.ResultSet{}
			Mock((*milvusclient.Client).HybridSearch).Return([]milvusclient.ResultSet{mockResultSet}, nil).Build()

			expectedDocs := []*schema.Document{
				{
					ID:      "doc1",
					Content: "hybrid search result",
				},
			}

			mockDocConverter := func(result []milvusclient.ResultSet) ([]*schema.Document, error) {
				return expectedDocs, nil
			}

			r, err := NewRetriever(&RetrieverConfig{
				Client:            mockClient,
				Embedding:         mockEmb,
				DocumentConverter: mockDocConverter,
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(r, convey.ShouldNotBeNil)

			// Use hybrid search option
			hybridOption := NewHybridSearchOption("vector_field", 10)
			docs, err := r.Retrieve(ctx, "test query", WithHybridSearchOption(hybridOption))
			convey.So(err, convey.ShouldBeNil)
			convey.So(docs, convey.ShouldNotBeNil)
			convey.So(len(docs), convey.ShouldEqual, 1)
			convey.So(docs[0].Content, convey.ShouldEqual, "hybrid search result")
		})

		PatchConvey("test retrieve with custom limit", func() {
			// Mock successful search
			mockResultSet := milvusclient.ResultSet{}
			Mock((*milvusclient.Client).Search).Return([]milvusclient.ResultSet{mockResultSet}, nil).Build()

			expectedDocs := []*schema.Document{
				{ID: "doc1", Content: "content1"},
				{ID: "doc2", Content: "content2"},
			}

			mockDocConverter := func(result []milvusclient.ResultSet) ([]*schema.Document, error) {
				return expectedDocs, nil
			}

			r, err := NewRetriever(&RetrieverConfig{
				Client:            mockClient,
				Embedding:         mockEmb,
				DocumentConverter: mockDocConverter,
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(r, convey.ShouldNotBeNil)

			// Use custom limit
			docs, err := r.Retrieve(ctx, "test query", WithLimit(10))
			convey.So(err, convey.ShouldBeNil)
			convey.So(docs, convey.ShouldNotBeNil)
			convey.So(len(docs), convey.ShouldEqual, 2)
		})
	})
}
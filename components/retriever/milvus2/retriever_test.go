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
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus/client/v2/column"
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

// mockSearchMode implements SearchMode for testing (avoids import cycle)
type mockSearchMode struct {
	retrieveFunc func(ctx context.Context, client *milvusclient.Client, conf *RetrieverConfig, query string, opts ...retriever.Option) ([]*schema.Document, error)
}

func (m *mockSearchMode) Retrieve(ctx context.Context, client *milvusclient.Client, conf *RetrieverConfig, query string, opts ...retriever.Option) ([]*schema.Document, error) {
	if m.retrieveFunc != nil {
		return m.retrieveFunc(ctx, client, conf, query, opts...)
	}
	return []*schema.Document{{ID: "1", Content: "doc1"}}, nil
}

func TestRetrieverConfig_validate(t *testing.T) {
	convey.Convey("test RetrieverConfig.validate", t, func() {
		mockEmb := &mockEmbedding{}
		mockSM := &mockSearchMode{}

		convey.Convey("test missing client and client config", func() {
			config := &RetrieverConfig{
				Client:       nil,
				ClientConfig: nil,
				Collection:   "test_collection",
				Embedding:    mockEmb,
				SearchMode:   mockSM,
			}
			err := config.validate()
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "client")
		})

		// QuerySearchMode and SparseSearchMode specific validations are removed as SearchMode is now polymorphic.
		// The requirement for Embedding is now enforced by specific implementations inside Retrieve/BuildOptions if needed, not strictly by Config.validate for generic SearchMode.
		// However, if the code still checks for embedding availability generally:

		convey.Convey("test valid config", func() {
			config := &RetrieverConfig{
				ClientConfig: &milvusclient.ClientConfig{Address: "localhost:19530"},
				Collection:   "test_collection",
				Embedding:    mockEmb,
				SearchMode:   mockSM,
			}
			err := config.validate()
			convey.So(err, convey.ShouldBeNil)
		})

		convey.Convey("test missing search mode", func() {
			config := &RetrieverConfig{
				ClientConfig: &milvusclient.ClientConfig{Address: "localhost:19530"},
				Collection:   "test_collection",
				Embedding:    mockEmb,
				SearchMode:   nil,
			}
			err := config.validate()
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "search mode")
		})
	})
}

func TestNewRetriever(t *testing.T) {
	PatchConvey("test NewRetriever", t, func() {
		ctx := context.Background()
		mockEmb := &mockEmbedding{dims: 128}
		mockSM := &mockSearchMode{}

		PatchConvey("test missing client and client config", func() {
			_, err := NewRetriever(ctx, &RetrieverConfig{
				Client:       nil,
				ClientConfig: nil,
				Collection:   "test_collection",
				Embedding:    mockEmb,
				SearchMode:   mockSM,
			})
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "client")
		})

		PatchConvey("test missing search mode", func() {
			_, err := NewRetriever(ctx, &RetrieverConfig{
				ClientConfig: &milvusclient.ClientConfig{Address: "localhost:19530"},
				Collection:   "test_collection",
				Embedding:    mockEmb,
				SearchMode:   nil,
			})
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "search mode")
		})
	})
}

func TestRetriever_GetType(t *testing.T) {
	convey.Convey("test Retriever.GetType", t, func() {
		r := &Retriever{}
		result := r.GetType()
		convey.So(result, convey.ShouldEqual, "Milvus2")
	})
}

func TestRetriever_IsCallbacksEnabled(t *testing.T) {
	convey.Convey("test Retriever.IsCallbacksEnabled", t, func() {
		r := &Retriever{
			config: &RetrieverConfig{},
		}
		result := r.IsCallbacksEnabled()
		convey.So(result, convey.ShouldBeTrue)
	})
}

func TestRetriever_Retrieve(t *testing.T) {
	PatchConvey("test Retriever.Retrieve", t, func() {
		ctx := context.Background()
		mockEmb := &mockEmbedding{dims: 128}
		mockClient := &milvusclient.Client{}
		mockSM := &mockSearchMode{}

		r := &Retriever{
			client: mockClient,
			config: &RetrieverConfig{
				Collection:   "test_collection",
				Embedding:    mockEmb,
				VectorField:  "vector",
				TopK:         10,
				OutputFields: []string{"id", "content"},
				SearchMode:   mockSM,
			},
		}

		PatchConvey("test retrieve success", func() {
			expectedDocs := []*schema.Document{{ID: "1", Content: "success"}}
			mockSM.retrieveFunc = func(ctx context.Context, client *milvusclient.Client, conf *RetrieverConfig, query string, opts ...retriever.Option) ([]*schema.Document, error) {
				return expectedDocs, nil
			}
			docs, err := r.Retrieve(ctx, "test query")
			convey.So(err, convey.ShouldBeNil)
			convey.So(docs, convey.ShouldResemble, expectedDocs)
		})

		PatchConvey("test retrieve error", func() {
			mockSM.retrieveFunc = func(ctx context.Context, client *milvusclient.Client, conf *RetrieverConfig, query string, opts ...retriever.Option) ([]*schema.Document, error) {
				return nil, fmt.Errorf("search error")
			}
			docs, err := r.Retrieve(ctx, "test query")
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "search error")
			convey.So(docs, convey.ShouldBeNil)
		})
	})
}

func TestEmbedQuery(t *testing.T) {
	PatchConvey("test EmbedQuery", t, func() {
		ctx := context.Background()
		// No need for retriever instance

		PatchConvey("test embedding success returns float32 vector", func() {
			mockEmb := &mockEmbedding{dims: 128}
			vector, err := EmbedQuery(ctx, mockEmb, "test query")
			convey.So(err, convey.ShouldBeNil)
			convey.So(len(vector), convey.ShouldEqual, 128)
			// First element should be 0.1 converted to float32
			convey.So(vector[0], convey.ShouldEqual, float32(0.1))
		})

		PatchConvey("test embedding error", func() {
			mockEmb := &mockEmbedding{err: fmt.Errorf("embedding failed")}
			vector, err := EmbedQuery(ctx, mockEmb, "test query")
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(vector, convey.ShouldBeNil)
		})

		PatchConvey("test embedding empty result", func() {
			mockEmb := &mockEmbedding{dims: 0}
			// Even with dims=0, the mock returns 128 (default)
			vector, err := EmbedQuery(ctx, mockEmb, "test query")
			convey.So(err, convey.ShouldBeNil)
			convey.So(len(vector), convey.ShouldBeGreaterThan, 0)
		})
	})
}

func TestWithFilter(t *testing.T) {
	convey.Convey("test WithFilter option", t, func() {
		opt := WithFilter("id > 10")
		convey.So(opt, convey.ShouldNotBeNil)
	})
}

func TestWithGrouping(t *testing.T) {
	convey.Convey("test WithGrouping option", t, func() {
		opt := WithGrouping("category", 3, true)
		convey.So(opt, convey.ShouldNotBeNil)
	})
}

func TestDocumentConverter(t *testing.T) {
	convey.Convey("test defaultDocumentConverter", t, func() {
		ctx := context.Background()
		converter := defaultDocumentConverter()

		convey.Convey("convert standard results with scores and metadata", func() {
			ids := []string{"1", "2"}
			contents := []string{"doc1", "doc2"}
			scores := []float32{0.9, 0.8}
			// Metadata: {"key": "val1"}, {"key": "val2"}
			metas := [][]byte{
				[]byte(`{"key": "val1"}`),
				[]byte(`{"key": "val2"}`),
			}

			resultSet := createMockResultSet(ids, contents, scores, metas)
			docs, err := converter(ctx, resultSet)

			convey.So(err, convey.ShouldBeNil)
			convey.So(len(docs), convey.ShouldEqual, 2)

			// Check first doc
			convey.So(docs[0].ID, convey.ShouldEqual, "1")
			convey.So(docs[0].Content, convey.ShouldEqual, "doc1")
			convey.So(docs[0].Score(), convey.ShouldAlmostEqual, 0.9, 0.0001)
			_, ok := docs[0].MetaData["score"]
			convey.So(ok, convey.ShouldBeFalse)
			convey.So(docs[0].MetaData["key"], convey.ShouldEqual, "val1")

			// Check second doc
			convey.So(docs[1].ID, convey.ShouldEqual, "2")
			convey.So(docs[1].Content, convey.ShouldEqual, "doc2")
			convey.So(docs[1].Score(), convey.ShouldAlmostEqual, 0.8, 0.0001)
			_, ok = docs[1].MetaData["score"]
			convey.So(ok, convey.ShouldBeFalse)
			convey.So(docs[1].MetaData["key"], convey.ShouldEqual, "val2")
		})

		convey.Convey("convert results without scores (e.g. Query)", func() {
			ids := []string{"3"}
			contents := []string{"doc3"}
			metas := [][]byte{[]byte(`{}`)}

			resultSet := createMockQueryResult(ids, contents, metas)
			docs, err := converter(ctx, resultSet)

			convey.So(err, convey.ShouldBeNil)
			convey.So(len(docs), convey.ShouldEqual, 1)
			convey.So(docs[0].ID, convey.ShouldEqual, "3")
			_, hasScore := docs[0].MetaData["score"]
			convey.So(hasScore, convey.ShouldBeFalse)
		})
	})
}

// Helper to create a mock ResultSet using real column implementations
func createMockResultSet(ids []string, contents []string, scores []float32, metadatas [][]byte) milvusclient.ResultSet {
	count := len(ids)
	if len(contents) != count {
		count = 0
	}

	// Create columns using official SDK constructors
	idCol := column.NewColumnVarChar("id", ids)
	contentCol := column.NewColumnVarChar("content", contents)
	metaCol := column.NewColumnJSONBytes("metadata", metadatas)

	// Cast to column.Column interface
	fields := []column.Column{idCol, contentCol, metaCol}

	return milvusclient.ResultSet{
		ResultCount: count,
		IDs:         idCol,
		Scores:      scores,
		Fields:      fields,
	}
}

// For Query results, which are just ResultSet in v2
func createMockQueryResult(ids []string, contents []string, metadatas [][]byte) milvusclient.ResultSet {
	return createMockResultSet(ids, contents, nil, metadatas)
}

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

// mockSearchMode implements SearchMode for testing (avoids import cycle)
type mockSearchMode struct{}

func (m *mockSearchMode) BuildSearchOption(ctx context.Context, config *RetrieverConfig, queryVector []float32, opts ...retriever.Option) (milvusclient.SearchOption, error) {
	// Return nil since we'll mock the Search call anyway
	return nil, nil
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

		convey.Convey("test optional embedding with QuerySearchMode", func() {
			mockQSMode := &mockQuerySearchMode{}
			config := &RetrieverConfig{
				ClientConfig: &milvusclient.ClientConfig{Address: "localhost:19530"},
				Collection:   "test_collection",
				Embedding:    nil, // Valid for QuerySearchMode
				SearchMode:   mockQSMode,
			}
			err := config.validate()
			convey.So(err, convey.ShouldBeNil)
		})

		convey.Convey("test missing embedding for vector search", func() {
			config := &RetrieverConfig{
				ClientConfig: &milvusclient.ClientConfig{Address: "localhost:19530"},
				Collection:   "test_collection",
				Embedding:    nil, // Invalid for mockSearchMode (Standard)
				SearchMode:   mockSM,
			}
			err := config.validate()
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "embedding not provided")
			convey.So(err.Error(), convey.ShouldContainSubstring, "QuerySearchMode")
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

		convey.Convey("test missing collection defaults", func() {
			config := &RetrieverConfig{
				ClientConfig: &milvusclient.ClientConfig{Address: "localhost:19530"},
				Collection:   "",
				Embedding:    mockEmb,
				SearchMode:   mockSM,
			}
			err := config.validate()
			convey.So(err, convey.ShouldBeNil)
			convey.So(config.Collection, convey.ShouldEqual, defaultCollection)
		})

		convey.Convey("test valid config with defaults", func() {
			config := &RetrieverConfig{
				ClientConfig: &milvusclient.ClientConfig{Address: "localhost:19530"},
				Collection:   "test_collection",
				Embedding:    mockEmb,
				SearchMode:   mockSM,
			}
			err := config.validate()
			convey.So(err, convey.ShouldBeNil)
			// Check defaults are set
			convey.So(config.VectorField, convey.ShouldEqual, defaultVectorField)
			convey.So(config.TopK, convey.ShouldEqual, defaultTopK)
			convey.So(config.DocumentConverter, convey.ShouldNotBeNil)
		})

		convey.Convey("test with custom values preserved", func() {
			config := &RetrieverConfig{
				ClientConfig: &milvusclient.ClientConfig{Address: "localhost:19530"},
				Collection:   "my_collection",
				Embedding:    mockEmb,
				VectorField:  "custom_vector",
				TopK:         50,
				Partitions:   []string{"p1", "p2"},
				SearchMode:   mockSM,
			}
			err := config.validate()
			convey.So(err, convey.ShouldBeNil)
			convey.So(config.Collection, convey.ShouldEqual, "my_collection")
			convey.So(config.VectorField, convey.ShouldEqual, "custom_vector")
			convey.So(config.TopK, convey.ShouldEqual, 50)
		})

		convey.Convey("test with custom output fields", func() {
			config := &RetrieverConfig{
				ClientConfig: &milvusclient.ClientConfig{Address: "localhost:19530"},
				Collection:   "test_collection",
				Embedding:    mockEmb,
				OutputFields: []string{"field1", "field2"},
				SearchMode:   mockSM,
			}
			err := config.validate()
			convey.So(err, convey.ShouldBeNil)
			convey.So(config.OutputFields, convey.ShouldResemble, []string{"field1", "field2"})
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

func TestRetriever_applyScoreThreshold(t *testing.T) {
	convey.Convey("test Retriever.applyScoreThreshold", t, func() {
		r := &Retriever{
			config: &RetrieverConfig{},
		}

		docs := []*schema.Document{
			{ID: "doc1", MetaData: map[string]interface{}{"score": float64(0.9)}},
			{ID: "doc2", MetaData: map[string]interface{}{"score": float64(0.7)}},
			{ID: "doc3", MetaData: map[string]interface{}{"score": float64(0.5)}},
			{ID: "doc4", MetaData: map[string]interface{}{"score": float64(0.3)}},
		}

		convey.Convey("test nil threshold returns all docs", func() {
			result := r.applyScoreThreshold(docs, nil)
			convey.So(len(result), convey.ShouldEqual, 4)
		})

		convey.Convey("test threshold filters docs", func() {
			threshold := 0.6
			result := r.applyScoreThreshold(docs, &threshold)
			convey.So(len(result), convey.ShouldEqual, 2)
			convey.So(result[0].ID, convey.ShouldEqual, "doc1")
			convey.So(result[1].ID, convey.ShouldEqual, "doc2")
		})

		convey.Convey("test high threshold filters all docs", func() {
			threshold := 0.95
			result := r.applyScoreThreshold(docs, &threshold)
			convey.So(len(result), convey.ShouldEqual, 0)
		})

		convey.Convey("test low threshold keeps all docs", func() {
			threshold := 0.1
			result := r.applyScoreThreshold(docs, &threshold)
			convey.So(len(result), convey.ShouldEqual, 4)
		})

		convey.Convey("test with missing score in metadata", func() {
			docsNoScore := []*schema.Document{
				{ID: "doc1", MetaData: map[string]interface{}{}},
				{ID: "doc2", MetaData: map[string]interface{}{"score": float64(0.7)}},
			}
			threshold := 0.5
			result := r.applyScoreThreshold(docsNoScore, &threshold)
			// Only doc2 has a score and passes threshold
			convey.So(len(result), convey.ShouldEqual, 1)
		})
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

		PatchConvey("test retrieve with embedding error", func() {
			r.config.Embedding = &mockEmbedding{err: fmt.Errorf("embedding error")}
			docs, err := r.Retrieve(ctx, "test query")
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "embed")
			convey.So(docs, convey.ShouldBeNil)
		})

		PatchConvey("test retrieve with search error", func() {
			r.config.Embedding = mockEmb
			// Mock Search to return an error
			Mock(GetMethod(mockClient, "Search")).Return(nil, fmt.Errorf("search error")).Build()

			docs, err := r.Retrieve(ctx, "test query")
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(docs, convey.ShouldBeNil)
		})

		PatchConvey("test retrieve with empty results", func() {
			r.config.Embedding = mockEmb
			// Mock Search to return empty results
			Mock(GetMethod(mockClient, "Search")).Return([]milvusclient.ResultSet{}, nil).Build()

			docs, err := r.Retrieve(ctx, "test query")
			// Empty result might return empty docs or nil
			if err != nil {
				convey.So(docs, convey.ShouldBeNil)
			} else {
				convey.So(docs, convey.ShouldNotBeNil)
			}
		})
	})
}

func TestRetriever_RetrieveQuery(t *testing.T) {
	PatchConvey("test Retriever.Retrieve with QuerySearchMode", t, func() {
		ctx := context.Background()
		mockClient := &milvusclient.Client{}
		mockQSMode := &mockQuerySearchMode{}

		r := &Retriever{
			client: mockClient,
			config: &RetrieverConfig{
				Collection:        "test_collection",
				OutputFields:      []string{"id", "content"},
				SearchMode:        mockQSMode,
				DocumentConverter: defaultDocumentConverter(),
			},
		}

		PatchConvey("test query success", func() {
			Mock(GetMethod(mockClient, "Query")).Return(createMockQueryResult(
				[]string{"1"}, []string{"doc1"}, [][]byte{[]byte(`{}`)}), nil).Build()

			docs, err := r.Retrieve(ctx, "id > 0")
			convey.So(err, convey.ShouldBeNil)
			convey.So(len(docs), convey.ShouldEqual, 1)
			convey.So(docs[0].ID, convey.ShouldEqual, "1")
		})

		PatchConvey("test build query option error", func() {
			mockQSMode.err = fmt.Errorf("build error")
			docs, err := r.Retrieve(ctx, "id > 0")
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "build query option")
			convey.So(docs, convey.ShouldBeNil)
		})

		PatchConvey("test client query error", func() {
			Mock(GetMethod(mockClient, "Query")).Return(milvusclient.ResultSet{}, fmt.Errorf("query fail")).Build()
			docs, err := r.Retrieve(ctx, "id > 0")
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "execute query")
			convey.So(docs, convey.ShouldBeNil)
		})
	})
}

// mockQuerySearchMode implements QuerySearchMode for testing
type mockQuerySearchMode struct {
	err error
}

func (m *mockQuerySearchMode) BuildQueryOption(ctx context.Context, config *RetrieverConfig, query string, opts ...retriever.Option) (milvusclient.QueryOption, error) {
	if m.err != nil {
		return nil, m.err
	}
	return milvusclient.NewQueryOption(config.Collection), nil
}

func (m *mockQuerySearchMode) BuildSearchOption(ctx context.Context, config *RetrieverConfig, queryVector []float32, opts ...retriever.Option) (milvusclient.SearchOption, error) {
	return nil, nil
}

// mockHybridSearchMode implements HybridSearchMode for testing
type mockHybridSearchMode struct {
	err error
}

func (m *mockHybridSearchMode) BuildHybridSearchOption(ctx context.Context, config *RetrieverConfig, queryVector []float32, querySparseVector map[int]float64, opts ...retriever.Option) (milvusclient.HybridSearchOption, error) {
	if m.err != nil {
		return nil, m.err
	}
	// NewHybridSearchOption needs collection, limit, and a request.
	// Since we mock the client call, we just need a valid option object.
	req := milvusclient.NewAnnRequest("vector", 10, entity.FloatVector(make([]float32, 128)))
	req.WithSearchParam("metric_type", "L2")
	return milvusclient.NewHybridSearchOption(config.Collection, 3, req), nil
}

func (m *mockHybridSearchMode) BuildSearchOption(ctx context.Context, config *RetrieverConfig, queryVector []float32, opts ...retriever.Option) (milvusclient.SearchOption, error) {
	return nil, nil
}

func TestRetriever_RetrieveHybrid(t *testing.T) {
	PatchConvey("test Retriever.Retrieve with HybridSearchMode", t, func() {
		ctx := context.Background()
		mockClient := &milvusclient.Client{}
		mockEmb := &mockEmbedding{dims: 128}
		mockHSMode := &mockHybridSearchMode{}

		r := &Retriever{
			client: mockClient,
			config: &RetrieverConfig{
				Collection:        "test_collection",
				Embedding:         mockEmb,
				SearchMode:        mockHSMode,
				DocumentConverter: defaultDocumentConverter(),
			},
		}

		PatchConvey("test hybrid search success", func() {
			// Mock HybridSearch to return []ResultSet
			results := []milvusclient.ResultSet{
				createMockResultSet([]string{"1"}, []string{"doc1"}, []float32{0.9}, [][]byte{[]byte(`{}`)}),
			}
			Mock(GetMethod(mockClient, "HybridSearch")).Return(results, nil).Build()

			docs, err := r.Retrieve(ctx, "test query")
			convey.So(err, convey.ShouldBeNil)
			convey.So(len(docs), convey.ShouldEqual, 1)
			convey.So(docs[0].ID, convey.ShouldEqual, "1")
		})

		PatchConvey("test embedding error", func() {
			r.config.Embedding = &mockEmbedding{err: fmt.Errorf("embed error")}
			docs, err := r.Retrieve(ctx, "test query")
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(docs, convey.ShouldBeNil)
		})

		PatchConvey("test build option error", func() {
			mockHSMode.err = fmt.Errorf("build error")
			docs, err := r.Retrieve(ctx, "test query")
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "build hybrid search option")
			convey.So(docs, convey.ShouldBeNil)
		})

		PatchConvey("test client hybrid search error", func() {
			Mock(GetMethod(mockClient, "HybridSearch")).Return(nil, fmt.Errorf("hybrid fail")).Build()
			docs, err := r.Retrieve(ctx, "test query")
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "hybrid search failed")
			convey.So(docs, convey.ShouldBeNil)
		})
		PatchConvey("test hybrid search with sparse embedding", func() {
			mockSparse := &mockSparseEmbedder{
				result: []map[int]float64{{1: 0.5, 2: 0.8}},
			}
			r.config.SparseEmbedding = mockSparse

			// Mock HybridSearch to return results
			Mock(GetMethod(mockClient, "HybridSearch")).Return([]milvusclient.ResultSet{
				createMockResultSet([]string{"1"}, []string{"doc1"}, []float32{0.9}, [][]byte{[]byte(`{}`)}),
			}, nil).Build()

			docs, err := r.Retrieve(ctx, "test query")
			convey.So(err, convey.ShouldBeNil)
			convey.So(len(docs), convey.ShouldEqual, 1)
			convey.So(docs[0].ID, convey.ShouldEqual, "1")
		})
	})
}

// mockSparseEmbedder implements SparseEmbedder for testing
type mockSparseEmbedder struct {
	result []map[int]float64
	err    error
}

func (m *mockSparseEmbedder) EmbedStrings(ctx context.Context, texts []string) ([]map[int]float64, error) {
	if m.err != nil {
		return nil, m.err
	}
	if len(texts) != 1 {
		return nil, fmt.Errorf("expected 1 text, got %d", len(texts))
	}
	return m.result, nil
}

// mockIteratorSearchMode implements IteratorSearchMode for testing
type mockIteratorSearchMode struct {
	err error
}

func (m *mockIteratorSearchMode) BuildSearchIteratorOption(ctx context.Context, config *RetrieverConfig, queryVector []float32, opts ...retriever.Option) (milvusclient.SearchIteratorOption, error) {
	if m.err != nil {
		return nil, m.err
	}
	// NewSearchIteratorOption takes (collectionName, vector)
	// assuming vector is entity.Vector
	vector := entity.FloatVector(make([]float32, 128))
	return milvusclient.NewSearchIteratorOption(config.Collection, vector).WithBatchSize(3), nil
}

func (m *mockIteratorSearchMode) BuildSearchOption(ctx context.Context, config *RetrieverConfig, queryVector []float32, opts ...retriever.Option) (milvusclient.SearchOption, error) {
	return nil, nil
}

// mockSearchIterator implements milvusclient.SearchIterator
type mockSearchIterator struct {
	err     error
	results []milvusclient.ResultSet
	idx     int
}

func (m *mockSearchIterator) Next(ctx context.Context) (milvusclient.ResultSet, error) {
	if m.err != nil {
		return milvusclient.ResultSet{}, m.err
	}
	if m.idx >= len(m.results) {
		return milvusclient.ResultSet{}, nil // End of results (or simulate EOF)
	}
	res := m.results[m.idx]
	m.idx++
	return res, nil
}

func (m *mockSearchIterator) Close() error {
	return nil
}

func TestRetriever_RetrieveIterator(t *testing.T) {
	PatchConvey("test Retriever.Retrieve with IteratorSearchMode", t, func() {
		ctx := context.Background()
		mockClient := &milvusclient.Client{}
		mockEmb := &mockEmbedding{dims: 128}
		mockISMode := &mockIteratorSearchMode{}

		r := &Retriever{
			client: mockClient,
			config: &RetrieverConfig{
				Collection:        "test_collection",
				Embedding:         mockEmb,
				SearchMode:        mockISMode,
				DocumentConverter: defaultDocumentConverter(),
			},
		}

		PatchConvey("test iterator success", func() {
			res1 := createMockResultSet([]string{"1"}, []string{"doc1"}, []float32{0.9}, nil)
			res2 := createMockResultSet([]string{"2"}, []string{"doc2"}, []float32{0.8}, nil)
			// Mock iterator
			mockIter := &mockSearchIterator{
				results: []milvusclient.ResultSet{res1, res2},
			}

			Mock(GetMethod(mockClient, "SearchIterator")).Return(mockIter, nil).Build()

			docs, err := r.Retrieve(ctx, "test query")
			convey.So(err, convey.ShouldBeNil)
			convey.So(len(docs), convey.ShouldEqual, 2)
			convey.So(docs[0].ID, convey.ShouldEqual, "1")
			convey.So(docs[1].ID, convey.ShouldEqual, "2")
		})

		PatchConvey("test client iterator creation error", func() {
			Mock(GetMethod(mockClient, "SearchIterator")).Return(nil, fmt.Errorf("create iter fail")).Build()
			docs, err := r.Retrieve(ctx, "test query")
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "create search iterator")
			convey.So(docs, convey.ShouldBeNil)
		})
	})
}
func TestRetriever_embedQuery(t *testing.T) {
	PatchConvey("test Retriever.embedQuery", t, func() {
		ctx := context.Background()
		r := &Retriever{
			config: &RetrieverConfig{},
		}

		PatchConvey("test embedding success returns float32 vector", func() {
			mockEmb := &mockEmbedding{dims: 128}
			vector, err := r.embedQuery(ctx, mockEmb, "test query")
			convey.So(err, convey.ShouldBeNil)
			convey.So(len(vector), convey.ShouldEqual, 128)
			// First element should be 0.1 converted to float32
			convey.So(vector[0], convey.ShouldEqual, float32(0.1))
		})

		PatchConvey("test embedding error", func() {
			mockEmb := &mockEmbedding{err: fmt.Errorf("embedding failed")}
			vector, err := r.embedQuery(ctx, mockEmb, "test query")
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(vector, convey.ShouldBeNil)
		})

		PatchConvey("test embedding empty result", func() {
			mockEmb := &mockEmbedding{dims: 0}
			// Even with dims=0, the mock returns 128 (default)
			vector, err := r.embedQuery(ctx, mockEmb, "test query")
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
			convey.So(docs[0].MetaData["score"], convey.ShouldAlmostEqual, 0.9, 0.0001)
			convey.So(docs[0].MetaData["key"], convey.ShouldEqual, "val1")

			// Check second doc
			convey.So(docs[1].ID, convey.ShouldEqual, "2")
			convey.So(docs[1].Content, convey.ShouldEqual, "doc2")
			convey.So(docs[1].MetaData["score"], convey.ShouldAlmostEqual, 0.8, 0.0001)
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

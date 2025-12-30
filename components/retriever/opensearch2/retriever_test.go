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

package opensearch2

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/bytedance/mockey"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	opensearch "github.com/opensearch-project/opensearch-go/v2"
	"github.com/smartystreets/goconvey/convey"
)

func TestNewRetriever(t *testing.T) {
	PatchConvey("test NewRetriever", t, func() {
		ctx := context.Background()
		client := &opensearch.Client{}

		PatchConvey("test missing search mode", func() {
			r, err := NewRetriever(ctx, &RetrieverConfig{
				Client: client,
			})
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "search mode not provided")
			convey.So(r, convey.ShouldBeNil)
		})

		PatchConvey("test missing client", func() {
			r, err := NewRetriever(ctx, &RetrieverConfig{
				SearchMode: &mockSearchMode{},
			})
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "client not provided")
			convey.So(r, convey.ShouldBeNil)
		})

		PatchConvey("test success with defaults", func() {
			r, err := NewRetriever(ctx, &RetrieverConfig{
				Client:     client,
				SearchMode: &mockSearchMode{},
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(r, convey.ShouldNotBeNil)
			convey.So(r.config.TopK, convey.ShouldEqual, defaultTopK)
			convey.So(r.config.ResultParser, convey.ShouldNotBeNil)
		})

		PatchConvey("test success with custom values", func() {
			customParser := func(ctx context.Context, hit map[string]any) (*schema.Document, error) {
				return &schema.Document{ID: "custom"}, nil
			}
			r, err := NewRetriever(ctx, &RetrieverConfig{
				Client:       client,
				SearchMode:   &mockSearchMode{},
				TopK:         20,
				ResultParser: customParser,
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(r, convey.ShouldNotBeNil)
			convey.So(r.config.TopK, convey.ShouldEqual, 20)
		})
	})
}

func TestWithFilters(t *testing.T) {
	PatchConvey("test WithFilters", t, func() {
		filters := []any{
			map[string]any{"term": map[string]string{"status": "active"}},
		}
		opt := WithFilters(filters)
		convey.So(opt, convey.ShouldNotBeNil)
		convey.So(filters, convey.ShouldHaveLength, 1)
	})
}

func TestRetrieve(t *testing.T) {
	PatchConvey("test Retrieve", t, func() {
		ctx := context.Background()

		PatchConvey("test BuildRequest error", func() {
			mockErr := fmt.Errorf("build request error")
			// Create a mock server for OpenSearch
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			client, err := opensearch.NewClient(opensearch.Config{
				Addresses: []string{server.URL},
			})
			convey.So(err, convey.ShouldBeNil)

			r := &Retriever{
				client: client,
				config: &RetrieverConfig{
					Index:      "test_index",
					TopK:       10,
					SearchMode: &mockSearchMode{err: mockErr},
					ResultParser: func(ctx context.Context, hit map[string]any) (*schema.Document, error) {
						return nil, nil
					},
				},
			}
			docs, err := r.Retrieve(ctx, "test_query")
			convey.So(err, convey.ShouldBeError, mockErr)
			convey.So(docs, convey.ShouldBeNil)
		})

		PatchConvey("test Search error response", func() {
			// Create a mock server that returns an error response
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"error": "bad request"}`))
			}))
			defer server.Close()

			client, err := opensearch.NewClient(opensearch.Config{
				Addresses: []string{server.URL},
			})
			convey.So(err, convey.ShouldBeNil)

			r := &Retriever{
				client: client,
				config: &RetrieverConfig{
					Index:        "test_index",
					TopK:         10,
					SearchMode:   &mockSearchMode{},
					ResultParser: defaultResultParser,
				},
			}

			docs, err := r.Retrieve(ctx, "test_query")
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "search failed")
			convey.So(docs, convey.ShouldBeNil)
		})

		PatchConvey("test success", func() {
			searchResp := `{
				"hits": {
					"hits": [
						{
							"_id": "doc1",
							"_score": 0.95,
							"_source": {
								"content": "test content 1"
							}
						},
						{
							"_id": "doc2",
							"_score": 0.85,
							"_source": {
								"content": "test content 2"
							}
						}
					]
				}
			}`
			// Create a mock server that returns success
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(searchResp))
			}))
			defer server.Close()

			client, err := opensearch.NewClient(opensearch.Config{
				Addresses: []string{server.URL},
			})
			convey.So(err, convey.ShouldBeNil)

			r := &Retriever{
				client: client,
				config: &RetrieverConfig{
					Index:        "test_index",
					TopK:         10,
					SearchMode:   &mockSearchMode{},
					ResultParser: defaultResultParser,
				},
			}

			docs, err := r.Retrieve(ctx, "test_query")
			convey.So(err, convey.ShouldBeNil)
			convey.So(len(docs), convey.ShouldEqual, 2)
			convey.So(docs[0].ID, convey.ShouldEqual, "doc1")
			convey.So(docs[0].Content, convey.ShouldEqual, "test content 1")
			convey.So(docs[1].ID, convey.ShouldEqual, "doc2")
			convey.So(docs[1].Content, convey.ShouldEqual, "test content 2")
		})

		PatchConvey("test with score threshold", func() {
			scoreThreshold := 0.5
			searchResp := `{"hits": {"hits": []}}`
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(searchResp))
			}))
			defer server.Close()

			client, err := opensearch.NewClient(opensearch.Config{
				Addresses: []string{server.URL},
			})
			convey.So(err, convey.ShouldBeNil)

			r := &Retriever{
				client: client,
				config: &RetrieverConfig{
					Index:          "test_index",
					TopK:           10,
					ScoreThreshold: &scoreThreshold,
					SearchMode:     &mockSearchMode{},
					ResultParser:   defaultResultParser,
				},
			}

			docs, err := r.Retrieve(ctx, "test_query")
			convey.So(err, convey.ShouldBeNil)
			convey.So(len(docs), convey.ShouldEqual, 0)
		})
	})
}

func TestParseSearchResult(t *testing.T) {
	PatchConvey("test parseSearchResult", t, func() {
		ctx := context.Background()
		r := &Retriever{
			config: &RetrieverConfig{
				ResultParser: defaultResultParser,
			},
		}

		PatchConvey("test invalid json", func() {
			body := bytes.NewReader([]byte(`invalid json`))
			docs, err := r.parseSearchResult(ctx, body)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "decode response failed")
			convey.So(docs, convey.ShouldBeNil)
		})

		PatchConvey("test missing hits field", func() {
			body := bytes.NewReader([]byte(`{"took": 10}`))
			docs, err := r.parseSearchResult(ctx, body)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "hits field missing")
			convey.So(docs, convey.ShouldBeNil)
		})

		PatchConvey("test empty hits array", func() {
			body := bytes.NewReader([]byte(`{"hits": {"hits": []}}`))
			docs, err := r.parseSearchResult(ctx, body)
			convey.So(err, convey.ShouldBeNil)
			convey.So(len(docs), convey.ShouldEqual, 0)
		})

		PatchConvey("test invalid hits format returns empty", func() {
			body := bytes.NewReader([]byte(`{"hits": {"hits": "invalid"}}`))
			docs, err := r.parseSearchResult(ctx, body)
			convey.So(err, convey.ShouldBeNil)
			convey.So(len(docs), convey.ShouldEqual, 0)
		})

		PatchConvey("test success with multiple docs", func() {
			jsonResp := `{
				"hits": {
					"hits": [
						{"_id": "1", "_score": 0.9, "_source": {"content": "content 1"}},
						{"_id": "2", "_score": 0.8, "_source": {"content": "content 2"}}
					]
				}
			}`
			body := bytes.NewReader([]byte(jsonResp))
			docs, err := r.parseSearchResult(ctx, body)
			convey.So(err, convey.ShouldBeNil)
			convey.So(len(docs), convey.ShouldEqual, 2)
			convey.So(docs[0].ID, convey.ShouldEqual, "1")
			convey.So(docs[1].ID, convey.ShouldEqual, "2")
		})

		PatchConvey("test ResultParser error", func() {
			mockErr := fmt.Errorf("parser error")
			r.config.ResultParser = func(ctx context.Context, hit map[string]any) (*schema.Document, error) {
				return nil, mockErr
			}
			jsonResp := `{"hits": {"hits": [{"_id": "1"}]}}`
			body := bytes.NewReader([]byte(jsonResp))
			docs, err := r.parseSearchResult(ctx, body)
			convey.So(err, convey.ShouldBeError, mockErr)
			convey.So(docs, convey.ShouldBeNil)
		})
	})
}

func TestDefaultResultParser(t *testing.T) {
	PatchConvey("test defaultResultParser", t, func() {
		ctx := context.Background()

		PatchConvey("test missing _id", func() {
			hit := map[string]any{}
			doc, err := defaultResultParser(ctx, hit)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "field '_id' not found in hit")
			convey.So(doc, convey.ShouldBeNil)
		})

		PatchConvey("test invalid _id type", func() {
			hit := map[string]any{
				"_id": 123,
			}
			doc, err := defaultResultParser(ctx, hit)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "field '_id' is not a string")
			convey.So(doc, convey.ShouldBeNil)
		})

		PatchConvey("test missing _source", func() {
			hit := map[string]any{
				"_id":    "doc1",
				"_score": 0.95,
			}
			doc, err := defaultResultParser(ctx, hit)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "field '_source' not found in document doc1")
			convey.So(doc, convey.ShouldBeNil)
		})

		PatchConvey("test missing content in _source", func() {
			hit := map[string]any{
				"_id": "doc1",
				"_source": map[string]any{
					"title": "foo",
				},
			}
			doc, err := defaultResultParser(ctx, hit)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "field 'content' not found in document doc1")
			convey.So(doc, convey.ShouldBeNil)
		})

		PatchConvey("test invalid content type in _source", func() {
			hit := map[string]any{
				"_id": "doc1",
				"_source": map[string]any{
					"content": 12345,
				},
			}
			doc, err := defaultResultParser(ctx, hit)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "field 'content' in document doc1 is not a string")
			convey.So(doc, convey.ShouldBeNil)
		})

		PatchConvey("test success with source content", func() {
			hit := map[string]any{
				"_id":    "doc2",
				"_score": 0.85,
				"_source": map[string]any{
					"content": "test content",
					"title":   "test title",
				},
			}
			doc, err := defaultResultParser(ctx, hit)
			convey.So(err, convey.ShouldBeNil)
			convey.So(doc.ID, convey.ShouldEqual, "doc2")
			convey.So(doc.Content, convey.ShouldEqual, "test content")
			convey.So(doc.MetaData["title"], convey.ShouldEqual, "test title")
			convey.So(doc.MetaData["score"], convey.ShouldEqual, 0.85)
			// content should not be in metadata
			_, hasContent := doc.MetaData["content"]
			convey.So(hasContent, convey.ShouldBeFalse)
		})
	})
}

func TestRetrieverGetType(t *testing.T) {
	PatchConvey("test GetType", t, func() {
		r := &Retriever{}
		convey.So(r.GetType(), convey.ShouldEqual, typ)
	})
}

func TestRetrieverIsCallbacksEnabled(t *testing.T) {
	PatchConvey("test IsCallbacksEnabled", t, func() {
		r := &Retriever{}
		convey.So(r.IsCallbacksEnabled(), convey.ShouldBeTrue)
	})
}

func TestGetTypeFunc(t *testing.T) {
	PatchConvey("test GetType function", t, func() {
		convey.So(GetType(), convey.ShouldEqual, typ)
	})
}

// mockSearchMode implements SearchMode interface for testing
type mockSearchMode struct {
	err error
}

func (m *mockSearchMode) BuildRequest(ctx context.Context, conf *RetrieverConfig, query string, opts ...retriever.Option) (map[string]any, error) {
	if m.err != nil {
		return nil, m.err
	}
	return map[string]any{
		"query": map[string]any{
			"match": map[string]any{
				"content": query,
			},
		},
	}, nil
}

// mockEmbedder implements Embedder interface for testing
type mockEmbedder struct{}

func (m *mockEmbedder) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	result := make([][]float64, len(texts))
	for i := range result {
		result[i] = []float64{0.1, 0.2, 0.3}
	}
	return result, nil
}

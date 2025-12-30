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

package es7

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/retriever"
	elasticsearch "github.com/elastic/go-elasticsearch/v7"
	. "github.com/smartystreets/goconvey/convey"
)

type mockTransport struct {
	Response *http.Response
	Err      error
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.Response, nil
}

type mockSearchMode struct {
	buildRequestFn func(ctx context.Context, conf *RetrieverConfig, query string, opts ...retriever.Option) (map[string]any, error)
}

func (m *mockSearchMode) BuildRequest(ctx context.Context, conf *RetrieverConfig, query string, opts ...retriever.Option) (map[string]any, error) {
	if m.buildRequestFn != nil {
		return m.buildRequestFn(ctx, conf, query, opts...)
	}
	return map[string]any{
		"query": map[string]any{
			"match_all": map[string]any{},
		},
	}, nil
}

func TestNewRetriever(t *testing.T) {
	Convey("TestNewRetriever", t, func() {
		ctx := context.Background()

		Convey("search mode not provided", func() {
			_, err := NewRetriever(ctx, &RetrieverConfig{})
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "search mode not provided")
		})

		Convey("client not provided", func() {
			_, err := NewRetriever(ctx, &RetrieverConfig{
				SearchMode: &mockSearchMode{},
			})
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "es client not provided")
		})

		Convey("success with default topK", func() {
			client, _ := elasticsearch.NewClient(elasticsearch.Config{})
			retriever, err := NewRetriever(ctx, &RetrieverConfig{
				Client:     client,
				SearchMode: &mockSearchMode{},
			})
			So(err, ShouldBeNil)
			So(retriever, ShouldNotBeNil)
			So(retriever.config.TopK, ShouldEqual, defaultTopK)
		})

		Convey("success with custom topK", func() {
			client, _ := elasticsearch.NewClient(elasticsearch.Config{})
			retriever, err := NewRetriever(ctx, &RetrieverConfig{
				Client:     client,
				TopK:       20,
				SearchMode: &mockSearchMode{},
			})
			So(err, ShouldBeNil)
			So(retriever, ShouldNotBeNil)
			So(retriever.config.TopK, ShouldEqual, 20)
		})

		Convey("success with default result parser", func() {
			client, _ := elasticsearch.NewClient(elasticsearch.Config{})
			retriever, err := NewRetriever(ctx, &RetrieverConfig{
				Client:     client,
				SearchMode: &mockSearchMode{},
			})
			So(err, ShouldBeNil)
			So(retriever, ShouldNotBeNil)
			So(retriever.config.ResultParser, ShouldNotBeNil)
		})
	})
}

func TestRetriever_GetType(t *testing.T) {
	Convey("TestRetriever_GetType", t, func() {
		client, _ := elasticsearch.NewClient(elasticsearch.Config{})
		retriever, _ := NewRetriever(context.Background(), &RetrieverConfig{
			Client:     client,
			SearchMode: &mockSearchMode{},
		})
		So(retriever.GetType(), ShouldEqual, "ElasticSearch7")
	})
}

func TestRetriever_IsCallbacksEnabled(t *testing.T) {
	Convey("TestRetriever_IsCallbacksEnabled", t, func() {
		client, _ := elasticsearch.NewClient(elasticsearch.Config{})
		retriever, _ := NewRetriever(context.Background(), &RetrieverConfig{
			Client:     client,
			SearchMode: &mockSearchMode{},
		})
		So(retriever.IsCallbacksEnabled(), ShouldBeTrue)
	})
}

func TestDefaultResultParser(t *testing.T) {
	Convey("TestDefaultResultParser", t, func() {
		ctx := context.Background()

		Convey("parse hit with full source", func() {
			hit := map[string]any{
				"_id":    "doc1",
				"_score": 0.95,
				"_source": map[string]any{
					"content":  "test content",
					"metadata": "some metadata",
				},
			}
			doc, err := defaultResultParser(ctx, hit)
			So(err, ShouldBeNil)
			So(doc.ID, ShouldEqual, "doc1")
			So(doc.Content, ShouldEqual, "test content")
			So(doc.MetaData["score"], ShouldEqual, 0.95)
			So(doc.MetaData["metadata"], ShouldEqual, "some metadata")
			So(doc.MetaData["content"], ShouldBeNil) // content should be excluded from metadata
		})

		Convey("missing _id", func() {
			hit := map[string]any{
				"_score": 0.8,
			}
			doc, err := defaultResultParser(ctx, hit)
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "field '_id' not found")
			So(doc, ShouldBeNil)
		})

		Convey("missing _source", func() {
			hit := map[string]any{
				"_id":    "doc2",
				"_score": 0.8,
			}
			doc, err := defaultResultParser(ctx, hit)
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "field '_source' not found")
			So(doc, ShouldBeNil)
		})

		Convey("missing content", func() {
			hit := map[string]any{
				"_id":    "doc3",
				"_score": 0.8,
				"_source": map[string]any{
					"other": "val",
				},
			}
			doc, err := defaultResultParser(ctx, hit)
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "field 'content' not found")
			So(doc, ShouldBeNil)
		})

		Convey("invalid content type", func() {
			hit := map[string]any{
				"_id":    "doc4",
				"_score": 0.8,
				"_source": map[string]any{
					"content": 123,
				},
			}
			doc, err := defaultResultParser(ctx, hit)
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "field 'content' in document doc4 is not a string")
			So(doc, ShouldBeNil)
		})
	})
}

func TestGetType(t *testing.T) {
	Convey("TestGetType", t, func() {
		So(GetType(), ShouldEqual, "ElasticSearch7")
	})
}
func TestRetriever_Retrieve(t *testing.T) {
	Convey("TestRetriever_Retrieve", t, func() {
		ctx := context.Background()

		Convey("convert request error", func() {
			client, _ := elasticsearch.NewClient(elasticsearch.Config{})
			retriever, _ := NewRetriever(ctx, &RetrieverConfig{
				Client: client,
				SearchMode: &mockSearchMode{
					buildRequestFn: func(ctx context.Context, conf *RetrieverConfig, query string, opts ...retriever.Option) (map[string]any, error) {
						return nil, fmt.Errorf("build request error")
					},
				},
			})
			_, err := retriever.Retrieve(ctx, "query")
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "build request error")
		})

		Convey("search error", func() {
			mockT := &mockTransport{
				Err: fmt.Errorf("transport error"),
			}
			client, _ := elasticsearch.NewClient(elasticsearch.Config{
				Transport: mockT,
			})
			retriever, _ := NewRetriever(ctx, &RetrieverConfig{
				Client:     client,
				SearchMode: &mockSearchMode{},
			})
			_, err := retriever.Retrieve(ctx, "query")
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "transport error")
		})

		Convey("search response error", func() {
			mockT := &mockTransport{
				Response: &http.Response{
					StatusCode: 500,
					Status:     "500 Internal Server Error",
					Body:       io.NopCloser(strings.NewReader(`{"error": "internal server error"}`)),
					Header:     http.Header{"X-Elastic-Product": []string{"Elasticsearch"}},
				},
			}
			client, _ := elasticsearch.NewClient(elasticsearch.Config{
				Transport: mockT,
			})
			retriever, _ := NewRetriever(ctx, &RetrieverConfig{
				Client:     client,
				SearchMode: &mockSearchMode{},
			})
			_, err := retriever.Retrieve(ctx, "query")
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "cannot retrieve information")
		})

		Convey("parse response error", func() {
			mockT := &mockTransport{
				Response: &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(strings.NewReader(`invalid json`)),
					Header:     http.Header{"X-Elastic-Product": []string{"Elasticsearch"}},
				},
			}
			client, _ := elasticsearch.NewClient(elasticsearch.Config{
				Transport: mockT,
			})
			retriever, _ := NewRetriever(ctx, &RetrieverConfig{
				Client:     client,
				SearchMode: &mockSearchMode{},
			})
			_, err := retriever.Retrieve(ctx, "query")
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "decode response failed")
		})

		Convey("success", func() {
			respBody := map[string]any{
				"hits": map[string]any{
					"hits": []any{
						map[string]any{
							"_id":    "doc1",
							"_score": 1.0,
							"_source": map[string]any{
								"content": "test content",
							},
						},
					},
				},
			}
			respBytes, _ := json.Marshal(respBody)
			mockT := &mockTransport{
				Response: &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewReader(respBytes)),
					Header:     http.Header{"X-Elastic-Product": []string{"Elasticsearch"}},
				},
			}
			client, _ := elasticsearch.NewClient(elasticsearch.Config{
				Transport: mockT,
			})
			retriever, _ := NewRetriever(ctx, &RetrieverConfig{
				Client:     client,
				SearchMode: &mockSearchMode{},
			})
			docs, err := retriever.Retrieve(ctx, "query")
			So(err, ShouldBeNil)
			So(len(docs), ShouldEqual, 1)
			So(docs[0].ID, ShouldEqual, "doc1")
			So(docs[0].Content, ShouldEqual, "test content")
		})
	})
}

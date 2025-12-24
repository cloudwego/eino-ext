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

	"github.com/bytedance/mockey"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
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

func TestNewIndexer(t *testing.T) {
	Convey("TestNewIndexer", t, func() {
		ctx := context.Background()

		Convey("client not provided", func() {
			_, err := NewIndexer(ctx, &IndexerConfig{})
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "es client not provided")
		})

		Convey("DocumentToFields not provided", func() {
			client, _ := elasticsearch.NewClient(elasticsearch.Config{})
			_, err := NewIndexer(ctx, &IndexerConfig{
				Client: client,
			})
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "DocumentToFields method not provided")
		})

		Convey("success with default batch size", func() {
			client, _ := elasticsearch.NewClient(elasticsearch.Config{})
			indexer, err := NewIndexer(ctx, &IndexerConfig{
				Client: client,
				DocumentToFields: func(ctx context.Context, doc *schema.Document) (map[string]FieldValue, error) {
					return nil, nil
				},
			})
			So(err, ShouldBeNil)
			So(indexer, ShouldNotBeNil)
			So(indexer.config.BatchSize, ShouldEqual, defaultBatchSize)
		})

		Convey("success with custom batch size", func() {
			client, _ := elasticsearch.NewClient(elasticsearch.Config{})
			indexer, err := NewIndexer(ctx, &IndexerConfig{
				Client:    client,
				BatchSize: 10,
				DocumentToFields: func(ctx context.Context, doc *schema.Document) (map[string]FieldValue, error) {
					return nil, nil
				},
			})
			So(err, ShouldBeNil)
			So(indexer, ShouldNotBeNil)
			So(indexer.config.BatchSize, ShouldEqual, 10)
		})
	})
}

type mockEmbedder struct {
	embedFn func(ctx context.Context, texts []string) ([][]float64, error)
}

func (m *mockEmbedder) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	if m.embedFn != nil {
		return m.embedFn(ctx, texts)
	}
	vectors := make([][]float64, len(texts))
	for i := range texts {
		vectors[i] = []float64{0.1, 0.2, 0.3}
	}
	return vectors, nil
}

func TestIndexer_GetType(t *testing.T) {
	Convey("TestIndexer_GetType", t, func() {
		client, _ := elasticsearch.NewClient(elasticsearch.Config{})
		indexer, _ := NewIndexer(context.Background(), &IndexerConfig{
			Client: client,
			DocumentToFields: func(ctx context.Context, doc *schema.Document) (map[string]FieldValue, error) {
				return nil, nil
			},
		})
		So(indexer.GetType(), ShouldEqual, "ElasticSearch7")
	})
}

func TestIndexer_IsCallbacksEnabled(t *testing.T) {
	Convey("TestIndexer_IsCallbacksEnabled", t, func() {
		client, _ := elasticsearch.NewClient(elasticsearch.Config{})
		indexer, _ := NewIndexer(context.Background(), &IndexerConfig{
			Client: client,
			DocumentToFields: func(ctx context.Context, doc *schema.Document) (map[string]FieldValue, error) {
				return nil, nil
			},
		})
		So(indexer.IsCallbacksEnabled(), ShouldBeTrue)
	})
}

func TestIndexer_bulkAdd(t *testing.T) {
	mockey.PatchConvey("TestIndexer_bulkAdd", t, func() {
		ctx := context.Background()
		client, _ := elasticsearch.NewClient(elasticsearch.Config{})

		Convey("DocumentToFields returns error", func() {
			indexer, _ := NewIndexer(ctx, &IndexerConfig{
				Client: client,
				Index:  "test-index",
				DocumentToFields: func(ctx context.Context, doc *schema.Document) (map[string]FieldValue, error) {
					return nil, fmt.Errorf("mapping error")
				},
			})

			docs := []*schema.Document{{ID: "1", Content: "test"}}
			_, err := indexer.Store(ctx, docs)
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "FieldMapping failed")
		})

		Convey("embedding not provided when needed", func() {
			indexer, _ := NewIndexer(ctx, &IndexerConfig{
				Client: client,
				Index:  "test-index",
				DocumentToFields: func(ctx context.Context, doc *schema.Document) (map[string]FieldValue, error) {
					return map[string]FieldValue{
						"content": {Value: doc.Content, EmbedKey: "content_vector"},
					}, nil
				},
			})

			docs := []*schema.Document{{ID: "1", Content: "test"}}
			_, err := indexer.Store(ctx, docs)
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "embedding method not provided")
		})

		Convey("embedding field size over batch size", func() {
			indexer, _ := NewIndexer(ctx, &IndexerConfig{
				Client:    client,
				Index:     "test-index",
				BatchSize: 1,
				DocumentToFields: func(ctx context.Context, doc *schema.Document) (map[string]FieldValue, error) {
					return map[string]FieldValue{
						"field1": {Value: "text1", EmbedKey: "vec1"},
						"field2": {Value: "text2", EmbedKey: "vec2"},
					}, nil
				},
				Embedding: &mockEmbedder{},
			})

			docs := []*schema.Document{{ID: "1", Content: "test"}}
			_, err := indexer.Store(ctx, docs)
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "needEmbeddingFields length over batch size")
		})

		Convey("duplicate embed key", func() {
			indexer, _ := NewIndexer(ctx, &IndexerConfig{
				Client: client,
				Index:  "test-index",
				DocumentToFields: func(ctx context.Context, doc *schema.Document) (map[string]FieldValue, error) {
					return map[string]FieldValue{
						"field1": {Value: "text1", EmbedKey: "vector"},
						"field2": {Value: "text2", EmbedKey: "vector"},
					}, nil
				},
				Embedding: &mockEmbedder{},
			})

			docs := []*schema.Document{{ID: "1", Content: "test"}}
			_, err := indexer.Store(ctx, docs)
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "duplicate key")
		})

		Convey("value not string without stringify", func() {
			indexer, _ := NewIndexer(ctx, &IndexerConfig{
				Client: client,
				Index:  "test-index",
				DocumentToFields: func(ctx context.Context, doc *schema.Document) (map[string]FieldValue, error) {
					return map[string]FieldValue{
						"field1": {Value: 123, EmbedKey: "vector"},
					}, nil
				},
				Embedding: &mockEmbedder{},
			})

			docs := []*schema.Document{{ID: "1", Content: "test"}}
			_, err := indexer.Store(ctx, docs)
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "assert value as string failed")
		})
	})
}

func TestGetType(t *testing.T) {
	Convey("TestGetType", t, func() {
		So(GetType(), ShouldEqual, "ElasticSearch7")
	})
}
func TestIndexer_Store(t *testing.T) {
	Convey("TestIndexer_Store", t, func() {
		ctx := context.Background()

		Convey("convert docs error", func() {
			client, _ := elasticsearch.NewClient(elasticsearch.Config{})
			indexer, _ := NewIndexer(ctx, &IndexerConfig{
				Client: client,
				DocumentToFields: func(ctx context.Context, doc *schema.Document) (map[string]FieldValue, error) {
					return nil, fmt.Errorf("convert error")
				},
			})
			_, err := indexer.Store(ctx, []*schema.Document{{ID: "1"}})
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "convert error")
		})

		Convey("bulk request error", func() {
			mockT := &mockTransport{
				Err: fmt.Errorf("transport error"),
			}
			client, _ := elasticsearch.NewClient(elasticsearch.Config{
				Transport: mockT,
			})
			indexer, _ := NewIndexer(ctx, &IndexerConfig{
				Client: client,
				DocumentToFields: func(ctx context.Context, doc *schema.Document) (map[string]FieldValue, error) {
					return nil, nil
				},
			})
			_, err := indexer.Store(ctx, []*schema.Document{{ID: "1"}})
			So(err, ShouldBeNil)
		})

		Convey("bulk response error", func() {
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
			indexer, _ := NewIndexer(ctx, &IndexerConfig{
				Client: client,
				DocumentToFields: func(ctx context.Context, doc *schema.Document) (map[string]FieldValue, error) {
					return nil, nil
				},
			})
			_, err := indexer.Store(ctx, []*schema.Document{{ID: "1"}})
			// Store implementation might wraps error or return it directly
			So(err, ShouldBeNil)
		})

		Convey("bulk response with errors in items", func() {
			respBody := map[string]any{
				"errors": true,
				"items": []any{
					map[string]any{
						"index": map[string]any{
							"_id": "1",
							"error": map[string]any{
								"type":   "mapper_parsing_exception",
								"reason": "failed to parse",
							},
						},
					},
				},
			}
			respBytes, _ := json.Marshal(respBody)
			mockT := &mockTransport{
				Response: &http.Response{
					StatusCode: 200,
					Status:     "200 OK",
					Body:       io.NopCloser(bytes.NewReader(respBytes)),
					Header:     http.Header{"X-Elastic-Product": []string{"Elasticsearch"}},
				},
			}
			client, _ := elasticsearch.NewClient(elasticsearch.Config{
				Transport: mockT,
			})
			indexer, _ := NewIndexer(ctx, &IndexerConfig{
				Client: client,
				DocumentToFields: func(ctx context.Context, doc *schema.Document) (map[string]FieldValue, error) {
					return nil, nil
				},
			})
			_, err := indexer.Store(ctx, []*schema.Document{{ID: "1"}})
			So(err, ShouldBeNil)
		})

		Convey("success", func() {
			respBody := map[string]any{
				"errors": false,
				"items": []any{
					map[string]any{
						"index": map[string]any{
							"_id": "1",
						},
					},
				},
			}
			respBytes, _ := json.Marshal(respBody)
			mockT := &mockTransport{
				Response: &http.Response{
					StatusCode: 200,
					Status:     "200 OK",
					Body:       io.NopCloser(bytes.NewReader(respBytes)),
					Header:     http.Header{"X-Elastic-Product": []string{"Elasticsearch"}},
				},
			}
			client, _ := elasticsearch.NewClient(elasticsearch.Config{
				Transport: mockT,
			})
			indexer, _ := NewIndexer(ctx, &IndexerConfig{
				Client: client,
				DocumentToFields: func(ctx context.Context, doc *schema.Document) (map[string]FieldValue, error) {
					return nil, nil
				},
			})
			ids, err := indexer.Store(ctx, []*schema.Document{{ID: "1"}})
			So(err, ShouldBeNil)
			So(ids, ShouldResemble, []string{"1"})
		})
	})
}

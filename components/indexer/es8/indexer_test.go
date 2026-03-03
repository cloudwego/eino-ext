/*
 * Copyright 2024 CloudWeGo Authors
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

package es8

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	. "github.com/bytedance/mockey"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/schema"
	elasticsearch "github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esutil"
	"github.com/smartystreets/goconvey/convey"
)

func TestBulkAdd(t *testing.T) {
	PatchConvey("test bulkAdd", t, func() {
		ctx := context.Background()
		extField := "extra_field"

		d1 := &schema.Document{ID: "123", Content: "asd", MetaData: map[string]any{extField: "ext_1"}}
		d2 := &schema.Document{ID: "456", Content: "qwe", MetaData: map[string]any{extField: "ext_2"}}
		docs := []*schema.Document{d1, d2}

		var mockRetBI esutil.BulkIndexer
		var mockRetErr error
		mockErr := fmt.Errorf("test err")

		Mock(esutil.NewBulkIndexer).To(func(cfg esutil.BulkIndexerConfig) (esutil.BulkIndexer, error) {
			return mockRetBI, mockRetErr
		}).Build()

		PatchConvey("test NewBulkIndexer error", func() {
			mockRetBI = nil
			mockRetErr = mockErr
			i := &Indexer{
				config: &IndexerConfig{
					Index: "mock_index",
					DocumentToFields: func(ctx context.Context, doc *schema.Document) (field2Value map[string]FieldValue, err error) {
						return nil, nil
					},
				},
			}
			err := i.bulkAdd(ctx, docs, &indexer.Options{
				Embedding: &mockEmbedding{size: []int{1}, mockVector: []float64{2.1}},
			})
			convey.So(err, convey.ShouldBeError, mockErr)
		})

		PatchConvey("test FieldMapping error", func() {
			mbi := &mockBulkIndexer{}
			mockRetBI = mbi
			mockRetErr = nil
			i := &Indexer{
				config: &IndexerConfig{
					Index: "mock_index",
					DocumentToFields: func(ctx context.Context, doc *schema.Document) (field2Value map[string]FieldValue, err error) {
						return nil, mockErr
					},
				},
			}
			err := i.bulkAdd(ctx, docs, &indexer.Options{
				Embedding: &mockEmbedding{size: []int{1}, mockVector: []float64{2.1}},
			})
			convey.So(err, convey.ShouldBeError, fmt.Errorf("[bulkAdd] FieldMapping failed, %w", mockErr))
		})

		PatchConvey("test len(needEmbeddingFields) > i.config.BatchSize", func() {
			mbi := &mockBulkIndexer{}
			mockRetBI = mbi
			mockRetErr = nil
			i := &Indexer{
				config: &IndexerConfig{
					Index:     "mock_index",
					BatchSize: 1,
					DocumentToFields: func(ctx context.Context, doc *schema.Document) (field2Value map[string]FieldValue, err error) {
						return map[string]FieldValue{
							"k1": {Value: "v1", EmbedKey: "k"},
							"k2": {Value: "v2", EmbedKey: "kk"},
						}, nil
					},
				},
			}
			err := i.bulkAdd(ctx, docs, &indexer.Options{
				Embedding: &mockEmbedding{size: []int{1}, mockVector: []float64{2.1}},
			})
			convey.So(err, convey.ShouldBeError, fmt.Errorf("[bulkAdd] needEmbeddingFields length over batch size, batch size=%d, got size=%d", i.config.BatchSize, 2))
		})

		PatchConvey("test embedding not provided", func() {
			mbi := &mockBulkIndexer{}
			mockRetBI = mbi
			mockRetErr = nil
			i := &Indexer{
				config: &IndexerConfig{
					Index:     "mock_index",
					BatchSize: 2,
					DocumentToFields: func(ctx context.Context, doc *schema.Document) (field2Value map[string]FieldValue, err error) {
						return map[string]FieldValue{
							"k0": {Value: "v0"},
							"k1": {Value: "v1", EmbedKey: "vk1"},
							"k2": {Value: 222, EmbedKey: "vk2", Stringify: func(val any) (string, error) {
								return "222", nil
							}},
							"k3": {Value: 123},
						}, nil
					},
				},
			}
			err := i.bulkAdd(ctx, docs, &indexer.Options{
				Embedding: nil,
			})
			convey.So(err, convey.ShouldBeError, fmt.Errorf("[bulkAdd] embedding method not provided"))
		})

		PatchConvey("test embed failed", func() {
			mbi := &mockBulkIndexer{}
			mockRetBI = mbi
			mockRetErr = nil
			i := &Indexer{
				config: &IndexerConfig{
					Index:     "mock_index",
					BatchSize: 2,
					DocumentToFields: func(ctx context.Context, doc *schema.Document) (field2Value map[string]FieldValue, err error) {
						return map[string]FieldValue{
							"k0": {Value: "v0"},
							"k1": {Value: "v1", EmbedKey: "vk1"},
							"k2": {Value: 222, EmbedKey: "vk2", Stringify: func(val any) (string, error) {
								return "222", nil
							}},
							"k3": {Value: 123},
						}, nil
					},
				},
			}
			err := i.bulkAdd(ctx, docs, &indexer.Options{
				Embedding: &mockEmbedding{err: mockErr},
			})
			convey.So(err, convey.ShouldBeError, fmt.Errorf("[bulkAdd] embedding failed, %w", mockErr))
		})

		PatchConvey("test len(vectors) != len(texts)", func() {
			mbi := &mockBulkIndexer{}
			mockRetBI = mbi
			mockRetErr = nil
			i := &Indexer{
				config: &IndexerConfig{
					Index:     "mock_index",
					BatchSize: 2,
					DocumentToFields: func(ctx context.Context, doc *schema.Document) (field2Value map[string]FieldValue, err error) {
						return map[string]FieldValue{
							"k0": {Value: "v0"},
							"k1": {Value: "v1", EmbedKey: "vk1"},
							"k2": {Value: 222, EmbedKey: "vk2", Stringify: func(val any) (string, error) {
								return "222", nil
							}},
							"k3": {Value: 123},
						}, nil
					},
				},
			}
			err := i.bulkAdd(ctx, docs, &indexer.Options{
				Embedding: &mockEmbedding{size: []int{1}, mockVector: []float64{2.1}},
			})
			convey.So(err, convey.ShouldBeError, fmt.Errorf("[bulkAdd] invalid vector length, expected=%d, got=%d", 2, 1))
		})

		PatchConvey("test success", func() {
			mbi := &mockBulkIndexer{}
			mockRetBI = mbi
			mockRetErr = nil
			var mps []esutil.BulkIndexerItem
			mbi.addFunc = func(ctx context.Context, item esutil.BulkIndexerItem) error {
				mps = append(mps, item)
				return nil
			}

			i := &Indexer{
				config: &IndexerConfig{
					Index:     "mock_index",
					BatchSize: 2,
					DocumentToFields: func(ctx context.Context, doc *schema.Document) (field2Value map[string]FieldValue, err error) {
						return map[string]FieldValue{
							"k0": {Value: doc.Content},
							"k1": {Value: "v1", EmbedKey: "vk1"},
							"k2": {Value: 222, EmbedKey: "vk2", Stringify: func(val any) (string, error) { return "222", nil }},
							"k3": {Value: 123},
						}, nil
					},
				},
			}
			err := i.bulkAdd(ctx, docs, &indexer.Options{
				Embedding: &mockEmbedding{size: []int{2, 2}, mockVector: []float64{2.1}},
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(len(mps), convey.ShouldEqual, 2)
			for j, doc := range docs {
				item := mps[j]
				convey.So(item.DocumentID, convey.ShouldEqual, doc.ID)
				b, err := io.ReadAll(item.Body)
				convey.So(err, convey.ShouldBeNil)
				var mp map[string]any
				convey.So(json.Unmarshal(b, &mp), convey.ShouldBeNil)
				convey.So(mp["k0"], convey.ShouldEqual, doc.Content)
				convey.So(mp["k1"], convey.ShouldEqual, "v1")
				convey.So(mp["k2"], convey.ShouldEqual, 222)
				convey.So(mp["k3"], convey.ShouldEqual, 123)
				convey.So(mp["vk1"], convey.ShouldEqual, []any{2.1})
				convey.So(mp["vk2"], convey.ShouldEqual, []any{2.1})
				convey.So(item.OnFailure, convey.ShouldNotBeNil)
			}
		})
	})
}

type mockBulkIndexer struct {
	esutil.BulkIndexer
	addFunc   func(context.Context, esutil.BulkIndexerItem) error
	closeFunc func(context.Context) error
}

func (m *mockBulkIndexer) Add(ctx context.Context, item esutil.BulkIndexerItem) error {
	if m.addFunc != nil {
		return m.addFunc(ctx, item)
	}
	return nil
}

func (m *mockBulkIndexer) Close(ctx context.Context) error {
	if m.closeFunc != nil {
		return m.closeFunc(ctx)
	}
	return nil
}

func (m *mockBulkIndexer) Stats() esutil.BulkIndexerStats {
	return esutil.BulkIndexerStats{}
}

type mockEmbedding struct {
	err        error
	call       int
	size       []int
	mockVector []float64
}

func (m *mockEmbedding) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	if m.err != nil {
		return nil, m.err
	}

	if m.call >= len(m.size) {
		return nil, fmt.Errorf("call limit error")
	}

	resp := make([][]float64, m.size[m.call])
	m.call++
	for i := range resp {
		resp[i] = m.mockVector
	}

	return resp, nil
}

// mockTransportCreation handles index creation API calls for testing
type mockTransportCreation struct {
	existsResponse *http.Response
	createResponse *http.Response
	existsCalled   bool
	createCalled   bool
	createBody     []byte
}

func (m *mockTransportCreation) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method == "GET" && req.URL.Path == "/" {
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewReader([]byte(`{"version":{"number":"8.12.0"}}`))),
			Header:     http.Header{"X-Elastic-Product": []string{"Elasticsearch"}},
		}, nil
	}
	if req.Method == "HEAD" {
		m.existsCalled = true
		if m.existsResponse != nil {
			return m.existsResponse, nil
		}
		return &http.Response{
			StatusCode: 404,
			Body:       io.NopCloser(bytes.NewReader([]byte{})),
			Header:     http.Header{"X-Elastic-Product": []string{"Elasticsearch"}},
		}, nil
	}
	if req.Method == "PUT" {
		m.createCalled = true
		if req.Body != nil {
			m.createBody, _ = io.ReadAll(req.Body)
			req.Body = io.NopCloser(bytes.NewReader(m.createBody))
		}
		if m.createResponse != nil {
			return m.createResponse, nil
		}
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewReader([]byte(`{"acknowledged": true}`))),
			Header:     http.Header{"X-Elastic-Product": []string{"Elasticsearch"}},
		}, nil
	}
	return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
}

func TestNewIndexer(t *testing.T) {
	PatchConvey("TestNewIndexer", t, func() {
		ctx := context.Background()
		client, _ := elasticsearch.NewClient(elasticsearch.Config{})
		docToFields := func(ctx context.Context, doc *schema.Document) (map[string]FieldValue, error) {
			return nil, nil
		}

		PatchConvey("success with defaults", func() {
			idx, err := NewIndexer(ctx, &IndexerConfig{
				Client:           client,
				DocumentToFields: docToFields,
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(idx, convey.ShouldNotBeNil)
			convey.So(idx.config.BatchSize, convey.ShouldEqual, defaultBatchSize)
			convey.So(idx.GetType(), convey.ShouldEqual, typ)
			convey.So(idx.IsCallbacksEnabled(), convey.ShouldBeTrue)
		})

		PatchConvey("success with custom batch size", func() {
			idx, err := NewIndexer(ctx, &IndexerConfig{
				Client:           client,
				BatchSize:        10,
				DocumentToFields: docToFields,
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(idx, convey.ShouldNotBeNil)
			convey.So(idx.config.BatchSize, convey.ShouldEqual, 10)
		})

		convey.Convey("IndexSpec - index exists", func() {
			mockT := &mockTransportCreation{
				existsResponse: &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewReader([]byte{})),
					Header:     http.Header{"X-Elastic-Product": []string{"Elasticsearch"}},
				},
			}
			client, _ := elasticsearch.NewClient(elasticsearch.Config{Transport: mockT})
			_, err := NewIndexer(ctx, &IndexerConfig{
				Client:    client,
				Index:     "test-index",
				BatchSize: 10,
				IndexSpec: &IndexSpec{Settings: map[string]any{"number_of_shards": 1}},
				DocumentToFields: func(ctx context.Context, doc *schema.Document) (map[string]FieldValue, error) {
					return nil, nil
				},
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(mockT.existsCalled, convey.ShouldBeTrue)
			convey.So(mockT.createCalled, convey.ShouldBeFalse)
		})

		convey.Convey("IndexSpec - index not exists and create success", func() {
			mockT := &mockTransportCreation{
				existsResponse: &http.Response{
					StatusCode: 404,
					Body:       io.NopCloser(bytes.NewReader([]byte{})),
					Header:     http.Header{"X-Elastic-Product": []string{"Elasticsearch"}},
				},
				createResponse: &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewReader([]byte(`{"acknowledged": true}`))),
					Header:     http.Header{"X-Elastic-Product": []string{"Elasticsearch"}},
				},
			}
			client, _ := elasticsearch.NewClient(elasticsearch.Config{Transport: mockT})
			_, err := NewIndexer(ctx, &IndexerConfig{
				Client:    client,
				Index:     "test-index",
				BatchSize: 10,
				IndexSpec: &IndexSpec{Settings: map[string]any{"number_of_shards": 1}},
				DocumentToFields: func(ctx context.Context, doc *schema.Document) (map[string]FieldValue, error) {
					return nil, nil
				},
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(mockT.existsCalled, convey.ShouldBeTrue)
			convey.So(mockT.createCalled, convey.ShouldBeTrue)
		})

		convey.Convey("IndexSpec - index creation fails", func() {
			mockT := &mockTransportCreation{
				existsResponse: &http.Response{
					StatusCode: 404,
					Body:       io.NopCloser(bytes.NewReader([]byte{})),
					Header:     http.Header{"X-Elastic-Product": []string{"Elasticsearch"}},
				},
				createResponse: &http.Response{
					StatusCode: 500,
					Body:       io.NopCloser(bytes.NewReader([]byte(`{"error": "failed"}`))),
					Header:     http.Header{"X-Elastic-Product": []string{"Elasticsearch"}},
				},
			}
			client, _ := elasticsearch.NewClient(elasticsearch.Config{Transport: mockT})
			_, err := NewIndexer(ctx, &IndexerConfig{
				Client:    client,
				Index:     "test-index",
				BatchSize: 10,
				IndexSpec: &IndexSpec{Settings: map[string]any{"number_of_shards": 1}},
				DocumentToFields: func(ctx context.Context, doc *schema.Document) (map[string]FieldValue, error) {
					return nil, nil
				},
			})
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "create index failed")
		})

		convey.Convey("client not provided", func() {
			_, err := NewIndexer(ctx, &IndexerConfig{
				DocumentToFields: func(ctx context.Context, doc *schema.Document) (map[string]FieldValue, error) {
					return nil, nil
				},
			})
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "es client not provided")
		})

		convey.Convey("DocumentToFields not provided", func() {
			client, _ := elasticsearch.NewClient(elasticsearch.Config{})
			_, err := NewIndexer(ctx, &IndexerConfig{
				Client: client,
			})
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "DocumentToFields method not provided")
		})

		convey.Convey("IndexSpec - index existence check fails", func() {
			mockT := &mockTransportCreation{
				existsResponse: &http.Response{
					StatusCode: 500,
					Body:       io.NopCloser(bytes.NewReader([]byte(`{"error": "failed"}`))),
					Header:     http.Header{"X-Elastic-Product": []string{"Elasticsearch"}},
				},
			}
			client, _ := elasticsearch.NewClient(elasticsearch.Config{Transport: mockT})
			_, err := NewIndexer(ctx, &IndexerConfig{
				Client:    client,
				Index:     "test-index",
				BatchSize: 10,
				IndexSpec: &IndexSpec{Settings: map[string]any{"number_of_shards": 1}},
				DocumentToFields: func(ctx context.Context, doc *schema.Document) (map[string]FieldValue, error) {
					return nil, nil
				},
			})
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "check index existence failed")
		})
	})
}

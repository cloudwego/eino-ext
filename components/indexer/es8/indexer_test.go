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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"testing"

	. "github.com/bytedance/mockey"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/schema"
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
		bi, err := esutil.NewBulkIndexer(esutil.BulkIndexerConfig{})
		convey.So(err, convey.ShouldBeNil)

		PatchConvey("test NewBulkIndexer error", func() {
			mockErr := fmt.Errorf("test err")
			Mock(esutil.NewBulkIndexer).Return(nil, mockErr).Build()
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
			mockErr := fmt.Errorf("test err")
			Mock(esutil.NewBulkIndexer).Return(bi, nil).Build()
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
			Mock(esutil.NewBulkIndexer).Return(bi, nil).Build()
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
			Mock(esutil.NewBulkIndexer).Return(bi, nil).Build()
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
			mockErr := fmt.Errorf("test err")
			Mock(esutil.NewBulkIndexer).Return(bi, nil).Build()
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
			Mock(esutil.NewBulkIndexer).Return(bi, nil).Build()
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
			var mps []esutil.BulkIndexerItem
			Mock(esutil.NewBulkIndexer).Return(bi, nil).Build()
			Mock(GetMethod(bi, "Add")).To(func(ctx context.Context, item esutil.BulkIndexerItem) error {
				mps = append(mps, item)
				return nil
			}).Build()
			Mock(GetMethod(bi, "Close")).Return(nil).Build()

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

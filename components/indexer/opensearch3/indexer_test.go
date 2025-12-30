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

package opensearch3

import (
	"context"
	"fmt"
	"testing"

	. "github.com/bytedance/mockey"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/schema"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
	"github.com/opensearch-project/opensearch-go/v4/opensearchutil"
	"github.com/smartystreets/goconvey/convey"
)

func TestNewIndexer(t *testing.T) {
	PatchConvey("test NewIndexer", t, func() {
		ctx := context.Background()
		client := &opensearchapi.Client{}

		PatchConvey("test missing client", func() {
			i, err := NewIndexer(ctx, &IndexerConfig{
				DocumentToFields: func(ctx context.Context, doc *schema.Document) (map[string]FieldValue, error) {
					return nil, nil
				},
			})
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "client not provided")
			convey.So(i, convey.ShouldBeNil)
		})

		PatchConvey("test missing DocumentToFields", func() {
			i, err := NewIndexer(ctx, &IndexerConfig{
				Client: client,
			})
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "DocumentToFields method not provided")
			convey.So(i, convey.ShouldBeNil)
		})

		PatchConvey("test success with defaults", func() {
			i, err := NewIndexer(ctx, &IndexerConfig{
				Client: client,
				DocumentToFields: func(ctx context.Context, doc *schema.Document) (map[string]FieldValue, error) {
					return nil, nil
				},
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(i, convey.ShouldNotBeNil)
			convey.So(i.config.BatchSize, convey.ShouldEqual, defaultBatchSize)
		})

		PatchConvey("test success with custom batch size", func() {
			i, err := NewIndexer(ctx, &IndexerConfig{
				Client:    client,
				BatchSize: 10,
				DocumentToFields: func(ctx context.Context, doc *schema.Document) (map[string]FieldValue, error) {
					return nil, nil
				},
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(i, convey.ShouldNotBeNil)
			convey.So(i.config.BatchSize, convey.ShouldEqual, 10)
		})
	})
}

func TestBulkAdd(t *testing.T) {
	PatchConvey("test bulkAdd", t, func() {
		ctx := context.Background()
		client := &opensearchapi.Client{}

		d1 := &schema.Document{ID: "123", Content: "content 1", MetaData: map[string]any{"field": "value1"}}
		d2 := &schema.Document{ID: "456", Content: "content 2", MetaData: map[string]any{"field": "value2"}}
		docs := []*schema.Document{d1, d2}

		bi, err := opensearchutil.NewBulkIndexer(opensearchutil.BulkIndexerConfig{})
		convey.So(err, convey.ShouldBeNil)

		PatchConvey("test NewBulkIndexer error", func() {
			mockErr := fmt.Errorf("test err")
			Mock(opensearchutil.NewBulkIndexer).Return(nil, mockErr).Build()
			i := &Indexer{
				client: client,
				config: &IndexerConfig{
					Index: "mock_index",
					DocumentToFields: func(ctx context.Context, doc *schema.Document) (map[string]FieldValue, error) {
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
			mockErr := fmt.Errorf("field mapping error")
			Mock(opensearchutil.NewBulkIndexer).Return(bi, nil).Build()
			i := &Indexer{
				client: client,
				config: &IndexerConfig{
					Index: "mock_index",
					DocumentToFields: func(ctx context.Context, doc *schema.Document) (map[string]FieldValue, error) {
						return nil, mockErr
					},
				},
			}
			err := i.bulkAdd(ctx, docs, &indexer.Options{
				Embedding: &mockEmbedding{size: []int{1}, mockVector: []float64{2.1}},
			})
			convey.So(err, convey.ShouldBeError, fmt.Errorf("[bulkAdd] FieldMapping failed, %w", mockErr))
		})

		PatchConvey("test needEmbeddingFields length over batch size", func() {
			Mock(opensearchutil.NewBulkIndexer).Return(bi, nil).Build()
			i := &Indexer{
				client: client,
				config: &IndexerConfig{
					Index:     "mock_index",
					BatchSize: 1,
					DocumentToFields: func(ctx context.Context, doc *schema.Document) (map[string]FieldValue, error) {
						return map[string]FieldValue{
							"k1": {Value: "v1", EmbedKey: "vk1"},
							"k2": {Value: "v2", EmbedKey: "vk2"},
						}, nil
					},
				},
			}
			err := i.bulkAdd(ctx, docs, &indexer.Options{
				Embedding: &mockEmbedding{size: []int{1}, mockVector: []float64{2.1}},
			})
			convey.So(err, convey.ShouldBeError, fmt.Errorf("[bulkAdd] needEmbeddingFields length over batch size, batch size=%d, got size=%d", 1, 2))
		})

		PatchConvey("test embedding not provided", func() {
			Mock(opensearchutil.NewBulkIndexer).Return(bi, nil).Build()
			i := &Indexer{
				client: client,
				config: &IndexerConfig{
					Index:     "mock_index",
					BatchSize: 2,
					DocumentToFields: func(ctx context.Context, doc *schema.Document) (map[string]FieldValue, error) {
						return map[string]FieldValue{
							"k1": {Value: "v1", EmbedKey: "vk1"},
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
			mockErr := fmt.Errorf("embed error")
			Mock(opensearchutil.NewBulkIndexer).Return(bi, nil).Build()
			i := &Indexer{
				client: client,
				config: &IndexerConfig{
					Index:     "mock_index",
					BatchSize: 2,
					DocumentToFields: func(ctx context.Context, doc *schema.Document) (map[string]FieldValue, error) {
						return map[string]FieldValue{
							"k1": {Value: "v1", EmbedKey: "vk1"},
						}, nil
					},
				},
			}
			err := i.bulkAdd(ctx, docs, &indexer.Options{
				Embedding: &mockEmbedding{err: mockErr},
			})
			convey.So(err, convey.ShouldBeError, fmt.Errorf("[bulkAdd] embedding failed, %w", mockErr))
		})

		PatchConvey("test invalid vector length", func() {
			Mock(opensearchutil.NewBulkIndexer).Return(bi, nil).Build()
			i := &Indexer{
				client: client,
				config: &IndexerConfig{
					Index:     "mock_index",
					BatchSize: 2,
					DocumentToFields: func(ctx context.Context, doc *schema.Document) (map[string]FieldValue, error) {
						return map[string]FieldValue{
							"k1": {Value: "v1", EmbedKey: "vk1"},
						}, nil
					},
				},
			}
			err := i.bulkAdd(ctx, docs, &indexer.Options{
				Embedding: &mockEmbedding{size: []int{1}, mockVector: []float64{2.1}},
			})
			convey.So(err, convey.ShouldBeError, fmt.Errorf("[bulkAdd] invalid vector length, expected=%d, got=%d", 2, 1))
		})

		PatchConvey("test duplicate key for origin key", func() {
			Mock(opensearchutil.NewBulkIndexer).Return(bi, nil).Build()
			i := &Indexer{
				client: client,
				config: &IndexerConfig{
					Index:     "mock_index",
					BatchSize: 5,
					DocumentToFields: func(ctx context.Context, doc *schema.Document) (map[string]FieldValue, error) {
						return map[string]FieldValue{
							"k1":  {Value: "v1", EmbedKey: "k2"}, // k2 already exists as a key
							"k2":  {Value: "v2"},
							"vk1": {Value: "unused"},
						}, nil
					},
				},
			}
			err := i.bulkAdd(ctx, docs, &indexer.Options{
				Embedding: &mockEmbedding{size: []int{1, 1}, mockVector: []float64{2.1}},
			})
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "duplicate key for origin key")
		})

		PatchConvey("test assert value as string failed", func() {
			Mock(opensearchutil.NewBulkIndexer).Return(bi, nil).Build()
			i := &Indexer{
				client: client,
				config: &IndexerConfig{
					Index:     "mock_index",
					BatchSize: 5,
					DocumentToFields: func(ctx context.Context, doc *schema.Document) (map[string]FieldValue, error) {
						return map[string]FieldValue{
							"k1": {Value: 123, EmbedKey: "vk1"}, // int cannot be asserted as string
						}, nil
					},
				},
			}
			err := i.bulkAdd(ctx, docs, &indexer.Options{
				Embedding: &mockEmbedding{size: []int{2}, mockVector: []float64{2.1}},
			})
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "assert value as string failed")
		})

		PatchConvey("test stringify error", func() {
			stringifyErr := fmt.Errorf("stringify error")
			Mock(opensearchutil.NewBulkIndexer).Return(bi, nil).Build()
			i := &Indexer{
				client: client,
				config: &IndexerConfig{
					Index:     "mock_index",
					BatchSize: 5,
					DocumentToFields: func(ctx context.Context, doc *schema.Document) (map[string]FieldValue, error) {
						return map[string]FieldValue{
							"k1": {
								Value:    123,
								EmbedKey: "vk1",
								Stringify: func(val any) (string, error) {
									return "", stringifyErr
								},
							},
						}, nil
					},
				},
			}
			err := i.bulkAdd(ctx, docs, &indexer.Options{
				Embedding: &mockEmbedding{size: []int{2}, mockVector: []float64{2.1}},
			})
			convey.So(err, convey.ShouldBeError, stringifyErr)
		})

		PatchConvey("test success without embedding", func() {
			var addedItems []opensearchutil.BulkIndexerItem
			Mock(opensearchutil.NewBulkIndexer).Return(bi, nil).Build()
			Mock(GetMethod(bi, "Add")).To(func(ctx context.Context, item opensearchutil.BulkIndexerItem) error {
				addedItems = append(addedItems, item)
				return nil
			}).Build()
			Mock(GetMethod(bi, "Close")).Return(nil).Build()

			i := &Indexer{
				client: client,
				config: &IndexerConfig{
					Index:     "mock_index",
					BatchSize: 5,
					DocumentToFields: func(ctx context.Context, doc *schema.Document) (map[string]FieldValue, error) {
						return map[string]FieldValue{
							"content": {Value: doc.Content},
							"field":   {Value: doc.MetaData["field"]},
						}, nil
					},
				},
			}
			err := i.bulkAdd(ctx, docs, &indexer.Options{})
			convey.So(err, convey.ShouldBeNil)
			convey.So(len(addedItems), convey.ShouldEqual, 2)
			convey.So(addedItems[0].DocumentID, convey.ShouldEqual, "123")
			convey.So(addedItems[1].DocumentID, convey.ShouldEqual, "456")
		})

		PatchConvey("test success with embedding", func() {
			var addedItems []opensearchutil.BulkIndexerItem
			Mock(opensearchutil.NewBulkIndexer).Return(bi, nil).Build()
			Mock(GetMethod(bi, "Add")).To(func(ctx context.Context, item opensearchutil.BulkIndexerItem) error {
				addedItems = append(addedItems, item)
				return nil
			}).Build()
			Mock(GetMethod(bi, "Close")).Return(nil).Build()

			i := &Indexer{
				client: client,
				config: &IndexerConfig{
					Index:     "mock_index",
					BatchSize: 5,
					DocumentToFields: func(ctx context.Context, doc *schema.Document) (map[string]FieldValue, error) {
						return map[string]FieldValue{
							"content": {Value: doc.Content, EmbedKey: "vector"},
						}, nil
					},
				},
			}
			err := i.bulkAdd(ctx, docs, &indexer.Options{
				Embedding: &mockEmbedding{size: []int{2}, mockVector: []float64{0.1, 0.2, 0.3}},
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(len(addedItems), convey.ShouldEqual, 2)
		})

		PatchConvey("test success with custom stringify", func() {
			var addedItems []opensearchutil.BulkIndexerItem
			Mock(opensearchutil.NewBulkIndexer).Return(bi, nil).Build()
			Mock(GetMethod(bi, "Add")).To(func(ctx context.Context, item opensearchutil.BulkIndexerItem) error {
				addedItems = append(addedItems, item)
				return nil
			}).Build()
			Mock(GetMethod(bi, "Close")).Return(nil).Build()

			i := &Indexer{
				client: client,
				config: &IndexerConfig{
					Index:     "mock_index",
					BatchSize: 5,
					DocumentToFields: func(ctx context.Context, doc *schema.Document) (map[string]FieldValue, error) {
						return map[string]FieldValue{
							"num": {
								Value:    123,
								EmbedKey: "vector",
								Stringify: func(val any) (string, error) {
									return fmt.Sprintf("%d", val.(int)), nil
								},
							},
						}, nil
					},
				},
			}
			err := i.bulkAdd(ctx, docs, &indexer.Options{
				Embedding: &mockEmbedding{size: []int{2}, mockVector: []float64{0.1, 0.2}},
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(len(addedItems), convey.ShouldEqual, 2)
		})
	})
}

func TestStore(t *testing.T) {
	PatchConvey("test Store", t, func() {
		ctx := context.Background()
		client := &opensearchapi.Client{}

		docs := []*schema.Document{
			{ID: "doc1", Content: "content 1"},
			{ID: "doc2", Content: "content 2"},
		}

		bi, err := opensearchutil.NewBulkIndexer(opensearchutil.BulkIndexerConfig{})
		convey.So(err, convey.ShouldBeNil)

		PatchConvey("test bulkAdd error", func() {
			mockErr := fmt.Errorf("bulk add error")
			Mock(opensearchutil.NewBulkIndexer).Return(nil, mockErr).Build()

			i := &Indexer{
				client: client,
				config: &IndexerConfig{
					Index:     "mock_index",
					BatchSize: 5,
					DocumentToFields: func(ctx context.Context, doc *schema.Document) (map[string]FieldValue, error) {
						return nil, nil
					},
				},
			}
			ids, err := i.Store(ctx, docs)
			convey.So(err, convey.ShouldBeError, mockErr)
			convey.So(ids, convey.ShouldBeNil)
		})

		PatchConvey("test success", func() {
			Mock(opensearchutil.NewBulkIndexer).Return(bi, nil).Build()
			Mock(GetMethod(bi, "Add")).Return(nil).Build()
			Mock(GetMethod(bi, "Close")).Return(nil).Build()

			i := &Indexer{
				client: client,
				config: &IndexerConfig{
					Index:     "mock_index",
					BatchSize: 5,
					DocumentToFields: func(ctx context.Context, doc *schema.Document) (map[string]FieldValue, error) {
						return map[string]FieldValue{
							"content": {Value: doc.Content},
						}, nil
					},
				},
			}
			ids, err := i.Store(ctx, docs)
			convey.So(err, convey.ShouldBeNil)
			convey.So(len(ids), convey.ShouldEqual, 2)
			convey.So(ids[0], convey.ShouldEqual, "doc1")
			convey.So(ids[1], convey.ShouldEqual, "doc2")
		})
	})
}

func TestIndexerGetType(t *testing.T) {
	PatchConvey("test GetType", t, func() {
		i := &Indexer{}
		convey.So(i.GetType(), convey.ShouldEqual, typ)
	})
}

func TestIndexerIsCallbacksEnabled(t *testing.T) {
	PatchConvey("test IsCallbacksEnabled", t, func() {
		i := &Indexer{}
		convey.So(i.IsCallbacksEnabled(), convey.ShouldBeTrue)
	})
}

func TestIndexerGetTypeFunc(t *testing.T) {
	PatchConvey("test GetType function", t, func() {
		convey.So(GetType(), convey.ShouldEqual, typ)
	})
}

// mockEmbedding implements Embedder interface for testing
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

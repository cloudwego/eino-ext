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

package redis

import (
	"context"
	"fmt"
	"log"
	"testing"

	. "github.com/bytedance/mockey"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/schema"
	"github.com/redis/go-redis/v9"
	"github.com/smartystreets/goconvey/convey"
)

func TestPipelineHSet(t *testing.T) {
	PatchConvey("test pipelineHSet", t, func() {
		ctx := context.Background()
		mockClient := &redis.Client{}
		d1 := &schema.Document{ID: "1", Content: "asd"}
		d2 := &schema.Document{ID: "2", Content: "qwe", MetaData: map[string]any{
			"mock_field_1": map[string]any{"extra_field_1": "asd"},
			"mock_field_2": int64(123),
		}}
		docs := []*schema.Document{d1, d2}

		PatchConvey("test DocumentToHashes failed", func() {
			i := &Indexer{
				config: &IndexerConfig{
					Client: mockClient,
					DocumentToHashes: func(ctx context.Context, doc *schema.Document) (*Hashes, error) {
						return nil, fmt.Errorf("mock err")
					},
					BatchSize: 10,
					Embedding: nil,
				},
			}

			convey.So(i.pipelineHSet(ctx, docs, &indexer.Options{
				Embedding: nil,
			}), convey.ShouldBeError, fmt.Errorf("mock err"))
		})

		PatchConvey("test embSize > i.config.BatchSize", func() {
			i := &Indexer{
				config: &IndexerConfig{
					Client: mockClient,
					DocumentToHashes: func(ctx context.Context, doc *schema.Document) (*Hashes, error) {
						return &Hashes{
							Key: doc.ID,
							Field2Value: map[string]FieldValue{
								defaultReturnFieldContent: {
									Value:    doc.Content,
									EmbedKey: defaultReturnFieldVectorContent,
								},
								"mock_another_field": {
									Value:    doc.Content,
									EmbedKey: "mock_another_vector_field",
								},
							},
						}, nil
					},
					BatchSize: 1,
					Embedding: nil,
				},
			}

			convey.So(i.pipelineHSet(ctx, docs, &indexer.Options{
				Embedding: nil,
			}), convey.ShouldBeError, fmt.Errorf("[pipelineHSet] embedding size over batch size, batch size=%d, got size=%d",
				i.config.BatchSize, 2))
		})

		PatchConvey("test embedding not provided error", func() {
			i := &Indexer{
				config: &IndexerConfig{
					Client:           mockClient,
					DocumentToHashes: defaultDocumentToFields,
					BatchSize:        1,
					Embedding:        nil,
				},
			}

			convey.So(i.pipelineHSet(ctx, docs, &indexer.Options{
				Embedding: nil,
			}), convey.ShouldBeError, fmt.Errorf("[pipelineHSet] embedding method not provided"))
		})

		PatchConvey("test embedding failed", func() {
			exp := fmt.Errorf("mock err")
			i := &Indexer{
				config: &IndexerConfig{
					Client:           mockClient,
					DocumentToHashes: defaultDocumentToFields,
					BatchSize:        1,
				},
			}

			convey.So(i.pipelineHSet(ctx, docs, &indexer.Options{
				Embedding: &mockEmbedding{err: exp},
			}), convey.ShouldBeError, fmt.Errorf("[pipelineHSet] embedding failed, %w", exp))
		})

		PatchConvey("test len(vectors) != len(texts)", func() {
			i := &Indexer{
				config: &IndexerConfig{
					Client:           mockClient,
					DocumentToHashes: defaultDocumentToFields,
					BatchSize:        1,
				},
			}

			convey.So(i.pipelineHSet(ctx, docs, &indexer.Options{
				Embedding: &mockEmbedding{sizeForCall: []int{2}, dims: 1024},
			}), convey.ShouldBeError, fmt.Errorf("[pipelineHSet] invalid vector length, expected=1, got=2"))
		})

		PatchConvey("test success", func() {
			args := make(map[string][]any)
			pl := &redis.Pipeline{}
			Mock(GetMethod(mockClient, "Pipeline")).Return(pl).Build()
			Mock(GetMethod(pl, "HSet")).To(func(ctx context.Context, key string, values ...interface{}) *redis.IntCmd {
				args[key] = values
				return nil
			}).Build()
			Mock(GetMethod(pl, "Exec")).Return(nil, nil).Build()

			prefix := "test_prefix"
			i := &Indexer{
				config: &IndexerConfig{
					Client:           mockClient,
					DocumentToHashes: defaultDocumentToFields,
					KeyPrefix:        prefix,
					BatchSize:        1,
				},
			}

			convey.So(i.pipelineHSet(ctx, docs, &indexer.Options{
				Embedding: &mockEmbedding{sizeForCall: []int{1, 1}, dims: 1024},
			}), convey.ShouldBeNil)

			slice := make([]float64, 1024)
			for i := range slice {
				slice[i] = 1.1
			}

			contains := func(doc *schema.Document) {
				a := args[prefix+doc.ID]
				convey.So(a, convey.ShouldNotBeNil)
				f2v := make(map[string]any)
				for i := 0; i < len(a); i += 2 {
					f2v[a[i].(string)] = a[i+1]
				}
				for field, val := range f2v {
					if field == defaultReturnFieldContent {
						convey.So(val.(string), convey.ShouldEqual, doc.Content)
					} else if field == defaultReturnFieldVectorContent {
						convey.So(val.([]byte), convey.ShouldEqual, vector2Bytes(slice))
					} else {
						val2, found := doc.MetaData[field]
						convey.So(found, convey.ShouldBeTrue)
						convey.So(val, convey.ShouldEqual, val2)
					}
				}
			}
			contains(d1)
			contains(d2)
		})
	})
}

type mockEmbedding struct {
	err         error
	cnt         int
	sizeForCall []int
	dims        int
}

func (m *mockEmbedding) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	if m.cnt > len(m.sizeForCall) {
		log.Fatal("unexpected")
	}

	if m.err != nil {
		return nil, m.err
	}

	slice := make([]float64, m.dims)
	for i := range slice {
		slice[i] = 1.1
	}

	r := make([][]float64, m.sizeForCall[m.cnt])
	m.cnt++
	for i := range r {
		r[i] = slice
	}

	return r, nil
}

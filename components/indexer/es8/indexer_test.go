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
	"fmt"
	"io"
	"testing"

	. "github.com/bytedance/mockey"
	"github.com/smartystreets/goconvey/convey"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/schema"
)

func TestVectorQueryItems(t *testing.T) {
	PatchConvey("test makeBulkItems", t, func() {
		ctx := context.Background()
		extField := "extra_field"

		d1 := &schema.Document{ID: "123", Content: "asd", MetaData: map[string]any{extField: "ext_1"}}
		d2 := &schema.Document{ID: "456", Content: "qwe", MetaData: map[string]any{extField: "ext_2"}}
		docs := []*schema.Document{d1, d2}

		PatchConvey("test FieldMapping error", func() {
			mockErr := fmt.Errorf("test err")
			i := &Indexer{
				config: &IndexerConfig{
					Index: "mock_index",
					FieldMapping: func(ctx context.Context, doc *schema.Document) (fields map[string]any, needEmbeddingFields map[string]string, err error) {
						return nil, nil, mockErr
					},
				},
			}

			bulks, err := i.makeBulkItems(ctx, docs, &indexer.Options{
				Embedding: &mockEmbedding{size: []int{1}, mockVector: []float64{2.1}},
			})
			convey.So(err, convey.ShouldBeError, fmt.Errorf("[makeBulkItems] FieldMapping failed, %w", mockErr))
			convey.So(len(bulks), convey.ShouldEqual, 0)
		})

		PatchConvey("test emb not provided", func() {
			i := &Indexer{
				config: &IndexerConfig{
					Index:        "mock_index",
					FieldMapping: defaultFieldMapping,
				},
			}

			bulks, err := i.makeBulkItems(ctx, docs, &indexer.Options{Embedding: nil})
			convey.So(err, convey.ShouldBeError, "[makeBulkItems] embedding method not provided")
			convey.So(len(bulks), convey.ShouldEqual, 0)
		})

		PatchConvey("test vector size invalid", func() {
			i := &Indexer{
				config: &IndexerConfig{
					Index:        "mock_index",
					FieldMapping: defaultFieldMapping,
				},
			}

			bulks, err := i.makeBulkItems(ctx, docs, &indexer.Options{
				Embedding: &mockEmbedding{size: []int{2, 2}, mockVector: []float64{2.1}},
			})
			convey.So(err, convey.ShouldBeError, "[makeBulkItems] invalid vector length, expected=1, got=2")
			convey.So(len(bulks), convey.ShouldEqual, 0)
		})

		PatchConvey("test success", func() {
			i := &Indexer{
				config: &IndexerConfig{
					Index:        "mock_index",
					FieldMapping: defaultFieldMapping,
				},
			}

			bulks, err := i.makeBulkItems(ctx, docs, &indexer.Options{
				Embedding: &mockEmbedding{size: []int{1, 1}, mockVector: []float64{2.1}},
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(len(bulks), convey.ShouldEqual, 2)
			exp := []string{
				`{"content":"asd","meta_data":{"extra_field":"ext_1"},"vector_content":[2.1]}`,
				`{"content":"qwe","meta_data":{"extra_field":"ext_2"},"vector_content":[2.1]}`,
			}

			for idx, item := range bulks {
				convey.So(item.Index, convey.ShouldEqual, i.config.Index)
				b, err := io.ReadAll(item.Body)
				fmt.Println(string(b))
				convey.So(err, convey.ShouldBeNil)
				convey.So(string(b), convey.ShouldEqual, exp[idx])
			}
		})
	})
}

type mockEmbedding struct {
	call       int
	size       []int
	mockVector []float64
}

func (m *mockEmbedding) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
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

func defaultFieldMapping(ctx context.Context, doc *schema.Document) (
	fields map[string]any, needEmbeddingFields map[string]string, err error) {

	fields = map[string]any{
		"content":   doc.Content,
		"meta_data": doc.MetaData,
	}

	needEmbeddingFields = map[string]string{
		"vector_content": doc.Content,
	}

	return fields, needEmbeddingFields, nil
}

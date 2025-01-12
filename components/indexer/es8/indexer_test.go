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

	"github.com/cloudwego/eino-ext/components/indexer/es8/field_mapping"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/schema"
)

func TestVectorQueryItems(t *testing.T) {
	PatchConvey("test vectorQueryItems", t, func() {
		ctx := context.Background()
		extField := "extra_field"

		d1 := &schema.Document{ID: "123", Content: "asd"}
		d1.WithVector([]float64{2.3, 4.4})
		field_mapping.SetExtraDataFields(d1, map[string]interface{}{extField: "ext_1"})

		d2 := &schema.Document{ID: "456", Content: "qwe"}
		field_mapping.SetExtraDataFields(d2, map[string]interface{}{extField: "ext_2"})

		docs := []*schema.Document{d1, d2}

		PatchConvey("test field not found", func() {
			i := &Indexer{
				config: &IndexerConfig{
					Index: "mock_index",
					VectorFields: []field_mapping.FieldKV{
						field_mapping.DefaultFieldKV("not_found_field"),
					},
				},
			}

			bulks, err := i.vectorQueryItems(ctx, docs, &indexer.Options{
				Embedding: &mockEmbedding{size: []int{1}, mockVector: []float64{2.1}},
			})
			convey.So(err, convey.ShouldBeError, fmt.Sprintf("[vectorQueryItems] field name not found or type incorrect, name=not_found_field, doc=%v", d1))
			convey.So(len(bulks), convey.ShouldEqual, 0)
		})

		PatchConvey("test emb not provided", func() {
			i := &Indexer{
				config: &IndexerConfig{
					Index: "mock_index",
					VectorFields: []field_mapping.FieldKV{
						field_mapping.DefaultFieldKV(field_mapping.DocFieldNameContent),
						field_mapping.DefaultFieldKV(field_mapping.FieldName(extField)),
					},
				},
			}

			bulks, err := i.vectorQueryItems(ctx, docs, &indexer.Options{Embedding: nil})
			convey.So(err, convey.ShouldBeError, "[vectorQueryItems] embedding not provided")
			convey.So(len(bulks), convey.ShouldEqual, 0)
		})

		PatchConvey("test vector size invalid", func() {
			i := &Indexer{
				config: &IndexerConfig{
					Index: "mock_index",
					VectorFields: []field_mapping.FieldKV{
						field_mapping.DefaultFieldKV(field_mapping.DocFieldNameContent),
						field_mapping.DefaultFieldKV(field_mapping.FieldName(extField)),
					},
				},
			}

			bulks, err := i.vectorQueryItems(ctx, docs, &indexer.Options{
				Embedding: &mockEmbedding{size: []int{2, 2}, mockVector: []float64{2.1}},
			})
			convey.So(err, convey.ShouldBeError, "[vectorQueryItems] invalid vector length, expected=1, got=2")
			convey.So(len(bulks), convey.ShouldEqual, 0)
		})

		PatchConvey("test success", func() {
			i := &Indexer{
				config: &IndexerConfig{
					Index: "mock_index",
					VectorFields: []field_mapping.FieldKV{
						field_mapping.DefaultFieldKV(field_mapping.DocFieldNameContent),
						field_mapping.DefaultFieldKV(field_mapping.FieldName(extField)),
					},
				},
			}

			bulks, err := i.vectorQueryItems(ctx, docs, &indexer.Options{
				Embedding: &mockEmbedding{size: []int{1, 2}, mockVector: []float64{2.1}},
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(len(bulks), convey.ShouldEqual, 2)
			exp := []string{
				`{"eino_doc_content":"asd","extra_field":"ext_1","vector_eino_doc_content":[2.3,4.4],"vector_extra_field":[2.1]}`,
				`{"eino_doc_content":"qwe","extra_field":"ext_2","vector_eino_doc_content":[2.1],"vector_extra_field":[2.1]}`,
			}

			for idx, item := range bulks {
				convey.So(item.Index, convey.ShouldEqual, i.config.Index)
				b, err := io.ReadAll(item.Body)
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

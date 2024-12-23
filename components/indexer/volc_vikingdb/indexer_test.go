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

package volc_vikingdb

import (
	"context"
	"fmt"
	"testing"

	. "github.com/bytedance/mockey"
	"github.com/smartystreets/goconvey/convey"
	"github.com/volcengine/volc-sdk-golang/service/vikingdb"

)

func TestNewIndexer(t *testing.T) {
	PatchConvey("test NewIndexer", t, func() {
		ctx := context.Background()

		PatchConvey("test embedding set error", func() {
			i, err := NewIndexer(ctx, &IndexerConfig{
				EmbeddingConfig: EmbeddingConfig{
					UseBuiltin: true,
					Embedding:  &mockEmbedding{},
				},
			})
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "no need to provide Embedding when UseBuiltin embedding is true")
			convey.So(i, convey.ShouldBeNil)

			i, err = NewIndexer(ctx, &IndexerConfig{
				EmbeddingConfig: EmbeddingConfig{
					UseBuiltin: false,
					Embedding:  nil,
				},
			})
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "need provide Embedding when UseBuiltin embedding is false")
			convey.So(i, convey.ShouldBeNil)
		})

		PatchConvey("test GetCollection failed", func() {
			svc := &vikingdb.VikingDBService{}
			Mock(vikingdb.NewVikingDBService).Return(svc).Build()
			Mock(GetMethod(svc, "SetConnectionTimeout")).Return().Build()
			Mock(GetMethod(svc, "GetCollection")).Return(nil, fmt.Errorf("mock err")).Build()

			i, err := NewIndexer(ctx, &IndexerConfig{
				AddBatchSize:      100,
				ConnectionTimeout: 10000,
				EmbeddingConfig: EmbeddingConfig{
					UseBuiltin: true,
				},
			})
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "mock err")
			convey.So(i, convey.ShouldBeNil)
		})

		PatchConvey("test success", func() {
			svc := &vikingdb.VikingDBService{}
			Mock(vikingdb.NewVikingDBService).Return(svc).Build()
			Mock(GetMethod(svc, "SetConnectionTimeout")).Return().Build()
			Mock(GetMethod(svc, "GetCollection")).Return(nil, nil).Build()

			i, err := NewIndexer(ctx, &IndexerConfig{
				AddBatchSize:      100,
				ConnectionTimeout: 10000,
				EmbeddingConfig: EmbeddingConfig{
					UseBuiltin: true,
				},
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(i, convey.ShouldNotBeNil)
		})
	})
}

func TestBuiltinEmbedding(t *testing.T) {
	PatchConvey("test builtinEmbedding", t, func() {
		ctx := context.Background()
		svc := &vikingdb.VikingDBService{}
		idx := &Indexer{
			service: svc,
			embModel: &vikingdb.EmbModel{
				ModelName: "qwe",
				Params: map[string]interface{}{
					vikingEmbeddingUseDense:  true,
					vikingEmbeddingUseSparse: true,
				},
			},
			config: &IndexerConfig{
				EmbeddingConfig: EmbeddingConfig{
					UseBuiltin: true,
					ModelName:  "qwe",
					UseSparse:  true,
				},
			},
		}

		queries := []string{"asd", "qwe"}

		PatchConvey("test EmbeddingV2 failed", func() {
			Mock(GetMethod(svc, "EmbeddingV2")).Return(nil, fmt.Errorf("mock err")).Build()
			dense, sparse, err := idx.builtinEmbedding(ctx, queries, nil)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(sparse, convey.ShouldBeNil)
			convey.So(dense, convey.ShouldBeNil)
		})

		PatchConvey("test dense parse error", func() {
			PatchConvey("test key vikingEmbeddingRespSentenceDense not found", func() {
				Mock(GetMethod(svc, "EmbeddingV2")).Return(map[string]interface{}{}, nil).Build()
				dense, sparse, err := idx.builtinEmbedding(ctx, queries, nil)
				convey.So(err, convey.ShouldNotBeNil)
				convey.So(err.Error(), convey.ShouldContainSubstring, "parse dense embedding from result failed")
				convey.So(sparse, convey.ShouldBeNil)
				convey.So(dense, convey.ShouldBeNil)
			})

			PatchConvey("test rawDense not []interface{}", func() {
				Mock(GetMethod(svc, "EmbeddingV2")).Return(map[string]interface{}{
					vikingEmbeddingRespSentenceDense: "asd",
				}, nil).Build()
				dense, sparse, err := idx.builtinEmbedding(ctx, queries, nil)
				convey.So(err, convey.ShouldNotBeNil)
				convey.So(err.Error(), convey.ShouldContainSubstring, "parse dense embedding from result failed")
				convey.So(sparse, convey.ShouldBeNil)
				convey.So(dense, convey.ShouldBeNil)
			})

			PatchConvey("test len(rawDense) != len(queries)", func() {
				v := []interface{}{[]interface{}{0.1}, []interface{}{0.2}, []interface{}{0.3}}
				Mock(GetMethod(svc, "EmbeddingV2")).Return(map[string]interface{}{
					vikingEmbeddingRespSentenceDense: v,
				}, nil).Build()
				dense, sparse, err := idx.builtinEmbedding(ctx, queries, nil)
				convey.So(err, convey.ShouldNotBeNil)
				convey.So(err.Error(), convey.ShouldContainSubstring, "parse dense embedding from result failed")
				convey.So(sparse, convey.ShouldBeNil)
				convey.So(dense, convey.ShouldBeNil)
			})

			PatchConvey("test rawDense item parse to []float64 failed", func() {
				v := []interface{}{[]interface{}{0.1}, "asd"}
				Mock(GetMethod(svc, "EmbeddingV2")).Return(map[string]interface{}{
					vikingEmbeddingRespSentenceDense: v,
				}, nil).Build()
				dense, sparse, err := idx.builtinEmbedding(ctx, queries, nil)
				convey.So(err, convey.ShouldNotBeNil)
				convey.So(err.Error(), convey.ShouldContainSubstring, "conv dense embedding item failed")
				convey.So(sparse, convey.ShouldBeNil)
				convey.So(dense, convey.ShouldBeNil)
			})
		})

		PatchConvey("test sparse parse error", func() {
			dv := []interface{}{
				[]interface{}{0.1},
				[]interface{}{0.2},
			}

			PatchConvey("test key vikingEmbeddingRespSentenceSparse not found", func() {
				Mock(GetMethod(svc, "EmbeddingV2")).Return(map[string]interface{}{
					vikingEmbeddingRespSentenceDense: dv,
				}, nil).Build()
				dense, sparse, err := idx.builtinEmbedding(ctx, queries, nil)
				convey.So(err, convey.ShouldNotBeNil)
				convey.So(err.Error(), convey.ShouldContainSubstring, "parse sparse embedding from result failed")
				convey.So(sparse, convey.ShouldBeNil)
				convey.So(dense, convey.ShouldBeNil)
			})

			PatchConvey("test rawSparse not []interface{}", func() {
				Mock(GetMethod(svc, "EmbeddingV2")).Return(map[string]interface{}{
					vikingEmbeddingRespSentenceDense:  dv,
					vikingEmbeddingRespSentenceSparse: "asd",
				}, nil).Build()
				dense, sparse, err := idx.builtinEmbedding(ctx, queries, nil)
				convey.So(err, convey.ShouldNotBeNil)
				convey.So(err.Error(), convey.ShouldContainSubstring, "parse sparse embedding from result failed")
				convey.So(sparse, convey.ShouldBeNil)
				convey.So(dense, convey.ShouldBeNil)
			})

			PatchConvey("test len(rawSparse) != len(queries)", func() {
				Mock(GetMethod(svc, "EmbeddingV2")).Return(map[string]interface{}{
					vikingEmbeddingRespSentenceDense:  dv,
					vikingEmbeddingRespSentenceSparse: []interface{}{map[string]any{}},
				}, nil).Build()
				dense, sparse, err := idx.builtinEmbedding(ctx, queries, nil)
				convey.So(err, convey.ShouldNotBeNil)
				convey.So(err.Error(), convey.ShouldContainSubstring, "parse sparse embedding from result failed")
				convey.So(sparse, convey.ShouldBeNil)
				convey.So(dense, convey.ShouldBeNil)
			})

			PatchConvey("test item parse to map[string]any failed", func() {
				sv := []interface{}{
					map[string]any{},
					"asd",
				}

				Mock(GetMethod(svc, "EmbeddingV2")).Return(map[string]interface{}{
					vikingEmbeddingRespSentenceDense:  dv,
					vikingEmbeddingRespSentenceSparse: sv,
				}, nil).Build()
				dense, sparse, err := idx.builtinEmbedding(ctx, queries, nil)
				convey.So(err, convey.ShouldNotBeNil)
				convey.So(err.Error(), convey.ShouldContainSubstring, "conv sparse embedding item failed")
				convey.So(sparse, convey.ShouldBeNil)
				convey.So(dense, convey.ShouldBeNil)
			})
		})

		PatchConvey("test success", func() {
			dv := []interface{}{[]interface{}{0.1}, []interface{}{0.2}}
			sv := []interface{}{map[string]any{}, map[string]any{}}

			Mock(GetMethod(svc, "EmbeddingV2")).Return(map[string]interface{}{
				vikingEmbeddingRespSentenceDense:  dv,
				vikingEmbeddingRespSentenceSparse: sv,
			}, nil).Build()
			dense, sparse, err := idx.builtinEmbedding(ctx, queries, nil)
			convey.So(err, convey.ShouldBeNil)
			convey.So(sparse, convey.ShouldEqual, []map[string]any{{}, {}})
			convey.So(dense, convey.ShouldEqual, [][]float64{{0.1}, {0.2}})
		})
	})
}

func TestCustomEmbedding(t *testing.T) {
	PatchConvey("test customEmbedding", t, func() {
		ctx := context.Background()
		emb := &mockEmbedding{}
		idx := &Indexer{
			config: &IndexerConfig{
				EmbeddingConfig: EmbeddingConfig{
					UseBuiltin: false,
					Embedding:  emb,
				},
			},
		}

		queries := []string{"asd", "qwe"}
		options := &indexer.Options{Embedding: emb}

		PatchConvey("test EmbedStrings error", func() {
			Mock(GetMethod(emb, "EmbedStrings")).Return(nil, fmt.Errorf("mock err")).Build()
			resp, err := idx.customEmbedding(ctx, queries, options)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "mock err")
			convey.So(resp, convey.ShouldBeNil)

		})

		PatchConvey("test vector size incorrect", func() {
			q := []string{"asd"}
			resp, err := idx.customEmbedding(ctx, q, options)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "invalid return length of vector")
			convey.So(resp, convey.ShouldBeNil)
		})

		PatchConvey("test success", func() {
			resp, err := idx.customEmbedding(ctx, queries, options)
			convey.So(err, convey.ShouldBeNil)
			convey.So(resp, convey.ShouldNotBeNil)
		})
	})
}

func TestConvertDocuments(t *testing.T) {
	PatchConvey("test convertDocuments", t, func() {
		ctx := context.Background()
		d1 := &schema.Document{ID: "1", Content: "asd"}
		d2 := &schema.Document{ID: "2", Content: "qwe", MetaData: map[string]any{
			extraKeyVikingDBFields: map[string]any{"extra_field_1": "asd"},
			extraKeyVikingDBTTL:    int64(123),
		}}
		docs := []*schema.Document{d1, d2}
		emb := &mockEmbedding{}
		idx := &Indexer{
			config: &IndexerConfig{
				EmbeddingConfig: EmbeddingConfig{
					UseBuiltin: false,
					Embedding:  emb,
				},
			},
		}
		options := &indexer.Options{
			Embedding: emb,
		}

		data, err := idx.convertDocuments(ctx, docs, options)
		convey.So(err, convey.ShouldBeNil)
		convey.So(len(data), convey.ShouldEqual, 2)
		convey.So(data[0].Fields, convey.ShouldEqual, map[string]any{
			defaultFieldID:      d1.ID,
			defaultFieldContent: d1.Content,
			defaultFieldVector:  []float64{1.1, 1.2, 1.3},
		})

		convey.So(data[1].Fields, convey.ShouldEqual, map[string]any{
			defaultFieldID:      d2.ID,
			defaultFieldContent: d2.Content,
			defaultFieldVector:  []float64{2.1, 2.2, 2.3},
			"extra_field_1":     "asd",
		})
		convey.So(data[1].TTL, convey.ShouldEqual, int64(123))
	})
}

type mockEmbedding struct{}

func (m *mockEmbedding) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	return [][]float64{{1.1, 1.2, 1.3}, {2.1, 2.2, 2.3}}, nil
}

func (m *mockEmbedding) GetType() string {
	return "asd"
}

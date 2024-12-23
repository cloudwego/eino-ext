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

func TestNewRetriever(t *testing.T) {
	PatchConvey("test NewRetriever", t, func() {
		ctx := context.Background()

		PatchConvey("test embedding set error", func() {
			ret, err := NewRetriever(ctx, &RetrieverConfig{
				EmbeddingConfig: EmbeddingConfig{UseBuiltin: true, Embedding: &mockEmbedding{fn: func() ([][]float64, error) {
					return [][]float64{{1.1, 1.2}}, nil
				}}},
			})
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(ret, convey.ShouldBeNil)

			ret, err = NewRetriever(ctx, &RetrieverConfig{
				EmbeddingConfig: EmbeddingConfig{UseBuiltin: false, Embedding: nil},
			})
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(ret, convey.ShouldBeNil)
		})

		PatchConvey("test GetIndex error", func() {
			svc := &vikingdb.VikingDBService{}
			Mock(vikingdb.NewVikingDBService).Return(svc).Build()
			Mock(GetMethod(svc, "GetIndex")).Return(nil, fmt.Errorf("mock err")).Build()

			ret, err := NewRetriever(ctx, &RetrieverConfig{
				EmbeddingConfig: EmbeddingConfig{UseBuiltin: true, UseSparse: true},
			})
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(ret, convey.ShouldBeNil)
		})

		PatchConvey("test success", func() {
			svc := &vikingdb.VikingDBService{}
			Mock(vikingdb.NewVikingDBService).Return(svc).Build()
			Mock(GetMethod(svc, "GetIndex")).Return(&vikingdb.Index{}, nil).Build()

			ret, err := NewRetriever(ctx, &RetrieverConfig{
				EmbeddingConfig: EmbeddingConfig{UseBuiltin: true, UseSparse: true},
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(ret, convey.ShouldNotBeNil)
		})
	})
}

func TestBuiltinEmbedding(t *testing.T) {
	PatchConvey("test builtinEmbedding", t, func() {
		ctx := context.Background()
		svc := &vikingdb.VikingDBService{}
		idx := &vikingdb.Index{}
		r := &Retriever{
			config: &RetrieverConfig{
				EmbeddingConfig: EmbeddingConfig{
					UseBuiltin: true,
					UseSparse:  true,
				},
			},
			service: svc,
			index:   idx,
			embModel: &vikingdb.EmbModel{
				ModelName: "asd",
				Params: map[string]interface{}{
					vikingEmbeddingUseDense:  true,
					vikingEmbeddingUseSparse: true,
				},
			},
		}
		query := "asd"

		PatchConvey("test EmbeddingV2 error", func() {
			Mock(GetMethod(svc, "EmbeddingV2")).Return(nil, fmt.Errorf("mock err")).Build()

			dense, sparse, err := r.builtinEmbedding(ctx, query, nil)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "mock err")
			convey.So(sparse, convey.ShouldBeNil)
			convey.So(dense, convey.ShouldBeNil)
		})

		PatchConvey("test dense parse error", func() {
			PatchConvey("test key vikingEmbeddingRespSentenceDense not found", func() {
				Mock(GetMethod(svc, "EmbeddingV2")).Return(map[string]interface{}{}, nil).Build()
				dense, sparse, err := r.builtinEmbedding(ctx, query, nil)
				convey.So(err, convey.ShouldNotBeNil)
				convey.So(err.Error(), convey.ShouldContainSubstring, "parse dense embedding from result failed")
				convey.So(sparse, convey.ShouldBeNil)
				convey.So(dense, convey.ShouldBeNil)
			})

			PatchConvey("test rawDense not []interface{}", func() {
				Mock(GetMethod(svc, "EmbeddingV2")).Return(map[string]interface{}{
					vikingEmbeddingRespSentenceDense: "asd",
				}, nil).Build()
				dense, sparse, err := r.builtinEmbedding(ctx, query, nil)
				convey.So(err, convey.ShouldNotBeNil)
				convey.So(err.Error(), convey.ShouldContainSubstring, "parse dense embedding from result failed")
				convey.So(sparse, convey.ShouldBeNil)
				convey.So(dense, convey.ShouldBeNil)
			})

			PatchConvey("test len(rawDense) == 0", func() {
				var v []interface{}
				Mock(GetMethod(svc, "EmbeddingV2")).Return(map[string]interface{}{
					vikingEmbeddingRespSentenceDense: v,
				}, nil).Build()
				dense, sparse, err := r.builtinEmbedding(ctx, query, nil)
				convey.So(err, convey.ShouldNotBeNil)
				convey.So(err.Error(), convey.ShouldContainSubstring, "parse dense embedding from result failed")
				convey.So(sparse, convey.ShouldBeNil)
				convey.So(dense, convey.ShouldBeNil)
			})

			PatchConvey("test rawDense item parse to []float64 failed", func() {
				v := []interface{}{[]interface{}{0.1, "asd"}}
				Mock(GetMethod(svc, "EmbeddingV2")).Return(map[string]interface{}{
					vikingEmbeddingRespSentenceDense: v,
				}, nil).Build()
				dense, sparse, err := r.builtinEmbedding(ctx, query, nil)
				convey.So(err, convey.ShouldNotBeNil)
				convey.So(err.Error(), convey.ShouldContainSubstring, "parse dense embedding first item failed")
				convey.So(sparse, convey.ShouldBeNil)
				convey.So(dense, convey.ShouldBeNil)
			})
		})

		PatchConvey("test sparse parse error", func() {
			dv := []interface{}{[]interface{}{0.1, 0.2}}

			PatchConvey("test key vikingEmbeddingRespSentenceSparse not found", func() {
				Mock(GetMethod(svc, "EmbeddingV2")).Return(map[string]interface{}{
					vikingEmbeddingRespSentenceDense: dv,
				}, nil).Build()
				dense, sparse, err := r.builtinEmbedding(ctx, query, nil)
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
				dense, sparse, err := r.builtinEmbedding(ctx, query, nil)
				convey.So(err, convey.ShouldNotBeNil)
				convey.So(err.Error(), convey.ShouldContainSubstring, "parse sparse embedding from result failed")
				convey.So(sparse, convey.ShouldBeNil)
				convey.So(dense, convey.ShouldBeNil)
			})

			PatchConvey("test len(rawSparse) == 0", func() {
				Mock(GetMethod(svc, "EmbeddingV2")).Return(map[string]interface{}{
					vikingEmbeddingRespSentenceDense:  dv,
					vikingEmbeddingRespSentenceSparse: []interface{}{},
				}, nil).Build()
				dense, sparse, err := r.builtinEmbedding(ctx, query, nil)
				convey.So(err, convey.ShouldNotBeNil)
				convey.So(err.Error(), convey.ShouldContainSubstring, "parse sparse embedding from result failed")
				convey.So(sparse, convey.ShouldBeNil)
				convey.So(dense, convey.ShouldBeNil)
			})

			PatchConvey("test item parse to map[string]any failed", func() {
				sv := []interface{}{"asd"}
				Mock(GetMethod(svc, "EmbeddingV2")).Return(map[string]interface{}{
					vikingEmbeddingRespSentenceDense:  dv,
					vikingEmbeddingRespSentenceSparse: sv,
				}, nil).Build()
				dense, sparse, err := r.builtinEmbedding(ctx, query, nil)
				convey.So(err, convey.ShouldNotBeNil)
				convey.So(err.Error(), convey.ShouldContainSubstring, "parse dense embedding first item failed")
				convey.So(sparse, convey.ShouldBeNil)
				convey.So(dense, convey.ShouldBeNil)
			})
		})

		PatchConvey("test success", func() {
			dv := []interface{}{[]interface{}{0.1, 0.2}}
			sv := []interface{}{map[string]interface{}{"__1.": 0.174072265625}}

			Mock(GetMethod(svc, "EmbeddingV2")).Return(map[string]interface{}{
				vikingEmbeddingRespSentenceDense:  dv,
				vikingEmbeddingRespSentenceSparse: sv,
			}, nil).Build()

			dense, sparse, err := r.builtinEmbedding(ctx, query, nil)
			convey.So(err, convey.ShouldBeNil)
			convey.So(sparse, convey.ShouldNotBeNil)
			convey.So(sparse, convey.ShouldEqual, map[string]interface{}{"__1.": 0.174072265625})
			convey.So(dense, convey.ShouldNotBeNil)
			convey.So(dense, convey.ShouldEqual, []float64{0.1, 0.2})
		})
	})
}

func TestCustomEmbedding(t *testing.T) {
	PatchConvey("test customEmbedding", t, func() {
		ctx := context.Background()
		r := &Retriever{}
		query := "asd"

		PatchConvey("test EmbedStrings failed", func() {
			emb := &mockEmbedding{fn: func() ([][]float64, error) {
				return nil, fmt.Errorf("mock err")
			}}
			options := &retriever.Options{Embedding: emb}

			v, err := r.customEmbedding(ctx, query, options)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(len(v), convey.ShouldEqual, 0)
		})

		PatchConvey("test vector size incorrect", func() {
			emb := &mockEmbedding{fn: func() ([][]float64, error) {
				return [][]float64{{1.1, 1.2}, {2.1, 2.2}}, nil
			}}
			options := &retriever.Options{Embedding: emb}

			v, err := r.customEmbedding(ctx, query, options)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(len(v), convey.ShouldEqual, 0)
		})

		PatchConvey("test success", func() {
			emb := &mockEmbedding{fn: func() ([][]float64, error) {
				return [][]float64{{1.1, 1.2}}, nil
			}}
			options := &retriever.Options{Embedding: emb}

			v, err := r.customEmbedding(ctx, query, options)
			convey.So(err, convey.ShouldBeNil)
			convey.So(len(v), convey.ShouldEqual, 2)
		})
	})
}

func TestMakeSearchOption(t *testing.T) {
	PatchConvey("test makeSearchOption", t, func() {
		r := &Retriever{config: &RetrieverConfig{EmbeddingConfig: EmbeddingConfig{DenseWeight: 0.5}}}
		searchOptions := r.makeSearchOption(map[string]interface{}{"__1.": 0.174072265625}, &retriever.Options{
			SubIndex: of("asd"),
			TopK:     of(123),
			DSLInfo:  map[string]interface{}{"asd": 123},
		})

		convey.So(searchOptions, convey.ShouldNotBeNil)
	})
}

func TestData2Document(t *testing.T) {
	PatchConvey("test data2Document", t, func() {
		r := &Retriever{}

		PatchConvey("test content not found", func() {
			fields := map[string]interface{}{
				"ID":            "asd",
				"extra_field_1": 123,
			}

			data := &vikingdb.Data{
				Id:        "asd",
				Fields:    fields,
				Timestamp: nil,
				TTL:       1000,
				Score:     0.2,
			}

			doc, err := r.data2Document(data)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(doc, convey.ShouldBeNil)
		})

		PatchConvey("test success", func() {
			fields := map[string]interface{}{
				"ID":            "asd",
				"content":       "vvv",
				"extra_field_1": 123,
			}

			data := &vikingdb.Data{
				Id:        "asd",
				Fields:    fields,
				Timestamp: nil,
				TTL:       1000,
				Score:     0.2,
			}

			doc, err := r.data2Document(data)
			convey.So(err, convey.ShouldBeNil)
			convey.So(doc, convey.ShouldEqual, &schema.Document{
				ID:      data.Id.(string),
				Content: "vvv",
				MetaData: map[string]any{
					ExtraKeyVikingDBFields: fields,
					ExtraKeyVikingDBTTL:    int64(1000),
					"_score":               data.Score,
				},
			})
		})
	})
}

type mockEmbedding struct {
	fn func() ([][]float64, error)
}

func (m *mockEmbedding) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	return m.fn()
}

func (m *mockEmbedding) GetType() string {
	return "asd"
}

func of[T any](v T) *T {
	return &v
}

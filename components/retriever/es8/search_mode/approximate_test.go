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

package search_mode

import (
	"context"
	"encoding/json"
	"testing"

	. "github.com/bytedance/mockey"
	"github.com/cloudwego/eino-ext/components/retriever/es8"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"github.com/smartystreets/goconvey/convey"
)

func TestSearchModeApproximate(t *testing.T) {
	PatchConvey("test SearchModeApproximate", t, func() {
		PatchConvey("test BuildRequest", func() {
			ctx := context.Background()
			queryFieldName := "eino_doc_content"
			vectorFieldName := "vector_eino_doc_content"
			query := "content"

			PatchConvey("test QueryVectorBuilderModelID", func() {
				a := &approximate{config: &ApproximateConfig{
					QueryFieldName:            queryFieldName,
					VectorFieldName:           vectorFieldName,
					Hybrid:                    false,
					RRF:                       false,
					RRFRankConstant:           nil,
					RRFWindowSize:             nil,
					QueryVectorBuilderModelID: ptrWithoutZero("mock_model"),
					Boost:                     ptrWithoutZero(float32(1.0)),
					K:                         ptrWithoutZero(10),
					NumCandidates:             ptrWithoutZero(100),
					Similarity:                ptrWithoutZero(float32(0.5)),
				}}

				conf := &es8.RetrieverConfig{}
				req, err := a.BuildRequest(ctx, conf, query,
					retriever.WithEmbedding(nil),
					es8.WithFilters([]types.Query{
						{Match: map[string]types.MatchQuery{"label": {Query: "good"}}},
					}))
				convey.So(err, convey.ShouldBeNil)
				b, err := json.Marshal(req)
				convey.So(err, convey.ShouldBeNil)
				convey.So(string(b), convey.ShouldEqual, `{"knn":[{"boost":1,"field":"vector_eino_doc_content","filter":[{"match":{"label":{"query":"good"}}}],"k":10,"num_candidates":100,"query_vector_builder":{"text_embedding":{"model_id":"mock_model","model_text":"content"}},"similarity":0.5}]}`)
			})

			PatchConvey("test embedding", func() {
				a := &approximate{config: &ApproximateConfig{
					QueryFieldName:            queryFieldName,
					VectorFieldName:           vectorFieldName,
					Hybrid:                    false,
					RRF:                       false,
					RRFRankConstant:           nil,
					RRFWindowSize:             nil,
					QueryVectorBuilderModelID: nil,
					Boost:                     ptrWithoutZero(float32(1.0)),
					K:                         ptrWithoutZero(10),
					NumCandidates:             ptrWithoutZero(100),
					Similarity:                ptrWithoutZero(float32(0.5)),
				}}

				conf := &es8.RetrieverConfig{}
				req, err := a.BuildRequest(ctx, conf, query,
					retriever.WithEmbedding(&mockEmbedding{size: 1, mockVector: []float64{1.1, 1.2}}),
					es8.WithFilters([]types.Query{
						{Match: map[string]types.MatchQuery{"label": {Query: "good"}}},
					}))
				convey.So(err, convey.ShouldBeNil)
				b, err := json.Marshal(req)
				convey.So(err, convey.ShouldBeNil)
				convey.So(string(b), convey.ShouldEqual, `{"knn":[{"boost":1,"field":"vector_eino_doc_content","filter":[{"match":{"label":{"query":"good"}}}],"k":10,"num_candidates":100,"query_vector":[1.1,1.2],"similarity":0.5}]}`)
			})

			PatchConvey("test hybrid with rrf", func() {
				a := &approximate{config: &ApproximateConfig{
					QueryFieldName:            queryFieldName,
					VectorFieldName:           vectorFieldName,
					Hybrid:                    true,
					RRF:                       true,
					RRFRankConstant:           ptrWithoutZero(int64(10)),
					RRFWindowSize:             ptrWithoutZero(int64(5)),
					QueryVectorBuilderModelID: nil,
					Boost:                     ptrWithoutZero(float32(1.0)),
					K:                         ptrWithoutZero(10),
					NumCandidates:             ptrWithoutZero(100),
					Similarity:                ptrWithoutZero(float32(0.5)),
				}}

				conf := &es8.RetrieverConfig{}
				req, err := a.BuildRequest(ctx, conf, query,
					retriever.WithEmbedding(&mockEmbedding{size: 1, mockVector: []float64{1.1, 1.2}}),
					retriever.WithTopK(10),
					retriever.WithScoreThreshold(1.1),
					es8.WithFilters([]types.Query{
						{Match: map[string]types.MatchQuery{"label": {Query: "good"}}},
					}))
				convey.So(err, convey.ShouldBeNil)
				b, err := json.Marshal(req)
				convey.So(err, convey.ShouldBeNil)
				convey.So(string(b), convey.ShouldEqual, `{"knn":[{"boost":1,"field":"vector_eino_doc_content","filter":[{"match":{"label":{"query":"good"}}}],"k":10,"num_candidates":100,"query_vector":[1.1,1.2],"similarity":0.5}],"min_score":1.1,"query":{"bool":{"filter":[{"match":{"label":{"query":"good"}}}],"must":[{"match":{"eino_doc_content":{"query":"content"}}}]}},"rank":{"rrf":{"rank_constant":10,"rank_window_size":5}},"size":10}`)
			})
		})
	})
}

type mockEmbedding struct {
	size       int
	mockVector []float64
}

func (m mockEmbedding) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	resp := make([][]float64, m.size)
	for i := range resp {
		resp[i] = m.mockVector
	}

	return resp, nil
}

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
	"fmt"
	"testing"

	. "github.com/bytedance/mockey"
	"github.com/cloudwego/eino-ext/components/retriever/es8"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"github.com/smartystreets/goconvey/convey"
)

func TestSearchModeDenseVectorSimilarity(t *testing.T) {
	PatchConvey("test SearchModeDenseVectorSimilarity", t, func() {
		PatchConvey("test BuildRequest", func() {
			ctx := context.Background()
			vectorFieldName := "vector_eino_doc_content"
			d := SearchModeDenseVectorSimilarity(DenseVectorSimilarityTypeCosineSimilarity, vectorFieldName)
			query := "content"

			PatchConvey("test embedding not provided", func() {
				conf := &es8.RetrieverConfig{}
				req, err := d.BuildRequest(ctx, conf, query, retriever.WithEmbedding(nil))
				convey.So(err, convey.ShouldBeError, "[BuildRequest][SearchModeDenseVectorSimilarity] embedding not provided")
				convey.So(req, convey.ShouldBeNil)
			})

			PatchConvey("test vector size invalid", func() {
				conf := &es8.RetrieverConfig{}
				req, err := d.BuildRequest(ctx, conf, query, retriever.WithEmbedding(mockEmbedding{size: 2, mockVector: []float64{1.1, 1.2}}))
				convey.So(err, convey.ShouldBeError, "[BuildRequest][SearchModeDenseVectorSimilarity] vector size invalid, expect=1, got=2")
				convey.So(req, convey.ShouldBeNil)
			})

			PatchConvey("test success", func() {
				typ2Exp := map[DenseVectorSimilarityType]string{
					DenseVectorSimilarityTypeCosineSimilarity: `{"min_score":1.1,"query":{"script_score":{"query":{"bool":{"filter":[{"match":{"label":{"query":"good"}}}]}},"script":{"params":{"embedding":[1.1,1.2]},"source":"cosineSimilarity(params.embedding, 'vector_eino_doc_content') + 1.0"}}},"size":10}`,
					DenseVectorSimilarityTypeDotProduct:       `{"min_score":1.1,"query":{"script_score":{"query":{"bool":{"filter":[{"match":{"label":{"query":"good"}}}]}},"script":{"params":{"embedding":[1.1,1.2]},"source":"\n    double value = dotProduct(params.embedding, 'vector_eino_doc_content');\n    return sigmoid(1, Math.E, -value);\n    "}}},"size":10}`,
					DenseVectorSimilarityTypeL1Norm:           `{"min_score":1.1,"query":{"script_score":{"query":{"bool":{"filter":[{"match":{"label":{"query":"good"}}}]}},"script":{"params":{"embedding":[1.1,1.2]},"source":"1 / (1 + l1norm(params.embedding, 'vector_eino_doc_content'))"}}},"size":10}`,
					DenseVectorSimilarityTypeL2Norm:           `{"min_score":1.1,"query":{"script_score":{"query":{"bool":{"filter":[{"match":{"label":{"query":"good"}}}]}},"script":{"params":{"embedding":[1.1,1.2]},"source":"1 / (1 + l2norm(params.embedding, 'vector_eino_doc_content'))"}}},"size":10}`,
				}

				for typ, exp := range typ2Exp {
					similarity := SearchModeDenseVectorSimilarity(typ, vectorFieldName)

					conf := &es8.RetrieverConfig{}
					req, err := similarity.BuildRequest(ctx, conf, query, retriever.WithEmbedding(&mockEmbedding{size: 1, mockVector: []float64{1.1, 1.2}}),
						retriever.WithTopK(10),
						retriever.WithScoreThreshold(1.1),
						es8.WithFilters([]types.Query{
							{Match: map[string]types.MatchQuery{"label": {Query: "good"}}},
						}))

					convey.So(err, convey.ShouldBeNil)
					b, err := json.Marshal(req)
					convey.So(err, convey.ShouldBeNil)
					fmt.Println(string(b))
					convey.So(string(b), convey.ShouldEqual, exp)
				}
			})
		})
	})
}

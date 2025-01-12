package search_mode

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	. "github.com/bytedance/mockey"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"github.com/smartystreets/goconvey/convey"

	"github.com/cloudwego/eino-ext/components/retriever/es8/field_mapping"
	"github.com/cloudwego/eino/components/retriever"
)

func TestSearchModeDenseVectorSimilarity(t *testing.T) {
	PatchConvey("test SearchModeDenseVectorSimilarity", t, func() {
		PatchConvey("test ToRetrieverQuery", func() {
			dq := &DenseVectorSimilarityQuery{
				FieldKV: field_mapping.FieldKV{
					FieldNameVector: field_mapping.GetDefaultVectorFieldKeyContent(),
					FieldName:       field_mapping.DocFieldNameContent,
					Value:           "content",
				},
				Filters: []types.Query{
					{Match: map[string]types.MatchQuery{"label": {Query: "good"}}},
				},
			}

			sq, err := dq.ToRetrieverQuery()
			convey.So(err, convey.ShouldBeNil)
			convey.So(sq, convey.ShouldEqual, `{"field_kv":{"field_name_vector":"vector_eino_doc_content","field_name":"eino_doc_content","value":"content"},"filters":[{"match":{"label":{"query":"good"}}}]}`)
		})

		PatchConvey("test BuildRequest", func() {
			ctx := context.Background()
			d := &denseVectorSimilarity{script: denseVectorScriptMap[DenseVectorSimilarityTypeCosineSimilarity]}
			dq := &DenseVectorSimilarityQuery{
				FieldKV: field_mapping.FieldKV{
					FieldNameVector: field_mapping.GetDefaultVectorFieldKeyContent(),
					FieldName:       field_mapping.DocFieldNameContent,
					Value:           "content",
				},
				Filters: []types.Query{
					{Match: map[string]types.MatchQuery{"label": {Query: "good"}}},
				},
			}
			sq, _ := dq.ToRetrieverQuery()

			PatchConvey("test embedding not provided", func() {
				req, err := d.BuildRequest(ctx, sq, &retriever.Options{Embedding: nil})
				convey.So(err, convey.ShouldBeError, "[BuildRequest][SearchModeDenseVectorSimilarity] embedding not provided")
				convey.So(req, convey.ShouldBeNil)
			})

			PatchConvey("test vector size invalid", func() {
				req, err := d.BuildRequest(ctx, sq, &retriever.Options{Embedding: mockEmbedding{size: 2, mockVector: []float64{1.1, 1.2}}})
				convey.So(err, convey.ShouldBeError, "[BuildRequest][SearchModeDenseVectorSimilarity] vector size invalid, expect=1, got=2")
				convey.So(req, convey.ShouldBeNil)
			})

			PatchConvey("test success", func() {
				typ2Exp := map[DenseVectorSimilarityType]string{
					DenseVectorSimilarityTypeCosineSimilarity: `{"min_score":1.1,"query":{"script_score":{"query":{"bool":{"filter":[{"match":{"label":{"query":"good"}}}]}},"script":{"params":{"embedding":[1.1,1.2]},"source":"cosineSimilarity(params.embedding, 'vector_eino_doc_content') + 1.0"}}},"size":10}`,
					DenseVectorSimilarityTypeDotProduct:       `{"min_score":1.1,"query":{"script_score":{"query":{"bool":{"filter":[{"match":{"label":{"query":"good"}}}]}},"script":{"params":{"embedding":[1.1,1.2]},"source":"\"\"\n          double value = dotProduct(params.embedding, 'vector_eino_doc_content');\n          return sigmoid(1, Math.E, -value); \n        \"\""}}},"size":10}`,
					DenseVectorSimilarityTypeL1Norm:           `{"min_score":1.1,"query":{"script_score":{"query":{"bool":{"filter":[{"match":{"label":{"query":"good"}}}]}},"script":{"params":{"embedding":[1.1,1.2]},"source":"1 / (1 + l1norm(params.embedding, 'vector_eino_doc_content'))"}}},"size":10}`,
					DenseVectorSimilarityTypeL2Norm:           `{"min_score":1.1,"query":{"script_score":{"query":{"bool":{"filter":[{"match":{"label":{"query":"good"}}}]}},"script":{"params":{"embedding":[1.1,1.2]},"source":"1 / (1 + l2norm(params.embedding, 'vector_eino_doc_content'))"}}},"size":10}`,
				}

				for typ, exp := range typ2Exp {
					nd := &denseVectorSimilarity{script: denseVectorScriptMap[typ]}
					req, err := nd.BuildRequest(ctx, sq, &retriever.Options{
						Embedding:      mockEmbedding{size: 1, mockVector: []float64{1.1, 1.2}},
						TopK:           of(10),
						ScoreThreshold: of(1.1),
					})
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

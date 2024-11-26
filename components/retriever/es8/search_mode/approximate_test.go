package search_mode

import (
	"context"
	"encoding/json"
	"testing"

	. "github.com/bytedance/mockey"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"github.com/smartystreets/goconvey/convey"

	"code.byted.org/flow/eino-ext/components/retriever/es8/field_mapping"
	"code.byted.org/flow/eino/components/embedding"
	"code.byted.org/flow/eino/components/retriever"
)

func TestSearchModeApproximate(t *testing.T) {
	PatchConvey("test SearchModeApproximate", t, func() {
		PatchConvey("test ToRetrieverQuery", func() {
			aq := &ApproximateQuery{
				FieldKV: field_mapping.FieldKV{
					FieldNameVector: field_mapping.GetDefaultVectorFieldKeyContent(),
					FieldName:       field_mapping.DocFieldNameContent,
					Value:           "content",
				},
				Filters: []types.Query{
					{Match: map[string]types.MatchQuery{"label": {Query: "good"}}},
				},
				Boost:         of(float32(1.0)),
				K:             of(10),
				NumCandidates: of(100),
				Similarity:    of(float32(0.5)),
			}

			sq, err := aq.ToRetrieverQuery()
			convey.So(err, convey.ShouldBeNil)
			convey.So(sq, convey.ShouldEqual, `{"field_kv":{"field_name_vector":"vector_eino_doc_content","field_name":"eino_doc_content","value":"content"},"boost":1,"filters":[{"match":{"label":{"query":"good"}}}],"k":10,"num_candidates":100,"similarity":0.5}`)
		})

		PatchConvey("test BuildRequest", func() {
			ctx := context.Background()

			PatchConvey("test QueryVectorBuilderModelID", func() {
				a := &approximate{config: &ApproximateConfig{}}
				aq := &ApproximateQuery{
					FieldKV: field_mapping.FieldKV{
						FieldNameVector: field_mapping.GetDefaultVectorFieldKeyContent(),
						FieldName:       field_mapping.DocFieldNameContent,
						Value:           "content",
					},
					QueryVectorBuilderModelID: of("mock_model"),
					Filters: []types.Query{
						{Match: map[string]types.MatchQuery{"label": {Query: "good"}}},
					},
					Boost:         of(float32(1.0)),
					K:             of(10),
					NumCandidates: of(100),
					Similarity:    of(float32(0.5)),
				}

				sq, err := aq.ToRetrieverQuery()
				convey.So(err, convey.ShouldBeNil)

				req, err := a.BuildRequest(ctx, sq, &retriever.Options{Embedding: nil})
				convey.So(err, convey.ShouldBeNil)
				b, err := json.Marshal(req)
				convey.So(err, convey.ShouldBeNil)
				convey.So(string(b), convey.ShouldEqual, `{"knn":[{"boost":1,"field":"vector_eino_doc_content","filter":[{"match":{"label":{"query":"good"}}}],"k":10,"num_candidates":100,"query_vector_builder":{"text_embedding":{"model_id":"mock_model","model_text":"content"}},"similarity":0.5}]}`)
			})

			PatchConvey("test embedding", func() {
				a := &approximate{config: &ApproximateConfig{}}
				aq := &ApproximateQuery{
					FieldKV: field_mapping.FieldKV{
						FieldNameVector: field_mapping.GetDefaultVectorFieldKeyContent(),
						FieldName:       field_mapping.DocFieldNameContent,
						Value:           "content",
					},
					Filters: []types.Query{
						{Match: map[string]types.MatchQuery{"label": {Query: "good"}}},
					},
					Boost:         of(float32(1.0)),
					K:             of(10),
					NumCandidates: of(100),
					Similarity:    of(float32(0.5)),
				}

				sq, err := aq.ToRetrieverQuery()
				convey.So(err, convey.ShouldBeNil)
				req, err := a.BuildRequest(ctx, sq, &retriever.Options{Embedding: &mockEmbedding{size: 1, mockVector: []float64{1.1, 1.2}}})
				convey.So(err, convey.ShouldBeNil)
				b, err := json.Marshal(req)
				convey.So(err, convey.ShouldBeNil)
				convey.So(string(b), convey.ShouldEqual, `{"knn":[{"boost":1,"field":"vector_eino_doc_content","filter":[{"match":{"label":{"query":"good"}}}],"k":10,"num_candidates":100,"query_vector":[1.1,1.2],"similarity":0.5}]}`)
			})

			PatchConvey("test hybrid with rrf", func() {
				a := &approximate{config: &ApproximateConfig{
					Hybrid:          true,
					Rrf:             true,
					RrfRankConstant: of(int64(10)),
					RrfWindowSize:   of(int64(5)),
				}}

				aq := &ApproximateQuery{
					FieldKV: field_mapping.FieldKV{
						FieldNameVector: field_mapping.GetDefaultVectorFieldKeyContent(),
						FieldName:       field_mapping.DocFieldNameContent,
						Value:           "content",
					},
					Filters: []types.Query{
						{Match: map[string]types.MatchQuery{"label": {Query: "good"}}},
					},
					Boost:         of(float32(1.0)),
					K:             of(10),
					NumCandidates: of(100),
					Similarity:    of(float32(0.5)),
				}

				sq, err := aq.ToRetrieverQuery()
				convey.So(err, convey.ShouldBeNil)
				req, err := a.BuildRequest(ctx, sq, &retriever.Options{
					Embedding:      &mockEmbedding{size: 1, mockVector: []float64{1.1, 1.2}},
					TopK:           of(10),
					ScoreThreshold: of(1.1),
				})
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

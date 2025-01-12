package search_mode

import (
	"context"
	"encoding/json"
	"testing"

	. "github.com/bytedance/mockey"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"github.com/smartystreets/goconvey/convey"

	"github.com/cloudwego/eino-ext/components/retriever/es8/field_mapping"
	"github.com/cloudwego/eino/components/retriever"
)

func TestSearchModeSparseVectorTextExpansion(t *testing.T) {
	PatchConvey("test SearchModeSparseVectorTextExpansion", t, func() {
		PatchConvey("test ToRetrieverQuery", func() {
			sq := &SparseVectorTextExpansionQuery{
				FieldKV: field_mapping.FieldKV{
					FieldNameVector: field_mapping.GetDefaultVectorFieldKeyContent(),
					FieldName:       field_mapping.DocFieldNameContent,
					Value:           "content",
				},
				Filters: []types.Query{
					{Match: map[string]types.MatchQuery{"label": {Query: "good"}}},
				},
			}

			ssq, err := sq.ToRetrieverQuery()
			convey.So(err, convey.ShouldBeNil)
			convey.So(ssq, convey.ShouldEqual, `{"field_kv":{"field_name_vector":"vector_eino_doc_content","field_name":"eino_doc_content","value":"content"},"filters":[{"match":{"label":{"query":"good"}}}]}`)

		})

		PatchConvey("test BuildRequest", func() {
			ctx := context.Background()
			s := SearchModeSparseVectorTextExpansion("mock_model_id")
			sq := &SparseVectorTextExpansionQuery{
				FieldKV: field_mapping.FieldKV{
					FieldNameVector: field_mapping.GetDefaultVectorFieldKeyContent(),
					FieldName:       field_mapping.DocFieldNameContent,
					Value:           "content",
				},
				Filters: []types.Query{
					{Match: map[string]types.MatchQuery{"label": {Query: "good"}}},
				},
			}

			query, _ := sq.ToRetrieverQuery()
			req, err := s.BuildRequest(ctx, query, &retriever.Options{TopK: of(10), ScoreThreshold: of(1.1)})
			convey.So(err, convey.ShouldBeNil)
			convey.So(req, convey.ShouldNotBeNil)
			b, err := json.Marshal(req)
			convey.So(err, convey.ShouldBeNil)
			convey.So(string(b), convey.ShouldEqual, `{"min_score":1.1,"query":{"bool":{"filter":[{"match":{"label":{"query":"good"}}}],"must":[{"text_expansion":{"vector_eino_doc_content.tokens":{"model_id":"mock_model_id","model_text":"content"}}}]}},"size":10}`)
		})
	})
}

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
	"github.com/cloudwego/eino/components/retriever"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"github.com/smartystreets/goconvey/convey"
)

func TestSearchModeSparseVectorTextExpansion(t *testing.T) {
	PatchConvey("test SearchModeSparseVectorTextExpansion", t, func() {
		PatchConvey("test BuildRequest", func() {
			ctx := context.Background()
			vectorFieldName := "vector_eino_doc_content"
			s := SearchModeSparseVectorTextExpansion("mock_model_id", vectorFieldName)

			conf := &es8.RetrieverConfig{}
			req, err := s.BuildRequest(ctx, conf, "content",
				retriever.WithTopK(10),
				retriever.WithScoreThreshold(1.1),
				es8.WithFilters([]types.Query{
					{Match: map[string]types.MatchQuery{"label": {Query: "good"}}},
				}))

			convey.So(err, convey.ShouldBeNil)
			convey.So(req, convey.ShouldNotBeNil)
			b, err := json.Marshal(req)
			convey.So(err, convey.ShouldBeNil)
			convey.So(string(b), convey.ShouldEqual, `{"min_score":1.1,"query":{"bool":{"filter":[{"match":{"label":{"query":"good"}}}],"must":[{"text_expansion":{"vector_eino_doc_content.tokens":{"model_id":"mock_model_id","model_text":"content"}}}]}},"size":10}`)
		})
	})
}

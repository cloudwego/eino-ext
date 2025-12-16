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

	"github.com/bytedance/mockey"
	"github.com/cloudwego/eino-ext/components/retriever/es7"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/smartystreets/goconvey/convey"
)

type mockEmbedder struct{}

func (m *mockEmbedder) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	return [][]float64{{0.1, 0.2}}, nil
}

func TestSearchModeExactMatch(t *testing.T) {
	mockey.PatchConvey("test SearchModeExactMatch", t, func() {
		ctx := context.Background()
		conf := &es7.RetrieverConfig{}
		searchMode := ExactMatch("test_field")
		req, err := searchMode.BuildRequest(ctx, conf, "test_query")
		convey.So(err, convey.ShouldBeNil)
		b, err := json.Marshal(req)
		convey.So(err, convey.ShouldBeNil)
		// Expected JSON for ES7 (match query)
		convey.So(string(b), convey.ShouldEqual, `{"query":{"match":{"test_field":{"query":"test_query"}}}}`)
	})
}

func TestSearchModeRawStringRequest(t *testing.T) {
	mockey.PatchConvey("test SearchModeRawStringRequest", t, func() {
		ctx := context.Background()
		conf := &es7.RetrieverConfig{}
		searchMode := RawStringRequest()

		mockey.PatchConvey("test from json error", func() {
			r, err := searchMode.BuildRequest(ctx, conf, "test_query")
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(r, convey.ShouldBeNil)
		})

		mockey.PatchConvey("test success", func() {
			q := `{"query":{"match":{"test_field":{"query":"test_query"}}}}`
			r, err := searchMode.BuildRequest(ctx, conf, q)
			convey.So(err, convey.ShouldBeNil)
			convey.So(r, convey.ShouldNotBeNil)

			// Verify structure
			qm, ok := r["query"].(map[string]any)
			convey.So(ok, convey.ShouldBeTrue)
			match, ok := qm["match"].(map[string]any)
			convey.So(ok, convey.ShouldBeTrue)
			field, ok := match["test_field"].(map[string]any)
			convey.So(ok, convey.ShouldBeTrue)
			convey.So(field["query"], convey.ShouldEqual, "test_query")
		})
	})
}

func TestDenseVectorSimilarity(t *testing.T) {
	mockey.PatchConvey("test DenseVectorSimilarity", t, func() {
		ctx := context.Background()
		conf := &es7.RetrieverConfig{}
		conf.Embedding = &mockEmbedder{}

		mockey.PatchConvey("CosineSimilarity", func() {
			searchMode := DenseVectorSimilarity(DenseVectorSimilarityTypeCosineSimilarity, "vector_field")
			req, err := searchMode.BuildRequest(ctx, conf, "test_query")
			convey.So(err, convey.ShouldBeNil)
			convey.So(req, convey.ShouldNotBeNil)

			// Verify query -> script_score -> script -> source
			q := req["query"].(map[string]any)
			ss := q["script_score"].(map[string]any)
			script := ss["script"].(map[string]any)
			source := script["source"].(string)

			// Verify script contains string literal for field name, not doc access
			convey.So(source, convey.ShouldContainSubstring, "cosineSimilarity(params.query_vector, 'vector_field')")
			convey.So(source, convey.ShouldNotContainSubstring, "doc['vector_field']")
		})

		mockey.PatchConvey("DotProduct", func() {
			searchMode := DenseVectorSimilarity(DenseVectorSimilarityTypeDotProduct, "vector_field")
			req, err := searchMode.BuildRequest(ctx, conf, "test_query")
			convey.So(err, convey.ShouldBeNil)

			q := req["query"].(map[string]any)
			ss := q["script_score"].(map[string]any)
			script := ss["script"].(map[string]any)
			source := script["source"].(string)

			convey.So(source, convey.ShouldContainSubstring, "dotProduct(params.query_vector, 'vector_field')")
		})

		mockey.PatchConvey("L1Norm", func() {
			searchMode := DenseVectorSimilarity(DenseVectorSimilarityTypeL1Norm, "vector_field")
			req, err := searchMode.BuildRequest(ctx, conf, "test_query")
			convey.So(err, convey.ShouldBeNil)

			q := req["query"].(map[string]any)
			ss := q["script_score"].(map[string]any)
			script := ss["script"].(map[string]any)
			source := script["source"].(string)

			convey.So(source, convey.ShouldContainSubstring, "l1norm(params.query_vector, 'vector_field')")
		})

		mockey.PatchConvey("L2Norm", func() {
			searchMode := DenseVectorSimilarity(DenseVectorSimilarityTypeL2Norm, "vector_field")
			req, err := searchMode.BuildRequest(ctx, conf, "test_query")
			convey.So(err, convey.ShouldBeNil)

			q := req["query"].(map[string]any)
			ss := q["script_score"].(map[string]any)
			script := ss["script"].(map[string]any)
			source := script["source"].(string)

			convey.So(source, convey.ShouldContainSubstring, "l2norm(params.query_vector, 'vector_field')")
		})

		mockey.PatchConvey("No Embedding", func() {
			conf.Embedding = nil
			searchMode := DenseVectorSimilarity(DenseVectorSimilarityTypeL2Norm, "vector_field")
			_, err := searchMode.BuildRequest(ctx, conf, "test_query")
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "embedding not provided")
		})
	})
}

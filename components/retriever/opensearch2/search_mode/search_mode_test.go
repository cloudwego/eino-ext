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
	"github.com/cloudwego/eino-ext/components/retriever/opensearch2"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/smartystreets/goconvey/convey"
)

func TestSearchModeExactMatch(t *testing.T) {
	PatchConvey("test SearchModeExactMatch", t, func() {
		ctx := context.Background()
		conf := &opensearch2.RetrieverConfig{}
		searchMode := ExactMatch("test_field")
		req, err := searchMode.BuildRequest(ctx, conf, "test_query")
		convey.So(err, convey.ShouldBeNil)
		b, err := json.Marshal(req)
		convey.So(err, convey.ShouldBeNil)
		// Expected JSON for OpenSearch (map based)
		convey.So(string(b), convey.ShouldEqual, `{"query":{"match":{"test_field":{"query":"test_query"}}}}`)
	})
}

func TestSearchModeRawStringRequest(t *testing.T) {
	PatchConvey("test SearchModeRawStringRequest", t, func() {
		ctx := context.Background()
		conf := &opensearch2.RetrieverConfig{}
		searchMode := RawStringRequest()

		PatchConvey("test from json error", func() {
			r, err := searchMode.BuildRequest(ctx, conf, "test_query")
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(r, convey.ShouldBeNil)
		})

		PatchConvey("test success", func() {
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
	PatchConvey("test DenseVectorSimilarity", t, func() {
		ctx := context.Background()
		conf := &opensearch2.RetrieverConfig{}
		conf.Embedding = &MockEmbedder{}

		searchMode := DenseVectorSimilarity(DenseVectorSimilarityTypeCosineSimilarity, "vector")

		PatchConvey("test success", func() {
			req, err := searchMode.BuildRequest(ctx, conf, "test_query")
			convey.So(err, convey.ShouldBeNil)
			convey.So(req, convey.ShouldNotBeNil)

			// Verify structure: query -> script_score -> script -> source
			q, ok := req["query"].(map[string]any)
			convey.So(ok, convey.ShouldBeTrue)
			ss, ok := q["script_score"].(map[string]any)
			convey.So(ok, convey.ShouldBeTrue)
			script, ok := ss["script"].(map[string]any)
			convey.So(ok, convey.ShouldBeTrue)
			source, ok := script["source"].(string)
			convey.So(ok, convey.ShouldBeTrue)
			convey.So(source, convey.ShouldContainSubstring, "cosineSimilarity")
		})
	})
}

type MockEmbedder struct{}

func (m *MockEmbedder) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	return [][]float64{{0.1, 0.2}}, nil
}

func TestApproximate(t *testing.T) {
	PatchConvey("test Approximate", t, func() {
		ctx := context.Background()
		conf := &opensearch2.RetrieverConfig{}
		conf.Embedding = &MockEmbedder{}

		searchMode := Approximate(&ApproximateConfig{
			VectorField: "vector",
			K:           5,
		})

		PatchConvey("test success", func() {
			req, err := searchMode.BuildRequest(ctx, conf, "test_query")
			convey.So(err, convey.ShouldBeNil)
			convey.So(req, convey.ShouldNotBeNil)

			q, ok := req["query"].(map[string]any)
			convey.So(ok, convey.ShouldBeTrue)
			_, ok = q["knn"].(map[string]any)
			convey.So(ok, convey.ShouldBeTrue)

			// Additional checks for Approximate logic if needed
		})
	})
}

func TestNeuralSparse(t *testing.T) {
	PatchConvey("test NeuralSparse", t, func() {
		ctx := context.Background()
		conf := &opensearch2.RetrieverConfig{}

		PatchConvey("test query_text", func() {
			searchMode := NeuralSparse("sparse_vector_field", &NeuralSparseConfig{
				ModelID: "test-model",
			})
			req, err := searchMode.BuildRequest(ctx, conf, "test_query")
			convey.So(err, convey.ShouldBeNil)

			q, ok := req["query"].(map[string]any)
			convey.So(ok, convey.ShouldBeTrue)
			ns := q["neural_sparse"].(map[string]any)
			field := ns["sparse_vector_field"].(map[string]any)
			convey.So(field["query_text"], convey.ShouldEqual, "test_query")
		})

		PatchConvey("test token_weights", func() {
			searchMode := NeuralSparse("sparse_vector_field", &NeuralSparseConfig{
				TokenWeights: map[string]float32{"term1": 1.0, "term2": 0.5},
			})
			req, err := searchMode.BuildRequest(ctx, conf, "test_query")
			convey.So(err, convey.ShouldBeNil)

			q, ok := req["query"].(map[string]any)
			convey.So(ok, convey.ShouldBeTrue)
			ns := q["neural_sparse"].(map[string]any)
			field := ns["sparse_vector_field"].(map[string]any)
			tokens := field["query_tokens"].(map[string]float32)
			convey.So(tokens["term1"], convey.ShouldEqual, 1.0)
		})
	})
}

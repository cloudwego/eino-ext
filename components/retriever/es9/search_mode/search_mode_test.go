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
	"fmt"
	"testing"

	"github.com/cloudwego/eino-ext/components/retriever/es9"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/smartystreets/goconvey/convey"
)

func TestSearchModeExactMatch(t *testing.T) {
	convey.Convey("test SearchModeExactMatch", t, func() {
		ctx := context.Background()
		conf := &es9.RetrieverConfig{}
		searchMode := SearchModeExactMatch("test_field")

		req, err := searchMode.BuildRequest(ctx, conf, "test_query")
		convey.So(err, convey.ShouldBeNil)
		convey.So(req, convey.ShouldNotBeNil)
		convey.So(req.Query, convey.ShouldNotBeNil)
		convey.So(req.Query.Match, convey.ShouldContainKey, "test_field")
		convey.So(req.Query.Match["test_field"].Query, convey.ShouldEqual, "test_query")
	})
}

func TestSearchModeRawStringRequest(t *testing.T) {
	convey.Convey("test SearchModeRawStringRequest", t, func() {
		ctx := context.Background()
		conf := &es9.RetrieverConfig{}
		searchMode := SearchModeRawStringRequest()

		convey.Convey("test from json error", func() {
			r, err := searchMode.BuildRequest(ctx, conf, `invalid_json`)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(r, convey.ShouldBeNil)
		})

		convey.Convey("test success", func() {
			// Note: FromJSON in typedapi might behave differently regarding validation,
			// but it should parse valid JSON.
			q := `{"query":{"match":{"test_field":{"query":"test_query"}}}}`
			r, err := searchMode.BuildRequest(ctx, conf, q)
			convey.So(err, convey.ShouldBeNil)
			convey.So(r, convey.ShouldNotBeNil)
			// Since FromJSON populates the struct, we check if it parsed correctly if possible.
			// However, FromJSON usually populates internal fields or the struct itself.
			// For typedapi, we assume if no error, it's good.
		})
	})
}

func TestDenseVectorSimilarity(t *testing.T) {
	convey.Convey("test DenseVectorSimilarity", t, func() {
		ctx := context.Background()
		conf := &es9.RetrieverConfig{}
		conf.Embedding = &MockEmbedder{}

		searchMode := SearchModeDenseVectorSimilarity(DenseVectorSimilarityTypeCosineSimilarity, "vector_field")

		convey.Convey("test success", func() {
			req, err := searchMode.BuildRequest(ctx, conf, "test_query")
			convey.So(err, convey.ShouldBeNil)
			convey.So(req, convey.ShouldNotBeNil)
			convey.So(req.Query, convey.ShouldNotBeNil)
			convey.So(req.Query.ScriptScore, convey.ShouldNotBeNil)
			convey.So(req.Query.ScriptScore.Script.Source, convey.ShouldNotBeNil)
			convey.So(req.Query.ScriptScore.Script.Source, convey.ShouldNotBeNil)
			source, ok := req.Query.ScriptScore.Script.Source.(*string)
			convey.So(ok, convey.ShouldBeTrue)
			convey.So(*source, convey.ShouldContainSubstring, "cosineSimilarity")
			convey.So(*source, convey.ShouldContainSubstring, "vector_field")
		})

		convey.Convey("test embedding missing", func() {
			conf.Embedding = nil
			_, err := searchMode.BuildRequest(ctx, conf, "test_query")
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "embedding not provided")
		})

		convey.Convey("test embedding error", func() {
			conf.Embedding = &MockEmbedder{err: fmt.Errorf("mock error")}
			_, err := searchMode.BuildRequest(ctx, conf, "test_query")
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "embedding failed")
		})
	})
}

func TestApproximate(t *testing.T) {
	convey.Convey("test Approximate", t, func() {
		ctx := context.Background()
		conf := &es9.RetrieverConfig{}
		conf.Embedding = &MockEmbedder{}

		searchMode := SearchModeApproximate(&ApproximateConfig{
			VectorFieldName: "vector_field",
			K:               of(5),
		})

		convey.Convey("test success simple", func() {
			req, err := searchMode.BuildRequest(ctx, conf, "test_query")
			convey.So(err, convey.ShouldBeNil)
			convey.So(req, convey.ShouldNotBeNil)
			convey.So(len(req.Knn), convey.ShouldEqual, 1)
			convey.So(req.Knn[0].Field, convey.ShouldEqual, "vector_field")
			convey.So(*req.Knn[0].K, convey.ShouldEqual, 5)
			convey.So(req.Knn[0].QueryVector, convey.ShouldNotBeNil)
		})

		convey.Convey("test success hybrid", func() {
			searchMode = SearchModeApproximate(&ApproximateConfig{
				VectorFieldName: "vector_field",
				QueryFieldName:  "text_field",
				Hybrid:          true,
				RRF:             true,
			})
			req, err := searchMode.BuildRequest(ctx, conf, "test_query")
			convey.So(err, convey.ShouldBeNil)
			convey.So(req.Query, convey.ShouldNotBeNil)
			convey.So(req.Query.Bool, convey.ShouldNotBeNil)
			convey.So(len(req.Query.Bool.Must), convey.ShouldEqual, 1)
			convey.So(req.Rank, convey.ShouldNotBeNil)
			convey.So(req.Rank.Rrf, convey.ShouldNotBeNil)
		})

		convey.Convey("test query vector builder", func() {
			modelID := "test_model"
			searchMode = SearchModeApproximate(&ApproximateConfig{
				VectorFieldName:           "vector_field",
				QueryVectorBuilderModelID: &modelID,
			})
			req, err := searchMode.BuildRequest(ctx, conf, "test_query")
			convey.So(err, convey.ShouldBeNil)
			convey.So(req.Knn[0].QueryVectorBuilder, convey.ShouldNotBeNil)
			convey.So(req.Knn[0].QueryVectorBuilder.TextEmbedding, convey.ShouldNotBeNil)
			convey.So(req.Knn[0].QueryVectorBuilder.TextEmbedding.ModelId, convey.ShouldEqual, "test_model")
		})
	})
}

func TestSparseVectorQuery(t *testing.T) {
	convey.Convey("test SparseVectorQuery", t, func() {
		ctx := context.Background()
		conf := &es9.RetrieverConfig{}

		convey.Convey("test inference id", func() {
			infID := "my-inference-endpoint"
			searchMode := SearchModeSparseVectorQuery(&SparseVectorQueryConfig{
				Field:       "sparse_field",
				InferenceID: &infID,
			})
			req, err := searchMode.BuildRequest(ctx, conf, "test query")
			convey.So(err, convey.ShouldBeNil)
			convey.So(req.Query, convey.ShouldNotBeNil)
			convey.So(req.Query.Bool, convey.ShouldNotBeNil)
			convey.So(len(req.Query.Bool.Should), convey.ShouldEqual, 1)
			sv := req.Query.Bool.Should[0].SparseVector
			convey.So(sv, convey.ShouldNotBeNil)
			convey.So(*sv.InferenceId, convey.ShouldEqual, "my-inference-endpoint")
			convey.So(*sv.Query, convey.ShouldEqual, "test query")
		})

		convey.Convey("test with sparse vector option", func() {
			searchMode := SearchModeSparseVectorQuery(&SparseVectorQueryConfig{
				Field: "sparse_field",
			})
			// Pass sparse vector via options
			opts := es9.WithSparseVector(map[string]float32{"token1": 1.0})
			req, err := searchMode.BuildRequest(ctx, conf, "test query", opts)
			convey.So(err, convey.ShouldBeNil)
			sv := req.Query.Bool.Should[0].SparseVector
			convey.So(sv, convey.ShouldNotBeNil)
			convey.So(sv.QueryVector, convey.ShouldContainKey, "token1")
		})

		convey.Convey("test error", func() {
			searchMode := SearchModeSparseVectorQuery(&SparseVectorQueryConfig{
				Field: "sparse_field",
			})
			_, err := searchMode.BuildRequest(ctx, conf, "test query")
			convey.So(err, convey.ShouldNotBeNil)
		})
	})
}

func TestSparseVectorTextExpansion(t *testing.T) {
	convey.Convey("test SparseVectorTextExpansion", t, func() {
		ctx := context.Background()
		conf := &es9.RetrieverConfig{}

		searchMode := SearchModeSparseVectorTextExpansion("model_id", "vector_field")

		req, err := searchMode.BuildRequest(ctx, conf, "test query")
		convey.So(err, convey.ShouldBeNil)
		convey.So(req.Query, convey.ShouldNotBeNil)
		convey.So(req.Query.Bool, convey.ShouldNotBeNil)
		convey.So(len(req.Query.Bool.Must), convey.ShouldEqual, 1)

		// In TextExpansionQuery, the field name is dynamically set in a map
		teqWrapper := req.Query.Bool.Must[0].TextExpansion
		convey.So(teqWrapper, convey.ShouldContainKey, "vector_field.tokens")
		teq := teqWrapper["vector_field.tokens"]
		convey.So(teq.ModelId, convey.ShouldEqual, "model_id")
		convey.So(teq.ModelText, convey.ShouldEqual, "test query")
	})
}

func of[T any](v T) *T {
	return &v
}

type MockEmbedder struct {
	err error
}

func (m *MockEmbedder) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	if m.err != nil {
		return nil, m.err
	}
	return [][]float64{{0.1, 0.2}}, nil
}

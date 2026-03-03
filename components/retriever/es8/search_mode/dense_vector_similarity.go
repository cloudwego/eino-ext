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

	"github.com/cloudwego/eino-ext/components/retriever/es8"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
)

// SearchModeDenseVectorSimilarity calculates the embedding similarity between a dense_vector field and the query.
// See: https://www.elastic.co/guide/en/elasticsearch/reference/7.17/query-dsl-script-score-query.html#vector-functions
func SearchModeDenseVectorSimilarity(typ DenseVectorSimilarityType, vectorFieldName string) es8.SearchMode {
	return &denseVectorSimilarity{fmt.Sprintf(denseVectorScriptMap[typ], vectorFieldName)}
}

type denseVectorSimilarity struct {
	script string
}

func (d *denseVectorSimilarity) BuildRequest(ctx context.Context, conf *es8.RetrieverConfig, query string,
	opts ...retriever.Option) (*search.Request, error) {

	co := retriever.GetCommonOptions(&retriever.Options{
		Index:          ptrWithoutZero(conf.Index),
		TopK:           ptrWithoutZero(conf.TopK),
		ScoreThreshold: conf.ScoreThreshold,
		Embedding:      conf.Embedding,
	}, opts...)

	io := retriever.GetImplSpecificOptions[es8.ImplOptions](nil, opts...)

	emb := co.Embedding
	if emb == nil {
		return nil, fmt.Errorf("[BuildRequest][SearchModeDenseVectorSimilarity] embedding not provided")
	}

	vector, err := emb.EmbedStrings(makeEmbeddingCtx(ctx, emb), []string{query})
	if err != nil {
		return nil, fmt.Errorf("[BuildRequest][SearchModeDenseVectorSimilarity] embedding failed, %w", err)
	}

	if len(vector) != 1 {
		return nil, fmt.Errorf("[BuildRequest][SearchModeDenseVectorSimilarity] vector size invalid, expect=1, got=%d", len(vector))
	}

	vb, err := json.Marshal(vector[0])
	if err != nil {
		return nil, fmt.Errorf("[BuildRequest][SearchModeDenseVectorSimilarity] marshal vector to bytes failed, %w", err)
	}

	q := &types.Query{
		ScriptScore: &types.ScriptScoreQuery{
			Script: types.Script{
				Source: ptrWithoutZero(d.script),
				Params: map[string]json.RawMessage{"embedding": vb},
			},
		},
	}

	if len(io.Filters) > 0 {
		q.ScriptScore.Query = &types.Query{
			Bool: &types.BoolQuery{Filter: io.Filters},
		}
	} else {
		q.ScriptScore.Query = &types.Query{
			MatchAll: &types.MatchAllQuery{},
		}
	}

	req := &search.Request{Query: q, Size: co.TopK}
	if co.ScoreThreshold != nil {
		req.MinScore = (*types.Float64)(ptrWithoutZero(*co.ScoreThreshold))
	}

	return req, nil
}

// DenseVectorSimilarityType represents the type of dense vector similarity.
type DenseVectorSimilarityType string

const (
	DenseVectorSimilarityTypeCosineSimilarity DenseVectorSimilarityType = "cosineSimilarity"
	DenseVectorSimilarityTypeDotProduct       DenseVectorSimilarityType = "dotProduct"
	DenseVectorSimilarityTypeL1Norm           DenseVectorSimilarityType = "l1norm"
	DenseVectorSimilarityTypeL2Norm           DenseVectorSimilarityType = "l2norm"
)

var denseVectorScriptMap = map[DenseVectorSimilarityType]string{
	DenseVectorSimilarityTypeCosineSimilarity: `cosineSimilarity(params.embedding, '%s') + 1.0`,
	DenseVectorSimilarityTypeDotProduct: `
    double value = dotProduct(params.embedding, '%s');
    return sigmoid(1, Math.E, -value);
    `,
	DenseVectorSimilarityTypeL1Norm: `1 / (1 + l1norm(params.embedding, '%s'))`,
	DenseVectorSimilarityTypeL2Norm: `1 / (1 + l2norm(params.embedding, '%s'))`,
}

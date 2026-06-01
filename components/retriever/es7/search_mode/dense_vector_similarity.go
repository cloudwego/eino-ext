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

	"github.com/cloudwego/eino-ext/components/retriever/es7"
	"github.com/cloudwego/eino/components/retriever"
)

// DenseVectorSimilarityType represents the type of dense vector similarity.
type DenseVectorSimilarityType string

const (
	DenseVectorSimilarityTypeCosineSimilarity DenseVectorSimilarityType = "cosineSimilarity"
	DenseVectorSimilarityTypeDotProduct       DenseVectorSimilarityType = "dotProduct"
	DenseVectorSimilarityTypeL1Norm           DenseVectorSimilarityType = "l1norm"
	DenseVectorSimilarityTypeL2Norm           DenseVectorSimilarityType = "l2norm"
)

var denseVectorScriptMap = map[DenseVectorSimilarityType]string{
	DenseVectorSimilarityTypeCosineSimilarity: `cosineSimilarity(params.query_vector, '%s') + 1.0`,
	DenseVectorSimilarityTypeDotProduct: `
    double value = dotProduct(params.query_vector, '%s');
    return sigmoid(1, Math.E, -value);
    `,
	DenseVectorSimilarityTypeL1Norm: `1 / (1 + l1norm(params.query_vector, '%s'))`,
	DenseVectorSimilarityTypeL2Norm: `1 / (1 + l2norm(params.query_vector, '%s'))`,
}

// DenseVectorSimilarity calculates the embedding similarity between a dense_vector field and the query using script_score.
// See: https://www.elastic.co/guide/en/elasticsearch/reference/7.10/query-dsl-script-score-query.html#vector-functions
func DenseVectorSimilarity(typ DenseVectorSimilarityType, vectorFieldName string) es7.SearchMode {
	return &denseVectorSimilarity{
		script: fmt.Sprintf(denseVectorScriptMap[typ], vectorFieldName),
	}
}

type denseVectorSimilarity struct {
	script string
}

func (d *denseVectorSimilarity) BuildRequest(ctx context.Context, conf *es7.RetrieverConfig, query string,
	opts ...retriever.Option) (map[string]any, error) {

	co := retriever.GetCommonOptions(&retriever.Options{
		Index:          &conf.Index,
		TopK:           &conf.TopK,
		ScoreThreshold: conf.ScoreThreshold,
		Embedding:      conf.Embedding,
	}, opts...)

	io := retriever.GetImplSpecificOptions[es7.ImplOptions](nil, opts...)

	emb := co.Embedding
	if emb == nil {
		return nil, fmt.Errorf("[BuildRequest][DenseVectorSimilarity] embedding not provided")
	}

	vector, err := emb.EmbedStrings(makeEmbeddingCtx(ctx, emb), []string{query})
	if err != nil {
		return nil, fmt.Errorf("[BuildRequest][DenseVectorSimilarity] embedding failed, %w", err)
	}

	if len(vector) != 1 {
		return nil, fmt.Errorf("[BuildRequest][DenseVectorSimilarity] vector size invalid, expect=1, got=%d", len(vector))
	}

	// Construct script_score query
	// https://www.elastic.co/guide/en/elasticsearch/reference/7.10/query-dsl-script-score-query.html

	scriptScore := map[string]any{
		"script": map[string]any{
			"source": d.script,
			"params": map[string]any{
				"query_vector": vector[0],
			},
		},
	}

	// Add query filter if exists, otherwise match_all
	if len(io.Filters) > 0 {
		scriptScore["query"] = map[string]any{
			"bool": map[string]any{
				"filter": io.Filters,
			},
		}
	} else {
		scriptScore["query"] = map[string]any{
			"match_all": map[string]any{},
		}
	}

	reqBody := map[string]any{
		"query": map[string]any{
			"script_score": scriptScore,
		},
	}

	return reqBody, nil
}

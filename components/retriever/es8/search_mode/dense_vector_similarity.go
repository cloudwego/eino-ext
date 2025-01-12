/*
 * Copyright 2024 CloudWeGo Authors
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

	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

	"github.com/cloudwego/eino/components/retriever"

	"github.com/cloudwego/eino-ext/components/retriever/es8/field_mapping"
)

// SearchModeDenseVectorSimilarity calculate embedding similarity between dense_vector field and query
// see: https://www.elastic.co/guide/en/elasticsearch/reference/7.17/query-dsl-script-score-query.html#vector-functions
func SearchModeDenseVectorSimilarity(typ DenseVectorSimilarityType) SearchMode {
	return &denseVectorSimilarity{script: denseVectorScriptMap[typ]}
}

type DenseVectorSimilarityQuery struct {
	FieldKV field_mapping.FieldKV `json:"field_kv"`
	Filters []types.Query         `json:"filters,omitempty"`
}

// ToRetrieverQuery convert approximate query to string query
func (d *DenseVectorSimilarityQuery) ToRetrieverQuery() (string, error) {
	b, err := json.Marshal(d)
	if err != nil {
		return "", fmt.Errorf("[ToRetrieverQuery] convert query failed, %w", err)
	}

	return string(b), nil
}

type denseVectorSimilarity struct {
	script string
}

func (d *denseVectorSimilarity) BuildRequest(ctx context.Context, query string, options *retriever.Options) (*search.Request, error) {
	var dq DenseVectorSimilarityQuery
	if err := json.Unmarshal([]byte(query), &dq); err != nil {
		return nil, fmt.Errorf("[BuildRequest][SearchModeDenseVectorSimilarity] parse query failed, %w", err)
	}

	emb := options.Embedding
	if emb == nil {
		return nil, fmt.Errorf("[BuildRequest][SearchModeDenseVectorSimilarity] embedding not provided")
	}

	vector, err := emb.EmbedStrings(makeEmbeddingCtx(ctx, emb), []string{dq.FieldKV.Value})
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
				Source: of(fmt.Sprintf(d.script, dq.FieldKV.FieldNameVector)),
				Params: map[string]json.RawMessage{"embedding": vb},
			},
		},
	}

	if len(dq.Filters) > 0 {
		q.ScriptScore.Query = &types.Query{
			Bool: &types.BoolQuery{Filter: dq.Filters},
		}
	} else {
		q.ScriptScore.Query = &types.Query{
			MatchAll: &types.MatchAllQuery{},
		}
	}

	req := &search.Request{Query: q, Size: options.TopK}
	if options.ScoreThreshold != nil {
		req.MinScore = (*types.Float64)(of(*options.ScoreThreshold))
	}

	return req, nil
}

type DenseVectorSimilarityType string

const (
	DenseVectorSimilarityTypeCosineSimilarity DenseVectorSimilarityType = "cosineSimilarity"
	DenseVectorSimilarityTypeDotProduct       DenseVectorSimilarityType = "dotProduct"
	DenseVectorSimilarityTypeL1Norm           DenseVectorSimilarityType = "l1norm"
	DenseVectorSimilarityTypeL2Norm           DenseVectorSimilarityType = "l2norm"
)

var denseVectorScriptMap = map[DenseVectorSimilarityType]string{
	DenseVectorSimilarityTypeCosineSimilarity: `cosineSimilarity(params.embedding, '%s') + 1.0`,
	DenseVectorSimilarityTypeDotProduct: `""
          double value = dotProduct(params.embedding, '%s');
          return sigmoid(1, Math.E, -value); 
        ""`,
	DenseVectorSimilarityTypeL1Norm: `1 / (1 + l1norm(params.embedding, '%s'))`,
	DenseVectorSimilarityTypeL2Norm: `1 / (1 + l2norm(params.embedding, '%s'))`,
}

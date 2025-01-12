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

// SearchModeApproximate retrieve with multiple approximate strategy (filter+knn+rrf)
// knn: https://www.elastic.co/guide/en/elasticsearch/reference/current/knn-search.html
// rrf: https://www.elastic.co/guide/en/elasticsearch/reference/current/rrf.html
func SearchModeApproximate(config *ApproximateConfig) SearchMode {
	return &approximate{config}
}

type ApproximateConfig struct {
	// Hybrid if true, add filters and rff to knn query
	Hybrid bool
	// Rrf is a method for combining multiple result sets, is used to
	// even the score from the knn query and text query
	Rrf bool
	// RrfRankConstant determines how much influence documents in
	// individual result sets per query have over the final ranked result set
	RrfRankConstant *int64
	// RrfWindowSize determines the size of the individual result sets per query
	RrfWindowSize *int64
}

type ApproximateQuery struct {
	// FieldKV es field info, QueryVectorBuilderModelID will be used if embedding not provided in config,
	// and Embedding will be used if QueryVectorBuilderModelID is nil
	FieldKV field_mapping.FieldKV `json:"field_kv"`
	// QueryVectorBuilderModelID the query vector builder model id
	// see: https://www.elastic.co/guide/en/machine-learning/8.16/ml-nlp-text-emb-vector-search-example.html
	QueryVectorBuilderModelID *string `json:"query_vector_builder_model_id,omitempty"`
	// Boost Floating point number used to decrease or increase the relevance scores of the query.
	// Boost values are relative to the default value of 1.0.
	// A boost value between 0 and 1.0 decreases the relevance score.
	// A value greater than 1.0 increases the relevance score.
	Boost *float32 `json:"boost,omitempty"`
	// Filters for the kNN search query
	Filters []types.Query `json:"filters,omitempty"`
	// K The final number of nearest neighbors to return as top hits
	K *int `json:"k,omitempty"`
	// NumCandidates The number of nearest neighbor candidates to consider per shard
	NumCandidates *int `json:"num_candidates,omitempty"`
	// Similarity The minimum similarity for a vector to be considered a match
	Similarity *float32 `json:"similarity,omitempty"`
}

// ToRetrieverQuery convert approximate query to string query
func (a *ApproximateQuery) ToRetrieverQuery() (string, error) {
	b, err := json.Marshal(a)
	if err != nil {
		return "", fmt.Errorf("[ToRetrieverQuery] convert query failed, %w", err)
	}

	return string(b), nil
}

type approximate struct {
	config *ApproximateConfig
}

func (a *approximate) BuildRequest(ctx context.Context, query string, options *retriever.Options) (*search.Request, error) {
	var appReq ApproximateQuery
	if err := json.Unmarshal([]byte(query), &appReq); err != nil {
		return nil, fmt.Errorf("[BuildRequest][SearchModeApproximate] parse query failed, %w", err)
	}

	knn := types.KnnSearch{
		Boost:              appReq.Boost,
		Field:              string(appReq.FieldKV.FieldNameVector),
		Filter:             appReq.Filters,
		K:                  appReq.K,
		NumCandidates:      appReq.NumCandidates,
		QueryVector:        nil,
		QueryVectorBuilder: nil,
		Similarity:         appReq.Similarity,
	}

	if appReq.QueryVectorBuilderModelID != nil {
		knn.QueryVectorBuilder = &types.QueryVectorBuilder{TextEmbedding: &types.TextEmbedding{
			ModelId:   *appReq.QueryVectorBuilderModelID,
			ModelText: appReq.FieldKV.Value,
		}}
	} else {
		emb := options.Embedding
		if emb == nil {
			return nil, fmt.Errorf("[BuildRequest][SearchModeApproximate] embedding not provided")
		}

		vector, err := emb.EmbedStrings(makeEmbeddingCtx(ctx, emb), []string{appReq.FieldKV.Value})
		if err != nil {
			return nil, fmt.Errorf("[BuildRequest][SearchModeApproximate] embedding failed, %w", err)
		}

		if len(vector) != 1 {
			return nil, fmt.Errorf("[BuildRequest][SearchModeApproximate] vector len error, expected=1, got=%d", len(vector))
		}

		knn.QueryVector = f64To32(vector[0])
	}

	req := &search.Request{Knn: []types.KnnSearch{knn}, Size: options.TopK}

	if a.config.Hybrid {
		req.Query = &types.Query{
			Bool: &types.BoolQuery{
				Filter: appReq.Filters,
				Must: []types.Query{
					{
						Match: map[string]types.MatchQuery{
							string(appReq.FieldKV.FieldName): {Query: appReq.FieldKV.Value},
						},
					},
				},
			},
		}

		if a.config.Rrf {
			req.Rank = &types.RankContainer{Rrf: &types.RrfRank{
				RankConstant:   a.config.RrfRankConstant,
				RankWindowSize: a.config.RrfWindowSize,
			}}
		}
	}

	if options.ScoreThreshold != nil {
		req.MinScore = (*types.Float64)(of(*options.ScoreThreshold))
	}

	return req, nil
}

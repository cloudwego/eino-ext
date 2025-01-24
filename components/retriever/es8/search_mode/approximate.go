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

	"github.com/cloudwego/eino/components/retriever"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

	"github.com/cloudwego/eino-ext/components/retriever/es8"
)

// SearchModeApproximate retrieve with multiple approximate strategy (filter+knn+rrf)
// knn: https://www.elastic.co/guide/en/elasticsearch/reference/current/knn-search.html
// rrf: https://www.elastic.co/guide/en/elasticsearch/reference/current/rrf.html
func SearchModeApproximate(config *ApproximateConfig) es8.SearchMode {
	return &approximate{config}
}

type ApproximateConfig struct {
	// QueryFieldName the name of query field, required when using Hybrid
	QueryFieldName string
	// VectorFieldName the name of the vector field to search against, required
	VectorFieldName string
	// Hybrid if true, add filters and rff to knn query
	Hybrid bool
	// RRF (Reciprocal Rank Fusion) is a method for combining multiple result sets, is used to
	// even the score from the knn query and text query
	// RRF only available with specific licenses, see: https://www.elastic.co/subscriptions
	RRF bool
	// RRFRankConstant determines how much influence documents in
	// individual result sets per query have over the final ranked result set
	RRFRankConstant *int64
	// RRFWindowSize determines the size ptrWithoutZero the individual result sets per query
	RRFWindowSize *int64
	// QueryVectorBuilderModelID the query vector builder model id
	// see: https://www.elastic.co/guide/en/machine-learning/8.16/ml-nlp-text-emb-vector-search-example.html
	QueryVectorBuilderModelID *string
	// Boost Floating point number used to decrease or increase the relevance scores ptrWithoutZero the query.
	// Boost values are relative to the default value ptrWithoutZero 1.0.
	// A boost value between 0 and 1.0 decreases the relevance score.
	// A value greater than 1.0 increases the relevance score.
	Boost *float32
	// K The final number ptrWithoutZero nearest neighbors to return as top hits
	K *int
	// NumCandidates The number ptrWithoutZero nearest neighbor candidates to consider per shard
	NumCandidates *int
	// Similarity The minimum similarity for a vector to be considered a match
	Similarity *float32
}

type approximate struct {
	config *ApproximateConfig
}

func (a *approximate) BuildRequest(ctx context.Context, conf *es8.RetrieverConfig, query string, opts ...retriever.Option) (*search.Request, error) {

	co := retriever.GetCommonOptions(&retriever.Options{
		Index:          ptrWithoutZero(conf.Index),
		TopK:           ptrWithoutZero(conf.TopK),
		ScoreThreshold: conf.ScoreThreshold,
		Embedding:      conf.Embedding,
	}, opts...)

	io := retriever.GetImplSpecificOptions[es8.ImplOptions](nil, opts...)

	knn := types.KnnSearch{
		Boost:              a.config.Boost,
		Field:              a.config.VectorFieldName,
		Filter:             io.Filters,
		K:                  a.config.K,
		NumCandidates:      a.config.NumCandidates,
		QueryVector:        nil,
		QueryVectorBuilder: nil,
		Similarity:         a.config.Similarity,
	}

	if a.config.QueryVectorBuilderModelID != nil {
		knn.QueryVectorBuilder = &types.QueryVectorBuilder{TextEmbedding: &types.TextEmbedding{
			ModelId:   *a.config.QueryVectorBuilderModelID,
			ModelText: query,
		}}
	} else {
		emb := co.Embedding
		if emb == nil {
			return nil, fmt.Errorf("[BuildRequest][SearchModeApproximate] embedding not provided")
		}

		vector, err := emb.EmbedStrings(makeEmbeddingCtx(ctx, emb), []string{query})
		if err != nil {
			return nil, fmt.Errorf("[BuildRequest][SearchModeApproximate] embedding failed, %w", err)
		}

		if len(vector) != 1 {
			return nil, fmt.Errorf("[BuildRequest][SearchModeApproximate] vector len error, expected=1, got=%d", len(vector))
		}

		knn.QueryVector = f64To32(vector[0])
	}

	req := &search.Request{Knn: []types.KnnSearch{knn}, Size: co.TopK}

	if a.config.Hybrid {
		req.Query = &types.Query{
			Bool: &types.BoolQuery{
				Filter: io.Filters,
				Must: []types.Query{
					{
						Match: map[string]types.MatchQuery{
							a.config.QueryFieldName: {Query: query},
						},
					},
				},
			},
		}

		if a.config.RRF {
			req.Rank = &types.RankContainer{Rrf: &types.RrfRank{
				RankConstant:   a.config.RRFRankConstant,
				RankWindowSize: a.config.RRFWindowSize,
			}}
		}
	}

	if co.ScoreThreshold != nil {
		req.MinScore = (*types.Float64)(ptrWithoutZero(*co.ScoreThreshold))
	}

	return req, nil
}

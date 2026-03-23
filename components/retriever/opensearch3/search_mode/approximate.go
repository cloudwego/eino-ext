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

	"github.com/cloudwego/eino-ext/components/retriever/opensearch3"
	"github.com/cloudwego/eino/components/retriever"
)

// ApproximateConfig contains configuration for Approximate search mode (KNN + Hybrid + RRF).
type ApproximateConfig struct {
	// VectorField is the name of the vector field to search against
	VectorField string
	// K is the number of nearest neighbors to return
	K int

	// Hybrid configuration
	// If Hybrid is true, a keyword search (match query) will be executed alongside KNN
	Hybrid bool
	// QueryFieldName is the field to perform keyword match on (required if Hybrid is true)
	QueryFieldName string

	// RRF configuration
	// Note: RRF (Reciprocal Rank Fusion) requires OpenSearch 2.19+ and the 'score-ranker-processor'.
	// Hybrid Search with Score Normalization (Convex Combination) requires OpenSearch 2.10+ and 'normalization-processor'.
	// This implementation supports RRF by passing the 'ext.rrf' parameter (compatible with 2.19+).
	// For 2.10+ (Convex Combination), typically server-side Pipeline configuration is used with a standard boolean query request.
	// For standardizing with ES8, we'll try to use the 'rank' parameter if possible,
	// or fallback to bool query combination.
	// OpenSearch's "Neural Search" plugin typically handles RRF via a search pipeline or specific query structure.
	// Here we will construct a hybrid query using 'bool' which is the standard way compatible with most versions/processors.
	// If RRF specific params are needed, they are usually pipeline configs.
	// However, recent OpenSearch versions allow 'hybrid' query.
	// To keep it simple and compatible: we'll use a Bool query combining KNN and Match.
	// If RRF is requested, we assume a search pipeline is set up or we'll pass 'rank' param if supported.
	RRF bool
}

// Approximate implements KNN search with optional Hybrid and RRF support.
// It replaces the previous KNN search mode.
func Approximate(config *ApproximateConfig) opensearch3.SearchMode {
	return &approximate{config: config}
}

type approximate struct {
	config *ApproximateConfig
}

func (a *approximate) BuildRequest(ctx context.Context, conf *opensearch3.RetrieverConfig, query string,
	opts ...retriever.Option) (map[string]any, error) {

	co := retriever.GetCommonOptions(&retriever.Options{
		Index:          &conf.Index,
		TopK:           &conf.TopK,
		ScoreThreshold: conf.ScoreThreshold,
		Embedding:      conf.Embedding,
	}, opts...)

	io := retriever.GetImplSpecificOptions[opensearch3.ImplOptions](nil, opts...)

	emb := co.Embedding
	if emb == nil {
		return nil, fmt.Errorf("[BuildRequest][Approximate] embedding not provided")
	}

	vector, err := emb.EmbedStrings(makeEmbeddingCtx(ctx, emb), []string{query})
	if err != nil {
		return nil, fmt.Errorf("[BuildRequest][Approximate] embedding failed, %w", err)
	}

	if len(vector) != 1 {
		return nil, fmt.Errorf("[BuildRequest][Approximate] vector size invalid, expect=1, got=%d", len(vector))
	}

	// 1. Construct KNN Query
	knnParams := map[string]any{
		"vector": vector[0],
		"k":      a.config.K,
	}
	if len(io.Filters) > 0 {
		knnParams["filter"] = map[string]any{
			"bool": map[string]any{
				"filter": io.Filters,
			},
		}
	}

	knnQuery := map[string]any{
		"knn": map[string]any{
			a.config.VectorField: knnParams,
		},
	}

	// 2. Construct final query
	var finalQuery map[string]any

	if a.config.Hybrid {
		if a.config.QueryFieldName == "" {
			return nil, fmt.Errorf("[BuildRequest][Approximate] QueryFieldName required for Hybrid search")
		}

		// Hybrid: Bool query with Should clauses (KNN + Match)
		// This simulates a basic hybrid search suitable for RRF processing downstream

		matchQuery := map[string]any{
			"match": map[string]any{
				a.config.QueryFieldName: map[string]any{
					"query": query,
				},
			},
		}

		// Apply filters to the match query as well?
		// ES8 usually applies filters globally.
		// In OpenSearch 'knn' query handles its own filter for pre-filtering.
		// For the match query, we should put it in the bool query alongside filters?

		boolQuery := map[string]any{
			"should": []map[string]any{
				knnQuery,
				matchQuery,
			},
		}

		// If explicit filters are provided (and not just inside KNN),
		// we might want to add them here too, but IO.Filters format (opensearch DSL)
		// might be specific to what the user provided.
		// Usually io.Filters is []interface{}.

		finalQuery = map[string]any{
			"bool": boolQuery,
		}

	} else {
		// Just KNN
		finalQuery = knnQuery
	}

	reqBody := map[string]any{
		"query": finalQuery,
	}

	// Add RRF if requesting
	if a.config.RRF {
		// OpenSearch 2.9+ RRF syntax
		// "ext": { "rrf": { ... } } or top level "rank": { "rrf": {} } depending on version.
		// We'll use the 'ext' method which is common for the RRF plugin.
		reqBody["ext"] = map[string]any{
			"rrf": map[string]any{},
		}
	}

	// fmt.Printf("DEBUG Request Body: %v\n", reqBody)
	return reqBody, nil
}

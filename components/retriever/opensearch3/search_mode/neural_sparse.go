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

	"github.com/cloudwego/eino-ext/components/retriever/opensearch3"
	"github.com/cloudwego/eino/components/retriever"
)

// NeuralSparseConfig contains configuration for NeuralSparse search mode.
type NeuralSparseConfig struct {
	// ModelID is optional. If not provided, the default model configured in the index will be used (if any).
	// In many cases for bi-encoders or sparse encoding, the model is implicit in the ingestion pipeline or default settings.
	// But explicit model_id can be passed in query.
	ModelID string
	// MaxTokenScore optional parameter to prune tokens with low scores
	MaxTokenScore *float32

	// TokenWeights map of token to weight, used for explicit sparse vector query (e.g. from ELSER or other models)
	// If provided, this will be used as the query instead of a query string.
	TokenWeights map[string]float32
}

// NeuralSparse performs a neural_sparse query.
// See: https://opensearch3.org/docs/latest/query-dsl/specialized/neural-sparse/
func NeuralSparse(vectorField string, config *NeuralSparseConfig) opensearch3.SearchMode {
	return &neuralSparse{
		vectorField: vectorField,
		config:      config,
	}
}

type neuralSparse struct {
	vectorField string
	config      *NeuralSparseConfig
}

func (n *neuralSparse) BuildRequest(ctx context.Context, conf *opensearch3.RetrieverConfig, query string,
	opts ...retriever.Option) (map[string]any, error) {

	// For neural_sparse, we usually don't need 'Embedding' component from Eino side
	// because the query text is passed directly to OpenSearch, which handles the embedding/tokenization via its model.
	// So we won't check/use conf.Embedding here unless we wanted to support "pre-encoded" sparse vectors (like passing a map/dict),
	// but the `neural_sparse` query typically takes "query_text" or "query_tokens".
	// The standard Eino `Retrieve(ctx, query string)` passes a string. We'll use "query_text".

	io := retriever.GetImplSpecificOptions[opensearch3.ImplOptions](nil, opts...)

	var neuralSparseQuery map[string]any

	if n.config != nil && len(n.config.TokenWeights) > 0 {
		// Explicit token weights (Sparse Vector Query)
		neuralSparseQuery = map[string]any{
			n.vectorField: map[string]any{
				"query_tokens": n.config.TokenWeights,
			},
		}
	} else {
		// Text expansion query
		neuralSparseQuery = map[string]any{
			n.vectorField: map[string]any{
				"query_text": query,
			},
		}
	}

	inner := neuralSparseQuery[n.vectorField].(map[string]any)

	if n.config != nil {
		if n.config.ModelID != "" {
			inner["model_id"] = n.config.ModelID
		}
		if n.config.MaxTokenScore != nil {
			inner["max_token_score"] = *n.config.MaxTokenScore
		}
	}

	reqBody := map[string]any{
		"query": map[string]any{
			"neural_sparse": neuralSparseQuery,
		},
	}

	// Add filters if any
	if len(io.Filters) > 0 {
		reqBody["query"] = map[string]any{
			"bool": map[string]any{
				"must": []map[string]any{
					{"neural_sparse": neuralSparseQuery},
				},
				"filter": io.Filters,
			},
		}
	}

	return reqBody, nil
}

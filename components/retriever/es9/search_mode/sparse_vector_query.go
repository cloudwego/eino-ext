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

	"github.com/cloudwego/eino-ext/components/retriever/es9"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
)

// SearchModeSparseVectorQuery executes a query consisting of sparse vectors.
// Note: Text expansion query has been replaced since Elasticsearch 8.15.
// See: https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl-sparse-vector-query.html

func SearchModeSparseVectorQuery(config *SparseVectorQueryConfig) es9.SearchMode {
	return &sparseVectorQuery{config}
}

// SparseVectorQueryConfig contains configuration for the SparseVectorQuery search mode.
type SparseVectorQueryConfig struct {
	// Field is the name of the field containing the token-weight pairs to be searched against.
	Field string
	// Boost is the floating-point number used to decrease or increase the relevance scores of the query.
	Boost *float32
	// InferenceID is used to convert the query text into token-weight pairs.
	// It must be the same inference ID that was used to create the tokens from the input text.
	// If InferenceID is not provided, this search mode will use the Document.SparseVector method to get the query sparse vector.
	// See: https://www.elastic.co/guide/en/elasticsearch/reference/current/inference-apis.html
	InferenceID *string
}

type sparseVectorQuery struct {
	config *SparseVectorQueryConfig
}

func (s *sparseVectorQuery) BuildRequest(ctx context.Context, conf *es9.RetrieverConfig, query string, opts ...retriever.Option) (*search.Request, error) {
	co := retriever.GetCommonOptions(&retriever.Options{
		Index:          ptrWithoutZero(conf.Index),
		TopK:           ptrWithoutZero(conf.TopK),
		ScoreThreshold: conf.ScoreThreshold,
		Embedding:      conf.Embedding,
	}, opts...)

	io := retriever.GetImplSpecificOptions[es9.ImplOptions](nil, opts...)

	svq := &types.SparseVectorQuery{
		Boost: s.config.Boost,
		Field: s.config.Field,
	}

	if s.config.InferenceID != nil {
		svq.InferenceId = s.config.InferenceID
		svq.Query = &query
	} else if io.SparseVector != nil {
		svq.QueryVector = io.SparseVector
	} else {
		return nil, fmt.Errorf("[sparseVectorQuery] neither inference id or query sparse vector is provided")
	}

	q := &types.Query{
		Bool: &types.BoolQuery{
			Should: []types.Query{
				{
					SparseVector: svq,
				},
			},
			Filter: io.Filters,
		},
	}

	req := &search.Request{Query: q, Size: co.TopK}
	if co.ScoreThreshold != nil {
		req.MinScore = (*types.Float64)(ptrWithoutZero(*co.ScoreThreshold))
	}

	return req, nil
}

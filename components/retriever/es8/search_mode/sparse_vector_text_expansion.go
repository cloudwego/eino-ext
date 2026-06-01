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

	"github.com/cloudwego/eino-ext/components/retriever/es8"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
)

// SearchModeSparseVectorTextExpansion converts the query text into a list of token-weight pairs,
// which are then used in a query against a sparse vector.
// See: https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl-text-expansion-query.html
func SearchModeSparseVectorTextExpansion(modelID, vectorFieldName string) es8.SearchMode {
	return &sparseVectorTextExpansion{modelID, vectorFieldName}
}

type sparseVectorTextExpansion struct {
	modelID         string
	vectorFieldName string
}

func (s sparseVectorTextExpansion) BuildRequest(ctx context.Context, conf *es8.RetrieverConfig, query string,
	opts ...retriever.Option) (*search.Request, error) {

	co := retriever.GetCommonOptions(&retriever.Options{
		Index:          ptrWithoutZero(conf.Index),
		TopK:           ptrWithoutZero(conf.TopK),
		ScoreThreshold: conf.ScoreThreshold,
		Embedding:      conf.Embedding,
	}, opts...)

	io := retriever.GetImplSpecificOptions[es8.ImplOptions](nil, opts...)

	name := fmt.Sprintf("%s.tokens", s.vectorFieldName)
	teq := types.TextExpansionQuery{
		ModelId:   s.modelID,
		ModelText: query,
	}

	q := &types.Query{
		Bool: &types.BoolQuery{
			Must: []types.Query{
				{TextExpansion: map[string]types.TextExpansionQuery{name: teq}},
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

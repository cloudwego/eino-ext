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

	"github.com/cloudwego/eino-ext/components/retriever/es8/field_mapping"
	"github.com/cloudwego/eino/components/retriever"
)

// SearchModeSparseVectorTextExpansion convert the query text into a list of token-weight pairs,
// which are then used in a query against a sparse vector
// see: https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl-text-expansion-query.html
func SearchModeSparseVectorTextExpansion(modelID string) SearchMode {
	return &sparseVectorTextExpansion{modelID}
}

type SparseVectorTextExpansionQuery struct {
	FieldKV field_mapping.FieldKV `json:"field_kv"`
	Filters []types.Query         `json:"filters,omitempty"`
}

// ToRetrieverQuery convert approximate query to string query
func (s *SparseVectorTextExpansionQuery) ToRetrieverQuery() (string, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return "", fmt.Errorf("[ToRetrieverQuery] convert query failed, %w", err)
	}

	return string(b), nil
}

type sparseVectorTextExpansion struct {
	modelID string
}

func (s sparseVectorTextExpansion) BuildRequest(ctx context.Context, query string, options *retriever.Options) (*search.Request, error) {
	var sq SparseVectorTextExpansionQuery
	if err := json.Unmarshal([]byte(query), &sq); err != nil {
		return nil, fmt.Errorf("[BuildRequest][SearchModeSparseVectorTextExpansion] parse query failed, %w", err)
	}

	name := fmt.Sprintf("%s.tokens", sq.FieldKV.FieldNameVector)
	teq := types.TextExpansionQuery{
		ModelId:   s.modelID,
		ModelText: sq.FieldKV.Value,
	}

	q := &types.Query{
		Bool: &types.BoolQuery{
			Must: []types.Query{
				{TextExpansion: map[string]types.TextExpansionQuery{name: teq}},
			},
			Filter: sq.Filters,
		},
	}

	req := &search.Request{Query: q, Size: options.TopK}
	if options.ScoreThreshold != nil {
		req.MinScore = (*types.Float64)(of(*options.ScoreThreshold))
	}

	return req, nil
}

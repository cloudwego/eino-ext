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

	"github.com/cloudwego/eino-ext/components/retriever/opensearch2"
	"github.com/cloudwego/eino/components/retriever"
)

// ExactMatch creates a match query by field.
func ExactMatch(queryFieldName string) opensearch2.SearchMode {
	return &exactMatch{queryFieldName}
}

type exactMatch struct {
	name string
}

func (e *exactMatch) BuildRequest(ctx context.Context, conf *opensearch2.RetrieverConfig, query string,
	opts ...retriever.Option) (map[string]any, error) {
	// Options like TopK, ScoreThreshold are handled by the caller (Retriever) generally merging them,
	// BUT `Retriever` in `opensearch/retriever.go` only blindly adds "size" and "min_score" to the top level.
	// So we just return the query part here?
	// `knn.go` returns `{"query": ...}`.
	// ES8 `exact_match.go` returns `search.Request` containing `Query`.

	// So for exact match:
	// {
	//    "query": {
	//        "match": {
	//            "<field>": {
	//                "query": "<query>"
	//            }
	//        }
	//    }
	// }

	queryMap := map[string]any{
		"match": map[string]any{
			e.name: map[string]any{
				"query": query,
			},
		},
	}

	io := retriever.GetImplSpecificOptions[opensearch2.ImplOptions](nil, opts...)
	if len(io.Filters) > 0 {
		return map[string]any{
			"query": map[string]any{
				"bool": map[string]any{
					"must": []map[string]any{
						queryMap,
					},
					"filter": io.Filters,
				},
			},
		}, nil
	}

	return map[string]any{
		"query": queryMap,
	}, nil
}

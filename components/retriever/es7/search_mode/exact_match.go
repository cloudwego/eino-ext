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

	"github.com/cloudwego/eino-ext/components/retriever/es7"
	"github.com/cloudwego/eino/components/retriever"
)

// ExactMatch creates a search mode that performs exact match queries on a specified field.
func ExactMatch(queryFieldName string) es7.SearchMode {
	return &exactMatch{name: queryFieldName}
}

type exactMatch struct {
	name string
}

func (e *exactMatch) BuildRequest(ctx context.Context, conf *es7.RetrieverConfig, query string,
	opts ...retriever.Option) (map[string]any, error) {

	matchQuery := map[string]any{
		"match": map[string]any{
			e.name: map[string]any{
				"query": query,
			},
		},
	}

	reqBody := map[string]any{
		"query": matchQuery,
	}

	return reqBody, nil
}

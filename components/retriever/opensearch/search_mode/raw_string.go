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
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino-ext/components/retriever/opensearch"
	"github.com/cloudwego/eino/components/retriever"
)

// RawStringRequest uses the query string as the request body directly.
// The query string must be a valid JSON string representing the search request body.
func RawStringRequest() opensearch.SearchMode {
	return &rawString{}
}

type rawString struct{}

func (r *rawString) BuildRequest(ctx context.Context, conf *opensearch.RetrieverConfig, query string,
	opts ...retriever.Option) (map[string]interface{}, error) {

	var req map[string]interface{}
	if err := json.Unmarshal([]byte(query), &req); err != nil {
		return nil, fmt.Errorf("[BuildRequest][RawStringRequest] unmarshal query failed: %w", err)
	}

	return req, nil
}

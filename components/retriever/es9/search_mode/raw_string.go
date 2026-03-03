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

	"github.com/cloudwego/eino-ext/components/retriever/es9"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/search"
)

// SearchModeRawStringRequest uses the query string as the JSON request body directly.
func SearchModeRawStringRequest() es9.SearchMode {
	return &rawString{}
}

type rawString struct{}

func (r rawString) BuildRequest(ctx context.Context, conf *es9.RetrieverConfig, query string,
	opts ...retriever.Option) (*search.Request, error) {

	req, err := search.NewRequest().FromJSON(query)
	if err != nil {
		return nil, err
	}

	return req, nil
}

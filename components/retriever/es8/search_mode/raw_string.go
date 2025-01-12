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

	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"

	"github.com/cloudwego/eino/components/retriever"
)

func SearchModeRawStringRequest() SearchMode {
	return &rawString{}
}

type rawString struct{}

func (r rawString) BuildRequest(_ context.Context, query string, _ *retriever.Options) (*search.Request, error) {
	req, err := search.NewRequest().FromJSON(query)
	if err != nil {
		return nil, err
	}

	return req, nil
}

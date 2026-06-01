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

package opensearch3

import (
	"github.com/cloudwego/eino/components/retriever"
)

// ImplOptions contains OpenSearch-specific options.
// Use retriever.GetImplSpecificOptions[ImplOptions] to get ImplOptions from options.
type ImplOptions struct {
	// Filters is a list of OpenSearch filter queries.
	//
	// Each element in this slice will be marshaled into JSON and added to the
	// "filter" array of the OpenSearch boolean query.
	//
	// You can pass:
	// 1. map[string]any: e.g., map[string]any{"term": map[string]any{"field": "value"}}
	// 2. Custom structs: Any Go struct that serializes to a valid OpenSearch Query DSL JSON object.
	//
	// This flexibility allows support for the full range of OpenSearch query types
	// without being limited by fixed Go types.
	Filters []any `json:"filters,omitempty"`
}

// WithFilters sets filters for the retrieve query.
// This may take effect in search modes.
//
// filters should be a list of items that can be marshaled into OpenSearch Query DSL (e.g. map[string]any or custom structs).
// These items are passed directly to the generic OpenSearch SDK and serialized to JSON.
func WithFilters(filters []any) retriever.Option {
	return retriever.WrapImplSpecificOptFn(func(o *ImplOptions) {
		o.Filters = filters
	})
}

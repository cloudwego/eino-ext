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

package es8

import (
	"github.com/cloudwego/eino/components/retriever"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
)

// ImplOptions contains Elasticsearch-specific options.
// Use retriever.GetImplSpecificOptions[ImplOptions] to get ImplOptions from options.
type ImplOptions struct {
	Filters      []types.Query      `json:"filters,omitempty"`
	SparseVector map[string]float32 `json:"sparse_vector,omitempty"`
}

// WithFilters sets filters for the retrieve query.
// This may take effect in search modes.
func WithFilters(filters []types.Query) retriever.Option {
	return retriever.WrapImplSpecificOptFn(func(o *ImplOptions) {
		o.Filters = filters
	})
}

// WithSparseVector sets the sparse vector for the retrieve query.
// For example, a stored vector {"feature_0": 0.12, "feature_1": 1.2, "feature_2": 3.0}.
// Eino prefers to define sparse vector as int token id to float32 vector mapping, so you may
// convert integer token id to string token.
func WithSparseVector(sparse map[string]float32) retriever.Option {
	return retriever.WrapImplSpecificOptFn(func(o *ImplOptions) {
		o.SparseVector = sparse
	})
}

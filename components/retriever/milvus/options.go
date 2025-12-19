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

package milvus

import (
	"github.com/cloudwego/eino/components/retriever"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
)

// ImplOptions contains Milvus-specific options for retriever operations.
type ImplOptions struct {
	// Filter specifies a boolean filter expression for the search.
	// Optional; defaults to empty (no filtering).
	// See: https://milvus.io/docs/boolean.md
	Filter string

	// SearchQueryOptFn provides additional search query configuration.
	// Optional; defaults to nil.
	SearchQueryOptFn func(option *client.SearchQueryOption)

	// Radius defines the outer boundary for range search.
	// Optional; used with RangeFilter for range queries.
	Radius *float64

	// RangeFilter specifies the minimum similarity threshold for range search.
	// Optional; results below this similarity score are excluded.
	RangeFilter *float64
}

// WithFilter returns an option that sets a boolean filter expression for the search.
// Filter expressions allow filtering results by metadata fields.
// See: https://milvus.io/docs/boolean.md
func WithFilter(filter string) retriever.Option {
	return retriever.WrapImplSpecificOptFn(func(o *ImplOptions) {
		o.Filter = filter
	})
}

// WithSearchQueryOptFn returns an option that sets additional search query configuration.
// This allows direct access to the underlying Milvus SearchQueryOption for advanced use cases.
func WithSearchQueryOptFn(f func(option *client.SearchQueryOption)) retriever.Option {
	return retriever.WrapImplSpecificOptFn(func(o *ImplOptions) {
		o.SearchQueryOptFn = f
	})
}

// WithRadius returns an option that sets the radius for range search.
// Radius defines the outer boundary of the search area; results beyond
// this distance are excluded.
func WithRadius(radius float64) retriever.Option {
	return retriever.WrapImplSpecificOptFn(func(o *ImplOptions) {
		o.Radius = &radius
	})
}

// WithRangeFilter returns an option that sets the minimum similarity threshold.
// Results with similarity below this value are filtered out from the search results.
func WithRangeFilter(rangeFilter float64) retriever.Option {
	return retriever.WrapImplSpecificOptFn(func(o *ImplOptions) {
		o.RangeFilter = &rangeFilter
	})
}

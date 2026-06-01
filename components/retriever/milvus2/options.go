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

package milvus2

import "github.com/cloudwego/eino/components/retriever"

// ImplOptions contains implementation-specific options for retrieval operations.
type ImplOptions struct {
	// Filter is a boolean expression for filtering search results.
	// Refer to https://milvus.io/docs/boolean.md for filter syntax.
	Filter string

	// Grouping configuration for grouping search.
	Grouping *GroupingConfig
}

// WithFilter returns an option that sets a boolean filter expression for search results.
// See https://milvus.io/docs/boolean.md for filter syntax.
func WithFilter(filter string) retriever.Option {
	return retriever.WrapImplSpecificOptFn(func(o *ImplOptions) {
		o.Filter = filter
	})
}

// GroupingConfig contains configuration for grouping search results by a specific field.
type GroupingConfig struct {
	// GroupByField specifies the field name to group results by.
	GroupByField string
	// GroupSize specifies the number of results to return per group.
	GroupSize int
	// StrictGroupSize enforces exact group size even if it degrades performance.
	StrictGroupSize bool
}

// WithGrouping returns an option that enables grouping search by a specified field.
// It configures the group field, number of items per group, and whether to enforce strict sizing.
func WithGrouping(groupByField string, groupSize int, strict bool) retriever.Option {
	return retriever.WrapImplSpecificOptFn(func(o *ImplOptions) {
		o.Grouping = &GroupingConfig{
			GroupByField:    groupByField,
			GroupSize:       groupSize,
			StrictGroupSize: strict,
		}
	})
}

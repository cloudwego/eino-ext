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

package pgvector

import (
	"github.com/cloudwego/eino/components/retriever"
)

// implOptions holds implementation-specific options for pgvector retriever.
type implOptions struct {
	// WhereClause is an optional SQL WHERE clause for filtering results.
	// Example: "metadata->>'category' = 'tech'"
	WhereClause string
	// DistanceFunction specifies the distance function for similarity search.
	// Default: DistanceCosine.
	DistanceFunction DistanceFunction
}

// WithWhereClause adds a SQL WHERE clause to filter search results.
func WithWhereClause(where string) retriever.Option {
	return retriever.WrapImplSpecificOptFn(func(o *implOptions) {
		o.WhereClause = where
	})
}

// WithDistanceFunction sets the distance function for vector similarity search.
func WithDistanceFunction(fn DistanceFunction) retriever.Option {
	return retriever.WrapImplSpecificOptFn(func(o *implOptions) {
		o.DistanceFunction = fn
	})
}

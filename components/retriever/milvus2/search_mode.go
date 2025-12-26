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

import (
	"context"

	"github.com/cloudwego/eino/components/retriever"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

// SearchMode defines the interface for building Milvus search options.
// It specifies how vector search is performed, including index parameters and search behavior.
type SearchMode interface {
	// BuildSearchOption creates a SearchOption for the given query vector.
	// It returns an error if the mode requires a specialized method (e.g., BuildHybridSearchOption).
	BuildSearchOption(ctx context.Context, conf *RetrieverConfig, queryVector []float32, opts ...retriever.Option) (milvusclient.SearchOption, error)
}

// HybridSearchMode defines the interface for building Milvus hybrid search options.
// It supports multi-vector search with result reranking.
type HybridSearchMode interface {
	SearchMode
	// BuildHybridSearchOption creates a HybridSearchOption for multi-vector search with reranking.
	BuildHybridSearchOption(ctx context.Context, conf *RetrieverConfig, queryVector []float32, querySparseVector map[int]float64, opts ...retriever.Option) (milvusclient.HybridSearchOption, error)
}

// QuerySearchMode defines the interface for scalar/query-only search.
// It enables document retrieval based solely on metadata filtering without vector similarity.
type QuerySearchMode interface {
	SearchMode
	// BuildQueryOption creates a QueryOption using the query string as a filter expression.
	BuildQueryOption(ctx context.Context, conf *RetrieverConfig, query string, opts ...retriever.Option) (milvusclient.QueryOption, error)
}

// IteratorSearchMode defines the interface for search iterator operations.
// It enables efficient traversal of large result sets by fetching results in batches.
type IteratorSearchMode interface {
	SearchMode
	// BuildSearchIteratorOption creates a SearchIteratorOption for batch-based result traversal.
	BuildSearchIteratorOption(ctx context.Context, conf *RetrieverConfig, queryVector []float32, opts ...retriever.Option) (milvusclient.SearchIteratorOption, error)
}

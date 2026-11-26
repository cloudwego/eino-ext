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
	"github.com/milvus-io/milvus/client/v2/milvusclient"

	milvus2 "github.com/cloudwego/eino-ext/components/retriever/milvus2"
)

// Hybrid implements hybrid search with reranking.
// Deprecated: Use milvus2.Hybrid instead.
type Hybrid = milvus2.Hybrid

// SubRequest defines a single ANN search request within a hybrid search.
// Deprecated: Use milvus2.SubRequest instead.
type SubRequest = milvus2.SubRequest

// NewHybrid creates a new Hybrid search mode with the given reranker and sub-requests.
// Deprecated: Use milvus2.NewHybrid instead.
func NewHybrid(reranker milvusclient.Reranker, subRequests ...*SubRequest) *Hybrid {
	return milvus2.NewHybrid(reranker, subRequests...)
}

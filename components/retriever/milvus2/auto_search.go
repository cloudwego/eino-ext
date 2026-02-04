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
	"fmt"

	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

// autoSearchMode implements automatic search strategy inference.
// It determines the best search mode (Dense, Sparse, or Hybrid) based on the provided configuration.
// For hybrid search, it defaults to RRFReranker for result fusion.
type autoSearchMode struct{}

// Retrieve performs search mapping configuration to the appropriate search mode.
func (a *autoSearchMode) Retrieve(ctx context.Context, client *milvusclient.Client, conf *RetrieverConfig, query string, opts ...retriever.Option) ([]*schema.Document, error) {
	hasDense := conf.VectorField != ""
	hasSparse := conf.SparseVectorField != ""

	// Case 1: Hybrid Search (Both configured)
	if hasDense && hasSparse {
		// Use RRF for hybrid search result fusion
		reranker := milvusclient.NewRRFReranker()

		// Prepare SubRequests
		// 1. Dense Request
		denseReq := &SubRequest{
			VectorField: conf.VectorField,
			VectorType:  DenseVector,
			// MetricType: leave empty to let Milvus use the index's default metric type.
			TopK:         conf.TopK,
			SearchParams: ExtractSearchParams(conf, conf.VectorField),
		}

		// 2. Sparse Request
		sparseReq := &SubRequest{
			VectorField:  conf.SparseVectorField,
			VectorType:   SparseVector,
			MetricType:   BM25,
			TopK:         conf.TopK,
			SearchParams: ExtractSearchParams(conf, conf.SparseVectorField),
		}

		// Delegate to Hybrid implementation (in same package)
		hybrid := NewHybrid(reranker, denseReq, sparseReq)
		return hybrid.Retrieve(ctx, client, conf, query, opts...)
	}

	// Case 2: Dense Only
	if hasDense {
		// Delegate to Approximate implementation
		approx := NewApproximate("")
		return approx.Retrieve(ctx, client, conf, query, opts...)
	}

	// Case 3: Sparse Only
	if hasSparse {
		// Delegate to Sparse implementation
		sparse := NewSparse("")
		return sparse.Retrieve(ctx, client, conf, query, opts...)
	}

	return nil, fmt.Errorf("[AutoSearch] no vector fields configured; set VectorField or SparseVectorField")
}

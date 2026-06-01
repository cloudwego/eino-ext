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
	"fmt"

	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus/client/v2/entity"
	"github.com/milvus-io/milvus/client/v2/milvusclient"

	milvus2 "github.com/cloudwego/eino-ext/components/retriever/milvus2"
)

// Hybrid implements hybrid search with reranking.
// It allows defining multiple ANN requests that share the same query vector
// (e.g., searching against different vector fields with different parameters),
// and then fusing the results using a Reranker.
type Hybrid struct {
	// SubRequests defines the configuration for each sub-search.
	SubRequests []*SubRequest

	// Reranker is the ranker used to fuse results from sub-requests.
	// Supported types: RRFReranker, WeightedReranker.
	Reranker milvusclient.Reranker

	// TopK overrides the final number of results to return.
	// If 0, uses RetrieverConfig.TopK.
	TopK int
}

// SubRequest defines a single ANN search request within a hybrid search.
type SubRequest struct {
	// VectorField to search in.
	// If empty, uses RetrieverConfig.VectorField.
	VectorField string

	// MetricType for this request.
	// Default: L2
	MetricType milvus2.MetricType

	// TopK for this specific sub-request.
	// Default: 10
	TopK int

	// SearchParams are extra parameters (e.g. "nprobe", "ef").
	SearchParams map[string]string

	// VectorType specifies the type of vector field (e.g., DenseVector, SparseVector).
	// Default: DenseVector
	VectorType milvus2.VectorType
}

// NewHybrid creates a new Hybrid search mode with the given reranker and sub-requests.
func NewHybrid(reranker milvusclient.Reranker, subRequests ...*SubRequest) *Hybrid {
	return &Hybrid{
		Reranker:    reranker,
		SubRequests: subRequests,
	}
}

// BuildSearchOption returns an error because Hybrid search mode requires BuildHybridSearchOption.
func (h *Hybrid) BuildSearchOption(ctx context.Context, conf *milvus2.RetrieverConfig, queryVector []float32, opts ...retriever.Option) (milvusclient.SearchOption, error) {
	return nil, fmt.Errorf("Hybrid search mode requires BuildHybridSearchOption")
}

// Retrieve performs the hybrid search operation.
func (h *Hybrid) Retrieve(ctx context.Context, client *milvusclient.Client, conf *milvus2.RetrieverConfig, query string, opts ...retriever.Option) ([]*schema.Document, error) {
	if conf.Embedding == nil {
		return nil, fmt.Errorf("embedding is required for hybrid search")
	}

	queryVector, err := EmbedQuery(ctx, conf.Embedding, query)
	if err != nil {
		return nil, err
	}

	searchOpt, err := h.BuildHybridSearchOption(ctx, conf, queryVector, query, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to build hybrid search option: %w", err)
	}

	result, err := client.HybridSearch(ctx, searchOpt)
	if err != nil {
		return nil, fmt.Errorf("failed to hybrid search: %w", err)
	}

	if len(result) == 0 {
		return []*schema.Document{}, nil
	}

	return conf.DocumentConverter(ctx, result[0])
}

// BuildHybridSearchOption creates a HybridSearchOption for multi-vector search with reranking.
// It is internal to the hybrid implementation now/helper but kept as method for cleaner code.
func (h *Hybrid) BuildHybridSearchOption(ctx context.Context, conf *milvus2.RetrieverConfig, queryVector []float32, query string, opts ...retriever.Option) (milvusclient.HybridSearchOption, error) {
	// Validate SubRequests - hybrid requires at least 2 SubRequests for meaningful fusion
	if len(h.SubRequests) < 2 {
		return nil, fmt.Errorf("hybrid search requires at least 2 SubRequests; use Approximate or Sparse search mode for single-vector search")
	}

	io := retriever.GetImplSpecificOptions(&milvus2.ImplOptions{}, opts...)
	co := retriever.GetCommonOptions(&retriever.Options{
		TopK: &conf.TopK,
	}, opts...)

	finalTopK := conf.TopK
	if h.TopK > 0 {
		finalTopK = h.TopK
	}
	if co.TopK != nil {
		finalTopK = *co.TopK
	}

	annRequests := make([]*milvusclient.AnnRequest, 0, len(h.SubRequests))
	for _, req := range h.SubRequests {
		// Determine vector field
		field := req.VectorField
		if field == "" {
			if req.VectorType == milvus2.SparseVector {
				field = conf.SparseVectorField
			} else {
				field = conf.VectorField
			}
		}

		// Determine Limit
		limit := req.TopK
		if limit <= 0 {
			limit = conf.TopK // Default to global TopK
		}

		// Create ANN request based on VectorType
		var annReq *milvusclient.AnnRequest
		if req.VectorType == milvus2.SparseVector {
			// Sparse vector: use raw text for BM25 function
			annReq = milvusclient.NewAnnRequest(field, limit, entity.Text(query))
		} else {
			// Dense vector: require query vector
			if len(queryVector) == 0 {
				return nil, fmt.Errorf("dense vector SubRequest requires embedding, but query vector is empty")
			}
			annReq = milvusclient.NewAnnRequest(field, limit, entity.FloatVector(queryVector))
		}

		// Apply search params
		for k, v := range req.SearchParams {
			annReq.WithSearchParam(k, v)
		}

		if req.MetricType != "" {
			annReq.WithSearchParam("metric_type", string(req.MetricType))
		}

		if io.Filter != "" {
			annReq.WithFilter(io.Filter)
		}

		if io.Grouping != nil {
			annReq.WithGroupByField(io.Grouping.GroupByField).
				WithGroupSize(io.Grouping.GroupSize)
			if io.Grouping.StrictGroupSize {
				annReq.WithStrictGroupSize(true)
			}
		}

		annRequests = append(annRequests, annReq)
	}

	hybridOpt := milvusclient.NewHybridSearchOption(conf.Collection, finalTopK, annRequests...).
		WithReranker(h.Reranker).
		WithOutputFields(conf.OutputFields...)

	// Apply partitions
	if len(conf.Partitions) > 0 {
		hybridOpt = hybridOpt.WithPartitions(conf.Partitions...)
	}

	if conf.ConsistencyLevel != milvus2.ConsistencyLevelDefault {
		hybridOpt = hybridOpt.WithConsistencyLevel(conf.ConsistencyLevel.ToEntity())
	}

	return hybridOpt, nil
}

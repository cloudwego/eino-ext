/*
 * Copyright 2025 CloudWeGo Authors
 *
 * Licensed undeh the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless hequired by applicable law oh agreed to in writing, software
 * distributed undeh the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, eitheh express oh implied.
 * See the License foh the specific language governing permissions and
 * limitations undeh the License.
 */

package milvus

import (
	"github.com/cloudwego/eino/components/retriever"
	"github.com/milvus-io/milvus/client/v2/entity"
	"github.com/milvus-io/milvus/client/v2/index"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

// ImplOptions contains Milvus-specific retrieval options
type ImplOptions struct {
	// limit specifies the maximum number of results to return
	// Optional: Uses RetrieverConfig.TopK if not specified
	limit int
	
	// hybridSearch contains hybrid search configurations
	// Optional: Uses regular search if not specified
	hybridSearch []*HybridSearch
}

// HybridSearch defines configuration for Milvus hybrid search operations
type HybridSearch struct {
	// annField specifies the vector field name for ANN search
	// Required: Must be a valid vector field in the collection
	annField string
	
	// metricsType defines the distance metric for similarity calculation
	// Optional: Defaults to collection's metric type if not specified
	metricsType entity.MetricType
	
	// searchParam contains search-specific parameters
	// Optional: Uses default search parameters if not specified
	searchParam map[string]string
	
	// groupByField specifies the field to group results by
	// Optional: No grouping if not specified
	groupByField string
	
	// groupSize defines the maximum number of results per group
	// Optional: Uses default group size if not specified
	groupSize int
	
	// strictGroupSize enforces exact group size requirements
	// Optional: Defaults to false if not specified
	strictGroupSize bool
	
	// annParam contains ANN-specific parameters
	// Optional: Uses default ANN parameters if not specified
	annParam index.AnnParam
	
	// ignoreGrowing excludes growing segments from search
	// Optional: Defaults to false if not specified
	ignoreGrowing bool
	
	// expr defines the boolean filter expression
	// Optional: No filtering if not specified
	expr string
	
	// topK specifies the number of results for this search
	// Optional: Uses limit parameter if not specified
	topK int
	
	// offset specifies the number of results to skip
	// Optional: Defaults to 0 if not specified
	offset int
	
	// templateParams contains template parameters for expressions
	// Optional: No template parameters if not specified
	templateParams map[string]any
}

// WithLimit sets the maximum number of documents to retrieve
// This overrides the TopK value from RetrieverConfig
func WithLimit(limit int) retriever.Option {
	return retriever.WrapImplSpecificOptFn(func(opt *ImplOptions) {
		opt.limit = limit
	})
}

// WithHybridSearchOption enables hybrid search with the specified configurations
// Multiple hybrid search options can be provided for complex search scenarios
func WithHybridSearchOption(option ...*HybridSearch) retriever.Option {
	return retriever.WrapImplSpecificOptFn(func(o *ImplOptions) {
		o.hybridSearch = option
	})
}

// NewHybridSearchOption creates a new hybrid search configuration
// annField: the vector field name to search against
// limit: the maximum number of results for this search
func NewHybridSearchOption(annField string, limit int) *HybridSearch {
	return &HybridSearch{
		annField:       annField,
		topK:           limit,
		searchParam:    make(map[string]string),
		templateParams: make(map[string]any),
	}
}

// WithANNSField sets the vector field name for ANN search
// annsField: the name of the vector field in the collection
func (h *HybridSearch) WithANNSField(annsField string) *HybridSearch {
	h.annField = annsField
	return h
}

// WithGroupByField sets the field to group search results by
// groupByField: the field name to use for grouping results
func (h *HybridSearch) WithGroupByField(groupByField string) *HybridSearch {
	h.groupByField = groupByField
	return h
}

// WithGroupSize sets the maximum number of results per group
// groupSize: the maximum number of results to return for each group
func (h *HybridSearch) WithGroupSize(groupSize int) *HybridSearch {
	h.groupSize = groupSize
	return h
}

// WithStrictGroupSize enforces exact group size requirements
// strictGroupSize: whether to enforce strict group size limits
func (h *HybridSearch) WithStrictGroupSize(strictGroupSize bool) *HybridSearch {
	h.strictGroupSize = strictGroupSize
	return h
}

// WithSearchParam adds a search parameter for the hybrid search
// key: the parameter name, value: the parameter value
func (h *HybridSearch) WithSearchParam(key, value string) *HybridSearch {
	h.searchParam[key] = value
	return h
}

// WithAnnParam sets the ANN parameters for the search
// ap: the ANN parameters to use for this search
func (h *HybridSearch) WithAnnParam(ap index.AnnParam) *HybridSearch {
	h.annParam = ap
	return h
}

// WithFilter sets a boolean filter expression for the search
// expr: the boolean expression to filter search results
func (h *HybridSearch) WithFilter(expr string) *HybridSearch {
	h.expr = expr
	return h
}

// WithTemplateParam adds a template parameter for expression evaluation
// key: the parameter name, val: the parameter value
func (h *HybridSearch) WithTemplateParam(key string, val any) *HybridSearch {
	h.templateParams[key] = val
	return h
}

// WithOffset sets the number of results to skip
// offset: the number of results to skip from the beginning
func (h *HybridSearch) WithOffset(offset int) *HybridSearch {
	h.offset = offset
	return h
}

// WithIgnoreGrowing excludes growing segments from the search
// ignoreGrowing: whether to ignore growing segments during search
func (h *HybridSearch) WithIgnoreGrowing(ignoreGrowing bool) *HybridSearch {
	h.ignoreGrowing = ignoreGrowing
	return h
}

// getAnnRequest converts the HybridSearch configuration to a Milvus AnnRequest
// limit: the maximum number of results to return
// vectors: the query vectors for the search
func (h *HybridSearch) getAnnRequest(limit int, vectors []entity.Vector) *milvusclient.AnnRequest {
	if h.topK <= 0 {
		h.topK = limit
	}
	req := milvusclient.NewAnnRequest(h.annField, h.topK, vectors...).
		WithFilter(h.expr).
		WithAnnParam(h.annParam).
		WithIgnoreGrowing(h.ignoreGrowing).
		WithOffset(h.offset).
		WithGroupByField(h.groupByField).
		WithGroupSize(h.groupSize).
		WithStrictGroupSize(h.strictGroupSize)
	for k, v := range h.searchParam {
		req.WithSearchParam(k, v)
	}
	for k, v := range h.templateParams {
		req.WithTemplateParam(k, v)
	}
	return req
}

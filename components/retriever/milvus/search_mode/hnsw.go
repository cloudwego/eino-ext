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
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

// HNSWConfig configures search behavior for HNSW indexes.
type HNSWConfig struct {
	// Ef controls the search-time trade-off between speed and accuracy.
	// Higher ef values give more accurate results but slower searches.
	// Must be >= TopK. Range: [1, 32768], recommended: [TopK, 10*TopK]
	Ef int
	// Metric is the distance metric used for similarity calculation.
	// Common values: entity.L2 (Euclidean), entity.IP (Inner Product), entity.COSINE.
	Metric entity.MetricType
}

// SearchModeHNSW creates a search mode for HNSW (Hierarchical Navigable Small World) indexes.
// HNSW provides excellent search performance with high recall.
//
// Parameters:
//   - config: HNSW search configuration with ef and metric type
//
// Returns an error if ef is out of valid range [1, 32768].
//
// Example:
//
//	mode, err := search_mode.SearchModeHNSW(&search_mode.HNSWConfig{
//	    Ef:     64,           // Search-time parameter
//	    Metric: entity.L2,    // Euclidean distance
//	})
func SearchModeHNSW(config *HNSWConfig) (SearchMode, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if config.Ef < 1 || config.Ef > 32768 {
		return nil, fmt.Errorf("ef must be in range [1, 32768], got %d", config.Ef)
	}
	if config.Metric == "" {
		config.Metric = entity.COSINE
	}
	return &hnswSearchMode{config: config}, nil
}

// hnswSearchMode implements SearchMode for HNSW indexes.
type hnswSearchMode struct {
	config *HNSWConfig
}

// BuildSearchParam creates HNSW-specific search parameters with the configured ef value.
// It applies optional Radius and RangeFilter from retriever options.
func (h *hnswSearchMode) BuildSearchParam(ctx context.Context, opts ...retriever.Option) (entity.SearchParam, error) {
	sp, err := entity.NewIndexHNSWSearchParam(h.config.Ef)
	if err != nil {
		return nil, fmt.Errorf("failed to create HNSW search param: %w", err)
	}

	// Apply any additional search params from options
	io := retriever.GetImplSpecificOptions(&ImplOptions{}, opts...)
	if io.Radius != nil {
		sp.AddRadius(*io.Radius)
	}
	if io.RangeFilter != nil {
		sp.AddRangeFilter(*io.RangeFilter)
	}

	return sp, nil
}

// MetricType returns the configured metric type for this HNSW search mode.
func (h *hnswSearchMode) MetricType() entity.MetricType {
	return h.config.Metric
}

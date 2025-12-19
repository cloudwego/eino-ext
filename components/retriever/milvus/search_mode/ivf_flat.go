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

// IvfFlatConfig configures search behavior for IVF_FLAT indexes.
type IvfFlatConfig struct {
	// NProbe is the number of cluster units to search.
	// Higher values give more accurate results but slower search.
	// Range: [1, 65536], recommended: 8-256 (typically sqrt(nlist) to nlist)
	NProbe int
	// Metric is the distance metric used for similarity calculation.
	// Common values: entity.L2 (Euclidean), entity.IP (Inner Product), entity.COSINE.
	Metric entity.MetricType
}

// SearchModeIvfFlat creates a search mode for IVF_FLAT (Inverted File with Flat quantizer) indexes.
// IVF_FLAT divides vectors into clusters and searches only relevant clusters,
// providing a good balance between speed and accuracy for large datasets.
//
// Parameters:
//   - config: IVF_FLAT search configuration with nprobe and metric type
//
// Returns an error if nprobe is out of valid range [1, 65536].
//
// Example:
//
//	mode, err := search_mode.SearchModeIvfFlat(&search_mode.IvfFlatConfig{
//	    NProbe: 16,            // Number of clusters to search
//	    Metric: entity.COSINE, // Cosine similarity
//	})
func SearchModeIvfFlat(config *IvfFlatConfig) (SearchMode, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if config.NProbe < 1 || config.NProbe > 65536 {
		return nil, fmt.Errorf("nprobe must be in range [1, 65536], got %d", config.NProbe)
	}
	if config.Metric == "" {
		config.Metric = entity.COSINE
	}
	return &ivfFlatSearchMode{config: config}, nil
}

// ivfFlatSearchMode implements SearchMode for IVF_FLAT indexes.
type ivfFlatSearchMode struct {
	config *IvfFlatConfig
}

// BuildSearchParam creates IVF_FLAT-specific search parameters with the configured nprobe.
// It applies optional Radius and RangeFilter from retriever options.
func (i *ivfFlatSearchMode) BuildSearchParam(ctx context.Context, opts ...retriever.Option) (entity.SearchParam, error) {
	sp, err := entity.NewIndexIvfFlatSearchParam(i.config.NProbe)
	if err != nil {
		return nil, fmt.Errorf("failed to create IVF_FLAT search param: %w", err)
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

// MetricType returns the configured metric type for this IVF_FLAT search mode.
func (i *ivfFlatSearchMode) MetricType() entity.MetricType {
	return i.config.Metric
}

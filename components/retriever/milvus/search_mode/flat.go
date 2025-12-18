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

	"github.com/cloudwego/eino/components/retriever"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

// FlatConfig configures search behavior for FLAT indexes.
type FlatConfig struct {
	// Metric is the distance metric used for similarity calculation.
	// Common values: entity.L2 (Euclidean), entity.IP (Inner Product), entity.COSINE.
	Metric entity.MetricType
}

// SearchModeFlat creates a search mode for FLAT indexes (brute force search).
// FLAT provides 100% recall but has O(n) search complexity.
// Best for small datasets (<10k vectors) or when perfect recall is required.
//
// Parameters:
//   - config: FLAT search configuration with metric type
//
// Example:
//
//	mode := search_mode.SearchModeFlat(&search_mode.FlatConfig{
//	    Metric: entity.L2, // Euclidean distance
//	})
func SearchModeFlat(config *FlatConfig) SearchMode {
	if config == nil {
		config = &FlatConfig{}
	}
	if config.Metric == "" {
		config.Metric = entity.COSINE
	}
	return &flatSearchMode{config: config}
}

type flatSearchMode struct {
	config *FlatConfig
}

func (f *flatSearchMode) BuildSearchParam(ctx context.Context, opts ...retriever.Option) (entity.SearchParam, error) {
	sp, _ := entity.NewIndexFlatSearchParam()

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

func (f *flatSearchMode) MetricType() entity.MetricType {
	return f.config.Metric
}

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

// AutoConfig configures search behavior for AUTOINDEX indexes.
type AutoConfig struct {
	// Level controls the search accuracy/speed tradeoff.
	// Range: 1-5, where 1 is fastest and 5 is most accurate.
	// Default is 1.
	Level int
	// Metric is the distance metric used for similarity calculation.
	// Common values: entity.L2 (Euclidean), entity.IP (Inner Product), entity.COSINE.
	Metric entity.MetricType
}

// SearchModeAuto creates a search mode for AUTOINDEX indexes.
// With AUTOINDEX, Milvus automatically chooses the optimal index type and parameters
// based on the data characteristics. This is recommended for most use cases.
//
// Parameters:
//   - config: AUTOINDEX search configuration with level and metric type
//
// Example:
//
//	mode := search_mode.SearchModeAuto(&search_mode.AutoConfig{
//	    Level:  1,         // Speed-optimized (default)
//	    Metric: entity.IP, // Inner product
//	})
func SearchModeAuto(config *AutoConfig) SearchMode {
	if config == nil {
		config = &AutoConfig{}
	}
	if config.Level < 1 || config.Level > 5 {
		config.Level = 1
	}
	if config.Metric == "" {
		config.Metric = entity.COSINE
	}
	return &autoSearchMode{config: config}
}

type autoSearchMode struct {
	config *AutoConfig
}

func (a *autoSearchMode) BuildSearchParam(ctx context.Context, opts ...retriever.Option) (entity.SearchParam, error) {
	sp, _ := entity.NewIndexAUTOINDEXSearchParam(a.config.Level)

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

func (a *autoSearchMode) MetricType() entity.MetricType {
	return a.config.Metric
}

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

// Package search_mode provides different search mode implementations for the Milvus retriever.
// Each search mode corresponds to a specific index type and defines how search parameters
// are built for that index type.
//
// Available search modes:
//   - SearchModeHNSW: For HNSW indexes, uses ef parameter
//   - SearchModeIvfFlat: For IVF_FLAT indexes, uses nprobe parameter
//   - SearchModeAuto: For AUTOINDEX, uses level parameter
//   - SearchModeFlat: For FLAT indexes (brute force)
package search_mode

import (
	"context"

	"github.com/cloudwego/eino/components/retriever"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

// SearchMode defines the interface for building Milvus search parameters.
// Different index types require different search parameters, and each SearchMode
// implementation encapsulates the logic for a specific index type.
//
// Use one of the provided constructors to create a SearchMode:
//   - SearchModeHNSW for HNSW indexes
//   - SearchModeIvfFlat for IVF_FLAT indexes
//   - SearchModeAuto for AUTOINDEX (recommended for most cases)
//   - SearchModeFlat for FLAT indexes
type SearchMode interface {
	// BuildSearchParam creates search parameters for the given query context.
	// The opts parameter can be used to pass additional search options like
	// WithRadius or WithRangeFilter.
	BuildSearchParam(ctx context.Context, opts ...retriever.Option) (entity.SearchParam, error)

	// MetricType returns the metric type this search mode is configured for.
	// Common values: L2 (Euclidean), IP (Inner Product), COSINE.
	MetricType() entity.MetricType
}

// ImplOptions contains implementation-specific options that can be passed
// through retriever options to customize search behavior.
// This mirrors the ImplOptions in the parent milvus package.
type ImplOptions struct {
	// Radius for range search
	Radius *float64
	// RangeFilter for range search (minimum similarity threshold)
	RangeFilter *float64
}

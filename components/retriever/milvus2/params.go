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

import "fmt"

// Common Milvus search parameters.
// See Milvus documentation for full list: https://milvus.io/docs/index_selection.md
const (
	// ParamNProbe is for IVF indices. Specifies the number of units to query.
	ParamNProbe = "nprobe"
	// ParamEF is for HNSW indices. Specifies the search scope.
	ParamEF = "ef"
	// ParamRadius is for Range Search. Specifies the radius distance.
	ParamRadius = "radius"
	// ParamRangeFilter is for Range Search. Filters out results within this distance.
	ParamRangeFilter = "range_filter"
	// ParamLevel is for SCANN indices. Specifies the pruning level.
	ParamLevel = "level"
	// ParamDropRatioSearch is for Sparse (IVF_FLAT) indices. Ignores small unrelated values.
	ParamDropRatioSearch = "drop_ratio_search"
)

// NewSearchParams creates a helper to build the SearchParams map.
func NewSearchParams() *SearchParamsBuilder {
	return &SearchParamsBuilder{
		m: make(map[string]interface{}),
	}
}

// SearchParamsBuilder helps construct the search parameter map in a typed way.
type SearchParamsBuilder struct {
	m map[string]interface{}
}

// WithNProbe sets the "nprobe" parameter (for IVF indices).
func (b *SearchParamsBuilder) WithNProbe(nprobe int) *SearchParamsBuilder {
	b.m[ParamNProbe] = nprobe
	return b
}

// WithEF sets the "ef" parameter (for HNSW indices).
func (b *SearchParamsBuilder) WithEF(ef int) *SearchParamsBuilder {
	b.m[ParamEF] = ef
	return b
}

// WithRadius sets the "radius" parameter (for Range Search).
func (b *SearchParamsBuilder) WithRadius(radius float64) *SearchParamsBuilder {
	b.m[ParamRadius] = radius
	return b
}

// WithRangeFilter sets the "range_filter" parameter (for Range Search).
func (b *SearchParamsBuilder) WithRangeFilter(filter float64) *SearchParamsBuilder {
	b.m[ParamRangeFilter] = filter
	return b
}

// WithDropRatioSearch sets the "drop_ratio_search" parameter (for Sparse indices).
func (b *SearchParamsBuilder) WithDropRatioSearch(ratio float64) *SearchParamsBuilder {
	b.m[ParamDropRatioSearch] = ratio
	return b
}

// With sets a custom parameter key-value pair.
func (b *SearchParamsBuilder) With(key string, value interface{}) *SearchParamsBuilder {
	b.m[key] = value
	return b
}

// Build returns the constructed map.
func (b *SearchParamsBuilder) Build() map[string]interface{} {
	return b.m
}

// ExtractSearchParams extracts and stringifies search parameters for a specific field from the configuration.
func ExtractSearchParams(conf *RetrieverConfig, field string) map[string]string {
	if conf.SearchParams == nil {
		return nil
	}

	// Milvus SDK expects search parameters (like "nprobe", "ef") to be strings.
	// We allow users to pass them as appropriate types (int, float) in configuration
	// and convert them to strings here.
	if params, ok := conf.SearchParams[field]; ok {
		out := make(map[string]string, len(params))
		for k, v := range params {
			out[k] = fmt.Sprintf("%v", v)
		}
		return out
	}
	return nil
}

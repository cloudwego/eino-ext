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
	"github.com/milvus-io/milvus/client/v2/entity"
	"github.com/milvus-io/milvus/client/v2/milvusclient"

	milvus2 "github.com/cloudwego/eino-ext/components/retriever/milvus2"
)

// Range implements range search to find vectors within a specified distance or similarity radius.
// It returns all vectors whose distance or similarity to the query falls within the radius boundary.
type Range struct {
	// MetricType specifies the metric type for vector similarity.
	MetricType milvus2.MetricType

	// Radius specifies the search radius boundary.
	// For distance metrics (L2/Hamming/Jaccard): finds vectors where distance <= Radius.
	// For similarity metrics (IP/Cosine): finds vectors where score >= Radius.
	Radius float64

	// RangeFilter excludes vectors that are too close or too similar.
	// For L2: excludes vectors where distance < RangeFilter (use with Radius for ring search).
	// For IP: excludes vectors where score > RangeFilter.
	// Optional; leave nil unless performing ring searches.
	RangeFilter *float64
}

// NewRange creates a new Range search mode.
func NewRange(metricType milvus2.MetricType, radius float64) *Range {
	if metricType == "" {
		metricType = milvus2.L2
	}
	return &Range{
		MetricType: metricType,
		Radius:     radius,
	}
}

// WithRangeFilter sets the inner boundary for ring searches.
func (r *Range) WithRangeFilter(rangeFilter float64) *Range {
	r.RangeFilter = &rangeFilter
	return r
}

// BuildSearchOption creates a SearchOption configured for range-based vector search.
func (r *Range) BuildSearchOption(ctx context.Context, conf *milvus2.RetrieverConfig, queryVector []float32, opts ...retriever.Option) (milvusclient.SearchOption, error) {
	io := retriever.GetImplSpecificOptions(&milvus2.ImplOptions{}, opts...)
	co := retriever.GetCommonOptions(&retriever.Options{
		TopK: &conf.TopK,
	}, opts...)

	// TopK acts as a limit on the number of returned vectors within the radius.
	searchOpt := milvusclient.NewSearchOption(conf.Collection, conf.TopK, []entity.Vector{entity.FloatVector(queryVector)}).
		WithANNSField(conf.VectorField).
		WithOutputFields(conf.OutputFields...).
		WithSearchParam("radius", fmt.Sprintf("%v", r.Radius))

	if r.RangeFilter != nil {
		searchOpt = searchOpt.WithSearchParam("range_filter", fmt.Sprintf("%v", *r.RangeFilter))
	}

	// Apply TopK
	if co.TopK != nil && *co.TopK != conf.TopK {
		// NewSearchOption is the only way to set limit
		searchOpt = milvusclient.NewSearchOption(conf.Collection, *co.TopK, []entity.Vector{entity.FloatVector(queryVector)}).
			WithANNSField(conf.VectorField).
			WithOutputFields(conf.OutputFields...).
			WithSearchParam("radius", fmt.Sprintf("%v", r.Radius))
		if r.RangeFilter != nil {
			searchOpt = searchOpt.WithSearchParam("range_filter", fmt.Sprintf("%v", *r.RangeFilter))
		}
	}

	if len(conf.Partitions) > 0 {
		searchOpt = searchOpt.WithPartitions(conf.Partitions...)
	}

	if io.Filter != "" {
		searchOpt = searchOpt.WithFilter(io.Filter)
	}

	if io.Grouping != nil {
		searchOpt = searchOpt.WithGroupByField(io.Grouping.GroupByField).
			WithGroupSize(io.Grouping.GroupSize)
		if io.Grouping.StrictGroupSize {
			searchOpt = searchOpt.WithStrictGroupSize(true)
		}
	}

	return searchOpt, nil
}

// Ensure Range implements milvus2.SearchMode
var _ milvus2.SearchMode = (*Range)(nil)

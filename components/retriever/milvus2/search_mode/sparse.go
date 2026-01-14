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

// Sparse implements sparse vector search (e.g. BM25).
// It uses raw text input allowing Milvus to generate sparse vectors server-side via functions.
type Sparse struct {
	// MetricType specifies the metric type for sparse similarity.
	// Default: BM25 (or IP if not specified but typically BM25 for text).
	MetricType milvus2.MetricType
}

// NewSparse creates a new Sparse search mode.
func NewSparse(metricType milvus2.MetricType) *Sparse {
	if metricType == "" {
		metricType = milvus2.BM25
	}
	return &Sparse{
		MetricType: metricType,
	}
}

// Retrieve performs the sparse search operation (text search via function).
func (s *Sparse) Retrieve(ctx context.Context, client *milvusclient.Client, conf *milvus2.RetrieverConfig, query string, opts ...retriever.Option) ([]*schema.Document, error) {
	searchOpt, err := s.BuildSparseSearchOption(ctx, conf, query, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to build sparse search option: %w", err)
	}

	result, err := client.Search(ctx, searchOpt)
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}

	if len(result) == 0 {
		return []*schema.Document{}, nil
	}

	return conf.DocumentConverter(ctx, result[0])
}

// BuildSparseSearchOption creates a SearchOption configured for sparse vector search using text query.
func (s *Sparse) BuildSparseSearchOption(ctx context.Context, conf *milvus2.RetrieverConfig, query string, opts ...retriever.Option) (milvusclient.SearchOption, error) {
	io := retriever.GetImplSpecificOptions(&milvus2.ImplOptions{}, opts...)
	co := retriever.GetCommonOptions(&retriever.Options{
		TopK: &conf.TopK,
	}, opts...)

	// Determine final topK
	topK := conf.TopK
	if co.TopK != nil {
		topK = *co.TopK
	}

	searchOpt := milvusclient.NewSearchOption(conf.Collection, topK, []entity.Vector{entity.Text(query)}).
		WithANNSField(conf.SparseVectorField).
		WithOutputFields(conf.OutputFields...)

	// Apply metric type
	if s.MetricType != "" {
		searchOpt.WithSearchParam("metric_type", string(s.MetricType))
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

	if conf.ConsistencyLevel != milvus2.ConsistencyLevelDefault {
		searchOpt = searchOpt.WithConsistencyLevel(conf.ConsistencyLevel.ToEntity())
	}

	return searchOpt, nil
}

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

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus/client/v2/entity"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

// Approximate implements approximate nearest neighbor (ANN) search.
// It provides efficient vector similarity search using Milvus indexes.
type Approximate struct {
	// MetricType specifies the metric type for vector similarity.
	// Default: L2.
	MetricType MetricType
}

// NewApproximate creates a new Approximate search mode with the specified metric type.
func NewApproximate(metricType MetricType) *Approximate {
	return &Approximate{
		MetricType: metricType,
	}
}

// Retrieve performs the approximate vector search.
func (a *Approximate) Retrieve(ctx context.Context, client *milvusclient.Client, conf *RetrieverConfig, query string, opts ...retriever.Option) ([]*schema.Document, error) {
	if conf.Embedding == nil {
		return nil, fmt.Errorf("embedding is required for approximate search")
	}

	queryVector, err := EmbedQuery(ctx, conf.Embedding, query)
	if err != nil {
		return nil, err
	}

	searchOpt, err := a.BuildSearchOption(ctx, conf, queryVector, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to build search option: %w", err)
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

// BuildSearchOption creates a SearchOption for ANN search with the configured metric type.
func (a *Approximate) BuildSearchOption(ctx context.Context, conf *RetrieverConfig, queryVector []float32, opts ...retriever.Option) (milvusclient.SearchOption, error) {
	io := retriever.GetImplSpecificOptions(&ImplOptions{}, opts...)
	co := retriever.GetCommonOptions(&retriever.Options{
		TopK: &conf.TopK,
	}, opts...)

	// Determine final topK
	topK := conf.TopK
	if co.TopK != nil {
		topK = *co.TopK
	}

	searchOpt := milvusclient.NewSearchOption(conf.Collection, topK, []entity.Vector{entity.FloatVector(queryVector)}).
		WithANNSField(conf.VectorField).
		WithOutputFields(conf.OutputFields...)

	// Apply metric type
	if a.MetricType != "" {
		searchOpt.WithSearchParam("metric_type", string(a.MetricType))
	}

	// Apply extra search params from config
	for k, v := range ExtractSearchParams(conf, conf.VectorField) {
		searchOpt.WithSearchParam(k, v)
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

	if conf.ConsistencyLevel != ConsistencyLevelDefault {
		searchOpt = searchOpt.WithConsistencyLevel(conf.ConsistencyLevel.ToEntity())
	}

	return searchOpt, nil
}

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
	"errors"
	"fmt"
	"io"

	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus/client/v2/entity"
	"github.com/milvus-io/milvus/client/v2/milvusclient"

	milvus2 "github.com/cloudwego/eino-ext/components/retriever/milvus2"
)

// Iterator implements search iterator mode for traversing large result sets.
// It fetches results in batches to manage memory usage efficiently.
type Iterator struct {
	// MetricType specifies the metric type for vector similarity.
	// Default: L2.
	MetricType milvus2.MetricType

	// BatchSize controls how many items are fetched per network call.
	// Default: 100.
	BatchSize int

	// SearchParams contains extra search parameters (e.g., "nprobe", "ef").
	SearchParams map[string]string
}

// NewIterator creates a new Iterator search mode.
func NewIterator(metricType milvus2.MetricType, batchSize int) *Iterator {
	if metricType == "" {
		metricType = milvus2.L2
	}
	if batchSize <= 0 {
		batchSize = 100
	}
	return &Iterator{
		MetricType: metricType,
		BatchSize:  batchSize,
	}
}

// WithSearchParams sets additional search parameters such as "nprobe" or "ef".
func (i *Iterator) WithSearchParams(params map[string]string) *Iterator {
	i.SearchParams = params
	return i
}

// BuildSearchOption returns an error because Iterator search mode requires BuildSearchIteratorOption.
func (i *Iterator) BuildSearchOption(ctx context.Context, conf *milvus2.RetrieverConfig, queryVector []float32, opts ...retriever.Option) (milvusclient.SearchOption, error) {
	return nil, fmt.Errorf("Iterator search mode requires BuildSearchIteratorOption")
}

// Retrieve performs the search iterator operation, fetching all results.
func (i *Iterator) Retrieve(ctx context.Context, client *milvusclient.Client, conf *milvus2.RetrieverConfig, query string, opts ...retriever.Option) ([]*schema.Document, error) {
	if conf.Embedding == nil {
		return nil, fmt.Errorf("embedding is required for iterator search")
	}

	queryVector, err := EmbedQuery(ctx, conf.Embedding, query)
	if err != nil {
		return nil, err
	}

	iterOpt, err := i.BuildSearchIteratorOption(ctx, conf, queryVector, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to build search iterator option: %w", err)
	}

	iterator, err := client.SearchIterator(ctx, iterOpt)
	if err != nil {
		return nil, fmt.Errorf("failed to create search iterator: %w", err)
	}

	var allDocs []*schema.Document
	for {
		res, err := iterator.Next(ctx)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("iterator next failed: %w", err)
		}
		if res.ResultCount == 0 {
			break
		}

		batchDocs, err := conf.DocumentConverter(ctx, res)
		if err != nil {
			return nil, fmt.Errorf("failed to convert batch results: %w", err)
		}
		allDocs = append(allDocs, batchDocs...)
	}

	return allDocs, nil
}

// BuildSearchIteratorOption creates a SearchIteratorOption for batch-based result traversal.
func (i *Iterator) BuildSearchIteratorOption(ctx context.Context, conf *milvus2.RetrieverConfig, queryVector []float32, opts ...retriever.Option) (milvusclient.SearchIteratorOption, error) {
	io := retriever.GetImplSpecificOptions(&milvus2.ImplOptions{}, opts...)
	co := retriever.GetCommonOptions(&retriever.Options{
		TopK: &conf.TopK,
	}, opts...)

	finalLimit := conf.TopK
	if co.TopK != nil {
		finalLimit = *co.TopK
	}
	opt := milvusclient.NewSearchIteratorOption(conf.Collection, entity.FloatVector(queryVector)).
		WithANNSField(conf.VectorField).
		WithBatchSize(i.BatchSize).
		WithOutputFields(conf.OutputFields...).
		WithIteratorLimit(int64(finalLimit)) // Set total limit

	if i.MetricType != "" {
		opt.WithSearchParam("metric_type", string(i.MetricType))
	}
	for k, v := range i.SearchParams {
		opt.WithSearchParam(k, v)
	}

	if len(conf.Partitions) > 0 {
		opt.WithPartitions(conf.Partitions...)
	}

	if io.Filter != "" {
		opt.WithFilter(io.Filter)
	}

	if io.Grouping != nil {
		opt.WithGroupByField(io.Grouping.GroupByField).
			WithGroupSize(io.Grouping.GroupSize)
		if io.Grouping.StrictGroupSize {
			opt.WithStrictGroupSize(true)
		}
	}

	if conf.ConsistencyLevel != milvus2.ConsistencyLevelDefault {
		opt.WithConsistencyLevel(conf.ConsistencyLevel.ToEntity())
	}

	return opt, nil
}

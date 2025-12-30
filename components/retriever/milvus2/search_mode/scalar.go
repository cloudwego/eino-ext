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
	"github.com/milvus-io/milvus/client/v2/milvusclient"

	milvus2 "github.com/cloudwego/eino-ext/components/retriever/milvus2"
)

// Scalar implements scalar/metadata search using the Milvus Query API.
// It treats the query string as a boolean filter expression (e.g., "id > 10").
type Scalar struct{}

// NewScalar creates a new Scalar search mode.
func NewScalar() *Scalar {
	return &Scalar{}
}

// BuildSearchOption returns nil because Scalar search mode uses the Query API instead of Search.
// Retriever dispatches to BuildQueryOption for QuerySearchMode implementations.
func (s *Scalar) BuildSearchOption(ctx context.Context, conf *milvus2.RetrieverConfig, queryVector []float32, opts ...retriever.Option) (milvusclient.SearchOption, error) {
	return nil, nil
}

// BuildQueryOption creates a QueryOption for scalar/metadata-based document retrieval.
func (s *Scalar) BuildQueryOption(ctx context.Context, conf *milvus2.RetrieverConfig, query string, opts ...retriever.Option) (milvusclient.QueryOption, error) {
	io := retriever.GetImplSpecificOptions(&milvus2.ImplOptions{}, opts...)
	co := retriever.GetCommonOptions(&retriever.Options{
		TopK: &conf.TopK,
	}, opts...)

	finalTopK := conf.TopK
	if co.TopK != nil {
		finalTopK = *co.TopK
	}

	// Combine query and filter with AND logic
	expr := query
	if io.Filter != "" {
		if expr != "" {
			expr = "(" + expr + ") and (" + io.Filter + ")"
		} else {
			expr = io.Filter
		}
	}

	opt := milvusclient.NewQueryOption(conf.Collection).
		WithFilter(expr).
		WithOutputFields(conf.OutputFields...).
		WithLimit(int(finalTopK))

	// Partitions
	if len(conf.Partitions) > 0 {
		opt = opt.WithPartitions(conf.Partitions...)
	}

	// Consistency
	if conf.ConsistencyLevel > 0 {
		opt = opt.WithConsistencyLevel(conf.ConsistencyLevel.ToEntity())
	}

	return opt, nil
}

// Ensure Scalar implements milvus2.QuerySearchMode
var _ milvus2.QuerySearchMode = (*Scalar)(nil)

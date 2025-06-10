/*
 * Copyright 2024 CloudWeGo Authors
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

package pinecone

import (
	"github.com/cloudwego/eino/components/retriever"
)

// options pinecone impl specific option
type options struct {
	DenseVector    []float64              `json:"dense_vector"`
	SparseVector   map[int]float64        `json:"sparse_vector"`
	MetadataFilter map[string]interface{} `json:"metadata_filter"`
}

// WithQueryDenseVector set dense vector for retrieve query.
// Embedding method from retriever config won't be used if DenseVector here is provided.
func WithQueryDenseVector(dense []float64) retriever.Option {
	return retriever.WrapImplSpecificOptFn(func(o *options) {
		o.DenseVector = dense
	})
}

// WithQuerySparseVector set sparse vector for retrieve query.
// sparse is indices -> vector.
func WithQuerySparseVector(sparse map[int]float64) retriever.Option {
	return retriever.WrapImplSpecificOptFn(func(o *options) {
		o.SparseVector = sparse
	})
}

// WithQueryMetadataFilter set filter for retrieve.
// see: https://docs.pinecone.io/guides/data/understanding-metadata#metadata-query-language
func WithQueryMetadataFilter(filter map[string]interface{}) retriever.Option {
	return retriever.WrapImplSpecificOptFn(func(o *options) {
		o.MetadataFilter = filter
	})
}

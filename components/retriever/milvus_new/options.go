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

package milvus_new

import (
	"github.com/cloudwego/eino/components/retriever"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

type ImplOptions struct {
	// Filter is the filter for the search
	// Optional, and the default value is empty
	// It's means the milvus search required param, and refer to https://milvus.io/docs/boolean.md
	Filter string

	// Partition is the partition name to search
	// Optional, and the default value is empty
	Partition string

	// SearchOptFn is the function to set additional search options
	// Optional, and the default value is nil
	// Note: SearchOption is an interface, not a pointer type
	SearchOptFn func(option milvusclient.SearchOption) milvusclient.SearchOption
}

func WithFilter(filter string) retriever.Option {
	return retriever.WrapImplSpecificOptFn(func(o *ImplOptions) {
		o.Filter = filter
	})
}

func WithPartition(partition string) retriever.Option {
	return retriever.WrapImplSpecificOptFn(func(o *ImplOptions) {
		o.Partition = partition
	})
}

func WithSearchOptFn(f func(option milvusclient.SearchOption) milvusclient.SearchOption) retriever.Option {
	return retriever.WrapImplSpecificOptFn(func(o *ImplOptions) {
		o.SearchOptFn = f
	})
}

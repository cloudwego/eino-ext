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

package milvus

import (
	"github.com/cloudwego/eino/components/retriever"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
)

type ImplOptions struct {
	// Filter is the filter for the search
	// Optional, and the default value is empty
	// It's means the milvus search required param, and refer to https://milvus.io/docs/boolean.md
	Filter string

	// SearchQueryOptFn is the function to set the search query option
	// Optional, and the default value is nil
	// It's means the milvus search extra search options, and refer to client.SearchQueryOptionFunc
	SearchQueryOptFn func(option *client.SearchQueryOption)
}

func WithFilter(filter string) retriever.Option {
	return retriever.WrapImplSpecificOptFn(func(o *ImplOptions) {
		o.Filter = filter
	})
}

func WithSearchQueryOptFn(f func(option *client.SearchQueryOption)) retriever.Option {
	return retriever.WrapImplSpecificOptFn(func(o *ImplOptions) {
		o.SearchQueryOptFn = f
	})
}

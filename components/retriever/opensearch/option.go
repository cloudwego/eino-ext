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

package opensearch

import (
	"github.com/cloudwego/eino/components/retriever"
)

// ImplOptions opensearch specified options
// Use retriever.GetImplSpecificOptions[ImplOptions] to get ImplOptions from options.
type ImplOptions struct {
	Filters []interface{} `json:"filters,omitempty"`
}

// WithFilters set filters for retrieve query.
// This may take effect in search modes.
func WithFilters(filters []interface{}) retriever.Option {
	return retriever.WrapImplSpecificOptFn(func(o *ImplOptions) {
		o.Filters = filters
	})
}

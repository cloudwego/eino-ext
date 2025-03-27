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

package ark

import (
	"github.com/cloudwego/eino/components/model"
)

type arkOptions struct {
	customHeaders map[string]string
	contextID     *string
}

// WithCustomHeader sets custom headers for a single request
// the headers will override all the headers given in ChatModelConfig.CustomHeader
func WithCustomHeader(m map[string]string) model.Option {
	return model.WrapImplSpecificOptFn(func(o *arkOptions) {
		o.customHeaders = m
	})
}

// WithPrefixCache creates an option to specify a context ID for the request.
// The context ID is typically obtained from a previous call to CreatePrefix.
//
// When this option is provided, the model will use the cached prefix context
// associated with this ID, allowing you to avoid resending the same context
// messages in each request, which improves efficiency and reduces token usage.
func WithPrefixCache(contextID string) model.Option {
	return model.WrapImplSpecificOptFn(func(o *arkOptions) {
		o.contextID = &contextID
	})
}

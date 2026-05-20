/*
 * Copyright 2026 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package agenticclaude

import "github.com/cloudwego/eino/components/model"

type claudeOptions struct {
	serverTools   []*ServerToolConfig
	customHeaders map[string]string
	extraFields   map[string]any
}

// WithServerTools specifies server-side tools available to the model.
func WithServerTools(tools []*ServerToolConfig) model.Option {
	return model.WrapImplSpecificOptFn(func(o *claudeOptions) {
		o.serverTools = tools
	})
}

// WithCustomHeaders specifies custom HTTP headers to include in API requests.
func WithCustomHeaders(headers map[string]string) model.Option {
	return model.WrapImplSpecificOptFn(func(o *claudeOptions) {
		o.customHeaders = headers
	})
}

// WithExtraFields specifies additional fields that will be directly added to the HTTP request body.
func WithExtraFields(fields map[string]any) model.Option {
	return model.WrapImplSpecificOptFn(func(o *claudeOptions) {
		o.extraFields = fields
	})
}

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

package litellm

import (
	"github.com/cloudwego/eino-ext/libs/acl/openai"
	"github.com/cloudwego/eino/components/model"
)

// WithExtraFields sets extra body fields for the request.
// These fields will be merged into the top-level JSON request body, overriding any existing fields with the same key.
// Useful for passing LiteLLM-specific parameters like "metadata", "tags", "drop_params", or "force_timeout".
func WithExtraFields(extraFields map[string]any) model.Option {
	return openai.WithExtraFields(extraFields)
}

// WithExtraHeader sets extra headers for the request.
// Useful for LiteLLM proxy routing and tag-based routing.
func WithExtraHeader(header map[string]string) model.Option {
	return openai.WithExtraHeader(header)
}

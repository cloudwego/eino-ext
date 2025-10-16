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
	arkModel "github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"

	"github.com/cloudwego/eino/components/model"
)

type arkOptions struct {
	customHeaders map[string]string

	thinking *arkModel.Thinking

	cache *CacheOption
}

// WithCustomHeader sets custom headers for a single request
// the headers will override all the headers given in ChatModelConfig.CustomHeader
func WithCustomHeader(m map[string]string) model.Option {
	return model.WrapImplSpecificOptFn(func(o *arkOptions) {
		o.customHeaders = m
	})
}

// WithThinking sets the thinking process configuration for the ark.
func WithThinking(thinking *arkModel.Thinking) model.Option {
	return model.WrapImplSpecificOptFn(func(o *arkOptions) {
		o.thinking = thinking
	})
}

// Deprecated: use WithCache instead.
// WithPrefixCache creates an option to specify a context ID for the request.
// The context ID is typically obtained from a previous call to CreatePrefixCache.
//
// When this option is provided, the model will use the cached prefix context
// associated with this ID, allowing you to avoid resending the same context
// messages in each request, which improves efficiency and reduces token usage.
//
// Note: it is unavailable for doubao models of version 1.6 and above.
func WithPrefixCache(contextID string) model.Option {
	return WithCache(&CacheOption{
		ContextID: &contextID,
		APIType:   ContextAPI,
	})
}

type CacheOption struct {
	// See [CacheConfig.APIType]
	// Required.
	APIType APIType

	// ContextID is the unique identifier returned by ContextAPI.
	// Note: This field is only applicable when using ContextAPI.
	// Important: ContextID will not be compatible with response ID from ResponsesAPI in future releases.
	// For prefix caching with ResponsesAPI, use HeadPreviousResponseID instead.
	// For session caching with ResponsesAPI, use SessionCache instead.
	// Optional.
	ContextID *string

	// HeadPreviousResponseID is a response ID from a previous ResponsesAPI call.
	// This ID links the current request to a previous conversation context, enabling
	// features like conversation continuation and prefix caching.
	// The referenced response must be cached before use.
	// Only applicable for ResponsesAPI.
	// Optional.
	HeadPreviousResponseID *string

	// SessionCache is the configuration of ResponsesAPI session cache.
	// Optional.
	SessionCache *SessionCacheConfig
}

// WithCache is an option to configure model caching.
func WithCache(cache *CacheOption) model.Option {
	return model.WrapImplSpecificOptFn(func(o *arkOptions) {
		o.cache = cache
	})
}

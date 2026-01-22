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

package agenticark

import (
	"github.com/cloudwego/eino/components/model"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model/responses"
)

type arkOptions struct {
	reasoning         *responses.ResponsesReasoning
	thinking          *responses.ResponsesThinking
	maxToolCalls      *int64
	parallelToolCalls *bool
	text              *responses.ResponsesText

	serverTools []*ServerToolConfig
	mcpTools    []*responses.ToolMcp

	customHeaders map[string]string
	cache         *CacheOption
}

type CacheOption struct {
	// HeadPreviousResponseID is a response ID from a previous ResponsesAPI call.
	// This ID links the current request to a previous conversation context, enabling
	// features like conversation continuation and prefix caching.
	// The referenced response must be cached before use.
	// Optional.
	HeadPreviousResponseID *string

	// SessionCache is the configuration of ResponsesAPI session cache.
	// Optional.
	SessionCache *SessionCacheConfig
}

func WithReasoning(reasoning *responses.ResponsesReasoning) model.Option {
	return model.WrapImplSpecificOptFn(func(o *arkOptions) {
		o.reasoning = reasoning
	})
}

func WithThinking(thinking *responses.ResponsesThinking) model.Option {
	return model.WrapImplSpecificOptFn(func(o *arkOptions) {
		o.thinking = thinking
	})
}

func WithText(text *responses.ResponsesText) model.Option {
	return model.WrapImplSpecificOptFn(func(o *arkOptions) {
		o.text = text
	})
}

func WithMaxToolCalls(maxToolCalls int64) model.Option {
	return model.WrapImplSpecificOptFn(func(o *arkOptions) {
		o.maxToolCalls = &maxToolCalls
	})
}

func WithParallelToolCalls(parallelToolCalls bool) model.Option {
	return model.WrapImplSpecificOptFn(func(o *arkOptions) {
		o.parallelToolCalls = &parallelToolCalls
	})
}

func WithServerTools(tools []*ServerToolConfig) model.Option {
	return model.WrapImplSpecificOptFn(func(o *arkOptions) {
		o.serverTools = tools
	})
}

func WithMCPTools(tools []*responses.ToolMcp) model.Option {
	return model.WrapImplSpecificOptFn(func(o *arkOptions) {
		o.mcpTools = tools
	})
}

func WithCustomHeaders(headers map[string]string) model.Option {
	return model.WrapImplSpecificOptFn(func(o *arkOptions) {
		o.customHeaders = headers
	})
}

func WithCache(option *CacheOption) model.Option {
	return model.WrapImplSpecificOptFn(func(o *arkOptions) {
		o.cache = option
	})
}

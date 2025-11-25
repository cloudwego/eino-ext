/*
 * Copyright 2025 CloudWeGo Authors
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

package ark

import (
	"github.com/cloudwego/eino/components/agentic"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model/responses"
)

type arkOptions struct {
	reasoning         *responses.ResponsesReasoning
	thinking          *responses.ResponsesThinking
	maxOutputTokens   *int64
	maxToolCalls      *int64
	parallelToolCalls *bool
	text              *responses.ResponsesText

	serverTools      []*ServerToolConfig
	forcedServerTool *ForcedServerTool

	mcpTools      []*responses.ToolMcp
	forcedMCPTool *responses.McpToolChoice

	customHeaders map[string]string
	cache         *CacheOption
}

type CacheOption struct {
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

type ForcedServerTool struct {
	WebSearch *responses.WebSearchToolChoice
}

func WithReasoning(reasoning *responses.ResponsesReasoning) agentic.Option {
	return agentic.WrapImplSpecificOptFn(func(o *arkOptions) {
		o.reasoning = reasoning
	})
}

func WithThinking(thinking *responses.ResponsesThinking) agentic.Option {
	return agentic.WrapImplSpecificOptFn(func(o *arkOptions) {
		o.thinking = thinking
	})
}

func WithText(text *responses.ResponsesText) agentic.Option {
	return agentic.WrapImplSpecificOptFn(func(o *arkOptions) {
		o.text = text
	})
}

func WithMaxOutputTokens(maxOutputTokens int64) agentic.Option {
	return agentic.WrapImplSpecificOptFn(func(o *arkOptions) {
		o.maxOutputTokens = &maxOutputTokens
	})
}

func WithMaxToolCalls(maxToolCalls int64) agentic.Option {
	return agentic.WrapImplSpecificOptFn(func(o *arkOptions) {
		o.maxToolCalls = &maxToolCalls
	})
}

func WithParallelToolCalls(parallelToolCalls bool) agentic.Option {
	return agentic.WrapImplSpecificOptFn(func(o *arkOptions) {
		o.parallelToolCalls = &parallelToolCalls
	})
}

func WithForcedServerTool(tool *ForcedServerTool) agentic.Option {
	return agentic.WrapImplSpecificOptFn(func(o *arkOptions) {
		o.forcedServerTool = tool
	})
}

func WithServerTools(tools []*ServerToolConfig) agentic.Option {
	return agentic.WrapImplSpecificOptFn(func(o *arkOptions) {
		o.serverTools = tools
	})
}

func WithForcedMCPTool(tool *responses.McpToolChoice) agentic.Option {
	return agentic.WrapImplSpecificOptFn(func(o *arkOptions) {
		o.forcedMCPTool = tool
	})
}

func WithMCPTools(tools []*responses.ToolMcp) agentic.Option {
	return agentic.WrapImplSpecificOptFn(func(o *arkOptions) {
		o.mcpTools = tools
	})
}

func WithCustomHeaders(headers map[string]string) agentic.Option {
	return agentic.WrapImplSpecificOptFn(func(o *arkOptions) {
		o.customHeaders = headers
	})
}

func WithCache(option *CacheOption) agentic.Option {
	return agentic.WrapImplSpecificOptFn(func(o *arkOptions) {
		o.cache = option
	})
}

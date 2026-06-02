/*
 * Copyright 2026 CloudWeGo Authors
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

package agenticopenai

import (
	"github.com/cloudwego/eino/components/model"
	"github.com/openai/openai-go/v3/responses"
)

type options struct {
	// Common fields (used by both chat and responses models)
	customHeaders map[string]string
	extraFields   map[string]any

	// Responses-specific fields
	reasoning              *responses.ReasoningParam
	maxToolCalls           *int
	parallelToolCalls      *bool
	text                   *responses.ResponseTextConfigParam
	store                  *bool
	promptCacheKey         *string
	headPreviousResponseID *string
	truncation             *responses.ResponseNewParamsTruncation

	serverTools []*ResponsesServerToolConfig
	mcpTools    []*responses.ToolMcpParam
}

// WithCustomHeaders sets custom HTTP headers for the request.
func WithCustomHeaders(headers map[string]string) model.Option {
	return model.WrapImplSpecificOptFn(func(o *options) {
		o.customHeaders = headers
	})
}

// WithExtraFields sets extra fields to include in the request body.
// These fields will be merged into the top-level JSON request body, overriding any existing fields with the same key.
//
// Example:
//
//	WithExtraFields(map[string]any{
//	    "reasoning_effort": "high",
//	    "service_tier": "default",
//	})
//
// The resulting request body will be:
//
//	{
//	    "model": "o1",
//	    "input": [...],
//	    "reasoning_effort": "high",
//	    "service_tier": "default"
//	}
func WithExtraFields(fields map[string]any) model.Option {
	return model.WrapImplSpecificOptFn(func(o *options) {
		o.extraFields = fields
	})
}

// WithHeadPreviousResponseID sets a response ID from a previous Responses API call.
// This ID links the current request to a previous conversation context, enabling
// features like conversation continuation and prefix caching.
// In populateCache, an auto-discovered response ID from input messages takes
// priority over this option.
// The referenced response must be cached before use.
// Available only for Responses API.
func WithHeadPreviousResponseID(id string) model.Option {
	return model.WrapImplSpecificOptFn(func(o *options) {
		o.headPreviousResponseID = &id
	})
}

// WithResponsesStore sets whether to store the response on the server.
// Available only for Responses API.
func WithResponsesStore(store bool) model.Option {
	return model.WrapImplSpecificOptFn(func(o *options) {
		o.store = &store
	})
}

// WithResponsesPromptCacheKey sets the prompt cache key for the request.
// Available only for Responses API.
func WithResponsesPromptCacheKey(key string) model.Option {
	return model.WrapImplSpecificOptFn(func(o *options) {
		o.promptCacheKey = &key
	})
}

// WithResponsesReasoning sets the reasoning configuration for the request.
// Available only for Responses API.
func WithResponsesReasoning(reasoning *responses.ReasoningParam) model.Option {
	return model.WrapImplSpecificOptFn(func(o *options) {
		o.reasoning = reasoning
	})
}

// WithResponsesText sets the text generation configuration for the request.
// Available only for Responses API.
func WithResponsesText(text *responses.ResponseTextConfigParam) model.Option {
	return model.WrapImplSpecificOptFn(func(o *options) {
		o.text = text
	})
}

// WithResponsesMaxToolCalls sets the maximum number of tool calls allowed in a single turn.
// Available only for Responses API.
func WithResponsesMaxToolCalls(maxToolCalls int) model.Option {
	return model.WrapImplSpecificOptFn(func(o *options) {
		o.maxToolCalls = &maxToolCalls
	})
}

// WithResponsesParallelToolCalls sets whether to allow multiple tool calls in a single turn.
// Available only for Responses API.
func WithResponsesParallelToolCalls(parallelToolCalls bool) model.Option {
	return model.WrapImplSpecificOptFn(func(o *options) {
		o.parallelToolCalls = &parallelToolCalls
	})
}

// WithResponsesServerTools sets server-side tools available to the model.
// Available only for Responses API.
func WithResponsesServerTools(tools []*ResponsesServerToolConfig) model.Option {
	return model.WrapImplSpecificOptFn(func(o *options) {
		o.serverTools = tools
	})
}

// WithResponsesMCPTools sets Model Context Protocol tools available to the model.
// Available only for Responses API.
func WithResponsesMCPTools(tools []*responses.ToolMcpParam) model.Option {
	return model.WrapImplSpecificOptFn(func(o *options) {
		o.mcpTools = tools
	})
}

// WithResponsesTruncation sets how to handle context that exceeds the model's context window.
// Available only for Responses API.
func WithResponsesTruncation(truncation responses.ResponseNewParamsTruncation) model.Option {
	return model.WrapImplSpecificOptFn(func(o *options) {
		o.truncation = &truncation
	})
}

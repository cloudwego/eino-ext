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

package agenticopenai

import (
	"github.com/cloudwego/eino/components/model"
	"github.com/openai/openai-go/v3/responses"
)

type openaiOptions struct {
	reasoning         *responses.ReasoningParam
	maxToolCalls      *int
	parallelToolCalls *bool
	text              *responses.ResponseTextConfigParam
	store             *bool
	promptCacheKey    *string

	serverTools []*ServerToolConfig
	mcpTools    []*responses.ToolMcpParam

	customHeaders map[string]string
	extraFields   map[string]any
}

func WithStore(store bool) model.Option {
	return model.WrapImplSpecificOptFn(func(o *openaiOptions) {
		o.store = &store
	})
}

func WithPromptCacheKey(key string) model.Option {
	return model.WrapImplSpecificOptFn(func(o *openaiOptions) {
		o.promptCacheKey = &key
	})
}

func WithReasoning(reasoning *responses.ReasoningParam) model.Option {
	return model.WrapImplSpecificOptFn(func(o *openaiOptions) {
		o.reasoning = reasoning
	})
}

func WithText(text *responses.ResponseTextConfigParam) model.Option {
	return model.WrapImplSpecificOptFn(func(o *openaiOptions) {
		o.text = text
	})
}

func WithMaxToolCalls(maxToolCalls int) model.Option {
	return model.WrapImplSpecificOptFn(func(o *openaiOptions) {
		o.maxToolCalls = &maxToolCalls
	})
}

func WithParallelToolCalls(parallelToolCalls bool) model.Option {
	return model.WrapImplSpecificOptFn(func(o *openaiOptions) {
		o.parallelToolCalls = &parallelToolCalls
	})
}

func WithServerTools(tools []*ServerToolConfig) model.Option {
	return model.WrapImplSpecificOptFn(func(o *openaiOptions) {
		o.serverTools = tools
	})
}

func WithMCPTools(tools []*responses.ToolMcpParam) model.Option {
	return model.WrapImplSpecificOptFn(func(o *openaiOptions) {
		o.mcpTools = tools
	})
}

func WithCustomHeaders(headers map[string]string) model.Option {
	return model.WrapImplSpecificOptFn(func(o *openaiOptions) {
		o.customHeaders = headers
	})
}

func WithExtraFields(fields map[string]any) model.Option {
	return model.WrapImplSpecificOptFn(func(o *openaiOptions) {
		o.extraFields = fields
	})
}

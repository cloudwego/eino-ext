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

package ark

import (
	"github.com/cloudwego/eino/components/model"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/responses"
)

// WithCache overrides the Responses API session-cache configuration for one call.
func WithCache(cache *CacheOption) model.Option {
	return model.WrapImplSpecificOptFn(func(o *arkOptions) { o.cache = cache })
}

// WithEnableToolWebSearch configures the Responses API's built-in search tool.
// A nil value disables it for this call.
func WithEnableToolWebSearch(tool *responses.WebSearchTool) model.Option {
	return model.WrapImplSpecificOptFn(func(o *arkOptions) { o.enableWebSearch = tool })
}

func WithMaxToolCalls(n int64) model.Option {
	return model.WrapImplSpecificOptFn(func(o *arkOptions) { o.maxToolCalls = &n })
}

// WithResponsesThinking configures thinking for the Responses API.
func WithResponsesThinking(thinking *responses.Thinking) model.Option {
	return model.WrapImplSpecificOptFn(func(o *arkOptions) { o.responsesThinking = thinking })
}

// WithResponsesReasoningEffort configures reasoning effort for the Responses API.
func WithResponsesReasoningEffort(effort responses.ReasoningEffort) model.Option {
	return model.WrapImplSpecificOptFn(func(o *arkOptions) { o.responsesReasoningEffort = &effort })
}

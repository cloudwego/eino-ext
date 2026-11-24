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
	"github.com/volcengine/ark-runtime-go/arkruntime/model/chat"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/responses"
)

type arkOptions struct {
	cache                    *CacheOption
	enableWebSearch          *responses.WebSearchTool
	maxToolCalls             *int64
	customHeaders            map[string]string
	thinking                 *chat.Thinking
	responsesThinking        *responses.Thinking
	reasoningEffort          *chat.ReasoningEffort
	responsesReasoningEffort *responses.ReasoningEffort
	maxCompletionTokens      *int
	parallelToolCalls        *bool
}

// WithCustomHeader replaces the configured custom headers for one call.
func WithCustomHeader(headers map[string]string) model.Option {
	return model.WrapImplSpecificOptFn(func(o *arkOptions) { o.customHeaders = headers })
}

// WithParallelToolCalls controls parallel tool calls for Chat Completions.
func WithParallelToolCalls(enabled bool) model.Option {
	return model.WrapImplSpecificOptFn(func(o *arkOptions) { o.parallelToolCalls = &enabled })
}

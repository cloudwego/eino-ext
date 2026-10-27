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

package glm

import (
	"github.com/cloudwego/eino-ext/libs/acl/openai"
	"github.com/cloudwego/eino/components/model"
)

// options holds GLM-specific call options.
type options struct {
	// EnableThinking toggles GLM deep-thinking (reasoning) mode.
	EnableThinking *bool

	// extraFields are merged into the top-level request body.
	extraFields map[string]any
}

// WithThinking sets whether GLM deep-thinking (reasoning) mode is enabled for the call.
// Only effective on reasoning-capable models such as glm-4.6 and glm-4.5-flash.
// The reasoning trace is returned via schema.Message.ReasoningContent.
func WithThinking(enable bool) model.Option {
	return model.WrapImplSpecificOptFn(func(opt *options) {
		opt.EnableThinking = &enable
	})
}

// WithWebSearch enables GLM's built-in web search tool for the call.
//
// GLM delivers web search as a special entry in the request `tools` array
// (`{"type":"web_search","web_search":{"enable":true}}`). It is exposed here via
// extra fields for convenience.
//
// Note: because it is injected through the top-level `tools` field, enabling web
// search alongside eino function tools (BindTools/WithTools) is not supported in a
// single call — the two would overwrite each other. Use it on calls without bound
// function tools.
func WithWebSearch(enable bool) model.Option {
	return model.WrapImplSpecificOptFn(func(opt *options) {
		if opt.extraFields == nil {
			opt.extraFields = make(map[string]any)
		}
		opt.extraFields["tools"] = []map[string]any{
			{
				"type": "web_search",
				"web_search": map[string]any{
					"enable": enable,
				},
			},
		}
	})
}

// WithExtraFields sets extra top-level fields for the request body.
// These fields are merged into the top-level JSON request body, overriding any
// existing fields with the same key.
func WithExtraFields(extraFields map[string]any) model.Option {
	return model.WrapImplSpecificOptFn(func(opt *options) {
		if opt.extraFields == nil {
			opt.extraFields = make(map[string]any)
		}
		for k, v := range extraFields {
			opt.extraFields[k] = v
		}
	})
}

// WithExtraHeader sets extra headers for the request.
func WithExtraHeader(header map[string]string) model.Option {
	return openai.WithExtraHeader(header)
}

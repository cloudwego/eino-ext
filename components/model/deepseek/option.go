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

package deepseek

import (
	"github.com/cloudwego/eino/components/model"
)

type deepseekOptions struct {
	// extraFields carries arbitrary passthrough fields that will be merged into the
	// top-level JSON payload of a chat completion request.
	//
	// It is useful when the upstream DeepSeek API introduces new request parameters
	// that are not yet modeled as first-class fields by this component (or by the
	// underlying deepseek-go SDK).
	extraFields map[string]interface{}
}

// WithExtraFields returns a request-level option that merges the provided
// key/value pairs into the top-level JSON payload sent to the DeepSeek API.
//
// Keys that collide with fields already populated by this component (e.g.
// "model", "messages", "temperature", "thinking", ...) will override them,
// which mirrors the behavior of the underlying deepseek-go SDK.
//
// Passing a nil or empty map is a no-op.
//
// Example:
//
//	msg, err := cm.Generate(ctx, in,
//	    deepseek.WithExtraFields(map[string]interface{}{
//	        "chat_template_kwargs": map[string]interface{}{
//	            "thinking": true,
//	        },
//	    }),
//	)
func WithExtraFields(extraFields map[string]interface{}) model.Option {
	return model.WrapImplSpecificOptFn(func(o *deepseekOptions) {
		o.extraFields = extraFields
	})
}

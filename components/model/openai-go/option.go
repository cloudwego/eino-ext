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

package openaigo

import "github.com/cloudwego/eino/components/model"

type options struct {
	MaxOutputTokens *int
	Reasoning       *Reasoning
	Store           *bool
	Metadata        map[string]string
	ExtraFields     map[string]any
}

// WithMaxOutputTokens sets max_output_tokens for the Responses API.
func WithMaxOutputTokens(n int) model.Option {
	return model.WrapImplSpecificOptFn(func(o *options) {
		o.MaxOutputTokens = &n
	})
}

// WithReasoning overrides the reasoning config for this request.
func WithReasoning(r *Reasoning) model.Option {
	return model.WrapImplSpecificOptFn(func(o *options) {
		o.Reasoning = r
	})
}

// WithStore sets whether to store the response.
func WithStore(store bool) model.Option {
	return model.WrapImplSpecificOptFn(func(o *options) {
		o.Store = &store
	})
}

// WithMetadata overrides request metadata.
func WithMetadata(m map[string]string) model.Option {
	return model.WrapImplSpecificOptFn(func(o *options) {
		o.Metadata = cloneStringMap(m)
	})
}

// WithExtraFields injects extra fields into the request body.
// Extra fields overwrite any existing fields with the same key.
func WithExtraFields(extra map[string]any) model.Option {
	return model.WrapImplSpecificOptFn(func(o *options) {
		o.ExtraFields = cloneAnyMap(extra)
	})
}

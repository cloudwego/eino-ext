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

package openrouter

import "github.com/cloudwego/eino/components/model"

type openrouterOption struct {
	models    []string
	reasoning *Reasoning
	metadata  map[string]string
}

// WithModels provider an array of model IDs in priority order.
// If the first model returns an error, OpenRouter will automatically try the next model in the list
func WithModels(models []string) model.Option {
	return model.WrapImplSpecificOptFn(func(o *openrouterOption) {
		o.models = models
	})
}

// WithReasoning provider advanced reasoning capabilities,
// allowing models to show their internal reasoning process with configurable effort、summary fields levels
func WithReasoning(r *Reasoning) model.Option {
	return model.WrapImplSpecificOptFn(func(o *openrouterOption) {
		o.reasoning = r
	})
}

// WithMetadata Set of 16 key-value pairs that can be attached to an object.
// This can be useful for storing additional information about the object in a structured format,
// and querying for objects via API or the dashboard.
// Keys are strings with a maximum length of 64 characters.
// Values are strings with a maximum length of 512 characters.
func WithMetadata(m map[string]string) model.Option {
	return model.WrapImplSpecificOptFn(func(o *openrouterOption) {
		o.metadata = m
	})
}

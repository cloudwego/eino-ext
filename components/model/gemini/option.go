/*
 * Copyright 2024 CloudWeGo Authors
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

package gemini

import (
	"github.com/eino-contrib/jsonschema"
	"github.com/getkin/kin-openapi/openapi3"
	"google.golang.org/genai"

	"github.com/cloudwego/eino/components/model"
)

type options struct {
	TopK               *int32
	ResponseSchema     *openapi3.Schema
	ResponseJSONSchema *jsonschema.Schema
	ThinkingConfig     *genai.ThinkingConfig
	ResponseModalities []GeminiResponseModality
}

func WithTopK(k int32) model.Option {
	return model.WrapImplSpecificOptFn(func(o *options) {
		o.TopK = &k
	})
}

func WithResponseSchema(s *openapi3.Schema) model.Option {
	return model.WrapImplSpecificOptFn(func(o *options) {
		o.ResponseSchema = s
	})
}

func WithResponseJSONSchema(s *jsonschema.Schema) model.Option {
	return model.WrapImplSpecificOptFn(func(o *options) {
		o.ResponseJSONSchema = s
	})
}

func WithThinkingConfig(t *genai.ThinkingConfig) model.Option {
	return model.WrapImplSpecificOptFn(func(o *options) {
		o.ThinkingConfig = t
	})
}

func WithResponseModalities(m []GeminiResponseModality) model.Option {
	return model.WrapImplSpecificOptFn(func(o *options) {
		o.ResponseModalities = m
	})
}

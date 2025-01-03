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
	"github.com/cloudwego/eino/components/model"
	"github.com/getkin/kin-openapi/openapi3"
)

type options struct {
	TopK           *int32
	ResponseSchema *openapi3.Schema
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

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

package openai

import (
	"github.com/meguminnnnnnnnn/go-openai"

	"github.com/cloudwego/eino/components/model"
)

// https://platform.openai.com/docs/api-reference/chat/create#chat-create-reasoning_effort
type ReasoningEffortLevel string

const (
	ReasoningEffortLevelLow    ReasoningEffortLevel = "low"
	ReasoningEffortLevelMedium ReasoningEffortLevel = "medium"
	ReasoningEffortLevelHigh   ReasoningEffortLevel = "high"
)

type openaiOptions struct {
	ExtraFields         map[string]any
	ReasoningEffort     ReasoningEffortLevel
	ExtraHeader         map[string]string
	RequestBodyModifier openai.RequestBodyModifier
	MaxCompletionTokens *int
}

func WithExtraFields(extraFields map[string]any) model.Option {
	return model.WrapImplSpecificOptFn(func(o *openaiOptions) {
		o.ExtraFields = extraFields
	})
}

func WithReasoningEffort(re ReasoningEffortLevel) model.Option {
	return model.WrapImplSpecificOptFn(func(o *openaiOptions) {
		o.ReasoningEffort = re
	})
}

// WithRequestBodyModifier is used to modify the request body before sending request.
// Useful for compatibility with custom fields when calling other models using OpenAI API.
func WithRequestBodyModifier(modifier openai.RequestBodyModifier) model.Option {
	return model.WrapImplSpecificOptFn(func(o *openaiOptions) {
		o.RequestBodyModifier = modifier
	})
}

// WithExtraHeader is used to set extra headers for the request.
func WithExtraHeader(header map[string]string) model.Option {
	return model.WrapImplSpecificOptFn(func(o *openaiOptions) {
		o.ExtraHeader = header
	})
}

func WithMaxCompletionTokens(maxCompletionTokens int) model.Option {
	return model.WrapImplSpecificOptFn(func(o *openaiOptions) {
		o.MaxCompletionTokens = &maxCompletionTokens
	})
}

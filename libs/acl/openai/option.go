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
	"context"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/meguminnnnnnnnn/go-openai"
)

// ReasoningEffortLevel see: https://platform.openai.com/docs/api-reference/chat/create#chat-create-reasoning_effort
type ReasoningEffortLevel string

const (
	ReasoningEffortLevelLow    ReasoningEffortLevel = "low"
	ReasoningEffortLevelMedium ReasoningEffortLevel = "medium"
	ReasoningEffortLevelHigh   ReasoningEffortLevel = "high"
)

// RequestPayloadModifier transforms the serialized request payload
// with access to input messages and the raw payload.
type RequestPayloadModifier func(ctx context.Context, msg []*schema.Message, rawBody []byte) ([]byte, error)

// ResponseMessageModifier transforms the generated message using the raw response body.
// It must return the final message.
type ResponseMessageModifier func(ctx context.Context, msg *schema.Message, rawBody []byte) (*schema.Message, error)

// ResponseChunkMessageModifier transforms the generated message chunk using the raw response body.
// When end is true, msg and rawBody may be nil.
type ResponseChunkMessageModifier func(ctx context.Context, msg *schema.Message, rawBody []byte, end bool) (*schema.Message, error)

type openaiOptions struct {
	ExtraFields                  map[string]any
	ReasoningEffort              ReasoningEffortLevel
	ExtraHeader                  map[string]string
	RequestBodyModifier          openai.RequestBodyModifier
	RequestPayloadModifier       RequestPayloadModifier
	ResponseMessageModifier      ResponseMessageModifier
	ResponseChunkMessageModifier ResponseChunkMessageModifier
	MaxCompletionTokens          *int
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

// WithRequestPayloadModifier registers a payload modifier to customize
// the serialized request based on input messages.
func WithRequestPayloadModifier(modifier RequestPayloadModifier) model.Option {
	return model.WrapImplSpecificOptFn(func(o *openaiOptions) {
		o.RequestPayloadModifier = modifier
	})
}

// WithResponseMessageModifier registers a message modifier to transform
// the output message using the raw response body.
func WithResponseMessageModifier(m ResponseMessageModifier) model.Option {
	return model.WrapImplSpecificOptFn(func(o *openaiOptions) {
		o.ResponseMessageModifier = m
	})
}

// WithResponseChunkMessageModifier registers a message modifier to transform
// the output message chunk using the raw response body.
func WithResponseChunkMessageModifier(m ResponseChunkMessageModifier) model.Option {
	return model.WrapImplSpecificOptFn(func(o *openaiOptions) {
		o.ResponseChunkMessageModifier = m
	})
}

// WithRequestBodyModifier modifies the request body before sending the request.
// Deprecated: Use WithRequestPayloadModifier.
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

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

package openai

import (
	"github.com/cloudwego/eino-ext/libs/acl/openai"
	"github.com/cloudwego/eino/components/model"
)

// RequestPayloadModifier transforms the serialized request payload
// with access to input messages and the raw payload.
type RequestPayloadModifier = openai.RequestPayloadModifier

// ResponseMessageModifier transforms the generated message using the raw response body.
// It must return the final message.
type ResponseMessageModifier = openai.ResponseMessageModifier

// ResponseChunkMessageModifier transforms the generated message chunk using the raw response body.
// When end is true, msg and rawBody may be nil.
type ResponseChunkMessageModifier = openai.ResponseChunkMessageModifier

// WithExtraFields is used to set extra body fields for the request.
func WithExtraFields(extraFields map[string]any) model.Option {
	return openai.WithExtraFields(extraFields)
}

// WithExtraHeader is used to set extra headers for the request.
func WithExtraHeader(header map[string]string) model.Option {
	return openai.WithExtraHeader(header)
}

func WithReasoningEffort(effort ReasoningEffortLevel) model.Option {
	return openai.WithReasoningEffort(openai.ReasoningEffortLevel(effort))
}

// WithMaxCompletionTokens is used to set the max completion tokens for the request.
func WithMaxCompletionTokens(maxCompletionTokens int) model.Option {
	return openai.WithMaxCompletionTokens(maxCompletionTokens)
}

// WithRequestPayloadModifier registers a payload modifier to customize
// the serialized request based on input messages.
// This is useful for OpenAI-compatible providers that require extra fields
// beyond what the standard API supports.
func WithRequestPayloadModifier(modifier RequestPayloadModifier) model.Option {
	return openai.WithRequestPayloadModifier(modifier)
}

// WithResponseMessageModifier registers a message modifier to transform
// the output message using the raw response body.
// This is useful for extracting provider-specific fields from the response.
func WithResponseMessageModifier(m ResponseMessageModifier) model.Option {
	return openai.WithResponseMessageModifier(m)
}

// WithResponseChunkMessageModifier registers a message modifier to transform
// the output message chunk using the raw response body.
// When end is true, msg and rawBody may be nil.
func WithResponseChunkMessageModifier(m ResponseChunkMessageModifier) model.Option {
	return openai.WithResponseChunkMessageModifier(m)
}

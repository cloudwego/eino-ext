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

// ToolInfoExtraKeyStrict is the schema.ToolInfo.Extra key used to opt a
// single tool definition into OpenAI Structured Outputs strict mode.
//
// Set the value to true to request the model emit tool-call arguments
// that strictly match the tool's JSON Schema. Supported by OpenAI's
// Structured Outputs for tools and by DeepSeek's tool-call strict mode.
// Without strict, providers occasionally emit syntactically invalid
// JSON in tool-call arguments (e.g. unescaped control bytes inside
// string values), which downstream JSON parsers reject.
//
// Sourced via schema.ToolInfo.Extra because schema.ToolInfo has no
// dedicated Strict field; using Extra keeps this provider-specific
// knob inside the openai package boundary and avoids changes to
// cloudwego/eino core.
//
// Example:
//
//	toolInfo := &schema.ToolInfo{
//	    Name:        "extract_facts",
//	    Desc:        "Extract structured facts from the user's text.",
//	    ParamsOneOf: schema.NewParamsOneOfByJSONSchema(mySchema),
//	    Extra: map[string]any{
//	        openai.ToolInfoExtraKeyStrict: true,
//	    },
//	}
//
// Non-bool values at this key are silently ignored.
const ToolInfoExtraKeyStrict = "openai_strict"

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
	ResponseFormat               *ChatCompletionResponseFormat
}

// WithExtraFields sets extra fields to include in the request body.
// These fields will be merged into the top-level JSON request body, overriding any existing fields with the same key.
//
// Example:
//
//	WithExtraFields(map[string]any{
//	    "reasoning_effort": "high",
//	    "service_tier": "default",
//	})
//
// The resulting request body will be:
//
//	{
//	    "model": "o1",
//	    "messages": [...],
//	    "reasoning_effort": "high",
//	    "service_tier": "default"
//	}
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

// WithResponseFormat sets the response_format for a single chat completion
// request, overriding any Config.ResponseFormat set at NewChatModel time.
//
// This is the per-call counterpart of Config.ResponseFormat. It is useful
// when a single shared ChatModel must mix structured-output calls (e.g.
// DeepSeek json_object mode for a structured-extraction sidecar) with
// free-form text calls that must NOT set response_format.
//
// Pass nil to leave the request's response_format alone and fall back to
// Config.ResponseFormat (the existing behavior).
//
// Example (DeepSeek JSON Output, per-call):
//
//	resp, err := cm.Generate(ctx, messages,
//	    openai.WithResponseFormat(&openai.ChatCompletionResponseFormat{
//	        Type: openai.ChatCompletionResponseFormatTypeJSONObject,
//	    }),
//	)
//
// Example (OpenAI Structured Outputs with a JSON Schema):
//
//	resp, err := cm.Generate(ctx, messages,
//	    openai.WithResponseFormat(&openai.ChatCompletionResponseFormat{
//	        Type: openai.ChatCompletionResponseFormatTypeJSONSchema,
//	        JSONSchema: &openai.ChatCompletionResponseFormatJSONSchema{
//	            Name:   "weather_response",
//	            Schema: mySchema,
//	            Strict: true,
//	        },
//	    }),
//	)
func WithResponseFormat(format *ChatCompletionResponseFormat) model.Option {
	return model.WrapImplSpecificOptFn(func(o *openaiOptions) {
		o.ResponseFormat = format
	})
}

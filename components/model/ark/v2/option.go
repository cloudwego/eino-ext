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

package ark

import (
	"context"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/chat"
)

// RequestPayloadModifier transforms the serialized JSON request using the input
// messages. The result must be an object containing model and messages. Generate
// and Stream control stream flags; n must be omitted or 1. The modifier runs once
// per call, before SDK retries, and must support concurrent calls on a shared model.
type RequestPayloadModifier func(ctx context.Context, messages []*schema.Message, rawBody []byte) ([]byte, error)

// ResponseMessageModifier transforms Generate output before callbacks receive it.
// rawBody contains the original response JSON, or nil for synchronous batch calls
// because the SDK's batch client does not expose it. The result must not be nil.
type ResponseMessageModifier func(ctx context.Context, message *schema.Message, rawBody []byte) (*schema.Message, error)

// ResponseChunkMessageModifier transforms Stream output before callbacks receive
// it. message may be nil for a chunk without a standard message projection.
// rawBody contains the original chunk JSON. After [DONE] it is called once with
// end true and nil message/rawBody, and may return a final message. A premature
// EOF is an error and does not trigger this final call. Returning nil skips a
// chunk; returning an error terminates the stream.
type ResponseChunkMessageModifier func(ctx context.Context, message *schema.Message, rawBody []byte, end bool) (*schema.Message, error)

type arkOptions struct {
	customHeaders                map[string]string
	thinking                     *chat.Thinking
	reasoningEffort              *chat.ReasoningEffort
	maxCompletionTokens          *int
	parallelToolCalls            *bool
	requestPayloadModifier       RequestPayloadModifier
	responseMessageModifier      ResponseMessageModifier
	responseChunkMessageModifier ResponseChunkMessageModifier
}

func (cm *ChatModel) getOptions(opts ...model.Option) (*model.Options, *arkOptions) {
	common := model.GetCommonOptions(&model.Options{
		Temperature: cm.config.Temperature,
		MaxTokens:   cm.config.MaxTokens,
		Model:       &cm.config.Model,
		TopP:        cm.config.TopP,
		Stop:        cm.config.Stop,
		ToolChoice:  cm.toolChoice,
	}, opts...)
	ark := model.GetImplSpecificOptions(&arkOptions{
		customHeaders:                cm.config.CustomHeader,
		thinking:                     cm.config.Thinking,
		reasoningEffort:              cm.config.ReasoningEffort,
		maxCompletionTokens:          cm.config.MaxCompletionTokens,
		parallelToolCalls:            cm.config.ParallelToolCalls,
		requestPayloadModifier:       cm.config.RequestPayloadModifier,
		responseMessageModifier:      cm.config.ResponseMessageModifier,
		responseChunkMessageModifier: cm.config.ResponseChunkMessageModifier,
	}, opts...)
	return common, ark
}

// WithCustomHeader replaces the configured custom headers for one call.
func WithCustomHeader(headers map[string]string) model.Option {
	return model.WrapImplSpecificOptFn(func(o *arkOptions) { o.customHeaders = headers })
}
func WithThinking(thinking *chat.Thinking) model.Option {
	return model.WrapImplSpecificOptFn(func(o *arkOptions) { o.thinking = thinking })
}
func WithReasoningEffort(effort chat.ReasoningEffort) model.Option {
	return model.WrapImplSpecificOptFn(func(o *arkOptions) { o.reasoningEffort = &effort })
}
func WithMaxCompletionTokens(n int) model.Option {
	return model.WrapImplSpecificOptFn(func(o *arkOptions) { o.maxCompletionTokens = &n })
}
func WithParallelToolCalls(enabled bool) model.Option {
	return model.WrapImplSpecificOptFn(func(o *arkOptions) { o.parallelToolCalls = &enabled })
}

// WithRequestPayloadModifier replaces the configured request payload modifier for
// one call. Passing nil disables the configured modifier.
func WithRequestPayloadModifier(modifier RequestPayloadModifier) model.Option {
	return model.WrapImplSpecificOptFn(func(o *arkOptions) { o.requestPayloadModifier = modifier })
}

// WithResponseMessageModifier replaces the configured Generate message modifier
// for one call. Passing nil disables the configured modifier.
func WithResponseMessageModifier(modifier ResponseMessageModifier) model.Option {
	return model.WrapImplSpecificOptFn(func(o *arkOptions) { o.responseMessageModifier = modifier })
}

// WithResponseChunkMessageModifier replaces the configured Stream chunk modifier
// for one call. Passing nil disables the configured modifier.
func WithResponseChunkMessageModifier(modifier ResponseChunkMessageModifier) model.Option {
	return model.WrapImplSpecificOptFn(func(o *arkOptions) { o.responseChunkMessageModifier = modifier })
}

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

package atlascloud

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	modelopenai "github.com/cloudwego/eino-ext/components/model/openai"
)

const (
	// DefaultBaseURL is Atlas Cloud's OpenAI-compatible LLM endpoint.
	DefaultBaseURL = "https://api.atlascloud.ai/v1"

	ChatCompletionResponseFormatTypeJSONObject = modelopenai.ChatCompletionResponseFormatTypeJSONObject
	ChatCompletionResponseFormatTypeJSONSchema = modelopenai.ChatCompletionResponseFormatTypeJSONSchema
	ChatCompletionResponseFormatTypeText       = modelopenai.ChatCompletionResponseFormatTypeText
)

type (
	ChatCompletionResponseFormat           = modelopenai.ChatCompletionResponseFormat
	ChatCompletionResponseFormatJSONSchema = modelopenai.ChatCompletionResponseFormatJSONSchema
	ChatModelConfig                        = modelopenai.ChatModelConfig
	RequestPayloadModifier                 = modelopenai.RequestPayloadModifier
	ResponseMessageModifier                = modelopenai.ResponseMessageModifier
	ResponseChunkMessageModifier           = modelopenai.ResponseChunkMessageModifier
	ReasoningEffortLevel                   = modelopenai.ReasoningEffortLevel
	Modality                               = modelopenai.Modality
	Audio                                  = modelopenai.Audio
	AudioFormat                            = modelopenai.AudioFormat
	AudioVoice                             = modelopenai.AudioVoice
)

const (
	ReasoningEffortLevelLow    = modelopenai.ReasoningEffortLevelLow
	ReasoningEffortLevelMedium = modelopenai.ReasoningEffortLevelMedium
	ReasoningEffortLevelHigh   = modelopenai.ReasoningEffortLevelHigh

	TextModality  = modelopenai.TextModality
	AudioModality = modelopenai.AudioModality

	AudioFormatMp3   = modelopenai.AudioFormatMp3
	AudioFormatWav   = modelopenai.AudioFormatWav
	AudioFormatFlac  = modelopenai.AudioFormatFlac
	AudioFormatOpus  = modelopenai.AudioFormatOpus
	AudioFormatPcm16 = modelopenai.AudioFormatPcm16

	AudioVoiceAlloy   = modelopenai.AudioVoiceAlloy
	AudioVoiceAsh     = modelopenai.AudioVoiceAsh
	AudioVoiceBallad  = modelopenai.AudioVoiceBallad
	AudioVoiceCoral   = modelopenai.AudioVoiceCoral
	AudioVoiceEcho    = modelopenai.AudioVoiceEcho
	AudioVoiceFable   = modelopenai.AudioVoiceFable
	AudioVoiceNova    = modelopenai.AudioVoiceNova
	AudioVoiceOnyx    = modelopenai.AudioVoiceOnyx
	AudioVoiceSage    = modelopenai.AudioVoiceSage
	AudioVoiceShimmer = modelopenai.AudioVoiceShimmer
)

// ChatModel wraps the OpenAI-compatible ChatModel with Atlas Cloud defaults.
type ChatModel struct {
	inner *modelopenai.ChatModel
}

var _ model.ToolCallingChatModel = (*ChatModel)(nil)

// NewChatModel creates an Atlas Cloud chat model using Atlas's OpenAI-compatible endpoint by default.
func NewChatModel(ctx context.Context, config *ChatModelConfig) (*ChatModel, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	nc := *config
	if nc.BaseURL == "" {
		nc.BaseURL = DefaultBaseURL
	}

	inner, err := modelopenai.NewChatModel(ctx, &nc)
	if err != nil {
		return nil, err
	}

	return &ChatModel{inner: inner}, nil
}

// Generate delegates to the underlying OpenAI-compatible model.
func (cm *ChatModel) Generate(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	return cm.inner.Generate(ctx, in, opts...)
}

// Stream delegates to the underlying OpenAI-compatible streaming API.
func (cm *ChatModel) Stream(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return cm.inner.Stream(ctx, in, opts...)
}

// WithTools binds tools and returns another Atlas Cloud chat model.
func (cm *ChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	toolModel, err := cm.inner.WithTools(tools)
	if err != nil {
		return nil, err
	}

	inner, ok := toolModel.(*modelopenai.ChatModel)
	if !ok {
		return nil, fmt.Errorf("unexpected tool model type %T", toolModel)
	}

	return &ChatModel{inner: inner}, nil
}

// BindTools binds tools on the current model instance.
func (cm *ChatModel) BindTools(tools []*schema.ToolInfo) error {
	return cm.inner.BindTools(tools)
}

// BindForcedTools binds tools and forces the model to call one of them.
func (cm *ChatModel) BindForcedTools(tools []*schema.ToolInfo) error {
	return cm.inner.BindForcedTools(tools)
}

func (cm *ChatModel) GetType() string {
	return "AtlasCloud"
}

func (cm *ChatModel) IsCallbacksEnabled() bool {
	return cm.inner.IsCallbacksEnabled()
}

// WithExtraFields sets additional top-level request fields.
func WithExtraFields(extraFields map[string]any) model.Option {
	return modelopenai.WithExtraFields(extraFields)
}

// WithExtraHeader sets additional request headers.
func WithExtraHeader(header map[string]string) model.Option {
	return modelopenai.WithExtraHeader(header)
}

// WithReasoningEffort overrides the request-level reasoning effort.
func WithReasoningEffort(effort ReasoningEffortLevel) model.Option {
	return modelopenai.WithReasoningEffort(effort)
}

// WithMaxCompletionTokens overrides the request-level max completion tokens.
func WithMaxCompletionTokens(maxCompletionTokens int) model.Option {
	return modelopenai.WithMaxCompletionTokens(maxCompletionTokens)
}

// WithRequestPayloadModifier customizes the serialized request body before sending.
func WithRequestPayloadModifier(modifier RequestPayloadModifier) model.Option {
	return modelopenai.WithRequestPayloadModifier(modifier)
}

// WithResponseMessageModifier customizes non-streaming responses using the raw body.
func WithResponseMessageModifier(m ResponseMessageModifier) model.Option {
	return modelopenai.WithResponseMessageModifier(m)
}

// WithResponseChunkMessageModifier customizes streaming responses using each raw chunk body.
func WithResponseChunkMessageModifier(m ResponseChunkMessageModifier) model.Option {
	return modelopenai.WithResponseChunkMessageModifier(m)
}

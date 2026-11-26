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

package litellm

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/libs/acl/openai"
)

var _ model.ToolCallingChatModel = (*ChatModel)(nil)

// Config holds the configuration for a LiteLLM proxy-backed ChatModel.
//
// LiteLLM (https://github.com/BerriAI/litellm) is an AI gateway that provides
// a unified OpenAI-compatible API to 100+ LLM providers including OpenAI, Anthropic,
// Google Gemini, AWS Bedrock, Azure OpenAI, Mistral, Cohere, and more.
//
// The Model field uses LiteLLM's provider/model naming convention,
// e.g. "openai/gpt-4o", "anthropic/claude-sonnet-4-20250514", "bedrock/anthropic.claude-3-haiku-20240307-v1:0".
type Config struct {
	// APIKey is the LiteLLM proxy master key or virtual key for authentication.
	// Required.
	APIKey string `json:"api_key"`

	// BaseURL is the LiteLLM proxy endpoint URL.
	// Required. Example: "http://localhost:4000"
	BaseURL string `json:"base_url"`

	// Timeout specifies the maximum duration to wait for API responses.
	// If HTTPClient is set, Timeout will not be used.
	// Optional. Default: no timeout
	Timeout time.Duration `json:"timeout"`

	// HTTPClient specifies the client to send HTTP requests.
	// If HTTPClient is set, Timeout will not be used.
	// Optional. Default: &http.Client{Timeout: Timeout}
	HTTPClient *http.Client `json:"http_client"`

	// Model specifies the LiteLLM model identifier.
	// Uses provider/model format, e.g. "openai/gpt-4o", "anthropic/claude-sonnet-4-20250514".
	// Required.
	Model string `json:"model"`

	// MaxTokens specifies the maximum number of tokens to generate.
	// Optional.
	MaxTokens *int `json:"max_tokens,omitempty"`

	// MaxCompletionTokens is the total number of tokens in the model's output,
	// including both the final output and any tokens generated during the thinking process.
	// Optional.
	MaxCompletionTokens *int `json:"max_completion_tokens,omitempty"`

	// Temperature specifies what sampling temperature to use.
	// Range: 0.0 to 2.0. Higher values make output more random.
	// Optional.
	Temperature *float32 `json:"temperature,omitempty"`

	// TopP controls diversity via nucleus sampling.
	// Range: 0.0 to 1.0.
	// Optional.
	TopP *float32 `json:"top_p,omitempty"`

	// Stop sequences where the API will stop generating further tokens.
	// Optional.
	Stop []string `json:"stop,omitempty"`

	// Seed enables deterministic sampling for consistent outputs.
	// Optional.
	Seed *int `json:"seed,omitempty"`

	// PresencePenalty prevents repetition by penalizing tokens based on presence.
	// Range: -2.0 to 2.0.
	// Optional.
	PresencePenalty *float32 `json:"presence_penalty,omitempty"`

	// FrequencyPenalty prevents repetition by penalizing tokens based on frequency.
	// Range: -2.0 to 2.0.
	// Optional.
	FrequencyPenalty *float32 `json:"frequency_penalty,omitempty"`

	// User is a unique identifier representing end-user.
	// Optional.
	User *string `json:"user,omitempty"`

	// ExtraFields will be merged into the top-level JSON request body,
	// overriding any existing fields with the same key.
	// Useful for passing LiteLLM-specific parameters like "metadata", "tags", or "drop_params".
	// Optional.
	ExtraFields map[string]any `json:"extra_fields,omitempty"`
}

// ChatModel implements the Eino ChatModel interface backed by a LiteLLM proxy.
type ChatModel struct {
	cli *openai.Client
}

// NewChatModel creates a new ChatModel connected to a LiteLLM proxy.
func NewChatModel(ctx context.Context, config *Config) (*ChatModel, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	if config.BaseURL == "" {
		return nil, fmt.Errorf("base_url is required for LiteLLM proxy")
	}

	var httpClient *http.Client
	if config.HTTPClient != nil {
		httpClient = config.HTTPClient
	} else {
		httpClient = &http.Client{Timeout: config.Timeout}
	}

	nConf := &openai.Config{
		BaseURL:             config.BaseURL,
		APIKey:              config.APIKey,
		HTTPClient:          httpClient,
		Model:               config.Model,
		MaxTokens:           config.MaxTokens,
		MaxCompletionTokens: config.MaxCompletionTokens,
		Temperature:         config.Temperature,
		TopP:                config.TopP,
		Stop:                config.Stop,
		Seed:                config.Seed,
		PresencePenalty:     config.PresencePenalty,
		FrequencyPenalty:    config.FrequencyPenalty,
		User:                config.User,
		ExtraFields:         config.ExtraFields,
	}

	cli, err := openai.NewClient(ctx, nConf)
	if err != nil {
		return nil, err
	}

	return &ChatModel{cli: cli}, nil
}

// Generate sends a chat completion request and returns the full response.
func (cm *ChatModel) Generate(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)
	return cm.cli.Generate(ctx, in, opts...)
}

// Stream sends a chat completion request and returns a streaming response.
func (cm *ChatModel) Stream(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)
	return cm.cli.Stream(ctx, in, opts...)
}

const typ = "LiteLLM"

// GetType returns the provider type name.
func (cm *ChatModel) GetType() string {
	return typ
}

// IsCallbacksEnabled returns whether callbacks are enabled.
func (cm *ChatModel) IsCallbacksEnabled() bool {
	return cm.cli.IsCallbacksEnabled()
}

// BindTools binds tool definitions to this ChatModel instance.
func (cm *ChatModel) BindTools(tools []*schema.ToolInfo) error {
	return cm.cli.BindTools(tools)
}

// WithTools returns a new ChatModel instance with the specified tools bound.
func (cm *ChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	cli, err := cm.cli.WithToolsForClient(tools)
	if err != nil {
		return nil, err
	}
	return &ChatModel{cli: cli}, nil
}

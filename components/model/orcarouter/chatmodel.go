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

package orcarouter

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

const (
	defaultBaseURL = "https://api.orcarouter.ai/v1"
)

// Config is the configuration for the OrcaRouter ChatModel.
type Config struct {
	// APIKey is your OrcaRouter API key.
	// Required.
	APIKey string

	// Timeout specifies the maximum duration to wait for API responses.
	// If HTTPClient is set, Timeout will not be used.
	// Optional. Default: no timeout
	Timeout time.Duration `json:"timeout"`

	// HTTPClient specifies the client to send HTTP requests.
	// If HTTPClient is set, Timeout will not be used.
	// Optional. Default &http.Client{Timeout: Timeout}
	HTTPClient *http.Client `json:"http_client"`

	// BaseURL specifies the OrcaRouter endpoint URL.
	// Optional. Default: https://api.orcarouter.ai/v1
	BaseURL string `json:"base_url"`

	// Model specifies the ID of the model to use.
	// OrcaRouter serves namespaced model ids (e.g. anthropic/claude-sonnet-5,
	// anthropic/claude-haiku-4.5) over one OpenAI-compatible endpoint.
	// Optional.
	Model string `json:"model,omitempty"`

	// MaxTokens represents the maximum number of tokens that can be generated in the chat completion.
	// Optional. Default: model's maximum
	MaxTokens *int `json:"max_tokens,omitempty"`

	// MaxCompletionTokens represents the total number of tokens in the model's output,
	// including both the final output and any tokens generated during the thinking process.
	MaxCompletionTokens *int `json:"max_completion_tokens,omitempty"`

	// Temperature specifies what sampling temperature to use.
	// Generally recommend altering this or TopP but not both.
	// Range: 0.0 to 2.0. Higher values make output more random.
	// Optional. Default: 1.0
	Temperature *float32 `json:"temperature,omitempty"`

	// TopP controls diversity via nucleus sampling.
	// Generally recommend altering this or Temperature but not both.
	// Range: 0.0 to 1.0. Lower values make output more focused.
	// Optional. Default: 1.0
	TopP *float32 `json:"top_p,omitempty"`

	// Stop sequences where the API will stop generating further tokens.
	// Optional. Example: []string{"\n", "User:"}
	Stop []string `json:"stop,omitempty"`

	// PresencePenalty prevents repetition by penalizing tokens based on presence.
	// Range: -2.0 to 2.0. Positive values increase likelihood of new topics.
	// Optional. Default: 0
	PresencePenalty *float32 `json:"presence_penalty,omitempty"`

	// FrequencyPenalty prevents repetition by penalizing tokens based on frequency.
	// Range: -2.0 to 2.0. Positive values decrease likelihood of repetition.
	// Optional. Default: 0
	FrequencyPenalty *float32 `json:"frequency_penalty,omitempty"`

	// Seed enables deterministic sampling for consistent outputs.
	// Optional. Set for reproducible results.
	Seed *int `json:"seed,omitempty"`

	// User unique identifier representing end-user.
	// Optional.
	User *string `json:"user,omitempty"`

	// LogitBias modifies the likelihood of specified tokens appearing in the completion.
	// Optional. Map token IDs to bias values from -100 to 100.
	LogitBias map[string]int `json:"logit_bias,omitempty"`

	// LogProbs specifies whether to return log probabilities of the output tokens.
	LogProbs bool `json:"log_probs"`

	// TopLogProbs is an integer between 0 and 20 specifying the number of most likely tokens
	// to return at each token position, each with an associated log probability.
	// logprobs must be set to true if this parameter is used.
	TopLogProbs int `json:"top_logprobs"`

	// ResponseFormat specifies the format of the model's response.
	// Optional. Use for structured outputs.
	ResponseFormat *openai.ChatCompletionResponseFormat `json:"response_format,omitempty"`

	// ExtraFields will override any existing fields with the same key.
	// Optional. Useful for experimental features not yet officially supported.
	ExtraFields map[string]any `json:"extra_fields,omitempty"`
}

// ChatModel is an OrcaRouter implementation of model.ChatModel.
type ChatModel struct {
	cli *openai.Client
}

var (
	_ model.ChatModel            = (*ChatModel)(nil)
	_ model.ToolCallingChatModel = (*ChatModel)(nil)
)

// NewChatModel creates a new OrcaRouter ChatModel.
func NewChatModel(ctx context.Context, config *Config) (*ChatModel, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	var httpClient *http.Client
	if config.HTTPClient != nil {
		httpClient = config.HTTPClient
	} else {
		httpClient = &http.Client{Timeout: config.Timeout}
	}

	if config.BaseURL == "" {
		config.BaseURL = defaultBaseURL
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
		PresencePenalty:     config.PresencePenalty,
		Seed:                config.Seed,
		FrequencyPenalty:    config.FrequencyPenalty,
		LogitBias:           config.LogitBias,
		LogProbs:            config.LogProbs,
		TopLogProbs:         config.TopLogProbs,
		User:                config.User,
		ResponseFormat:      config.ResponseFormat,
		ExtraFields:         config.ExtraFields,
	}

	cli, err := openai.NewClient(ctx, nConf)
	if err != nil {
		return nil, err
	}

	return &ChatModel{cli: cli}, nil
}

// Generate generates a single chat completion.
func (cm *ChatModel) Generate(ctx context.Context, in []*schema.Message, opts ...model.Option) (outMsg *schema.Message, err error) {
	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)
	return cm.cli.Generate(ctx, in, opts...)
}

// Stream streams chat completion responses.
func (cm *ChatModel) Stream(ctx context.Context, in []*schema.Message, opts ...model.Option) (outStream *schema.StreamReader[*schema.Message], err error) {
	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)
	return cm.cli.Stream(ctx, in, opts...)
}

// WithTools returns a new ChatModel with the given tools bound.
func (cm *ChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	cli, err := cm.cli.WithToolsForClient(tools)
	if err != nil {
		return nil, err
	}
	return &ChatModel{cli: cli}, nil
}

// BindTools binds the given tools to this ChatModel.
func (cm *ChatModel) BindTools(tools []*schema.ToolInfo) error {
	return cm.cli.BindTools(tools)
}

// BindForcedTools binds the given tools to this ChatModel and forces the model to call one of them.
func (cm *ChatModel) BindForcedTools(tools []*schema.ToolInfo) error {
	return cm.cli.BindForcedTools(tools)
}

const typ = "OrcaRouter"

// GetType returns the type of this ChatModel.
func (cm *ChatModel) GetType() string {
	return typ
}

// IsCallbacksEnabled returns whether callbacks are enabled.
func (cm *ChatModel) IsCallbacksEnabled() bool {
	return cm.cli.IsCallbacksEnabled()
}

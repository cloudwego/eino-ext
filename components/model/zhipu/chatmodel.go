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

package zhipu

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/cloudwego/eino-ext/libs/acl/openai"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

var _ model.ToolCallingChatModel = (*ChatModel)(nil)

type Modality = openai.Modality

// ThinkingType defines the thinking mode type
type ThinkingType string

const (
	// ThinkingEnabled enables thinking mode for complex reasoning
	ThinkingEnabled ThinkingType = "enabled"
	// ThinkingDisabled disables thinking mode (default)
	ThinkingDisabled ThinkingType = "disabled"
)

// Thinking specifies the thinking mode settings
type Thinking struct {
	// Type specifies whether to enable thinking mode.
	// Use ThinkingEnabled for complex reasoning problems, ThinkingDisabled otherwise.
	Type ThinkingType `json:"type"`
}

// ChatModelConfig parameters detail see:
// https://docs.bigmodel.cn/cn/guide/develop/openai/introduction
type ChatModelConfig struct {

	// APIKey is your authentication key from 智谱AI
	// Required
	APIKey string `json:"api_key"`

	// Timeout specifies the maximum duration to wait for API responses
	// If HTTPClient is set, Timeout will not be used.
	// Optional. Default: no timeout
	Timeout time.Duration `json:"timeout"`

	// HTTPClient specifies the client to send HTTP requests.
	// If HTTPClient is set, Timeout will not be used.
	// Optional. Default &http.Client{Timeout: Timeout}
	HTTPClient *http.Client `json:"http_client"`

	// BaseURL specifies the Zhipu AI endpoint URL
	// Optional. Default: https://open.bigmodel.cn/api/paas/v4/
	BaseURL string `json:"base_url"`

	// The following fields correspond to OpenAI's chat completion API parameters
	// Ref: https://platform.openai.com/docs/api-reference/chat/create

	// Model specifies the ID of the model to use
	// Required. Examples: "glm-4.7", "glm-4.6v", "glm-4-flash"
	Model string `json:"model"`

	// MaxTokens limits the maximum number of tokens that can be generated in the chat completion
	// Optional. Default: model's maximum
	MaxTokens *int `json:"max_tokens,omitempty"`

	// Temperature specifies what sampling temperature to use
	// Generally recommend altering this or TopP but not both.
	// Range: 0.0 to 1.0. Higher values make output more random
	// Note: temperature=0 (do_sample=False) is not supported in OpenAI compatible mode
	// Optional. Default: 0.6
	Temperature *float32 `json:"temperature,omitempty"`

	// TopP controls diversity via nucleus sampling
	// Generally recommend altering this or Temperature but not both.
	// Range: 0.0 to 1.0. Lower values make output more focused
	// Optional. Default: 0.95
	TopP *float32 `json:"top_p,omitempty"`

	// Stop sequences where the API will stop generating further tokens
	// Optional. Example: []string{"\n", "User:"}
	Stop []string `json:"stop,omitempty"`

	// PresencePenalty prevents repetition by penalizing tokens based on presence
	// Range: -2.0 to 2.0. Positive values increase likelihood of new topics
	// Optional. Default: 0
	PresencePenalty *float32 `json:"presence_penalty,omitempty"`

	// ResponseFormat specifies the format of the model's response
	// Optional. Use for structured outputs
	ResponseFormat *openai.ChatCompletionResponseFormat `json:"response_format,omitempty"`

	// Seed enables deterministic sampling for consistent outputs
	// Optional. Set for reproducible results
	Seed *int `json:"seed,omitempty"`

	// FrequencyPenalty prevents repetition by penalizing tokens based on frequency
	// Range: -2.0 to 2.0. Positive values decrease likelihood of repetition
	// Optional. Default: 0
	FrequencyPenalty *float32 `json:"frequency_penalty,omitempty"`

	// LogitBias modifies likelihood of specific tokens appearing in completion
	// Optional. Map token IDs to bias values from -100 to 100
	LogitBias map[string]int `json:"logit_bias,omitempty"`

	// User unique identifier representing end-user
	// Optional. Helps monitor and detect abuse
	User *string `json:"user,omitempty"`

	// Thinking enables thinking mode for complex reasoning problems
	// https://docs.bigmodel.cn/cn/guide/develop/openai/introduction
	// Optional. Default: nil (disabled)
	Thinking *Thinking `json:"thinking,omitempty"`
}

type ChatModel struct {
	cli *openai.Client

	extraOptions *options
}

func NewChatModel(ctx context.Context, config *ChatModelConfig) (*ChatModel, error) {
	if config == nil {
		return nil, fmt.Errorf("[NewChatModel] config not provided")
	}

	var httpClient *http.Client

	if config.HTTPClient != nil {
		httpClient = config.HTTPClient
	} else {
		httpClient = &http.Client{Timeout: config.Timeout}
	}

	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://open.bigmodel.cn/api/paas/v4/"
	}

	nConfig := &openai.Config{
		BaseURL:          baseURL,
		APIKey:           config.APIKey,
		HTTPClient:       httpClient,
		Model:            config.Model,
		MaxTokens:        config.MaxTokens,
		Temperature:      config.Temperature,
		TopP:             config.TopP,
		Stop:             config.Stop,
		PresencePenalty:  config.PresencePenalty,
		ResponseFormat:   config.ResponseFormat,
		Seed:             config.Seed,
		FrequencyPenalty: config.FrequencyPenalty,
		LogitBias:        config.LogitBias,
		User:             config.User,
	}

	cli, err := openai.NewClient(ctx, nConfig)

	if err != nil {
		return nil, err
	}

	return &ChatModel{
		cli: cli,

		extraOptions: &options{
			Thinking: config.Thinking,
		},
	}, nil
}

func validateToolOptions(opts ...model.Option) error {
	modelOptions := model.GetCommonOptions(&model.Options{}, opts...)
	if modelOptions.ToolChoice != nil {
		if *modelOptions.ToolChoice == schema.ToolChoiceAllowed && len(modelOptions.AllowedToolNames) > 0 {
			return fmt.Errorf("tool_choice 'allowed' is not supported when allowed tool names are present")
		}
		if *modelOptions.ToolChoice == schema.ToolChoiceForced && len(modelOptions.AllowedToolNames) > 1 {
			return fmt.Errorf("only one allowed tool name can be configured for tool_choice 'forced'")
		}
	}
	return nil
}

func (cm *ChatModel) Generate(ctx context.Context, in []*schema.Message, opts ...model.Option) (
	outMsg *schema.Message, err error) {
	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)
	opts = cm.parseCustomOptions(opts...)
	if err = validateToolOptions(opts...); err != nil {
		return nil, err
	}

	return cm.cli.Generate(ctx, in, opts...)
}

func (cm *ChatModel) Stream(ctx context.Context, in []*schema.Message, opts ...model.Option) (outStream *schema.StreamReader[*schema.Message], err error) {
	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)
	opts = cm.parseCustomOptions(opts...)
	if err = validateToolOptions(opts...); err != nil {
		return nil, err
	}

	return cm.cli.Stream(ctx, in, opts...)
}

func (cm *ChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	cli, err := cm.cli.WithToolsForClient(tools)
	if err != nil {
		return nil, err
	}
	return &ChatModel{cli: cli, extraOptions: cm.extraOptions}, nil
}

func (cm *ChatModel) BindTools(tools []*schema.ToolInfo) error {
	return cm.cli.BindTools(tools)
}

func (cm *ChatModel) BindForcedTools(tools []*schema.ToolInfo) error {
	return cm.cli.BindForcedTools(tools)
}

func (cm *ChatModel) parseCustomOptions(opts ...model.Option) []model.Option {
	zhipuOpts := model.GetImplSpecificOptions(&options{
		Thinking: cm.extraOptions.Thinking,
	}, opts...)

	// Using extra_body to pass the custom options to the underlying client
	extraFields := make(map[string]any)
	if zhipuOpts.Thinking != nil {
		extraFields["thinking"] = map[string]string{
			"type": string(zhipuOpts.Thinking.Type),
		}
	}
	if len(extraFields) > 0 {
		opts = append(opts, openai.WithExtraFields(extraFields))
	}
	return opts
}

const typ = "Zhipu"

func (cm *ChatModel) GetType() string {
	return typ
}

func (cm *ChatModel) IsCallbacksEnabled() bool {
	return cm.cli.IsCallbacksEnabled()
}

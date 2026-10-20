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

package moonshot

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

// defaultBaseURL is the Moonshot (Kimi) OpenAI-compatible endpoint.
// See https://platform.moonshot.cn/docs/api/chat
const defaultBaseURL = "https://api.moonshot.cn/v1"

// ChatModelConfig parameters detail see:
// https://platform.moonshot.cn/docs/api/chat
type ChatModelConfig struct {
	// APIKey is your Moonshot authentication key.
	// Required.
	APIKey string `json:"api_key"`

	// Timeout specifies the maximum duration to wait for API responses.
	// If HTTPClient is set, Timeout will not be used.
	// Optional. Default: no timeout
	Timeout time.Duration `json:"timeout"`

	// HTTPClient specifies the client to send HTTP requests.
	// If HTTPClient is set, Timeout will not be used.
	// Optional. Default &http.Client{Timeout: Timeout}
	HTTPClient *http.Client `json:"http_client"`

	// BaseURL specifies the Moonshot endpoint URL.
	// Optional. Default: https://api.moonshot.cn/v1
	BaseURL string `json:"base_url"`

	// The following fields correspond to OpenAI's chat completion API parameters.
	// Ref: https://platform.openai.com/docs/api-reference/chat/create

	// Model specifies the ID of the model to use, e.g. "kimi-k2-0905-preview",
	// "moonshot-v1-8k", "moonshot-v1-32k", "moonshot-v1-128k".
	// Required.
	Model string `json:"model"`

	// MaxTokens limits the maximum number of tokens that can be generated.
	// Optional. Default: model's maximum
	MaxTokens *int `json:"max_tokens,omitempty"`

	// Temperature specifies what sampling temperature to use.
	// Range: 0.0 to 1.0 for Moonshot. Higher values make output more random.
	// Optional.
	Temperature *float32 `json:"temperature,omitempty"`

	// TopP controls diversity via nucleus sampling.
	// Range: 0.0 to 1.0. Lower values make output more focused.
	// Optional.
	TopP *float32 `json:"top_p,omitempty"`

	// Stop sequences where the API will stop generating further tokens.
	// Optional.
	Stop []string `json:"stop,omitempty"`

	// PresencePenalty prevents repetition by penalizing tokens based on presence.
	// Range: -2.0 to 2.0.
	// Optional.
	PresencePenalty *float32 `json:"presence_penalty,omitempty"`

	// ResponseFormat specifies the format of the model's response.
	// Optional. Use for structured outputs (e.g. JSON mode).
	ResponseFormat *openai.ChatCompletionResponseFormat `json:"response_format,omitempty"`

	// FrequencyPenalty prevents repetition by penalizing tokens based on frequency.
	// Range: -2.0 to 2.0.
	// Optional.
	FrequencyPenalty *float32 `json:"frequency_penalty,omitempty"`

	// User unique identifier representing the end-user.
	// Optional.
	User *string `json:"user,omitempty"`
}

type ChatModel struct {
	cli *openai.Client
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
		baseURL = defaultBaseURL
	}

	cli, err := openai.NewClient(ctx, &openai.Config{
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
		FrequencyPenalty: config.FrequencyPenalty,
		User:             config.User,
	})
	if err != nil {
		return nil, err
	}

	return &ChatModel{cli: cli}, nil
}

func (cm *ChatModel) Generate(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)
	return cm.cli.Generate(ctx, in, opts...)
}

func (cm *ChatModel) Stream(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)
	return cm.cli.Stream(ctx, in, opts...)
}

func (cm *ChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	cli, err := cm.cli.WithToolsForClient(tools)
	if err != nil {
		return nil, err
	}
	return &ChatModel{cli: cli}, nil
}

func (cm *ChatModel) BindTools(tools []*schema.ToolInfo) error {
	return cm.cli.BindTools(tools)
}

func (cm *ChatModel) BindForcedTools(tools []*schema.ToolInfo) error {
	return cm.cli.BindForcedTools(tools)
}

const typ = "Moonshot"

func (cm *ChatModel) GetType() string {
	return typ
}

func (cm *ChatModel) IsCallbacksEnabled() bool {
	return cm.cli.IsCallbacksEnabled()
}

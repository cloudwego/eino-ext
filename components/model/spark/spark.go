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

package spark

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

const (
	// defaultBaseURL is the OpenAI-compatible HTTP endpoint of iFLYTEK Spark.
	// This is the HTTP endpoint that authenticates with a single Bearer
	// "API Password"; it is NOT the legacy WebSocket endpoint
	// (spark-api.xf-yun.com), which uses HMAC request signing.
	defaultBaseURL = "https://spark-api-open.xf-yun.com/v1"

	// defaultModel is the current flagship Spark model.
	defaultModel = "4.0Ultra"

	typ = "Spark"
)

// ChatModelConfig contains the configuration for the iFLYTEK Spark chat model.
//
// The Spark HTTP service is OpenAI-compatible, so most fields mirror OpenAI's
// chat completion parameters.
// Endpoint / model reference:
// https://www.xfyun.cn/doc/spark/HTTP%E8%B0%83%E7%94%A8%E6%96%87%E6%A1%A3.html
type ChatModelConfig struct {
	// APIKey is the Spark HTTP service "API Password" — the single Bearer
	// credential shown as APIPassword in the iFLYTEK console. It is NOT the
	// legacy APPID / APIKey / APISecret triple used by the WebSocket API.
	// Required.
	APIKey string `json:"api_key"`

	// Timeout specifies the maximum duration to wait for API responses.
	// If HTTPClient is set, Timeout will not be used.
	// Optional. Default: no timeout.
	Timeout time.Duration `json:"timeout"`

	// HTTPClient specifies the client to send HTTP requests.
	// If HTTPClient is set, Timeout will not be used.
	// Optional. Default: &http.Client{Timeout: Timeout}.
	HTTPClient *http.Client `json:"http_client"`

	// BaseURL specifies the Spark OpenAI-compatible endpoint URL.
	// Optional. Default: https://spark-api-open.xf-yun.com/v1
	BaseURL string `json:"base_url"`

	// Model specifies the ID of the model to use.
	// Optional. Default: 4.0Ultra.
	// Valid HTTP models: 4.0Ultra, generalv3.5, max-32k, generalv3, pro-128k, lite.
	Model string `json:"model"`

	// The following fields correspond to OpenAI's chat completion API parameters.
	// Ref: https://platform.openai.com/docs/api-reference/chat/create

	// MaxTokens limits the maximum number of tokens that can be generated in the chat completion.
	// Optional. Default: model's maximum.
	MaxTokens *int `json:"max_tokens,omitempty"`

	// Temperature specifies what sampling temperature to use.
	// Generally recommend altering this or TopP but not both.
	// Optional.
	Temperature *float32 `json:"temperature,omitempty"`

	// TopP controls diversity via nucleus sampling.
	// Generally recommend altering this or Temperature but not both.
	// Optional.
	TopP *float32 `json:"top_p,omitempty"`

	// Stop sequences where the API will stop generating further tokens.
	// Optional. Example: []string{"\n", "User:"}
	Stop []string `json:"stop,omitempty"`

	// PresencePenalty prevents repetition by penalizing tokens based on presence.
	// Range: -2.0 to 2.0. Positive values increase likelihood of new topics.
	// Optional. Default: 0.
	PresencePenalty *float32 `json:"presence_penalty,omitempty"`

	// ResponseFormat specifies the format of the model's response.
	// Optional. Use for structured outputs.
	ResponseFormat *openai.ChatCompletionResponseFormat `json:"response_format,omitempty"`

	// Seed enables deterministic sampling for consistent outputs.
	// Optional. Set for reproducible results.
	Seed *int `json:"seed,omitempty"`

	// FrequencyPenalty prevents repetition by penalizing tokens based on frequency.
	// Range: -2.0 to 2.0. Positive values decrease likelihood of repetition.
	// Optional. Default: 0.
	FrequencyPenalty *float32 `json:"frequency_penalty,omitempty"`

	// LogitBias modifies likelihood of specific tokens appearing in completion.
	// Optional. Map token IDs to bias values from -100 to 100.
	LogitBias map[string]int `json:"logit_bias,omitempty"`

	// User is a unique identifier representing the end-user.
	// Optional. Helps the provider monitor and detect abuse.
	User *string `json:"user,omitempty"`
}

// ChatModel is an iFLYTEK Spark chat model implementation that satisfies
// eino's model.ToolCallingChatModel interface. It is a thin wrapper over the
// shared OpenAI-compatible client, since Spark's HTTP API is OpenAI-compatible.
type ChatModel struct {
	cli *openai.Client
}

// NewChatModel creates an iFLYTEK Spark ChatModel.
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

	modelName := config.Model
	if modelName == "" {
		modelName = defaultModel
	}

	cli, err := openai.NewClient(ctx, &openai.Config{
		BaseURL:          baseURL,
		APIKey:           config.APIKey,
		HTTPClient:       httpClient,
		Model:            modelName,
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
	})
	if err != nil {
		return nil, err
	}

	return &ChatModel{cli: cli}, nil
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
	if err = validateToolOptions(opts...); err != nil {
		return nil, err
	}

	return cm.cli.Generate(ctx, in, opts...)
}

func (cm *ChatModel) Stream(ctx context.Context, in []*schema.Message, opts ...model.Option) (
	outStream *schema.StreamReader[*schema.Message], err error) {
	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)
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
	return &ChatModel{cli: cli}, nil
}

func (cm *ChatModel) BindTools(tools []*schema.ToolInfo) error {
	return cm.cli.BindTools(tools)
}

func (cm *ChatModel) BindForcedTools(tools []*schema.ToolInfo) error {
	return cm.cli.BindForcedTools(tools)
}

func (cm *ChatModel) GetType() string {
	return typ
}

func (cm *ChatModel) IsCallbacksEnabled() bool {
	return cm.cli.IsCallbacksEnabled()
}

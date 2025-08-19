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
	"net/http"
	"time"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/libs/acl/openai"
)

var _ model.ToolCallingChatModel = (*ChatModel)(nil)
var _ model.ChatModel = (*ChatModel)(nil)

const (
	ChatCompletionResponseFormatTypeJSONObject = openai.ChatCompletionResponseFormatTypeJSONObject
	ChatCompletionResponseFormatTypeJSONSchema = openai.ChatCompletionResponseFormatTypeJSONSchema
	ChatCompletionResponseFormatTypeText       = openai.ChatCompletionResponseFormatTypeText
)

type ChatCompletionResponseFormat = openai.ChatCompletionResponseFormat
type ChatCompletionResponseFormatJSONSchema = openai.ChatCompletionResponseFormatJSONSchema

type ChatModelConfig struct {
	// APIKey is your authentication key
	// Use OpenAI API key or Azure API key depending on the service
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

	// The following three fields are only required when using Azure OpenAI Service, otherwise they can be ignored.
	// For more details, see: https://learn.microsoft.com/en-us/azure/ai-services/openai/

	// ByAzure indicates whether to use Azure OpenAI Service
	// Required for Azure
	ByAzure bool `json:"by_azure"`

	// AzureModelMapperFunc is used to map the model name to the deployment name for Azure OpenAI Service.
	// This is useful when the model name is different from the deployment name.
	// Optional for Azure, remove [,:] from the model name by default.
	AzureModelMapperFunc func(model string) string

	// BaseURL is the Azure OpenAI endpoint URL
	// Format: https://{YOUR_RESOURCE_NAME}.openai.azure.com. YOUR_RESOURCE_NAME is the name of your resource that you have created on Azure.
	// Required for Azure
	BaseURL string `json:"base_url"`

	// APIVersion specifies the Azure OpenAI API version
	// Required for Azure
	APIVersion string `json:"api_version"`

	// The following fields correspond to OpenAI's chat completion API parameters
	// Ref: https://platform.openai.com/docs/api-reference/chat/create

	// Model specifies the ID of the model to use
	// Required
	Model string `json:"model"`

	// MaxTokens limits the maximum number of tokens that can be generated in the chat completion
	// Optional. Default: model's maximum
	MaxTokens *int `json:"max_tokens,omitempty"`

	// Temperature specifies what sampling temperature to use
	// Generally recommend altering this or TopP but not both.
	// Range: 0.0 to 2.0. Higher values make output more random
	// Optional. Default: 1.0
	Temperature *float32 `json:"temperature,omitempty"`

	// TopP controls diversity via nucleus sampling
	// Generally recommend altering this or Temperature but not both.
	// Range: 0.0 to 1.0. Lower values make output more focused
	// Optional. Default: 1.0
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
	ResponseFormat *ChatCompletionResponseFormat `json:"response_format,omitempty"`

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
	// Optional. Helps OpenAI monitor and detect abuse
	User *string `json:"user,omitempty"`

	// ExtraFields will override any existing fields with the same key.
	// Optional. Useful for experimental features not yet officially supported.
	ExtraFields map[string]any `json:"extra_fields,omitempty"`

	// ReasoningEffort will override the default reasoning level of "medium"
	// Optional. Useful for fine tuning response latency vs. accuracy
	ReasoningEffort ReasoningEffortLevel
}

type ChatModel struct {
	cli *openai.Client
}

func NewChatModel(ctx context.Context, config *ChatModelConfig) (*ChatModel, error) {
	var nConf *openai.Config
	if config != nil {
		var httpClient *http.Client

		if config.HTTPClient != nil {
			httpClient = config.HTTPClient
		} else {
			httpClient = &http.Client{Timeout: config.Timeout}
		}

		nConf = &openai.Config{
			ByAzure:              config.ByAzure,
			BaseURL:              config.BaseURL,
			APIVersion:           config.APIVersion,
			APIKey:               config.APIKey,
			HTTPClient:           httpClient,
			Model:                config.Model,
			MaxTokens:            config.MaxTokens,
			Temperature:          config.Temperature,
			TopP:                 config.TopP,
			Stop:                 config.Stop,
			PresencePenalty:      config.PresencePenalty,
			ResponseFormat:       config.ResponseFormat,
			Seed:                 config.Seed,
			FrequencyPenalty:     config.FrequencyPenalty,
			LogitBias:            config.LogitBias,
			User:                 config.User,
			AzureModelMapperFunc: config.AzureModelMapperFunc,
			ExtraFields:          config.ExtraFields,
			ReasoningEffort:      openai.ReasoningEffortLevel(config.ReasoningEffort),
		}
	}
	cli, err := openai.NewClient(ctx, nConf)
	if err != nil {
		return nil, err
	}

	return &ChatModel{
		cli: cli,
	}, nil
}

func (cm *ChatModel) Generate(ctx context.Context, in []*schema.Message, opts ...model.Option) (
	outMsg *schema.Message, err error) {
	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)
	return cm.cli.Generate(ctx, in, opts...)
}

func (cm *ChatModel) Stream(ctx context.Context, in []*schema.Message, opts ...model.Option) (outStream *schema.StreamReader[*schema.Message], err error) {
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

const typ = "OpenAI"

func (cm *ChatModel) GetType() string {
	return typ
}

func (cm *ChatModel) IsCallbacksEnabled() bool {
	return cm.cli.IsCallbacksEnabled()
}

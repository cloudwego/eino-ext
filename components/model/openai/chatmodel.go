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

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/libs/acl/openai"
)

type ChatModelConfig struct {
	// if you want to use Azure OpenAI Service, set the next three fields. refs: https://learn.microsoft.com/en-us/azure/ai-services/openai/
	// ByAzure set this field to true when using Azure OpenAI Service, otherwise it does not need to be set.
	ByAzure bool `json:"by_azure"`
	// BaseURL https://{{$YOUR_RESOURCE_NAME}}.openai.azure.com, YOUR_RESOURCE_NAME is the name of your resource that you have created on Azure.
	BaseURL string `json:"base_url"`
	// APIVersion specifies the API version you want to use.
	APIVersion string `json:"api_version"`

	// APIKey is typically OPENAI_API_KEY, but if you have set up Azure, then it is Azure API_KEY.
	APIKey string `json:"api_key"`

	// Timeout specifies the http request timeout.
	Timeout time.Duration `json:"timeout"`

	// The following fields have the same meaning as the fields in the openai chat completion API request. Ref: https://platform.openai.com/docs/api-reference/chat/create
	Model            string                               `json:"model"`
	MaxTokens        *int                                 `json:"max_tokens,omitempty"`
	Temperature      *float32                             `json:"temperature,omitempty"`
	TopP             *float32                             `json:"top_p,omitempty"`
	N                *int                                 `json:"n,omitempty"`
	Stop             []string                             `json:"stop,omitempty"`
	PresencePenalty  *float32                             `json:"presence_penalty,omitempty"`
	ResponseFormat   *openai.ChatCompletionResponseFormat `json:"response_format,omitempty"`
	Seed             *int                                 `json:"seed,omitempty"`
	FrequencyPenalty *float32                             `json:"frequency_penalty,omitempty"`
	LogitBias        map[string]int                       `json:"logit_bias,omitempty"`
	LogProbs         *bool                                `json:"logprobs,omitempty"`
	TopLogProbs      *int                                 `json:"top_logprobs,omitempty"`
	User             *string                              `json:"user,omitempty"`
}

var _ model.ChatModel = (*ChatModel)(nil)

type ChatModel struct {
	cli *openai.Client
}

func NewChatModel(ctx context.Context, config *ChatModelConfig) (*ChatModel, error) {
	var nConf *openai.Config
	if config != nil {
		nConf = &openai.Config{
			ByAzure:          config.ByAzure,
			BaseURL:          config.BaseURL,
			APIVersion:       config.APIVersion,
			APIKey:           config.APIKey,
			HTTPClient:       &http.Client{Timeout: config.Timeout},
			Model:            config.Model,
			MaxTokens:        config.MaxTokens,
			Temperature:      config.Temperature,
			TopP:             config.TopP,
			N:                config.N,
			Stop:             config.Stop,
			PresencePenalty:  config.PresencePenalty,
			ResponseFormat:   config.ResponseFormat,
			Seed:             config.Seed,
			FrequencyPenalty: config.FrequencyPenalty,
			LogitBias:        config.LogitBias,
			LogProbs:         config.LogProbs,
			TopLogProbs:      config.TopLogProbs,
			User:             config.User,
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
	return cm.cli.Generate(ctx, in, opts...)
}

func (cm *ChatModel) Stream(ctx context.Context, in []*schema.Message, opts ...model.Option) (outStream *schema.StreamReader[*schema.Message], err error) {
	return cm.cli.Stream(ctx, in, opts...)
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
	return true
}

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

package qwen

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/cloudwego/eino-ext/libs/acl/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// ChatModelConfig parameters detail see:
// https://help.aliyun.com/zh/model-studio/developer-reference/use-qwen-by-calling-api?spm=a2c4g.11186623.help-menu-2400256.d_3_3_0.c3b24823WzuCqJ&scm=20140722.H_2712576._.OR_help-T_cn-DAS-zh-V_1
// https://help.aliyun.com/zh/model-studio/developer-reference/compatibility-of-openai-with-dashscope?spm=a2c4g.11186623.0.i49
type ChatModelConfig struct {
	BaseURL string `json:"base_url"` // 公有云: https://dashscope.aliyuncs.com/compatible-mode/v1
	APIKey  string `json:"api_key"`
	// Timeout specifies the http request timeout.
	Timeout time.Duration `json:"timeout"`

	// The following fields have the same meaning as the fields in the openai chat completion API request. Ref: https://platform.openai.com/docs/api-reference/chat/create
	// Model list see: https://help.aliyun.com/zh/model-studio/getting-started/models
	Model            string                               `json:"model"`
	MaxTokens        *int                                 `json:"max_tokens,omitempty"`
	Temperature      *float32                             `json:"temperature,omitempty"`
	TopP             *float32                             `json:"top_p,omitempty"`
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

type ChatModel struct {
	cli *openai.Client
}

func NewChatModel(ctx context.Context, config *ChatModelConfig) (*ChatModel, error) {
	if config == nil {
		return nil, fmt.Errorf("[NewChatModel] config not provided")
	}

	cli, err := openai.NewClient(ctx, &openai.Config{
		BaseURL:          config.BaseURL,
		APIKey:           config.APIKey,
		HTTPClient:       &http.Client{Timeout: config.Timeout},
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
		LogProbs:         config.LogProbs,
		TopLogProbs:      config.TopLogProbs,
		User:             config.User,
	})
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

const typ = "Qwen"

func (cm *ChatModel) GetType() string {
	return typ
}

func (cm *ChatModel) IsCallbacksEnabled() bool {
	return true
}

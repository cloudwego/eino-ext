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

package hunyuan

import (
	"context"
	"fmt"
	"github.com/cloudwego/eino-ext/libs/acl/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"net/http"
	"strings"
	"time"
)

const (
	baseURL = "https://api.hunyuan.cloud.tencent.com/v1"
)

// ChatModelConfig parameters detail see:
// https://cloud.tencent.com/document/api/1729/105701
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

	// BaseURL specifies the QWen endpoint URL
	// Required. Example: https://api.hunyuan.cloud.tencent.com/v1
	BaseURL string `json:"base_url"`

	// The following fields correspond to OpenAI's chat completion API parameters
	// Model specifies the ID of the model to use
	// Required. See https://cloud.tencent.com/document/product/1729/104753 for all model types
	// also hunyuan/model.go has the model list you call directly use
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
	// Optional. Helps OpenAI monitor and detect abuse
	User *string `json:"user,omitempty"`
}

// todo: To quickly develop and iterate while adapting to the eino project,
//  we have not yet found an elegant method to provide custom input parameters
//  for the Tencent Huoyuan model with minimal cost while being compatible with the OpenAPI Go SDK.
//  We will consider implementing this by wrapping HTTP requests,
//  as the Huoyuan model's default settings are all set to false,
//  and there is no urgent need for priority support.

//type HunYuanCustomConfig struct {
//	// The following fields correspond to hunyuan's chat completion API parameters
//	// Ref: https://cloud.tencent.com/document/product/1729/111007 for hunyuan
//
//	// Citation Used in conjunction with EnableEnhancement and SearchInfo parameters. When enabled,
//	// search results that match in the response will include a superscript notation at the end of the snippet
//	// corresponding to the links in the search_info list.
//	// Optional. Default: false
//	Citation bool `json:"citation,omitempty"`
//
//	// EnableEnhancement Used to enable features such as search. Note:
//	// 1. hunyuan-lite does not support enhancement features like search
//	// 2. starting from April 20, 2025, 00:00:00, the default state will change to closed.
//	// When the switch is closed, the main model will directly generate the response,
//	// which can reduce response latency (especially the first word delay in streaming output).
//	// However, in some scenarios, the response quality may decrease.
//	// Optional. Default: true **before** April 20, 2025, 00:00:00
//	EnableEnhancement bool `json:"enable_enhancement,omitempty"`
//
//	// EnableMultimedia
//	// 1. This parameter currently only applies to users on the whitelist. If you wish to experience this feature, please contact https://cloud.tencent.com/online-service?from=connect-us.
//	// Read https://cloud.tencent.com/document/product/1729/111178 for more information.
//	// 2. This parameter is only effective when the Enable Enhancement (e.g., Search) Switch is enabled (EnableEnhancement=true) and the Speed Search Switch is disabled (EnableDeepSearch=false).
//	// 3. this parameter does not apply to the hunyuan-lite version.
//	// Optional. Default: false
//	EnableMultimedia bool `json:"enable_multimedia,omitempty"`
//
//	// EnableRecommendedQuestions When enabled, the response will include a recommended_questions field in the last package, providing up to three recommended questions.
//	// Optional. Default: false
//	EnableRecommendedQuestions bool `json:"enable_recommended_questions,omitempty"`
//
//	// ForceSearchEnhancement When enabled, it will force the use of AI search. If the AI search results are empty, the large model will provide a fallback response.
//	// Optional. Default: false
//	ForceSearchEnhancement bool `json:"force_search_enhancement,omitempty"`
//
//	// SearchInfo When the value is true and a search match is found, the interface will return search_info.
//	// Optional. Default: false
//	SearchInfo bool `json:"search_info,omitempty"`
//
//	// EnableDeepSearch determines whether in-depth research on the issue is enabled.
//	// If set to true and the issue qualifies for in-depth research,
//	// detailed research information will be returned.
//	// Optional. Default: false
//	EnableDeepSearch bool `json:"enable_deep_search,omitempty"`
//
//	// EnableDeepRead controls whether deep reading is enabled.
//	// When enabled, a specific prompt template must be chosen based on the document type,
//	// such as Core Summary, Paper Evaluation, Main Content, or Key Questions and Answers.
//	// Currently, only single-turn, single-document deep reading is supported.
//	// Optional. Default: false
//	EnableDeepRead bool `json:"enable_deep_read,omitempty"`
//}

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

	if config.BaseURL == "" {
		withBaseUrl(config)
	} else {
		// sdk won't add '/' automatically
		if !strings.HasSuffix(config.BaseURL, "/") {
			config.BaseURL = config.BaseURL + "/"
		}
	}

	cli, err := openai.NewClient(ctx, &openai.Config{
		BaseURL:          config.BaseURL,
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
	})
	if err != nil {
		return nil, err
	}

	return &ChatModel{
		cli: cli,
	}, nil
}

func withBaseUrl(config *ChatModelConfig) {
	config.BaseURL = baseURL
}

func (cm *ChatModel) Generate(ctx context.Context, in []*schema.Message, opts ...model.Option) (
	outMsg *schema.Message, err error) {
	return cm.cli.Generate(ctx, in, opts...)
}

func (cm *ChatModel) Stream(ctx context.Context, in []*schema.Message, opts ...model.Option) (outStream *schema.StreamReader[*schema.Message], err error) {
	outStream, err = cm.cli.Stream(ctx, in, opts...)
	if err != nil {
		return nil, err
	}

	var lastIndex *int

	sr := schema.StreamReaderWithConvert(outStream, func(msg *schema.Message) (*schema.Message, error) {
		// issue: https://github.com/cloudwego/eino-examples/issues/23
		// for some case, the response toolcall index is nil, but content is not empty
		// use the last index as the toolcall index, so the concat can be correct
		// suppose only the first toolcall should be fixed.
		if len(msg.ToolCalls) > 0 {
			firstToolCall := msg.ToolCalls[0]

			if msg.ResponseMeta == nil || len(msg.ResponseMeta.FinishReason) == 0 {
				lastIndex = firstToolCall.Index
				return msg, nil
			}

			if firstToolCall.Index == nil && len(msg.ResponseMeta.FinishReason) != 0 {
				firstToolCall.Index = lastIndex
				msg.ToolCalls[0] = firstToolCall
			}
		}
		return msg, nil
	})
	return sr, nil
}

func (cm *ChatModel) BindTools(tools []*schema.ToolInfo) error {
	return cm.cli.BindTools(tools)
}

func (cm *ChatModel) BindForcedTools(tools []*schema.ToolInfo) error {
	return cm.cli.BindForcedTools(tools)
}

const typ = "hunyuan"

func (cm *ChatModel) GetType() string {
	return typ
}

func (cm *ChatModel) IsCallbacksEnabled() bool {
	return true
}

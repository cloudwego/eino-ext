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

package agenticdeepseek

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/cloudwego/eino-ext/libs/acl/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

var _ model.AgenticModel = (*Model)(nil)

type ResponseFormatType string

const (
	ResponseFormatTypeText       = "text"
	ResponseFormatTypeJSONObject = "json_object"
)

// Config parameters detail see:
// https://api-docs.deepseek.com/zh-cn/api/create-chat-completion
type Config struct {
	// APIKey is your authentication key.
	// Required.
	APIKey string `json:"api_key"`

	// Timeout specifies the maximum duration to wait for API responses.
	// If HTTPClient is set, Timeout will not be used.
	// Optional.
	Timeout time.Duration `json:"timeout"`

	// HTTPClient specifies the client to send HTTP requests.
	// If HTTPClient is set, Timeout will not be used.
	// Optional. Default: &http.Client{Timeout: Timeout}
	HTTPClient *http.Client `json:"http_client"`

	// BaseURL is your custom deepseek endpoint url.
	// Optional. Default: https://api.deepseek.com
	BaseURL string `json:"base_url"`

	// Model specifies the ID of the model to use.
	// Required.
	Model string `json:"model"`

	// MaxTokens limits the maximum number of tokens that can be generated in the chat completion.
	// Optional. Default see: https://api-docs.deepseek.com/zh-cn/api/create-chat-completion
	MaxTokens *int `json:"max_tokens,omitempty"`

	// Temperature specifies what sampling temperature to use.
	// Optional. Default see: https://api-docs.deepseek.com/zh-cn/api/create-chat-completion
	Temperature *float32 `json:"temperature,omitempty"`

	// TopP controls diversity via nucleus sampling.
	// Optional. Default see: https://api-docs.deepseek.com/zh-cn/api/create-chat-completion
	TopP *float32 `json:"top_p,omitempty"`

	// Stop sequences where the API will stop generating further tokens.
	// Optional. Default see: https://api-docs.deepseek.com/zh-cn/api/create-chat-completion
	Stop []string `json:"stop,omitempty"`

	// PresencePenalty prevents repetition by penalizing tokens based on presence.
	// Optional. Default see: https://api-docs.deepseek.com/zh-cn/api/create-chat-completion
	PresencePenalty *float32 `json:"presence_penalty,omitempty"`

	// ResponseFormatType specifies the format of the model's response.
	// Optional. Default see: https://api-docs.deepseek.com/zh-cn/api/create-chat-completion
	ResponseFormatType ResponseFormatType `json:"response_format_type,omitempty"`

	// FrequencyPenalty prevents repetition by penalizing tokens based on frequency.
	// Optional. Default see: https://api-docs.deepseek.com/zh-cn/api/create-chat-completion
	FrequencyPenalty *float32 `json:"frequency_penalty,omitempty"`

	// LogProbs specifies whether to return log probabilities of the output tokens.
	// Optional. Default see: https://api-docs.deepseek.com/zh-cn/api/create-chat-completion
	LogProbs *bool `json:"log_probs,omitempty"`

	// TopLogProbs specifies the number of most likely tokens to return at each token position.
	// Optional. Default see: https://api-docs.deepseek.com/zh-cn/api/create-chat-completion
	TopLogProbs *int `json:"top_log_probs,omitempty"`
}

type Model struct {
	cli *openai.AgenticClient
}

func New(ctx context.Context, config *Config) (*Model, error) {
	if config == nil {
		return nil, fmt.Errorf("[New] config not provided")
	}

	var httpClient *http.Client

	if config.HTTPClient != nil {
		httpClient = config.HTTPClient
	} else {
		httpClient = &http.Client{Timeout: config.Timeout}
	}

	baseURL := config.BaseURL
	if len(baseURL) == 0 {
		baseURL = "https://api.deepseek.com"
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
		FrequencyPenalty: config.FrequencyPenalty,
	}
	if config.LogProbs != nil && *config.LogProbs {
		nConfig.LogProbs = true
	}
	if config.TopLogProbs != nil {
		nConfig.TopLogProbs = *config.TopLogProbs
	}
	if len(config.ResponseFormatType) > 0 {
		nConfig.ResponseFormat = &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatType(config.ResponseFormatType),
		}
	}
	cli, err := openai.NewAgenticClient(ctx, nConfig)
	if err != nil {
		return nil, err
	}

	return &Model{
		cli: cli,
	}, nil
}

func (m *Model) Generate(ctx context.Context, in []*schema.AgenticMessage, opts ...model.Option) (
	*schema.AgenticMessage, error) {

	opts = append(opts, responseMetaModifier())

	out, err := m.cli.Generate(ctx, in, opts...)
	if err != nil {
		return nil, err
	}

	extractResponseMetaExtension(out)

	return out, nil
}

func (m *Model) Stream(ctx context.Context, in []*schema.AgenticMessage, opts ...model.Option) (
	*schema.StreamReader[*schema.AgenticMessage], error) {

	opts = append(opts, responseMetaChunkModifier())

	sr, err := m.cli.Stream(ctx, in, opts...)
	if err != nil {
		return nil, err
	}

	return schema.StreamReaderWithConvert(sr, func(msg *schema.AgenticMessage) (*schema.AgenticMessage, error) {
		extractResponseMetaExtension(msg)
		return msg, nil
	}), nil
}

const typ = "AgenticDeepSeek"

func (m *Model) GetType() string {
	return typ
}

func (m *Model) IsCallbacksEnabled() bool {
	return m.cli.IsCallbacksEnabled()
}

const extraKeyResponseMetaExtension = "_deepseek_response_meta_ext"

func responseMetaModifier() model.Option {
	return openai.WithResponseMessageModifier(
		func(ctx context.Context, msg *schema.Message, rawBody []byte) (*schema.Message, error) {
			if msg != nil && msg.ResponseMeta != nil {
				setMsgExtra(msg, extraKeyResponseMetaExtension, &ResponseMetaExtension{
					FinishReason: msg.ResponseMeta.FinishReason,
					LogProbs:     msg.ResponseMeta.LogProbs,
				})
			}
			return msg, nil
		},
	)
}

func responseMetaChunkModifier() model.Option {
	return openai.WithResponseChunkMessageModifier(
		func(ctx context.Context, msg *schema.Message, rawBody []byte, end bool) (*schema.Message, error) {
			if msg != nil && msg.ResponseMeta != nil {
				setMsgExtra(msg, extraKeyResponseMetaExtension, &ResponseMetaExtension{
					FinishReason: msg.ResponseMeta.FinishReason,
					LogProbs:     msg.ResponseMeta.LogProbs,
				})
			}
			return msg, nil
		},
	)
}

func extractResponseMetaExtension(out *schema.AgenticMessage) {
	if out.Extra == nil {
		return
	}
	ext, ok := out.Extra[extraKeyResponseMetaExtension].(*ResponseMetaExtension)
	if !ok {
		return
	}
	if out.ResponseMeta == nil {
		out.ResponseMeta = &schema.AgenticResponseMeta{}
	}
	out.ResponseMeta.Extension = ext
}

func setMsgExtra(msg *schema.Message, key string, value any) {
	if msg.Extra == nil {
		msg.Extra = make(map[string]any)
	}
	msg.Extra[key] = value
}

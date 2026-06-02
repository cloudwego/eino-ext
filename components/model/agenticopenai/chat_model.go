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

package agenticopenai

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/cloudwego/eino-ext/libs/acl/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

var _ model.AgenticModel = (*ChatModel)(nil)

// Config parameters detail see:
// https://platform.openai.com/docs/api-reference/chat/create
type ChatConfig struct {
	// APIKey is your authentication key
	// Required
	APIKey string

	// Timeout specifies the maximum duration to wait for API responses
	// If HTTPClient is set, Timeout will not be used.
	// Optional. Default: no timeout
	Timeout time.Duration

	// HTTPClient specifies the client to send HTTP requests.
	// If HTTPClient is set, Timeout will not be used.
	// Optional. Default &http.Client{Timeout: Timeout}
	HTTPClient *http.Client

	// ByAzure indicates whether to use Azure OpenAI Service
	// Optional. Default: false
	ByAzure bool

	// AzureModelMapperFunc is used to map the model name to the deployment name for Azure OpenAI Service.
	// Optional for Azure.
	AzureModelMapperFunc func(model string) string

	// APIVersion specifies the Azure OpenAI API version
	// Required for Azure
	APIVersion string

	// BaseURL specifies the API endpoint URL
	// Optional. Default: https://api.openai.com/v1
	BaseURL string

	// Model specifies the ID of the model to use
	// Required
	Model string

	// MaxCompletionTokens specifies an upper bound for the number of tokens that can be generated
	// for a completion, including visible output tokens and reasoning tokens.
	// Optional.
	MaxCompletionTokens *int

	// Temperature specifies what sampling temperature to use
	// Range: 0.0 to 2.0. Higher values make output more random
	// Optional. Default: 1.0
	Temperature *float32

	// TopP controls diversity via nucleus sampling
	// Range: 0.0 to 1.0. Lower values make output more focused
	// Optional. Default: 1.0
	TopP *float32

	// Stop sequences where the API will stop generating further tokens
	// Optional. Example: []string{"\n", "User:"}
	Stop []string

	// PresencePenalty prevents repetition by penalizing tokens based on presence
	// Range: -2.0 to 2.0. Positive values increase likelihood of new topics
	// Optional. Default: 0
	PresencePenalty *float32

	// FrequencyPenalty prevents repetition by penalizing tokens based on frequency
	// Range: -2.0 to 2.0. Positive values decrease likelihood of repetition
	// Optional. Default: 0
	FrequencyPenalty *float32

	// LogitBias modifies likelihood of specific tokens appearing in completion
	// Optional. Map token IDs to bias values from -100 to 100
	LogitBias map[string]int

	// LogProbs specifies whether to return log probabilities of the output tokens.
	// Optional. Default: false
	LogProbs bool

	// TopLogProbs specifies the number of most likely tokens to return at each token position.
	// Optional.
	TopLogProbs int

	// CustomHeaders specifies custom HTTP headers to include in the request.
	// Optional.
	CustomHeaders map[string]string

	// ExtraFields specifies extra fields to include in the request body.
	// These fields will be merged into the top-level JSON request body, overriding any existing fields with the same key.
	// Optional.
	//
	// Example:
	//
	//	ExtraFields: map[string]any{
	//	    "reasoning_effort": "high",
	//	    "service_tier": "default",
	//	}
	//
	// The resulting request body will be:
	//
	//	{
	//	    "model": "o1",
	//	    "messages": [...],
	//	    "reasoning_effort": "high",
	//	    "service_tier": "default"
	//	}
	ExtraFields map[string]any
}

type ChatModel struct {
	cli           *openai.AgenticClient
	customHeaders map[string]string
}

func NewChatModel(ctx context.Context, config *ChatConfig) (*ChatModel, error) {
	if config == nil {
		return nil, fmt.Errorf("[New] config not provided")
	}

	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	var httpClient *http.Client
	if config.HTTPClient != nil {
		httpClient = config.HTTPClient
	} else {
		httpClient = &http.Client{Timeout: config.Timeout}
	}

	nConfig := &openai.Config{
		APIKey:               config.APIKey,
		HTTPClient:           httpClient,
		ByAzure:              config.ByAzure,
		AzureModelMapperFunc: config.AzureModelMapperFunc,
		BaseURL:              baseURL,
		APIVersion:           config.APIVersion,
		Model:                config.Model,
		MaxCompletionTokens:  config.MaxCompletionTokens,
		Temperature:          config.Temperature,
		TopP:                 config.TopP,
		Stop:                 config.Stop,
		PresencePenalty:      config.PresencePenalty,
		FrequencyPenalty:     config.FrequencyPenalty,
		LogitBias:            config.LogitBias,
		LogProbs:             config.LogProbs,
		TopLogProbs:          config.TopLogProbs,
		ExtraFields:          config.ExtraFields,
	}

	cli, err := openai.NewAgenticClient(ctx, nConfig)
	if err != nil {
		return nil, err
	}

	return &ChatModel{
		cli:           cli,
		customHeaders: config.CustomHeaders,
	}, nil
}

func (m *ChatModel) Generate(ctx context.Context, in []*schema.AgenticMessage, opts ...model.Option) (
	*schema.AgenticMessage, error) {

	opts = m.parseCustomOptions(opts...)
	opts = append(opts, responseMetaModifier())

	out, err := m.cli.Generate(ctx, in, opts...)
	if err != nil {
		return nil, err
	}

	extractChatResponseMetaExtension(out)

	return out, nil
}

func (m *ChatModel) Stream(ctx context.Context, in []*schema.AgenticMessage, opts ...model.Option) (
	*schema.StreamReader[*schema.AgenticMessage], error) {

	opts = m.parseCustomOptions(opts...)
	opts = append(opts, responseMetaChunkModifier())

	sr, err := m.cli.Stream(ctx, in, opts...)
	if err != nil {
		return nil, err
	}

	return schema.StreamReaderWithConvert(sr, func(msg *schema.AgenticMessage) (*schema.AgenticMessage, error) {
		extractChatResponseMetaExtension(msg)
		return msg, nil
	}), nil
}

func (m *ChatModel) parseCustomOptions(opts ...model.Option) []model.Option {
	customOpts := model.GetImplSpecificOptions(&options{}, opts...)

	headers := m.customHeaders
	if len(customOpts.customHeaders) > 0 {
		headers = customOpts.customHeaders
	}
	if len(headers) > 0 {
		opts = append(opts, openai.WithExtraHeader(headers))
	}
	if len(customOpts.extraFields) > 0 {
		opts = append(opts, openai.WithExtraFields(customOpts.extraFields))
	}

	return opts
}

func (m *ChatModel) GetType() string {
	return chatImplType
}

func (m *ChatModel) IsCallbacksEnabled() bool {
	return m.cli.IsCallbacksEnabled()
}

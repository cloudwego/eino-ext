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

// Package ark implements chat model for ark runtime.
package ark

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/responses"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	fmodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

var _ fmodel.ToolCallingChatModel = (*ChatModel)(nil)

var (
	// all default values are from github.com/volcengine/volcengine-go-sdk/service/arkruntime/config.go
	defaultBaseURL    = "https://ark.cn-beijing.volces.com/api/v3"
	defaultRegion     = "cn-beijing"
	defaultRetryTimes = 2
	defaultTimeout    = 10 * time.Minute
)

var (
	ErrEmptyResponse = errors.New("empty response received from model")
)

type ChatModelConfig struct {
	// Timeout specifies the maximum duration to wait for API responses
	// If HTTPClient is set, Timeout will not be used.
	// Optional. Default: 10 minutes
	Timeout *time.Duration `json:"timeout"`

	// HTTPClient specifies the client to send HTTP requests.
	// If HTTPClient is set, Timeout will not be used.
	// Optional. Default &http.Client{Timeout: Timeout}
	HTTPClient *http.Client `json:"http_client"`

	// RetryTimes specifies the number of retry attempts for failed API calls
	// Optional. Default: 2
	RetryTimes *int `json:"retry_times"`

	// BaseURL specifies the base URL for Ark service
	// Optional. Default: "https://ark.cn-beijing.volces.com/api/v3"
	BaseURL string `json:"base_url"`

	// Region specifies the region where Ark service is located
	// Optional. Default: "cn-beijing"
	Region string `json:"region"`

	// The following three fields are about authentication - either APIKey or AccessKey/SecretKey pair is required
	// For authentication details, see: https://www.volcengine.com/docs/82379/1298459
	// APIKey takes precedence if both are provided
	APIKey string `json:"api_key"`

	AccessKey string `json:"access_key"`

	SecretKey string `json:"secret_key"`

	// The following fields correspond to Ark's chat completion API parameters
	// Ref: https://www.volcengine.com/docs/82379/1298454

	// Model specifies the ID of endpoint on ark platform
	// Required
	Model string `json:"model"`

	// MaxTokens limits the maximum number of tokens that can be generated in the chat completion.
	// Optional. Default: 4096
	MaxTokens *int `json:"max_tokens,omitempty"`

	// Temperature specifies what sampling temperature to use
	// Generally recommend altering this or TopP but not both
	// Range: 0.0 to 1.0. Higher values make output more random
	// Optional. Default: 1.0
	Temperature *float32 `json:"temperature,omitempty"`

	// TopP controls diversity via nucleus sampling
	// Generally recommend altering this or Temperature but not both
	// Range: 0.0 to 1.0. Lower values make output more focused
	// Optional. Default: 0.7
	TopP *float32 `json:"top_p,omitempty"`

	// Stop sequences where the API will stop generating further tokens
	// Optional. Example: []string{"\n", "User:"}
	Stop []string `json:"stop,omitempty"`

	// FrequencyPenalty prevents repetition by penalizing tokens based on frequency
	// Range: -2.0 to 2.0. Positive values decrease likelihood of repetition
	// Optional. Default: 0
	FrequencyPenalty *float32 `json:"frequency_penalty,omitempty"`

	// LogitBias modifies likelihood of specific tokens appearing in completion
	// Optional. Map token IDs to bias values from -100 to 100
	LogitBias map[string]int `json:"logit_bias,omitempty"`

	// PresencePenalty prevents repetition by penalizing tokens based on presence
	// Range: -2.0 to 2.0. Positive values increase likelihood of new topics
	// Optional. Default: 0
	PresencePenalty *float32 `json:"presence_penalty,omitempty"`

	// CustomHeader the http header passed to model when requesting model
	CustomHeader map[string]string `json:"custom_header"`

	// LogProbs specifies whether to return log probabilities of the output tokens.
	LogProbs bool `json:"log_probs"`

	// TopLogProbs specifies the number of most likely tokens to return at each token position, each with an associated log probability.
	TopLogProbs int `json:"top_log_probs"`

	// ResponseFormat specifies the format that the model must output.
	ResponseFormat *ResponseFormat `json:"response_format,omitempty"`

	// Thinking controls whether the model is set to activate the deep thinking mode.
	// It is set to be enabled by default.
	Thinking *model.Thinking `json:"thinking,omitempty"`

	Cache *CacheConfig `json:"cache,omitempty"`
}

type CacheConfig struct {
	// APIType controls which API the cache uses to make calls.
	// Note that if the type is ResponsesAPI,
	// the following configuration will not be available (ARK may support it in the future):
	// `Region`, `AccessKey`, `SecretKey`, `Stop`, `FrequencyPenalty`, `LogitBias`, `PresencePenalty`,
	// `LogProbs`, `TopLogProbs`, `ResponseFormat.JSONSchema`.
	// It can be overridden by [WithCache].
	// Optional. Default: ContextAPI.
	APIType *APIType `json:"api_type,omitempty"`

	// SessionCache is the configuration of ResponsesAPI session cache.
	// It can be overridden by [WithCache].
	// Optional.
	SessionCache *SessionCacheConfig `json:"session_cache,omitempty"`
}

type SessionCacheConfig struct {
	// EnableCache specifies whether to enable session cache.
	// If enabled, the model will cache each conversation and reuse it for subsequent requests.
	EnableCache bool `json:"enable_cache"`

	// TTL specifies the survival time of cached data in seconds, with a maximum of 3 * 86400(3 days).
	TTL int `json:"ttl"`
}

type APIType string

const (
	// To learn more about ContextAPI, see https://www.volcengine.com/docs/82379/1528789
	ContextAPI APIType = "context_api"
	// To learn more about ResponsesAPI, see https://www.volcengine.com/docs/82379/1569618
	ResponsesAPI APIType = "responses_api"
)

type ResponseFormat struct {
	Type       model.ResponseFormatType                       `json:"type"`
	JSONSchema *model.ResponseFormatJSONSchemaJSONSchemaParam `json:"json_schema,omitempty"`
}

type caching string

const (
	cachingEnabled  caching = "enabled"
	cachingDisabled caching = "disabled"
)

func NewChatModel(_ context.Context, config *ChatModelConfig) (*ChatModel, error) {
	if config == nil {
		config = &ChatModelConfig{}
	}

	chatModel := buildChatCompletionAPIChatModel(config)

	respChatModel, err := buildResponsesAPIChatModel(config)
	if err != nil {
		return nil, err
	}

	return &ChatModel{
		chatModel:     chatModel,
		respChatModel: respChatModel,
	}, nil
}

func buildChatCompletionAPIChatModel(config *ChatModelConfig) *completionAPIChatModel {
	baseURL := defaultBaseURL
	if config.BaseURL != "" {
		baseURL = config.BaseURL
	}
	region := defaultRegion
	if config.Region != "" {
		region = config.Region
	}
	timeout := defaultTimeout
	if config.Timeout != nil {
		timeout = *config.Timeout
	}
	retryTimes := defaultRetryTimes
	if config.RetryTimes != nil {
		retryTimes = *config.RetryTimes
	}

	opts := []arkruntime.ConfigOption{
		arkruntime.WithRetryTimes(retryTimes),
		arkruntime.WithBaseUrl(baseURL),
		arkruntime.WithRegion(region),
		arkruntime.WithTimeout(timeout),
	}
	if config.HTTPClient != nil {
		opts = append(opts, arkruntime.WithHTTPClient(config.HTTPClient))
	}

	var client *arkruntime.Client
	if len(config.APIKey) > 0 {
		client = arkruntime.NewClientWithApiKey(config.APIKey, opts...)
	} else {
		client = arkruntime.NewClientWithAkSk(config.AccessKey, config.SecretKey, opts...)
	}

	cm := &completionAPIChatModel{
		client:           client,
		model:            config.Model,
		maxTokens:        config.MaxTokens,
		temperature:      config.Temperature,
		topP:             config.TopP,
		stop:             config.Stop,
		frequencyPenalty: config.FrequencyPenalty,
		logitBias:        config.LogitBias,
		presencePenalty:  config.PresencePenalty,
		customHeader:     config.CustomHeader,
		logProbs:         config.LogProbs,
		topLogProbs:      config.TopLogProbs,
		responseFormat:   config.ResponseFormat,
		thinking:         config.Thinking,
		cache:            config.Cache,
	}

	return cm
}

func buildResponsesAPIChatModel(config *ChatModelConfig) (*responsesAPIChatModel, error) {
	if config.Cache != nil && ptrFromOrZero(config.Cache.APIType) == ResponsesAPI {
		if err := checkResponsesAPIConfig(config); err != nil {
			return nil, err
		}
	}

	var opts []option.RequestOption

	if config.Timeout != nil {
		opts = append(opts, option.WithRequestTimeout(*config.Timeout))
	} else {
		opts = append(opts, option.WithRequestTimeout(defaultTimeout))
	}
	if config.HTTPClient != nil {
		opts = append(opts, option.WithHTTPClient(config.HTTPClient))
	}
	if config.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(config.BaseURL))
	} else {
		opts = append(opts, option.WithBaseURL(defaultBaseURL))
	}
	if config.RetryTimes != nil {
		opts = append(opts, option.WithMaxRetries(*config.RetryTimes))
	} else {
		opts = append(opts, option.WithMaxRetries(defaultRetryTimes))
	}
	if config.APIKey != "" {
		opts = append(opts, option.WithAPIKey(config.APIKey))
	}

	client := responses.NewResponseService(opts...)

	cm := &responsesAPIChatModel{
		client:         client,
		model:          config.Model,
		maxTokens:      config.MaxTokens,
		temperature:    config.Temperature,
		topP:           config.TopP,
		customHeader:   config.CustomHeader,
		responseFormat: config.ResponseFormat,
		thinking:       config.Thinking,
		cache:          config.Cache,
	}

	return cm, nil
}

func checkResponsesAPIConfig(config *ChatModelConfig) error {
	if config.Region != "" {
		return fmt.Errorf("'Region' is not supported by ResponsesAPI")
	}
	if config.APIKey == "" {
		if config.AccessKey != "" {
			return fmt.Errorf("'AccessKey' is not supported by ResponsesAPI")
		}
		if config.SecretKey != "" {
			return fmt.Errorf("'SecretKey' is not supported by ResponsesAPI")
		}
	}
	if len(config.Stop) > 0 {
		return fmt.Errorf("'Stop' is not supported by ResponsesAPI")
	}
	if config.FrequencyPenalty != nil {
		return fmt.Errorf("'FrequencyPenalty' is not supported by ResponsesAPI")
	}
	if len(config.LogitBias) > 0 {
		return fmt.Errorf("'LogitBias' is not supported by ResponsesAPI")
	}
	if config.PresencePenalty != nil {
		return fmt.Errorf("'PresencePenalty' is not supported by ResponsesAPI")
	}
	if config.LogProbs {
		return fmt.Errorf("'LogProbs' is not supported by ResponsesAPI")
	}
	if config.TopLogProbs > 0 {
		return fmt.Errorf("'TopLogProbs' is not supported by ResponsesAPI")
	}
	if config.ResponseFormat != nil && config.ResponseFormat.JSONSchema != nil {
		return fmt.Errorf("'ResponseFormat.JSONSchema' is not supported by ResponsesAPI")
	}
	return nil
}

type ChatModel struct {
	respChatModel *responsesAPIChatModel
	chatModel     *completionAPIChatModel
}

type CacheInfo struct {
	// ContextID specifies the id of prefix that can be used with [WithCache] option.
	ContextID string
	// Usage specifies the token usage of prefix
	Usage schema.TokenUsage
}

func (cm *ChatModel) Generate(ctx context.Context, in []*schema.Message, opts ...fmodel.Option) (
	outMsg *schema.Message, err error) {

	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)

	ok, err := cm.callByResponsesAPI(opts...)
	if err != nil {
		return nil, err
	}
	if ok {
		return cm.respChatModel.Generate(ctx, in, opts...)
	}

	return cm.chatModel.Generate(ctx, in, opts...)
}

func (cm *ChatModel) Stream(ctx context.Context, in []*schema.Message, opts ...fmodel.Option) (
	outStream *schema.StreamReader[*schema.Message], err error) {

	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)

	ok, err := cm.callByResponsesAPI(opts...)
	if err != nil {
		return nil, err
	}
	if ok {
		return cm.respChatModel.Stream(ctx, in, opts...)
	}

	return cm.chatModel.Stream(ctx, in, opts...)
}

func (cm *ChatModel) callByResponsesAPI(opts ...fmodel.Option) (bool, error) {
	var cacheOpt *CacheOption

	if cm.respChatModel.cache != nil {
		apiType := ContextAPI
		if cm.respChatModel.cache.APIType != nil {
			apiType = *cm.respChatModel.cache.APIType
		}
		cacheOpt = &CacheOption{
			APIType:      apiType,
			SessionCache: cm.respChatModel.cache.SessionCache,
		}
	}

	arkOpts := fmodel.GetImplSpecificOptions(&arkOptions{
		cache: cacheOpt,
	}, opts...)

	if arkOpts.cache != nil {
		switch arkOpts.cache.APIType {
		case ResponsesAPI:
			return true, nil
		case ContextAPI:
			return false, nil
		default:
			return false, fmt.Errorf("invalid api type: %s", arkOpts.cache.APIType)
		}
	}

	return false, nil
}

func (cm *ChatModel) WithTools(tools []*schema.ToolInfo) (fmodel.ToolCallingChatModel, error) {
	if len(tools) == 0 {
		return nil, errors.New("no tools to bind")
	}

	arkTools, err := cm.chatModel.toTools(tools)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to ark tools: %w", err)
	}

	respTools, err := cm.respChatModel.toTools(tools)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to ark responsesAPI tools: %w", err)
	}

	ncm := *cm.chatModel
	ncm.rawTools = tools
	ncm.tools = arkTools

	nrcm := *cm.respChatModel
	nrcm.rawTools = tools
	nrcm.tools = respTools

	return &ChatModel{
		chatModel:     &ncm,
		respChatModel: &nrcm,
	}, nil
}

func (cm *ChatModel) BindTools(tools []*schema.ToolInfo) (err error) {
	if len(tools) == 0 {
		return errors.New("no tools to bind")
	}

	cm.chatModel.tools, err = cm.chatModel.toTools(tools)
	if err != nil {
		return err
	}

	cm.respChatModel.tools, err = cm.respChatModel.toTools(tools)
	if err != nil {
		return err
	}

	cm.chatModel.rawTools = tools
	cm.respChatModel.rawTools = tools

	return nil
}

func (cm *ChatModel) GetType() string {
	return getType()
}

func (cm *ChatModel) IsCallbacksEnabled() bool {
	return true
}

// CreatePrefixCache creates a prefix context on the server side.
// In each subsequent turn of conversation, use [WithCache] to pass in the ContextID.
// The server will input the prefix cached context and this turn of input into the model for processing.
// This improves efficiency by reducing token usage and request size.
//
// Parameters:
//   - ctx: The context for the request
//   - prefix: Initial messages to be cached as prefix context
//   - ttl: Time-to-live in seconds for the cached prefix, default: 86400
//
// Returns:
//   - info: Information about the created prefix cache, including the context ID and token usage
//   - err: Any error encountered during the operation
//
// ref: https://www.volcengine.com/docs/82379/1396490#_1-%E5%88%9B%E5%BB%BA%E5%89%8D%E7%BC%80%E7%BC%93%E5%AD%98
//
// Note:
//   - It is unavailable for doubao models of version 1.6 and above.
//   - Currently, only supports calling by ContextAPI.
func (cm *ChatModel) CreatePrefixCache(ctx context.Context, prefix []*schema.Message, ttl int) (info *CacheInfo, err error) {
	if cm.respChatModel.cache != nil && ptrFromOrZero(cm.respChatModel.cache.APIType) == ResponsesAPI {
		return nil, fmt.Errorf("CreatePrefixCache is not supported by ResponsesAPI")
	}
	return cm.createContextByContextAPI(ctx, prefix, ttl, model.ContextModeCommonPrefix, nil)
}

// CreateSessionCache creates an initial session context on the server side.
// It returns an initial context ID.
// In each subsequent turn of conversation, use [WithCache] to pass in the ContextID.
// The server will input all cached context and this turn of input into the model for processing.
// This turn of conversation will also be automatically cached.
// Suitable for use in multi-turn conversation scenarios.
// Note that it does not apply to concurrent requests.
//
// Parameters:
//   - ctx: The context for the request
//   - prefix: Initial messages to be cached as prefix context
//   - ttl: Time-to-live in seconds for the cached prefix, default: 86400
//   - truncation: Truncation strategy, default: nil
//
// Returns:
//   - info: Information about the created session cache, including the context ID and token usage
//   - err: Any error encountered during the operation
//
// ref: https://www.volcengine.com/docs/82379/1396491?redirect=1#%E5%BF%AB%E9%80%9F%E5%BC%80%E5%A7%8B
//
// Note:
//   - It is unavailable for doubao models of version 1.6 and above.
//   - Only supports calling by ContextAPI.
func (cm *ChatModel) CreateSessionCache(ctx context.Context, prefix []*schema.Message, ttl int, truncation *model.TruncationStrategy) (info *CacheInfo, err error) {
	if cm.respChatModel.cache != nil && ptrFromOrZero(cm.respChatModel.cache.APIType) == ResponsesAPI {
		return nil, fmt.Errorf("CreateSessionCache is not supported by ResponsesAPI")
	}
	return cm.createContextByContextAPI(ctx, prefix, ttl, model.ContextModeSession, truncation)
}

func (cm *ChatModel) createContextByContextAPI(ctx context.Context, prefix []*schema.Message, ttl int, mode model.ContextMode,
	truncation *model.TruncationStrategy) (info *CacheInfo, err error) {

	req := model.CreateContextRequest{
		Model:              cm.chatModel.model,
		Mode:               mode,
		Messages:           make([]*model.ChatCompletionMessage, 0, len(prefix)),
		TTL:                nil,
		TruncationStrategy: truncation,
	}
	for _, msg := range prefix {
		content, err := cm.chatModel.toArkContent(msg.Content, msg.MultiContent)
		if err != nil {
			return nil, fmt.Errorf("convert message fail: %w", err)
		}

		req.Messages = append(req.Messages, &model.ChatCompletionMessage{
			Content:    content,
			Role:       string(msg.Role),
			ToolCallID: msg.ToolCallID,
			ToolCalls:  cm.chatModel.toArkToolCalls(msg.ToolCalls),
		})
	}
	if ttl > 0 {
		req.TTL = &ttl
	}

	resp, err := cm.chatModel.client.CreateContext(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("CreateContext fail: %w", err)
	}

	return &CacheInfo{
		ContextID: resp.ID,
		Usage: schema.TokenUsage{
			PromptTokens: resp.Usage.PromptTokens,
			PromptTokenDetails: schema.PromptTokenDetails{
				CachedTokens: resp.Usage.PromptTokensDetails.CachedTokens,
			},
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}, nil
}

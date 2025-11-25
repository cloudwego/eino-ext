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

// Package ark implements chat model for ark runtime.
package ark

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/components/model"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model/responses"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/agency"
	"github.com/cloudwego/eino/schema"
)

var _ agency.AgenticModel = (*AgenticModel)(nil)

var (
	// all default values are from github.com/volcengine/volcengine-go-sdk/service/arkruntime/config.go
	defaultBaseURL = "https://ark.cn-beijing.volces.com/api/v3"
	defaultRegion  = "cn-beijing"
)

var (
	ErrEmptyResponse = errors.New("empty response received from model")
)

type Config struct {
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

	MaxTokens *int64 `json:"max_tokens,omitempty"`

	Temperature *float64 `json:"temperature,omitempty"`

	TopP *float64 `json:"top_p,omitempty"`

	Text *responses.ResponsesText

	MaxToolCalls *int64 `json:"max_tool_calls,omitempty"`

	Thinking *responses.ResponsesThinking

	Reasoning *responses.ResponsesReasoning `json:"reasoning,omitempty"`

	Cache *CacheConfig `json:"cache,omitempty"`

	CustomHeader map[string]string `json:"custom_header,omitempty"`
}

type CacheConfig struct {
	// SessionCache is the configuration of ResponsesAPI session cache.
	// It can be overridden by [WithCache].
	// Optional.
	SessionCache *SessionCacheConfig `json:"session_cache,omitempty"`
}

type SessionCacheConfig struct {
	// EnableCache controls whether session caching is active.
	// When enabled, the model stores both inputs and responses for each conversation turn,
	// allowing them to be retrieved later via API.
	// Response IDs are saved in output messages and can be accessed using GetResponseID.
	// For multi-turn conversations, the ARK ChatModel automatically identifies the most recent
	// cached message from all inputs and passes its response ID to model to maintain context continuity.
	// This message and all previous ones are trimmed before being sent to the model.
	// When both HeadPreviousResponseID and cached message exist, the message's response ID takes precedence.
	// Use InvalidateMessageCaches to disables caching for the specified messages.
	EnableCache bool `json:"enable_cache"`

	// TTL specifies the survival time of cached data in seconds, with a maximum of 3 * 86400(3 days).
	TTL int `json:"ttl"`
}

func New(_ context.Context, config *Config) (*AgenticModel, error) {
	if config == nil {
		config = &Config{}
	}

	c, err := buildClient(config)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func buildClient(config *Config) (*AgenticModel, error) {
	var opts []arkruntime.ConfigOption

	if config.Region == "" {
		opts = append(opts, arkruntime.WithRegion(defaultRegion))
	} else {
		opts = append(opts, arkruntime.WithRegion(config.Region))
	}

	if config.Timeout != nil {
		opts = append(opts, arkruntime.WithTimeout(*config.Timeout))
	}
	if config.HTTPClient != nil {
		opts = append(opts, arkruntime.WithHTTPClient(config.HTTPClient))
	}
	if config.RetryTimes != nil {
		opts = append(opts, arkruntime.WithRetryTimes(*config.RetryTimes))
	}
	if config.BaseURL != "" {
		opts = append(opts, arkruntime.WithBaseUrl(config.BaseURL))
	} else {
		opts = append(opts, arkruntime.WithBaseUrl(defaultBaseURL))
	}

	var client *arkruntime.Client
	if len(config.APIKey) > 0 {
		client = arkruntime.NewClientWithApiKey(config.APIKey, opts...)
	} else if config.AccessKey != "" && config.SecretKey != "" {
		client = arkruntime.NewClientWithAkSk(config.AccessKey, config.SecretKey, opts...)
	} else {
		return nil, fmt.Errorf("new client fail, missing credentials: set 'APIKey' or both 'AccessKey' and 'SecretKey'")
	}

	cm := &AgenticModel{
		cli:          client,
		model:        config.Model,
		maxTokens:    config.MaxTokens,
		temperature:  config.Temperature,
		topP:         config.TopP,
		text:         config.Text,
		thinking:     config.Thinking,
		reasoning:    config.Reasoning,
		cache:        config.Cache,
		customHeader: config.CustomHeader,
	}

	return cm, nil
}

type AgenticModel struct {
	cli *arkruntime.Client

	tools      []*responses.ResponsesTool
	rawTools   []*schema.ToolInfo
	toolChoice *schema.ToolChoice

	model       string
	maxTokens   *int64
	temperature *float64
	topP        *float64
	serviceTier *string
	text        *responses.ResponsesText
	thinking    *responses.ResponsesThinking
	reasoning   *responses.ResponsesReasoning

	cache        *CacheConfig
	customHeader map[string]string
}

type CacheInfo struct {
	// ResponseID return by ResponsesAPI, it's specifies the id of prefix that can be used with [WithCache.HeadPreviousResponseID] option.
	ResponseID string
	// Usage specifies the token usage of prefix
	Usage schema.TokenUsage
}

func (am *AgenticModel) Generate(ctx context.Context, input []*schema.AgenticMessage, opts ...agency.Option) (
	outMsg *schema.AgenticMessage, err error) {

	ctx = callbacks.EnsureRunInfo(ctx, am.GetType(), components.ComponentOfAgenticModel)

}

func (am *AgenticModel) Stream(ctx context.Context, input []*schema.AgenticMessage, opts ...agency.Option) (
	outStream *schema.StreamReader[*schema.AgenticMessage], err error) {

	ctx = callbacks.EnsureRunInfo(ctx, am.GetType(), components.ComponentOfAgenticModel)

}

func (am *AgenticModel) WithTools(tools []*schema.ToolInfo) (agency.AgenticModel, error) {
	if len(tools) == 0 {
		return nil, errors.New("no tools to bind")
	}

	arkTools, err := am.toTools(tools)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to ark tools: %w", err)
	}

	tc := schema.ToolChoiceAllowed

	nam := *am
	nam.rawTools = tools
	nam.tools = arkTools
	nam.toolChoice = &tc

	return &nam, nil
}

func (am *AgenticModel) GetType() string {
	return getType()
}

func (am *AgenticModel) IsCallbacksEnabled() bool {
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
func (am *AgenticModel) CreatePrefixCache(ctx context.Context, prefix []*schema.Message, ttl int, opts ...agency.Option) (info *CacheInfo, err error) {

}

func (am *AgenticModel) toTools(tis []*schema.ToolInfo) ([]*responses.ResponsesTool, error) {
	tools := make([]*responses.ResponsesTool, len(tis))
	for i := range tis {
		ti := tis[i]
		if ti == nil {
			return nil, fmt.Errorf("tool info cannot be nil in WithTools")
		}

		paramsJSONSchema, err := ti.ParamsOneOf.ToJSONSchema()
		if err != nil {
			return nil, fmt.Errorf("failed to convert tool parameters to JSONSchema: %w", err)
		}

		b, err := sonic.Marshal(paramsJSONSchema)
		if err != nil {
			return nil, fmt.Errorf("marshal paramsJSONSchema fail: %w", err)
		}

		tools[i] = &responses.ResponsesTool{
			Union: &responses.ResponsesTool_ToolFunction{
				ToolFunction: &responses.ToolFunction{
					Name:        ti.Name,
					Type:        responses.ToolType_function,
					Description: &ti.Desc,
					Parameters: &responses.Bytes{
						Value: b,
					},
				},
			},
		}
	}

	return tools, nil
}

func (am *AgenticModel) genRequestAndOptions(in []*schema.Message, options *model.Options,
	specOptions *arkOptions) (responseReq *responses.ResponsesRequest, err error) {

	responseReq = &responses.ResponsesRequest{
		Text:      am.text,
		Thinking:  specOptions.thinking,
		Reasoning: specOptions.reasoning,
	}

	if options.Model != nil {
		responseReq.Model = *options.Model
	}
	if options.MaxTokens != nil {
		responseReq.MaxOutputTokens = ptrOf(int64(*options.MaxTokens))
	}
	if options.Temperature != nil {
		responseReq.Temperature = ptrOf(float64(*options.Temperature))
	}
	if options.TopP != nil {
		responseReq.TopP = ptrOf(float64(*options.TopP))
	}

	if am.serviceTier != nil {
		switch *am.serviceTier {
		case "auto":
			responseReq.ServiceTier = responses.ResponsesServiceTier_auto.Enum()
		case "default":
			responseReq.ServiceTier = responses.ResponsesServiceTier_default.Enum()
		}
	}

	in, err = am.populateCache(in, responseReq, specOptions)
	if err != nil {
		return nil, err
	}

	err = am.populateInput(in, responseReq)
	if err != nil {
		return nil, err
	}

	err = am.populateTools(responseReq, options.Tools, options.ToolChoice)
	if err != nil {
		return nil, err
	}

	return responseReq, nil
}

func (am *AgenticModel) populateCache(in []*schema.Message, responseReq *responses.ResponsesRequest, arkOpts *arkOptions,
) ([]*schema.Message, error) {

	var (
		store       = false
		cacheStatus = cachingDisabled
		cacheTTL    *int
		headRespID  *string
		contextID   *string
	)

	if am.cache != nil {
		if sCache := am.cache.SessionCache; sCache != nil {
			if sCache.EnableCache {
				store = true
				cacheStatus = cachingEnabled
			}
			cacheTTL = &sCache.TTL
		}
	}

	if cacheOpt := arkOpts.cache; cacheOpt != nil {
		headRespID = cacheOpt.HeadPreviousResponseID

		if sCacheOpt := cacheOpt.SessionCache; sCacheOpt != nil {
			cacheTTL = &sCacheOpt.TTL

			if sCacheOpt.EnableCache {
				store = true
				cacheStatus = cachingEnabled
			} else {
				store = false
				cacheStatus = cachingDisabled
			}
		}
	}

	var (
		preRespID *string
		inputIdx  int
	)

	now := time.Now().Unix()

	// If the user implements session caching with ContextID,
	// ContextID and ResponseID will exist at the same time.
	// Using ContextID is prioritized to maintain compatibility with the old logic.
	// In this usage scenario, ResponseID cannot be used.
	if cacheStatus == cachingEnabled {
		for i := len(in) - 1; i >= 0; i-- {
			msg := in[i]
			inputIdx = i
			if expireAtSec, ok := getCacheExpiration(msg); !ok || expireAtSec < now {
				continue
			}
			if id, ok := GetResponseID(msg); ok {
				preRespID = &id
				break
			}
		}
	}

	if preRespID != nil {
		if inputIdx+1 >= len(in) {
			return in, fmt.Errorf("not found incremental input after ResponseID")
		}
		in = in[inputIdx+1:]
	}

	// ResponseID has a higher priority than HeadPreviousResponseID
	if preRespID == nil {
		preRespID = headRespID
	}

	responseReq.PreviousResponseId = preRespID
	responseReq.Store = &store

	if cacheTTL != nil {
		responseReq.ExpireAt = ptrOf(now + int64(*cacheTTL))
	}

	var cacheType *responses.CacheType_Enum
	if cacheStatus == cachingDisabled {
		cacheType = responses.CacheType_disabled.Enum()
	} else {
		cacheType = responses.CacheType_enabled.Enum()
	}

	responseReq.Caching = &responses.ResponsesCaching{
		Type: cacheType,
	}

	return in, nil
}

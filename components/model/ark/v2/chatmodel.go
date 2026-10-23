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

// Package ark implements the Ark Chat Completions API using ark-runtime-go.
package ark

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	fmodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/chat"
)

var _ fmodel.ToolCallingChatModel = (*ChatModel)(nil)

// ErrEmptyResponse indicates that the API returned no completion choices.
var ErrEmptyResponse = errors.New("empty response received from model")

// ChatModelConfig configures the Ark service connection, generation parameters,
// function calling, and synchronous batch Chat Completions.
type ChatModelConfig struct {
	// Timeout defaults to 10 minutes. HTTPClient, when provided, takes precedence.
	Timeout    *time.Duration `json:"timeout,omitempty"`
	HTTPClient *http.Client   `json:"http_client,omitempty"`
	// RetryTimes defaults to 2. A pointer to zero disables retries.
	RetryTimes *int `json:"retry_times,omitempty"`
	// BaseURL defaults to https://ark.cn-beijing.volces.com/api/v3.
	BaseURL string `json:"base_url,omitempty"`
	// Region defaults to cn-beijing.
	Region string `json:"region,omitempty"`
	// APIKey takes precedence over AccessKey and SecretKey.
	APIKey              string                             `json:"api_key,omitempty"`
	AccessKey           string                             `json:"access_key,omitempty"`
	SecretKey           string                             `json:"secret_key,omitempty"`
	Model               string                             `json:"model"`
	MaxTokens           *int                               `json:"max_tokens,omitempty"`
	MaxCompletionTokens *int                               `json:"max_completion_tokens,omitempty"`
	Temperature         *float32                           `json:"temperature,omitempty"`
	TopP                *float32                           `json:"top_p,omitempty"`
	Stop                []string                           `json:"stop,omitempty"`
	FrequencyPenalty    *float32                           `json:"frequency_penalty,omitempty"`
	PresencePenalty     *float32                           `json:"presence_penalty,omitempty"`
	RepetitionPenalty   *float32                           `json:"repetition_penalty,omitempty"`
	LogitBias           map[string]int                     `json:"logit_bias,omitempty"`
	LogProbs            bool                               `json:"log_probs,omitempty"`
	TopLogProbs         int                                `json:"top_log_probs,omitempty"`
	ResponseFormat      *chat.ChatCompletionResponseFormat `json:"response_format,omitempty"`
	Thinking            *chat.Thinking                     `json:"thinking,omitempty"`
	ServiceTier         *string                            `json:"service_tier,omitempty"`
	ReasoningEffort     *chat.ReasoningEffort              `json:"reasoning_effort,omitempty"`
	ParallelToolCalls   *bool                              `json:"parallel_tool_calls,omitempty"`
	User                *string                            `json:"user,omitempty"`
	// ToolStrict sets the default strict schema adherence for function tools.
	// WithToolStrict overrides it for an individual tool. Nil omits the field
	// for tools without an explicit override.
	ToolStrict                   *bool                        `json:"tool_strict,omitempty"`
	CustomHeader                 map[string]string            `json:"custom_header,omitempty"`
	BatchChat                    *BatchChatConfig             `json:"batch_chat,omitempty"`
	RequestPayloadModifier       RequestPayloadModifier       `json:"-"`
	ResponseMessageModifier      ResponseMessageModifier      `json:"-"`
	ResponseChunkMessageModifier ResponseChunkMessageModifier `json:"-"`
}

// BatchChatConfig configures synchronous batch inference. Streaming is unsupported.
type BatchChatConfig struct {
	EnableBatchChat            bool          `json:"enable_batch_chat,omitempty"`
	BatchChatAsyncRetryTimeout time.Duration `json:"batch_chat_async_retry_timeout,omitempty"`
	BatchMaxParallel           *int          `json:"batch_max_parallel,omitempty"`
}

type ChatModel struct {
	client     *arkruntime.Client
	config     ChatModelConfig
	tools      []chat.ChatCompletionTool
	rawTools   []*schema.ToolInfo
	toolChoice *schema.ToolChoice
}

func NewChatModel(_ context.Context, config *ChatModelConfig) (*ChatModel, error) {
	if config == nil {
		config = &ChatModelConfig{}
	}
	opts := []arkruntime.ConfigOption{}
	if config.BaseURL != "" {
		opts = append(opts, arkruntime.WithBaseUrl(config.BaseURL))
	}
	if config.Region != "" {
		opts = append(opts, arkruntime.WithRegion(config.Region))
	}
	if config.Timeout != nil {
		opts = append(opts, arkruntime.WithTimeout(*config.Timeout))
	}
	if config.RetryTimes != nil {
		opts = append(opts, arkruntime.WithRetryTimes(*config.RetryTimes))
	}
	if config.HTTPClient != nil {
		opts = append(opts, arkruntime.WithHTTPClient(config.HTTPClient))
	}
	if b := config.BatchChat; b != nil && b.EnableBatchChat {
		if b.BatchChatAsyncRetryTimeout <= 0 {
			return nil, errors.New("batch chat timeout must be positive when enable batch chat")
		}
		if b.BatchMaxParallel != nil {
			if *b.BatchMaxParallel <= 0 {
				return nil, errors.New("batch max parallel must be positive")
			}
			opts = append(opts, arkruntime.WithBatchMaxParallel(*b.BatchMaxParallel))
		}
	}
	// Copy the SDK-configured HTTP client so capturing response bodies does not
	// mutate a caller-owned client or change its transport/timeout policies.
	connection := arkruntime.NewClientConfig(config.APIKey, config.AccessKey, config.SecretKey, opts...)
	httpClient := *connection.HTTPClient
	transport := httpClient.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	httpClient.Transport = &responseCaptureTransport{next: transport}
	opts = append(opts, arkruntime.WithHTTPClient(&httpClient))
	var client *arkruntime.Client
	if config.APIKey != "" {
		client = arkruntime.NewClientWithApiKey(config.APIKey, opts...)
	} else {
		client = arkruntime.NewClientWithAkSk(config.AccessKey, config.SecretKey, opts...)
	}
	return &ChatModel{client: client, config: *config}, nil
}

func (cm *ChatModel) WithTools(tools []*schema.ToolInfo) (fmodel.ToolCallingChatModel, error) {
	if len(tools) == 0 {
		return nil, errors.New("no tools to bind")
	}
	converted, err := toTools(tools, cm.config.ToolStrict)
	if err != nil {
		return nil, fmt.Errorf("convert tools: %w", err)
	}
	next := *cm
	next.tools, next.rawTools = converted, tools
	return &next, nil
}

// BindTools mutates the model. Prefer WithTools when sharing a model concurrently.
func (cm *ChatModel) BindTools(tools []*schema.ToolInfo) error {
	next, err := cm.WithTools(tools)
	if err != nil {
		return err
	}
	*cm = *next.(*ChatModel)
	return nil
}

// BindForcedTools binds tools and requires a tool call.
func (cm *ChatModel) BindForcedTools(tools []*schema.ToolInfo) error {
	if err := cm.BindTools(tools); err != nil {
		return err
	}
	choice := schema.ToolChoiceForced
	cm.toolChoice = &choice
	return nil
}

func (cm *ChatModel) GetType() string          { return "Ark" }
func (cm *ChatModel) IsCallbacksEnabled() bool { return true }

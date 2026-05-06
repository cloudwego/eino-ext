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

package openaigo

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
)

var _ model.ToolCallingChatModel = (*ChatModel)(nil)

type Config struct {
	APIKey string `json:"api_key"`

	// Timeout specifies the maximum duration to wait for API responses.
	// If HTTPClient is set, Timeout will not be used.
	// Optional. Default: no timeout
	Timeout time.Duration `json:"timeout"`

	// HTTPClient specifies the client to send HTTP requests.
	// If HTTPClient is set, Timeout will not be used.
	// Optional. Default &http.Client{Timeout: Timeout}
	HTTPClient *http.Client `json:"http_client"`

	// BaseURL specifies the OpenAI endpoint URL
	// Optional. Default: https://api.openai.com/v1
	BaseURL string `json:"base_url"`

	// Model specifies the ID of the model to use.
	// Optional.
	Model string `json:"model,omitempty"`

	// MaxOutputTokens is an upper bound for the number of tokens that can be generated for a response,
	// including visible output tokens and reasoning tokens.
	MaxOutputTokens *int `json:"max_output_tokens,omitempty"`

	TopP        *float32 `json:"top_p,omitempty"`
	Temperature *float32 `json:"temperature,omitempty"`

	// Reasoning config for reasoning models.
	Reasoning *Reasoning `json:"reasoning,omitempty"`

	// Store indicates whether to store the generated model response for later retrieval.
	Store *bool `json:"store,omitempty"`

	// Metadata set of key-value pairs that can be attached to an object.
	Metadata map[string]string `json:"metadata,omitempty"`

	// ExtraFields will override any existing fields with the same key.
	// Optional. Useful for experimental features not yet officially supported.
	ExtraFields map[string]any `json:"extra_fields,omitempty"`
}

type ChatModel struct {
	cli openai.Client

	model       string
	maxOutTok   *int
	topP        *float32
	temperature *float32
	reasoning   *Reasoning
	store       *bool
	metadata    map[string]string
	extraFields map[string]any

	tools      []responses.ToolUnionParam
	rawTools   []*schema.ToolInfo
	toolChoice *schema.ToolChoice
}

func NewChatModel(_ context.Context, config *Config) (*ChatModel, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	opts := make([]option.RequestOption, 0, 4)
	if config.APIKey != "" {
		opts = append(opts, option.WithAPIKey(config.APIKey))
	}
	if config.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(config.BaseURL))
	}
	if config.HTTPClient != nil {
		opts = append(opts, option.WithHTTPClient(config.HTTPClient))
	} else if config.Timeout > 0 {
		opts = append(opts, option.WithHTTPClient(&http.Client{Timeout: config.Timeout}))
	}

	cli := openai.NewClient(opts...)

	cm := &ChatModel{
		cli:         cli,
		model:       config.Model,
		maxOutTok:   config.MaxOutputTokens,
		topP:        config.TopP,
		temperature: config.Temperature,
		reasoning:   config.Reasoning,
		store:       config.Store,
		metadata:    cloneStringMap(config.Metadata),
		extraFields: cloneAnyMap(config.ExtraFields),
	}

	return cm, nil
}

func (cm *ChatModel) Generate(ctx context.Context, in []*schema.Message, opts ...model.Option) (outMsg *schema.Message, err error) {
	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)

	params, cbIn, err := cm.buildParams(in, false, opts...)
	if err != nil {
		return nil, err
	}

	ctx = callbacks.OnStart(ctx, cbIn)
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	resp, err := cm.cli.Responses.New(ctx, params)
	if err != nil {
		return nil, err
	}

	outMsg, err = cm.convertResponseToMessage(resp)
	if err != nil {
		return nil, err
	}

	callbacks.OnEnd(ctx, &model.CallbackOutput{
		Message:    outMsg,
		Config:     cbIn.Config,
		TokenUsage: toModelTokenUsage(outMsg.ResponseMeta),
		Extra: map[string]any{
			callbackExtraModelName: string(resp.Model),
		},
	})

	return outMsg, nil
}

func (cm *ChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	if len(tools) == 0 {
		return nil, errors.New("no tools to bind")
	}
	openAITools, rawTools, err := toOpenAITools(tools)
	if err != nil {
		return nil, err
	}

	tc := schema.ToolChoiceAllowed
	ncm := *cm
	ncm.tools = openAITools
	ncm.rawTools = rawTools
	ncm.toolChoice = &tc
	return &ncm, nil
}

const typ = "OpenAI"

func (cm *ChatModel) GetType() string { return typ }

func (cm *ChatModel) IsCallbacksEnabled() bool { return true }

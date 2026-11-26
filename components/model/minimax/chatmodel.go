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

package minimax

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/cloudwego/eino-ext/libs/acl/openai"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	openai2 "github.com/meguminnnnnnnnn/go-openai"
)

var _ model.ToolCallingChatModel = (*ChatModel)(nil)
var _ model.ChatModel = (*ChatModel)(nil)

const (
	defaultBaseURL = "https://api.minimax.io/v1"
	defaultModel   = "MiniMax-M2.7"
)

// Config holds the configuration for the MiniMax ChatModel.
type Config struct {
	// APIKey is your MiniMax API key.
	// Required. Can also be set via the MINIMAX_API_KEY environment variable.
	APIKey string `json:"api_key"`

	// Timeout specifies the maximum duration to wait for API responses.
	// If HTTPClient is set, Timeout will not be used.
	// Optional. Default: no timeout
	Timeout time.Duration `json:"timeout"`

	// HTTPClient specifies the client to send HTTP requests.
	// If HTTPClient is set, Timeout will not be used.
	// Optional. Default: &http.Client{Timeout: Timeout}
	HTTPClient *http.Client `json:"http_client"`

	// BaseURL specifies the MiniMax API endpoint URL.
	// Optional. Default: https://api.minimax.io/v1
	// For users in mainland China, use: https://api.minimaxi.com/v1
	BaseURL string `json:"base_url"`

	// Model specifies the ID of the model to use.
	// Supported models: MiniMax-M2.7, MiniMax-M2.7-highspeed
	// Optional. Default: MiniMax-M2.7
	Model string `json:"model"`

	// MaxTokens limits the maximum number of tokens that can be generated in the chat completion.
	// Optional. Default: model's maximum
	MaxTokens *int `json:"max_tokens,omitempty"`

	// Temperature specifies what sampling temperature to use.
	// Range: (0.0, 1.0]. MiniMax does not accept temperature=0.
	// Optional. Default: 1.0
	Temperature *float32 `json:"temperature,omitempty"`

	// TopP controls diversity via nucleus sampling.
	// Range: 0.0 to 1.0. Lower values make output more focused.
	// Optional. Default: 1.0
	TopP *float32 `json:"top_p,omitempty"`

	// Stop sequences where the API will stop generating further tokens.
	// Optional.
	Stop []string `json:"stop,omitempty"`

	// User unique identifier representing end-user.
	// Optional.
	User *string `json:"user,omitempty"`

	// ExtraFields will override any existing fields with the same key.
	// Optional. Useful for experimental features not yet officially supported.
	ExtraFields map[string]any `json:"extra_fields,omitempty"`
}

// ChatModel is the MiniMax implementation of the Eino ChatModel interface.
// It uses the MiniMax Cloud API (OpenAI-compatible) to generate chat completions.
type ChatModel struct {
	cli *openai.Client
}

// NewChatModel creates a new MiniMax ChatModel with the given configuration.
func NewChatModel(ctx context.Context, config *Config) (*ChatModel, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	if config.APIKey == "" {
		return nil, fmt.Errorf("api_key is required")
	}

	var httpClient *http.Client
	if config.HTTPClient != nil {
		httpClient = config.HTTPClient
	} else {
		httpClient = &http.Client{Timeout: config.Timeout}
	}

	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	modelName := config.Model
	if modelName == "" {
		modelName = defaultModel
	}

	// Clamp temperature for MiniMax: must be in (0.0, 1.0]
	var temperature *float32
	if config.Temperature != nil {
		t := *config.Temperature
		if t <= 0 {
			t = 0.01
		}
		if t > 1.0 {
			t = 1.0
		}
		temperature = &t
	}

	nConf := &openai.Config{
		BaseURL:     baseURL,
		APIKey:      config.APIKey,
		HTTPClient:  httpClient,
		Model:       modelName,
		MaxTokens:   config.MaxTokens,
		Temperature: temperature,
		TopP:        config.TopP,
		Stop:        config.Stop,
		User:        config.User,
		ExtraFields: config.ExtraFields,
	}

	cli, err := openai.NewClient(ctx, nConf)
	if err != nil {
		return nil, err
	}

	return &ChatModel{cli: cli}, nil
}

// Generate sends a chat completion request to MiniMax and returns the response message.
func (cm *ChatModel) Generate(ctx context.Context, in []*schema.Message, opts ...model.Option) (
	outMsg *schema.Message, err error) {
	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)
	opts = cm.clampTemperatureOpts(opts...)
	out, err := cm.cli.Generate(ctx, in, opts...)
	if err != nil {
		return nil, convAPIError(err)
	}
	return out, nil
}

// Stream sends a streaming chat completion request to MiniMax and returns a stream reader.
func (cm *ChatModel) Stream(ctx context.Context, in []*schema.Message, opts ...model.Option) (
	outStream *schema.StreamReader[*schema.Message], err error) {
	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)
	opts = cm.clampTemperatureOpts(opts...)
	out, err := cm.cli.Stream(ctx, in, opts...)
	if err != nil {
		return nil, convAPIError(err)
	}
	return out, nil
}

// WithTools returns a new ChatModel with the specified tools bound for tool calling.
func (cm *ChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	cli, err := cm.cli.WithToolsForClient(tools)
	if err != nil {
		return nil, err
	}
	return &ChatModel{cli: cli}, nil
}

// BindTools binds tools to the current ChatModel instance.
// Deprecated: Use WithTools instead for thread safety.
func (cm *ChatModel) BindTools(tools []*schema.ToolInfo) error {
	return cm.cli.BindTools(tools)
}

const typ = "MiniMax"

// GetType returns the provider type name.
func (cm *ChatModel) GetType() string {
	return typ
}

// IsCallbacksEnabled returns whether callbacks are enabled.
func (cm *ChatModel) IsCallbacksEnabled() bool {
	return cm.cli.IsCallbacksEnabled()
}

// clampTemperatureOpts ensures runtime temperature options are within MiniMax's allowed range (0.0, 1.0].
func (cm *ChatModel) clampTemperatureOpts(opts ...model.Option) []model.Option {
	commonOpts := model.GetCommonOptions(&model.Options{}, opts...)
	if commonOpts.Temperature != nil {
		t := *commonOpts.Temperature
		clamped := false
		if t <= 0 {
			t = 0.01
			clamped = true
		}
		if t > 1.0 {
			t = 1.0
			clamped = true
		}
		if clamped {
			opts = append(opts, model.WithTemperature(t))
		}
	}
	return opts
}

// APIError represents an error returned by the MiniMax API.
type APIError struct {
	Code           any     `json:"code,omitempty"`
	Message        string  `json:"message"`
	Param          *string `json:"param,omitempty"`
	Type           string  `json:"type"`
	HTTPStatus     string  `json:"-"`
	HTTPStatusCode int     `json:"-"`
}

// Error returns the error message string.
func (e *APIError) Error() string {
	if e.HTTPStatusCode > 0 {
		return fmt.Sprintf("error, status code: %d, status: %s, message: %s",
			e.HTTPStatusCode, e.HTTPStatus, e.Message)
	}
	return e.Message
}

func convAPIError(err error) error {
	apiErr := &openai2.APIError{}
	if errors.As(err, &apiErr) {
		return &APIError{
			Code:           apiErr.Code,
			Message:        apiErr.Message,
			Param:          apiErr.Param,
			Type:           apiErr.Type,
			HTTPStatus:     apiErr.HTTPStatus,
			HTTPStatusCode: apiErr.HTTPStatusCode,
		}
	}
	return err
}

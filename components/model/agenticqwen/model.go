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

package agenticqwen

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

// AudioConfig specifies the audio output settings
type AudioConfig struct {
	// Format specifies the output audio format.
	Format AudioFormat `json:"format"`
	// Voice specifies the voice the model uses to respond.
	Voice AudioVoice `json:"voice"`
}

// Config parameters detail see:
// https://help.aliyun.com/zh/model-studio/developer-reference/use-qwen-by-calling-api?spm=a2c4g.11186623.help-menu-2400256.d_3_3_0.c3b24823WzuCqJ&scm=20140722.H_2712576._.OR_help-T_cn-DAS-zh-V_1
// https://help.aliyun.com/zh/model-studio/developer-reference/compatibility-of-openai-with-dashscope?spm=a2c4g.11186623.0.i49
type Config struct {

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

	// BaseURL specifies the QWen endpoint URL
	// Optional. Default: https://dashscope-intl.aliyuncs.com/compatible-mode/v1
	BaseURL string

	// Model specifies the ID of the model to use
	// Required
	Model string

	// MaxTokens limits the maximum number of tokens that can be generated in the chat completion
	// Optional. Default: model's maximum
	MaxTokens *int

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

	// Seed enables deterministic sampling for consistent outputs
	// Optional. Set for reproducible results
	Seed *int

	// FrequencyPenalty prevents repetition by penalizing tokens based on frequency
	// Range: -2.0 to 2.0. Positive values decrease likelihood of repetition
	// Optional. Default: 0
	FrequencyPenalty *float32

	// LogitBias modifies likelihood of specific tokens appearing in completion
	// Optional. Map token IDs to bias values from -100 to 100
	LogitBias map[string]int

	// User unique identifier representing end-user
	// Optional. Helps monitor and detect abuse
	User *string

	// EnableThinking enables thinking mode
	// https://help.aliyun.com/zh/model-studio/deep-thinking
	// Optional. Default: base on the Model
	EnableThinking *bool

	// PreserveThinking preserves thinking content in multi-turn conversations.
	// https://help.aliyun.com/zh/model-studio/deep-thinking
	// Optional. Default: false
	PreserveThinking *bool

	// Modalities specifies the output data modalities and is only supported by the Qwen-Omni model.
	// Possible values are:
	// - ["text", "audio"]: Output text and audio.
	// - ["text"]: Output text (default).
	Modalities []Modality

	// Audio parameters for audio output. Required when modalities includes "audio".
	Audio *AudioConfig
}

type Model struct {
	cli *openai.AgenticClient

	extraOptions *options
}

func New(ctx context.Context, config *Config) (*Model, error) {
	if config == nil {
		return nil, fmt.Errorf("[New] config not provided")
	}

	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://dashscope-intl.aliyuncs.com/compatible-mode/v1"
	}

	var httpClient *http.Client

	if config.HTTPClient != nil {
		httpClient = config.HTTPClient
	} else {
		httpClient = &http.Client{Timeout: config.Timeout}
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
		Seed:             config.Seed,
		FrequencyPenalty: config.FrequencyPenalty,
		LogitBias:        config.LogitBias,
		User:             config.User,
		Audio:            &openai.Audio{},
	}
	if len(config.Modalities) > 0 {
		ms := make([]openai.Modality, len(config.Modalities))
		for i, m := range config.Modalities {
			ms[i] = openai.Modality(m)
		}
		nConfig.Modalities = ms
	}
	if config.Audio != nil {
		nConfig.Audio = &openai.Audio{Format: string(config.Audio.Format), Voice: string(config.Audio.Voice)}
	}
	cli, err := openai.NewAgenticClient(ctx, nConfig)
	if err != nil {
		return nil, err
	}

	return &Model{
		cli: cli,
		extraOptions: &options{
			EnableThinking:   config.EnableThinking,
			PreserveThinking: config.PreserveThinking,
		},
	}, nil
}

func (m *Model) Generate(ctx context.Context, in []*schema.AgenticMessage, opts ...model.Option) (
	*schema.AgenticMessage, error) {

	opts = m.parseCustomOptions(opts...)
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

	opts = m.parseCustomOptions(opts...)
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

func (m *Model) parseCustomOptions(opts ...model.Option) []model.Option {
	qwenOpts := model.GetImplSpecificOptions(&options{
		EnableThinking:   m.extraOptions.EnableThinking,
		PreserveThinking: m.extraOptions.PreserveThinking,
	}, opts...)

	extraFields := make(map[string]any)
	if qwenOpts.EnableThinking != nil {
		enableThinkingSwitch := map[string]bool{
			"enable_thinking": *qwenOpts.EnableThinking,
		}
		extraFields["chat_template_kwargs"] = enableThinkingSwitch
		extraFields["enable_thinking"] = *qwenOpts.EnableThinking
	}
	if qwenOpts.PreserveThinking != nil {
		extraFields["preserve_thinking"] = *qwenOpts.PreserveThinking
	}
	if len(extraFields) > 0 {
		opts = append(opts, openai.WithExtraFields(extraFields))
	}
	return opts
}

func (m *Model) GetType() string {
	return implType
}

func (m *Model) IsCallbacksEnabled() bool {
	return m.cli.IsCallbacksEnabled()
}

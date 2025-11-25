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
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/cloudwego/eino-ext/libs/acl/openai"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

var _ model.ToolCallingChatModel = (*ChatModel)(nil)

type Modality = openai.Modality

type AudioFormat string

const (
	AudioFormatWav AudioFormat = "wav"
)

type AudioVoice string

const (
	AudioVoiceCherry  AudioVoice = "Cherry"
	AudioVoiceSerena  AudioVoice = "Serena"
	AudioVoiceEthan   AudioVoice = "Ethan"
	AudioVoiceChelsie AudioVoice = "Chelsie"
)

// Audio specifies the audio output settings
type Audio struct {
	// Format specifies the output audio format.
	Format AudioFormat `json:"format"`
	// Voice specifies the voice the model uses to respond.
	Voice AudioVoice `json:"voice"`
}

// ChatModelConfig parameters detail see:
// https://help.aliyun.com/zh/model-studio/developer-reference/use-qwen-by-calling-api?spm=a2c4g.11186623.help-menu-2400256.d_3_3_0.c3b24823WzuCqJ&scm=20140722.H_2712576._.OR_help-T_cn-DAS-zh-V_1
// https://help.aliyun.com/zh/model-studio/developer-reference/compatibility-of-openai-with-dashscope?spm=a2c4g.11186623.0.i49
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
	// Required. Example: https://dashscope.aliyuncs.com/compatible-mode/v1
	BaseURL string `json:"base_url"`

	// The following fields correspond to OpenAI's chat completion API parameters
	// Ref: https://platform.openai.com/docs/api-reference/chat/create

	// Model specifies the ID of the model to use
	// Required
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

	// EnableThinking enables thinking mode
	// https://help.aliyun.com/zh/model-studio/deep-thinking
	// Optional. Default: base on the Model
	EnableThinking *bool `json:"enable_thinking,omitempty"`

	// Modalities specifies the output data modalities and is only supported by the Qwen-Omni model.
	// Possible values are:
	// - ["text", "audio"]: Output text and audio.
	// - ["text"]: Output text (default).
	Modalities []Modality `json:"modalities,omitempty"`

	// Audio parameters for audio output. Required when modalities includes "audio".
	// To generate audio, include "audio". Audio generation is only supported by the Qwen-Omni model.
	// Voice options: Cherry, Serena, Ethan, Chelsie.
	// Format: currently only "wav" is supported.
	Audio *Audio `json:"audio,omitempty"`
}

type ChatModel struct {
	cli *openai.Client

	extraOptions *options
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
	nConfig := &openai.Config{
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
		Modalities:       config.Modalities,
		Audio:            &openai.Audio{},
	}
	if config.Audio != nil {
		nConfig.Audio = &openai.Audio{Format: string(config.Audio.Format), Voice: string(config.Audio.Voice)}
	}
	cli, err := openai.NewClient(ctx, nConfig)

	if err != nil {
		return nil, err
	}

	return &ChatModel{
		cli: cli,

		extraOptions: &options{
			EnableThinking: config.EnableThinking,
		},
	}, nil
}

func (cm *ChatModel) Generate(ctx context.Context, in []*schema.Message, opts ...model.Option) (
	outMsg *schema.Message, err error) {
	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)
	opts = cm.parseCustomOptions(opts...)
	opts = cm.appendBodyModifierOptions(in, opts...)
	return cm.cli.Generate(ctx, in, opts...)
}

func (cm *ChatModel) Stream(ctx context.Context, in []*schema.Message, opts ...model.Option) (outStream *schema.StreamReader[*schema.Message], err error) {
	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)
	opts = cm.parseCustomOptions(opts...)
	opts = cm.appendBodyModifierOptions(in, opts...)
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

func (cm *ChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	cli, err := cm.cli.WithToolsForClient(tools)
	if err != nil {
		return nil, err
	}
	return &ChatModel{cli: cli, extraOptions: cm.extraOptions}, nil
}

func (cm *ChatModel) BindTools(tools []*schema.ToolInfo) error {
	return cm.cli.BindTools(tools)
}

func (cm *ChatModel) BindForcedTools(tools []*schema.ToolInfo) error {
	return cm.cli.BindForcedTools(tools)
}

func (cm *ChatModel) parseCustomOptions(opts ...model.Option) []model.Option {
	qwenOpts := model.GetImplSpecificOptions(&options{
		EnableThinking: cm.extraOptions.EnableThinking,
	}, opts...)

	// Using extra fields to pass the custom options to the underlying client
	extraFields := make(map[string]any)
	if qwenOpts.EnableThinking != nil {
		// If using Bailian, use `chat_template_kwargs`.
		// See https://bailian.console.aliyun.com/?tab=api#/api/?type=model&url=2712576
		enableThinkingSwitch := map[string]bool{
			"enable_thinking": *qwenOpts.EnableThinking,
		}
		extraFields["chat_template_kwargs"] = enableThinkingSwitch

		// If using vLLM, use `enable_thinking`.
		// See https://qwen.readthedocs.io/zh-cn/latest/deployment/vllm.html
		extraFields["enable_thinking"] = *qwenOpts.EnableThinking
	}
	if len(extraFields) > 0 {
		opts = append(opts, openai.WithExtraFields(extraFields))
	}
	return opts
}

const (
	// field name in request body indicate body.messages
	fieldNameMessagesInReq = "messages"
	// field name in request body indicate body.messages[i].role
	fieldNameRoleInReq = "role"
	// field name in request body indicate body.messages[i].content
	fieldNameContentInReq = "content"
	// field name in one single message indicate one prefix message
	fieldNamePartialInMessage = "partial"
	// field name in message.Extra indicate one prefix message
	fieldNamePartialInExtraMsg = "_eino_qwen_partial"
)

func getKeyInMessage(message *schema.Message) string {
	return string(message.Role) + message.Content
}

func isNeedModifyMessageInRequest(messageInRequest map[string]any, messageKeys map[string]struct{}) bool {
	role := messageInRequest[fieldNameRoleInReq].(string)
	content := messageInRequest[fieldNameContentInReq].(string)
	key := role + content
	_, exist := messageKeys[key]
	return exist
}

func (cm *ChatModel) appendBodyModifierOptions(in []*schema.Message, opts ...model.Option) []model.Option {
	messageKeys := make(map[string]struct{})
	for _, message := range in {
		if len(message.Extra) == 0 {
			continue
		}
		if _, isPartialKeyExist := message.Extra[fieldNamePartialInExtraMsg]; isPartialKeyExist {
			messageKey := getKeyInMessage(message)
			messageKeys[messageKey] = struct{}{}
		}
	}
	if len(messageKeys) > 0 {
		opts = append(opts, withPartialMessageOption(messageKeys))
	}
	return opts
}

func withPartialMessageOption(messageKeys map[string]struct{}) model.Option {
	return openai.WithRequestBodyModifier(func(rawBody []byte) ([]byte, error) {
		return modifyRequestBody(rawBody, messageKeys), nil
	})
}

func modifyRequestBody(rawBody []byte, messageKeys map[string]struct{}) []byte {
	var data map[string]interface{}
	err := json.Unmarshal(rawBody, &data)
	if err != nil {
		log.Fatalf("invalid json body: error=%v", err)
		return rawBody
	}

	messages, ok := data[fieldNameMessagesInReq].([]interface{})
	if !ok {
		log.Fatalf("not found field \"messages\"")
		return rawBody
	}
	for _, msg := range messages {
		unmarshalMessage, ok := msg.(map[string]interface{})
		if !ok {
			log.Fatalf("unexpected message: error=%v", err)
			return rawBody
		}
		if isNeedModifyMessageInRequest(unmarshalMessage, messageKeys) {
			unmarshalMessage[fieldNamePartialInMessage] = true
		}
	}
	modifiedBody, err := json.Marshal(&data)
	if err != nil {
		log.Fatalf("invalid json data: error=%v", err)
		return rawBody
	}
	return modifiedBody
}

func NewPartialMessage(in *schema.Message, isPartial bool) *schema.Message {
	if in.Extra == nil {
		in.Extra = make(map[string]any)
	}
	in.Extra[fieldNamePartialInExtraMsg] = isPartial
	return in
}

const typ = "Qwen"

func (cm *ChatModel) GetType() string {
	return typ
}

func (cm *ChatModel) IsCallbacksEnabled() bool {
	return cm.cli.IsCallbacksEnabled()
}

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

package openrouter

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino-ext/libs/acl/openai"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	jsoniter "github.com/json-iterator/go"
	"github.com/tidwall/sjson"
)

const (
	defaultBaseURL = "https://openrouter.ai/api/v1"
)

type Config struct {
	APIKey string
	// Timeout specifies the maximum duration to wait for API responses
	// If HTTPClient is set, Timeout will not be used.
	// Optional. Default: no timeout
	Timeout time.Duration `json:"timeout"`

	// HTTPClient specifies the client to send HTTP requests.
	// If HTTPClient is set, Timeout will not be used.
	// Optional. Default &http.Client{Timeout: Timeout}
	HTTPClient *http.Client `json:"http_client"`

	// BaseURL specifies the OpenRouter endpoint URL
	// Required. Example: https://openrouter.ai/api/v1
	BaseURL string `json:"base_url"`

	// Model specifies the ID of the model to use.
	// Optional.
	Model string `json:"model,omitempty"`

	// Models parameter lets you automatically try other models if the primary model’s providers are down,
	// rate-limited, or refuse to reply due to content moderation.
	// Optional.
	Models []string `json:"models,omitempty"`

	// MaxCompletionTokens specifies an upper bound for the number of tokens that can be generated for a completion, including visible output tokens and reasoning tokens.
	MaxCompletionTokens *int `json:"max_completion_tokens,omitempty"`

	// MaxTokens limits the maximum number of tokens that can be generated in the chat completion
	MaxTokens *int `json:"max_tokens,omitempty"`

	// Seed enables deterministic sampling for consistent outputs
	// Optional. Set for reproducible results
	Seed *int `json:"seed,omitempty"`

	// Stop sequences where the API will stop generating further tokens
	// Optional. Example: []string{"\n", "User:"}
	Stop []string `json:"stop,omitempty"`

	// TopP controls diversity via nucleus sampling
	// Generally recommend altering this or Temperature but not both.
	// Range: 0.0 to 1.0. Lower values make output more focused
	// Optional. Default: 1.0
	TopP *float32 `json:"top_p,omitempty"`

	// Temperature specifies what sampling temperature to use
	// Generally recommend altering this or TopP but not both.
	// Range: 0.0 to 2.0. Higher values make output more random
	// Optional. Default: 1.0
	Temperature *float32 `json:"temperature,omitempty"`

	// ResponseFormat specifies the format of the model's response
	// Optional. Use for structured outputs
	ResponseFormat *ChatCompletionResponseFormat `json:"response_format,omitempty"`

	// PresencePenalty prevents repetition by penalizing tokens based on presence
	// Range: -2.0 to 2.0. Positive values increase likelihood of new topics
	// Optional. Default: 0
	PresencePenalty *float32 `json:"presence_penalty,omitempty"`

	// FrequencyPenalty prevents repetition by penalizing tokens based on frequency
	// Range: -2.0 to 2.0. Positive values decrease likelihood of repetition
	// Optional. Default: 0
	FrequencyPenalty *float32 `json:"frequency_penalty,omitempty"`

	// LogitBias modifies likelihood of specific tokens appearing in completion
	// Optional. Map token IDs to bias values from -100 to 100
	LogitBias map[string]int `json:"logit_bias,omitempty"`

	// LogProbs specifies whether to return log probabilities of the output tokens.
	LogProbs bool `json:"log_probs"`

	// TopLogProbs is an integer between 0 and 20 specifying the number of most likely tokens to return at each
	// token position, each with an associated log probability.
	// logprobs must be set to true if this parameter is used.
	TopLogProbs int `json:"top_logprobs"`

	// Reasoning supports advanced reasoning capabilities, allowing models to show their internal reasoning process with configurable effort、summary fields levels
	Reasoning *Reasoning `json:"reasoning,omitempty"`

	// User unique identifier representing end-user
	// Optional. Helps OpenAI monitor and detect abuse
	User *string `json:"user,omitempty"`

	// Metadata Set of 16 key-value pairs that can be attached to an object.
	// This can be useful for storing additional information about the object in a structured format, and querying for objects via API or the dashboard.
	// Keys are strings with a maximum length of 64 characters. Values are strings with a maximum length of 512 characters.
	Metadata map[string]string `json:"metadata,omitempty"`

	// ExtraFields will override any existing fields with the same key.
	// Optional. Useful for experimental features not yet officially supported.
	ExtraFields map[string]any `json:"extra_fields,omitempty"`
}

type ChatModel struct {
	cli            *openai.Client
	models         []string
	reasoning      *Reasoning
	responseFormat *ChatCompletionResponseFormat
	metadata       map[string]string
}

func (cm *ChatModel) Generate(ctx context.Context, in []*schema.Message, opts ...model.Option) (outMsg *schema.Message, err error) {
	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)
	opt := cm.buildOptions(ctx, false, opts...)

	out, err := cm.cli.Generate(ctx, in, opt...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (cm *ChatModel) Stream(ctx context.Context, in []*schema.Message, opts ...model.Option) (outStream *schema.StreamReader[*schema.Message], err error) {
	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)
	opts = cm.buildOptions(ctx, true, opts...)
	out, err := cm.cli.Stream(ctx, in, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func NewChatModel(ctx context.Context, config *Config) (*ChatModel, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	var httpClient *http.Client

	if config.HTTPClient != nil {
		httpClient = config.HTTPClient
	} else {
		httpClient = &http.Client{Timeout: config.Timeout}
	}

	if config.BaseURL == "" {
		config.BaseURL = defaultBaseURL
	}

	nConf := &openai.Config{
		BaseURL:             config.BaseURL,
		APIKey:              config.APIKey,
		HTTPClient:          httpClient,
		MaxTokens:           config.MaxTokens,
		MaxCompletionTokens: config.MaxCompletionTokens,
		Temperature:         config.Temperature,
		TopP:                config.TopP,
		Stop:                config.Stop,
		PresencePenalty:     config.PresencePenalty,
		Seed:                config.Seed,
		FrequencyPenalty:    config.FrequencyPenalty,
		LogitBias:           config.LogitBias,
		LogProbs:            config.LogProbs,
		TopLogProbs:         config.TopLogProbs,
		User:                config.User,
		ExtraFields:         config.ExtraFields,
		Model:               config.Model,
	}

	cli, err := openai.NewClient(ctx, nConf)
	if err != nil {
		return nil, err
	}

	chatModel := &ChatModel{
		cli: cli,
	}
	chatModel.models = config.Models
	chatModel.reasoning = config.Reasoning
	chatModel.metadata = config.Metadata
	chatModel.responseFormat = config.ResponseFormat

	return chatModel, nil
}

func (cm *ChatModel) buildRequestModifier(option *openrouterOption) openai.RequestPayloadModifier {
	return func(ctx context.Context, in []*schema.Message, rawBody []byte) ([]byte, error) {
		models := option.models
		reasoning := option.reasoning
		metadata := option.metadata
		var (
			modifiedRawBody []byte
			err             error
		)

		modifiedRawBody = rawBody
		if len(models) > 0 {
			modifiedRawBody, err = sjson.SetBytes(modifiedRawBody, "models", models)
			if err != nil {
				return nil, err
			}
		}
		if reasoning != nil {
			modifiedRawBody, err = sjson.SetBytes(modifiedRawBody, "reasoning", reasoning)
			if err != nil {
				return nil, err
			}
		}

		if cm.metadata != nil {
			modifiedRawBody, err = sjson.SetBytes(modifiedRawBody, "metadata", metadata)
			if err != nil {
				return nil, err
			}
		}

		if cm.responseFormat != nil {
			modifiedRawBody, err = sjson.SetBytes(modifiedRawBody, "response_format", cm.responseFormat)
			if err != nil {
				return nil, err
			}
		}

		for index, msg := range in {
			if details, ok := getReasoningDetails(msg); ok {
				modifiedRawBody, err = sjson.SetBytes(modifiedRawBody, fmt.Sprintf("messages.%d.reasoning_details", index), details)
				if err != nil {
					return nil, err
				}
			}
		}

		return modifiedRawBody, err
	}
}

func (cm *ChatModel) buildResponseMessageModifier() openai.ResponseMessageModifier {
	return func(ctx context.Context, msg *schema.Message, rawBody []byte) (*schema.Message, error) {
		choices := make([]*responseChoice, 0)
		if choicesString := jsoniter.Get(rawBody, "choices").ToString(); choicesString != "" {
			err := sonic.UnmarshalString(choicesString, &choices)
			if err != nil {
				return nil, err
			}
			var firstChoice *responseChoice
			for _, choice := range choices {
				if choice.Index == 0 {
					firstChoice = choice
					break
				}
			}
			if firstChoice == nil {
				return msg, nil
			}

			if firstChoice.Message != nil {
				msg.ReasoningContent = firstChoice.Message.Reasoning
				if len(firstChoice.Message.ReasoningDetails) > 0 {
					setReasoningDetails(msg, firstChoice.Message.ReasoningDetails)
				}
			}

		}
		return msg, nil
	}
}

func (cm *ChatModel) buildResponseChunkMessageModifier() openai.ResponseChunkMessageModifier {
	return func(ctx context.Context, msg *schema.Message, rawBody []byte, end bool) (*schema.Message, error) {
		const reasonError = "error"

		if msg.ResponseMeta != nil && msg.ResponseMeta.FinishReason == reasonError {
			tError := jsoniter.Get(rawBody, reasonError)
			if len(tError.ToString()) > 0 {
				err := setStreamTerminatedError(msg, tError.ToString())
				if err != nil {
					return nil, err
				}
			}
			return msg, nil
		}

		choices := make([]*responseChoice, 0)
		if choicesString := jsoniter.Get(rawBody, "choices").ToString(); choicesString != "" {
			err := sonic.UnmarshalString(choicesString, &choices)
			if err != nil {
				return nil, err
			}
			var firstChoice *responseChoice
			for _, choice := range choices {
				if choice.Index == 0 {
					firstChoice = choice
					break
				}
			}
			if firstChoice == nil {
				return msg, nil
			}

			if firstChoice.Delta != nil {
				msg.ReasoningContent = firstChoice.Delta.Reasoning
				if len(firstChoice.Delta.ReasoningDetails) > 0 {
					setReasoningDetails(msg, firstChoice.Delta.ReasoningDetails)
				}
			}

		}
		return msg, nil

	}
}

func (cm *ChatModel) buildOptions(_ context.Context, isStream bool, opts ...model.Option) []model.Option {
	specificOption := model.GetImplSpecificOptions(&openrouterOption{
		models:    cm.models,
		reasoning: cm.reasoning,
		metadata:  cm.metadata,
	}, opts...)

	modelOption := model.GetImplSpecificOptions(&model.Options{}, opts...)

	options := make([]model.Option, 0, len(opts))

	if modelOption.Model != nil {
		options = append(options, model.WithModel(*modelOption.Model))
	}

	if modelOption.MaxTokens != nil {
		options = append(options, model.WithMaxTokens(*modelOption.MaxTokens))
	}

	if modelOption.TopP != nil {
		options = append(options, model.WithTopP(*modelOption.TopP))
	}

	if len(modelOption.Stop) > 0 {
		options = append(options, model.WithStop(modelOption.Stop))
	}

	if len(modelOption.Tools) > 0 {
		options = append(options, model.WithTools(modelOption.Tools))
	}

	if modelOption.ToolChoice != nil {
		options = append(options, model.WithToolChoice(*modelOption.ToolChoice))
	}

	options = append(options, openai.WithRequestPayloadModifier(cm.buildRequestModifier(specificOption)))
	if !isStream {
		options = append(options, openai.WithResponseMessageModifier(cm.buildResponseMessageModifier()))
	} else {
		options = append(options, openai.WithResponseChunkMessageModifier(cm.buildResponseChunkMessageModifier()))
	}

	return options

}

const typ = "OpenRouter"

func (cm *ChatModel) GetType() string {
	return typ
}

func (cm *ChatModel) IsCallbacksEnabled() bool {
	return cm.cli.IsCallbacksEnabled()
}

func (cm *ChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	cli, err := cm.cli.WithToolsForClient(tools)
	if err != nil {
		return nil, err
	}
	return &ChatModel{cli: cli}, nil
}

func (cm *ChatModel) BindTools(tools []*schema.ToolInfo) error {
	return cm.cli.BindTools(tools)
}

func (cm *ChatModel) BindForcedTools(tools []*schema.ToolInfo) error {
	return cm.cli.BindForcedTools(tools)
}

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

package glm

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/cloudwego/eino-ext/libs/acl/openai"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

var _ model.ToolCallingChatModel = (*ChatModel)(nil)

// defaultBaseURL is the OpenAI-compatible endpoint of Zhipu BigModel (GLM).
// Ref: https://docs.bigmodel.cn/cn/guide/develop/openai/introduction
const defaultBaseURL = "https://open.bigmodel.cn/api/paas/v4"

// ChatModelConfig configures the GLM ChatModel.
//
// GLM (Zhipu AI / BigModel) exposes an OpenAI-compatible chat completion API,
// so most fields map directly to the OpenAI chat completion parameters.
// Ref: https://docs.bigmodel.cn/cn/guide/develop/openai/introduction
type ChatModelConfig struct {
	// APIKey is your BigModel API key.
	// Required.
	APIKey string `json:"api_key"`

	// Timeout specifies the maximum duration to wait for API responses.
	// If HTTPClient is set, Timeout will not be used.
	// Optional. Default: no timeout
	Timeout time.Duration `json:"timeout"`

	// HTTPClient specifies the client to send HTTP requests.
	// If HTTPClient is set, Timeout will not be used.
	// Optional. Default: &http.Client{Timeout: Timeout}
	HTTPClient *http.Client `json:"http_client"`

	// BaseURL specifies the GLM endpoint URL.
	// Optional. Default: https://open.bigmodel.cn/api/paas/v4
	BaseURL string `json:"base_url"`

	// Model specifies the ID of the model to use, e.g. "glm-4.6", "glm-4.5-flash", "glm-4-flash-250414".
	// Required.
	Model string `json:"model"`

	// MaxTokens limits the maximum number of tokens that can be generated in the chat completion.
	// Optional. Default: model's maximum
	MaxTokens *int `json:"max_tokens,omitempty"`

	// Temperature specifies what sampling temperature to use.
	// Generally recommend altering this or TopP but not both.
	// Range: (0.0, 1.0). Higher values make output more random.
	// Optional.
	Temperature *float32 `json:"temperature,omitempty"`

	// TopP controls diversity via nucleus sampling.
	// Generally recommend altering this or Temperature but not both.
	// Range: (0.0, 1.0). Lower values make output more focused.
	// Optional.
	TopP *float32 `json:"top_p,omitempty"`

	// Stop sequences where the API will stop generating further tokens.
	// Optional. Example: []string{"\n", "User:"}
	Stop []string `json:"stop,omitempty"`

	// PresencePenalty prevents repetition by penalizing tokens based on presence.
	// Range: [-2.0, 2.0]. Positive values increase likelihood of new topics.
	// Optional. Default: 0
	PresencePenalty *float32 `json:"presence_penalty,omitempty"`

	// ResponseFormat specifies the format of the model's response.
	// Optional. Use for structured outputs.
	ResponseFormat *openai.ChatCompletionResponseFormat `json:"response_format,omitempty"`

	// FrequencyPenalty prevents repetition by penalizing tokens based on frequency.
	// Range: [-2.0, 2.0]. Positive values decrease likelihood of repetition.
	// Optional. Default: 0
	FrequencyPenalty *float32 `json:"frequency_penalty,omitempty"`

	// User is a unique identifier representing your end-user.
	// Optional. Helps the provider monitor and detect abuse.
	User *string `json:"user,omitempty"`

	// EnableThinking toggles GLM's deep-thinking (reasoning) mode on reasoning-capable
	// models such as glm-4.6 and glm-4.5-flash. When set, a top-level
	// `thinking: {"type": "enabled"|"disabled"}` field is attached to the request and
	// the reasoning trace is returned via schema.Message.ReasoningContent.
	// Ref: https://docs.bigmodel.cn/cn/guide/develop/http/introduction
	// Optional. Default: base on the model.
	EnableThinking *bool `json:"enable_thinking,omitempty"`
}

type ChatModel struct {
	cli *openai.Client

	extraOptions *options
}

// NewChatModel creates a GLM ChatModel backed by the OpenAI-compatible transport.
func NewChatModel(ctx context.Context, config *ChatModelConfig) (*ChatModel, error) {
	if config == nil {
		return nil, fmt.Errorf("[NewChatModel] config not provided")
	}
	if len(config.Model) == 0 {
		return nil, fmt.Errorf("[NewChatModel] model not provided")
	}

	var httpClient *http.Client
	if config.HTTPClient != nil {
		httpClient = config.HTTPClient
	} else {
		httpClient = &http.Client{Timeout: config.Timeout}
	}

	baseURL := config.BaseURL
	if len(baseURL) == 0 {
		baseURL = defaultBaseURL
	}

	cli, err := openai.NewClient(ctx, &openai.Config{
		BaseURL:          baseURL,
		APIKey:           config.APIKey,
		HTTPClient:       httpClient,
		Model:            config.Model,
		MaxTokens:        config.MaxTokens,
		Temperature:      config.Temperature,
		TopP:             config.TopP,
		Stop:             config.Stop,
		PresencePenalty:  config.PresencePenalty,
		ResponseFormat:   config.ResponseFormat,
		FrequencyPenalty: config.FrequencyPenalty,
		User:             config.User,
	})
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

func validateToolOptions(opts ...model.Option) error {
	modelOptions := model.GetCommonOptions(&model.Options{}, opts...)
	if modelOptions.ToolChoice != nil {
		if *modelOptions.ToolChoice == schema.ToolChoiceAllowed && len(modelOptions.AllowedToolNames) > 0 {
			return fmt.Errorf("tool_choice 'allowed' is not supported when allowed tool names are present")
		}
		if *modelOptions.ToolChoice == schema.ToolChoiceForced && len(modelOptions.AllowedToolNames) > 1 {
			return fmt.Errorf("only one allowed tool name can be configured for tool_choice 'forced'")
		}
	}
	return nil
}

func (cm *ChatModel) Generate(ctx context.Context, in []*schema.Message, opts ...model.Option) (
	outMsg *schema.Message, err error) {
	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)
	opts = cm.parseCustomOptions(opts...)
	if err = validateToolOptions(opts...); err != nil {
		return nil, err
	}

	return cm.cli.Generate(ctx, in, opts...)
}

func (cm *ChatModel) Stream(ctx context.Context, in []*schema.Message, opts ...model.Option) (
	outStream *schema.StreamReader[*schema.Message], err error) {
	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)
	opts = cm.parseCustomOptions(opts...)
	if err = validateToolOptions(opts...); err != nil {
		return nil, err
	}

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
	glmOpts := model.GetImplSpecificOptions(&options{
		EnableThinking: cm.extraOptions.EnableThinking,
	}, opts...)

	extraFields := make(map[string]any)
	if glmOpts.EnableThinking != nil {
		// GLM controls reasoning via a top-level `thinking` object.
		// Ref: https://docs.bigmodel.cn/cn/guide/develop/http/introduction
		thinkingType := "disabled"
		if *glmOpts.EnableThinking {
			thinkingType = "enabled"
		}
		extraFields["thinking"] = map[string]any{"type": thinkingType}
	}
	if len(glmOpts.extraFields) > 0 {
		for k, v := range glmOpts.extraFields {
			extraFields[k] = v
		}
	}
	if len(extraFields) > 0 {
		opts = append(opts, openai.WithExtraFields(extraFields))
	}
	return opts
}

const typ = "GLM"

func (cm *ChatModel) GetType() string {
	return typ
}

func (cm *ChatModel) IsCallbacksEnabled() bool {
	return cm.cli.IsCallbacksEnabled()
}

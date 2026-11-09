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

// Package ark implements Ark Chat Completions and Responses using ark-runtime-go.
package ark

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	fmodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/chat"
)

var _ fmodel.ToolCallingChatModel = (*ChatModel)(nil)

// ErrEmptyResponse indicates that the API returned no completion choices.
var ErrEmptyResponse = errors.New("empty response received from model")

// ChatModelConfig configures the Ark service connection, generation parameters,
// and function calling.
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
	RequestPayloadModifier       RequestPayloadModifier       `json:"-"`
	ResponseMessageModifier      ResponseMessageModifier      `json:"-"`
	ResponseChunkMessageModifier ResponseChunkMessageModifier `json:"-"`
}

type ChatModel struct {
	client     *arkruntime.Client
	config     ChatModelConfig
	rawTools   []*schema.ToolInfo
	toolChoice *schema.ToolChoice
}

func NewChatModel(_ context.Context, config *ChatModelConfig) (*ChatModel, error) {
	if config == nil {
		config = &ChatModelConfig{}
	}
	client := newArkClient(clientConfig{
		timeout: config.Timeout, httpClient: config.HTTPClient, retryTimes: config.RetryTimes,
		baseURL: config.BaseURL, region: config.Region,
		apiKey: config.APIKey, accessKey: config.AccessKey, secretKey: config.SecretKey,
	})
	return &ChatModel{client: client, config: *config}, nil
}

func (cm *ChatModel) WithTools(tools []*schema.ToolInfo) (fmodel.ToolCallingChatModel, error) {
	if len(tools) == 0 {
		return nil, errors.New("no tools to bind")
	}
	_, err := toTools(tools, cm.config.ToolStrict)
	if err != nil {
		return nil, fmt.Errorf("convert tools: %w", err)
	}
	next := *cm
	next.rawTools = append([]*schema.ToolInfo(nil), tools...)
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

func (cm *ChatModel) getOptions(opts ...fmodel.Option) (*fmodel.Options, *arkOptions) {
	common := fmodel.GetCommonOptions(&fmodel.Options{
		Temperature: cm.config.Temperature,
		MaxTokens:   cm.config.MaxTokens,
		Model:       &cm.config.Model,
		TopP:        cm.config.TopP,
		Stop:        cm.config.Stop,
		ToolChoice:  cm.toolChoice,
	}, opts...)
	ark := fmodel.GetImplSpecificOptions(&arkOptions{
		customHeaders:                cm.config.CustomHeader,
		thinking:                     cm.config.Thinking,
		reasoningEffort:              cm.config.ReasoningEffort,
		maxCompletionTokens:          cm.config.MaxCompletionTokens,
		parallelToolCalls:            cm.config.ParallelToolCalls,
		requestPayloadModifier:       cm.config.RequestPayloadModifier,
		responseMessageModifier:      cm.config.ResponseMessageModifier,
		responseChunkMessageModifier: cm.config.ResponseChunkMessageModifier,
	}, opts...)
	return common, ark
}

// WithThinking configures thinking for Chat Completions.
func WithThinking(thinking *chat.Thinking) fmodel.Option {
	return fmodel.WrapImplSpecificOptFn(func(o *arkOptions) { o.thinking = thinking })
}

// WithReasoningEffort configures reasoning effort for Chat Completions.
func WithReasoningEffort(effort chat.ReasoningEffort) fmodel.Option {
	return fmodel.WrapImplSpecificOptFn(func(o *arkOptions) { o.reasoningEffort = &effort })
}

func WithMaxCompletionTokens(n int) fmodel.Option {
	return fmodel.WrapImplSpecificOptFn(func(o *arkOptions) { o.maxCompletionTokens = &n })
}

func (cm *ChatModel) Generate(ctx context.Context, in []*schema.Message, opts ...fmodel.Option) (
	outMsg *schema.Message, err error) {

	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)

	options, specOptions := cm.getOptions(opts...)

	request, err := cm.genRequest(in, options, specOptions)
	if err != nil {
		return nil, err
	}

	req := request.payload
	req.Stream = chat.NewOptBool(false)
	ctx = cm.start(ctx, in, options, request)

	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	wireReq, extraBody, err := prepareRequestPayload(ctx, in, req, specOptions.requestPayloadModifier)
	if err != nil {
		return nil, err
	}
	var capture *responseCapture
	if specOptions.responseMessageModifier != nil {
		capture = &responseCapture{}
	}
	ctx = context.WithValue(ctx, responseCaptureKey{}, capture)
	resp, err := cm.client.CreateChatCompletion(ctx, wireReq, arkruntime.WithCustomHeaders(specOptions.customHeaders), arkruntime.WithExtraBody(extraBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create chat completion: %w", err)
	}

	outMsg, err = resolveChatResponse(&resp.ChatCompletionResponse)
	if err != nil {
		return nil, err
	}
	if specOptions.responseMessageModifier != nil {
		outMsg, err = specOptions.responseMessageModifier(ctx, outMsg, capture.rawBody)
		if err != nil {
			return nil, fmt.Errorf("modify response message: %w", err)
		}
		if outMsg == nil {
			return nil, fmt.Errorf("response message modifier returned nil")
		}
	}

	callbacks.OnEnd(ctx, &fmodel.CallbackOutput{
		Message:    outMsg,
		Config:     callbackConfig(wireReq),
		TokenUsage: toModelCallbackUsage(outMsg.ResponseMeta),
		Extra: map[string]any{
			"thinking":   callbackThinking(wireReq),
			"model_name": resp.Model,
		},
	})

	return outMsg, nil
}

func (cm *ChatModel) Stream(ctx context.Context, in []*schema.Message, opts ...fmodel.Option) (
	outStream *schema.StreamReader[*schema.Message], err error) {

	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)

	options, arkOpts := cm.getOptions(opts...)

	request, err := cm.genRequest(in, options, arkOpts)
	if err != nil {
		return nil, err
	}

	req := request.payload
	req.Stream = chat.NewOptBool(true)
	streamOptions := req.StreamOptions.Or(chat.ChatCompletionStreamOptions{})
	streamOptions.IncludeUsage = chat.NewOptBool(true)
	req.StreamOptions = chat.NewOptChatCompletionStreamOptions(streamOptions)

	ctx = cm.start(ctx, in, options, request)

	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	wireReq, extraBody, err := prepareRequestPayload(ctx, in, req, arkOpts.requestPayloadModifier)
	if err != nil {
		return nil, err
	}
	reqConf := callbackConfig(wireReq)
	// A stream may be started from a response modifier's context. Do not let
	// that parent's response capture buffer consume this SSE response body.
	ctx = context.WithValue(ctx, responseCaptureKey{}, (*responseCapture)(nil))
	stream, err := cm.client.CreateChatCompletionStream(ctx, wireReq, arkruntime.WithCustomHeaders(arkOpts.customHeaders), arkruntime.WithExtraBody(extraBody))
	if err != nil {
		return nil, err
	}

	decoder := &streamPayloadDecoder{capture: arkOpts.responseChunkMessageModifier != nil}
	stream.Unmarshaler = decoder
	sr, sw := schema.Pipe[*fmodel.CallbackOutput](1)
	go func(ctx context.Context) {
		defer func() {
			panicErr := recover()
			if panicErr != nil {
				_ = sw.Send(nil, newPanicErr(panicErr, debug.Stack()))
			}

			sw.Close()
			_ = stream.Close()

		}()

		seenChoice := false
		for {
			resp, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				// The SDK also returns EOF for a body closed before [DONE]. Such
				// a response is incomplete and must not run successful finalization.
				if !stream.IsFinished {
					_ = sw.Send(nil, io.ErrUnexpectedEOF)
					return
				}
				if !seenChoice {
					_ = sw.Send(nil, ErrEmptyResponse)
					return
				}
				if modifier := arkOpts.responseChunkMessageModifier; modifier != nil {
					last, e := modifier(ctx, nil, nil, true)
					if e != nil {
						_ = sw.Send(nil, fmt.Errorf("modify final response chunk message: %w", e))
					} else if last != nil {
						_ = sw.Send(&fmodel.CallbackOutput{Message: last, Config: reqConf, TokenUsage: toModelCallbackUsage(last.ResponseMeta)}, nil)
					}
				}
				return
			}

			if err != nil {
				_ = sw.Send(nil, err)
				return
			}

			if resp != nil {
				for _, choice := range resp.Choices {
					if choice.Index == 0 {
						seenChoice = true
						break
					}
				}
			}
			msg, msgFound, e := resolveStreamResponse(resp)
			if e != nil {
				_ = sw.Send(nil, e)
				return
			}

			if modifier := arkOpts.responseChunkMessageModifier; modifier != nil {
				msg, e = modifier(ctx, msg, decoder.rawBody, false)
				if e != nil {
					_ = sw.Send(nil, fmt.Errorf("modify response chunk message: %w", e))
					return
				}
				msgFound = msg != nil
			}
			if !msgFound {
				continue
			}

			closed := sw.Send(&fmodel.CallbackOutput{
				Message:    msg,
				Config:     reqConf,
				TokenUsage: toModelCallbackUsage(msg.ResponseMeta),
				Extra: map[string]any{
					"thinking":   callbackThinking(wireReq),
					"model_name": resp.Model,
				},
			}, nil)
			if closed {
				return
			}
		}
	}(ctx)

	ctx, nsr := callbacks.OnEndWithStreamOutput(ctx, schema.StreamReaderWithConvert(sr,
		func(src *fmodel.CallbackOutput) (callbacks.CallbackOutput, error) {
			return src, nil
		}))

	outStream = schema.StreamReaderWithConvert(nsr,
		func(src callbacks.CallbackOutput) (*schema.Message, error) {
			s := src.(*fmodel.CallbackOutput)
			if s.Message == nil {
				return nil, schema.ErrNoValue
			}

			return s.Message, nil
		},
	)

	return outStream, nil
}

func (cm *ChatModel) start(ctx context.Context, in []*schema.Message, common *fmodel.Options, request *chatRequest) context.Context {
	return callbacks.OnStart(ctx, &fmodel.CallbackInput{
		Messages: in, Tools: request.callbackTools, ToolChoice: common.ToolChoice,
		Config: callbackConfig(request.payload), Extra: map[string]any{"thinking": callbackThinking(request.payload)},
	})
}

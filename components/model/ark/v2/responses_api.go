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
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/responses"
)

var _ model.ToolCallingChatModel = (*ResponsesAPIChatModel)(nil)

// ResponsesAPIConfig configures an Ark Responses API model.
// SDK-specific formats and built-in tools use ark-runtime-go Responses types.
type ResponsesAPIConfig struct {
	Timeout             *time.Duration
	HTTPClient          *http.Client
	RetryTimes          *int
	BaseURL             string
	Region              string
	APIKey              string
	AccessKey           string
	SecretKey           string
	Model               string
	MaxOutputTokens     *int
	Temperature         *float32
	TopP                *float32
	CustomHeader        map[string]string
	ResponseFormat      *responses.TextFormat
	Thinking            *responses.Thinking
	ServiceTier         *string
	ReasoningEffort     *responses.ReasoningEffort
	SessionCache        *SessionCacheConfig
	EnableToolWebSearch *responses.WebSearchTool
	MaxToolCalls        *int64
}

type ResponsesAPIChatModel struct {
	client     *arkruntime.Client
	config     ResponsesAPIConfig
	rawTools   []*schema.ToolInfo
	toolChoice *schema.ToolChoice
}

func NewResponsesAPIChatModel(_ context.Context, config *ResponsesAPIConfig) (*ResponsesAPIChatModel, error) {
	if config == nil {
		return nil, fmt.Errorf("config must not be nil")
	}
	if config.APIKey == "" && (config.AccessKey == "" || config.SecretKey == "") {
		return nil, fmt.Errorf("set APIKey or both AccessKey and SecretKey")
	}
	client := newArkClient(clientConfig{
		timeout: config.Timeout, httpClient: config.HTTPClient, retryTimes: config.RetryTimes,
		baseURL: config.BaseURL, region: config.Region,
		apiKey: config.APIKey, accessKey: config.AccessKey, secretKey: config.SecretKey,
	})
	return &ResponsesAPIChatModel{client: client, config: *config}, nil
}

func (cm *ResponsesAPIChatModel) GetType() string          { return "ResponsesAPI" }
func (cm *ResponsesAPIChatModel) IsCallbacksEnabled() bool { return true }

func (cm *ResponsesAPIChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	if len(tools) == 0 {
		return nil, fmt.Errorf("no tools to bind")
	}
	if _, err := toResponsesTools(tools, nil); err != nil {
		return nil, err
	}
	next := *cm
	next.rawTools = append([]*schema.ToolInfo(nil), tools...)
	return &next, nil
}

// BindTools mutates the model. Prefer WithTools for concurrently shared models.
func (cm *ResponsesAPIChatModel) BindTools(tools []*schema.ToolInfo) error {
	next, err := cm.WithTools(tools)
	if err != nil {
		return err
	}
	*cm = *next.(*ResponsesAPIChatModel)
	return nil
}
func (cm *ResponsesAPIChatModel) BindForcedTools(tools []*schema.ToolInfo) error {
	if err := cm.BindTools(tools); err != nil {
		return err
	}
	choice := schema.ToolChoiceForced
	cm.toolChoice = &choice
	return nil
}

func (cm *ResponsesAPIChatModel) start(ctx context.Context, in []*schema.Message, common *model.Options, opts *arkOptions, request *responsesRequest) context.Context {
	req := request.payload
	return callbacks.OnStart(ctx, &model.CallbackInput{Messages: in, Tools: request.callbackTools, ToolChoice: common.ToolChoice,
		Config: responsesCallbackConfig(req), Extra: map[string]any{"thinking": opts.responsesThinking, "ark-previous-response-id": req.PreviousResponseID.Or("")}})
}

func (cm *ResponsesAPIChatModel) Generate(ctx context.Context, in []*schema.Message, options ...model.Option) (out *schema.Message, err error) {
	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)
	common, opts := cm.getOptions(options...)
	request, err := cm.genRequest(in, common, opts)
	if err != nil {
		return nil, err
	}
	request.payload.Stream = responses.NewOptBool(false)
	ctx = cm.start(ctx, in, common, opts, request)
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()
	resp, err := cm.client.CreateResponses(ctx, request.payload, arkruntime.WithCustomHeaders(responsesHeaders(opts)))
	if err != nil {
		return nil, fmt.Errorf("create Responses: %w", err)
	}
	out, err = projectResponsesResponse(&resp.Response, request.payload)
	if err != nil {
		return nil, err
	}
	if requestID := resp.GetHeader().Get("X-Request-Id"); requestID != "" {
		out.Extra[keyResponsesRequestID] = metadataString(requestID)
	}
	callbacks.OnEnd(ctx, &model.CallbackOutput{Message: out, Config: responsesCallbackConfig(request.payload), TokenUsage: toModelCallbackUsage(out.ResponseMeta),
		Extra: map[string]any{"thinking": responsesCallbackThinking(request.payload), "model_name": resp.Model}})
	return out, nil
}

func (cm *ResponsesAPIChatModel) Stream(ctx context.Context, in []*schema.Message, options ...model.Option) (out *schema.StreamReader[*schema.Message], err error) {
	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)
	common, opts := cm.getOptions(options...)
	request, err := cm.genRequest(in, common, opts)
	if err != nil {
		return nil, err
	}
	request.payload.Stream = responses.NewOptBool(true)
	ctx = cm.start(ctx, in, common, opts, request)
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()
	stream, err := cm.client.CreateResponsesStream(ctx, request.payload, arkruntime.WithCustomHeaders(responsesHeaders(opts)))
	if err != nil {
		return nil, err
	}
	callbackConfig := responsesCallbackConfig(request.payload)
	thinking := responsesCallbackThinking(request.payload)
	sr, sw := schema.Pipe[*model.CallbackOutput](1)
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				_ = sw.Send(nil, newPanicErr(recovered, debug.Stack()))
			}
			sw.Close()
			_ = stream.Close()
		}()
		send := func(msg *schema.Message) bool {
			return sw.Send(&model.CallbackOutput{Message: msg, Config: callbackConfig, TokenUsage: toModelCallbackUsage(msg.ResponseMeta),
				Extra: map[string]any{"thinking": thinking}}, nil)
		}
		state := &responsesStreamState{}
		for {
			event, e := stream.Recv()
			// The SDK may return (nil, nil) on a clean HTTP EOF. A Responses
			// terminal event is required even when the wire includes [DONE].
			if errors.Is(e, io.EOF) || (e == nil && event == nil) {
				_ = sw.Send(nil, io.ErrUnexpectedEOF)
				return
			}
			if e != nil {
				_ = sw.Send(nil, e)
				return
			}
			msg, terminal, e := state.resolve(event, request.payload)
			if e != nil {
				_ = sw.Send(nil, e)
				return
			}
			if terminal && msg != nil {
				if requestID := stream.Header().Get("X-Request-Id"); requestID != "" {
					msg.Extra[keyResponsesRequestID] = metadataString(requestID)
				}
			}
			if msg != nil && send(msg) {
				return
			}
			if terminal {
				return
			}
		}
	}()
	_, nsr := callbacks.OnEndWithStreamOutput(ctx, schema.StreamReaderWithConvert(sr, func(value *model.CallbackOutput) (callbacks.CallbackOutput, error) { return value, nil }))
	return schema.StreamReaderWithConvert(nsr, func(value callbacks.CallbackOutput) (*schema.Message, error) {
		return value.(*model.CallbackOutput).Message, nil
	}), nil
}

// CreatePrefixCache stores a reusable prefix. Availability depends on the model
// and account; unsupported combinations return the service's error.
func (cm *ResponsesAPIChatModel) CreatePrefixCache(ctx context.Context, prefix []*schema.Message, ttl int, options ...model.Option) (*CacheInfo, error) {
	if len(prefix) == 0 {
		return nil, fmt.Errorf("prefix must not be empty")
	}
	if ttl < 0 {
		return nil, fmt.Errorf("prefix TTL must not be negative")
	}
	// Prefix creation ignores history reuse without modifying the model or the
	// parsed options. Only the derived options passed to this build omit cache.
	common, opts := cm.getOptions(options...)
	prefixOptions := *opts
	prefixOptions.cache = CacheOption{}
	request, err := cm.genRequest(prefix, common, &prefixOptions)
	if err != nil {
		return nil, err
	}
	req := request.payload
	req.Store = responses.NewOptBool(true)
	req.Stream = responses.NewOptBool(false)
	req.Caching = responses.NewOptResponseCaching(responses.ResponseCaching{Type: responses.NewOptCacheMode(responses.CacheModeEnabled), Prefix: responses.NewOptBool(true)})
	if ttl > 0 {
		req.ExpireAt = responses.NewOptInt64(time.Now().Unix() + int64(ttl))
	}
	resp, err := cm.client.CreateResponses(ctx, req, arkruntime.WithCustomHeaders(responsesHeaders(opts)))
	if err != nil {
		return nil, err
	}
	msg, err := projectResponsesMetadata(&resp.Response, req)
	if err != nil {
		return nil, err
	}
	result := &CacheInfo{ResponseID: resp.ID}
	if msg.ResponseMeta.Usage != nil {
		result.Usage = *msg.ResponseMeta.Usage
	}
	return result, nil
}

// WithCache replaces the Responses API cache configuration for one call.
// Omit this option to use the model's session-cache configuration.
func WithCache(cache CacheOption) model.Option {
	return model.WrapImplSpecificOptFn(func(o *arkOptions) { o.cache = cache })
}

// WithEnableToolWebSearch configures the Responses API's built-in search tool.
// A nil value disables it for this call.
func WithEnableToolWebSearch(tool *responses.WebSearchTool) model.Option {
	return model.WrapImplSpecificOptFn(func(o *arkOptions) { o.enableWebSearch = tool })
}

func WithMaxToolCalls(n int64) model.Option {
	return model.WrapImplSpecificOptFn(func(o *arkOptions) { o.maxToolCalls = &n })
}

// WithResponsesThinking configures thinking for the Responses API.
func WithResponsesThinking(thinking *responses.Thinking) model.Option {
	return model.WrapImplSpecificOptFn(func(o *arkOptions) { o.responsesThinking = thinking })
}

// WithResponsesReasoningEffort configures reasoning effort for the Responses API.
func WithResponsesReasoningEffort(effort responses.ReasoningEffort) model.Option {
	return model.WrapImplSpecificOptFn(func(o *arkOptions) { o.responsesReasoningEffort = &effort })
}

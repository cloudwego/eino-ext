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
	"github.com/volcengine/ark-runtime-go/arkruntime/model/chat"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/responses"
)

var _ model.ToolCallingChatModel = (*ResponsesAPIChatModel)(nil)

// ResponsesAPIConfig configures an Ark Responses API model. Shared call options
// (WithThinking, WithReasoningEffort, WithCustomHeader and modifiers) also apply.
// SDK-specific formats and built-in tools use ark-runtime-go Responses types.
type ResponsesAPIConfig struct {
	Timeout                        *time.Duration               `json:"timeout,omitempty"`
	HTTPClient                     *http.Client                 `json:"http_client,omitempty"`
	RetryTimes                     *int                         `json:"retry_times,omitempty"`
	BaseURL                        string                       `json:"base_url,omitempty"`
	Region                         string                       `json:"region,omitempty"`
	APIKey                         string                       `json:"api_key,omitempty"`
	AccessKey                      string                       `json:"access_key,omitempty"`
	SecretKey                      string                       `json:"secret_key,omitempty"`
	Model                          string                       `json:"model"`
	MaxOutputTokens                *int                         `json:"max_output_tokens,omitempty"`
	Temperature                    *float32                     `json:"temperature,omitempty"`
	TopP                           *float32                     `json:"top_p,omitempty"`
	CustomHeader                   map[string]string            `json:"custom_header,omitempty"`
	ResponseFormat                 *responses.TextFormat        `json:"response_format,omitempty"`
	Thinking                       *chat.Thinking               `json:"thinking,omitempty"`
	ServiceTier                    *string                      `json:"service_tier,omitempty"`
	ReasoningEffort                *chat.ReasoningEffort        `json:"reasoning_effort,omitempty"`
	ParallelToolCalls              *bool                        `json:"parallel_tool_calls,omitempty"`
	ToolStrict                     *bool                        `json:"tool_strict,omitempty"`
	SessionCache                   *SessionCacheConfig          `json:"session_cache,omitempty"`
	EnableToolWebSearch            *responses.WebSearchTool     `json:"enable_tool_web_search,omitempty"`
	MaxToolCalls                   *int64                       `json:"max_tool_calls,omitempty"`
	EnableReasoningContentPassback bool                         `json:"enable_reasoning_content_passback,omitempty"`
	RequestPayloadModifier         RequestPayloadModifier       `json:"-"`
	ResponseMessageModifier        ResponseMessageModifier      `json:"-"`
	ResponseChunkMessageModifier   ResponseChunkMessageModifier `json:"-"`
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
	if _, err := toTools(tools, cm.config.ToolStrict); err != nil {
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

func (cm *ResponsesAPIChatModel) start(ctx context.Context, in []*schema.Message, common *model.Options, opts *arkOptions, req *responses.ResponsesRequest) context.Context {
	tools := cm.rawTools
	if common.Tools != nil {
		tools = common.Tools
	}
	return callbacks.OnStart(ctx, &model.CallbackInput{Messages: in, Tools: tools, ToolChoice: common.ToolChoice,
		Config: responsesCallbackConfig(req), Extra: map[string]any{"thinking": opts.thinking, "ark-previous-response-id": req.PreviousResponseID.Or("")}})
}

func (cm *ResponsesAPIChatModel) Generate(ctx context.Context, in []*schema.Message, options ...model.Option) (out *schema.Message, err error) {
	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)
	common, opts := cm.getOptions(options...)
	req, extra, err := cm.genRequest(in, common, opts)
	if err != nil {
		return nil, err
	}
	req.Stream = responses.NewOptBool(false)
	ctx = cm.start(ctx, in, common, opts, req)
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()
	wire, extra, err := prepareResponsesPayload(ctx, in, req, extra, opts.requestPayloadModifier)
	if err != nil {
		return nil, err
	}
	var capture *responseCapture
	if opts.responseMessageModifier != nil {
		capture = &responseCapture{}
	}
	ctx = context.WithValue(ctx, responseCaptureKey{}, capture)
	resp, err := cm.client.CreateResponses(ctx, wire, arkruntime.WithCustomHeaders(responsesHeaders(opts)), arkruntime.WithExtraBody(extra))
	if err != nil {
		return nil, fmt.Errorf("create Responses: %w", err)
	}
	out, err = resolveResponsesResponse(&resp.Response, wire)
	if err != nil {
		return nil, err
	}
	if requestID := resp.GetHeader().Get("X-Request-Id"); requestID != "" {
		out.Extra[keyRequestID] = metadataString(requestID)
	}
	if opts.responseMessageModifier != nil {
		out, err = opts.responseMessageModifier(ctx, out, capture.rawBody)
		if err != nil {
			return nil, fmt.Errorf("modify response message: %w", err)
		}
		if out == nil {
			return nil, fmt.Errorf("response message modifier returned nil")
		}
	}
	callbacks.OnEnd(ctx, &model.CallbackOutput{Message: out, Config: responsesCallbackConfig(req), TokenUsage: toModelCallbackUsage(out.ResponseMeta),
		Extra: map[string]any{"thinking": opts.thinking, "model_name": resp.Model}})
	return out, nil
}

func (cm *ResponsesAPIChatModel) Stream(ctx context.Context, in []*schema.Message, options ...model.Option) (out *schema.StreamReader[*schema.Message], err error) {
	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)
	common, opts := cm.getOptions(options...)
	req, extra, err := cm.genRequest(in, common, opts)
	if err != nil {
		return nil, err
	}
	req.Stream = responses.NewOptBool(true)
	ctx = cm.start(ctx, in, common, opts, req)
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()
	wire, extra, err := prepareResponsesPayload(ctx, in, req, extra, opts.requestPayloadModifier)
	if err != nil {
		return nil, err
	}
	ctx = context.WithValue(ctx, responseCaptureKey{}, (*responseCapture)(nil))
	stream, err := cm.client.CreateResponsesStream(ctx, wire, arkruntime.WithCustomHeaders(responsesHeaders(opts)), arkruntime.WithExtraBody(extra))
	if err != nil {
		return nil, err
	}
	decoder := &responsesPayloadDecoder{}
	stream.Unmarshaler = decoder
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
			return sw.Send(&model.CallbackOutput{Message: msg, Config: responsesCallbackConfig(req), TokenUsage: toModelCallbackUsage(msg.ResponseMeta),
				Extra: map[string]any{"thinking": opts.thinking}}, nil)
		}
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
			msg, terminal, e := resolveResponsesStreamEvent(event, wire)
			if e != nil {
				_ = sw.Send(nil, e)
				return
			}
			if terminal && msg != nil {
				if requestID := stream.Header().Get("X-Request-Id"); requestID != "" {
					msg.Extra[keyRequestID] = metadataString(requestID)
				}
			}
			if modifier := opts.responseChunkMessageModifier; modifier != nil {
				msg, e = modifier(ctx, msg, decoder.raw, false)
				if e != nil {
					_ = sw.Send(nil, fmt.Errorf("modify response chunk message: %w", e))
					return
				}
			}
			if msg != nil && send(msg) {
				return
			}
			if !terminal {
				continue
			}
			if modifier := opts.responseChunkMessageModifier; modifier != nil {
				last, e := modifier(ctx, nil, nil, true)
				if e != nil {
					_ = sw.Send(nil, fmt.Errorf("modify final response chunk message: %w", e))
					return
				}
				if last != nil {
					_ = send(last)
				}
			}
			return
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
	// Build independently so session-history trimming cannot discard prefix input.
	next := *cm
	next.config.SessionCache = nil
	common, opts := next.getOptions(options...)
	opts.cache = nil
	req, extra, err := next.genRequest(prefix, common, opts)
	if err != nil {
		return nil, err
	}
	req.Store = responses.NewOptBool(true)
	req.Stream = responses.NewOptBool(false)
	req.Caching = responses.NewOptResponseCaching(responses.ResponseCaching{Type: responses.NewOptCacheMode(responses.CacheModeEnabled), Prefix: responses.NewOptBool(true)})
	if ttl > 0 {
		req.ExpireAt = responses.NewOptInt64(time.Now().Unix() + int64(ttl))
	}
	wire, extra, err := prepareResponsesPayload(ctx, prefix, req, extra, opts.requestPayloadModifier)
	if err != nil {
		return nil, err
	}
	resp, err := cm.client.CreateResponses(ctx, wire, arkruntime.WithCustomHeaders(responsesHeaders(opts)), arkruntime.WithExtraBody(extra))
	if err != nil {
		return nil, err
	}
	msg, err := responsesMetadata(&resp.Response, wire)
	if err != nil {
		return nil, err
	}
	result := &CacheInfo{ResponseID: resp.ID}
	if msg.ResponseMeta.Usage != nil {
		result.Usage = *msg.ResponseMeta.Usage
	}
	return result, nil
}

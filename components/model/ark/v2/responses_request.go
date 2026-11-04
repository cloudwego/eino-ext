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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/responses"
)

func (cm *ResponsesAPIChatModel) getOptions(opts ...model.Option) (*model.Options, *arkOptions) {
	return model.GetCommonOptions(&model.Options{Model: &cm.config.Model, MaxTokens: cm.config.MaxOutputTokens,
			Temperature: cm.config.Temperature, TopP: cm.config.TopP, ToolChoice: cm.toolChoice}, opts...),
		model.GetImplSpecificOptions(&arkOptions{customHeaders: cm.config.CustomHeader, responsesThinking: cm.config.Thinking,
			responsesReasoningEffort: cm.config.ReasoningEffort, parallelToolCalls: cm.config.ParallelToolCalls,
			cache: &CacheOption{SessionCache: cm.config.SessionCache}, enableWebSearch: cm.config.EnableToolWebSearch,
			maxToolCalls: cm.config.MaxToolCalls, enableReasoningContentPassback: cm.config.EnableReasoningContentPassback,
			requestPayloadModifier: cm.config.RequestPayloadModifier, responseMessageModifier: cm.config.ResponseMessageModifier,
			responseChunkMessageModifier: cm.config.ResponseChunkMessageModifier}, opts...)
}

func (cm *ResponsesAPIChatModel) genRequest(in []*schema.Message, common *model.Options, opts *arkOptions) (*responses.ResponsesRequest, map[string]interface{}, error) {
	if len(common.Stop) > 0 {
		return nil, nil, fmt.Errorf("Responses API does not support stop sequences")
	}
	req := &responses.ResponsesRequest{Model: dereferenceOrZero(common.Model), Store: responses.NewOptBool(false)}
	if common.MaxTokens != nil {
		if *common.MaxTokens <= 0 {
			return nil, nil, fmt.Errorf("max_output_tokens must be positive")
		}
		req.MaxOutputTokens = responses.NewOptInt64(int64(*common.MaxTokens))
	}
	if common.Temperature != nil {
		req.Temperature = responses.NewOptFloat64(float64FromFloat32(*common.Temperature))
	}
	if common.TopP != nil {
		req.TopP = responses.NewOptFloat64(float64FromFloat32(*common.TopP))
	}
	if opts.responsesThinking != nil {
		req.Thinking = responses.NewOptThinking(*opts.responsesThinking)
	}
	if opts.responsesReasoningEffort != nil {
		req.Reasoning = responses.NewOptReasoning(responses.Reasoning{Effort: responses.NewOptReasoningEffort(*opts.responsesReasoningEffort)})
	}
	if opts.parallelToolCalls != nil {
		req.ParallelToolCalls = responses.NewOptBool(*opts.parallelToolCalls)
	}
	if cm.config.ServiceTier != nil {
		req.ServiceTier = responses.NewOptServiceTier(responses.ServiceTier(*cm.config.ServiceTier))
	}
	if cm.config.ResponseFormat != nil {
		req.Text = responses.NewOptResponseTextConfig(responses.ResponseTextConfig{Format: *cm.config.ResponseFormat})
	}
	var err error
	in, err = populateResponsesCache(in, req, opts.cache)
	if err != nil {
		return nil, nil, err
	}
	items, err := toResponsesInput(in, opts.enableReasoningContentPassback && !req.PreviousResponseID.IsSet())
	if err != nil {
		return nil, nil, err
	}
	req.Input = responses.NewInputItemArrayResponsesInput(items.items)
	var extra map[string]interface{}
	if len(items.output) > 0 {
		extra = map[string]interface{}{"input": items}
	}
	tools := cm.rawTools
	if common.Tools != nil {
		tools = common.Tools
	}
	tools, err = selectToolInfos(tools, common.ToolChoice, common.AllowedToolNames)
	if err != nil {
		return nil, nil, err
	}
	var rawTools []json.RawMessage
	req.Tools, rawTools, err = toResponsesTools(tools, cm.config.ToolStrict)
	if err != nil {
		return nil, nil, err
	}
	if err = populateResponsesToolChoice(req, common.ToolChoice); err != nil {
		return nil, nil, err
	}
	if opts.enableWebSearch != nil {
		tool := *opts.enableWebSearch
		tool.Type = responses.WebSearchToolTypeWebSearch
		req.Tools = append(req.Tools, responses.Tool{OneOf: responses.NewWebSearchToolToolSum(tool)})
		if rawTools != nil {
			raw, e := sonic.Marshal(&tool)
			if e != nil {
				return nil, nil, fmt.Errorf("serialize Responses web search: %w", e)
			}
			rawTools = append(rawTools, raw)
		}
	}
	if opts.maxToolCalls != nil {
		req.MaxToolCalls = responses.NewOptInt64(*opts.maxToolCalls)
	}
	// Cached responses already contain the tool definitions. Ark rejects even
	// tools: [], so omit the SDK field and the strict extra-body override.
	if req.PreviousResponseID.IsSet() {
		req.Tools = nil
		return req, extra, nil
	}
	// The pinned SDK omits function strictness. Its public extra-body hook
	// preserves that field without changing SDK authentication, retries or SSE.
	if rawTools != nil {
		raw, e := sonic.Marshal(rawTools)
		if e != nil {
			return nil, nil, fmt.Errorf("serialize Responses strict list: %w", e)
		}
		if extra == nil {
			extra = make(map[string]interface{})
		}
		extra["tools"] = json.RawMessage(raw)
	}
	return req, extra, nil
}

func prepareResponsesPayload(ctx context.Context, in []*schema.Message, req *responses.ResponsesRequest, extra map[string]interface{}, modifier RequestPayloadModifier) (*responses.ResponsesRequest, map[string]interface{}, error) {
	if modifier == nil {
		return req, extra, nil
	}
	raw, err := sonic.Marshal(req)
	if err != nil {
		return nil, nil, err
	}
	var fields map[string]json.RawMessage
	if err = sonic.Unmarshal(raw, &fields); err != nil {
		return nil, nil, err
	}
	for key, value := range extra {
		fields[key], err = sonic.Marshal(value)
		if err != nil {
			return nil, nil, err
		}
	}
	originalInput := fields["input"]
	raw, err = sonic.Marshal(fields)
	if err != nil {
		return nil, nil, err
	}
	raw, err = modifier(ctx, in, raw)
	if err != nil {
		return nil, nil, fmt.Errorf("modify request payload: %w", err)
	}
	fields = nil
	if err = sonic.Unmarshal(raw, &fields); err != nil || fields == nil {
		return nil, nil, fmt.Errorf("modified request payload must be a JSON object")
	}
	var name string
	if value, ok := fields["model"]; !ok || sonic.Unmarshal(value, &name) != nil || name == "" {
		return nil, nil, fmt.Errorf("modified request payload must contain a non-empty string model")
	}
	input, ok := fields["input"]
	if !ok || bytes.Equal(bytes.TrimSpace(input), []byte("null")) {
		return nil, nil, fmt.Errorf("modified request payload must contain input")
	}
	// Validate the input union before dispatch; retain original JSON for unknown
	// fields and exact numbers instead of round-tripping through generated types.
	var typedInput responses.ResponsesInput
	if err = sonic.Unmarshal(input, &typedInput); err != nil {
		return nil, nil, fmt.Errorf("modified input: %w", err)
	}
	fields["stream"], _ = sonic.Marshal(req.Stream.Or(false))
	body := make(map[string]interface{}, len(fields))
	for key, value := range fields {
		body[key] = value
	}
	wire := &responses.ResponsesRequest{Model: name, Input: typedInput, Stream: req.Stream}
	// When input was not edited, retain the live SDK view used by native output
	// replay. Its new file:// inputs are updated during SDK preprocessing.
	if replay, ok := extra["input"].(*responsesInput); ok && bytes.Equal(input, originalInput) {
		wire.Input = req.Input
		body["input"] = replay
	}
	// Cache metadata must describe the payload actually sent after modification.
	for key, target := range map[string]interface{}{"store": &wire.Store, "expire_at": &wire.ExpireAt, "caching": &wire.Caching} {
		if value, exists := fields[key]; exists {
			if err = sonic.Unmarshal(value, target); err != nil {
				return nil, nil, fmt.Errorf("modified %s: %w", key, err)
			}
		}
	}
	return wire, body, nil
}

func responsesCallbackConfig(req *responses.ResponsesRequest) *model.Config {
	return &model.Config{Model: req.Model, MaxTokens: int(req.MaxOutputTokens.Or(0)),
		Temperature: float32(req.Temperature.Or(0)), TopP: float32(req.TopP.Or(0))}
}

func responsesHeaders(opts *arkOptions) map[string]string {
	if opts.enableWebSearch == nil {
		return opts.customHeaders
	}
	headers := make(map[string]string, len(opts.customHeaders)+1)
	hasBeta := false
	for key, value := range opts.customHeaders {
		headers[key] = value
		if strings.EqualFold(key, "ark-beta-web-search") {
			hasBeta = true
		}
	}
	if !hasBeta {
		headers["ark-beta-web-search"] = "true"
	}
	return headers
}

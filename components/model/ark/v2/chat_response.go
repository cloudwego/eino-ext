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
	"encoding/json"
	"fmt"
	"math"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/chat"
)

const (
	keyEncryptedContent  = "ark-encrypted-content"
	keyServiceStatus     = "ark-service-status"
	keyUsageDetails      = "ark-usage-details"
	keyModerationHitType = "ark-moderation-hit-type"
)

func resolveChatResponse(resp *chat.ChatCompletionResponse) (*schema.Message, error) {
	if resp == nil || len(resp.Choices) == 0 {
		return nil, ErrEmptyResponse
	}
	for _, choice := range resp.Choices {
		if choice.Index != 0 {
			continue
		}
		m := choice.Message
		msg := &schema.Message{Role: schema.RoleType(m.Role), Content: m.Content, ReasoningContent: m.ReasoningContent.Or(""),
			ResponseMeta: &schema.ResponseMeta{FinishReason: string(choice.FinishReason)}, Extra: map[string]any{},
		}
		for _, tc := range m.ToolCalls {
			msg.ToolCalls = append(msg.ToolCalls, schema.ToolCall{ID: tc.ID, Type: string(tc.Type), Function: schema.FunctionCall{Name: tc.Function.Name, Arguments: tc.Function.Arguments}})
		}
		if !choice.Logprobs.Null {
			msg.ResponseMeta.LogProbs = toLogProbs(choice.Logprobs.Value.Content)
		}
		if usage, ok := resp.Usage.Get(); ok {
			setUsage(msg, usage)
		}
		if m.EncryptedContent.IsSet() {
			msg.Extra[keyEncryptedContent] = m.EncryptedContent.Value
		}
		if choice.ModerationHitType.IsSet() {
			msg.Extra[keyModerationHitType] = metadataString(choice.ModerationHitType.Value)
		}
		setResponseExtra(msg, resp.ID, resp.Model, resp.ServiceTier, resp.ServiceStatus)
		return msg, nil
	}
	return nil, fmt.Errorf("invalid response format: choice with index 0 not found")
}

func resolveStreamResponse(resp *chat.ChatCompletionStreamResponse) (*schema.Message, bool, error) {
	if resp == nil {
		return nil, false, nil
	}
	var msg *schema.Message
	for _, choice := range resp.Choices {
		if choice.Index != 0 {
			continue
		}
		d := choice.Delta
		msg = &schema.Message{Role: schema.RoleType(d.Role.Or("")), Content: d.Content.Or(""), ReasoningContent: d.ReasoningContent.Or(""),
			ResponseMeta: &schema.ResponseMeta{FinishReason: string(choice.FinishReason.Or(""))}, Extra: map[string]any{},
		}
		for _, tc := range d.ToolCalls {
			index := int(tc.Index)
			function := tc.Function.Or(chat.ChatCompletionMessageToolCallChunkFunction{})
			msg.ToolCalls = append(msg.ToolCalls, schema.ToolCall{Index: &index, ID: tc.ID.Or(""), Type: tc.Type.Or(""),
				Function: schema.FunctionCall{Name: function.Name.Or(""), Arguments: function.Arguments.Or("")},
			})
		}
		if probs, ok := choice.Logprobs.Get(); ok {
			msg.ResponseMeta.LogProbs = toLogProbs(probs.Content)
		}
		if d.EncryptedContent.IsSet() {
			msg.Extra[keyEncryptedContent] = d.EncryptedContent.Value
		}
		if choice.ModerationHitType.IsSet() {
			msg.Extra[keyModerationHitType] = metadataString(choice.ModerationHitType.Value)
		}
		break
	}
	// Usage-only final chunks must survive conversion and stream concatenation.
	if msg == nil && !resp.Usage.Null {
		msg = &schema.Message{ResponseMeta: &schema.ResponseMeta{}, Extra: map[string]any{}}
	}
	if msg == nil {
		return nil, false, nil
	}
	if !resp.Usage.Null {
		setUsage(msg, resp.Usage.Value)
	}
	setResponseExtra(msg, resp.ID, resp.Model, resp.ServiceTier, resp.ServiceStatus)
	return msg, true, nil
}

func setResponseExtra(msg *schema.Message, id, model string, tier chat.OptResponseServiceTier, status chat.OptChatCompletionServiceStatus) {
	if id != "" {
		msg.Extra[keyRequestID] = metadataString(id)
	}
	if model != "" {
		msg.Extra[keyModelName] = metadataString(model)
	}
	if tier.IsSet() {
		msg.Extra[keyServiceTier] = metadataString(tier.Value)
	}
	if status.IsSet() {
		setServiceStatus(msg, status.Value)
	}
}

func setUsage(msg *schema.Message, usage chat.CompletionUsage) {
	msg.ResponseMeta.Usage = &schema.TokenUsage{
		PromptTokens: int(usage.PromptTokens), CompletionTokens: int(usage.CompletionTokens), TotalTokens: int(usage.TotalTokens),
		PromptTokenDetails:      schema.PromptTokenDetails{CachedTokens: int(usage.PromptTokensDetails.CachedTokens)},
		CompletionTokensDetails: schema.CompletionTokensDetails{ReasoningTokens: int(usage.CompletionTokensDetails.ReasoningTokens)},
	}
	setChatUsage(msg, usage)
}

func toLogProbs(probs []chat.ChatCompletionTokenLogprob) *schema.LogProbs {
	out := &schema.LogProbs{}
	for _, p := range probs {
		prob := schema.LogProb{Token: p.Token, LogProb: p.Logprob, Bytes: toBytes(p.Bytes)}
		for _, top := range p.TopLogprobs {
			prob.TopLogProbs = append(prob.TopLogProbs, schema.TopLogProb{Token: top.Token, LogProb: top.Logprob, Bytes: toBytes(top.Bytes)})
		}
		out.Content = append(out.Content, prob)
	}
	return out
}
func toBytes(bytes []int32) []int64 {
	if bytes == nil {
		return nil
	}
	out := make([]int64, len(bytes))
	for i, b := range bytes {
		out[i] = int64(b)
	}
	return out
}

func GetModerationHitType(msg *schema.Message) (string, bool) {
	if msg == nil {
		return "", false
	}
	return extraString(msg.Extra, keyModerationHitType)
}

// GetEncryptedContent returns encrypted reasoning for passing an assistant message
// back to the API. Stream fragments concatenate as ordinary strings.
func GetEncryptedContent(msg *schema.Message) (string, bool) {
	if msg == nil {
		return "", false
	}
	return extraString(msg.Extra, keyEncryptedContent)
}

// GetServiceStatus returns the service's model fallback information.
func GetServiceStatus(msg *schema.Message) (chat.ChatCompletionServiceStatus, bool) {
	return readServiceStatus(msg)
}

// GetTokenUsageDetails combines the current ResponseMeta.Usage with SDK-specific
// counts stored in Extra. It returns false when ResponseMeta.Usage is absent.
// It returns false if common counts cannot fit the SDK's int32 fields.
func GetTokenUsageDetails(msg *schema.Message) (chat.CompletionUsage, bool) {
	return readChatUsage(msg)
}

func setServiceStatus(msg *schema.Message, status chat.ChatCompletionServiceStatus) {
	msg.Extra[keyServiceStatus] = metadataBool(true)
	fallback, present := status.ModelFallback.Get()
	msg.Extra[keyServiceStatus+".model_fallback"] = metadataBool(present)
	msg.Extra[keyServiceStatus+".model_fallback.fallback_triggered"] = metadataBool(fallback.FallbackTriggered)
	msg.Extra[keyServiceStatus+".model_fallback.original_model"] = metadataString(fallback.OriginalModel)
}

func readServiceStatus(msg *schema.Message) (chat.ChatCompletionServiceStatus, bool) {
	if msg == nil {
		return chat.ChatCompletionServiceStatus{}, false
	}
	if present, ok := extraBool(msg.Extra, keyServiceStatus); !ok || !present {
		return chat.ChatCompletionServiceStatus{}, false
	}
	var result chat.ChatCompletionServiceStatus
	if present, _ := extraBool(msg.Extra, keyServiceStatus+".model_fallback"); present {
		triggered, _ := extraBool(msg.Extra, keyServiceStatus+".model_fallback.fallback_triggered")
		model, _ := extraString(msg.Extra, keyServiceStatus+".model_fallback.original_model")
		result.ModelFallback = chat.NewOptModelFallbackStatus(chat.ModelFallbackStatus{FallbackTriggered: triggered, OriginalModel: model})
	}
	return result, true
}

func setChatUsage(msg *schema.Message, usage chat.CompletionUsage) {
	for key, value := range map[string]chat.OptInt32{
		"prompt_tokens_details.text_tokens":            usage.PromptTokensDetails.TextTokens,
		"prompt_tokens_details.image_tokens":           usage.PromptTokensDetails.ImageTokens,
		"prompt_tokens_details.provisioned_tokens":     usage.PromptTokensDetails.ProvisionedTokens,
		"prompt_tokens_details.audio_tokens":           usage.PromptTokensDetails.AudioTokens,
		"prompt_tokens_details.audio_cached_tokens":    usage.PromptTokensDetails.AudioCachedTokens,
		"completion_tokens_details.provisioned_tokens": usage.CompletionTokensDetails.ProvisionedTokens,
	} {
		n, present := value.Get()
		setOptionalCount(msg, keyUsageDetails+"."+key, int64(n), present)
	}
}

func readChatUsage(msg *schema.Message) (chat.CompletionUsage, bool) {
	if msg == nil || msg.ResponseMeta == nil || msg.ResponseMeta.Usage == nil {
		return chat.CompletionUsage{}, false
	}
	usage := msg.ResponseMeta.Usage
	var result chat.CompletionUsage
	for field, n := range map[*int32]int{
		&result.PromptTokens: usage.PromptTokens, &result.CompletionTokens: usage.CompletionTokens, &result.TotalTokens: usage.TotalTokens,
		&result.PromptTokensDetails.CachedTokens:        usage.PromptTokenDetails.CachedTokens,
		&result.CompletionTokensDetails.ReasoningTokens: usage.CompletionTokensDetails.ReasoningTokens,
	} {
		if n < math.MinInt32 || n > math.MaxInt32 {
			return chat.CompletionUsage{}, false
		}
		*field = int32(n)
	}
	for key, field := range map[string]*chat.OptInt32{
		"prompt_tokens_details.text_tokens":            &result.PromptTokensDetails.TextTokens,
		"prompt_tokens_details.image_tokens":           &result.PromptTokensDetails.ImageTokens,
		"prompt_tokens_details.provisioned_tokens":     &result.PromptTokensDetails.ProvisionedTokens,
		"prompt_tokens_details.audio_tokens":           &result.PromptTokensDetails.AudioTokens,
		"prompt_tokens_details.audio_cached_tokens":    &result.PromptTokensDetails.AudioCachedTokens,
		"completion_tokens_details.provisioned_tokens": &result.CompletionTokensDetails.ProvisionedTokens,
	} {
		if n, ok := extraInt64(msg.Extra, keyUsageDetails+"."+key); ok && n >= math.MinInt32 && n <= math.MaxInt32 {
			*field = chat.NewOptInt32(int32(n))
		}
	}
	return result, true
}

// v0.12.0 models optional stream fields as non-nullable Opt values. The API
// also sends null for them, and can omit usage until the last chunk. Normalize
// only those known differences; keep the generated decoder for all other fields.
// This is installed through the SDK's exported stream Unmarshaler hook.
type chatStreamUnmarshaler struct{}

func (chatStreamUnmarshaler) Unmarshal(data []byte, target interface{}) error {
	response, ok := target.(*chat.ChatCompletionStreamResponse)
	if !ok {
		return sonic.Unmarshal(data, target)
	}
	decodeErr := sonic.Unmarshal(data, response)
	if decodeErr == nil {
		return nil
	}
	var body map[string]json.RawMessage
	if err := sonic.Unmarshal(data, &body); err != nil {
		return err
	}
	if body == nil {
		return decodeErr
	}
	if _, ok := body["usage"]; !ok {
		body["usage"] = json.RawMessage("null")
	}
	choices := []map[string]json.RawMessage{}
	if err := sonic.Unmarshal(body["choices"], &choices); err != nil {
		return err
	}
	for _, choice := range choices {
		omitNull(choice, "finish_reason", "logprobs", "moderation_hit_type")
		if raw, ok := choice["delta"]; ok {
			var delta map[string]json.RawMessage
			if err := sonic.Unmarshal(raw, &delta); err != nil {
				return err
			}
			omitNull(delta, "role", "content", "reasoning_content", "encrypted_content", "tool_calls")
			raw, err := sonic.Marshal(delta)
			if err != nil {
				return err
			}
			choice["delta"] = raw
		}
	}
	raw, err := sonic.Marshal(choices)
	if err != nil {
		return err
	}
	body["choices"] = raw
	raw, err = sonic.Marshal(body)
	if err != nil {
		return err
	}
	*response = chat.ChatCompletionStreamResponse{}
	return sonic.Unmarshal(raw, response)
}

func omitNull(fields map[string]json.RawMessage, names ...string) {
	for _, name := range names {
		if bytes.Equal(bytes.TrimSpace(fields[name]), []byte("null")) {
			delete(fields, name)
		}
	}
}

type streamPayloadDecoder struct {
	rawBody []byte
	capture bool
}

func (d *streamPayloadDecoder) Unmarshal(raw []byte, value interface{}) error {
	if _, ok := value.(*chat.ChatCompletionStreamResponse); ok && d.capture {
		d.rawBody = append([]byte(nil), raw...)
	}
	return (chatStreamUnmarshaler{}).Unmarshal(raw, value)
}

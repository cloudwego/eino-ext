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
	"fmt"

	fmodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/chat"
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
		msg.Extra[keyServiceStatus] = serviceStatus(status.Value)
	}
	if msg.ReasoningContent != "" {
		msg.Extra[keyReasoningContent] = msg.ReasoningContent
	}
}

func setUsage(msg *schema.Message, usage chat.CompletionUsage) {
	msg.ResponseMeta.Usage = &schema.TokenUsage{
		PromptTokens: int(usage.PromptTokens), CompletionTokens: int(usage.CompletionTokens), TotalTokens: int(usage.TotalTokens),
		PromptTokenDetails:      schema.PromptTokenDetails{CachedTokens: int(usage.PromptTokensDetails.CachedTokens)},
		CompletionTokensDetails: schema.CompletionTokensDetails{ReasoningTokens: int(usage.CompletionTokensDetails.ReasoningTokens)},
	}
	msg.Extra[keyUsageDetails] = tokenUsageDetails(usage)
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
func toModelCallbackUsage(meta *schema.ResponseMeta) *fmodel.TokenUsage {
	if meta == nil || meta.Usage == nil {
		return nil
	}
	u := meta.Usage
	return &fmodel.TokenUsage{PromptTokens: u.PromptTokens, CompletionTokens: u.CompletionTokens, TotalTokens: u.TotalTokens,
		PromptTokenDetails:      fmodel.PromptTokenDetails{CachedTokens: u.PromptTokenDetails.CachedTokens},
		CompletionTokensDetails: fmodel.CompletionTokensDetails{ReasoningTokens: u.CompletionTokensDetails.ReasoningTokens},
	}
}

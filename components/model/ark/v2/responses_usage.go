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
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/responses"
)

func setResponsesUsage(msg *schema.Message, usage responses.ResponseUsage) error {
	counts := map[string]responses.OptInt64{
		"input_tokens_details.audio_tokens":        usage.InputTokensDetails.AudioTokens,
		"input_tokens_details.audio_cached_tokens": usage.InputTokensDetails.AudioCachedTokens,
	}
	tools, present := usage.ToolUsage.Get()
	if present {
		msg.Extra[keyResponsesUsage+".tool_usage"] = metadataBool(true)
	} else {
		delete(msg.Extra, keyResponsesUsage+".tool_usage")
	}
	counts["tool_usage.web_search"], counts["tool_usage.mcp"] = tools.WebSearch, tools.Mcp
	counts["tool_usage.knowledge_search"], counts["tool_usage.doubao_app"] = tools.KnowledgeSearch, tools.DoubaoApp
	for key, value := range counts {
		n, present := value.Get()
		setOptionalCount(msg, keyResponsesUsage+"."+key, n, present)
	}
	if details, ok := usage.ToolUsageDetails.Get(); ok {
		return setJSONExtra(msg, keyResponsesUsage+".tool_usage_details", &details)
	}
	delete(msg.Extra, keyResponsesUsage+".tool_usage_details")
	return nil
}

func readResponsesUsage(msg *schema.Message) (responses.ResponseUsage, bool) {
	if msg == nil || msg.ResponseMeta == nil || msg.ResponseMeta.Usage == nil {
		return responses.ResponseUsage{}, false
	}
	usage := msg.ResponseMeta.Usage
	var result responses.ResponseUsage
	result.InputTokens, result.OutputTokens, result.TotalTokens = int64(usage.PromptTokens), int64(usage.CompletionTokens), int64(usage.TotalTokens)
	result.InputTokensDetails.CachedTokens = int64(usage.PromptTokenDetails.CachedTokens)
	result.OutputTokensDetails.ReasoningTokens = int64(usage.CompletionTokensDetails.ReasoningTokens)
	var tools responses.ToolUsage
	for key, field := range map[string]*responses.OptInt64{
		"input_tokens_details.audio_tokens":        &result.InputTokensDetails.AudioTokens,
		"input_tokens_details.audio_cached_tokens": &result.InputTokensDetails.AudioCachedTokens,
		"tool_usage.web_search":                    &tools.WebSearch, "tool_usage.mcp": &tools.Mcp,
		"tool_usage.knowledge_search": &tools.KnowledgeSearch, "tool_usage.doubao_app": &tools.DoubaoApp,
	} {
		if n, ok := extraInt64(msg.Extra, keyResponsesUsage+"."+key); ok {
			*field = responses.NewOptInt64(n)
		}
	}
	if present, _ := extraBool(msg.Extra, keyResponsesUsage+".tool_usage"); present {
		result.ToolUsage = responses.NewOptToolUsage(tools)
	}
	if details, ok := readJSONExtra[responses.ToolUsageDetails](msg, keyResponsesUsage+".tool_usage_details"); ok {
		result.ToolUsageDetails = responses.NewOptToolUsageDetails(details)
	}
	return result, true
}

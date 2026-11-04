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
	"encoding/json"
	"math"
	"reflect"

	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/chat"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/responses"
)

func extraInt64(msg *schema.Message, key string) (int64, bool) {
	if msg == nil {
		return 0, false
	}
	if number, ok := msg.Extra[key].(json.Number); ok {
		value, err := number.Int64()
		return value, err == nil
	}
	value := reflect.ValueOf(msg.Extra[key])
	if value.IsValid() {
		switch value.Kind() {
		case reflect.Int, reflect.Int32, reflect.Int64:
			return value.Int(), true
		case reflect.Float64:
			// Ordinary JSON decoding restores numbers as float64.
			f := value.Float()
			if math.Trunc(f) == f && f >= math.MinInt64 && f < math.MaxInt64 {
				return int64(f), true
			}
		}
	}
	return 0, false
}

func extraBool(msg *schema.Message, key string) (bool, bool) {
	if msg != nil {
		value := reflect.ValueOf(msg.Extra[key])
		if value.IsValid() && value.Kind() == reflect.Bool {
			return value.Bool(), true
		}
	}
	return false, false
}

func setServiceStatus(msg *schema.Message, status chat.ChatCompletionServiceStatus) {
	msg.Extra[keyServiceStatus] = metadataBool(true)
	fallback, present := status.ModelFallback.Get()
	msg.Extra[keyServiceStatus+".model_fallback"] = metadataBool(present)
	msg.Extra[keyServiceStatus+".model_fallback.fallback_triggered"] = metadataBool(fallback.FallbackTriggered)
	msg.Extra[keyServiceStatus+".model_fallback.original_model"] = metadataString(fallback.OriginalModel)
}

func readServiceStatus(msg *schema.Message) (chat.ChatCompletionServiceStatus, bool) {
	if present, ok := extraBool(msg, keyServiceStatus); !ok || !present {
		return readJSONExtra[chat.ChatCompletionServiceStatus](msg, keyServiceStatus)
	}
	var result chat.ChatCompletionServiceStatus
	if present, _ := extraBool(msg, keyServiceStatus+".model_fallback"); present {
		triggered, _ := extraBool(msg, keyServiceStatus+".model_fallback.fallback_triggered")
		model, _ := extraString(msg, keyServiceStatus+".model_fallback.original_model")
		result.ModelFallback = chat.NewOptModelFallbackStatus(chat.ModelFallbackStatus{FallbackTriggered: triggered, OriginalModel: model})
	}
	return result, true
}

func setOptionalCount(msg *schema.Message, key string, value int64, present bool) {
	msg.Extra[key] = metadataInt64(value)
	// Explicit absence lets a later usage chunk clear an earlier optional count.
	msg.Extra[key+".set"] = metadataBool(present)
}

func optionalCount(msg *schema.Message, key string) (int64, bool) {
	if present, exists := extraBool(msg, key+".set"); exists && !present {
		return 0, false
	}
	return extraInt64(msg, key)
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
	result, found := readJSONExtra[chat.CompletionUsage](msg, keyUsageDetails)
	if msg != nil && msg.ResponseMeta != nil && msg.ResponseMeta.Usage != nil {
		usage := msg.ResponseMeta.Usage
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
		found = true
	} else if _, flat := extraInt64(msg, keyUsageDetails+".total_tokens"); flat {
		// Older messages also stored the common counts in Extra.
		for key, field := range map[string]*int32{
			"prompt_tokens": &result.PromptTokens, "completion_tokens": &result.CompletionTokens, "total_tokens": &result.TotalTokens,
			"prompt_tokens_details.cached_tokens":        &result.PromptTokensDetails.CachedTokens,
			"completion_tokens_details.reasoning_tokens": &result.CompletionTokensDetails.ReasoningTokens,
		} {
			n, ok := extraInt64(msg, keyUsageDetails+"."+key)
			if !ok || n < math.MinInt32 || n > math.MaxInt32 {
				return chat.CompletionUsage{}, false
			}
			*field = int32(n)
		}
		found = true
	}
	if !found {
		return chat.CompletionUsage{}, false
	}
	for key, field := range map[string]*chat.OptInt32{
		"prompt_tokens_details.text_tokens":            &result.PromptTokensDetails.TextTokens,
		"prompt_tokens_details.image_tokens":           &result.PromptTokensDetails.ImageTokens,
		"prompt_tokens_details.provisioned_tokens":     &result.PromptTokensDetails.ProvisionedTokens,
		"prompt_tokens_details.audio_tokens":           &result.PromptTokensDetails.AudioTokens,
		"prompt_tokens_details.audio_cached_tokens":    &result.PromptTokensDetails.AudioCachedTokens,
		"completion_tokens_details.provisioned_tokens": &result.CompletionTokensDetails.ProvisionedTokens,
	} {
		if n, ok := optionalCount(msg, keyUsageDetails+"."+key); ok && n >= math.MinInt32 && n <= math.MaxInt32 {
			*field = chat.NewOptInt32(int32(n))
		}
	}
	return result, true
}

func setResponsesUsage(msg *schema.Message, usage responses.ResponseUsage) error {
	counts := map[string]responses.OptInt64{
		"input_tokens_details.audio_tokens":        usage.InputTokensDetails.AudioTokens,
		"input_tokens_details.audio_cached_tokens": usage.InputTokensDetails.AudioCachedTokens,
	}
	tools, present := usage.ToolUsage.Get()
	msg.Extra[keyResponsesUsage+".tool_usage"] = metadataBool(present)
	counts["tool_usage.web_search"], counts["tool_usage.mcp"] = tools.WebSearch, tools.Mcp
	counts["tool_usage.knowledge_search"], counts["tool_usage.doubao_app"] = tools.KnowledgeSearch, tools.DoubaoApp
	for key, value := range counts {
		n, present := value.Get()
		setOptionalCount(msg, keyResponsesUsage+"."+key, n, present)
	}
	if details, ok := usage.ToolUsageDetails.Get(); ok {
		return setJSONExtra(msg, keyResponsesUsage+".tool_usage_details", &details)
	}
	msg.Extra[keyResponsesUsage+".tool_usage_details"] = jsonMetadata("")
	return nil
}

func readResponsesUsage(msg *schema.Message) (responses.ResponseUsage, bool) {
	result, found := readJSONExtra[responses.ResponseUsage](msg, keyResponsesUsage)
	if msg != nil && msg.ResponseMeta != nil && msg.ResponseMeta.Usage != nil {
		usage := msg.ResponseMeta.Usage
		result.InputTokens, result.OutputTokens, result.TotalTokens = int64(usage.PromptTokens), int64(usage.CompletionTokens), int64(usage.TotalTokens)
		result.InputTokensDetails.CachedTokens = int64(usage.PromptTokenDetails.CachedTokens)
		result.OutputTokensDetails.ReasoningTokens = int64(usage.CompletionTokensDetails.ReasoningTokens)
		found = true
	} else if _, flat := extraInt64(msg, keyResponsesUsage+".total_tokens"); flat {
		// Older messages also stored the common counts in Extra.
		for key, field := range map[string]*int64{
			"input_tokens": &result.InputTokens, "output_tokens": &result.OutputTokens, "total_tokens": &result.TotalTokens,
			"input_tokens_details.cached_tokens":     &result.InputTokensDetails.CachedTokens,
			"output_tokens_details.reasoning_tokens": &result.OutputTokensDetails.ReasoningTokens,
		} {
			n, ok := extraInt64(msg, keyResponsesUsage+"."+key)
			if !ok {
				return responses.ResponseUsage{}, false
			}
			*field = n
		}
		found = true
	}
	if !found {
		return responses.ResponseUsage{}, false
	}
	var tools responses.ToolUsage
	for key, field := range map[string]*responses.OptInt64{
		"input_tokens_details.audio_tokens":        &result.InputTokensDetails.AudioTokens,
		"input_tokens_details.audio_cached_tokens": &result.InputTokensDetails.AudioCachedTokens,
		"tool_usage.web_search":                    &tools.WebSearch, "tool_usage.mcp": &tools.Mcp,
		"tool_usage.knowledge_search": &tools.KnowledgeSearch, "tool_usage.doubao_app": &tools.DoubaoApp,
	} {
		if n, ok := optionalCount(msg, keyResponsesUsage+"."+key); ok {
			*field = responses.NewOptInt64(n)
		}
	}
	if present, _ := extraBool(msg, keyResponsesUsage+".tool_usage"); present {
		result.ToolUsage = responses.NewOptToolUsage(tools)
	}
	if details, ok := readJSONExtra[responses.ToolUsageDetails](msg, keyResponsesUsage+".tool_usage_details"); ok {
		result.ToolUsageDetails = responses.NewOptToolUsageDetails(details)
	}
	return result, true
}

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
	"math"

	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/chat"
)

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

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

package claude

import (
	"github.com/anthropics/anthropic-sdk-go"

	"github.com/cloudwego/eino/components/model"
)

// keyOfCacheUsage is the model.CallbackOutput.Extra key that carries CacheUsage.
// It is deliberately separate from the message extra keys in message_extra.go:
// cache accounting is callback metadata for one request, not message state that
// should travel with the conversation history.
const keyOfCacheUsage = "_eino_claude_cache_usage"

// CacheUsage reports Anthropic's granular prompt cache token counts for a single request.
//
// Anthropic bills cache reads at ~10% and cache writes at ~125% of the base input
// token rate, so the read and creation counts have to stay separate to compute cost.
// schema.TokenUsage.PromptTokenDetails.CachedTokens only carries the read side, which
// is why the creation count is surfaced here.
type CacheUsage struct {
	// CacheReadInputTokens is the Anthropic usage.cache_read_input_tokens value:
	// input tokens served from an existing cache entry.
	CacheReadInputTokens int
	// CacheCreationInputTokens is the Anthropic usage.cache_creation_input_tokens value:
	// input tokens written into a new cache entry.
	CacheCreationInputTokens int
}

// GetCacheUsage reports the cache token counts carried by a chat model callback output.
// The second return value is false when the request reported no cache activity.
func GetCacheUsage(out *model.CallbackOutput) (*CacheUsage, bool) {
	if out == nil {
		return nil, false
	}
	usage, ok := out.Extra[keyOfCacheUsage].(*CacheUsage)
	if !ok || usage == nil {
		return nil, false
	}
	return usage, true
}

// toCacheUsage returns nil when the response reported no cache activity, so that
// callback outputs of non-caching requests stay free of empty extra entries.
func toCacheUsage(u anthropic.Usage) *CacheUsage {
	if u.CacheReadInputTokens == 0 && u.CacheCreationInputTokens == 0 {
		return nil
	}
	return &CacheUsage{
		CacheReadInputTokens:     int(u.CacheReadInputTokens),
		CacheCreationInputTokens: int(u.CacheCreationInputTokens),
	}
}

func toDeltaCacheUsage(u anthropic.MessageDeltaUsage) *CacheUsage {
	if u.CacheReadInputTokens == 0 && u.CacheCreationInputTokens == 0 {
		return nil
	}
	return &CacheUsage{
		CacheReadInputTokens:     int(u.CacheReadInputTokens),
		CacheCreationInputTokens: int(u.CacheCreationInputTokens),
	}
}

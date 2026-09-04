/*
 * Copyright 2026 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package agenticopenai

import (
	"strconv"

	"github.com/cloudwego/eino/schema"
	"github.com/openai/openai-go/v3/responses"
)

// GetCacheWriteTokens returns the OpenAI cache_write_tokens count from the
// message. This is the number of input tokens written to the prompt cache.
// Pricing for cache writes depends on the model.
//
// The cache-read side is available through the standard token usage path:
//
//	msg.ResponseMeta.TokenUsage.PromptTokenDetails.CachedTokens
//
// When streaming, OpenAI reports cache-write usage on a response lifecycle
// event. schema.ConcatAgenticMessages merges Extra maps with a last-value-wins
// policy for int, so the final concatenated message preserves the count.
func GetCacheWriteTokens(msg *schema.AgenticMessage) (int, bool) {
	if msg == nil || msg.Extra == nil {
		return 0, false
	}
	tokens, ok := msg.Extra[keyOfCacheWriteTokens].(int)
	return tokens, ok
}

func cacheWriteTokensExtra(resp *responses.Response) map[string]any {
	if resp == nil {
		return nil
	}
	field, ok := resp.Usage.InputTokensDetails.JSON.ExtraFields["cache_write_tokens"]
	if !ok {
		return nil
	}
	tokens, err := strconv.Atoi(field.Raw())
	if err != nil || tokens <= 0 {
		return nil
	}
	return map[string]any{keyOfCacheWriteTokens: tokens}
}

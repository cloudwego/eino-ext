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

package agenticclaude

import "github.com/cloudwego/eino/schema"

// GetCacheCreationInputTokens returns the Anthropic cache_creation_input_tokens
// count from the message. This is the number of input tokens written into a new
// prompt cache entry, billed at ~125% of the base input token rate.
//
// The cache-read side is available through the standard token usage path:
//
//	msg.ResponseMeta.TokenUsage.PromptTokenDetails.CachedTokens
//
// When streaming, Anthropic reports this once per request (on message_start or
// message_delta). schema.ConcatAgenticMessages merges Extra maps with a
// last-value-wins policy for int, so the final concatenated message carries
// the correct count without accumulation.
func GetCacheCreationInputTokens(msg *schema.AgenticMessage) (int, bool) {
	if msg == nil || msg.Extra == nil {
		return 0, false
	}
	v, ok := msg.Extra[keyOfCacheCreationInputTokens].(int)
	return v, ok
}

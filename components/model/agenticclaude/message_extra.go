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

import (
	"github.com/anthropics/anthropic-sdk-go/packages/respjson"
	"github.com/cloudwego/eino/schema"
)

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

type responseExtraField string

// GetResponseExtraFields returns additional fields from the Messages API response
// preserved by WithResponseExtraFields. Each value is the original JSON text of
// the field.
func GetResponseExtraFields(msg *schema.AgenticMessage) map[string]string {
	if msg == nil {
		return nil
	}
	var fields map[string]string
	for key, value := range msg.Extra {
		var raw string
		switch value := value.(type) {
		case responseExtraField:
			raw = string(value)
		case string:
			raw = value
		default:
			continue
		}
		if fields == nil {
			fields = make(map[string]string)
		}
		fields[key] = raw
	}
	return fields
}

func extractResponseExtraFields(msg *schema.AgenticMessage, fields map[string]respjson.Field, keys map[string]struct{}) *schema.AgenticMessage {
	if msg == nil || len(fields) == 0 || keys == nil {
		return msg
	}
	for key, field := range fields {
		if len(keys) > 0 {
			if _, ok := keys[key]; !ok {
				continue
			}
		}
		raw := field.Raw()
		if raw == "" {
			continue
		}
		if msg.Extra == nil {
			msg.Extra = make(map[string]any)
		}
		msg.Extra[key] = responseExtraField(raw)
	}
	return msg
}

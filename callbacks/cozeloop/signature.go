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

package cozeloop

import "encoding/base64"

const (
	thoughtSignatureKey = "gemini_thought_signature"
)

// GetThoughtSignatureFromExtra
// copy from github.com/cloudwego/eino-ext/components/model/gemini, because it's go version is 1.24. we can't depend on it.
//
// tries to read thought_signature from an Extra map.
//
// thought_signature should be read from:
//   - message.AssistantGenMultiContent[i].Extra: thought_signature on each generated output part
//   - toolCall.Extra: thought_signature on toolCall
//   - message.Extra: thought_signature on generated content (legacy, only used when message.AssistantGenMultiContent are absent)
//
// The returned bool indicates whether thought_signature key exists in Extra.
// The returned []byte is the thought signature if available
func GetThoughtSignatureFromExtra(extra map[string]any) ([]byte, bool) {
	if extra == nil {
		return nil, false
	}

	signature, exists := extra[thoughtSignatureKey]
	if !exists {
		return nil, false
	}

	switch sig := signature.(type) {
	case []byte:
		if len(sig) == 0 {
			return nil, true
		}
		return sig, true
	case string:
		// When marshaling a map[string]any to JSON, a []byte value is encoded as a base64 string.
		// After unmarshaling back into map[string]any, the value becomes string.
		// Decode it here for compatibility with messages restored from JSON.
		if sig == "" {
			return nil, true
		}
		decoded, err := base64.StdEncoding.DecodeString(sig)
		if err != nil {
			return nil, true
		}
		if len(decoded) == 0 {
			return nil, true
		}
		return decoded, true
	default:
		return nil, true
	}
}

func GetBase64ThoughtSignatureFromExtra(extra map[string]any) string {
	signBytes, ok := GetThoughtSignatureFromExtra(extra)
	if !ok {
		return ""
	}
	if signBytes == nil {
		return ""
	}

	return base64.StdEncoding.EncodeToString(signBytes)
}

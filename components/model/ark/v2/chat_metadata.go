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
	"github.com/volcengine/ark-runtime-go/arkruntime/model/chat"
)

const (
	keyEncryptedContent  = "ark-encrypted-content"
	keyServiceStatus     = "ark-service-status"
	keyUsageDetails      = "ark-usage-details"
	keyModerationHitType = "ark-moderation-hit-type"
)

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

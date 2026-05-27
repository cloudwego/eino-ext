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
	"github.com/cloudwego/eino/schema"
)

const (
	keyOfResponseAutoCached = "_eino_ext_agenticopenai_auto_cached"
)

// allowedNonSelfGeneratedBlockTypes defines the whitelist of ContentBlockTypes
// that can be processed when a message is NOT self-generated (i.e., from other models).
// These types are model-agnostic and have standardized fields without model-specific extensions.
var allowedNonSelfGeneratedBlockTypes = map[schema.ContentBlockType]bool{
	// User Input types - standardized fields, no Extension
	schema.ContentBlockTypeUserInputText:  true,
	schema.ContentBlockTypeUserInputImage: true,
	schema.ContentBlockTypeUserInputAudio: true,
	schema.ContentBlockTypeUserInputVideo: true,
	schema.ContentBlockTypeUserInputFile:  true,
	// Assistant Gen types - standardized fields
	// Note: AssistantGenText has Extension, but we only read the Text field
	schema.ContentBlockTypeAssistantGenText:  true,
	schema.ContentBlockTypeAssistantGenImage: true,
	schema.ContentBlockTypeAssistantGenAudio: true,
	schema.ContentBlockTypeAssistantGenVideo: true,
	// Function Tool types - user-defined tools, cross-model compatible
	schema.ContentBlockTypeFunctionToolCall:   true,
	schema.ContentBlockTypeFunctionToolResult: true,
	schema.ContentBlockTypeToolSearchResult:   true,
}

// isAllowedNonSelfGeneratedBlockType checks if a ContentBlockType is in the whitelist
// for non-self-generated messages.
func isAllowedNonSelfGeneratedBlockType(blockType schema.ContentBlockType) bool {
	return allowedNonSelfGeneratedBlockTypes[blockType]
}

func isSelfGeneratedMessage(msg *schema.AgenticMessage) bool {
	return msg != nil && msg.ResponseMeta != nil && msg.ResponseMeta.OpenAIExtension != nil
}

func setAutoCached(msg *schema.AgenticMessage) *schema.AgenticMessage {
	if msg.Extra == nil {
		msg.Extra = map[string]any{}
	}
	msg.Extra[keyOfResponseAutoCached] = true
	return msg
}

// InvalidateMessageCaches temporarily disables caching for the specified messages.
// When a message is modified or model is switched, OPENAI invalidates caches for that message and all subsequent ones.
// Call this to mark those message caches as invalid temporarily.
func InvalidateMessageCaches(messages []*schema.AgenticMessage) error {
	for _, msg := range messages {
		if msg.Extra != nil {
			delete(msg.Extra, keyOfResponseAutoCached)
		}
	}
	return nil
}

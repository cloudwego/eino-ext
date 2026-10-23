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
	"reflect"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/chat"
)

const (
	keyRequestID         = "ark-request-id"
	keyModelName         = "ark-model-name"
	keyReasoningContent  = "ark-reasoning-content"
	keyServiceTier       = "ark-service-tier"
	keyEncryptedContent  = "ark-encrypted-content"
	keyServiceStatus     = "ark-service-status"
	keyUsageDetails      = "ark-usage-details"
	keyModerationHitType = "ark-moderation-hit-type"
	videoURLFPS          = "ark-model-video-url-fps"
)

// Distinct registration names let both major versions coexist in one process.
type metadataString string
type serviceStatus chat.ChatCompletionServiceStatus
type tokenUsageDetails chat.CompletionUsage

func init() {
	schema.RegisterName[metadataString]("_eino_ext_ark_v2_metadata")
	schema.RegisterName[serviceStatus]("_eino_ext_ark_v2_service_status")
	schema.RegisterName[tokenUsageDetails]("_eino_ext_ark_v2_usage_details")
	compose.RegisterStreamChunkConcatFunc(lastValue[metadataString])
	compose.RegisterStreamChunkConcatFunc(lastValue[serviceStatus])
	compose.RegisterStreamChunkConcatFunc(lastValue[tokenUsageDetails])
}
func lastValue[T any](chunks []T) (T, error) {
	if len(chunks) == 0 {
		var zero T
		return zero, nil
	}
	return chunks[len(chunks)-1], nil
}
func extraString(msg *schema.Message, key string) (string, bool) {
	if msg == nil {
		return "", false
	}
	// Accept legacy defined string types and JSON-restored plain strings too.
	value := reflect.ValueOf(msg.Extra[key])
	if value.IsValid() && value.Kind() == reflect.String {
		return value.String(), true
	}
	return "", false
}
func GetArkRequestID(msg *schema.Message) string {
	value, _ := extraString(msg, keyRequestID)
	return value
}
func GetModelName(msg *schema.Message) (string, bool) {
	return extraString(msg, keyModelName)
}
func GetReasoningContent(msg *schema.Message) (string, bool) {
	return extraString(msg, keyReasoningContent)
}
func GetServiceTier(msg *schema.Message) (string, bool) { return extraString(msg, keyServiceTier) }
func GetModerationHitType(msg *schema.Message) (string, bool) {
	return extraString(msg, keyModerationHitType)
}

// GetEncryptedContent returns encrypted reasoning for passing an assistant message
// back to the API. Stream fragments concatenate as ordinary strings.
func GetEncryptedContent(msg *schema.Message) (string, bool) {
	return extraString(msg, keyEncryptedContent)
}

// GetServiceStatus returns the service's model fallback information.
func GetServiceStatus(msg *schema.Message) (chat.ChatCompletionServiceStatus, bool) {
	if msg == nil {
		return chat.ChatCompletionServiceStatus{}, false
	}
	value, ok := msg.Extra[keyServiceStatus].(serviceStatus)
	return chat.ChatCompletionServiceStatus(value), ok
}

// GetTokenUsageDetails returns the full SDK usage, including modality-specific
// counts that the common Eino TokenUsage does not represent.
func GetTokenUsageDetails(msg *schema.Message) (chat.CompletionUsage, bool) {
	if msg == nil {
		return chat.CompletionUsage{}, false
	}
	value, ok := msg.Extra[keyUsageDetails].(tokenUsageDetails)
	return chat.CompletionUsage(value), ok
}

func SetInputVideoFPS(part *schema.MessageInputVideo, fps float64) {
	if part == nil {
		return
	}
	if part.Extra == nil {
		part.Extra = map[string]any{}
	}
	part.Extra[videoURLFPS] = fps
}
func GetInputVideoFPS(part *schema.MessageInputVideo) *float64 {
	if part == nil {
		return nil
	}
	return getFPS(part.Extra)
}
func SetFPS(part *schema.ChatMessageVideoURL, fps float64) {
	if part == nil {
		return
	}
	if part.Extra == nil {
		part.Extra = map[string]any{}
	}
	part.Extra[videoURLFPS] = fps
}
func GetFPS(part *schema.ChatMessageVideoURL) *float64 {
	if part == nil {
		return nil
	}
	return getFPS(part.Extra)
}
func GetOutputVideoFPS(part *schema.MessageOutputVideo) *float64 {
	if part == nil {
		return nil
	}
	return getFPS(part.Extra)
}
func getFPS(extra map[string]any) *float64 {
	value, ok := extra[videoURLFPS].(float64)
	if !ok {
		return nil
	}
	return &value
}

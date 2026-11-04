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
	"fmt"
	"reflect"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/chat"
)

const (
	keyRequestID         = "ark-request-id"
	keyModelName         = "ark-model-name"
	keyServiceTier       = "ark-service-tier"
	keyEncryptedContent  = "ark-encrypted-content"
	keyServiceStatus     = "ark-service-status"
	keyUsageDetails      = "ark-usage-details"
	keyModerationHitType = "ark-moderation-hit-type"
	videoURLFPS          = "ark-model-video-url-fps"
)

// Distinct registration names let both major versions coexist in one process.
type metadataString string

// Named atomic values preserve last-value stream concatenation. Their Go types
// need not survive JSON decoding; readers also accept ordinary scalar values.
type metadataInt64 int64
type metadataBool bool

// jsonMetadata is a JSON string for variable-shaped SDK output/tool details.
// Keeping the payload opaque preserves private SDK union fields with gob too.
type jsonMetadata string

func init() {
	schema.RegisterName[metadataString]("_eino_ext_ark_v2_metadata")
	schema.RegisterName[metadataInt64]("_eino_ext_ark_v2_metadata_int64")
	schema.RegisterName[metadataBool]("_eino_ext_ark_v2_metadata_bool")
	schema.RegisterName[jsonMetadata]("_eino_ext_ark_v2_json_metadata")
	compose.RegisterStreamChunkConcatFunc(lastValue[metadataString])
	compose.RegisterStreamChunkConcatFunc(lastValue[metadataInt64])
	compose.RegisterStreamChunkConcatFunc(lastValue[metadataBool])
	compose.RegisterStreamChunkConcatFunc(lastValue[jsonMetadata])
}

func setJSONExtra(msg *schema.Message, key string, value any) error {
	raw, err := sonic.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode %s metadata: %w", key, err)
	}
	msg.Extra[key] = jsonMetadata(string(raw))
	return nil
}

func readJSONExtra[T any](msg *schema.Message, key string) (T, bool) {
	var zero T
	raw, ok := extraString(msg, key)
	if !ok {
		return zero, false
	}
	var result T
	if err := sonic.UnmarshalString(raw, &result); err != nil {
		return zero, false
	}
	return result, true
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
	// JSON decoding restores named scalar values as ordinary strings.
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
	if msg != nil && msg.ReasoningContent != "" {
		return msg.ReasoningContent, true
	}
	return "", false
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
	return readServiceStatus(msg)
}

// GetTokenUsageDetails combines the current ResponseMeta.Usage with SDK-specific
// counts stored in Extra. It returns false when ResponseMeta.Usage is absent.
// It returns false if common counts cannot fit the SDK's int32 fields.
func GetTokenUsageDetails(msg *schema.Message) (chat.CompletionUsage, bool) {
	return readChatUsage(msg)
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

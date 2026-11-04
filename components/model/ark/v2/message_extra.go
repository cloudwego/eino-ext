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
	"encoding/json"
	"fmt"

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
	keyVideoFPS          = "ark-model-video-url-fps"
)

// Named atomic values preserve last-value stream concatenation. JSON decoding
// restores their built-in scalar types; gob preserves these registered types.
type metadataString string
type metadataInt64 int64
type metadataBool bool

func init() {
	schema.RegisterName[metadataString]("_eino_ext_ark_v2_metadata")
	schema.RegisterName[metadataInt64]("_eino_ext_ark_v2_metadata_int64")
	schema.RegisterName[metadataBool]("_eino_ext_ark_v2_metadata_bool")
	compose.RegisterStreamChunkConcatFunc(lastValue[metadataString])
	compose.RegisterStreamChunkConcatFunc(lastValue[metadataInt64])
	compose.RegisterStreamChunkConcatFunc(lastValue[metadataBool])
}

func setJSONExtra(msg *schema.Message, key string, value any) error {
	raw, err := sonic.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode %s metadata: %w", key, err)
	}
	// Keeping SDK JSON in a string preserves private union fields with gob too.
	msg.Extra[key] = metadataString(raw)
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
	switch value := msg.Extra[key].(type) {
	case metadataString:
		return string(value), true
	case string:
		return value, true
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

// SetInputVideoFPS sets the frame sampling rate in MessageInputPart.Extra.
// Nil or non-video parts are ignored.
func SetInputVideoFPS(part *schema.MessageInputPart, fps float64) {
	if part == nil || part.Type != schema.ChatMessagePartTypeVideoURL {
		return
	}
	if part.Extra == nil {
		part.Extra = map[string]any{}
	}
	part.Extra[keyVideoFPS] = fps
}

// GetInputVideoFPS reads the FPS setting from a video input part's Extra.
func GetInputVideoFPS(part *schema.MessageInputPart) *float64 {
	if part == nil || part.Type != schema.ChatMessagePartTypeVideoURL {
		return nil
	}
	return getFPS(part.Extra)
}

// GetOutputVideoFPS reads the FPS setting from a video output part's Extra.
func GetOutputVideoFPS(part *schema.MessageOutputPart) *float64 {
	if part == nil || part.Type != schema.ChatMessagePartTypeVideoURL {
		return nil
	}
	return getFPS(part.Extra)
}
func getFPS(extra map[string]any) *float64 {
	switch value := extra[keyVideoFPS].(type) {
	case float64:
		return &value
	case json.Number:
		// JSON decoders configured with UseNumber restore the same numeric
		// metadata as json.Number rather than float64.
		fps, err := value.Float64()
		if err == nil {
			return &fps
		}
	}
	return nil
}

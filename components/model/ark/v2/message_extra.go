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
	"math"

	"github.com/bytedance/sonic"
	fmodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

const (
	keyRequestID   = "ark-request-id"
	keyModelName   = "ark-model-name"
	keyServiceTier = "ark-service-tier"
	keyVideoFPS    = "ark-model-video-url-fps"
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
	if msg == nil {
		return zero, false
	}
	raw, ok := extraString(msg.Extra, key)
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
func extraString(extra map[string]any, key string) (string, bool) {
	return stringValue(extra[key])
}

func stringValue(raw any) (string, bool) {
	switch value := raw.(type) {
	case metadataString:
		return string(value), true
	case string:
		return value, true
	}
	return "", false
}
func extraInt64(extra map[string]any, key string) (int64, bool) {
	return int64Value(extra[key])
}

func int64Value(raw any) (int64, bool) {
	switch value := raw.(type) {
	case metadataInt64:
		return int64(value), true
	case json.Number:
		n, err := value.Int64()
		return n, err == nil
	case float64:
		// Ordinary JSON decoding restores numbers as float64.
		if math.Trunc(value) == value && value >= math.MinInt64 && value < math.MaxInt64 {
			return int64(value), true
		}
	}
	return 0, false
}

func extraBool(extra map[string]any, key string) (bool, bool) {
	return boolValue(extra[key])
}

func boolValue(raw any) (bool, bool) {
	switch value := raw.(type) {
	case metadataBool:
		return bool(value), true
	case bool:
		return value, true
	}
	return false, false
}

// extraIndex distinguishes a missing key from a present, invalid index.
func extraIndex(extra map[string]any, key string) (index int, exists bool, err error) {
	raw, exists := extra[key]
	if !exists {
		return 0, false, nil
	}
	value, ok := int64Value(raw)
	if !ok || value < 0 || int64(int(value)) != value {
		return 0, true, fmt.Errorf("Extra[%q] must be a non-negative index that fits int", key)
	}
	return int(value), true, nil
}

func GetArkRequestID(msg *schema.Message) string {
	if msg == nil {
		return ""
	}
	value, _ := extraString(msg.Extra, keyRequestID)
	return value
}
func GetModelName(msg *schema.Message) (string, bool) {
	if msg == nil {
		return "", false
	}
	return extraString(msg.Extra, keyModelName)
}
func GetServiceTier(msg *schema.Message) (string, bool) {
	if msg == nil {
		return "", false
	}
	return extraString(msg.Extra, keyServiceTier)
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

func setOptionalCount(msg *schema.Message, key string, value int64, present bool) {
	if present {
		msg.Extra[key] = metadataInt64(value)
	} else {
		delete(msg.Extra, key)
	}
}

func toModelCallbackUsage(meta *schema.ResponseMeta) *fmodel.TokenUsage {
	if meta == nil || meta.Usage == nil {
		return nil
	}
	u := meta.Usage
	return &fmodel.TokenUsage{PromptTokens: u.PromptTokens, CompletionTokens: u.CompletionTokens, TotalTokens: u.TotalTokens,
		PromptTokenDetails:      fmodel.PromptTokenDetails{CachedTokens: u.PromptTokenDetails.CachedTokens},
		CompletionTokensDetails: fmodel.CompletionTokensDetails{ReasoningTokens: u.CompletionTokensDetails.ReasoningTokens},
	}
}

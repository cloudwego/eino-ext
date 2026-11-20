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
	"strings"

	"github.com/bytedance/sonic"
	fmodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// extraKeyPrefix namespaces adapter metadata carried by messages and their parts or tool calls.
const extraKeyPrefix = "ark-v2-"

const (
	keyRequestID           = extraKeyPrefix + "request-id"
	keyModelName           = extraKeyPrefix + "model-name"
	keyServiceTier         = extraKeyPrefix + "service-tier"
	keyRawReasoningContent = extraKeyPrefix + "raw-reasoning-content"
	keyVideoFPS            = extraKeyPrefix + "model-video-url-fps"
)

// Named atomic values preserve last-value stream concatenation. JSON decoding
// restores their built-in scalar types; gob preserves these registered types.
type metadataString string
type metadataInt64 int64
type metadataBool bool

type rawReasoningContent []string

// ClearReasoning returns history suitable for replay through Chat Completions or
// Responses after removing reasoning text, signatures, and native reasoning items.
// It copies each message, its Extra map and its output part slice. Other nested
// values are shared. To replay the full history with Responses, pass WithCache(nil).
func ClearReasoning(messages []*schema.Message) []*schema.Message {
	if messages == nil {
		return nil
	}
	clean := make([]*schema.Message, len(messages))
	for i, msg := range messages {
		if msg == nil {
			continue
		}
		m := *msg
		m.ReasoningContent = ""
		m.AssistantGenMultiContent = withoutReasoningParts(msg.AssistantGenMultiContent)
		if msg.Extra != nil {
			m.Extra = make(map[string]any, len(msg.Extra))
			for key, value := range msg.Extra {
				if key != keyEncryptedContent {
					m.Extra[key] = value
				}
			}
		}
		clean[i] = &m
	}
	return clean
}

func withoutReasoningParts(parts []schema.MessageOutputPart) []schema.MessageOutputPart {
	if parts == nil {
		return nil
	}
	// Only one child carries the native item type. Opaque siblings can precede
	// or follow the standard reasoning part and must leave history with it.
	isReasoning := func(part schema.MessageOutputPart) bool {
		kind, _ := extraString(part.Extra, keyResponsesItemType)
		return part.Type == schema.ChatMessagePartTypeReasoning || kind == "reasoning"
	}
	reasoningItems := make(map[int]bool)
	for _, part := range parts {
		if isReasoning(part) {
			if index, exists, err := extraIndex(part.Extra, keyResponsesItemIndex); exists && err == nil {
				reasoningItems[index] = true
			}
		}
	}
	kept := make([]schema.MessageOutputPart, 0, len(parts))
	for _, part := range parts {
		if isReasoning(part) {
			continue
		}
		if index, exists, err := extraIndex(part.Extra, keyResponsesItemIndex); exists && err == nil && reasoningItems[index] {
			continue
		}
		kept = append(kept, part)
	}
	return kept
}

// GetRawReasoningContent returns the ordered reasoning.content text segments of an Ark reasoning part.
func GetRawReasoningContent(part *schema.MessageOutputPart) ([]string, bool) {
	if part == nil || part.Reasoning == nil {
		return nil, false
	}
	raw, ok := decodeRawReasoning(part.Extra[keyRawReasoningContent])
	return raw, ok && len(raw) > 0
}

func decodeRawReasoning(value any) ([]string, bool) {
	switch raw := value.(type) {
	case rawReasoningContent:
		return []string(raw), true
	case []string:
		return raw, true
	case []any:
		texts := make([]string, len(raw))
		for i, v := range raw {
			text, ok := v.(string)
			if !ok {
				return nil, false
			}
			texts[i] = text
		}
		return texts, true
	}
	return nil, false
}

func concatRawReasoning(chunks []rawReasoningContent) (rawReasoningContent, error) {
	var count int
	for _, chunk := range chunks {
		if len(chunk) > count {
			count = len(chunk)
		}
	}
	text := make([]strings.Builder, count)
	for _, chunk := range chunks {
		for i, delta := range chunk {
			text[i].WriteString(delta)
		}
	}
	result := make(rawReasoningContent, len(text))
	for i := range text {
		result[i] = text[i].String()
	}
	return result, nil
}

func init() {
	schema.RegisterName[rawReasoningContent]("_eino_ext_ark_v2_raw_reasoning_content")
	compose.RegisterStreamChunkConcatFunc(concatRawReasoning)
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

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

package ark

import (
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

const (
	keyOfRequestID             = "ark-request-id"
	keyOfReasoningContent      = "ark-reasoning-content"
	keyOfModelName             = "ark-model-name"
	videoURLFPS                = "ark-model-video-url-fps"
	keyOfContextID             = "ark-context-id"
	keyOfResponseID            = "ark-response-id"
	keyOfResponseCacheExpireAt = "ark-response-cache-expire-at"
	keyOfServiceTier           = "ark-service-tier"
	ImageSizeKey               = "seedream-image-size"
)

type arkRequestID string
type arkModelName string
type arkServiceTier string
type arkResponseID string
type arkContextID string
type arkResponseCacheExpireAt int64

func init() {
	compose.RegisterStreamChunkConcatFunc(func(chunks []arkRequestID) (final arkRequestID, err error) {
		if len(chunks) == 0 {
			return "", nil
		}
		return chunks[len(chunks)-1], nil
	})
	schema.RegisterName[arkRequestID]("_eino_ext_ark_request_id")

	compose.RegisterStreamChunkConcatFunc(func(chunks []arkModelName) (final arkModelName, err error) {
		if len(chunks) == 0 {
			return "", nil
		}
		return chunks[len(chunks)-1], nil
	})
	schema.RegisterName[arkModelName]("_eino_ext_ark_model_name")

	compose.RegisterStreamChunkConcatFunc(func(chunks []arkServiceTier) (final arkServiceTier, err error) {
		if len(chunks) == 0 {
			return "", nil
		}
		return chunks[len(chunks)-1], nil
	})
	schema.RegisterName[arkServiceTier]("_eino_ext_ark_service_tier")

	compose.RegisterStreamChunkConcatFunc(func(chunks []arkContextID) (final arkContextID, err error) {
		if len(chunks) == 0 {
			return "", nil
		}
		// Some chunks may not contain a contextID, so it is more reliable to take the first non-empty contextID.
		for _, chunk := range chunks {
			if chunk != "" {
				return chunk, nil
			}
		}
		return "", nil
	})
	schema.RegisterName[arkContextID]("_eino_ext_ark_context_id")

	compose.RegisterStreamChunkConcatFunc(func(chunks []arkResponseID) (final arkResponseID, err error) {
		if len(chunks) == 0 {
			return "", nil
		}
		// Some chunks may not contain a responseID, so it is more reliable to take the first non-empty responseID.
		for _, chunk := range chunks {
			if chunk != "" {
				return chunk, nil
			}
		}
		return "", nil
	})
	schema.RegisterName[arkResponseID]("_eino_ext_ark_response_id")

	compose.RegisterStreamChunkConcatFunc(func(chunks []arkResponseCacheExpireAt) (final arkResponseCacheExpireAt, err error) {
		if len(chunks) == 0 {
			return 0, nil
		}
		return chunks[len(chunks)-1], nil
	})
	schema.RegisterName[arkResponseCacheExpireAt]("_eino_ext_ark_response_cache_expire_at")
}

func GetArkRequestID(msg *schema.Message) string {
	reqID, _ := getMsgExtraValue[arkRequestID](msg, keyOfRequestID)
	return string(reqID)
}

func setArkRequestID(msg *schema.Message, reqID string) {
	setMsgExtra(msg, keyOfRequestID, arkRequestID(reqID))
}

func GetReasoningContent(msg *schema.Message) (string, bool) {
	return getMsgExtraValue[string](msg, keyOfReasoningContent)
}

func setReasoningContent(msg *schema.Message, reasoningContent string) {
	setMsgExtra(msg, keyOfReasoningContent, reasoningContent)
}

func GetModelName(msg *schema.Message) (string, bool) {
	modelName, ok := getMsgExtraValue[arkModelName](msg, keyOfModelName)
	if !ok {
		return "", false
	}
	return string(modelName), true
}

func setModelName(msg *schema.Message, name string) {
	setMsgExtra(msg, keyOfModelName, arkModelName(name))
}

// Deprecated: Use GetResponseID instead.
// GetContextID returns the conversation context ID from the message.
// Available only for ResponsesAPI responses.
func GetContextID(msg *schema.Message) (string, bool) {
	contextID_, ok := getMsgExtraValue[arkContextID](msg, keyOfContextID)
	if ok {
		return string(contextID_), true
	}
	// Since registering the concat logic requires defining `arkContextID` type,
	// this fallback logic needs to be retained to be compatible with `string` type.
	contextIDStr, ok := getMsgExtraValue[string](msg, keyOfContextID)
	if !ok {
		return "", false
	}
	return contextIDStr, true
}

func setContextID(msg *schema.Message, contextID string) {
	setMsgExtra(msg, keyOfContextID, arkContextID(contextID))
}

// InvalidateMessageCaches disables caching for the specified messages.
// When a message is modified, ARK invalidates caches for that message and all subsequent ones.
// Call this to mark those message caches as invalid.
func InvalidateMessageCaches(messages []*schema.Message) error {
	for _, msg := range messages {
		expireAtSec, ok := GetCacheExpiration(msg)
		if !ok || expireAtSec <= 0 {
			continue
		}

		// there may be concurrency
		extra := make(map[string]any, len(msg.Extra))
		for k, v := range msg.Extra {
			extra[k] = v
		}

		delete(extra, keyOfResponseCacheExpireAt)
		msg.Extra = extra
	}
	return nil
}

// GetResponseID returns the response ID from the message.
// Available only for ResponsesAPI responses.
func GetResponseID(msg *schema.Message) (string, bool) {
	responseID_, ok := getMsgExtraValue[arkResponseID](msg, keyOfResponseID)
	if ok {
		return string(responseID_), true
	}
	// When the user serializes and deserializes the message,
	// the type will be lost and compatibility with the string type is required.
	responseIDStr, ok := getMsgExtraValue[string](msg, keyOfResponseID)
	if !ok {
		return "", false
	}
	return responseIDStr, true
}

func setResponseID(msg *schema.Message, responseID string) {
	setMsgExtra(msg, keyOfResponseID, arkResponseID(responseID))
}

// GetCacheExpiration returns the cache expiration time in seconds.
// Only available for ResponsesAPI responses.
func GetCacheExpiration(msg *schema.Message) (expireAtSec int64, ok bool) {
	expireAtSec_, ok := getMsgExtraValue[arkResponseCacheExpireAt](msg, keyOfResponseCacheExpireAt)
	if ok {
		return int64(expireAtSec_), true
	}
	expireAtSec, ok = getMsgExtraValue[int64](msg, keyOfResponseCacheExpireAt)
	if ok {
		return expireAtSec, true
	}
	return 0, false
}

func setResponseCacheExpireAt(msg *schema.Message, expireAt arkResponseCacheExpireAt) {
	setMsgExtra(msg, keyOfResponseCacheExpireAt, expireAt)
}

func getMsgExtraValue[T any](msg *schema.Message, key string) (T, bool) {
	if msg == nil {
		var t T
		return t, false
	}
	val, ok := msg.Extra[key].(T)
	return val, ok
}

func setMsgExtra(msg *schema.Message, key string, value any) {
	if msg == nil {
		return
	}
	if msg.Extra == nil {
		msg.Extra = make(map[string]any)
	}
	msg.Extra[key] = value
}

func SetFPS(part *schema.ChatMessageVideoURL, fps float64) {
	if part == nil {
		return
	}
	if part.Extra == nil {
		part.Extra = make(map[string]any)
	}
	setFPS(part.Extra, fps)
}

func GetFPS(part *schema.ChatMessageVideoURL) *float64 {
	if part == nil {
		return nil
	}
	return getFPS(part.Extra)
}

func SetInputVideoFPS(part *schema.MessageInputVideo, fps float64) {
	if part == nil {
		return
	}
	if part.Extra == nil {
		part.Extra = make(map[string]any)
	}
	setFPS(part.Extra, fps)
}

func GetInputVideoFPS(part *schema.MessageInputVideo) *float64 {
	if part == nil {
		return nil
	}
	return getFPS(part.Extra)
}

func setOutputVideoFPS(part *schema.MessageOutputVideo, fps float64) {
	if part == nil {
		return
	}
	if part.Extra == nil {
		part.Extra = make(map[string]any)
	}
	setFPS(part.Extra, fps)
}

func GetOutputVideoFPS(part *schema.MessageOutputVideo) *float64 {
	if part == nil {
		return nil
	}
	return getFPS(part.Extra)
}

func setFPS(extra map[string]any, fps float64) {
	extra[videoURLFPS] = fps
}

func getFPS(extra map[string]any) *float64 {
	if extra == nil {
		return nil
	}
	fps, ok := extra[videoURLFPS].(float64)
	if !ok {
		return nil
	}
	return &fps
}

func GetServiceTier(msg *schema.Message) (string, bool) {
	t, ok := getMsgExtraValue[arkServiceTier](msg, keyOfServiceTier)
	if !ok {
		return "", false
	}
	return string(t), true
}

func setServiceTier(msg *schema.Message, serviceTier string) {
	if len(serviceTier) == 0 {
		return
	}
	setMsgExtra(msg, keyOfServiceTier, arkServiceTier(serviceTier))
}

func SetImageSize(part *schema.ChatMessageImageURL, size string) {
	if part == nil {
		return
	}
	if part.Extra == nil {
		part.Extra = make(map[string]any)
	}
	setImageSize(part.Extra, size)
}

func GetImageSize(part *schema.ChatMessageImageURL) (string, bool) {
	if part == nil {
		return "", false
	}
	return getImageSize(part.Extra)
}

func setOutputImageSize(part *schema.MessageOutputImage, size string) {
	if part == nil {
		return
	}
	if part.Extra == nil {
		part.Extra = make(map[string]any)
	}
	setImageSize(part.Extra, size)
}

func GetOutputImageSize(part *schema.MessageOutputImage) (string, bool) {
	if part == nil {
		return "", false
	}
	return getImageSize(part.Extra)
}

func setInputImageSize(part *schema.MessageInputImage, size string) {
	if part == nil {
		return
	}
	if part.Extra == nil {
		part.Extra = make(map[string]any)
	}
	setImageSize(part.Extra, size)
}

func GetInputImageSize(part *schema.MessageInputImage) (string, bool) {
	if part == nil {
		return "", false
	}
	return getImageSize(part.Extra)
}

func setImageSize(extra map[string]any, size string) {
	if extra == nil {
		return
	}
	extra[ImageSizeKey] = size
}

func getImageSize(extra map[string]any) (string, bool) {
	if extra == nil {
		return "", false
	}
	size, ok := extra[ImageSizeKey].(string)
	if !ok {
		return "", false
	}
	return size, true
}

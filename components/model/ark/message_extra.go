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
	keyOfRequestID        = "ark-request-id"
	keyOfReasoningContent = "ark-reasoning-content"
	keyOfModelName        = "ark-model-name"
	videoURLFPS           = "ark-model-video-url-fps"
	keyOfContextID        = "ark-context-id"
	keyOfResponseID       = "ark-response-id"
	keyOfResponseCaching  = "ark-response-caching"
	keyOfServiceTier      = "ark-service-tier"
	ImageSizeKey          = "seedream-image-size"
)

type arkRequestID string
type arkModelName string
type arkServiceTier string

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

	compose.RegisterStreamChunkConcatFunc(func(chunks []caching) (final caching, err error) {
		if len(chunks) == 0 {
			return "", nil
		}
		return chunks[len(chunks)-1], nil
	})
	schema.RegisterName[caching]("_eino_ext_ark_response_caching")
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
	return getMsgExtraValue[string](msg, keyOfContextID)
}

func setContextID(msg *schema.Message, contextID string) {
	setMsgExtra(msg, keyOfContextID, contextID)
}

// GetResponseID returns the response ID from the message.
// Available only for ResponsesAPI responses.
func GetResponseID(msg *schema.Message) (string, bool) {
	return getMsgExtraValue[string](msg, keyOfResponseID)
}

func setResponseID(msg *schema.Message, responseID string) {
	setMsgExtra(msg, keyOfResponseID, responseID)
}

func getResponseCaching(msg *schema.Message) (string, bool) {
	caching_, ok := getMsgExtraValue[caching](msg, keyOfResponseCaching)
	if !ok {
		return "", false
	}
	return string(caching_), true
}

// setResponseCaching sets the cached status of the response.
func setResponseCaching(msg *schema.Message, caching caching) {
	setMsgExtra(msg, keyOfResponseCaching, caching)
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
	part.Extra[videoURLFPS] = fps
}

func GetFPS(part *schema.ChatMessageVideoURL) *float64 {
	if part == nil {
		return nil
	}
	fps, ok := part.Extra[videoURLFPS].(float64)
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
	part.Extra[ImageSizeKey] = size
}

func GetImageSize(part *schema.ChatMessageImageURL) (string, bool) {
	if part == nil {
		return "", false
	}
	size, ok := part.Extra[ImageSizeKey].(string)
	if !ok {
		return "", false
	}
	return size, true
}

// func SetImageSize(part *schema.MessageOutputImage, size string) {
// 	if part == nil {
// 		return
// 	}
// 	if part.Extra == nil {
// 		part.Extra = make(map[string]any)
// 	}
// 	part.Extra[ImageSizeKey] = size
// }

// func GetImageSize(part *schema.MessageOutputImage) (string, bool) {
// 	if part == nil {
// 		return "", false
// 	}
// 	size, ok := part.Extra[ImageSizeKey].(string)
// 	if !ok {
// 		return "", false
// 	}
// 	return size, true
// }

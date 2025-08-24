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
	"encoding/gob"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

const (
	keyOfRequestID        = "ark-request-id"
	keyOfReasoningContent = "ark-reasoning-content"
	keyOfModelName        = "ark-model-name"
	videoURLFPS           = "ark-model-video-url-fps"
	keyOfContextID        = "ark-context-id"
)

type arkRequestID string
type arkModelName string

func init() {
	compose.RegisterStreamChunkConcatFunc(func(chunks []arkRequestID) (final arkRequestID, err error) {
		if len(chunks) == 0 {
			return "", nil
		}

		return chunks[len(chunks)-1], nil
	})
	_ = compose.RegisterSerializableType[arkRequestID]("_eino_ext_ark_request_id")
	gob.RegisterName("_eino_ext_ark_request_id", arkRequestID(""))

	compose.RegisterStreamChunkConcatFunc(func(chunks []arkModelName) (final arkModelName, err error) {
		if len(chunks) == 0 {
			return "", nil
		}

		return chunks[len(chunks)-1], nil
	})
	_ = compose.RegisterSerializableType[arkModelName]("_eino_ext_ark_model_name")
	gob.RegisterName("_eino_ext_ark_model_name", arkModelName(""))
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

// GetContextID returns the conversation context ID of the given message.
// Note:
//   - Only the first chunk returns the context ID.
//   - It is only available for ResponsesAPI.
func GetContextID(msg *schema.Message) (string, bool) {
	if msg == nil {
		return "", false
	}
	contextID, ok := getMsgExtraValue[string](msg, keyOfContextID)
	return contextID, ok
}

func setContextID(msg *schema.Message, contextID string) {
	setMsgExtra(msg, keyOfContextID, contextID)
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

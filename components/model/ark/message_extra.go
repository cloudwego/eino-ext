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

	compose.RegisterStreamChunkConcatFunc(func(chunks []arkModelName) (final arkModelName, err error) {
		if len(chunks) == 0 {
			return "", nil
		}

		return chunks[len(chunks)-1], nil
	})
	_ = compose.RegisterSerializableType[arkModelName]("_eino_ext_ark_model_name")
}

func GetArkRequestID(msg *schema.Message) string {
	reqID, ok := msg.Extra[keyOfRequestID].(arkRequestID)
	if !ok {
		return ""
	}
	return string(reqID)
}

func setArkRequestID(msg *schema.Message, reqID string) {
	if msg == nil {
		return
	}
	if msg.Extra == nil {
		msg.Extra = make(map[string]interface{})
	}
	msg.Extra[keyOfRequestID] = arkRequestID(reqID)
}

func GetReasoningContent(msg *schema.Message) (string, bool) {
	if msg == nil {
		return "", false
	}
	reasoningContent, ok := msg.Extra[keyOfReasoningContent].(string)
	if !ok {
		return "", false
	}

	return reasoningContent, true
}

func setReasoningContent(msg *schema.Message, reasoningContent string) {
	if msg == nil {
		return
	}
	if msg.Extra == nil {
		msg.Extra = make(map[string]interface{})
	}
	msg.Extra[keyOfReasoningContent] = reasoningContent
}

func GetModelName(msg *schema.Message) (string, bool) {
	if msg == nil {
		return "", false
	}
	modelName, ok := msg.Extra[keyOfModelName].(arkModelName)
	if !ok {
		return "", false
	}
	return string(modelName), true
}

func setModelName(msg *schema.Message, name string) {
	if msg == nil {
		return
	}
	if msg.Extra == nil {
		msg.Extra = make(map[string]interface{})
	}
	msg.Extra[keyOfModelName] = arkModelName(name)
}

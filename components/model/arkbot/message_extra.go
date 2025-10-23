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

package arkbot

import (
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
)

const (
	// Deprecated: use keyOfBotRequestID instead
	keyOfRequestID    = "ark-request-id"
	keyOfBotRequestID = "ark-bot-request-id"

	// Deprecated: use keyOfBotReasoningContent instead
	keyOfReasoningContent    = "ark-reasoning-content"
	keyOfBotReasoningContent = "ark-bot-reasoning-content"

	keyOfBotUsage = "ark-bot-usage"

	// Deprecated: use keyOfBotReferences instead
	keyOfReferences    = "ark-references"
	keyOfBotReferences = "ark-bot-references"
)

type arkRequestID string

func init() {
	compose.RegisterStreamChunkConcatFunc(func(chunks []arkRequestID) (final arkRequestID, err error) {
		if len(chunks) == 0 {
			return "", nil
		}

		return chunks[len(chunks)-1], nil
	})
	compose.RegisterStreamChunkConcatFunc(func(ts []*model.BotUsage) (*model.BotUsage, error) {
		ret := &model.BotUsage{}
		for _, t := range ts {
			if t == nil {
				continue
			}
			ret.ModelUsage = append(ret.ModelUsage, t.ModelUsage...)
			ret.ActionUsage = append(ret.ActionUsage, t.ActionUsage...)
		}
		return ret, nil
	})
	compose.RegisterStreamChunkConcatFunc(func(ts [][]*model.BotChatResultReference) ([]*model.BotChatResultReference, error) {
		var ret []*model.BotChatResultReference
		for _, t := range ts {
			ret = append(ret, t...)
		}
		return ret, nil
	})

	schema.RegisterName[arkRequestID]("_eino_ext_ark_bot_request_id")
	schema.RegisterName[*model.BotUsage]("_eino_ext_ark_bot_usage")
	schema.RegisterName[*model.BotChatResultReference]("_eino_ext_ark_bot_chat_result_reference")
	schema.RegisterName[*model.BotCoverImage]("_eino_ext_ark_bot_cover_image")
}

func setArkRequestID(msg *schema.Message, id string) {
	setMsgExtra(msg, keyOfBotRequestID, arkRequestID(id))
}

func GetArkRequestID(msg *schema.Message) string {
	reqID, ok := getMsgExtraValue[arkRequestID](msg, keyOfBotRequestID)
	if ok {
		return string(reqID)
	}
	reqID, _ = getMsgExtraValue[arkRequestID](msg, keyOfRequestID)
	return string(reqID)
}

func setReasoningContent(msg *schema.Message, rc string) {
	setMsgExtra(msg, keyOfBotReasoningContent, rc)
}

func GetReasoningContent(msg *schema.Message) (string, bool) {
	reason, ok := getMsgExtraValue[string](msg, keyOfBotReasoningContent)
	if ok {
		return reason, true
	}
	return getMsgExtraValue[string](msg, keyOfReasoningContent)
}

func setBotUsage(msg *schema.Message, bu *model.BotUsage) {
	setMsgExtra(msg, keyOfBotUsage, bu)
}

func GetBotUsage(msg *schema.Message) (*model.BotUsage, bool) {
	return getMsgExtraValue[*model.BotUsage](msg, keyOfBotUsage)
}

func setBotChatResultReference(msg *schema.Message, rc []*model.BotChatResultReference) {
	setMsgExtra(msg, keyOfBotReferences, rc)
}

func GetBotChatResultReference(msg *schema.Message) ([]*model.BotChatResultReference, bool) {
	ref, ok := getMsgExtraValue[[]*model.BotChatResultReference](msg, keyOfBotReferences)
	if ok {
		return ref, ok
	}
	// compatible with historical logic
	return getMsgExtraValue[[]*model.BotChatResultReference](msg, keyOfReferences)
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

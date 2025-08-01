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
	"encoding/gob"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
)

const (
	keyOfRequestID           = "ark-request-id"
	keyOfReasoningContent    = "ark-reasoning-content"
	keyOfBotUsage            = "ark-bot-usage"
	keyOfReferences          = "ark-references"
	keyOfGroupChatConfig     = "ark-group-chat-config"
	keyOfTargetCharacterName = "ark-target-character-name"
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

	_ = compose.RegisterSerializableType[arkRequestID]("_eino_ext_ark_request_id")
	gob.RegisterName("_eino_ext_ark_request_id", arkRequestID(""))
	_ = compose.RegisterSerializableType[model.BotUsage]("_eino_ext_ark_bot_usage")
	gob.RegisterName("_eino_ext_ark_bot_usage", &model.BotUsage{})
	_ = compose.RegisterSerializableType[model.BotChatResultReference]("_eino_ext_ark_bot_chat_result_reference")
	gob.RegisterName("_eino_ext_ark_bot_chat_result_reference", &model.BotChatResultReference{})
	_ = compose.RegisterSerializableType[model.BotCoverImage]("_eino_ext_ark_bot_cover_image")
	gob.RegisterName("_eino_ext_ark_bot_cover_image", &model.BotCoverImage{})
}

func setArkRequestID(msg *schema.Message, id string) {
	if msg == nil {
		return
	}
	if msg.Extra == nil {
		msg.Extra = map[string]interface{}{}
	}
	msg.Extra[keyOfRequestID] = arkRequestID(id)
}

func GetArkRequestID(msg *schema.Message) string {
	reqID, ok := msg.Extra[keyOfRequestID].(arkRequestID)
	if !ok {
		return ""
	}
	return string(reqID)
}

func setReasoningContent(msg *schema.Message, rc string) {
	if msg == nil {
		return
	}
	if msg.Extra == nil {
		msg.Extra = make(map[string]interface{})
	}
	msg.Extra[keyOfReasoningContent] = rc
}

func GetReasoningContent(msg *schema.Message) (string, bool) {
	reasoningContent, ok := msg.Extra[keyOfReasoningContent].(string)
	if !ok {
		return "", false
	}

	return reasoningContent, true
}

func setBotUsage(msg *schema.Message, bu *model.BotUsage) {
	if msg == nil {
		return
	}
	if msg.Extra == nil {
		msg.Extra = make(map[string]interface{})
	}
	msg.Extra[keyOfBotUsage] = bu
}

func GetBotUsage(msg *schema.Message) (*model.BotUsage, bool) {
	if msg == nil {
		return nil, false
	}
	bu, ok := msg.Extra[keyOfBotUsage].(*model.BotUsage)
	return bu, ok
}

func setBotChatResultReference(msg *schema.Message, rc []*model.BotChatResultReference) {
	if msg == nil {
		return
	}
	if msg.Extra == nil {
		msg.Extra = make(map[string]interface{})
	}
	msg.Extra[keyOfReferences] = rc
}

func GetBotChatResultReference(msg *schema.Message) ([]*model.BotChatResultReference, bool) {
	if msg == nil {
		return nil, false
	}
	ref, ok := msg.Extra[keyOfReferences].([]*model.BotChatResultReference)
	return ref, ok
}

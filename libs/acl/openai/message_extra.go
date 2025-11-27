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

package openai

import (
	"errors"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

const (
	keyOfReasoningContent     = "reasoning-content"
	extraKeyOfAudioID         = "openai-audio-id"
	extraKeyOfAudioTranscript = "openai_audio-transcript"
	keyOfRequestID            = "openai-request-id"
)

type audioID string
type openaiRequestID string

func init() {
	compose.RegisterStreamChunkConcatFunc(func(chunks []audioID) (final audioID, err error) {
		if len(chunks) == 0 {
			return "", nil
		}
		firstID := chunks[0]
		for i := 1; i < len(chunks); i++ {
			if chunks[i] != firstID {
				return "", errors.New("audio IDs are not consistent")
			}
		}

		return chunks[len(chunks)-1], nil
	})

	schema.RegisterName[audioID]("_eino_ext_openai_audio_id")

	compose.RegisterStreamChunkConcatFunc(func(chunks []openaiRequestID) (final openaiRequestID, err error) {
		if len(chunks) == 0 {
			return "", nil
		}
		return chunks[len(chunks)-1], nil
	})
	schema.RegisterName[openaiRequestID]("_eino_ext_openai_request_id")
}

func GetReasoningContent(msg *schema.Message) (string, bool) {
	return getMsgExtraValue[string](msg, keyOfReasoningContent)
}

func setReasoningContent(msg *schema.Message, reasoningContent string) {
	setMsgExtra(msg, keyOfReasoningContent, reasoningContent)
}

func setMessageOutputAudioID(audio *schema.MessageOutputAudio, ID audioID) {
	if len(ID) == 0 {
		return
	}
	if audio.Extra == nil {
		audio.Extra = make(map[string]interface{})
	}
	audio.Extra[extraKeyOfAudioID] = ID
}

func getMessageOutputAudioID(audio *schema.MessageOutputAudio) (audioID, bool) {
	if audio == nil {
		return "", false
	}
	id, ok := audio.Extra[extraKeyOfAudioID].(audioID)
	if !ok {
		return "", false
	}
	return id, true
}

func setMessageOutputAudioTranscript(audio *schema.MessageOutputAudio, transcript string) {
	if audio == nil || len(transcript) == 0 {
		return
	}
	if audio.Extra == nil {
		audio.Extra = make(map[string]interface{})
	}
	audio.Extra[extraKeyOfAudioTranscript] = transcript
}

func GetMessageOutputAudioTranscript(audio *schema.MessageOutputAudio) (string, bool) {
	if audio == nil {
		return "", false
	}
	transcript, ok := audio.Extra[extraKeyOfAudioTranscript].(string)
	if !ok {
		return "", false
	}
	return transcript, true
}

func setRequestID(msg *schema.Message, ID string) {
	setMsgExtra(msg, keyOfRequestID, openaiRequestID(ID))
}

func GetRequestID(msg *schema.Message) string {
	reqID, _ := getMsgExtraValue[openaiRequestID](msg, keyOfRequestID)
	return string(reqID)
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

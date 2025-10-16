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
)

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

type audioID string

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

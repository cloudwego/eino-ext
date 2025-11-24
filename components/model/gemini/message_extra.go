/*
 * Copyright 2024 CloudWeGo Authors
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

package gemini

import (
	"encoding/base64"

	"github.com/cloudwego/eino/schema"
	"google.golang.org/genai"
)

const (
	videoMetaDataKey    = "gemini_video_meta_data"
	thoughtSignatureKey = "gemini_thought_signature"
)

// Deprecated: use SetInputVideoMetaData or SetOutputVideoMetaData instead.
func SetVideoMetaData(part *schema.ChatMessageVideoURL, metaData *genai.VideoMetadata) {
	if part == nil {
		return
	}
	if part.Extra == nil {
		part.Extra = make(map[string]any)
	}
	setVideoMetaData(part.Extra, metaData)
}

// Deprecated: use GetInputVideoMetaData or GetOutputVideoMetaData instead.
func GetVideoMetaData(part *schema.ChatMessageVideoURL) *genai.VideoMetadata {
	if part == nil || part.Extra == nil {
		return nil
	}
	return getVideoMetaData(part.Extra)
}

func setInputVideoMetaData(part *schema.MessageInputVideo, metaData *genai.VideoMetadata) {
	if part == nil {
		return
	}
	if part.Extra == nil {
		part.Extra = make(map[string]any)
	}
	setVideoMetaData(part.Extra, metaData)
}

func GetInputVideoMetaData(part *schema.MessageInputVideo) *genai.VideoMetadata {
	if part == nil || part.Extra == nil {
		return nil
	}
	return getVideoMetaData(part.Extra)
}

func setVideoMetaData(extra map[string]any, metaData *genai.VideoMetadata) {
	extra[videoMetaDataKey] = metaData
}

func getVideoMetaData(extra map[string]any) *genai.VideoMetadata {
	if extra == nil {
		return nil
	}
	videoMetaData, ok := extra[videoMetaDataKey].(*genai.VideoMetadata)
	if !ok {
		return nil
	}
	return videoMetaData
}

// setMessageThoughtSignature stores the thought signature in the Message's Extra field.
// This is used for non-functionCall responses where the signature appears on text/inlineData parts.
//
// Thought signatures are encrypted representations of the model's internal thought process
// that preserve reasoning state during multi-turn conversations.
//
// For functionCall responses, use setToolCallThoughtSignature instead.
//
// See: https://cloud.google.com/vertex-ai/generative-ai/docs/thought-signatures
func setMessageThoughtSignature(message *schema.Message, signature []byte) {
	if message == nil || len(signature) == 0 {
		return
	}
	if message.Extra == nil {
		message.Extra = make(map[string]any)
	}
	message.Extra[thoughtSignatureKey] = signature
}

// getMessageThoughtSignature retrieves the thought signature from a Message's Extra field.
func getMessageThoughtSignature(message *schema.Message) []byte {
	if message == nil || message.Extra == nil {
		return nil
	}

	return getThoughtSignatureFromExtra(message.Extra)
}

// getThoughtSignatureFromExtra is a helper function that extracts thought signature from an Extra map.
func getThoughtSignatureFromExtra(extra map[string]any) []byte {
	if extra == nil {
		return nil
	}

	signature, exists := extra[thoughtSignatureKey]
	if !exists {
		return nil
	}

	switch sig := signature.(type) {
	case []byte:
		if len(sig) == 0 {
			return nil
		}
		return sig
	case string:
		if sig == "" {
			return nil
		}
		decoded, err := base64.StdEncoding.DecodeString(sig)
		if err != nil {
			return nil
		}
		return decoded
	default:
		return nil
	}
}

// setToolCallThoughtSignature stores the thought signature for a specific tool call
// in the ToolCall's Extra field.
//
// Per Gemini docs, thought signatures on functionCall parts are required for Gemini 3 Pro:
//   - For parallel function calls: only the first functionCall part contains the signature
//   - For sequential function calls: each functionCall part has its own signature
//   - Omitting a required signature results in a 400 error
//
// See: https://cloud.google.com/vertex-ai/generative-ai/docs/thought-signatures
func setToolCallThoughtSignature(toolCall *schema.ToolCall, signature []byte) {
	if toolCall == nil || len(signature) == 0 {
		return
	}
	if toolCall.Extra == nil {
		toolCall.Extra = make(map[string]any)
	}
	toolCall.Extra[thoughtSignatureKey] = signature
}

// getToolCallThoughtSignature retrieves the thought signature from a ToolCall's Extra field.
func getToolCallThoughtSignature(toolCall *schema.ToolCall) []byte {
	if toolCall == nil || toolCall.Extra == nil {
		return nil
	}
	return getThoughtSignatureFromExtra(toolCall.Extra)
}

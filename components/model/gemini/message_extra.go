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
	"strings"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"google.golang.org/genai"
)

func init() {
	compose.RegisterStreamChunkConcatFunc(func(chunks []*genai.ExecutableCode) (final *genai.ExecutableCode, err error) {
		if len(chunks) == 0 {
			return nil, nil
		}
		var lang genai.Language
		code := &strings.Builder{}
		for _, chunk := range chunks {
			if chunk == nil {
				continue
			}
			if len(chunk.Language) > 0 {
				lang = chunk.Language
			}
			if len(chunk.Code) > 0 {
				code.WriteString(chunk.Code)
			}
		}
		return &genai.ExecutableCode{
			Code:     code.String(),
			Language: lang,
		}, nil
	})
	schema.RegisterName[*genai.ExecutableCode]("_eino_ext_gemini_executalbe_code")

	compose.RegisterStreamChunkConcatFunc(func(chunks []*genai.CodeExecutionResult) (final *genai.CodeExecutionResult, err error) {
		if len(chunks) == 0 {
			return nil, nil
		}
		var outcome genai.Outcome
		output := &strings.Builder{}
		for _, chunk := range chunks {
			if chunk == nil {
				continue
			}
			if len(chunk.Outcome) > 0 {
				outcome = chunk.Outcome
			}
			if len(chunk.Output) > 0 {
				output.WriteString(chunk.Output)
			}
		}
		return &genai.CodeExecutionResult{
			Outcome: outcome,
			Output:  output.String(),
		}, nil
	})
	schema.RegisterName[*genai.CodeExecutionResult]("_eino_ext_gemini_code_execution_result")

	compose.RegisterStreamChunkConcatFunc(func(chunks []*genai.GroundingMetadata) (final *genai.GroundingMetadata, err error) {
		if len(chunks) == 0 {
			return nil, nil
		}
		ret := &genai.GroundingMetadata{}
		for _, chunk := range chunks {
			if chunk == nil {
				continue
			}
			ret.GoogleMapsWidgetContextToken += chunk.GoogleMapsWidgetContextToken
			ret.GroundingChunks = append(ret.GroundingChunks, chunk.GroundingChunks...)
			ret.GroundingSupports = append(ret.GroundingSupports, chunk.GroundingSupports...)
			if chunk.RetrievalMetadata != nil {
				ret.RetrievalMetadata = chunk.RetrievalMetadata
			}
			ret.RetrievalQueries = append(ret.RetrievalQueries, chunk.RetrievalQueries...)
			if chunk.SearchEntryPoint != nil {
				ret.SearchEntryPoint = chunk.SearchEntryPoint
			}
			ret.SourceFlaggingUris = append(ret.SourceFlaggingUris, chunk.SourceFlaggingUris...)
			ret.WebSearchQueries = append(ret.WebSearchQueries, chunk.WebSearchQueries...)
		}
		return ret, nil
	})
	schema.RegisterName[*genai.GroundingMetadata]("_eino_ext_gemini_ground_metadata")
}

const (
	videoMetaDataKey    = "gemini_video_meta_data"
	thoughtSignatureKey = "gemini_thought_signature"
	specialParteKey     = "gemini_special_part"
	groundMetadataKey   = "gemini_ground_metadata"
)

// Deprecated: use SetInputVideoMetaData instead.
func SetVideoMetaData(part *schema.ChatMessageVideoURL, metaData *genai.VideoMetadata) {
	if part == nil {
		return
	}
	if part.Extra == nil {
		part.Extra = make(map[string]any)
	}
	setVideoMetaData(part.Extra, metaData)
}

// Deprecated: use GetInputVideoMetaData instead.
func GetVideoMetaData(part *schema.ChatMessageVideoURL) *genai.VideoMetadata {
	if part == nil || part.Extra == nil {
		return nil
	}
	return getVideoMetaData(part.Extra)
}

func SetInputVideoMetaData(part *schema.MessageInputVideo, metaData *genai.VideoMetadata) {
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

func getMessageThoughtSignature(message *schema.Message) []byte {
	if message == nil {
		return nil
	}
	sig, _ := GetThoughtSignatureFromExtra(message.Extra)
	return sig
}

func setMessageOutputPartThoughtSignature(part *schema.MessageOutputPart, signature []byte) {
	if part == nil || len(signature) == 0 {
		return
	}
	if part.Extra == nil {
		part.Extra = make(map[string]any)
	}
	part.Extra[thoughtSignatureKey] = signature
}

// GetThoughtSignatureFromExtra tries to read thought_signature from an Extra map.
//
// thought_signature should be read from:
//   - message.AssistantGenMultiContent[i].Extra: thought_signature on each generated output part
//   - toolCall.Extra: thought_signature on toolCall
//   - message.Extra: thought_signature on generated content (legacy, only used when message.AssistantGenMultiContent are absent)
//
// The returned bool indicates whether thought_signature key exists in Extra.
// The returned []byte is the thought signature if available
func GetThoughtSignatureFromExtra(extra map[string]any) ([]byte, bool) {
	if extra == nil {
		return nil, false
	}

	signature, exists := extra[thoughtSignatureKey]
	if !exists {
		return nil, false
	}

	switch sig := signature.(type) {
	case []byte:
		if len(sig) == 0 {
			return nil, true
		}
		return sig, true
	case string:
		// When marshaling a map[string]any to JSON, a []byte value is encoded as a base64 string.
		// After unmarshaling back into map[string]any, the value becomes string.
		// Decode it here for compatibility with messages restored from JSON.
		if sig == "" {
			return nil, true
		}
		decoded, err := base64.StdEncoding.DecodeString(sig)
		if err != nil {
			return nil, true
		}
		if len(decoded) == 0 {
			return nil, true
		}
		return decoded, true
	default:
		return nil, true
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

func getToolCallThoughtSignature(toolCall *schema.ToolCall) []byte {
	if toolCall == nil {
		return nil
	}
	sig, _ := GetThoughtSignatureFromExtra(toolCall.Extra)
	return sig
}

func setGroundMetadata(m *schema.Message, gm *genai.GroundingMetadata) {
	if m == nil {
		return
	}
	if m.Extra == nil {
		m.Extra = make(map[string]any)
	}
	m.Extra[groundMetadataKey] = gm
}

func GetGroundMetadata(m *schema.Message) *genai.GroundingMetadata {
	if m == nil {
		return nil
	}
	if gm, ok := m.Extra[groundMetadataKey].(*genai.GroundingMetadata); ok {
		return gm
	}
	return nil
}

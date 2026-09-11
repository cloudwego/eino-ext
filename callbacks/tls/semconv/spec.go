/*
 * Copyright 2026 CloudWeGo Authors
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

package semconv

import "encoding/json"

// ModelInput is the input for model span, for tag key: input
type ModelInput struct {
	Messages        []*ModelMessage  `json:"messages,omitempty"`
	Tools           []*ModelTool     `json:"tools,omitempty"`
	ModelToolChoice *ModelToolChoice `json:"tool_choice,omitempty"`
}

// ModelOutput is the output for model span, for tag key: output
type ModelOutput struct {
	Choices []*ModelChoice `json:"choices"`
}

// ModelCallOption is the option for model span, for tag key: call_options
type ModelCallOption struct {
	Temperature      float32  `json:"temperature"`
	MaxTokens        int64    `json:"max_tokens,omitempty"`
	Stop             []string `json:"stop,omitempty"`
	TopP             float32  `json:"top_p,omitempty"`
	N                int64    `json:"n,omitempty"`
	TopK             *int64   `json:"top_k,omitempty"`
	PresencePenalty  *float32 `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float32 `json:"frequency_penalty,omitempty"`
	ReasoningEffort  string   `json:"reasoning_effort,omitempty"`
}

type ModelMessage struct {
	Role             string              `json:"role"`                        // from enum VRole in span_value
	Content          string              `json:"content,omitempty"`           // single content
	ReasoningContent string              `json:"reasoning_content,omitempty"` // only for output
	Parts            []*ModelMessagePart `json:"parts,omitempty"`             // multi-modality content
	Name             string              `json:"name,omitempty"`
	ToolCalls        []*ModelToolCall    `json:"tool_calls,omitempty"`
	ToolCallID       string              `json:"tool_call_id,omitempty"`
	Metadata         map[string]string   `json:"metadata,omitempty"`
}

type ModelMessagePart struct {
	Type     ModelMessagePartType `json:"type"` // Required. The type of the content.
	Text     string               `json:"text,omitempty"`
	ImageURL *ModelImageURL       `json:"image_url,omitempty"`
	AudioURL *ModelMediaURL       `json:"audio_url,omitempty"`
	VideoURL *ModelMediaURL       `json:"video_url,omitempty"`
	FileURL  *ModelFileURL        `json:"file_url,omitempty"`
}

type ModelMessagePartType string

var (
	ModelMessagePartTypeText      ModelMessagePartType = "text"
	ModelMessagePartTypeImage     ModelMessagePartType = "image_url"
	ModelMessagePartTypeAudio     ModelMessagePartType = "audio_url"
	ModelMessagePartTypeVideo     ModelMessagePartType = "video_url"
	ModelMessagePartTypeFile      ModelMessagePartType = "file_url"
	ModelMessagePartTypeReasoning ModelMessagePartType = "reasoning"
)

type ModelImageURL struct {
	Name     string `json:"name,omitempty"`
	URL      string `json:"url,omitempty"`
	Detail   string `json:"detail,omitempty"`
	MIMEType string `json:"mime_type,omitempty"`
}

// ModelMediaURL is the common representation used for audio and video parts.
type ModelMediaURL struct {
	URL      string `json:"url,omitempty"`
	MIMEType string `json:"mime_type,omitempty"`
}

type ModelFileURL struct {
	Name     string `json:"name,omitempty"`
	URL      string `json:"url,omitempty"`
	Detail   string `json:"detail,omitempty"`
	Suffix   string `json:"suffix,omitempty"`
	MIMEType string `json:"mime_type,omitempty"`
}

type ModelToolCall struct {
	ID       string                 `json:"id,omitempty"`
	Type     string                 `json:"type,omitempty"` // Always be: "function"
	Function *ModelToolCallFunction `json:"function"`
}

type ModelToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments,omitempty"`
}

type ModelTool struct {
	Type     string             `json:"type"` // Always be: "function"
	Function *ModelToolFunction `json:"function"`
}

type ModelToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type ModelChoice struct {
	FinishReason string        `json:"finish_reason"`
	Index        int64         `json:"index"`
	Message      *ModelMessage `json:"message"`
}

type ModelToolChoice struct {
	Type     string                 `json:"type"`               // from enum VToolChoice in span_value
	Function *ModelToolCallFunction `json:"function,omitempty"` // field name only.
}

// PromptInput is the input of prompt span, for tag key: input
type PromptInput struct {
	Templates []*ModelMessage   `json:"templates"`
	Arguments []*PromptArgument `json:"arguments"`
}

type PromptArgument struct {
	Key       string                  `json:"key"`
	Value     any                     `json:"value"`
	Source    string                  `json:"source"` // from enum VPromptArgSource in span_value.go
	ValueType PromptArgumentValueType `json:"value_type"`
}

type PromptArgumentValueType string

var (
	PromptArgumentValueTypeText         PromptArgumentValueType = "text"
	PromptArgumentValueTypeModelMessage PromptArgumentValueType = "model_message"
	PromptArgumentValueTypeMessagePart  PromptArgumentValueType = "model_message_part"
)

// PromptOutput is the output of prompt span, for tag key: output
type PromptOutput struct {
	Prompts []*ModelMessage `json:"prompts"`
}

type RetrieverInput struct {
	Query string `json:"query,omitempty"`
}

type RetrieverOutput struct {
	Documents []*RetrieverDocument `json:"documents,omitempty"`
}

type RetrieverDocument struct {
	ID      string    `json:"id,omitempty"`
	Index   string    `json:"index,omitempty"`
	Content string    `json:"content"`
	Vector  []float64 `json:"vector,omitempty"`
	Score   float64   `json:"score"`
}

type RetrieverCallOption struct {
	TopK     int64    `json:"top_k,omitempty"`
	MinScore *float64 `json:"min_score,omitempty"`
	Filter   string   `json:"filter,omitempty"`
}

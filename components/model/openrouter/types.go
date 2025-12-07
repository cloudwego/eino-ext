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

package openrouter

import (
	"github.com/eino-contrib/jsonschema"
)

type ChatCompletionResponseFormatType string

const (
	ChatCompletionResponseFormatTypeJSONObject ChatCompletionResponseFormatType = "json_object"
	ChatCompletionResponseFormatTypeJSONSchema ChatCompletionResponseFormatType = "json_schema"
	ChatCompletionResponseFormatTypeText       ChatCompletionResponseFormatType = "text"
)

type ChatCompletionResponseFormat struct {
	Type       ChatCompletionResponseFormatType        `json:"type,omitempty"`
	JSONSchema *ChatCompletionResponseFormatJSONSchema `json:"json_schema,omitempty"`
}

type ChatCompletionResponseFormatJSONSchema struct {
	Name        string             `json:"name"`
	Description string             `json:"description,omitempty"`
	JSONSchema  *jsonschema.Schema `json:"schema"`
	Strict      bool               `json:"strict"`
}

type Effort string

const (
	EffortOfNone    Effort = "none"
	EffortOfMinimal Effort = "minimal"
	EffortOfLow     Effort = "low"
	EffortOfMedium  Effort = "medium"
	EffortOfHigh    Effort = "high"
)

type Summary string

const (
	SummaryOfAuto     Summary = "auto"
	SummaryOfConcise  Summary = "concise"
	SummaryOfDetailed Summary = "detailed"
)

// Reasoning configures reasoning capabilities across different models.
// See documentation for each field to understand model support and behavior differences.
// Reference: https://openrouter.ai/docs/guides/best-practices/reasoning-tokens
type Reasoning struct {
	// Effort controls the reasoning strength level.
	Effort Effort `json:"effort,omitempty"`
	// Summary specifies whether and how reasoning should be summarized.
	Summary Summary `json:"summary,omitempty"`

	// MaxTokens directly specifies the maximum tokens to allocate for reasoning.
	// For models that only support effort-based reasoning, this value determines
	// the appropriate effort level. See: https://openrouter.ai/docs/guides/best-practices/reasoning-tokens
	MaxTokens int `json:"maxTokens,omitempty"`

	// Exclude indicates whether reasoning should occur internally but not appear
	// in the response. When true, reasoning tokens appear in the "reasoning"
	// field of each message.
	Exclude bool `json:"exclude,omitempty"`

	// Enabled explicitly enables or disables reasoning capabilities.
	Enabled *bool `json:"enabled,omitempty"`
}

type image struct {
	Type     string `json:"type"`
	ImageURL struct {
		URL string `json:"url"`
	} `json:"image_url"`
	Index int64 `json:"index"`
}

type message struct {
	Role             string              `json:"role,omitempty"`
	Content          string              `json:"content,omitempty"`
	Reasoning        string              `json:"reasoning,omitempty"`
	ReasoningDetails []*reasoningDetails `json:"reasoning_details,omitempty"`
	Images           []*image            `json:"images,omitempty"`
}

type responseChoice struct {
	Index   int64    `json:"index,omitempty"`
	Message *message `json:"message,omitempty"`
	Delta   *message `json:"delta,omitempty"`
}

type reasoningDetails struct {
	Format string `json:"format,omitempty"`
	Index  int64  `json:"index,omitempty"`
	Type   string `json:"type,omitempty"`
	Data   string `json:"data,omitempty"`
	Text   string `json:"text,omitempty"`
}

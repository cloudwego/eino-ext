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
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
)

type SessionCacheConfig struct {
	// EnableCache controls whether session caching is active.
	// When enabled, the model stores both inputs and responses for each conversation turn,
	// allowing them to be retrieved later via API.
	// Response IDs are saved in output messages and can be accessed using GetResponseID.
	// For multi-turn conversations, the ARK ChatModel automatically identifies the most recent
	// cached message from all inputs and passes its response ID to model to maintain context continuity.
	// This message and all previous ones are trimmed before being sent to the model.
	// When both HeadPreviousResponseID and cached message exist, the message's response ID takes precedence.
	// Use InvalidateMessageCaches to disables caching for the specified messages.
	EnableCache bool `json:"enable_cache"`

	// TTL specifies the survival time of cached data in seconds, with a maximum of 3 * 86400(3 days).
	TTL int `json:"ttl"`
}

type APIType string

const (
	// ContextAPI is defined from  https://www.volcengine.com/docs/82379/1528789
	ContextAPI APIType = "context_api"
	// ResponsesAPI is defined from https://www.volcengine.com/docs/82379/1569618
	// Deprecated: Use NewResponsesAPIChatModel to create a model for the ResponsesAPIChatModel.
	ResponsesAPI APIType = "responses_api"
)

type ResponseFormat struct {
	Type       model.ResponseFormatType                       `json:"type"`
	JSONSchema *model.ResponseFormatJSONSchemaJSONSchemaParam `json:"json_schema,omitempty"`
}

type caching string

const (
	cachingEnabled  caching = "enabled"
	cachingDisabled caching = "disabled"
)

const (
	callbackExtraKeyThinking      = "thinking"
	callbackExtraKeyPreResponseID = "ark-previous-response-id"
	callbackExtraModelName        = "model_name"
)

type toolChoice string

const (
	toolChoiceNone     toolChoice = "none"
	toolChoiceAuto     toolChoice = "auto"
	toolChoiceRequired toolChoice = "required"
)

// Source specifies the additional content source for web searches.
// Optional sources are Toutiao, Douyin , and Moji Weather.
//   - toutiao: Additional content source from Toutiao for web searches.
//   - douyin: Additional content source from Douyin for web searches.
//   - moji: Additional content source from Moji Weather for web searches.
type Source string

const (
	SourceOfToutiao = "toutiao"
	SourceOfDouyin  = "douyin"
	SourceOfMoji    = "moji"
)

// ToolWebSearch holds the configuration for the web search tool.
type ToolWebSearch struct {
	// Limit is the maximum number of results to retrieve per search in a single round.
	// It affects input size and performance. The value must be in the range [1, 50].
	// Optional. Default 10
	Limit *int64 `json:"limit,omitempty"`

	// UserLocation is the user's geographical location, used for scenarios like weather queries.
	// It includes `type`, `country`, `city`, and `region` fields.
	UserLocation *UserLocation `json:"user_location,omitempty"`

	// Sources is a list of additional content sources for web searches. See the Source type for available options.
	Sources []Source `json:"sources,omitempty"`

	// MaxKeyword is the maximum number of keywords to search in parallel within a single tool-use round.
	// For example, if the model identifies multiple keywords to search (e.g., "A", "B", "C")
	// and max_keyword is 1, only the first keyword ("A") will be searched.
	// The value must be in the range [1, 50].
	// Optional.
	MaxKeyword *int32 `json:"max_keyword,omitempty"`
}

// UserLocation is the user's geographical location, used for scenarios like weather queries.
// It includes `country`, `city`, `region` and `timezone` fields.
type UserLocation struct {
	City     *string `json:"city,omitempty"`
	Country  *string `json:"country,omitempty"`
	Region   *string `json:"region,omitempty"`
	Timezone *string `json:"timezone,omitempty"`
}

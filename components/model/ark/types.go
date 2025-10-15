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
	EnableCache bool `json:"enable_cache"`

	// TTL specifies the survival time of cached data in seconds, with a maximum of 3 * 86400(3 days).
	TTL int `json:"ttl"`
}

type APIType string

const (
	// ContextAPI is defined from  https://www.volcengine.com/docs/82379/1528789
	ContextAPI APIType = "context_api"
	// ResponsesAPI is defined from https://www.volcengine.com/docs/82379/1569618
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
	callbackExtraKeyThinking = "thinking"
)

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

package ark

import (
	"fmt"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/responses"
)

const (
	keyResponseID      = "ark-response-id"
	keyCacheExpiration = "ark-response-cache-expire-at"
	keyResponsesUsage  = "ark-responses-usage-details"
	keyResponsesOutput = "ark-responses-output"
)

// SessionCacheConfig enables storing Responses and reusing their server context.
// TTL is in seconds. Zero leaves expiration to the service's default.
type SessionCacheConfig struct {
	EnableCache bool `json:"enable_cache"`
	TTL         int  `json:"ttl"`
}

// CacheOption controls Responses context reuse. A valid cached message takes
// precedence over HeadPreviousResponseID; only later messages are then sent.
type CacheOption struct {
	HeadPreviousResponseID *string
	SessionCache           *SessionCacheConfig
}

type CacheInfo struct {
	ResponseID string
	Usage      schema.TokenUsage
}

func GetResponseID(msg *schema.Message) (string, bool) { return extraString(msg, keyResponseID) }
func GetCacheExpiration(msg *schema.Message) (int64, bool) {
	return extraInt64(msg, keyCacheExpiration)
}

// InvalidateMessageCaches prevents automatic cache reuse for these messages.
func InvalidateMessageCaches(messages []*schema.Message) error {
	for _, msg := range messages {
		if msg != nil {
			delete(msg.Extra, keyCacheExpiration)
		}
	}
	return nil
}

// GetResponsesTokenUsageDetails combines the current ResponseMeta.Usage with
// modality and built-in tool usage in Extra. It returns false when
// ResponseMeta.Usage is absent.
func GetResponsesTokenUsageDetails(msg *schema.Message) (responses.ResponseUsage, bool) {
	return readResponsesUsage(msg)
}

// GetResponsesOutput returns all final SDK output items, including built-in tool
// results and annotations that are not represented by Eino's common fields.
func GetResponsesOutput(msg *schema.Message) ([]responses.OutputItem, bool) {
	return readJSONExtra[[]responses.OutputItem](msg, keyResponsesOutput)
}

func getResponsesReasoning(msg *schema.Message) ([]responses.ItemReasoning, bool) {
	var value []responses.ItemReasoning
	if output, ok := GetResponsesOutput(msg); ok {
		for _, item := range output {
			if reasoning, ok := item.OneOf.GetItemReasoning(); ok {
				value = append(value, reasoning)
			}
		}
	}
	if len(value) == 0 {
		return nil, false
	}
	// Respect caller edits instead of restoring stale reasoning from Extra.
	if responsesReasoningText(value) != msg.ReasoningContent {
		return nil, false
	}
	return value, true
}

func populateResponsesCache(in []*schema.Message, req *responses.ResponsesRequest, config *CacheOption) ([]*schema.Message, error) {
	if config == nil {
		return in, nil
	}
	now := time.Now().Unix()
	if config.HeadPreviousResponseID != nil {
		if *config.HeadPreviousResponseID == "" {
			return nil, fmt.Errorf("HeadPreviousResponseID must not be empty")
		}
		req.PreviousResponseID = responses.NewOptString(*config.HeadPreviousResponseID)
	}
	cache := config.SessionCache
	if cache == nil {
		return in, nil
	}
	if cache.TTL < 0 {
		return nil, fmt.Errorf("session cache TTL must not be negative")
	}
	mode := responses.CacheModeDisabled
	if cache.EnableCache {
		mode = responses.CacheModeEnabled
	}
	req.Store = responses.NewOptBool(cache.EnableCache)
	req.Caching = responses.NewOptResponseCaching(responses.ResponseCaching{Type: responses.NewOptCacheMode(mode)})
	if cache.TTL > 0 {
		req.ExpireAt = responses.NewOptInt64(now + int64(cache.TTL))
	}
	if !cache.EnableCache {
		return in, nil
	}
	for i := len(in) - 1; i >= 0; i-- {
		expires, ok := GetCacheExpiration(in[i])
		if !ok || expires <= now {
			continue
		}
		id, ok := GetResponseID(in[i])
		if !ok || id == "" {
			continue
		}
		if i+1 == len(in) {
			return nil, fmt.Errorf("no incremental input after cached response")
		}
		req.PreviousResponseID = responses.NewOptString(id)
		return in[i+1:], nil
	}
	return in, nil
}

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
	"reflect"
	"time"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/responses"
)

const (
	keyResponseID         = "ark-response-id"
	keyCacheExpiration    = "ark-response-cache-expire-at"
	keyResponsesReasoning = "ark-responses-reasoning"
	keyResponsesUsage     = "ark-responses-usage-details"
	keyResponsesOutput    = "ark-responses-output"
)

type cacheExpiration int64
type responsesReasoning []responses.ItemReasoning
type responsesUsage responses.ResponseUsage
type responsesOutput []responses.OutputItem

func init() {
	schema.RegisterName[cacheExpiration]("_eino_ext_ark_v2_cache_expiration")
	schema.RegisterName[responsesReasoning]("_eino_ext_ark_v2_responses_reasoning")
	schema.RegisterName[responsesUsage]("_eino_ext_ark_v2_responses_usage")
	schema.RegisterName[responsesOutput]("_eino_ext_ark_v2_responses_output")
	compose.RegisterStreamChunkConcatFunc(lastValue[cacheExpiration])
	compose.RegisterStreamChunkConcatFunc(lastValue[responsesReasoning])
	compose.RegisterStreamChunkConcatFunc(lastValue[responsesUsage])
	compose.RegisterStreamChunkConcatFunc(lastValue[responsesOutput])
}

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
	if msg == nil {
		return 0, false
	}
	v := reflect.ValueOf(msg.Extra[keyCacheExpiration])
	if v.IsValid() && v.Kind() == reflect.Int64 {
		return v.Int(), true
	}
	// Support messages restored from plain JSON too.
	if value, ok := msg.Extra[keyCacheExpiration].(float64); ok {
		return int64(value), true
	}
	return 0, false
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

// GetResponsesTokenUsageDetails returns modality and built-in tool usage.
func GetResponsesTokenUsageDetails(msg *schema.Message) (responses.ResponseUsage, bool) {
	if msg == nil {
		return responses.ResponseUsage{}, false
	}
	value, ok := msg.Extra[keyResponsesUsage].(responsesUsage)
	return responses.ResponseUsage(value), ok
}

// GetResponsesOutput returns all final SDK output items, including built-in tool
// results and annotations that are not represented by Eino's common fields.
func GetResponsesOutput(msg *schema.Message) ([]responses.OutputItem, bool) {
	if msg == nil {
		return nil, false
	}
	value, ok := msg.Extra[keyResponsesOutput].(responsesOutput)
	return []responses.OutputItem(value), ok
}

func getResponsesReasoning(msg *schema.Message) ([]responses.ItemReasoning, bool) {
	value, ok := msg.Extra[keyResponsesReasoning].(responsesReasoning)
	if !ok && msg.Extra[keyResponsesReasoning] != nil {
		// Plain JSON restoration does not retain registered Go types.
		raw, err := sonic.Marshal(msg.Extra[keyResponsesReasoning])
		if err != nil || sonic.Unmarshal(raw, &value) != nil {
			return nil, false
		}
		ok = true
	}
	if !ok {
		return nil, false
	}
	// Respect caller edits instead of restoring stale reasoning from Extra.
	if responsesReasoningText(value) != msg.ReasoningContent {
		return nil, false
	}
	return []responses.ItemReasoning(value), true
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

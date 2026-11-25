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
	keyResponseID      = extraKeyPrefix + "response-id"
	keyCacheExpiration = extraKeyPrefix + "response-cache-expire-at"
)

// SessionCacheConfig enables storing Responses and reusing their server context.
// TTL is in seconds. Zero leaves expiration to the service's default.
type SessionCacheConfig struct {
	EnableCache bool `json:"enable_cache"`
	TTL         int  `json:"ttl"`
}

// CacheOption controls Responses context reuse. A valid cached message takes
// precedence over HeadPreviousResponseID; only later messages are then sent.
// Continuations inherit the initial tool set, including name filtering. Start
// a new cache chain to change that set or apply a new ToolChoice. Continuations
// omit both tools and tool_choice; local options do not replace cached settings.
type CacheOption struct {
	HeadPreviousResponseID *string
	SessionCache           *SessionCacheConfig
}

type CacheInfo struct {
	ResponseID string
	Usage      schema.TokenUsage
}

// ResponseCacheRef identifies a stored response and its expiration in Unix seconds.
type ResponseCacheRef struct {
	ResponseID string
	ExpireAt   int64
}

// GetResponseCacheRef reads a stored Responses API result's ID and expiration.
// It returns false for nil messages or when either field is missing. An expired
// reference remains readable, but cannot be used for automatic continuation.
func GetResponseCacheRef(msg *schema.Message) (ResponseCacheRef, bool) {
	if msg == nil {
		return ResponseCacheRef{}, false
	}
	id, ok := extraString(msg.Extra, keyResponseID)
	if !ok || id == "" {
		return ResponseCacheRef{}, false
	}
	expires, ok := extraInt64(msg.Extra, keyCacheExpiration)
	if !ok {
		return ResponseCacheRef{}, false
	}
	return ResponseCacheRef{ResponseID: id, ExpireAt: expires}, true
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

func populateResponsesCache(in []*schema.Message, req *responses.ResponsesRequest, config CacheOption) ([]*schema.Message, error) {
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
		ref, ok := GetResponseCacheRef(in[i])
		if !ok || ref.ExpireAt <= now {
			continue
		}
		if i+1 == len(in) {
			return nil, fmt.Errorf("no incremental input after cached response")
		}
		req.PreviousResponseID = responses.NewOptString(ref.ResponseID)
		return in[i+1:], nil
	}
	return in, nil
}

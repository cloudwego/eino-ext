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

package claude

import (
	"github.com/cloudwego/eino/components/model"
)

type options struct {
	TopK *int32

	Thinking *Thinking

	DisableParallelToolUse *bool

	EnableAutoCache *bool
	AutoCacheTTL    CacheTTL
}

func WithTopK(k int32) model.Option {
	return model.WrapImplSpecificOptFn(func(o *options) {
		o.TopK = &k
	})
}

func WithThinking(t *Thinking) model.Option {
	return model.WrapImplSpecificOptFn(func(o *options) {
		o.Thinking = t
	})
}

func WithDisableParallelToolUse() model.Option {
	return model.WrapImplSpecificOptFn(func(o *options) {
		b := true
		o.DisableParallelToolUse = &b
	})
}

// AutoCacheOption is a functional option for configuring auto cache behavior.
type AutoCacheOption func(*autoCacheOptions)

type autoCacheOptions struct {
	TTL CacheTTL
}

// WithAutoCacheTTL sets the TTL for automatically placed cache breakpoints.
func WithAutoCacheTTL(ttl CacheTTL) AutoCacheOption {
	return func(o *autoCacheOptions) {
		o.TTL = ttl
	}
}

// WithEnableAutoCache enables automatic caching in a multi-turn conversation.
// The caching strategy sets separate breakpoints for tool and system messages.
// Additionally, a breakpoint is set on the last input message of each turn to cache the session.
// Use WithAutoCacheTTL to control the cache duration.
func WithEnableAutoCache(enabled bool, opts ...AutoCacheOption) model.Option {
	return model.WrapImplSpecificOptFn(func(o *options) {
		o.EnableAutoCache = &enabled
		ao := &autoCacheOptions{}
		for _, opt := range opts {
			opt(ao)
		}
		o.AutoCacheTTL = ao.TTL
	})
}

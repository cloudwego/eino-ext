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
	"time"

	"github.com/eino-contrib/jsonschema"
	"google.golang.org/genai"

	"github.com/cloudwego/eino/components/model"
)

type options struct {
	TopK                        *int32
	ResponseJSONSchema          *jsonschema.Schema
	ThinkingConfig              *genai.ThinkingConfig
	ResponseModalities          []GeminiResponseModality
	ImageConfig                 *genai.ImageConfig
	CachedContentName           string
	PrefixCacheTTL              *time.Duration
	PrefixCacheExpireTime       *time.Time
	PrefixCacheUpdateTTL        *time.Duration
	PrefixCacheUpdateExpireTime *time.Time
}

func WithTopK(k int32) model.Option {
	return model.WrapImplSpecificOptFn(func(o *options) {
		o.TopK = &k
	})
}

func WithResponseJSONSchema(s *jsonschema.Schema) model.Option {
	return model.WrapImplSpecificOptFn(func(o *options) {
		o.ResponseJSONSchema = s
	})
}

func WithThinkingConfig(t *genai.ThinkingConfig) model.Option {
	return model.WrapImplSpecificOptFn(func(o *options) {
		o.ThinkingConfig = t
	})
}

func WithResponseModalities(m []GeminiResponseModality) model.Option {
	return model.WrapImplSpecificOptFn(func(o *options) {
		o.ResponseModalities = m
	})
}

// WithCachedContentName the name of the content cached to use as context to serve the prediction.
// Format: cachedContents/{cachedContent}
func WithCachedContentName(name string) model.Option {
	return model.WrapImplSpecificOptFn(func(o *options) {
		o.CachedContentName = name
	})
}

// WithPrefixCacheTTL sets the TTL for CreatePrefixCache on this call.
// When set, it overrides Config.Cache.TTL for that CreatePrefixCache invocation.
func WithPrefixCacheTTL(ttl time.Duration) model.Option {
	return model.WrapImplSpecificOptFn(func(o *options) {
		o.PrefixCacheTTL = &ttl
	})
}

// WithPrefixCacheExpireTime sets the absolute expiry for CreatePrefixCache on this call.
// When set, it overrides Config.Cache.ExpireTime for that CreatePrefixCache invocation.
func WithPrefixCacheExpireTime(expireTime time.Time) model.Option {
	return model.WrapImplSpecificOptFn(func(o *options) {
		o.PrefixCacheExpireTime = &expireTime
	})
}

// WithPrefixCacheUpdateTTL sets the TTL for UpdatePrefixCache on this call.
// When set, it overrides Config.Cache.TTL for that UpdatePrefixCache invocation.
func WithPrefixCacheUpdateTTL(ttl time.Duration) model.Option {
	return model.WrapImplSpecificOptFn(func(o *options) {
		o.PrefixCacheUpdateTTL = &ttl
	})
}

// WithPrefixCacheUpdateExpireTime sets the absolute expiry for UpdatePrefixCache on this call.
// When set, it overrides Config.Cache.ExpireTime for that UpdatePrefixCache invocation.
func WithPrefixCacheUpdateExpireTime(expireTime time.Time) model.Option {
	return model.WrapImplSpecificOptFn(func(o *options) {
		o.PrefixCacheUpdateExpireTime = &expireTime
	})
}

// WithImageConfig sets the image generation configuration.
// Note: an error will be returned for a model that does not support the configuration options.
// Optional.
func WithImageConfig(cfg *genai.ImageConfig) model.Option {
	return model.WrapImplSpecificOptFn(func(o *options) {
		o.ImageConfig = cfg
	})
}

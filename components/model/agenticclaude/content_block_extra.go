/*
 * Copyright 2026 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package agenticclaude

import (
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/schema"
)

const (
	keyOfWebSearchToolResultCaller = "_agenticclaude_web_search_tool_result_caller"
	keyOfWebFetchToolResultCaller  = "_agenticclaude_web_fetch_tool_result_caller"
	keyOfCacheControlTTL           = "_agenticclaude_cache_control_ttl"
)

func setWebSearchResultCaller(block *schema.ContentBlock, caller anthropic.WebSearchToolResultBlockCallerUnion) {
	setContentBlockExtraValue(block, keyOfWebSearchToolResultCaller, caller.RawJSON())
}

func toWebSearchResultCallerParam(block *schema.ContentBlock) (param anthropic.WebSearchToolResultBlockParamCallerUnion, err error) {
	caller, ok := getContentBlockExtraValue[string](block, keyOfWebSearchToolResultCaller)
	if !ok || caller == "" {
		return param, nil
	}
	return param, sonic.UnmarshalString(caller, &param)
}

func setWebFetchResultCaller(block *schema.ContentBlock, caller anthropic.WebFetchToolResultBlockCallerUnion) {
	setContentBlockExtraValue(block, keyOfWebFetchToolResultCaller, caller.RawJSON())
}

func toWebFetchResultCallerParam(block *schema.ContentBlock) (param anthropic.WebFetchToolResultBlockParamCallerUnion, err error) {
	caller, ok := getContentBlockExtraValue[string](block, keyOfWebFetchToolResultCaller)
	if !ok || caller == "" {
		return param, nil
	}
	return param, sonic.UnmarshalString(caller, &param)
}

func setContentBlockExtraValue[T any](block *schema.ContentBlock, key string, value T) {
	if block == nil {
		return
	}
	if block.Extra == nil {
		block.Extra = map[string]any{}
	}
	block.Extra[key] = value
}

func getContentBlockExtraValue[T any](block *schema.ContentBlock, key string) (T, bool) {
	var zero T
	if block == nil || block.Extra == nil {
		return zero, false
	}
	value, ok := block.Extra[key].(T)
	if !ok {
		return zero, false
	}
	return value, true
}

func copyExtra(src map[string]any) map[string]any {
	if src == nil {
		return map[string]any{}
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// SetToolInfoCacheControl sets a cache control on a tool info.
// When a manual control is set, the top-level auto-cache will not override it.
func SetToolInfoCacheControl(toolInfo *schema.ToolInfo, ctrl *anthropic.CacheControlEphemeralParam) *schema.ToolInfo {
	if toolInfo == nil {
		return nil
	}
	if ctrl == nil {
		return toolInfo
	}
	ti := *toolInfo
	ti.Extra = copyExtra(toolInfo.Extra)
	ti.Extra[keyOfCacheControlTTL] = string(ctrl.TTL)
	return &ti
}

func hasCacheControlOnToolInfo(toolInfo *schema.ToolInfo) bool {
	if toolInfo == nil || toolInfo.Extra == nil {
		return false
	}
	_, ok := toolInfo.Extra[keyOfCacheControlTTL]
	return ok
}

func getToolInfoCacheControl(toolInfo *schema.ToolInfo) *anthropic.CacheControlEphemeralParam {
	if toolInfo == nil || toolInfo.Extra == nil {
		return nil
	}
	ttl, ok := toolInfo.Extra[keyOfCacheControlTTL].(string)
	if !ok {
		return nil
	}
	p := anthropic.NewCacheControlEphemeralParam()
	if ttl != "" {
		p.TTL = anthropic.CacheControlEphemeralTTL(ttl)
	}
	return &p
}

// SetContentBlockCacheControl sets a cache control on a content block.
// When a manual control is set, the top-level auto-cache will not override it.
func SetContentBlockCacheControl(block *schema.ContentBlock, ctrl *anthropic.CacheControlEphemeralParam) *schema.ContentBlock {
	if block == nil {
		return nil
	}
	if ctrl == nil {
		return block
	}
	b := *block
	b.Extra = copyExtra(block.Extra)
	b.Extra[keyOfCacheControlTTL] = string(ctrl.TTL)
	return &b
}

func hasCacheControlOnContentBlock(block *schema.ContentBlock) bool {
	if block == nil || block.Extra == nil {
		return false
	}
	_, ok := block.Extra[keyOfCacheControlTTL]
	return ok
}

func getContentBlockCacheControl(block *schema.ContentBlock) *anthropic.CacheControlEphemeralParam {
	if block == nil || block.Extra == nil {
		return nil
	}
	ttl, ok := block.Extra[keyOfCacheControlTTL].(string)
	if !ok {
		return nil
	}
	p := anthropic.NewCacheControlEphemeralParam()
	if ttl != "" {
		p.TTL = anthropic.CacheControlEphemeralTTL(ttl)
	}
	return &p
}

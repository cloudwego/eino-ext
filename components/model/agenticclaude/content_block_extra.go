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
	extraKeyWebSearchToolResultCaller = "_claude_web_search_tool_result_caller"
	extraKeyWebFetchToolResultCaller  = "_claude_web_fetch_tool_result_caller"
)

func setWebSearchResultCaller(block *schema.ContentBlock, caller anthropic.WebSearchToolResultBlockCallerUnion) {
	setContentBlockExtraValue(block, extraKeyWebSearchToolResultCaller, caller.RawJSON())
}

func toWebSearchResultCallerParam(block *schema.ContentBlock) (param anthropic.WebSearchToolResultBlockParamCallerUnion, err error) {
	caller, ok := getContentBlockExtraValue[string](block, extraKeyWebSearchToolResultCaller)
	if !ok || caller == "" {
		return param, nil
	}
	return param, sonic.UnmarshalString(caller, &param)
}

func setWebFetchResultCaller(block *schema.ContentBlock, caller anthropic.WebFetchToolResultBlockCallerUnion) {
	setContentBlockExtraValue(block, extraKeyWebFetchToolResultCaller, caller.RawJSON())
}

func toWebFetchResultCallerParam(block *schema.ContentBlock) (param anthropic.WebFetchToolResultBlockParamCallerUnion, err error) {
	caller, ok := getContentBlockExtraValue[string](block, extraKeyWebFetchToolResultCaller)
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

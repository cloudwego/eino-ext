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

package claude

import (
	"github.com/anthropics/anthropic-sdk-go"

	"github.com/cloudwego/eino/schema"
)

// CacheTTL is a type alias for the Anthropic SDK's cache control TTL.
// Supported values: CacheTTL5m ("5m", default) and CacheTTL1h ("1h").
type CacheTTL = anthropic.CacheControlEphemeralTTL

const (
	// CacheTTL5m sets the cache TTL to 5 minutes (default).
	CacheTTL5m CacheTTL = anthropic.CacheControlEphemeralTTLTTL5m
	// CacheTTL1h sets the cache TTL to 1 hour.
	CacheTTL1h CacheTTL = anthropic.CacheControlEphemeralTTLTTL1h
)

// CacheControl configures cache control behavior for manual cache breakpoints.
// A nil CacheControl or zero-value TTL means the SDK default (5 minutes) is used.
type CacheControl struct {
	TTL CacheTTL
}

const (
	keyOfThinking          = "_eino_claude_thinking"
	keyOfBreakPoint        = "_eino_claude_breakpoint"
	keyOfBreakPointTTL     = "_eino_claude_breakpoint_ttl"
	keyOfThinkingSignature = "_eino_claude_thinking_signature"
)

func GetThinking(msg *schema.Message) (string, bool) {
	reasoningContent, ok := getMsgExtraValue[string](msg, keyOfThinking)
	return reasoningContent, ok
}

func setThinking(msg *schema.Message, reasoningContent string) {
	setMsgExtra(msg, keyOfThinking, reasoningContent)
}

// Deprecated: Use SetMessageCacheControl instead.
func SetMessageBreakpoint(msg *schema.Message) *schema.Message {
	return SetMessageCacheControl(msg, nil)
}

// Deprecated: Use SetToolInfoCacheControl instead.
func SetToolInfoBreakpoint(toolInfo *schema.ToolInfo) *schema.ToolInfo {
	return SetToolInfoCacheControl(toolInfo, nil)
}

// SetMessageCacheControl sets a cache breakpoint on the message with the given cache control settings.
// A nil CacheControl or zero-value TTL means the SDK default (5 minutes) is used.
func SetMessageCacheControl(msg *schema.Message, ctrl *CacheControl) *schema.Message {
	msg_ := *msg
	msg_.Extra = copyExtra(msg.Extra)

	setMsgExtra(&msg_, keyOfBreakPoint, true)

	if ctrl != nil && ctrl.TTL != "" {
		setMsgExtra(&msg_, keyOfBreakPointTTL, string(ctrl.TTL))
	}

	return &msg_
}

// SetToolInfoCacheControl sets a cache breakpoint on the tool info with the given cache control settings.
// A nil CacheControl or zero-value TTL means the SDK default (5 minutes) is used.
func SetToolInfoCacheControl(toolInfo *schema.ToolInfo, ctrl *CacheControl) *schema.ToolInfo {
	toolInfo_ := *toolInfo
	toolInfo_.Extra = copyExtra(toolInfo.Extra)

	setToolInfoExtra(&toolInfo_, keyOfBreakPoint, true)

	if ctrl != nil && ctrl.TTL != "" {
		setToolInfoExtra(&toolInfo_, keyOfBreakPointTTL, string(ctrl.TTL))
	}

	return &toolInfo_
}

func copyExtra(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func getMsgExtraValue[T any](msg *schema.Message, key string) (T, bool) {
	if msg == nil {
		var t T
		return t, false
	}
	val, ok := msg.Extra[key].(T)
	return val, ok
}

func setMsgExtra(msg *schema.Message, key string, value any) {
	if msg == nil {
		return
	}
	if msg.Extra == nil {
		msg.Extra = make(map[string]any)
	}
	msg.Extra[key] = value
}

func getToolInfoExtraValue[T any](toolInfo *schema.ToolInfo, key string) (T, bool) {
	if toolInfo == nil {
		var t T
		return t, false
	}
	val, ok := toolInfo.Extra[key].(T)
	return val, ok
}

func setToolInfoExtra(toolInfo *schema.ToolInfo, key string, value any) {
	if toolInfo == nil {
		return
	}
	if toolInfo.Extra == nil {
		toolInfo.Extra = make(map[string]any)
	}
	toolInfo.Extra[key] = value
}

func isBreakpointTool(toolInfo *schema.ToolInfo) bool {
	isBreakpoint, _ := getToolInfoExtraValue[bool](toolInfo, keyOfBreakPoint)
	return isBreakpoint
}

func isBreakpointMessage(msg *schema.Message) bool {
	isBreakpoint, _ := getMsgExtraValue[bool](msg, keyOfBreakPoint)
	return isBreakpoint
}

func getThinkingSignature(msg *schema.Message) (string, bool) {
	signature, ok := getMsgExtraValue[string](msg, keyOfThinkingSignature)
	return signature, ok
}

func setThinkingSignature(msg *schema.Message, signature string) {
	setMsgExtra(msg, keyOfThinkingSignature, signature)
}

func getMessageBreakpointCacheControl(msg *schema.Message) *CacheControl {
	ttl, _ := getMsgExtraValue[string](msg, keyOfBreakPointTTL)
	if ttl == "" {
		return nil
	}
	return &CacheControl{TTL: CacheTTL(ttl)}
}

func getToolBreakpointCacheControl(toolInfo *schema.ToolInfo) *CacheControl {
	ttl, _ := getToolInfoExtraValue[string](toolInfo, keyOfBreakPointTTL)
	if ttl == "" {
		return nil
	}
	return &CacheControl{TTL: CacheTTL(ttl)}
}

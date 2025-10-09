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
	"github.com/cloudwego/eino/schema"
)

const (
	keyOfThinking          = "_eino_claude_thinking"
	keyOfBreakPoint        = "_eino_claude_breakpoint"
	keyOfThinkingSignature = "_eino_claude_thinking_signature"
)

func GetThinking(msg *schema.Message) (string, bool) {
	reasoningContent, ok := getMsgExtraValue[string](msg, keyOfThinking)
	return reasoningContent, ok
}

func setThinking(msg *schema.Message, reasoningContent string) {
	setMsgExtra(msg, keyOfThinking, reasoningContent)
}

func SetMessageBreakpoint(msg *schema.Message) *schema.Message {
	msg_ := *msg

	extra := make(map[string]any, len(msg.Extra))
	for k, v := range msg.Extra {
		extra[k] = v
	}

	msg_.Extra = extra

	setMsgExtra(&msg_, keyOfBreakPoint, true)

	return &msg_
}

func SetToolInfoBreakpoint(toolInfo *schema.ToolInfo) *schema.ToolInfo {
	toolInfo_ := *toolInfo

	extra := make(map[string]any, len(toolInfo.Extra))
	for k, v := range toolInfo.Extra {
		extra[k] = v
	}

	toolInfo_.Extra = extra

	setToolInfoExtra(&toolInfo_, keyOfBreakPoint, true)

	return &toolInfo_
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

func GetThinkingSignature(msg *schema.Message) (string, bool) {
	signature, ok := getMsgExtraValue[string](msg, keyOfThinkingSignature)
	return signature, ok
}

func SetThinkingSignature(msg *schema.Message, signature string) {
	setMsgExtra(msg, keyOfThinkingSignature, signature)
}

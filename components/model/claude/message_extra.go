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
	keyOfThinking = "_eino_claude_thinking"
)

func GetThinking(msg *schema.Message) (string, bool) {
	if msg == nil {
		return "", false
	}
	reasoningContent, ok := msg.Extra[keyOfThinking].(string)
	if !ok {
		return "", false
	}

	return reasoningContent, true
}

func setThinking(msg *schema.Message, reasoningContent string) {
	if msg == nil {
		return
	}
	if msg.Extra == nil {
		msg.Extra = make(map[string]interface{})
	}
	msg.Extra[keyOfThinking] = reasoningContent
}

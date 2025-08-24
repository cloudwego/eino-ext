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

package deepseek

import (
	"github.com/cloudwego/eino/schema"
)

const (
	extraKeyReasoningContent = "_eino_deepseek_reasoning_content"
	extraKeyPrefix           = "_eino_deepseek_prefix"
)

func SetReasoningContent(message *schema.Message, content string) {
	if message == nil {
		return
	}
	if message.Extra == nil {
		message.Extra = make(map[string]interface{})
	}
	message.Extra[extraKeyReasoningContent] = content
}

func GetReasoningContent(message *schema.Message) (string, bool) {
	if message == nil || message.Extra == nil {
		return "", false
	}
	result, ok := message.Extra[extraKeyReasoningContent].(string)
	return result, ok
}

func SetPrefix(message *schema.Message) {
	if message == nil {
		return
	}
	if message.Extra == nil {
		message.Extra = make(map[string]interface{})
	}
	message.Extra[extraKeyPrefix] = true
}

func HasPrefix(message *schema.Message) bool {
	if message == nil || message.Extra == nil {
		return false
	}
	_, ok := message.Extra[extraKeyPrefix].(bool)
	return ok
}

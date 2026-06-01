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

package agenticgemini

import (
	"github.com/cloudwego/eino/schema"
)

const (
	thoughtSignatureExtraKey = "_eino_ext_agentic_gemini_thought_signature"
)

func setThoughtSignature(cb *schema.ContentBlock, ts []byte) {
	if cb == nil {
		return
	}
	if cb.Extra == nil {
		cb.Extra = make(map[string]interface{})
	}
	cb.Extra[thoughtSignatureExtraKey] = ts
}

func getThoughtSignature(cb *schema.ContentBlock) []byte {
	if cb == nil {
		return nil
	}
	if cb.Extra == nil {
		cb.Extra = make(map[string]interface{})
	}
	if v, ok := cb.Extra[thoughtSignatureExtraKey].([]byte); ok {
		return v
	}
	return nil
}

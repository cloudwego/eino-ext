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

import "github.com/cloudwego/eino/schema"

const toolStrictKey = "ark-tool-strict"

// WithToolStrict returns a copy of tool with its function strict setting. The
// setting overrides ChatModelConfig.ToolStrict for this tool in both WithTools
// and model.WithTools. The original ToolInfo and its Extra map are unchanged.
func WithToolStrict(tool *schema.ToolInfo, strict bool) *schema.ToolInfo {
	if tool == nil {
		return nil
	}
	toolCopy := *tool
	toolCopy.Extra = make(map[string]any, len(tool.Extra)+1)
	for key, value := range tool.Extra {
		toolCopy.Extra[key] = value
	}
	toolCopy.Extra[toolStrictKey] = strict
	return &toolCopy
}

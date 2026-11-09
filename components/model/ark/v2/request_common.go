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

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/schema"
)

// Developer is the role for developer instructions sent to either Ark API.
const Developer schema.RoleType = "developer"

type mediaKind string

const (
	mediaKindImage mediaKind = "image"
	mediaKindAudio mediaKind = "audio"
	mediaKindVideo mediaKind = "video"
	mediaKindFile  mediaKind = "file"
)

// Validate content ownership without converting inputs and outputs to one type.
func validateMessageContent(msg *schema.Message) error {
	if len(msg.UserInputMultiContent) > 0 && len(msg.AssistantGenMultiContent) > 0 {
		return fmt.Errorf("a message cannot contain both UserInputMultiContent and AssistantGenMultiContent")
	}
	if len(msg.UserInputMultiContent) > 0 && msg.Role != schema.User && msg.Role != schema.Tool && msg.Role != schema.System && msg.Role != Developer {
		return fmt.Errorf("input multi content only supports user, tool, system and developer roles, got %s", msg.Role)
	}
	if len(msg.AssistantGenMultiContent) > 0 && msg.Role != schema.Assistant {
		return fmt.Errorf("assistant gen multi content only supports assistant role, got %s", msg.Role)
	}
	return nil
}

// Select URL, then raw Base64Data, then FileID. Validate only the selected source.
func mediaSource(c schema.MessagePartCommon, kind mediaKind) (value, fileID string, base64 bool, err error) {
	if c.URL != nil && *c.URL != "" {
		return *c.URL, "", false, nil
	}
	if c.Base64Data != nil && *c.Base64Data != "" {
		raw := *c.Base64Data
		if strings.HasPrefix(raw, "data:") {
			return "", "", true, fmt.Errorf("%s Base64Data must be raw base64, without a data URL prefix", kind)
		}
		if c.MIMEType == "" {
			return "", "", true, fmt.Errorf("%s part must have MIMEType when using Base64Data", kind)
		}
		return raw, "", true, nil
	}
	if c.FileID != nil && *c.FileID != "" {
		return "", *c.FileID, false, nil
	}
	return "", "", false, fmt.Errorf("%s part must contain URL, Base64Data or FileID", kind)
}

func mediaDataURL(mimeType, data string) string {
	return "data:" + mimeType + ";base64," + data
}

const toolStrictKey = "ark-tool-strict"

// WithToolStrict returns a copy of tool with its function strict setting. The
// setting overrides the model's ToolStrict default for this tool in both WithTools
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

func toolStrict(ti *schema.ToolInfo, fallback *bool) (*bool, error) {
	if value, exists := ti.Extra[toolStrictKey]; exists {
		override, ok := value.(bool)
		if !ok {
			return nil, fmt.Errorf("tool %s strict must be a bool, got %T", ti.Name, value)
		}
		return &override, nil
	}
	return fallback, nil
}

func toolParametersJSON(ti *schema.ToolInfo) ([]byte, error) {
	if ti.ParamsOneOf == nil {
		return nil, nil
	}
	params, err := ti.ParamsOneOf.ToJSONSchema()
	if err != nil {
		return nil, fmt.Errorf("tool %s parameters: %w", ti.Name, err)
	}
	raw, err := sonic.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("tool %s schema: %w", ti.Name, err)
	}
	return raw, nil
}

func selectToolInfos(tools []*schema.ToolInfo, choice *schema.ToolChoice, names []string) ([]*schema.ToolInfo, error) {
	if choice == nil || len(names) == 0 || (*choice != schema.ToolChoiceAllowed && *choice != schema.ToolChoiceForced) {
		return tools, nil
	}
	allowed := make(map[string]bool, len(names))
	for _, name := range names {
		allowed[name] = false
	}
	selected := make([]*schema.ToolInfo, 0, len(tools))
	for _, tool := range tools {
		if tool == nil {
			return nil, fmt.Errorf("tool info cannot be nil")
		}
		if _, ok := allowed[tool.Name]; ok {
			selected = append(selected, tool)
			allowed[tool.Name] = true
		}
	}
	for _, name := range names {
		if !allowed[name] {
			return nil, fmt.Errorf("allowed tool name %q not found in tools list", name)
		}
	}
	return selected, nil
}

// Preserve the decimal JSON representation when widening a float32 setting.
func float64FromFloat32(value float32) float64 {
	result, _ := strconv.ParseFloat(strconv.FormatFloat(float64(value), 'g', -1, 32), 64)
	return result
}

func newPanicErr(info any, stack []byte) error {
	return fmt.Errorf("panic error: %v, stack: %s", info, stack)
}

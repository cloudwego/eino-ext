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

package openaigo

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"
	"github.com/openai/openai-go/v3/packages/param"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"
)

// Assistant messages are previous model outputs. The Responses API is strict:
// when role=assistant, content parts must be of type "output_text"/"refusal",
// not "input_text".
//
// The openai-go SDK's easiest compatible representation is to send assistant
// content as a plain string (not a typed content-part list).
// We therefore:
//   - allow text-only assistant history (as string)
//   - reject non-text assistant multimodal content when re-sending history
func extractAssistantTextForHistory(msg *schema.Message) (text string, ok bool, err error) {
	if msg == nil {
		return "", false, nil
	}

	// Prefer the canonical Content field.
	if msg.Content != "" {
		return msg.Content, true, nil
	}

	// If Content is empty, attempt to derive text from multi-content.
	// If any non-text part exists, we fail fast to avoid producing invalid request bodies.
	if len(msg.AssistantGenMultiContent) > 0 {
		var b strings.Builder
		for _, part := range msg.AssistantGenMultiContent {
			if part.Type != schema.ChatMessagePartTypeText {
				return "", false, fmt.Errorf("assistant history contains non-text part (%s); cannot re-send as Responses API input", part.Type)
			}
			if part.Text == "" {
				continue
			}
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			b.WriteString(part.Text)
		}
		if b.Len() > 0 {
			return b.String(), true, nil
		}
	}

	// Deprecated MultiContent.
	if len(msg.MultiContent) > 0 {
		var b strings.Builder
		for _, c := range msg.MultiContent {
			if c.Type != schema.ChatMessagePartTypeText {
				return "", false, fmt.Errorf("assistant history contains deprecated MultiContent non-text part (%s); cannot re-send", c.Type)
			}
			if c.Text == "" {
				continue
			}
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			b.WriteString(c.Text)
		}
		if b.Len() > 0 {
			return b.String(), true, nil
		}
	}

	// Do not attempt to re-send UserInputMultiContent on assistant messages.
	if len(msg.UserInputMultiContent) > 0 {
		return "", false, fmt.Errorf("assistant history contains UserInputMultiContent; cannot re-send as Responses API input")
	}

	return "", false, nil
}

func toolNameExists(tools []responses.ToolUnionParam, name string) bool {
	for _, t := range tools {
		if t.OfFunction != nil && t.OfFunction.Name == name {
			return true
		}
	}
	return false
}

// The OpenAI Responses API requires strict tool schemas to include:
//   - type: "object"
//   - properties: {...}
//   - additionalProperties: false
//   - required: [all keys in properties]
//
// Many JSON Schema generators omit "required" for fields tagged with `omitempty`.
func enforceOpenAIStrictJSONSchema(schema map[string]any) {
	if schema == nil {
		return
	}

	// Recurse into nested schemas first.
	if items, ok := schema["items"]; ok {
		switch v := items.(type) {
		case map[string]any:
			enforceOpenAIStrictJSONSchema(v)
		case []any:
			for _, it := range v {
				if m, ok := it.(map[string]any); ok {
					enforceOpenAIStrictJSONSchema(m)
				}
			}
		}
	}
	if props, ok := schema["properties"].(map[string]any); ok {
		for _, pv := range props {
			if pm, ok := pv.(map[string]any); ok {
				enforceOpenAIStrictJSONSchema(pm)
			}
		}
	}
	if oneOf, ok := schema["oneOf"].([]any); ok {
		for _, ov := range oneOf {
			if om, ok := ov.(map[string]any); ok {
				enforceOpenAIStrictJSONSchema(om)
			}
		}
	}
	if anyOf, ok := schema["anyOf"].([]any); ok {
		for _, av := range anyOf {
			if am, ok := av.(map[string]any); ok {
				enforceOpenAIStrictJSONSchema(am)
			}
		}
	}
	if allOf, ok := schema["allOf"].([]any); ok {
		for _, av := range allOf {
			if am, ok := av.(map[string]any); ok {
				enforceOpenAIStrictJSONSchema(am)
			}
		}
	}

	// Now enforce strictness for object schemas.
	props, ok := schema["properties"].(map[string]any)
	if !ok || len(props) == 0 {
		return
	}

	// Ensure type is object (some generators omit it at the top level).
	if _, ok := schema["type"]; !ok {
		schema["type"] = "object"
	}

	// OpenAI strict schema expects additionalProperties=false.
	if _, ok := schema["additionalProperties"]; !ok {
		schema["additionalProperties"] = false
	}

	// Ensure required includes *all* keys in properties.
	existing := map[string]struct{}{}
	if req, ok := schema["required"]; ok {
		switch v := req.(type) {
		case []any:
			for _, it := range v {
				if s, ok := it.(string); ok {
					existing[s] = struct{}{}
				}
			}
		case []string:
			for _, s := range v {
				existing[s] = struct{}{}
			}
		}
	}

	required := make([]any, 0, len(props))
	for k := range props {
		if _, ok := existing[k]; !ok {
			existing[k] = struct{}{}
		}
	}
	for k := range existing {
		required = append(required, k)
	}

	// If there were no required keys produced for some reason, at least include all properties.
	if len(required) == 0 {
		for k := range props {
			required = append(required, k)
		}
	}

	schema["required"] = required
}

func jsonSchemaToMap(s any) (map[string]any, error) {
	if s == nil {
		return map[string]any{}, nil
	}
	// jsonschema.Schema has json tags; encode/decode to map[string]any.
	b, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func commonToDataOrURL(common schema.MessagePartCommon) (string, error) {
	if common.URL == nil && common.Base64Data == nil {
		return "", fmt.Errorf("message part must have URL or Base64Data")
	}
	if common.URL != nil {
		return *common.URL, nil
	}
	if common.MIMEType == "" {
		return "", fmt.Errorf("message part must have MIMEType when using Base64Data")
	}
	if strings.HasPrefix(*common.Base64Data, "data:") {
		return "", fmt.Errorf("base64Data must be raw base64 without 'data:' prefix")
	}
	return fmt.Sprintf("data:%s;base64,%s", common.MIMEType, *common.Base64Data), nil
}

func joinReasoningText(item responses.ResponseReasoningItem) string {
	// Summary is often what people want.
	if len(item.Summary) > 0 {
		var b strings.Builder
		for i, s := range item.Summary {
			if s.Text == "" {
				continue
			}
			if i > 0 {
				b.WriteString("\n\n")
			}
			b.WriteString(s.Text)
		}
		out := b.String()
		if out != "" {
			return out
		}
	}

	if len(item.Content) > 0 {
		var b strings.Builder
		for i, c := range item.Content {
			if c.Text == "" {
				continue
			}
			if i > 0 {
				b.WriteString("\n\n")
			}
			b.WriteString(c.Text)
		}
		return b.String()
	}

	return ""
}

func ensureResponseMeta(meta *schema.ResponseMeta) *schema.ResponseMeta {
	if meta == nil {
		return &schema.ResponseMeta{}
	}
	return meta
}

func responsesModelFromString(s string) responses.ResponsesModel { return shared.ResponsesModel(s) }

func optInt64(v param.Opt[int64]) int64 {
	if v.Valid() {
		return v.Value
	}
	return 0
}

func optFloat64(v param.Opt[float64]) float64 {
	if v.Valid() {
		return v.Value
	}
	return 0
}

const callbackExtraModelName = "model_name"

type panicErr struct {
	info  any
	stack []byte
}

func (p *panicErr) Error() string {
	return fmt.Sprintf("panic error: %v, \nstack: %s", p.info, string(p.stack))
}

func newPanicErr(info any, stack []byte) error {
	return &panicErr{info: info, stack: stack}
}

func cloneStringMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneAnyMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

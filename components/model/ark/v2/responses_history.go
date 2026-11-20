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
	"sort"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/responses"
)

func hasResponsesItemMetadata(part *schema.MessageOutputPart) bool {
	_, exists := part.Extra[keyResponsesItemType]
	return exists
}

// Native item indices order the projected items and ToolCalls. Within each
// message item, content follows the order of AssistantGenMultiContent.
// Without Ark item metadata, leave the entire message to the standard mapping.
func toResponsesHistoryInput(msg *schema.Message) ([]responses.InputItem, bool, error) {
	partsByItem := make(map[int][]*schema.MessageOutputPart)
	var indices []int
	var standardParts []schema.MessageOutputPart
	for i := range msg.AssistantGenMultiContent {
		part := &msg.AssistantGenMultiContent[i]
		index, exists, err := extraIndex(part.Extra, keyResponsesItemIndex)
		if err != nil {
			return nil, true, fmt.Errorf("part %d: %w", i, err)
		}
		if !exists {
			standardParts = append(standardParts, *part)
			continue
		}
		if _, exists := partsByItem[index]; !exists {
			indices = append(indices, index)
		}
		partsByItem[index] = append(partsByItem[index], part)
	}
	native := len(partsByItem) > 0
	for _, call := range msg.ToolCalls {
		if _, exists := call.Extra[keyResponsesItemType]; exists {
			native = true
		}
	}
	if !native {
		return nil, false, nil
	}
	callsByItem := make(map[int]*schema.ToolCall)
	var standardCalls []schema.ToolCall
	for i := range msg.ToolCalls {
		call := &msg.ToolCalls[i]
		index, exists, err := extraIndex(call.Extra, keyResponsesItemIndex)
		if err != nil {
			return nil, true, fmt.Errorf("tool %d: %w", i, err)
		}
		if !exists {
			if _, native := call.Extra[keyResponsesItemType]; native {
				return nil, true, fmt.Errorf("tool %d has no Responses item index", i)
			}
			standardCalls = append(standardCalls, *call)
			continue
		}
		if callsByItem[index] != nil {
			return nil, true, fmt.Errorf("Responses item %d has duplicate tool calls", index)
		}
		callsByItem[index] = call
		if _, exists := partsByItem[index]; !exists {
			indices = append(indices, index)
		}
	}
	sort.Ints(indices)
	var result []responses.InputItem
	for _, index := range indices {
		item, err := toResponsesHistoryItem(partsByItem[index], callsByItem[index])
		if err != nil {
			return nil, true, fmt.Errorf("Responses item %d: %w", index, err)
		}
		if item != nil {
			result = append(result, *item)
		}
	}
	// Parts and calls without native indices use only their standard fields.
	items, err := toResponsesAssistantInput(&schema.Message{AssistantGenMultiContent: standardParts, ToolCalls: standardCalls})
	if err != nil {
		return nil, true, err
	}
	return append(result, items...), true, nil
}

func toResponsesHistoryItem(parts []*schema.MessageOutputPart, call *schema.ToolCall) (*responses.InputItem, error) {
	if call != nil {
		if len(parts) != 0 {
			return nil, fmt.Errorf("tool call overlaps content parts")
		}
		return toResponsesHistoryToolCall(call)
	}
	var metadataPart *schema.MessageOutputPart
	var content []*schema.MessageOutputPart
	for _, part := range parts {
		if hasResponsesItemMetadata(part) {
			if metadataPart != nil {
				return nil, fmt.Errorf("duplicate item metadata")
			}
			metadataPart = part
		}
		_, section := part.Extra[keyResponsesSection]
		_, indexed := part.Extra[keyResponsesContentIndex]
		if part.Type != ResponsesOutputItem || section || indexed {
			content = append(content, part)
		}
	}
	var extra map[string]any
	var kind string
	if metadataPart != nil {
		extra = metadataPart.Extra
		var ok bool
		kind, ok = extraString(extra, keyResponsesItemType)
		if !ok {
			return nil, fmt.Errorf("missing Responses item type")
		}
	} else {
		return nil, fmt.Errorf("content has no item metadata")
	}
	var sum responses.InputItemSum
	switch kind {
	case "message":
		item, err := toResponsesHistoryMessage(extra, content)
		if err != nil {
			return nil, err
		}
		sum.SetItemEasyMessage(item)
	case "reasoning":
		item, err := toResponsesHistoryReasoning(extra, content)
		if err != nil {
			return nil, err
		}
		sum.SetItemReasoning(item)
	default:
		// Output-only items have no SDK input member. Server-side MCP and tool
		// search are outside this adapter's supported workflows. Keep these SDK
		// values in Message parts for inspection, without constructing input.
		return nil, nil
	}
	return &responses.InputItem{OneOf: sum}, nil
}

func toResponsesHistoryToolCall(call *schema.ToolCall) (*responses.InputItem, error) {
	extra := call.Extra
	if _, exists := extra[keyResponsesItemType]; exists {
		kind, ok := extraString(extra, keyResponsesItemType)
		if !ok || kind != "function_call" {
			return nil, fmt.Errorf("invalid tool call item type")
		}
	}
	if call.Type != "" && call.Type != "function" {
		return nil, fmt.Errorf("unsupported tool call type %q", call.Type)
	}
	id, err := responsesMetadataString(extra, keyResponsesItemID)
	if err != nil {
		return nil, err
	}
	status, err := responsesMetadataString(extra, keyResponsesItemStatus)
	if err != nil {
		return nil, err
	}
	item := responses.ItemFunctionToolCall{Type: responses.ItemFunctionToolCallTypeFunctionCall,
		ID: id, CallID: responseString(call.ID), Name: responseString(call.Function.Name), Arguments: responseString(call.Function.Arguments)}
	item.Namespace, err = responsesMetadataString(extra, keyResponsesNamespace)
	if err != nil {
		return nil, err
	}
	if status.IsSet() {
		item.Status = responses.NewOptItemStatus(responses.ItemStatus(status.Value))
	}
	var sum responses.InputItemSum
	sum.SetItemFunctionToolCall(item)
	return &responses.InputItem{OneOf: sum}, nil
}

func toResponsesHistoryMessage(extra map[string]any, parts []*schema.MessageOutputPart) (responses.ItemEasyMessage, error) {
	item := responses.ItemEasyMessage{Type: responses.NewOptItemEasyMessageType(responses.ItemEasyMessageTypeMessage),
		Role: responses.NewOptMessageRole(responses.MessageRoleAssistant)}
	var err error
	item.ID, err = responsesMetadataString(extra, keyResponsesItemID)
	if err != nil {
		return item, err
	}
	if value, exists := extra[keyResponsesPartial]; exists && value != nil {
		partial, ok := boolValue(value)
		if !ok {
			return item, fmt.Errorf("Extra[%q] must be boolean", keyResponsesPartial)
		}
		item.Partial = responses.NewOptBool(partial)
	}
	// Only SDK input fields are mapped; output-only attributes are omitted.
	content := make([]responses.ContentItem, 0, len(parts))
	for _, part := range parts {
		kind, _ := extraString(part.Extra, keyResponsesContentType)
		text, annotations, err := responsesHistoryText(part)
		if err != nil {
			return item, err
		}
		if kind != "output_text" || part.Type != schema.ChatMessagePartTypeText {
			return item, fmt.Errorf("Responses message content must be output_text with a text part")
		}
		var sum responses.ContentItemSum
		sum.SetOutputContentItemText(responses.OutputContentItemText{Type: responses.OutputContentItemTextTypeOutputText, Text: text, Annotations: annotations})
		content = append(content, responses.ContentItem{OneOf: sum})
	}
	item.Content = responses.NewContentItemArrayMessageContent(content)
	return item, nil
}

func toResponsesHistoryReasoning(extra map[string]any, parts []*schema.MessageOutputPart) (responses.ItemReasoning, error) {
	if len(parts) != 1 || parts[0].Type != schema.ChatMessagePartTypeReasoning {
		return responses.ItemReasoning{}, fmt.Errorf("Responses reasoning item must contain exactly one reasoning part")
	}
	// A content type belongs to an individual SDK segment, not the public item.
	if _, typed := parts[0].Extra[keyResponsesContentType]; typed {
		return responses.ItemReasoning{}, fmt.Errorf("unexpected content type on reasoning item")
	}
	item, err := responsesReasoningPart(parts[0])
	if err != nil {
		return item, err
	}
	item.ID, err = responsesMetadataString(extra, keyResponsesItemID)
	if err != nil {
		return item, err
	}
	status, err := responsesMetadataString(extra, keyResponsesItemStatus)
	if err != nil {
		return item, err
	}
	if status.IsSet() {
		item.Status = responses.ItemStatus(status.Value)
	}
	return item, nil
}

func responsesHistoryText(part *schema.MessageOutputPart) (responses.OptString, []responses.Annotation, error) {
	if part.Type != schema.ChatMessagePartTypeText {
		return responses.OptString{}, nil, fmt.Errorf("expected output text part")
	}
	text := part.Text
	var annotations []responses.Annotation
	if value, exists := part.Extra[keyResponsesAnnotations]; exists {
		raw, ok := stringValue(value)
		if !ok {
			return responses.OptString{}, nil, fmt.Errorf("Extra[%q] must contain JSON", keyResponsesAnnotations)
		}
		// AnnotationSum explicitly preserves unknown variants in the SDK codec.
		if err := sonic.UnmarshalString(raw, &annotations); err != nil {
			return responses.OptString{}, nil, err
		}
	}
	return responses.NewOptString(text), annotations, nil
}

func responsesMetadataString(extra map[string]any, key string) (responses.OptString, error) {
	value, exists := extra[key]
	if !exists || value == nil {
		return responses.OptString{}, nil
	}
	text, ok := stringValue(value)
	if !ok {
		return responses.OptString{}, fmt.Errorf("Extra[%q] must be a string", key)
	}
	return responses.NewOptString(text), nil
}

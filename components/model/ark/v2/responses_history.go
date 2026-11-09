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

// Native item/content indices preserve the order of the projected parts and
// ToolCalls. Build SDK input items directly; output JSON is never replayed.
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
	if len(partsByItem) == 0 {
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
	kind := "function_call"
	if metadataPart != nil {
		extra = metadataPart.Extra
		var ok bool
		kind, ok = extraString(extra, keyResponsesItemType)
		if !ok {
			return nil, fmt.Errorf("missing Responses item type")
		}
	} else if len(content) != 0 || call == nil {
		return nil, fmt.Errorf("content has no item metadata")
	}
	if call != nil && kind != "function_call" {
		return nil, fmt.Errorf("tool call overlaps %q item", kind)
	}
	var sum responses.InputItemSum
	switch kind {
	case "function_call":
		if call == nil {
			return nil, nil // Deleting the ToolCall removes its function input.
		}
		if len(content) != 0 {
			return nil, fmt.Errorf("function call has content parts")
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
		sum.SetItemFunctionToolCall(item)
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
	summary, ordered, err := responsesHistorySections(parts, false)
	if err != nil {
		return item, err
	}
	if len(summary) > 0 {
		return item, fmt.Errorf("message cannot contain reasoning summaries")
	}
	content := make([]responses.ContentItem, 0, len(ordered))
	for _, part := range ordered {
		kind, _ := extraString(part.Extra, keyResponsesContentType)
		text, annotations, err := responsesHistoryText(part)
		if err != nil {
			return item, err
		}
		var sum responses.ContentItemSum
		switch kind {
		case "output_text":
			sum.SetOutputContentItemText(responses.OutputContentItemText{Type: responses.OutputContentItemTextTypeOutputText, Text: text, Annotations: annotations})
		case "reasoning_text":
			sum.SetOutputContentItemReasoningText(responses.OutputContentItemReasoningText{Type: responses.OutputContentItemReasoningTextTypeReasoningText, Text: text, Annotations: annotations})
		default:
			return item, fmt.Errorf("unsupported Responses input content type %q", kind)
		}
		content = append(content, responses.ContentItem{OneOf: sum})
	}
	item.Content = responses.NewContentItemArrayMessageContent(content)
	return item, nil
}

func toResponsesHistoryReasoning(extra map[string]any, parts []*schema.MessageOutputPart) (responses.ItemReasoning, error) {
	item := responses.ItemReasoning{Type: responses.ItemReasoningTypeReasoning, Status: responses.ItemStatusCompleted}
	var err error
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
	summary, content, err := responsesHistorySections(parts, true)
	if err != nil {
		return item, err
	}
	for _, part := range summary {
		item.Summary = append(item.Summary, responses.ReasoningSummaryPart{Type: responses.ReasoningSummaryPartTypeSummaryText, Text: responses.NewOptString(part.Reasoning.Text)})
	}
	for _, part := range content {
		kind, _ := extraString(part.Extra, keyResponsesContentType)
		text, annotations, err := responsesHistoryText(part)
		if err != nil {
			return item, err
		}
		var sum responses.OutputContentItemSum
		switch kind {
		case "output_text":
			sum.SetOutputContentItemText(responses.OutputContentItemText{Type: responses.OutputContentItemTextTypeOutputText, Text: text, Annotations: annotations})
		case "reasoning_text":
			sum.SetOutputContentItemReasoningText(responses.OutputContentItemReasoningText{Type: responses.OutputContentItemReasoningTextTypeReasoningText, Text: text, Annotations: annotations})
		default:
			return item, fmt.Errorf("unsupported reasoning input content type %q", kind)
		}
		item.Content = append(item.Content, responses.OutputContentItem{OneOf: sum})
	}
	var signature string
	for _, part := range parts {
		if part.Type != schema.ChatMessagePartTypeReasoning || part.Reasoning == nil {
			continue
		}
		if value := part.Reasoning.Signature; value != "" {
			if signature != "" && signature != value {
				return item, fmt.Errorf("conflicting reasoning signatures")
			}
			signature = value
		}
		if _, hasSection := part.Extra[keyResponsesSection]; !hasSection && part.Reasoning.Text != "" {
			item.Summary = append(item.Summary, responses.ReasoningSummaryPart{Type: responses.ReasoningSummaryPartTypeSummaryText, Text: responses.NewOptString(part.Reasoning.Text)})
		}
	}
	item.EncryptedContent = responseString(signature)
	return item, nil
}

// Summary and content preserve their SDK indices independently.
func responsesHistorySections(parts []*schema.MessageOutputPart, reasoning bool) (summary []*schema.MessageOutputPart, ordered []*schema.MessageOutputPart, err error) {
	content := make(map[int]*schema.MessageOutputPart)
	summaries := make(map[int]*schema.MessageOutputPart)
	for _, part := range parts {
		sectionValue, hasSection := part.Extra[keyResponsesSection]
		section, sectionOK := stringValue(sectionValue)
		index, hasIndex, err := extraIndex(part.Extra, keyResponsesContentIndex)
		if err != nil {
			return nil, nil, err
		}
		if reasoning && !hasIndex && part.Type == schema.ChatMessagePartTypeReasoning && part.Reasoning != nil {
			if !hasSection {
				continue // A reasoning item can contain just an encrypted signature.
			}
			if sectionOK && section == "summary" {
				summaryIndex, _, err := extraIndex(part.Extra, keyResponsesSummaryIndex)
				if err != nil {
					return nil, nil, err
				}
				if summaries[summaryIndex] != nil {
					return nil, nil, fmt.Errorf("duplicate reasoning summary index %d", summaryIndex)
				}
				summaries[summaryIndex] = part
				continue
			}
		}
		if !sectionOK || section != "content" || !hasIndex {
			return nil, nil, fmt.Errorf("incomplete Responses content metadata")
		}
		if content[index] != nil {
			return nil, nil, fmt.Errorf("duplicate Responses content index %d", index)
		}
		content[index] = part
	}
	var summaryIndices []int
	for index := range summaries {
		summaryIndices = append(summaryIndices, index)
	}
	sort.Ints(summaryIndices)
	for _, index := range summaryIndices {
		summary = append(summary, summaries[index])
	}
	indices := make([]int, 0, len(content))
	for index := range content {
		indices = append(indices, index)
	}
	sort.Ints(indices)
	for _, index := range indices {
		ordered = append(ordered, content[index])
	}
	return summary, ordered, nil
}

func responsesHistoryText(part *schema.MessageOutputPart) (responses.OptString, []responses.Annotation, error) {
	var text string
	switch part.Type {
	case schema.ChatMessagePartTypeText:
		text = part.Text
	case schema.ChatMessagePartTypeReasoning:
		if part.Reasoning == nil {
			return responses.OptString{}, nil, fmt.Errorf("assistant reasoning must not be nil")
		}
		text = part.Reasoning.Text
	default:
		return responses.OptString{}, nil, fmt.Errorf("unsupported Responses content part %q", part.Type)
	}
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

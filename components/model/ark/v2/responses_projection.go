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
	"strings"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/responses"
)

// ResponsesOutputItem carries an item with no mapped data part, such as an
// empty message or a built-in tool output. Other items attach their metadata to
// the first data part. Applications can skip this type when rendering text.
const ResponsesOutputItem schema.ChatMessagePartType = "ark_responses_item"

func projectResponsesResponse(resp *responses.Response, req *responses.ResponsesRequest) (*schema.Message, error) {
	msg, err := projectResponsesMetadata(resp, req)
	if err != nil {
		return nil, err
	}
	if err := validateResponsesOutput(resp, req); err != nil {
		return nil, err
	}
	if err := projectResponsesOutput(msg, resp.Output); err != nil {
		return nil, err
	}
	return msg, nil
}

func projectResponsesMetadata(resp *responses.Response, req *responses.ResponsesRequest) (*schema.Message, error) {
	if resp == nil {
		return nil, ErrEmptyResponse
	}
	if resp.Status.Or("") == responses.ResponseStatusFailed || resp.Error.IsSet() {
		problem := resp.Error.Or(responses.ResponseError{})
		return nil, fmt.Errorf("Ark response failed (%s): %s", problem.Code, problem.Message)
	}
	msg := &schema.Message{Role: schema.Assistant, ResponseMeta: &schema.ResponseMeta{FinishReason: string(resp.Status.Or(""))}, Extra: map[string]any{}}
	if d, ok := resp.IncompleteDetails.Get(); ok {
		msg.ResponseMeta.FinishReason = d.Reason
	}
	if resp.ID != "" {
		msg.Extra[keyResponseID] = metadataString(resp.ID)
	}
	if resp.Model != "" {
		msg.Extra[keyModelName] = metadataString(resp.Model)
	}
	if tier, ok := resp.ServiceTier.Get(); ok {
		msg.Extra[keyServiceTier] = metadataString(tier)
	}
	if req.Store.Or(false) {
		if expires := resp.ExpireAt.Or(req.ExpireAt.Or(0)); expires > 0 {
			msg.Extra[keyCacheExpiration] = metadataInt64(expires)
		}
	}
	if usage, ok := resp.Usage.Get(); ok {
		msg.ResponseMeta.Usage = &schema.TokenUsage{PromptTokens: int(usage.InputTokens), CompletionTokens: int(usage.OutputTokens), TotalTokens: int(usage.TotalTokens),
			PromptTokenDetails:      schema.PromptTokenDetails{CachedTokens: int(usage.InputTokensDetails.CachedTokens)},
			CompletionTokensDetails: schema.CompletionTokensDetails{ReasoningTokens: int(usage.OutputTokensDetails.ReasoningTokens)}}
		if err := setResponsesUsage(msg, usage); err != nil {
			return nil, err
		}
	}
	return msg, nil
}

func validateResponsesOutput(resp *responses.Response, req *responses.ResponsesRequest) error {
	if len(resp.Output) == 0 && resp.Status.Or("") != responses.ResponseStatusIncomplete && !req.Caching.Or(responses.ResponseCaching{}).Prefix.Or(false) {
		return ErrEmptyResponse
	}
	return nil
}

// Project a complete SDK output into canonical parts and legacy message fields
// together. Streaming callers reconcile the legacy fields after projection.
func projectResponsesOutput(msg *schema.Message, output []responses.OutputItem) error {
	var parts []schema.MessageOutputPart
	var content strings.Builder
	var reasoning []responses.ItemReasoning
	var toolCalls []schema.ToolCall
	for index, item := range output {
		children, err := projectResponsesItem(item, index)
		if err != nil {
			return fmt.Errorf("Responses output item %d: %w", index, err)
		}
		parts = append(parts, children...)
		switch item.OneOf.Type {
		case responses.ItemOutputMessageOutputItemSum:
			for _, part := range children {
				if part.Type == schema.ChatMessagePartTypeText {
					content.WriteString(part.Text)
				}
			}
		case responses.ItemFunctionToolCallOutputItemSum:
			toolCalls = append(toolCalls, projectResponsesToolCall(item.OneOf.ItemFunctionToolCall, index))
		case responses.ItemReasoningOutputItemSum:
			reasoning = append(reasoning, item.OneOf.ItemReasoning)
		}
	}
	msg.AssistantGenMultiContent = parts
	msg.Content = content.String()
	msg.ReasoningContent = projectResponsesSummaryText(reasoning)
	msg.ToolCalls = toolCalls
	return nil
}

// Project only SDK-exposed fields. Metadata belongs to the first data part;
// encrypted content belongs to the first reasoning part of the same item.
func projectResponsesItem(item responses.OutputItem, index int) ([]schema.MessageOutputPart, error) {
	extra := map[string]any{keyResponsesItemIndex: metadataInt64(index), keyResponsesItemType: metadataString(item.OneOf.Type)}
	var children []schema.MessageOutputPart
	var err error
	switch item.OneOf.Type {
	case responses.ItemOutputMessageOutputItemSum:
		message := item.OneOf.ItemOutputMessage
		extra[keyResponsesItemID] = metadataString(message.ID)
		extra[keyResponsesItemStatus] = metadataString(message.Status)
		extra[keyResponsesItemRole] = metadataString(message.Role)
		if partial, ok := message.Partial.Get(); ok {
			extra[keyResponsesPartial] = metadataBool(partial)
		}
		children, err = projectResponsesOutputContent(message.Content, index, responses.OutputContentItemTextOutputContentItemSum)
	case responses.ItemReasoningOutputItemSum:
		reasoning := item.OneOf.ItemReasoning
		if id, ok := reasoning.ID.Get(); ok {
			extra[keyResponsesItemID] = metadataString(id)
		}
		extra[keyResponsesItemStatus] = metadataString(reasoning.Status)
		for summaryIndex, part := range reasoning.Summary {
			children = append(children, projectResponsesSummaryPart(index, summaryIndex, part.Text.Or("")))
		}
		var content []schema.MessageOutputPart
		content, err = projectResponsesOutputContent(reasoning.Content, index, responses.OutputContentItemReasoningTextOutputContentItemSum)
		children = append(children, content...)
		carrier := -1
		for i := range children {
			if children[i].Reasoning != nil {
				carrier = i
				break
			}
		}
		if carrier < 0 {
			children = append(children, schema.MessageOutputPart{Type: schema.ChatMessagePartTypeReasoning,
				Reasoning: &schema.MessageOutputReasoning{}, Extra: map[string]any{keyResponsesItemIndex: metadataInt64(index)}})
			carrier = len(children) - 1
		}
		children[carrier].Reasoning.Signature = reasoning.EncryptedContent.Or("")
	case responses.ItemFunctionToolCallOutputItemSum:
		// Function values and item metadata belong to the same ToolCall.
		return nil, nil
	case responses.ItemFunctionWebSearchOutputItemSum:
		search := item.OneOf.ItemFunctionWebSearch
		extra[keyResponsesItemID] = metadataString(search.ID)
		extra[keyResponsesItemStatus] = metadataString(search.Status)
		fallthrough
	default:
		// Built-in outputs have no standard Eino data field. Retain the SDK
		// value for inspection, including its own unknown-variant fallback.
		if kind, _, unknown := item.OneOf.GetUnknown(); unknown {
			extra[keyResponsesItemType] = metadataString(kind)
		}
		var raw []byte
		raw, err = sonic.Marshal(&item)
		if err == nil {
			extra[keyResponsesItemData] = metadataString(raw)
		}
	}
	if err != nil {
		return nil, err
	}
	if len(children) == 0 {
		children = append(children, schema.MessageOutputPart{Type: ResponsesOutputItem, Extra: map[string]any{}})
	}
	for key, value := range extra {
		children[0].Extra[key] = value
	}
	return children, nil
}

func projectResponsesToolCall(call responses.ItemFunctionToolCall, index int) schema.ToolCall {
	extra := map[string]any{keyResponsesItemIndex: metadataInt64(index), keyResponsesItemType: metadataString("function_call")}
	if id, ok := call.ID.Get(); ok {
		extra[keyResponsesItemID] = metadataString(id)
	}
	if status, ok := call.Status.Get(); ok {
		extra[keyResponsesItemStatus] = metadataString(status)
	}
	if namespace, ok := call.Namespace.Get(); ok {
		extra[keyResponsesNamespace] = metadataString(namespace)
	}
	return schema.ToolCall{ID: call.CallID.Or(""), Type: "function", Extra: extra,
		Function: schema.FunctionCall{Name: call.Name.Or(""), Arguments: call.Arguments.Or("")}}
}

// The SDK shares the wire union; the parent item restricts the valid variant.
func projectResponsesOutputContent(content []responses.OutputContentItem, index int, expected responses.OutputContentItemSumType) ([]schema.MessageOutputPart, error) {
	parts := make([]schema.MessageOutputPart, 0, len(content))
	for contentIndex, child := range content {
		kind := child.OneOf.Type
		if (kind == responses.OutputContentItemTextOutputContentItemSum || kind == responses.OutputContentItemReasoningTextOutputContentItemSum) && kind != expected {
			return nil, fmt.Errorf("content part %d: expected %s, got %s", contentIndex, expected, kind)
		}
		part, err := projectResponsesOutputContentPart(child, index, contentIndex)
		if err != nil {
			return nil, err
		}
		parts = append(parts, part)
	}
	return parts, nil
}

// OutputContentItem covers both message.content and reasoning.content. Parent
// type constraints are checked by the full-item or streaming-state caller.
func projectResponsesOutputContentPart(child responses.OutputContentItem, itemIndex, contentIndex int) (schema.MessageOutputPart, error) {
	var part schema.MessageOutputPart
	var annotations []responses.Annotation
	switch child.OneOf.Type {
	case responses.OutputContentItemTextOutputContentItemSum:
		value := child.OneOf.OutputContentItemText
		part, annotations = projectResponsesTextPart(itemIndex, contentIndex, value.Text.Or("")), value.Annotations
	case responses.OutputContentItemReasoningTextOutputContentItemSum:
		value := child.OneOf.OutputContentItemReasoningText
		part, annotations = projectResponsesReasoningPart(itemIndex, contentIndex, value.Text.Or("")), value.Annotations
	default:
		part = schema.MessageOutputPart{Type: ResponsesOutputItem, Extra: responsesContentPartExtra(itemIndex, contentIndex, string(child.OneOf.Type))}
		if kind, _, unknown := child.OneOf.GetUnknown(); unknown {
			part.Extra[keyResponsesContentType] = metadataString(kind)
		}
		raw, err := sonic.Marshal(&child)
		if err != nil {
			return part, fmt.Errorf("content part %d: %w", contentIndex, err)
		}
		part.Extra[keyResponsesContentData] = metadataString(raw)
	}
	if annotations != nil {
		raw, err := sonic.Marshal(annotations)
		if err != nil {
			return part, fmt.Errorf("content part %d annotations: %w", contentIndex, err)
		}
		part.Extra[keyResponsesAnnotations] = metadataString(raw)
	}
	return part, nil
}

func projectResponsesTextPart(itemIndex, contentIndex int, text string) schema.MessageOutputPart {
	return schema.MessageOutputPart{Type: schema.ChatMessagePartTypeText, Text: text,
		Extra: responsesContentPartExtra(itemIndex, contentIndex, "output_text")}
}

func projectResponsesReasoningPart(itemIndex, contentIndex int, text string) schema.MessageOutputPart {
	return schema.MessageOutputPart{Type: schema.ChatMessagePartTypeReasoning, Reasoning: &schema.MessageOutputReasoning{Text: text},
		Extra: responsesContentPartExtra(itemIndex, contentIndex, "reasoning_text")}
}

func projectResponsesSummaryPart(itemIndex, summaryIndex int, text string) schema.MessageOutputPart {
	return schema.MessageOutputPart{Type: schema.ChatMessagePartTypeReasoning, Reasoning: &schema.MessageOutputReasoning{Text: text}, Extra: map[string]any{
		keyResponsesItemIndex: metadataInt64(itemIndex), keyResponsesSection: metadataString("summary"), keyResponsesSummaryIndex: metadataInt64(summaryIndex),
	}}
}

func responsesContentPartExtra(itemIndex, contentIndex int, contentType string) map[string]any {
	return map[string]any{
		keyResponsesItemIndex: metadataInt64(itemIndex), keyResponsesSection: metadataString("content"),
		keyResponsesContentIndex: metadataInt64(contentIndex), keyResponsesContentType: metadataString(contentType),
	}
}

func projectResponsesSummaryText(items []responses.ItemReasoning) string {
	var summary []string
	for _, item := range items {
		for _, part := range item.Summary {
			if text := part.Text.Or(""); text != "" {
				summary = append(summary, text)
			}
		}
	}
	return strings.Join(summary, "\n")
}

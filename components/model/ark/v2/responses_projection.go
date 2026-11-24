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
	"errors"
	"fmt"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/responses"
)

const (
	keyResponsesUsage = extraKeyPrefix + "responses-usage-details"

	keyResponsesItemIndex    = extraKeyPrefix + "responses-item-index"
	keyResponsesSection      = extraKeyPrefix + "responses-section"
	keyResponsesContentIndex = extraKeyPrefix + "responses-content-index"
	keyResponsesSummaryIndex = extraKeyPrefix + "responses-summary-index"
	keyResponsesItemType     = extraKeyPrefix + "responses-item-type"
	keyResponsesContentType  = extraKeyPrefix + "responses-content-type"
	keyResponsesItemID       = extraKeyPrefix + "responses-item-id"
	keyResponsesItemStatus   = extraKeyPrefix + "responses-item-status"
	keyResponsesItemRole     = extraKeyPrefix + "responses-item-role"
	keyResponsesPartial      = extraKeyPrefix + "responses-partial"
	keyResponsesNamespace    = extraKeyPrefix + "responses-namespace"
	keyResponsesAnnotations  = extraKeyPrefix + "responses-annotations"
	// SDK values without an Eino data mapping remain observable as atomic JSON.
	// They are never decoded into additional request fields.
	keyResponsesItemData    = extraKeyPrefix + "responses-item-data"
	keyResponsesContentData = extraKeyPrefix + "responses-content-data"
)

// ResponsesOutputItem carries an item with no mapped data part, such as an
// empty message or a built-in tool output. Other items attach their metadata to
// the first data part. Applications can skip this type when rendering text.
const ResponsesOutputItem schema.ChatMessagePartType = "ark_responses_item"

func projectResponsesResponse(resp *responses.Response, req *responses.ResponsesRequest) (*schema.Message, error) {
	return projectResponsesResponseParts(resp, req, false)
}

func projectResponsesResponseParts(resp *responses.Response, req *responses.ResponsesRequest, streaming bool) (*schema.Message, error) {
	msg, err := projectResponsesMetadata(resp, req)
	if err != nil {
		return nil, err
	}
	if err := validateResponsesOutput(resp, req); err != nil {
		return nil, err
	}
	if err := projectResponsesOutput(msg, resp.Output, streaming); err != nil {
		return nil, err
	}
	return msg, nil
}

func responseFailedError(resp *responses.Response) error {
	problem := resp.Error.Or(responses.ResponseError{})
	message := "Ark response failed"
	if problem.Code != "" {
		message += " (" + problem.Code + ")"
	}
	if problem.Message != "" {
		message += ": " + problem.Message
	}
	return errors.New(message)
}

func projectResponsesMetadata(resp *responses.Response, req *responses.ResponsesRequest) (*schema.Message, error) {
	if resp == nil {
		return nil, ErrEmptyResponse
	}
	if resp.Status.Or("") == responses.ResponseStatusFailed || resp.Error.IsSet() {
		return nil, responseFailedError(resp)
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
// together. Streaming callers keep reasoning segments and get text from deltas.
func projectResponsesOutput(msg *schema.Message, output []responses.OutputItem, streaming bool) error {
	var parts []schema.MessageOutputPart
	var content strings.Builder
	var reasoning []responses.ItemReasoning
	var toolCalls []schema.ToolCall
	for index, item := range output {
		children, err := projectResponsesItem(item, index)
		if err != nil {
			return fmt.Errorf("Responses output item %d: %w", index, err)
		}
		if !streaming && item.OneOf.Type == responses.ItemReasoningOutputItemSum {
			var projection reasoningItemProjection
			children = projection.parts(children)
		}
		parts = append(parts, children...)
		switch item.OneOf.Type {
		case responses.ItemOutputMessageOutputItemSum:
			if streaming {
				continue
			}
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
	switch child.OneOf.Type {
	case responses.OutputContentItemReasoningTextOutputContentItemSum:
		return projectResponsesReasoningPart(itemIndex, contentIndex, child.OneOf.OutputContentItemReasoningText.Text.Or("")), nil
	case responses.OutputContentItemTextOutputContentItemSum:
		value := child.OneOf.OutputContentItemText
		part := projectResponsesTextPart(itemIndex, contentIndex, value.Text.Or(""))
		if value.Annotations != nil {
			raw, err := sonic.Marshal(value.Annotations)
			if err != nil {
				return part, fmt.Errorf("content part %d annotations: %w", contentIndex, err)
			}
			part.Extra[keyResponsesAnnotations] = metadataString(raw)
		}
		return part, nil
	default:
		part := schema.MessageOutputPart{Type: ResponsesOutputItem, Extra: responsesContentPartExtra(itemIndex, contentIndex, string(child.OneOf.Type))}
		if kind, _, unknown := child.OneOf.GetUnknown(); unknown {
			part.Extra[keyResponsesContentType] = metadataString(kind)
		}
		raw, err := sonic.Marshal(&child)
		if err != nil {
			return part, fmt.Errorf("content part %d: %w", contentIndex, err)
		}
		part.Extra[keyResponsesContentData] = metadataString(raw)
		return part, nil
	}
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

// Fold the ordered native summary and raw segments of one item into its public
// reasoning part. A stream retains this state between emissions of that item.
type reasoningItemProjection struct {
	summaryStarted bool
	summaryIndex   int
	rawSlots       map[int]int
}

func (p *reasoningItemProjection) parts(segments []schema.MessageOutputPart) []schema.MessageOutputPart {
	if len(segments) == 0 || (len(segments) == 1 && segments[0].Reasoning == nil) {
		return segments
	}
	var out []schema.MessageOutputPart
	var reasoning *schema.MessageOutputReasoning
	var extra map[string]any
	var summary strings.Builder
	var raw rawReasoningContent
	for _, segment := range segments {
		if segment.Reasoning == nil {
			out = append(out, segment)
			continue
		}
		if reasoning == nil {
			reasoning = &schema.MessageOutputReasoning{}
			extra = make(map[string]any)
			out = append(out, schema.MessageOutputPart{Type: schema.ChatMessagePartTypeReasoning, Reasoning: reasoning, Extra: extra})
		}
		for key, value := range segment.Extra {
			switch key {
			case keyResponsesSection, keyResponsesContentType, keyResponsesContentIndex, keyResponsesSummaryIndex:
			default:
				extra[key] = value
			}
		}
		if signature := segment.Reasoning.Signature; signature != "" {
			reasoning.Signature = signature
		}
		key := responsesPartIdentity(segment)
		text := segment.Reasoning.Text
		switch key.section {
		case "summary":
			if text != "" {
				if p.summaryStarted && p.summaryIndex != key.index {
					summary.WriteByte('\n')
				}
				summary.WriteString(text)
				p.summaryIndex, p.summaryStarted = key.index, true
			}
		case "content":
			if p.rawSlots == nil {
				p.rawSlots = make(map[int]int)
			}
			slot, ok := p.rawSlots[key.index]
			if !ok {
				slot = len(p.rawSlots)
				p.rawSlots[key.index] = slot
			}
			for len(raw) <= slot {
				raw = append(raw, "")
			}
			raw[slot] += text
		}
	}
	if reasoning != nil {
		reasoning.Text = summary.String()
		if len(raw) > 0 {
			extra[keyRawReasoningContent] = raw
		}
	}
	return out
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

// GetResponsesTokenUsageDetails combines the current ResponseMeta.Usage with
// modality and built-in tool usage in Extra. It returns false when
// ResponseMeta.Usage is absent.
func GetResponsesTokenUsageDetails(msg *schema.Message) (responses.ResponseUsage, bool) {
	return readResponsesUsage(msg)
}

func setResponsesUsage(msg *schema.Message, usage responses.ResponseUsage) error {
	counts := map[string]responses.OptInt64{
		"input_tokens_details.audio_tokens":        usage.InputTokensDetails.AudioTokens,
		"input_tokens_details.audio_cached_tokens": usage.InputTokensDetails.AudioCachedTokens,
	}
	tools, present := usage.ToolUsage.Get()
	if present {
		msg.Extra[keyResponsesUsage+".tool_usage"] = metadataBool(true)
	} else {
		delete(msg.Extra, keyResponsesUsage+".tool_usage")
	}
	counts["tool_usage.web_search"], counts["tool_usage.mcp"] = tools.WebSearch, tools.Mcp
	counts["tool_usage.knowledge_search"], counts["tool_usage.doubao_app"] = tools.KnowledgeSearch, tools.DoubaoApp
	for key, value := range counts {
		n, present := value.Get()
		setOptionalCount(msg, keyResponsesUsage+"."+key, n, present)
	}
	if details, ok := usage.ToolUsageDetails.Get(); ok {
		return setJSONExtra(msg, keyResponsesUsage+".tool_usage_details", &details)
	}
	delete(msg.Extra, keyResponsesUsage+".tool_usage_details")
	return nil
}

func readResponsesUsage(msg *schema.Message) (responses.ResponseUsage, bool) {
	if msg == nil || msg.ResponseMeta == nil || msg.ResponseMeta.Usage == nil {
		return responses.ResponseUsage{}, false
	}
	usage := msg.ResponseMeta.Usage
	var result responses.ResponseUsage
	result.InputTokens, result.OutputTokens, result.TotalTokens = int64(usage.PromptTokens), int64(usage.CompletionTokens), int64(usage.TotalTokens)
	result.InputTokensDetails.CachedTokens = int64(usage.PromptTokenDetails.CachedTokens)
	result.OutputTokensDetails.ReasoningTokens = int64(usage.CompletionTokensDetails.ReasoningTokens)
	var tools responses.ToolUsage
	for key, field := range map[string]*responses.OptInt64{
		"input_tokens_details.audio_tokens":        &result.InputTokensDetails.AudioTokens,
		"input_tokens_details.audio_cached_tokens": &result.InputTokensDetails.AudioCachedTokens,
		"tool_usage.web_search":                    &tools.WebSearch, "tool_usage.mcp": &tools.Mcp,
		"tool_usage.knowledge_search": &tools.KnowledgeSearch, "tool_usage.doubao_app": &tools.DoubaoApp,
	} {
		if n, ok := extraInt64(msg.Extra, keyResponsesUsage+"."+key); ok {
			*field = responses.NewOptInt64(n)
		}
	}
	if present, _ := extraBool(msg.Extra, keyResponsesUsage+".tool_usage"); present {
		result.ToolUsage = responses.NewOptToolUsage(tools)
	}
	if details, ok := readJSONExtra[responses.ToolUsageDetails](msg, keyResponsesUsage+".tool_usage_details"); ok {
		result.ToolUsageDetails = responses.NewOptToolUsageDetails(details)
	}
	return result, true
}

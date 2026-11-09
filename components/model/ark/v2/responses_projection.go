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

func setResponsesFinalOutput(msg *schema.Message, output []responses.OutputItem) error {
	var parts []schema.MessageOutputPart
	for index, item := range output {
		children, err := projectResponsesItem(item, index)
		if err != nil {
			return fmt.Errorf("Responses output item %d: %w", index, err)
		}
		parts = append(parts, children...)
	}
	msg.AssistantGenMultiContent = parts
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
		children, err = projectResponsesContent(message.Content, index, false)
	case responses.ItemReasoningOutputItemSum:
		reasoning := item.OneOf.ItemReasoning
		if id, ok := reasoning.ID.Get(); ok {
			extra[keyResponsesItemID] = metadataString(id)
		}
		extra[keyResponsesItemStatus] = metadataString(reasoning.Status)
		if len(reasoning.Summary) > 0 {
			texts := make([]string, len(reasoning.Summary))
			for i, part := range reasoning.Summary {
				texts[i] = part.Text.Or("")
			}
			children = append(children, schema.MessageOutputPart{
				Type: schema.ChatMessagePartTypeReasoning, Reasoning: &schema.MessageOutputReasoning{Text: strings.Join(texts, "\n")},
				Extra: map[string]any{keyResponsesItemIndex: metadataInt64(index), keyResponsesSection: metadataString("summary")},
			})
		}
		var content []schema.MessageOutputPart
		content, err = projectResponsesContent(reasoning.Content, index, true)
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
		call := item.OneOf.ItemFunctionToolCall
		if id, ok := call.ID.Get(); ok {
			extra[keyResponsesItemID] = metadataString(id)
		}
		if status, ok := call.Status.Get(); ok {
			extra[keyResponsesItemStatus] = metadataString(status)
		}
		if namespace, ok := call.Namespace.Get(); ok {
			extra[keyResponsesNamespace] = metadataString(namespace)
		}
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

func projectResponsesContent(content []responses.OutputContentItem, index int, reasoning bool) ([]schema.MessageOutputPart, error) {
	parts := make([]schema.MessageOutputPart, 0, len(content))
	for childIndex, child := range content {
		part := schema.MessageOutputPart{Type: ResponsesOutputItem, Extra: map[string]any{
			keyResponsesItemIndex: metadataInt64(index), keyResponsesSection: metadataString("content"), keyResponsesContentIndex: metadataInt64(childIndex),
			keyResponsesContentType: metadataString(child.OneOf.Type),
		}}
		var text string
		var annotations []responses.Annotation
		switch child.OneOf.Type {
		case responses.OutputContentItemTextOutputContentItemSum:
			value := child.OneOf.OutputContentItemText
			text, annotations = value.Text.Or(""), value.Annotations
		case responses.OutputContentItemReasoningTextOutputContentItemSum:
			value := child.OneOf.OutputContentItemReasoningText
			text, annotations = value.Text.Or(""), value.Annotations
		default:
			if kind, _, unknown := child.OneOf.GetUnknown(); unknown {
				part.Extra[keyResponsesContentType] = metadataString(kind)
			}
			raw, err := sonic.Marshal(&child)
			if err != nil {
				return nil, fmt.Errorf("content part %d: %w", childIndex, err)
			}
			part.Extra[keyResponsesContentData] = metadataString(raw)
			parts = append(parts, part)
			continue
		}
		if reasoning {
			part.Type = schema.ChatMessagePartTypeReasoning
			part.Reasoning = &schema.MessageOutputReasoning{Text: text}
		} else {
			part.Type = schema.ChatMessagePartTypeText
			part.Text = text
		}
		if annotations != nil {
			raw, err := sonic.Marshal(annotations)
			if err != nil {
				return nil, fmt.Errorf("content part %d annotations: %w", childIndex, err)
			}
			part.Extra[keyResponsesAnnotations] = metadataString(raw)
		}
		parts = append(parts, part)
	}
	return parts, nil
}

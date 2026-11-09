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
	"encoding/json"
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

func setResponsesFinalOutput(msg *schema.Message, output []responses.OutputItem, raw []byte) error {
	items, err := responsesOutputJSON(output, raw)
	if err != nil {
		return fmt.Errorf("decode Responses output: %w", err)
	}
	var parts []schema.MessageOutputPart
	for index, item := range items {
		children, err := projectResponsesItem(item, index)
		if err != nil {
			return fmt.Errorf("Responses output item %d: %w", index, err)
		}
		for _, child := range children {
			// Stream merge indices are independent of persistent item indices.
			child.StreamingMeta = &schema.MessageStreamingMeta{Index: len(parts)}
			parts = append(parts, child)
		}
	}
	msg.AssistantGenMultiContent = parts
	return nil
}

// Generate and terminal SSE events use different envelopes for the same output.
// Keep raw item fields that are not represented by the SDK's generated types.
func responsesOutputJSON(output []responses.OutputItem, raw []byte) ([]json.RawMessage, error) {
	var items []json.RawMessage
	if len(raw) > 0 {
		var envelope struct {
			Output   json.RawMessage `json:"output"`
			Response *struct {
				Output json.RawMessage `json:"output"`
			} `json:"response"`
		}
		if err := sonic.Unmarshal(raw, &envelope); err != nil {
			return nil, err
		}
		if envelope.Response != nil {
			envelope.Output = envelope.Response.Output
		}
		if len(envelope.Output) > 0 {
			if err := sonic.Unmarshal(envelope.Output, &items); err != nil {
				return nil, err
			}
		}
	} else {
		items = make([]json.RawMessage, len(output))
		for i := range output {
			value, err := sonic.Marshal(&output[i])
			if err != nil {
				return nil, err
			}
			items[i] = value
		}
	}
	return items, nil
}

// Item ownership belongs here: metadata goes on the first data part, and an
// encrypted signature goes on the first reasoning part of that same item.
func projectResponsesItem(raw json.RawMessage, index int) ([]schema.MessageOutputPart, error) {
	item, err := decodeResponseObject(raw)
	if err != nil {
		return nil, err
	}
	kind := responseObjectType(item)
	if kind == "function_call" {
		delete(item, "call_id")
		delete(item, "name")
		delete(item, "arguments")
	}
	var signature string
	var children []schema.MessageOutputPart
	if kind == "reasoning" {
		signature, err = takeResponseString(item, "encrypted_content")
		if err != nil {
			return nil, err
		}
		children, err = projectResponsesSummary(item["summary"], index)
		if err != nil {
			return nil, fmt.Errorf("summary: %w", err)
		}
		delete(item, "summary")
	}
	if kind == "message" || kind == "reasoning" {
		content, err := projectResponsesContent(item["content"], index, kind == "reasoning")
		if err != nil {
			return nil, fmt.Errorf("content: %w", err)
		}
		children = append(children, content...)
		delete(item, "content")
	}
	if kind == "reasoning" {
		carrier := -1
		for i := range children {
			if children[i].Reasoning != nil {
				carrier = i
				break
			}
		}
		if carrier < 0 {
			children = append(children, schema.MessageOutputPart{Type: schema.ChatMessagePartTypeReasoning, Reasoning: &schema.MessageOutputReasoning{}, Extra: map[string]any{keyResponsesItemIndex: metadataInt64(index)}})
			carrier = len(children) - 1
		}
		children[carrier].Reasoning.Signature = signature
	}
	if len(children) == 0 {
		children = append(children, schema.MessageOutputPart{Type: ResponsesOutputItem, Extra: map[string]any{keyResponsesItemIndex: metadataInt64(index)}})
	}
	storeResponseMetadata(children[0].Extra, item, responseItemMetadataKeys, keyResponsesItemRawPrefix)
	return children, nil
}

func projectResponsesSummary(raw json.RawMessage, index int) ([]schema.MessageOutputPart, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var summary []responses.ReasoningSummaryPart
	if err := sonic.Unmarshal(raw, &summary); err != nil {
		return nil, err
	}
	if len(summary) == 0 {
		return nil, nil
	}
	texts := make([]string, len(summary))
	for i, part := range summary {
		texts[i] = part.Text.Or("")
	}
	return []schema.MessageOutputPart{{
		Type:      schema.ChatMessagePartTypeReasoning,
		Reasoning: &schema.MessageOutputReasoning{Text: strings.Join(texts, "\n")},
		Extra:     map[string]any{keyResponsesItemIndex: metadataInt64(index), keyResponsesSection: metadataString("summary")},
	}}, nil
}

func projectResponsesContent(raw json.RawMessage, index int, reasoning bool) ([]schema.MessageOutputPart, error) {
	if len(raw) == 0 || responseJSONNull(raw) {
		return nil, nil
	}
	var leaves []json.RawMessage
	if err := sonic.Unmarshal(raw, &leaves); err != nil {
		return nil, err
	}
	parts := make([]schema.MessageOutputPart, 0, len(leaves))
	for childIndex, rawLeaf := range leaves {
		leaf, err := decodeResponseObject(rawLeaf)
		if err != nil {
			return nil, fmt.Errorf("part %d: %w", childIndex, err)
		}
		part := schema.MessageOutputPart{Type: ResponsesOutputItem, Extra: map[string]any{
			keyResponsesItemIndex: metadataInt64(index), keyResponsesSection: metadataString("content"), keyResponsesContentIndex: metadataInt64(childIndex),
		}}
		kind := responseObjectType(leaf)
		if kind == "output_text" || kind == "reasoning_text" {
			text, err := takeResponseString(leaf, "text")
			if err != nil {
				return nil, fmt.Errorf("part %d: %w", childIndex, err)
			}
			if reasoning {
				part.Type = schema.ChatMessagePartTypeReasoning
				part.Reasoning = &schema.MessageOutputReasoning{Text: text}
			} else {
				part.Type = schema.ChatMessagePartTypeText
				part.Text = text
			}
		}
		storeResponseMetadata(part.Extra, leaf, responseContentMetadataKeys, keyResponsesContentRawPrefix)
		parts = append(parts, part)
	}
	return parts, nil
}

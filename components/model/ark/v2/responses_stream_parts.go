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
	"reflect"
	"sort"
	"strings"

	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/responses"
)

type responsesPartKey struct {
	section string
	index   int
}

type responsesStreamPart struct {
	prototype schema.MessageOutputPart
	received  strings.Builder
	emitted   string
	index     int
	pending   bool
}

type responsesStreamItem struct {
	kind         responses.OutputItemSumType
	parts        map[responsesPartKey]*responsesStreamPart
	section      string
	source       []schema.MessageOutputPart
	done, closed bool
	signature    string
}

// Eino merges adjacent part chunks. Keep one item open until its done event
// supplies final metadata, buffering later parts/items until they can follow it.
type responsesPartsStream struct {
	items               map[int]*responsesStreamItem
	nextItem, nextIndex int
}

func (s *responsesPartsStream) item(index int) *responsesStreamItem {
	if s.items == nil {
		s.items = make(map[int]*responsesStreamItem)
	}
	if s.items[index] == nil {
		s.items[index] = &responsesStreamItem{parts: make(map[responsesPartKey]*responsesStreamPart)}
	}
	return s.items[index]
}

func responsesPartIdentity(part schema.MessageOutputPart) responsesPartKey {
	section, _ := extraString(part.Extra, keyResponsesSection)
	key := keyResponsesContentIndex
	if section == "summary" {
		key = keyResponsesSummaryIndex
	}
	index, _, _ := extraIndex(part.Extra, key)
	return responsesPartKey{section, index}
}

func copyResponsesPart(part schema.MessageOutputPart) schema.MessageOutputPart {
	extra := make(map[string]any, len(part.Extra))
	for k, v := range part.Extra {
		extra[k] = v
	}
	part.Extra = extra
	if part.Reasoning != nil {
		r := *part.Reasoning
		part.Reasoning = &r
	}
	return part
}

func responsesPartText(part schema.MessageOutputPart) string {
	if part.Reasoning != nil {
		return part.Reasoning.Text
	}
	return part.Text
}

func setResponsesPartText(part *schema.MessageOutputPart, text string) {
	if part.Reasoning != nil {
		part.Reasoning.Text = text
	} else {
		part.Text = text
	}
}

func (s *responsesPartsStream) resolve(event *responses.ResponseStreamEvent, msg *schema.Message, terminal bool) ([]schema.MessageOutputPart, error) {
	if event == nil {
		return nil, nil
	}
	ev := &event.OneOf
	switch ev.Type {
	case responses.ResponseOutputItemAddedEventResponseStreamEventSum:
		if ev.ResponseOutputItemAddedEvent.OutputIndex < 0 {
			return nil, fmt.Errorf("negative Responses output index")
		}
		s.item(int(ev.ResponseOutputItemAddedEvent.OutputIndex)).kind = ev.ResponseOutputItemAddedEvent.Item.OneOf.Type
		if reasoning, ok := ev.ResponseOutputItemAddedEvent.Item.OneOf.GetItemReasoning(); ok && reasoning.EncryptedContent.Or("") != "" {
			s.item(int(ev.ResponseOutputItemAddedEvent.OutputIndex)).signature = reasoning.EncryptedContent.Value
		}
	case responses.ResponseOutputItemDoneEventResponseStreamEventSum:
		index := int(ev.ResponseOutputItemDoneEvent.OutputIndex)
		if index < 0 {
			return nil, fmt.Errorf("negative Responses output index")
		}
		if msg != nil {
			if err := s.finish(index, msg.AssistantGenMultiContent, false); err != nil {
				return nil, err
			}
		}
	default:
	}
	if ev.Type != responses.ResponseOutputItemDoneEventResponseStreamEventSum {
		if !terminal && msg != nil {
			for _, part := range msg.AssistantGenMultiContent {
				index, _, err := extraIndex(part.Extra, keyResponsesItemIndex)
				if err != nil {
					return nil, err
				}
				key := responsesPartIdentity(part)
				if key.index < 0 {
					return nil, fmt.Errorf("negative Responses content index")
				}
				item := s.item(index)
				if key.section == "content" &&
					((item.kind == responses.ItemReasoningOutputItemSum && part.Type == schema.ChatMessagePartTypeText) ||
						(item.kind == responses.ItemOutputMessageOutputItemSum && part.Type == schema.ChatMessagePartTypeReasoning)) {
					return nil, fmt.Errorf("Responses item %d (%s) cannot contain a %s part", index, item.kind, part.Type)
				}
				if item.closed {
					return nil, fmt.Errorf("Responses item %d received a delta after output_item.done", index)
				}
				cell := item.parts[key]
				if cell == nil {
					cell = &responsesStreamPart{prototype: copyResponsesPart(part), index: -1}
					if cell.prototype.Reasoning != nil {
						cell.prototype.Reasoning.Signature = ""
					}
					item.parts[key] = cell
				}
				if item.section == "" {
					item.section = key.section
				}
				snapshot := false
				switch ev.Type {
				case responses.ResponseOutputItemAddedEventResponseStreamEventSum,
					responses.ResponseContentPartAddedEventResponseStreamEventSum,
					responses.ResponseContentPartDoneEventResponseStreamEventSum,
					responses.ResponseReasoningSummaryPartAddedEventResponseStreamEventSum,
					responses.ResponseReasoningSummaryPartDoneEventResponseStreamEventSum:
					snapshot = true
				}
				text := responsesPartText(part)
				if snapshot {
					if text != "" {
						if !strings.HasPrefix(text, cell.received.String()) {
							return nil, fmt.Errorf("Responses part snapshot differs from its delta prefix")
						}
						cell.received.WriteString(text[cell.received.Len():])
					}
					cell.prototype = copyResponsesPart(part)
					if cell.prototype.Reasoning != nil {
						cell.prototype.Reasoning.Signature = ""
					}
					cell.pending = true
				} else {
					cell.received.WriteString(text)
				}
			}
		}
	}
	if terminal && msg != nil {
		groups := make(map[int][]schema.MessageOutputPart)
		for _, part := range msg.AssistantGenMultiContent {
			index, _, err := extraIndex(part.Extra, keyResponsesItemIndex)
			if err != nil {
				return nil, err
			}
			groups[index] = append(groups[index], part)
		}
		for index, item := range s.items {
			if index >= len(groups) && len(item.parts) > 0 {
				return nil, fmt.Errorf("terminal response omitted streamed item %d", index)
			}
		}
		for index := 0; index < len(groups); index++ {
			if err := s.finish(index, groups[index], true); err != nil {
				return nil, err
			}
		}
	}
	return s.drain()
}

func (s *responsesPartsStream) finish(index int, parts []schema.MessageOutputPart, terminal bool) error {
	item := s.item(index)
	currentSignature := ""
	for _, part := range parts {
		if part.Reasoning != nil && part.Reasoning.Signature != "" {
			currentSignature = part.Reasoning.Signature
		}
	}
	if currentSignature != "" {
		item.signature = currentSignature
	}
	if currentSignature == "" && item.signature != "" {
		for i, part := range parts {
			if part.Reasoning != nil {
				parts = append([]schema.MessageOutputPart(nil), parts...)
				parts[i] = copyResponsesPart(part)
				parts[i].Reasoning.Signature = item.signature
				break
			}
		}
	}
	if item.closed {
		if !reflect.DeepEqual(item.source, parts) {
			return fmt.Errorf("Responses item %d changed after output_item.done", index)
		}
		return nil
	}
	item.source = parts
	if !terminal && len(parts) > 0 {
		kind, _ := extraString(parts[0].Extra, keyResponsesItemType)
		if kind == "reasoning" {
			// Encrypted reasoning can first arrive in the terminal response.
			if currentSignature == "" {
				return nil
			}
		}
	}
	item.done = true
	return nil
}

func (s *responsesPartsStream) drain() ([]schema.MessageOutputPart, error) {
	var out []schema.MessageOutputPart
	for {
		item := s.items[s.nextItem]
		if item == nil {
			break
		}
		key := responsesPartKey{section: item.section, index: 0}
		active := item.parts[key]
		if !item.done {
			if active != nil && (active.index < 0 || active.pending || active.received.Len() > len(active.emitted)) && (active.prototype.Type == schema.ChatMessagePartTypeText || active.prototype.Type == schema.ChatMessagePartTypeReasoning) {
				part := copyResponsesPart(active.prototype)
				setResponsesPartText(&part, active.received.String()[len(active.emitted):])
				if active.index < 0 {
					active.index = s.nextIndex
					s.nextIndex++
				}
				part.StreamingMeta = &schema.MessageStreamingMeta{Index: active.index}
				active.emitted = active.received.String()
				active.pending = false
				out = append(out, part)
			}
			break
		}
		full := s.completeParts(item, key)
		for _, original := range full {
			part := copyResponsesPart(original)
			partKey := responsesPartIdentity(part)
			cell := item.parts[partKey]
			if cell == nil {
				cell = &responsesStreamPart{index: -1}
				item.parts[partKey] = cell
			}
			text := responsesPartText(part)
			if text == "" && cell.received.Len() > 0 {
				text = cell.received.String()
			}
			if !strings.HasPrefix(text, cell.received.String()) || !strings.HasPrefix(text, cell.emitted) {
				return nil, fmt.Errorf("Responses item %d part %v differs from its streamed prefix", s.nextItem, partKey)
			}
			setResponsesPartText(&part, text[len(cell.emitted):])
			if cell.index < 0 {
				cell.index = s.nextIndex
				s.nextIndex++
			}
			part.StreamingMeta = &schema.MessageStreamingMeta{Index: cell.index}
			cell.emitted = text
			out = append(out, part)
		}
		item.closed = true
		s.nextItem++
	}
	return out, nil
}

func (s *responsesPartsStream) completeParts(item *responsesStreamItem, activeKey responsesPartKey) []schema.MessageOutputPart {
	full := make([]schema.MessageOutputPart, len(item.source))
	keys := make(map[responsesPartKey]bool)
	header := make(map[string]any)
	var signature string
	for i, original := range item.source {
		full[i] = copyResponsesPart(original)
		keys[responsesPartIdentity(original)] = true
		if original.Reasoning != nil && original.Reasoning.Signature != "" {
			signature = original.Reasoning.Signature
		}
	}
	if len(full) > 0 {
		for k, v := range full[0].Extra {
			switch k {
			case keyResponsesItemIndex, keyResponsesSection, keyResponsesContentIndex, keyResponsesSummaryIndex, keyResponsesContentType, keyResponsesAnnotations, keyResponsesContentData:
			default:
				header[k] = v
			}
		}
	}

	// A terminal response may omit plaintext that was supplied through deltas.
	var missing []responsesPartKey
	for key, cell := range item.parts {
		if !keys[key] && (cell.received.Len() > 0 || cell.index >= 0) {
			missing = append(missing, key)
		}
	}
	sort.Slice(missing, func(i, j int) bool {
		if missing[i].section != missing[j].section {
			return missing[i].section < missing[j].section
		}
		return missing[i].index < missing[j].index
	})
	for _, key := range missing {
		part := copyResponsesPart(item.parts[key].prototype)
		setResponsesPartText(&part, item.parts[key].received.String())
		if len(full) == 1 && full[0].Reasoning != nil && responsesPartIdentity(full[0]).section == "" {
			full[0] = part
		} else {
			full = append(full, part)
		}
	}
	if active := item.parts[activeKey]; active != nil && active.index >= 0 {
		sort.SliceStable(full, func(i, j int) bool {
			a, b := responsesPartIdentity(full[i]), responsesPartIdentity(full[j])
			if (a.section == activeKey.section) != (b.section == activeKey.section) {
				return a.section == activeKey.section
			}
			if a.section == b.section {
				return a.index < b.index
			}
			return false
		})
	}

	// Item metadata belongs to the first part in emission order. A summary keeps
	// the encrypted signature when one exists, regardless of delta arrival order.
	carrier, summaryCarrier := -1, -1
	for i := range full {
		for k := range header {
			delete(full[i].Extra, k)
		}
		if full[i].Reasoning != nil {
			full[i].Reasoning.Signature = ""
			if carrier < 0 {
				carrier = i
			}
			if summaryCarrier < 0 && responsesPartIdentity(full[i]).section == "summary" {
				summaryCarrier = i
			}
		}
	}
	if len(full) > 0 {
		for k, v := range header {
			full[0].Extra[k] = v
		}
	}
	if summaryCarrier >= 0 {
		carrier = summaryCarrier
	}
	if carrier >= 0 {
		full[carrier].Reasoning.Signature = signature
	}
	return full
}

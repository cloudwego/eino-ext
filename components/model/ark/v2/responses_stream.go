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

// Track the projected stream segments. Terminal output supplies any
// missing reasoning and tool fields. Message text comes only from text deltas.
type responsesStreamState struct {
	parts          responsesPartsStream
	tools          map[int]*responsesStreamTool // keyed by SDK output_index
	summaryIndex   int
	summaryStarted bool
}

type responsesStreamTool struct {
	index     int
	id, name  string
	arguments strings.Builder
}

func (s *responsesStreamState) record(msg *schema.Message) error {
	for i := range msg.ToolCalls {
		call := &msg.ToolCalls[i]
		outputIndex, exists, err := extraIndex(call.Extra, keyResponsesItemIndex)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("Responses tool call has no item index")
		}
		if s.tools == nil {
			s.tools = make(map[int]*responsesStreamTool)
		}
		tool := s.tools[outputIndex]
		if tool == nil {
			tool = &responsesStreamTool{index: len(s.tools)}
			s.tools[outputIndex] = tool
		}
		// A stream's tool-call ordinals follow first occurrence. Keep the native
		// output index in Extra for history, and never renumber emitted chunks.
		index := tool.index
		call.Index = &index
		if call.ID != "" {
			if _, err := responsesStreamIdentity(call.ID, tool.id, "tool call ID"); err != nil {
				return err
			}
			tool.id = call.ID
		}
		if call.Function.Name != "" {
			if _, err := responsesStreamIdentity(call.Function.Name, tool.name, "tool name"); err != nil {
				return err
			}
			tool.name = call.Function.Name
		}
		tool.arguments.WriteString(call.Function.Arguments)
	}
	return nil
}

func (s *responsesStreamState) resolve(event *responses.ResponseStreamEvent, req *responses.ResponsesRequest) (*schema.Message, bool, error) {
	msg, terminal, err := projectResponsesStreamEvent(event, req)
	if err != nil {
		return msg, terminal, err
	}
	// Message text comes from output_text.delta. For now, we ignore differences
	// between the accumulated deltas and the text in done or terminal events.
	if msg != nil && event.OneOf.Type != responses.ResponseTextDeltaEventResponseStreamEventSum {
		for i := range msg.AssistantGenMultiContent {
			msg.AssistantGenMultiContent[i].Text = ""
		}
	}
	parts, err := s.parts.resolve(event, msg, terminal)
	if err != nil {
		return nil, terminal, err
	}
	if msg == nil && len(parts) > 0 {
		msg = &schema.Message{Role: schema.Assistant}
	}
	if msg == nil {
		return nil, terminal, nil
	}
	msg.AssistantGenMultiContent = parts
	msg.ReasoningContent = s.summaryDelta(parts)
	if !terminal {
		// Added/done carry snapshots, whereas argument events carry deltas.
		var item responses.OutputItem
		var index int
		switch event.OneOf.Type {
		case responses.ResponseOutputItemAddedEventResponseStreamEventSum:
			item = event.OneOf.ResponseOutputItemAddedEvent.Item
			index = int(event.OneOf.ResponseOutputItemAddedEvent.OutputIndex)
		case responses.ResponseOutputItemDoneEventResponseStreamEventSum:
			item = event.OneOf.ResponseOutputItemDoneEvent.Item
			index = int(event.OneOf.ResponseOutputItemDoneEvent.OutputIndex)
		}
		if call, ok := item.OneOf.GetItemFunctionToolCall(); ok {
			delta, err := s.toolDelta(call, index)
			if err != nil {
				return nil, false, err
			}
			msg.ToolCalls = []schema.ToolCall{delta}
		}
		if err := s.record(msg); err != nil {
			return nil, false, err
		}
		return msg, false, nil
	}
	var resp *responses.Response
	switch event.OneOf.Type {
	case responses.ResponseCompletedEventResponseStreamEventSum:
		resp = &event.OneOf.ResponseCompletedEvent.Response
	case responses.ResponseIncompleteEventResponseStreamEventSum:
		resp = &event.OneOf.ResponseIncompleteEvent.Response
	default:
		return msg, terminal, nil
	}
	var toolDeltas []schema.ToolCall
	for index, output := range resp.Output {
		call, ok := output.OneOf.GetItemFunctionToolCall()
		if !ok {
			continue
		}
		delta, err := s.toolDelta(call, index)
		if err != nil {
			return nil, true, err
		}
		toolDeltas = append(toolDeltas, delta)
	}
	msg.ToolCalls = toolDeltas
	if err := s.record(msg); err != nil {
		return nil, true, err
	}

	return msg, true, nil
}

// Reconcile values as deltas but emit metadata atomically, including status
// updates when all function arguments were already sent before item.done.
func (s *responsesStreamState) toolDelta(call responses.ItemFunctionToolCall, index int) (schema.ToolCall, error) {
	seen := s.tools[index]
	if seen == nil {
		seen = &responsesStreamTool{}
	}
	delta := projectResponsesToolCall(call, index)
	var err error
	delta.ID, err = responsesStreamIdentity(call.CallID.Or(seen.id), seen.id, "tool call ID")
	if err != nil {
		return delta, err
	}
	delta.Function.Name, err = responsesStreamIdentity(call.Name.Or(seen.name), seen.name, "tool name")
	if err != nil {
		return delta, err
	}
	delta.Function.Arguments, err = responsesStreamSuffix(call.Arguments.Or(seen.arguments.String()), seen.arguments.String(), "tool arguments")
	return delta, err
}

// Canonical part chunks already contain only un-emitted suffixes in order.
// Derive the display from summary chunks alone, including added/done snapshots.
func (s *responsesStreamState) summaryDelta(parts []schema.MessageOutputPart) string {
	var text strings.Builder
	for _, part := range parts {
		if part.Reasoning == nil || part.Reasoning.Text == "" {
			continue
		}
		index := part.StreamingMeta.Index
		if s.summaryStarted && s.summaryIndex != index {
			text.WriteByte('\n')
		}
		text.WriteString(part.Reasoning.Text)
		s.summaryIndex, s.summaryStarted = index, true
	}
	return text.String()
}

// Eino merges tool IDs and names atomically; only arguments are text deltas.
func responsesStreamIdentity(full, emitted, field string) (string, error) {
	if emitted == "" {
		return full, nil
	}
	if full != emitted {
		return "", fmt.Errorf("Responses stream %s differs from terminal output", field)
	}
	return "", nil
}

func responsesStreamSuffix(full, emitted, field string) (string, error) {
	if !strings.HasPrefix(full, emitted) {
		return "", fmt.Errorf("Responses stream %s differs from terminal output", field)
	}
	return full[len(emitted):], nil
}

// Metadata is emitted once at the terminal event, so token usage and IDs do
// not get duplicated when Eino concatenates stream chunks.
func projectResponsesStreamEvent(event *responses.ResponseStreamEvent, req *responses.ResponsesRequest) (msg *schema.Message, terminal bool, err error) {
	if event == nil {
		return nil, false, nil
	}
	ev := &event.OneOf
	switch ev.Type {
	case responses.ResponseCompletedEventResponseStreamEventSum:
		msg, err = projectResponsesResponseParts(&ev.ResponseCompletedEvent.Response, req, true)
		return msg, true, err
	case responses.ResponseIncompleteEventResponseStreamEventSum:
		msg, err = projectResponsesResponseParts(&ev.ResponseIncompleteEvent.Response, req, true)
		return msg, true, err
	case responses.ResponseFailedEventResponseStreamEventSum:
		return nil, true, responseFailedError(&ev.ResponseFailedEvent.Response)
	case responses.ResponseErrorEventResponseStreamEventSum:
		return nil, false, fmt.Errorf("Ark stream error (%s): %s", ev.ResponseErrorEvent.Code.Or(""), ev.ResponseErrorEvent.Message)
	case responses.ResponseTextDeltaEventResponseStreamEventSum:
		msg = &schema.Message{Role: schema.Assistant, Content: ev.ResponseTextDeltaEvent.Delta.Or("")}
		msg.AssistantGenMultiContent = []schema.MessageOutputPart{projectResponsesTextPart(int(ev.ResponseTextDeltaEvent.OutputIndex), int(ev.ResponseTextDeltaEvent.ContentIndex), msg.Content)}
	case responses.ResponseReasoningTextDeltaEventResponseStreamEventSum:
		msg = &schema.Message{Role: schema.Assistant, AssistantGenMultiContent: []schema.MessageOutputPart{
			projectResponsesReasoningPart(int(ev.ResponseReasoningTextDeltaEvent.OutputIndex), int(ev.ResponseReasoningTextDeltaEvent.ContentIndex), ev.ResponseReasoningTextDeltaEvent.Delta.Or("")),
		}}
	case responses.ResponseReasoningRawTextDeltaEventResponseStreamEventSum:
		msg = &schema.Message{Role: schema.Assistant, AssistantGenMultiContent: []schema.MessageOutputPart{
			projectResponsesReasoningPart(int(ev.ResponseReasoningRawTextDeltaEvent.OutputIndex), int(ev.ResponseReasoningRawTextDeltaEvent.ContentIndex), ev.ResponseReasoningRawTextDeltaEvent.Delta.Or("")),
		}}
	case responses.ResponseReasoningSummaryTextDeltaEventResponseStreamEventSum:
		msg = &schema.Message{Role: schema.Assistant, ReasoningContent: ev.ResponseReasoningSummaryTextDeltaEvent.Delta.Or("")}
		msg.AssistantGenMultiContent = []schema.MessageOutputPart{projectResponsesSummaryPart(int(ev.ResponseReasoningSummaryTextDeltaEvent.OutputIndex), int(ev.ResponseReasoningSummaryTextDeltaEvent.SummaryIndex), msg.ReasoningContent)}
	case responses.ResponseReasoningSummaryTextDoneEventResponseStreamEventSum:
		e := ev.ResponseReasoningSummaryTextDoneEvent
		msg = &schema.Message{Role: schema.Assistant, AssistantGenMultiContent: []schema.MessageOutputPart{projectResponsesSummaryPart(int(e.OutputIndex), int(e.SummaryIndex), e.Text.Or(""))}}
	case responses.ResponseContentPartAddedEventResponseStreamEventSum:
		e := ev.ResponseContentPartAddedEvent
		msg, err = projectResponsesStreamContentPart(e.Part, int(e.OutputIndex), int(e.ContentIndex))
	case responses.ResponseContentPartDoneEventResponseStreamEventSum:
		e := ev.ResponseContentPartDoneEvent
		msg, err = projectResponsesStreamContentPart(e.Part, int(e.OutputIndex), int(e.ContentIndex))
	case responses.ResponseReasoningSummaryPartAddedEventResponseStreamEventSum:
		e := ev.ResponseReasoningSummaryPartAddedEvent
		msg = &schema.Message{Role: schema.Assistant, AssistantGenMultiContent: []schema.MessageOutputPart{projectResponsesSummaryPart(int(e.OutputIndex), int(e.SummaryIndex), e.Part.Text.Or(""))}}
	case responses.ResponseReasoningSummaryPartDoneEventResponseStreamEventSum:
		e := ev.ResponseReasoningSummaryPartDoneEvent
		msg = &schema.Message{Role: schema.Assistant, AssistantGenMultiContent: []schema.MessageOutputPart{projectResponsesSummaryPart(int(e.OutputIndex), int(e.SummaryIndex), e.Part.Text.Or(""))}}
	case responses.ResponseOutputItemDoneEventResponseStreamEventSum:
		done := ev.ResponseOutputItemDoneEvent
		parts, e := projectResponsesItem(done.Item, int(done.OutputIndex))
		if e != nil {
			return nil, false, e
		}
		msg = &schema.Message{Role: schema.Assistant, AssistantGenMultiContent: parts}
		if call, ok := done.Item.OneOf.GetItemFunctionToolCall(); ok {
			msg.ToolCalls = []schema.ToolCall{projectResponsesToolCall(call, int(done.OutputIndex))}
		}
	case responses.ResponseOutputItemAddedEventResponseStreamEventSum:
		added := ev.ResponseOutputItemAddedEvent
		if call, ok := added.Item.OneOf.GetItemFunctionToolCall(); ok {
			msg = &schema.Message{Role: schema.Assistant, ToolCalls: []schema.ToolCall{projectResponsesToolCall(call, int(added.OutputIndex))}}
		} else if added.Item.OneOf.Type == responses.ItemOutputMessageOutputItemSum || added.Item.OneOf.Type == responses.ItemReasoningOutputItemSum {
			parts, e := projectResponsesItem(added.Item, int(added.OutputIndex))
			if e != nil {
				return nil, false, e
			}
			msg = &schema.Message{Role: schema.Assistant}
			for _, part := range parts {
				if section, ok := extraString(part.Extra, keyResponsesSection); ok && section != "" {
					msg.AssistantGenMultiContent = append(msg.AssistantGenMultiContent, part)
				}
			}
		}
	case responses.ResponseFunctionCallArgumentsDeltaEventResponseStreamEventSum:
		delta := ev.ResponseFunctionCallArgumentsDeltaEvent
		index := int(delta.OutputIndex)
		msg = &schema.Message{Role: schema.Assistant, ToolCalls: []schema.ToolCall{{Extra: map[string]any{keyResponsesItemIndex: metadataInt64(index)}, Function: schema.FunctionCall{Arguments: delta.Delta.Or("")}}}}
	}
	return msg, false, err
}

func projectResponsesStreamContentPart(part responses.OutputContentItem, itemIndex, contentIndex int) (*schema.Message, error) {
	projected, err := projectResponsesOutputContentPart(part, itemIndex, contentIndex)
	if err != nil {
		return nil, err
	}
	return &schema.Message{Role: schema.Assistant, AssistantGenMultiContent: []schema.MessageOutputPart{projected}}, nil
}

type responsesPartKey struct {
	section string
	index   int
}

type responsesStreamPart struct {
	prototype schema.MessageOutputPart
	received  strings.Builder
	emitted   string
	started   bool
	pending   bool
}

type responsesStreamItem struct {
	projection       reasoningItemProjection
	publicIndices    map[responsesPartKey]int
	kind             responses.OutputItemSumType
	parts            map[responsesPartKey]*responsesStreamPart
	pending          []responsesPartKey
	lastSummaryIndex int
	source           []schema.MessageOutputPart
	done, closed     bool
	signature        string
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
	if !terminal && msg != nil && ev.Type != responses.ResponseOutputItemDoneEventResponseStreamEventSum {
		snapshot := false
		switch ev.Type {
		case responses.ResponseOutputItemAddedEventResponseStreamEventSum,
			responses.ResponseContentPartAddedEventResponseStreamEventSum,
			responses.ResponseContentPartDoneEventResponseStreamEventSum,
			responses.ResponseReasoningSummaryPartAddedEventResponseStreamEventSum,
			responses.ResponseReasoningSummaryPartDoneEventResponseStreamEventSum,
			responses.ResponseReasoningSummaryTextDoneEventResponseStreamEventSum:
			snapshot = true
		}

		for _, part := range msg.AssistantGenMultiContent {
			index, _, err := extraIndex(part.Extra, keyResponsesItemIndex)
			if err != nil {
				return nil, err
			}
			if err := s.item(index).recordSegment(index, part, snapshot, index == s.nextItem); err != nil {
				return nil, err
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
		// ToolCalls occupy SDK output slots but produce no content parts.
		var outputCount int
		switch ev.Type {
		case responses.ResponseCompletedEventResponseStreamEventSum:
			outputCount = len(ev.ResponseCompletedEvent.Response.Output)
		case responses.ResponseIncompleteEventResponseStreamEventSum:
			outputCount = len(ev.ResponseIncompleteEvent.Response.Output)
		}
		for index, item := range s.items {
			if index >= outputCount && (len(item.parts) > 0 || item.kind != "") {
				return nil, fmt.Errorf("terminal response omitted streamed item %d", index)
			}
		}
		for index := 0; index < outputCount; index++ {
			if err := s.finish(index, groups[index], true); err != nil {
				return nil, err
			}
		}
	}
	return s.drain()
}

func (item *responsesStreamItem) recordSegment(index int, part schema.MessageOutputPart, snapshot, active bool) error {
	key := responsesPartIdentity(part)
	if key.index < 0 {
		return fmt.Errorf("negative Responses content index")
	}

	if key.section == "content" &&
		((item.kind == responses.ItemReasoningOutputItemSum && part.Type == schema.ChatMessagePartTypeText) ||
			(item.kind == responses.ItemOutputMessageOutputItemSum && part.Type == schema.ChatMessagePartTypeReasoning)) {
		return fmt.Errorf("Responses item %d (%s) cannot contain a %s part", index, item.kind, part.Type)
	}
	if item.closed {
		return fmt.Errorf("Responses item %d received a delta after output_item.done", index)
	}
	cell := item.parts[key]
	if cell == nil {
		cell = &responsesStreamPart{prototype: copyResponsesPart(part)}
		if cell.prototype.Reasoning != nil {
			cell.prototype.Reasoning.Signature = ""
		}
		item.parts[key] = cell
	}
	delta := responsesPartText(part)
	if snapshot {
		if delta != "" {
			suffix, ok := strings.CutPrefix(delta, cell.received.String())
			if !ok {
				return fmt.Errorf("Responses part snapshot differs from its delta prefix")
			}
			delta = suffix
		}
		cell.prototype = copyResponsesPart(part)
		if cell.prototype.Reasoning != nil {
			cell.prototype.Reasoning.Signature = ""
		}
	}
	if key.section == "summary" && delta != "" {
		if key.index < item.lastSummaryIndex {
			return fmt.Errorf("Responses item %d summary_index decreased from %d to %d", index, item.lastSummaryIndex, key.index)
		}
		item.lastSummaryIndex = key.index
	}
	cell.received.WriteString(delta)
	// Message parts must stay adjacent for Eino Concat. Reasoning segments
	// share one public part; raw slots can stream once their position is known.
	ready := key.index == 0 && part.Type == schema.ChatMessagePartTypeText
	if part.Reasoning != nil {
		ready = key.section == "summary" || key.index <= len(item.projection.rawSlots)
	}
	if ready && !cell.pending && (delta != "" || (active && (snapshot || !cell.started))) {
		cell.pending = true
		item.pending = append(item.pending, key)
	}
	return nil
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
				for _, part := range parts {
					if part.Reasoning != nil && responsesPartIdentity(part).section != "" {
						if err := item.recordSegment(index, part, true, index == s.nextItem); err != nil {
							return err
						}
					}
				}
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
		var segments []schema.MessageOutputPart
		if !item.done {
			for _, key := range item.pending {
				cell := item.parts[key]
				part := copyResponsesPart(cell.prototype)
				setResponsesPartText(&part, cell.received.String()[len(cell.emitted):])
				cell.started = true
				cell.emitted = cell.received.String()
				cell.pending = false
				segments = append(segments, part)
			}
			item.pending = item.pending[:0]
			out = append(out, s.publicParts(item, segments)...)
			break
		}
		full := s.completeParts(item)
		for _, original := range full {
			part := copyResponsesPart(original)
			partKey := responsesPartIdentity(part)
			cell := item.parts[partKey]
			if cell == nil {
				cell = &responsesStreamPart{}
				item.parts[partKey] = cell
			}
			text := responsesPartText(part)
			if text == "" && cell.received.Len() > 0 {
				text = cell.received.String()
			}
			if !strings.HasPrefix(text, cell.received.String()) || !strings.HasPrefix(text, cell.emitted) {
				return nil, fmt.Errorf("Responses item %d part %v differs from its streamed prefix", s.nextItem, partKey)
			}
			suffix := text[len(cell.emitted):]
			if partKey.section == "summary" && suffix != "" && partKey.index < item.projection.summaryIndex {
				return nil, fmt.Errorf("Responses item %d snapshot extends summary_index %d after index %d was emitted", s.nextItem, partKey.index, item.projection.summaryIndex)
			}
			setResponsesPartText(&part, suffix)
			cell.started = true
			cell.emitted = text
			segments = append(segments, part)
		}
		out = append(out, s.publicParts(item, segments)...)
		item.closed = true
		s.nextItem++
	}
	return out, nil
}

func (s *responsesPartsStream) publicParts(item *responsesStreamItem, segments []schema.MessageOutputPart) []schema.MessageOutputPart {
	parts := item.projection.parts(segments)
	for i := range parts {
		key := responsesPartIdentity(parts[i])
		if item.publicIndices == nil {
			item.publicIndices = make(map[responsesPartKey]int)
		}
		index, ok := item.publicIndices[key]
		if !ok {
			index = s.nextIndex
			s.nextIndex++
			item.publicIndices[key] = index
		}
		parts[i].StreamingMeta = &schema.MessageStreamingMeta{Index: index}
	}
	// Eino merges adjacent chunks. A terminal snapshot can omit an already
	// streamed part, which completeParts restores after the SDK's segments.
	if item.done && len(parts) > 1 {
		sort.SliceStable(parts, func(i, j int) bool { return parts[i].StreamingMeta.Index < parts[j].StreamingMeta.Index })
	}
	return parts
}

func (s *responsesPartsStream) completeParts(item *responsesStreamItem) []schema.MessageOutputPart {
	full := item.source
	keys := make(map[responsesPartKey]bool, len(full))
	for _, part := range full {
		keys[responsesPartIdentity(part)] = true
	}

	// A terminal response may omit plaintext that was supplied through deltas.
	var missing []responsesPartKey
	for key, cell := range item.parts {
		if !keys[key] && (cell.received.Len() > 0 || cell.started) {
			missing = append(missing, key)
		}
	}
	sort.Slice(missing, func(i, j int) bool {
		if missing[i].section != missing[j].section {
			return missing[i].section < missing[j].section
		}
		return missing[i].index < missing[j].index
	})
	if len(missing) > 0 {
		full = append([]schema.MessageOutputPart(nil), full...)
		for _, key := range missing {
			cell := item.parts[key]
			part := copyResponsesPart(cell.prototype)
			// The terminal source owns item metadata; only recover the missing segment.
			delete(part.Extra, keyResponsesItemID)
			delete(part.Extra, keyResponsesItemType)
			delete(part.Extra, keyResponsesItemStatus)
			delete(part.Extra, keyResponsesItemRole)
			delete(part.Extra, keyResponsesPartial)
			setResponsesPartText(&part, cell.received.String())
			full = append(full, part)
		}
	}

	return full
}

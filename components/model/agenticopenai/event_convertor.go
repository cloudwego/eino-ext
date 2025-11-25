/*
 * Copyright 2026 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package agenticopenai

import (
	"fmt"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/eino/schema/openai"
	"github.com/openai/openai-go/v3/packages/ssestream"
	"github.com/openai/openai-go/v3/responses"
)

func receivedStreamingResponse(sr *ssestream.Stream[responses.ResponseStreamEventUnion],
	config *model.AgenticConfig, sw *schema.StreamWriter[*model.AgenticCallbackOutput]) {

	receiver := newStreamReceiver()
	sender := newCallbackSender(sw, config)

	if sr.Err() != nil {
		_ = sw.Send(nil, fmt.Errorf("failed to read stream: %w", sr.Err()))
		return
	}

	for sr.Next() {
		event := sr.Current()

		sender.errHeader = fmt.Sprintf("failed to convert event '%s'", event.Type)

		switch variant := event.AsAny().(type) {
		case responses.ResponseTextDoneEvent,
			responses.ResponseReasoningSummaryPartAddedEvent,
			responses.ResponseReasoningSummaryPartDoneEvent,
			responses.ResponseReasoningSummaryTextDoneEvent,
			responses.ResponseFunctionCallArgumentsDoneEvent,
			responses.ResponseMcpCallArgumentsDoneEvent,
			responses.ResponseRefusalDoneEvent:

			// Do nothing.
			continue

		case responses.ResponseErrorEvent:
			_ = sw.Send(nil, fmt.Errorf("received error event: code=%s message=%s", variant.Code, variant.Message))

		case responses.ResponseCreatedEvent:
			meta := responseObjectToResponseMeta(&variant.Response)
			sender.sendMeta(meta, nil)

		case responses.ResponseInProgressEvent:
			meta := responseObjectToResponseMeta(&variant.Response)
			sender.sendMeta(meta, nil)

		case responses.ResponseCompletedEvent:
			meta := responseObjectToResponseMeta(&variant.Response)
			sender.sendMeta(meta, nil)

		case responses.ResponseIncompleteEvent:
			meta := responseObjectToResponseMeta(&variant.Response)
			sender.sendMeta(meta, nil)

		case responses.ResponseFailedEvent:
			meta := responseObjectToResponseMeta(&variant.Response)
			sender.sendMeta(meta, nil)

		case responses.ResponseOutputItemAddedEvent:
			blocks, err := receiver.itemAddedEventToContentBlock(variant)
			for _, block := range blocks {
				sender.sendBlock(block, err)
			}

		case responses.ResponseOutputItemDoneEvent:
			blocks, err := receiver.itemDoneEventToContentBlocks(variant)
			for _, block := range blocks {
				sender.sendBlock(block, err)
			}

		case responses.ResponseContentPartAddedEvent:
			block, err := receiver.contentPartAddedEventToContentBlock(variant)
			sender.sendBlock(block, err)

		case responses.ResponseContentPartDoneEvent:
			block, err := receiver.contentPartDoneEventToContentBlock(variant)
			sender.sendBlock(block, err)

		case responses.ResponseRefusalDeltaEvent:
			block := receiver.refusalDeltaEventToContentBlock(variant)
			sender.sendBlock(block, nil)

		case responses.ResponseTextDeltaEvent:
			block := receiver.outputTextDeltaEventToContentBlock(variant)
			sender.sendBlock(block, nil)

		case responses.ResponseOutputTextAnnotationAddedEvent:
			block, err := receiver.annotationAddedEventToContentBlock(variant)
			sender.sendBlock(block, err)

		case responses.ResponseReasoningSummaryTextDeltaEvent:
			block := receiver.reasoningSummaryTextDeltaEventToContentBlock(variant)
			sender.sendBlock(block, nil)

		case responses.ResponseFunctionCallArgumentsDeltaEvent:
			block := receiver.functionCallArgumentsDeltaEventToContentBlock(variant)
			sender.sendBlock(block, nil)

		case responses.ResponseMcpListToolsInProgressEvent:
			block := receiver.mcpListToolsPhaseToContentBlock(variant.ItemID, variant.OutputIndex, string(responses.ResponseStatusInProgress))
			sender.sendBlock(block, nil)

		case responses.ResponseMcpListToolsFailedEvent:
			block := receiver.mcpListToolsPhaseToContentBlock(variant.ItemID, variant.OutputIndex, string(responses.ResponseStatusFailed))
			sender.sendBlock(block, nil)

		case responses.ResponseMcpListToolsCompletedEvent:
			block := receiver.mcpListToolsPhaseToContentBlock(variant.ItemID, variant.OutputIndex, string(responses.ResponseStatusCompleted))
			sender.sendBlock(block, nil)

		case responses.ResponseMcpCallArgumentsDeltaEvent:
			block := receiver.mcpCallArgumentsDeltaEventToContentBlock(variant)
			sender.sendBlock(block, nil)

		case responses.ResponseMcpCallInProgressEvent:
			block := receiver.mcpCallPhaseToContentBlock(variant.ItemID, variant.OutputIndex, string(responses.ResponseStatusInProgress))
			sender.sendBlock(block, nil)

		case responses.ResponseMcpCallCompletedEvent:
			block := receiver.mcpCallPhaseToContentBlock(variant.ItemID, variant.OutputIndex, string(responses.ResponseStatusCompleted))
			sender.sendBlock(block, nil)

		case responses.ResponseMcpCallFailedEvent:
			block := receiver.mcpCallPhaseToContentBlock(variant.ItemID, variant.OutputIndex, string(responses.ResponseStatusFailed))
			sender.sendBlock(block, nil)

		case responses.ResponseWebSearchCallInProgressEvent:
			block := receiver.webSearchPhaseToContentBlock(variant.ItemID, variant.OutputIndex, string(responses.ResponseStatusInProgress))
			sender.sendBlock(block, nil)

		case responses.ResponseWebSearchCallSearchingEvent:
			const phase = "searching"
			block := receiver.webSearchPhaseToContentBlock(variant.ItemID, variant.OutputIndex, phase)
			sender.sendBlock(block, nil)

		case responses.ResponseWebSearchCallCompletedEvent:
			block := receiver.webSearchPhaseToContentBlock(variant.ItemID, variant.OutputIndex, string(responses.ResponseStatusCompleted))
			sender.sendBlock(block, nil)

		default:
			sw.Send(nil, fmt.Errorf("invalid event type: %s", event.Type))
		}
	}

	if sr.Err() != nil {
		_ = sw.Send(nil, fmt.Errorf("failed to read stream: %w", sr.Err()))
		return
	}
}

type callbackSender struct {
	sw        *schema.StreamWriter[*model.AgenticCallbackOutput]
	config    *model.AgenticConfig
	errHeader string
}

func newCallbackSender(sw *schema.StreamWriter[*model.AgenticCallbackOutput], config *model.AgenticConfig) *callbackSender {
	return &callbackSender{
		sw:     sw,
		config: config,
	}
}

func (s *callbackSender) sendMeta(meta *schema.AgenticResponseMeta, err error) {
	s.send(meta, nil, err)
}

func (s *callbackSender) sendBlock(block *schema.ContentBlock, err error) {
	s.send(nil, block, err)
}

func (s *callbackSender) send(meta *schema.AgenticResponseMeta, block *schema.ContentBlock, err error) {
	if err != nil {
		_ = s.sw.Send(nil, fmt.Errorf("%s: %w", s.errHeader, err))
		return
	}

	msg := &schema.AgenticMessage{
		Role:         schema.AgenticRoleTypeAssistant,
		ResponseMeta: meta,
	}
	if block != nil {
		msg.ContentBlocks = []*schema.ContentBlock{block}
	}

	s.sw.Send(&model.AgenticCallbackOutput{
		Message: msg,
		Config:  s.config,
	}, nil)
}

type streamReceiver struct {
	ProcessingAssistantGenTextBlockIndex map[string]map[int]bool

	MaxBlockIndex int
	IndexMapper   map[string]int

	MaxReasoningSummaryIndex    map[string]int
	ReasoningSummaryIndexMapper map[string]int

	MaxTextAnnotationIndex    map[string]int
	TextAnnotationIndexMapper map[string]int

	ItemAddedEventCache map[string]any
}

func newStreamReceiver() *streamReceiver {
	return &streamReceiver{
		ProcessingAssistantGenTextBlockIndex: map[string]map[int]bool{},
		MaxBlockIndex:                        -1,
		IndexMapper:                          map[string]int{},
		MaxReasoningSummaryIndex:             map[string]int{},
		ReasoningSummaryIndexMapper:          map[string]int{},
		TextAnnotationIndexMapper:            map[string]int{},
		MaxTextAnnotationIndex:               map[string]int{},
		ItemAddedEventCache:                  map[string]any{},
	}
}

func (r *streamReceiver) getBlockIndex(key string) int {
	if idx, ok := r.IndexMapper[key]; ok {
		return idx
	}

	r.MaxBlockIndex++
	r.IndexMapper[key] = r.MaxBlockIndex

	return r.MaxBlockIndex
}

func (r *streamReceiver) isNewReasoningSummaryIndex(outputIdx, summaryIdx int64) bool {
	maxSummaryIndex := -1
	if idx, ok := r.MaxReasoningSummaryIndex[int64ToStr(outputIdx)]; ok {
		maxSummaryIndex = idx
	}

	idxKey := fmt.Sprintf("%d:%d", outputIdx, summaryIdx)
	if _, ok := r.ReasoningSummaryIndexMapper[idxKey]; ok {
		return false
	}

	maxSummaryIndex++
	r.ReasoningSummaryIndexMapper[idxKey] = maxSummaryIndex
	r.MaxReasoningSummaryIndex[int64ToStr(outputIdx)] = maxSummaryIndex

	return true
}

func (r *streamReceiver) getTextAnnotationIndex(outputIdx, contentIdx, annotationIdx int64) int {
	maxAnnotationIndex := -1

	maxIdxKey := fmt.Sprintf("%d:%d", outputIdx, contentIdx)
	if idx, ok := r.MaxTextAnnotationIndex[maxIdxKey]; ok {
		maxAnnotationIndex = idx
	}

	idxKey := fmt.Sprintf("%d:%d:%d", outputIdx, contentIdx, annotationIdx)
	if idx, ok := r.TextAnnotationIndexMapper[idxKey]; ok {
		return idx
	}

	maxAnnotationIndex++
	r.TextAnnotationIndexMapper[idxKey] = maxAnnotationIndex
	r.MaxTextAnnotationIndex[maxIdxKey] = maxAnnotationIndex

	return maxAnnotationIndex
}

func (r *streamReceiver) itemAddedEventToContentBlock(ev responses.ResponseOutputItemAddedEvent) (blocks []*schema.ContentBlock, err error) {
	switch item := ev.Item.AsAny().(type) {
	case responses.ResponseFunctionToolCall:
		block, err := r.itemAddedEventFunctionToolCallToContentBlock(ev.OutputIndex, item)
		if err != nil {
			return nil, err
		}

		blocks = append(blocks, block)

	case responses.ResponseReasoningItem:
		block, err := r.itemAddedEventReasoningToContentBlock(ev.OutputIndex, item)
		if err != nil {
			return nil, err
		}

		blocks = append(blocks, block)

	case responses.ResponseOutputItemMcpCall:
		k := makeMCPToolCallItemAddedEventCacheKey(item.ID, ev.OutputIndex)
		r.ItemAddedEventCache[k] = item

	case responses.ResponseOutputItemMcpListTools:
		k := makeMCPListToolsItemAddedEventCacheKey(item.ID, ev.OutputIndex)
		r.ItemAddedEventCache[k] = item

	case responses.ResponseOutputMessage,
		responses.ResponseFunctionWebSearch,
		responses.ResponseOutputItemMcpApprovalRequest:

		// Do nothing.

	default:
		return nil, fmt.Errorf("invalid item type %T with 'output_item.added' event", item)
	}

	return blocks, nil
}

func (r *streamReceiver) itemAddedEventFunctionToolCallToContentBlock(outputIdx int64, item responses.ResponseFunctionToolCall) (block *schema.ContentBlock, err error) {
	block, err = functionToolCallToContentBlock(item)
	if err != nil {
		return nil, err
	}

	block.StreamingMeta = &schema.StreamingMeta{
		Index: r.getBlockIndex(makeFunctionToolCallIndexKey(outputIdx)),
	}

	return block, nil
}

func (r *streamReceiver) itemAddedEventReasoningToContentBlock(outputIdx int64, item responses.ResponseReasoningItem) (block *schema.ContentBlock, err error) {
	block, err = reasoningToContentBlocks(item)
	if err != nil {
		return nil, err
	}

	block.StreamingMeta = &schema.StreamingMeta{
		Index: r.getBlockIndex(makeReasoningIndexKey(outputIdx)),
	}

	return block, nil
}

func (r *streamReceiver) itemDoneEventToContentBlocks(ev responses.ResponseOutputItemDoneEvent) (blocks []*schema.ContentBlock, err error) {
	switch item := ev.Item.AsAny().(type) {
	case responses.ResponseOutputMessage:
		blocks, err = r.itemDoneEventOutputMessageToContentBlock(item)
		if err != nil {
			return nil, err
		}

	case responses.ResponseReasoningItem:
		block, err := r.itemDoneEventReasoningToContentBlock(ev.OutputIndex, item)
		if err != nil {
			return nil, err
		}

		blocks = append(blocks, block)

	case responses.ResponseFunctionToolCall:
		block, err := r.itemDoneEventFunctionToolCallToContentBlock(ev.OutputIndex, item)
		if err != nil {
			return nil, err
		}

		blocks = append(blocks, block)

	case responses.ResponseFunctionWebSearch:
		blocks, err = r.itemDoneEventFunctionWebSearchToContentBlocks(ev.OutputIndex, item)
		if err != nil {
			return nil, err
		}

	case responses.ResponseOutputItemMcpCall:
		blocks, err = r.itemDoneEventFunctionMCPCallToContentBlocks(ev.OutputIndex, item)
		if err != nil {
			return nil, err
		}

	case responses.ResponseOutputItemMcpListTools:
		block, err := r.itemDoneEventFunctionMCPListToolsToContentBlock(ev.OutputIndex, item)
		if err != nil {
			return nil, err
		}

		blocks = append(blocks, block)

	case responses.ResponseOutputItemMcpApprovalRequest:
		block, err := r.itemDoneEventFunctionMCPApprovalRequestToContentBlock(ev.OutputIndex, item)
		if err != nil {
			return nil, err
		}

		blocks = append(blocks, block)

	default:
		return nil, fmt.Errorf("invalid item type %T with 'output_item.done' event", item)
	}

	return blocks, nil
}

func (r *streamReceiver) itemDoneEventOutputMessageToContentBlock(item responses.ResponseOutputMessage) (blocks []*schema.ContentBlock, err error) {
	indices, ok := r.ProcessingAssistantGenTextBlockIndex[item.ID]
	if !ok {
		return nil, fmt.Errorf("item %s not found in processing queue", item.ID)
	}

	for idx := range indices {
		meta := &schema.StreamingMeta{Index: idx}
		block := schema.NewContentBlockChunk(&schema.AssistantGenText{}, meta)

		setItemID(block, item.ID)
		if string(item.Status) != "" {
			setItemStatus(block, string(item.Status))
		}

		blocks = append(blocks, block)
	}

	return blocks, nil
}

func (r *streamReceiver) itemDoneEventReasoningToContentBlock(outputIdx int64, item responses.ResponseReasoningItem) (block *schema.ContentBlock, err error) {
	meta := &schema.StreamingMeta{
		Index: r.getBlockIndex(makeReasoningIndexKey(outputIdx)),
	}
	block = schema.NewContentBlockChunk(&schema.Reasoning{}, meta)

	setItemID(block, item.ID)
	if s := string(item.Status); s != "" {
		setItemStatus(block, s)
	}

	return block, nil
}

func (r *streamReceiver) itemDoneEventFunctionToolCallToContentBlock(outputIdx int64, item responses.ResponseFunctionToolCall) (block *schema.ContentBlock, err error) {
	meta := &schema.StreamingMeta{
		Index: r.getBlockIndex(makeFunctionToolCallIndexKey(outputIdx)),
	}
	block = schema.NewContentBlockChunk(&schema.FunctionToolCall{
		CallID: item.CallID,
		Name:   item.Name,
	}, meta)

	setItemID(block, item.ID)
	if s := string(item.Status); s != "" {
		setItemStatus(block, s)
	}

	return block, nil
}

func (r *streamReceiver) itemDoneEventFunctionWebSearchToContentBlocks(outputIdx int64, item responses.ResponseFunctionWebSearch) (blocks []*schema.ContentBlock, err error) {
	blocks, err = webSearchToContentBlocks(item)
	if err != nil {
		return nil, err
	}

	blocks[0].StreamingMeta = &schema.StreamingMeta{
		Index: r.getBlockIndex(makeServerToolCallIndexKey(outputIdx)),
	}
	blocks[1].StreamingMeta = &schema.StreamingMeta{
		Index: r.getBlockIndex(makeServerToolResultIndexKey(outputIdx)),
	}

	return blocks, nil
}

func (r *streamReceiver) itemDoneEventFunctionMCPCallToContentBlocks(outputIdx int64, item responses.ResponseOutputItemMcpCall) (blocks []*schema.ContentBlock, err error) {
	blocks, err = mcpCallToContentBlocks(item)
	if err != nil {
		return nil, err
	}

	for _, block := range blocks {
		switch block.Type {
		case schema.ContentBlockTypeMCPToolCall:
			block.StreamingMeta = &schema.StreamingMeta{
				Index: r.getBlockIndex(makeMCPToolCallIndexKey(outputIdx)),
			}
		case schema.ContentBlockTypeMCPToolResult:
			block.StreamingMeta = &schema.StreamingMeta{
				Index: r.getBlockIndex(makeMCPToolResultIndexKey(outputIdx)),
			}
		default:
			return nil, fmt.Errorf("expected MCP tool call or result block, but got %q", block.Type)
		}
	}

	return blocks, nil
}

func (r *streamReceiver) itemDoneEventFunctionMCPListToolsToContentBlock(outputIdx int64, item responses.ResponseOutputItemMcpListTools) (block *schema.ContentBlock, err error) {
	block, err = mcpListToolsToContentBlock(item)
	if err != nil {
		return nil, err
	}

	block.StreamingMeta = &schema.StreamingMeta{
		Index: r.getBlockIndex(makeMCPListToolsResultIndexKey(outputIdx)),
	}

	return block, nil
}

func (r *streamReceiver) itemDoneEventFunctionMCPApprovalRequestToContentBlock(outputIdx int64, item responses.ResponseOutputItemMcpApprovalRequest) (block *schema.ContentBlock, err error) {
	block, err = mcpApprovalRequestToContentBlock(item)
	if err != nil {
		return nil, err
	}

	block.StreamingMeta = &schema.StreamingMeta{
		Index: r.getBlockIndex(makeMCPToolApprovalRequestIndexKey(outputIdx)),
	}

	return block, nil
}

func (r *streamReceiver) contentPartAddedEventToContentBlock(ev responses.ResponseContentPartAddedEvent) (block *schema.ContentBlock, err error) {
	key := makeAssistantGenTextIndexKey(ev.OutputIndex, ev.ContentIndex)
	blockIdx := r.getBlockIndex(key)

	indices, ok := r.ProcessingAssistantGenTextBlockIndex[ev.ItemID]
	if !ok {
		indices = map[int]bool{}
		r.ProcessingAssistantGenTextBlockIndex[ev.ItemID] = indices
	}

	indices[blockIdx] = true

	meta := &schema.StreamingMeta{Index: blockIdx}

	switch ev.Part.AsAny().(type) {
	case responses.ResponseOutputText, responses.ResponseOutputRefusal:
		block = schema.NewContentBlockChunk(&schema.AssistantGenText{}, meta)
	default:
		return nil, fmt.Errorf("invalid content part type: %T", ev.Part)
	}

	setItemStatus(block, string(responses.ResponseStatusInProgress))
	setItemID(block, ev.ItemID)

	return block, nil
}

func (r *streamReceiver) contentPartDoneEventToContentBlock(ev responses.ResponseContentPartDoneEvent) (block *schema.ContentBlock, err error) {
	key := makeAssistantGenTextIndexKey(ev.OutputIndex, ev.ContentIndex)
	blockIdx := r.getBlockIndex(key)

	indices, ok := r.ProcessingAssistantGenTextBlockIndex[ev.ItemID]
	if !ok {
		return nil, fmt.Errorf("item %q has no processing assistant gen text block index", ev.ItemID)
	}

	delete(indices, blockIdx)

	meta := &schema.StreamingMeta{Index: blockIdx}

	switch ev.Part.AsAny().(type) {
	case responses.ResponseOutputText:
		block = schema.NewContentBlockChunk(&schema.AssistantGenText{}, meta)
	default:
		return nil, fmt.Errorf("invalid content part type: %T", ev.Part)
	}

	block.StreamingMeta = &schema.StreamingMeta{
		Index: blockIdx,
	}

	setItemStatus(block, string(responses.ResponseStatusCompleted))
	setItemID(block, ev.ItemID)

	return block, nil
}

func (r *streamReceiver) refusalDeltaEventToContentBlock(ev responses.ResponseRefusalDeltaEvent) *schema.ContentBlock {
	meta := &schema.StreamingMeta{
		Index: r.getBlockIndex(makeAssistantGenTextIndexKey(ev.OutputIndex, ev.ContentIndex)),
	}
	block := schema.NewContentBlockChunk(&schema.AssistantGenText{
		OpenAIExtension: &openai.AssistantGenTextExtension{
			Refusal: &openai.OutputRefusal{
				Reason: ev.Delta,
			},
		},
	}, meta)

	setItemID(block, ev.ItemID)

	return block
}

func (r *streamReceiver) outputTextDeltaEventToContentBlock(ev responses.ResponseTextDeltaEvent) *schema.ContentBlock {
	meta := &schema.StreamingMeta{
		Index: r.getBlockIndex(makeAssistantGenTextIndexKey(ev.OutputIndex, ev.ContentIndex)),
	}
	block := schema.NewContentBlockChunk(&schema.AssistantGenText{
		Text: ev.Delta,
	}, meta)

	setItemID(block, ev.ItemID)

	return block
}

func (r *streamReceiver) annotationAddedEventToContentBlock(ev responses.ResponseOutputTextAnnotationAddedEvent) (block *schema.ContentBlock, err error) {
	annoBytes, err := sonic.Marshal(ev.Annotation)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal annotation: %w", err)
	}

	anno := responses.ResponseOutputTextAnnotationUnion{}
	err = sonic.Unmarshal(annoBytes, &anno)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal annotation: %w", err)
	}

	annotation, err := outputTextAnnotationToTextAnnotation(anno)
	if err != nil {
		return nil, fmt.Errorf("failed to convert annotation: %w", err)
	}

	annotation.Index = r.getTextAnnotationIndex(ev.OutputIndex, ev.ContentIndex, ev.AnnotationIndex)

	genText := &schema.AssistantGenText{
		OpenAIExtension: &openai.AssistantGenTextExtension{
			Annotations: []*openai.TextAnnotation{annotation},
		},
	}
	meta := &schema.StreamingMeta{
		Index: r.getBlockIndex(makeAssistantGenTextIndexKey(ev.OutputIndex, ev.ContentIndex)),
	}
	block = schema.NewContentBlockChunk(genText, meta)

	setItemID(block, ev.ItemID)

	return block, nil
}

func (r *streamReceiver) reasoningSummaryTextDeltaEventToContentBlock(ev responses.ResponseReasoningSummaryTextDeltaEvent) *schema.ContentBlock {
	text := ev.Delta
	if r.isNewReasoningSummaryIndex(ev.OutputIndex, ev.SummaryIndex) && ev.SummaryIndex != 0 {
		text = "\n" + text
	}

	reasoning := &schema.Reasoning{
		Text: text,
	}

	meta := &schema.StreamingMeta{
		Index: r.getBlockIndex(makeReasoningIndexKey(ev.OutputIndex)),
	}
	block := schema.NewContentBlockChunk(reasoning, meta)

	setItemID(block, ev.ItemID)

	return block
}

func (r *streamReceiver) functionCallArgumentsDeltaEventToContentBlock(ev responses.ResponseFunctionCallArgumentsDeltaEvent) *schema.ContentBlock {
	meta := &schema.StreamingMeta{
		Index: r.getBlockIndex(makeFunctionToolCallIndexKey(ev.OutputIndex)),
	}
	block := schema.NewContentBlockChunk(&schema.FunctionToolCall{
		Arguments: ev.Delta,
	}, meta)

	setItemID(block, ev.ItemID)

	return block
}

func (r *streamReceiver) mcpListToolsPhaseToContentBlock(itemID string, outputIdx int64, status string) *schema.ContentBlock {
	meta := &schema.StreamingMeta{
		Index: r.getBlockIndex(makeMCPListToolsResultIndexKey(outputIdx)),
	}

	content := &schema.MCPListToolsResult{}

	item, ok := r.ItemAddedEventCache[makeMCPListToolsItemAddedEventCacheKey(itemID, outputIdx)]
	if ok {
		item_, ok := item.(responses.ResponseOutputItemMcpListTools)
		if ok {
			content = &schema.MCPListToolsResult{
				ServerLabel: item_.ServerLabel,
			}
		}
	}

	block := schema.NewContentBlockChunk(content, meta)

	setItemID(block, itemID)
	if status != "" {
		setItemStatus(block, status)
	}

	return block
}

func (r *streamReceiver) mcpCallArgumentsDeltaEventToContentBlock(ev responses.ResponseMcpCallArgumentsDeltaEvent) *schema.ContentBlock {
	meta := &schema.StreamingMeta{
		Index: r.getBlockIndex(makeMCPToolCallIndexKey(ev.OutputIndex)),
	}
	block := schema.NewContentBlockChunk(&schema.MCPToolCall{
		Arguments: ev.Delta,
	}, meta)

	setItemID(block, ev.ItemID)

	return block
}

func (r *streamReceiver) mcpCallPhaseToContentBlock(itemID string, outputIdx int64, status string) *schema.ContentBlock {
	meta := &schema.StreamingMeta{
		Index: r.getBlockIndex(makeMCPToolCallIndexKey(outputIdx)),
	}

	content := &schema.MCPToolCall{}
	item, ok := r.ItemAddedEventCache[makeMCPToolCallItemAddedEventCacheKey(itemID, outputIdx)]
	if ok {
		item_, ok := item.(responses.ResponseOutputItemMcpCall)
		if ok {
			content = &schema.MCPToolCall{
				ServerLabel:       item_.ServerLabel,
				ApprovalRequestID: item_.ApprovalRequestID,
				Name:              item_.Name,
			}
		}
	}

	block := schema.NewContentBlockChunk(content, meta)

	setItemID(block, itemID)
	if status != "" {
		setItemStatus(block, status)
	}

	return block
}

func (r *streamReceiver) webSearchPhaseToContentBlock(itemID string, outputIdx int64, status string) *schema.ContentBlock {
	meta := &schema.StreamingMeta{
		Index: r.getBlockIndex(makeServerToolCallIndexKey(outputIdx)),
	}
	block := schema.NewContentBlockChunk(&schema.ServerToolCall{
		Name: string(ServerToolNameWebSearch),
	}, meta)

	setItemID(block, itemID)
	if status != "" {
		setItemStatus(block, status)
	}

	return block
}

func makeAssistantGenTextIndexKey(outputIndex, contentIndex int64) string {
	return fmt.Sprintf("assistant_gen_text:%d:%d", outputIndex, contentIndex)
}

func makeReasoningIndexKey(outputIndex int64) string {
	return fmt.Sprintf("reasoning:%d", outputIndex)
}

func makeFunctionToolCallIndexKey(outputIndex int64) string {
	return fmt.Sprintf("function_tool_call:%d", outputIndex)
}

func makeServerToolCallIndexKey(outputIndex int64) string {
	return fmt.Sprintf("server_tool_call:%d", outputIndex)
}

func makeServerToolResultIndexKey(outputIndex int64) string {
	return fmt.Sprintf("server_tool_result:%d", outputIndex)
}

func makeMCPListToolsResultIndexKey(outputIndex int64) string {
	return fmt.Sprintf("mcp_list_tools_result:%d", outputIndex)
}

func makeMCPToolApprovalRequestIndexKey(outputIndex int64) string {
	return fmt.Sprintf("mcp_tool_approval_request:%d", outputIndex)
}

func makeMCPToolCallIndexKey(outputIndex int64) string {
	return fmt.Sprintf("mcp_tool_call:%d", outputIndex)
}

func makeMCPToolResultIndexKey(outputIndex int64) string {
	return fmt.Sprintf("mcp_tool_result:%d", outputIndex)
}

func makeMCPToolCallItemAddedEventCacheKey(itemID string, outputIndex int64) string {
	return fmt.Sprintf("mcp_tool_call_item_added_event_cache:%s:%d", itemID, outputIndex)
}

func makeMCPListToolsItemAddedEventCacheKey(itemID string, outputIndex int64) string {
	return fmt.Sprintf("mcp_list_tools_item_added_event_cache:%s:%d", itemID, outputIndex)
}

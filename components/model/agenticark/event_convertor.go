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

package agenticark

import (
	"errors"
	"fmt"
	"io"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model/responses"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/utils"
)

func receivedStreamResponse(streamReader *utils.ResponsesStreamReader,
	config *model.AgenticConfig, sw *schema.StreamWriter[*model.AgenticCallbackOutput]) {

	receiver := newStreamReceiver()
	sender := newCallbackSender(sw, config)

	for {
		event, err := streamReader.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			_ = sw.Send(nil, fmt.Errorf("failed to read stream: %w", err))
			return
		}

		sender.errHeader = fmt.Sprintf("failed to convert event %q", event.GetEventType())

		switch ev := event.Event.(type) {
		case *responses.Event_TextDone,
			*responses.Event_ReasoningPart,
			*responses.Event_ReasoningPartDone,
			*responses.Event_ReasoningTextDone,
			*responses.Event_FunctionCallArgumentsDone,
			*responses.Event_ResponseMcpCallArgumentsDone,
			*responses.Event_ResponseMcpApprovalRequest:

			// Do nothing.
			continue

		case *responses.Event_Error:
			meta := receiver.errorEventToResponseMeta(ev.Error)
			sender.sendMeta(meta, nil)

		case *responses.Event_Response:
			meta := responseObjectToResponseMeta(ev.Response.Response)
			sender.sendMeta(meta, nil)

		case *responses.Event_ResponseInProgress:
			meta := responseObjectToResponseMeta(ev.ResponseInProgress.Response)
			sender.sendMeta(meta, nil)

		case *responses.Event_ResponseCompleted:
			meta := responseObjectToResponseMeta(ev.ResponseCompleted.Response)
			sender.sendMeta(meta, nil)

		case *responses.Event_ResponseIncomplete:
			meta := responseObjectToResponseMeta(ev.ResponseIncomplete.Response)
			sender.sendMeta(meta, nil)

		case *responses.Event_ResponseFailed:
			meta := responseObjectToResponseMeta(ev.ResponseFailed.Response)
			sender.sendMeta(meta, nil)

		case *responses.Event_Item:
			blocks, err := receiver.itemAddedEventToContentBlock(ev.Item)
			for _, block := range blocks {
				sender.sendBlock(block, err)
			}

		case *responses.Event_ItemDone:
			blocks, err := receiver.itemDoneEventToContentBlocks(ev.ItemDone)
			for _, block := range blocks {
				sender.sendBlock(block, err)
			}

		case *responses.Event_ContentPart:
			block, err := receiver.contentPartAddedEventToContentBlock(ev.ContentPart)
			sender.sendBlock(block, err)

		case *responses.Event_ContentPartDone:
			block, err := receiver.contentPartDoneEventToContentBlock(ev.ContentPartDone)
			sender.sendBlock(block, err)

		case *responses.Event_Text:
			block := receiver.outputTextDeltaEventToContentBlock(ev.Text)
			sender.sendBlock(block, nil)

		case *responses.Event_ResponseAnnotationAdded:
			block, err := receiver.annotationAddedEventToContentBlock(ev.ResponseAnnotationAdded)
			sender.sendBlock(block, err)

		case *responses.Event_ReasoningText:
			block := receiver.reasoningSummaryTextDeltaEventToContentBlock(ev.ReasoningText)
			sender.sendBlock(block, nil)

		case *responses.Event_FunctionCallArguments:
			block := receiver.functionCallArgumentsDeltaEventToContentBlock(ev.FunctionCallArguments)
			sender.sendBlock(block, nil)

		case *responses.Event_ResponseMcpListToolsInProgress:
			phase := ev.ResponseMcpListToolsInProgress
			block := receiver.mcpListToolsPhaseToContentBlock(phase.ItemId, phase.OutputIndex, responses.ItemStatus_in_progress)
			sender.sendBlock(block, nil)

		case *responses.Event_ResponseMcpListToolsCompleted:
			phase := ev.ResponseMcpListToolsCompleted
			block := receiver.mcpListToolsPhaseToContentBlock(phase.ItemId, phase.OutputIndex, responses.ItemStatus_completed)
			sender.sendBlock(block, nil)

		case *responses.Event_ResponseMcpCallArgumentsDelta:
			block := receiver.mcpCallArgumentsDeltaEventToContentBlock(ev.ResponseMcpCallArgumentsDelta)
			sender.sendBlock(block, nil)

		case *responses.Event_ResponseMcpCallInProgress:
			phase := ev.ResponseMcpCallInProgress
			block := receiver.mcpCallPhaseToContentBlock(phase.ItemId, phase.OutputIndex, responses.ItemStatus_in_progress)
			sender.sendBlock(block, nil)

		case *responses.Event_ResponseMcpCallCompleted:
			phase := ev.ResponseMcpCallCompleted
			block := receiver.mcpCallPhaseToContentBlock(phase.ItemId, phase.OutputIndex, responses.ItemStatus_completed)
			sender.sendBlock(block, nil)

		case *responses.Event_ResponseMcpCallFailed:
			phase := ev.ResponseMcpCallFailed
			block := receiver.mcpCallPhaseToContentBlock(phase.ItemId, phase.OutputIndex, responses.ItemStatus_failed)
			sender.sendBlock(block, nil)

		case *responses.Event_ResponseWebSearchCallInProgress:
			phase := ev.ResponseWebSearchCallInProgress
			block := receiver.webSearchPhaseToContentBlock(phase.ItemId, phase.OutputIndex, responses.ItemStatus_in_progress)
			sender.sendBlock(block, nil)

		case *responses.Event_ResponseWebSearchCallSearching:
			phase := ev.ResponseWebSearchCallSearching
			block := receiver.webSearchPhaseToContentBlock(phase.ItemId, phase.OutputIndex, responses.ItemStatus_searching)
			sender.sendBlock(block, nil)

		case *responses.Event_ResponseWebSearchCallCompleted:
			phase := ev.ResponseWebSearchCallCompleted
			block := receiver.webSearchPhaseToContentBlock(phase.ItemId, phase.OutputIndex, responses.ItemStatus_completed)
			sender.sendBlock(block, nil)

		default:
			sw.Send(nil, fmt.Errorf("invalid event type: %T", ev))
		}
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

func (r *streamReceiver) errorEventToResponseMeta(ev *responses.ErrorEvent) *schema.AgenticResponseMeta {
	return &schema.AgenticResponseMeta{
		Extension: &ResponseMetaExtension{
			StreamingError: &StreamingResponseError{
				Code:    ev.GetCode(),
				Message: ev.GetMessage(),
				Param:   ev.GetParam(),
			},
		},
	}
}

func (r *streamReceiver) itemAddedEventToContentBlock(ev *responses.ItemEvent) (blocks []*schema.ContentBlock, err error) {
	switch item := ev.Item.Union.(type) {
	case *responses.OutputItem_FunctionToolCall:
		block, err := r.itemAddedEventFunctionToolCallToContentBlock(ev.OutputIndex, item)
		if err != nil {
			return nil, err
		}

		blocks = append(blocks, block)

	case *responses.OutputItem_Reasoning:
		block, err := r.itemAddedEventReasoningToContentBlock(ev.OutputIndex, item)
		if err != nil {
			return nil, err
		}

		blocks = append(blocks, block)

	case *responses.OutputItem_FunctionMcpCall:
		k := makeMCPToolCallItemAddedEventCacheKey(item.FunctionMcpCall.GetId(), ev.OutputIndex)
		r.ItemAddedEventCache[k] = item.FunctionMcpCall

	case *responses.OutputItem_FunctionMcpListTools:
		k := makeMCPListToolsItemAddedEventCacheKey(item.FunctionMcpListTools.GetId(), ev.OutputIndex)
		r.ItemAddedEventCache[k] = item.FunctionMcpListTools

	case *responses.OutputItem_OutputMessage,
		*responses.OutputItem_FunctionWebSearch,
		*responses.OutputItem_FunctionMcpApprovalRequest:

		// Do nothing.

	default:
		return nil, fmt.Errorf("invalid item type %T with 'output_item.added' event", item)
	}

	return blocks, nil
}

func (r *streamReceiver) itemAddedEventFunctionToolCallToContentBlock(outputIdx int64, item *responses.OutputItem_FunctionToolCall) (block *schema.ContentBlock, err error) {
	block, err = functionToolCallToContentBlock(item)
	if err != nil {
		return nil, err
	}

	block.StreamingMeta = &schema.StreamingMeta{
		Index: r.getBlockIndex(makeFunctionToolCallIndexKey(outputIdx)),
	}

	return block, nil
}

func (r *streamReceiver) itemAddedEventReasoningToContentBlock(outputIdx int64, item *responses.OutputItem_Reasoning) (block *schema.ContentBlock, err error) {
	block, err = reasoningToContentBlocks(item)
	if err != nil {
		return nil, err
	}

	block.StreamingMeta = &schema.StreamingMeta{
		Index: r.getBlockIndex(makeReasoningIndexKey(outputIdx)),
	}

	return block, nil
}

func (r *streamReceiver) itemDoneEventToContentBlocks(ev *responses.ItemDoneEvent) (blocks []*schema.ContentBlock, err error) {
	switch item := ev.Item.Union.(type) {
	case *responses.OutputItem_OutputMessage:
		blocks, err = r.itemDoneEventOutputMessageToContentBlock(item)
		if err != nil {
			return nil, err
		}

	case *responses.OutputItem_Reasoning:
		block, err := r.itemDoneEventReasoningToContentBlock(ev.OutputIndex, item)
		if err != nil {
			return nil, err
		}

		blocks = append(blocks, block)

	case *responses.OutputItem_FunctionToolCall:
		block, err := r.itemDoneEventFunctionToolCallToContentBlock(ev.OutputIndex, item)
		if err != nil {
			return nil, err
		}

		blocks = append(blocks, block)

	case *responses.OutputItem_FunctionWebSearch:
		block, err := r.itemDoneEventFunctionWebSearchToContentBlock(ev.OutputIndex, item)
		if err != nil {
			return nil, err
		}

		blocks = append(blocks, block)

	case *responses.OutputItem_FunctionMcpCall:
		blocks, err = r.itemDoneEventFunctionMCPCallToContentBlocks(ev.OutputIndex, item)
		if err != nil {
			return nil, err
		}

	case *responses.OutputItem_FunctionMcpListTools:
		block, err := r.itemDoneEventFunctionMCPListToolsToContentBlock(ev.OutputIndex, item)
		if err != nil {
			return nil, err
		}

		blocks = append(blocks, block)

	case *responses.OutputItem_FunctionMcpApprovalRequest:
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

func (r *streamReceiver) itemDoneEventOutputMessageToContentBlock(item *responses.OutputItem_OutputMessage) (blocks []*schema.ContentBlock, err error) {
	msg := item.OutputMessage
	if msg == nil {
		return nil, fmt.Errorf("received empty output message")
	}

	indices, ok := r.ProcessingAssistantGenTextBlockIndex[msg.Id]
	if !ok {
		return nil, fmt.Errorf("item %q not found in processing queue", msg.Id)
	}

	for idx := range indices {
		meta := &schema.StreamingMeta{Index: idx}
		block := schema.NewContentBlockChunk(&schema.AssistantGenText{}, meta)
		setItemID(block, msg.Id)
		setItemStatus(block, msg.Status.String())

		blocks = append(blocks, block)
	}

	return blocks, nil
}

func (r *streamReceiver) itemDoneEventReasoningToContentBlock(outputIdx int64, item *responses.OutputItem_Reasoning) (block *schema.ContentBlock, err error) {
	reasoning := item.Reasoning
	if reasoning == nil {
		return nil, fmt.Errorf("received empty reasoning")
	}

	meta := &schema.StreamingMeta{
		Index: r.getBlockIndex(makeReasoningIndexKey(outputIdx)),
	}
	block = schema.NewContentBlockChunk(&schema.Reasoning{}, meta)

	if reasoning.Id != nil {
		setItemID(block, *reasoning.Id)
	}
	setItemStatus(block, reasoning.Status.String())

	return block, nil
}

func (r *streamReceiver) itemDoneEventFunctionToolCallToContentBlock(outputIdx int64, item *responses.OutputItem_FunctionToolCall) (block *schema.ContentBlock, err error) {
	toolCall := item.FunctionToolCall
	if toolCall == nil {
		return nil, fmt.Errorf("received empty function tool call")
	}

	meta := &schema.StreamingMeta{
		Index: r.getBlockIndex(makeFunctionToolCallIndexKey(outputIdx)),
	}
	block = schema.NewContentBlockChunk(&schema.FunctionToolCall{
		CallID: toolCall.CallId,
		Name:   toolCall.Name,
	}, meta)

	if toolCall.Id != nil {
		setItemID(block, *toolCall.Id)
	}
	if toolCall.Status != nil {
		setItemStatus(block, toolCall.Status.String())
	}

	return block, nil
}

func (r *streamReceiver) itemDoneEventFunctionWebSearchToContentBlock(outputIdx int64, item *responses.OutputItem_FunctionWebSearch) (block *schema.ContentBlock, err error) {
	block, err = webSearchToContentBlock(item)
	if err != nil {
		return nil, err
	}

	block.StreamingMeta = &schema.StreamingMeta{
		Index: r.getBlockIndex(makeServerToolCallIndexKey(outputIdx)),
	}

	return block, nil
}

func (r *streamReceiver) itemDoneEventFunctionMCPCallToContentBlocks(outputIdx int64, item *responses.OutputItem_FunctionMcpCall) (blocks []*schema.ContentBlock, err error) {
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

func (r *streamReceiver) itemDoneEventFunctionMCPListToolsToContentBlock(outputIdx int64, item *responses.OutputItem_FunctionMcpListTools) (block *schema.ContentBlock, err error) {
	block, err = mcpListToolsToContentBlock(item)
	if err != nil {
		return nil, err
	}

	block.StreamingMeta = &schema.StreamingMeta{
		Index: r.getBlockIndex(makeMCPListToolsResultIndexKey(outputIdx)),
	}

	return block, nil
}

func (r *streamReceiver) itemDoneEventFunctionMCPApprovalRequestToContentBlock(outputIdx int64, item *responses.OutputItem_FunctionMcpApprovalRequest) (block *schema.ContentBlock, err error) {
	block, err = mcpApprovalRequestToContentBlock(item)
	if err != nil {
		return nil, err
	}

	block.StreamingMeta = &schema.StreamingMeta{
		Index: r.getBlockIndex(makeMCPToolApprovalRequestIndexKey(outputIdx)),
	}

	return block, nil
}

func (r *streamReceiver) contentPartAddedEventToContentBlock(ev *responses.ContentPartEvent) (block *schema.ContentBlock, err error) {
	key := makeAssistantGenTextIndexKey(ev.OutputIndex, ev.ContentIndex)
	blockIdx := r.getBlockIndex(key)

	indices, ok := r.ProcessingAssistantGenTextBlockIndex[ev.ItemId]
	if !ok {
		indices = map[int]bool{}
		r.ProcessingAssistantGenTextBlockIndex[ev.ItemId] = indices
	}

	indices[blockIdx] = true

	return r.eventContentPartToContentBlock(ev.ItemId, ev.Part, blockIdx, responses.ItemStatus_in_progress)
}

func (r *streamReceiver) contentPartDoneEventToContentBlock(ev *responses.ContentPartDoneEvent) (block *schema.ContentBlock, err error) {
	key := makeAssistantGenTextIndexKey(ev.OutputIndex, ev.ContentIndex)
	blockIdx := r.getBlockIndex(key)

	indices, ok := r.ProcessingAssistantGenTextBlockIndex[ev.ItemId]
	if !ok {
		return nil, fmt.Errorf("item %q has no processing assistant gen text block index", ev.ItemId)
	}

	delete(indices, blockIdx)

	return r.eventContentPartToContentBlock(ev.ItemId, ev.Part, blockIdx, responses.ItemStatus_completed)
}

func (r *streamReceiver) eventContentPartToContentBlock(itemID string, content *responses.OutputContentItem,
	blockIdx int, status responses.ItemStatus_Enum) (block *schema.ContentBlock, err error) {

	meta := &schema.StreamingMeta{Index: blockIdx}

	switch part := content.Union.(type) {
	case *responses.OutputContentItem_Text:
		block = schema.NewContentBlockChunk(&schema.AssistantGenText{}, meta)
	default:
		return nil, fmt.Errorf("invalid content part type: %T", part)
	}

	setItemStatus(block, status.String())
	setItemID(block, itemID)

	return block, nil
}

func (r *streamReceiver) outputTextDeltaEventToContentBlock(ev *responses.OutputTextEvent) *schema.ContentBlock {
	meta := &schema.StreamingMeta{
		Index: r.getBlockIndex(makeAssistantGenTextIndexKey(ev.OutputIndex, ev.ContentIndex)),
	}
	block := schema.NewContentBlockChunk(&schema.AssistantGenText{
		Text: ev.GetDelta(),
	}, meta)

	setItemID(block, ev.ItemId)

	return block
}

func (r *streamReceiver) annotationAddedEventToContentBlock(ev *responses.ResponseAnnotationAddedEvent) (block *schema.ContentBlock, err error) {
	annotation, err := outputTextAnnotationToTextAnnotation(ev.Annotation)
	if err != nil {
		return nil, fmt.Errorf("failed to convert annotation: %w", err)
	}

	annotation.Index = r.getTextAnnotationIndex(ev.OutputIndex, ev.ContentIndex, ev.AnnotationIndex)

	genText := &schema.AssistantGenText{
		Text: ev.GetDelta(),
		Extension: &AssistantGenTextExtension{
			Annotations: []*TextAnnotation{annotation},
		},
	}

	meta := &schema.StreamingMeta{
		Index: r.getBlockIndex(makeAssistantGenTextIndexKey(ev.OutputIndex, ev.ContentIndex)),
	}
	block = schema.NewContentBlockChunk(genText, meta)

	setItemID(block, ev.ItemId)

	return block, nil
}

func (r *streamReceiver) reasoningSummaryTextDeltaEventToContentBlock(ev *responses.ReasoningSummaryTextEvent) *schema.ContentBlock {
	text := ev.GetDelta()
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

	setItemID(block, ev.ItemId)

	return block
}

func (r *streamReceiver) functionCallArgumentsDeltaEventToContentBlock(ev *responses.FunctionCallArgumentsEvent) *schema.ContentBlock {
	meta := &schema.StreamingMeta{
		Index: r.getBlockIndex(makeFunctionToolCallIndexKey(ev.OutputIndex)),
	}
	block := schema.NewContentBlockChunk(&schema.FunctionToolCall{
		Arguments: ev.GetDelta(),
	}, meta)

	setItemID(block, ev.ItemId)

	return block
}

func (r *streamReceiver) mcpListToolsPhaseToContentBlock(itemID string, outputIdx int64, status responses.ItemStatus_Enum) *schema.ContentBlock {
	meta := &schema.StreamingMeta{
		Index: r.getBlockIndex(makeMCPListToolsResultIndexKey(outputIdx)),
	}

	content := &schema.MCPListToolsResult{}

	item, ok := r.ItemAddedEventCache[makeMCPListToolsItemAddedEventCacheKey(itemID, outputIdx)]
	if ok {
		item_, ok := item.(*responses.OutputItem_FunctionMcpListTools)
		if ok {
			content = &schema.MCPListToolsResult{
				ServerLabel: item_.FunctionMcpListTools.ServerLabel,
			}
		}
	}

	block := schema.NewContentBlockChunk(content, meta)

	setItemID(block, itemID)
	setItemStatus(block, status.String())

	return block
}

func (r *streamReceiver) mcpApprovalRequestEventToContentBlock(ev *responses.ResponseMcpApprovalRequestEvent) (block *schema.ContentBlock, err error) {
	apReq := ev.FunctionMcpApprovalRequest

	meta := &schema.StreamingMeta{
		Index: r.getBlockIndex(makeMCPToolApprovalRequestIndexKey(ev.OutputIndex)),
	}
	block = schema.NewContentBlockChunk(&schema.MCPToolApprovalRequest{
		ID:          apReq.GetId(),
		Name:        apReq.Name,
		Arguments:   apReq.Arguments,
		ServerLabel: apReq.ServerLabel,
	}, meta)

	setItemID(block, apReq.GetId())

	return block, nil
}

func (r *streamReceiver) mcpCallArgumentsDeltaEventToContentBlock(ev *responses.ResponseMcpCallArgumentsDeltaEvent) *schema.ContentBlock {
	meta := &schema.StreamingMeta{
		Index: r.getBlockIndex(makeMCPToolCallIndexKey(ev.OutputIndex)),
	}
	block := schema.NewContentBlockChunk(&schema.MCPToolCall{
		Arguments: ev.Delta,
	}, meta)

	setItemID(block, ev.ItemId)

	return block
}

func (r *streamReceiver) mcpCallPhaseToContentBlock(itemID string, outputIdx int64, status responses.ItemStatus_Enum) *schema.ContentBlock {
	meta := &schema.StreamingMeta{
		Index: r.getBlockIndex(makeMCPToolCallIndexKey(outputIdx)),
	}

	content := &schema.MCPToolCall{}

	item, ok := r.ItemAddedEventCache[makeMCPToolCallItemAddedEventCacheKey(itemID, outputIdx)]
	if ok {
		item_, ok := item.(*responses.OutputItem_FunctionMcpCall)
		if ok {
			content = &schema.MCPToolCall{
				ServerLabel:       item_.FunctionMcpCall.ServerLabel,
				ApprovalRequestID: item_.FunctionMcpCall.GetApprovalRequestId(),
				Name:              item_.FunctionMcpCall.Name,
			}
		}
	}

	block := schema.NewContentBlockChunk(content, meta)

	setItemID(block, itemID)
	setItemStatus(block, status.String())
	return block
}

func (r *streamReceiver) webSearchPhaseToContentBlock(itemID string, outputIdx int64, status responses.ItemStatus_Enum) *schema.ContentBlock {
	meta := &schema.StreamingMeta{
		Index: r.getBlockIndex(makeServerToolCallIndexKey(outputIdx)),
	}
	block := schema.NewContentBlockChunk(&schema.ServerToolCall{
		Name: string(ServerToolNameWebSearch),
	}, meta)

	setItemID(block, itemID)
	setItemStatus(block, status.String())

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

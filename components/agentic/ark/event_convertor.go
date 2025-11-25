/*
 * Copyright 2025 CloudWeGo Authors
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

package ark

import (
	"errors"
	"fmt"
	"io"

	"github.com/cloudwego/eino/components/agentic"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model/responses"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/utils"
)

func receivedStreamResponse(streamReader *utils.ResponsesStreamReader,
	config *agentic.Config, sw *schema.StreamWriter[*agentic.CallbackOutput]) {

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
			block := receiver.mcpListToolsPhaseToContentBlock(phase.ItemId, phase.OutputIndex, responses.ItemStatus_searching)
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
	sw        *schema.StreamWriter[*agentic.CallbackOutput]
	config    *agentic.Config
	errHeader string
}

func newCallbackSender(sw *schema.StreamWriter[*agentic.CallbackOutput], config *agentic.Config) *callbackSender {
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

	s.sw.Send(&agentic.CallbackOutput{
		Message: msg,
		Config:  s.config,
	}, nil)
}

type streamReceiver struct {
	ProcessingAssistantGenTextBlockIndex map[string]map[int64]bool

	MaxBlockIndex int64
	IndexMapper   map[string]int64

	MaxReasoningSummaryIndex    map[string]int64
	ReasoningSummaryIndexMapper map[string]int64

	MaxTextAnnotationIndex    map[string]int64
	TextAnnotationIndexMapper map[string]int64
}

func newStreamReceiver() *streamReceiver {
	return &streamReceiver{
		ProcessingAssistantGenTextBlockIndex: map[string]map[int64]bool{},
		MaxBlockIndex:                        int64(-1),
		IndexMapper:                          map[string]int64{},
		MaxReasoningSummaryIndex:             map[string]int64{},
		ReasoningSummaryIndexMapper:          map[string]int64{},
		TextAnnotationIndexMapper:            map[string]int64{},
		MaxTextAnnotationIndex:               map[string]int64{},
	}
}

func (r *streamReceiver) getBlockIndex(key string) int64 {
	if idx, ok := r.IndexMapper[key]; ok {
		return idx
	}

	r.MaxBlockIndex++
	r.IndexMapper[key] = r.MaxBlockIndex

	return r.MaxBlockIndex
}

func (r *streamReceiver) getReasoningSummaryIndex(outputIdx, summaryIdx int64) int64 {
	maxSummaryIndex := int64(-1)
	if idx, ok := r.MaxReasoningSummaryIndex[int64ToStr(outputIdx)]; ok {
		maxSummaryIndex = idx
	}

	key := fmt.Sprintf("%d:%d", outputIdx, summaryIdx)
	if idx, ok := r.ReasoningSummaryIndexMapper[key]; ok {
		return idx
	}

	maxSummaryIndex++
	r.ReasoningSummaryIndexMapper[key] = maxSummaryIndex

	return maxSummaryIndex
}

func (r *streamReceiver) getTextAnnotationIndex(outputIdx, contentIdx, annotationIdx int64) int64 {
	maxAnnotationIndex := int64(-1)

	key := fmt.Sprintf("%d:%d", outputIdx, contentIdx)
	if idx, ok := r.MaxTextAnnotationIndex[key]; ok {
		maxAnnotationIndex = idx
	}

	key = fmt.Sprintf("%d:%d:%d", outputIdx, contentIdx, annotationIdx)
	if idx, ok := r.TextAnnotationIndexMapper[key]; ok {
		return idx
	}

	maxAnnotationIndex++
	r.TextAnnotationIndexMapper[key] = maxAnnotationIndex

	return maxAnnotationIndex
}

func (r *streamReceiver) errorEventToResponseMeta(ev *responses.ErrorEvent) *schema.AgenticResponseMeta {
	return &schema.AgenticResponseMeta{
		Extension: &ResponseMetaExtension{
			StreamError: &StreamResponseError{
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

	case *responses.OutputItem_OutputMessage,
		*responses.OutputItem_FunctionWebSearch,
		*responses.OutputItem_FunctionMcpListTools,
		*responses.OutputItem_FunctionMcpApprovalRequest,
		*responses.OutputItem_FunctionMcpCall:

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

	block.StreamMeta = &schema.StreamMeta{
		Index: r.getBlockIndex(makeFunctionToolCallIndexKey(outputIdx)),
	}

	return block, nil
}

func (r *streamReceiver) itemAddedEventReasoningToContentBlock(outputIdx int64, item *responses.OutputItem_Reasoning) (block *schema.ContentBlock, err error) {
	block, err = reasoningToContentBlocks(item)
	if err != nil {
		return nil, err
	}

	block.StreamMeta = &schema.StreamMeta{
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
		block := schema.NewContentBlock(&schema.AssistantGenText{})
		block.StreamMeta = &schema.StreamMeta{
			Index: idx,
		}

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

	block = schema.NewContentBlock(&schema.Reasoning{})
	block.StreamMeta = &schema.StreamMeta{
		Index: r.getBlockIndex(makeReasoningIndexKey(outputIdx)),
	}

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

	block = schema.NewContentBlock(&schema.FunctionToolCall{
		CallID: toolCall.CallId,
		Name:   toolCall.Name,
	})
	block.StreamMeta = &schema.StreamMeta{
		Index: r.getBlockIndex(makeFunctionToolCallIndexKey(outputIdx)),
	}

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

	block.StreamMeta = &schema.StreamMeta{
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
		block.StreamMeta = &schema.StreamMeta{
			Index: r.getBlockIndex(makeMCPToolCallIndexKey(outputIdx)),
		}
	}

	return blocks, nil
}

func (r *streamReceiver) itemDoneEventFunctionMCPListToolsToContentBlock(outputIdx int64, item *responses.OutputItem_FunctionMcpListTools) (block *schema.ContentBlock, err error) {
	block, err = mcpListToolsToContentBlock(item)
	if err != nil {
		return nil, err
	}

	block.StreamMeta = &schema.StreamMeta{
		Index: r.getBlockIndex(makeMCPListToolsResultIndexKey(outputIdx)),
	}

	return block, nil
}

func (r *streamReceiver) itemDoneEventFunctionMCPApprovalRequestToContentBlock(outputIdx int64, item *responses.OutputItem_FunctionMcpApprovalRequest) (block *schema.ContentBlock, err error) {
	block, err = mcpApprovalRequestToContentBlock(item)
	if err != nil {
		return nil, err
	}

	block.StreamMeta = &schema.StreamMeta{
		Index: r.getBlockIndex(makeMCPToolApprovalRequestIndexKey(outputIdx)),
	}

	return block, nil
}

func (r *streamReceiver) contentPartAddedEventToContentBlock(ev *responses.ContentPartEvent) (block *schema.ContentBlock, err error) {
	key := makeAssistantGenTextIndexKey(ev.OutputIndex, ev.ContentIndex)
	blockIdx := r.getBlockIndex(key)

	indices, ok := r.ProcessingAssistantGenTextBlockIndex[ev.ItemId]
	if !ok {
		indices = map[int64]bool{}
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
		return nil, fmt.Errorf("item %s has no processing assistant gen text block index", ev.ItemId)
	}

	delete(indices, blockIdx)

	return r.eventContentPartToContentBlock(ev.ItemId, ev.Part, blockIdx, responses.ItemStatus_completed)
}

func (r *streamReceiver) eventContentPartToContentBlock(itemID string, content *responses.OutputContentItem,
	blockIdx int64, status responses.ItemStatus_Enum) (block *schema.ContentBlock, err error) {

	switch part := content.Union.(type) {
	case *responses.OutputContentItem_Text:
		block, err = outputContentTextToContentBlock(part.Text)
		if err != nil {
			return nil, fmt.Errorf("failed to convert output text to content block: %w", err)
		}

	default:
		return nil, fmt.Errorf("invalid content part type: %T", part)
	}

	block.StreamMeta = &schema.StreamMeta{
		Index: blockIdx,
	}

	setItemStatus(block, status.String())
	setItemID(block, itemID)

	return block, nil
}

func (r *streamReceiver) outputTextDeltaEventToContentBlock(ev *responses.OutputTextEvent) *schema.ContentBlock {
	block := schema.NewContentBlock(&schema.AssistantGenText{
		Text: ev.GetDelta(),
	})
	block.StreamMeta = &schema.StreamMeta{
		Index: r.getBlockIndex(makeAssistantGenTextIndexKey(ev.OutputIndex, ev.ContentIndex)),
	}

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

	block = schema.NewContentBlock(genText)

	block.StreamMeta = &schema.StreamMeta{
		Index: r.getBlockIndex(makeAssistantGenTextIndexKey(ev.OutputIndex, ev.ContentIndex)),
	}

	setItemID(block, ev.ItemId)

	return block, nil
}

func (r *streamReceiver) reasoningSummaryTextDeltaEventToContentBlock(ev *responses.ReasoningSummaryTextEvent) *schema.ContentBlock {
	reasoning := &schema.Reasoning{
		Summary: []*schema.ReasoningSummary{
			{
				Index: r.getReasoningSummaryIndex(ev.OutputIndex, ev.SummaryIndex),
				Text:  ev.GetDelta(),
			},
		},
	}

	block := schema.NewContentBlock(reasoning)
	block.StreamMeta = &schema.StreamMeta{
		Index: r.getBlockIndex(makeReasoningIndexKey(ev.OutputIndex)),
	}

	setItemID(block, ev.ItemId)

	return block
}

func (r *streamReceiver) functionCallArgumentsDeltaEventToContentBlock(ev *responses.FunctionCallArgumentsEvent) *schema.ContentBlock {
	block := schema.NewContentBlock(&schema.FunctionToolCall{
		Arguments: ev.GetDelta(),
	})
	block.StreamMeta = &schema.StreamMeta{
		Index: r.getBlockIndex(makeFunctionToolCallIndexKey(ev.OutputIndex)),
	}

	setItemID(block, ev.ItemId)

	return block
}

func (r *streamReceiver) mcpListToolsPhaseToContentBlock(itemID string, outputIdx int64, status responses.ItemStatus_Enum) *schema.ContentBlock {
	block := schema.NewContentBlock(&schema.ServerToolCall{})
	block.StreamMeta = &schema.StreamMeta{
		Index: r.getBlockIndex(makeMCPListToolsResultIndexKey(outputIdx)),
	}

	setItemID(block, itemID)
	setItemStatus(block, status.String())

	return block
}

func (r *streamReceiver) mcpApprovalRequestEventToContentBlock(ev *responses.ResponseMcpApprovalRequestEvent) (block *schema.ContentBlock, err error) {
	apReq := ev.FunctionMcpApprovalRequest

	block = schema.NewContentBlock(&schema.MCPToolApprovalRequest{
		ID:          apReq.GetId(),
		Name:        apReq.Name,
		Arguments:   apReq.Arguments,
		ServerLabel: apReq.ServerLabel,
	})
	block.StreamMeta = &schema.StreamMeta{
		Index: r.getBlockIndex(makeMCPToolApprovalRequestIndexKey(ev.OutputIndex)),
	}

	setItemID(block, apReq.GetId())

	return block, nil
}

func (r *streamReceiver) mcpCallArgumentsDeltaEventToContentBlock(ev *responses.ResponseMcpCallArgumentsDeltaEvent) *schema.ContentBlock {
	block := schema.NewContentBlock(&schema.MCPToolCall{
		Arguments: ev.Delta,
	})
	block.StreamMeta = &schema.StreamMeta{
		Index: r.getBlockIndex(makeMCPToolCallIndexKey(ev.OutputIndex)),
	}

	setItemID(block, ev.ItemId)

	return block
}

func (r *streamReceiver) mcpCallPhaseToContentBlock(itemID string, outputIdx int64, status responses.ItemStatus_Enum) *schema.ContentBlock {
	block := schema.NewContentBlock(&schema.MCPToolCall{})
	block.StreamMeta = &schema.StreamMeta{
		Index: r.getBlockIndex(makeMCPToolResultIndexKey(outputIdx)),
	}

	setItemID(block, itemID)
	setItemStatus(block, status.String())
	return block
}

func (r *streamReceiver) webSearchPhaseToContentBlock(itemID string, outputIdx int64, status responses.ItemStatus_Enum) *schema.ContentBlock {
	block := schema.NewContentBlock(&schema.ServerToolCall{})
	block.StreamMeta = &schema.StreamMeta{
		Index: r.getBlockIndex(makeServerToolCallIndexKey(outputIdx)),
	}

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

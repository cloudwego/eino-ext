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
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model/responses"
)

func TestNewStreamReceiverInit(t *testing.T) {
	r := newStreamReceiver()
	assert.NotNil(t, r.ProcessingAssistantGenTextBlockIndex)
	assert.Equal(t, -1, r.MaxBlockIndex)
	assert.NotNil(t, r.IndexMapper)
	assert.NotNil(t, r.MaxReasoningSummaryIndex)
	assert.NotNil(t, r.ReasoningSummaryIndexMapper)
	assert.NotNil(t, r.TextAnnotationIndexMapper)
	assert.NotNil(t, r.MaxTextAnnotationIndex)
}

func TestGetBlockIndexAndReuse(t *testing.T) {
	r := newStreamReceiver()
	a := r.getBlockIndex("k1")
	b := r.getBlockIndex("k2")
	c := r.getBlockIndex("k1")
	assert.Equal(t, a, c)
	assert.NotEqual(t, a, b)
	assert.GreaterOrEqual(t, r.MaxBlockIndex, 1)
}

func TestGetReasoningSummaryIndex(t *testing.T) {
	r := newStreamReceiver()
	i1 := r.isNewReasoningSummaryIndex(1, 1)
	i2 := r.isNewReasoningSummaryIndex(1, 2)
	i3 := r.isNewReasoningSummaryIndex(2, 1)
	i4 := r.isNewReasoningSummaryIndex(1, 1)
	assert.True(t, i1)
	assert.True(t, i2)
	assert.True(t, i3)
	assert.False(t, i4)
}

func TestGetTextAnnotationIndex(t *testing.T) {
	r := newStreamReceiver()
	i1 := r.getTextAnnotationIndex(1, 1, 1)
	i2 := r.getTextAnnotationIndex(1, 1, 2)
	i3 := r.getTextAnnotationIndex(1, 2, 1)
	i4 := r.getTextAnnotationIndex(2, 1, 1)
	i5 := r.getTextAnnotationIndex(1, 1, 1)
	assert.Equal(t, 0, i1)
	assert.Equal(t, 1, i2)
	assert.Equal(t, 0, i3)
	assert.Equal(t, 0, i4)
	assert.Equal(t, i1, i5)
}

func TestItemAddedEventToContentBlockFunctionToolCall(t *testing.T) {
	r := newStreamReceiver()
	ev := &responses.ItemEvent{
		OutputIndex: 1,
		Item: &responses.OutputItem{
			Union: &responses.OutputItem_FunctionToolCall{
				FunctionToolCall: &responses.ItemFunctionToolCall{
					CallId: "cid",
					Name:   "name",
				},
			},
		},
	}
	blocks, err := r.itemAddedEventToContentBlock(ev)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(blocks))
	assert.NotNil(t, blocks[0].FunctionToolCall)
	assert.GreaterOrEqual(t, blocks[0].StreamingMeta.Index, 0)
}

func TestItemAddedEventToContentBlockReasoning(t *testing.T) {
	r := newStreamReceiver()
	ev := &responses.ItemEvent{
		OutputIndex: 2,
		Item: &responses.OutputItem{
			Union: &responses.OutputItem_Reasoning{
				Reasoning: &responses.ItemReasoning{
					Status: responses.ItemStatus_completed,
					Summary: []*responses.ReasoningSummaryPart{
						{Text: "x"},
					},
				},
			},
		},
	}
	blocks, err := r.itemAddedEventToContentBlock(ev)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(blocks))
	assert.NotNil(t, blocks[0].Reasoning)
}

func TestItemAddedEventToContentBlockInvalid(t *testing.T) {
	r := newStreamReceiver()
	ev := &responses.ItemEvent{
		Item: &responses.OutputItem{Union: nil},
	}
	_, err := r.itemAddedEventToContentBlock(ev)
	assert.Error(t, err)
}

func TestItemDoneEventToContentBlocksReasoning(t *testing.T) {
	r := newStreamReceiver()
	ev := &responses.ItemDoneEvent{
		OutputIndex: 3,
		Item: &responses.OutputItem{
			Union: &responses.OutputItem_Reasoning{
				Reasoning: &responses.ItemReasoning{
					Status: responses.ItemStatus_completed,
				},
			},
		},
	}
	blocks, err := r.itemDoneEventToContentBlocks(ev)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(blocks))
	assert.NotNil(t, blocks[0].Reasoning)
}

func TestItemDoneEventToContentBlocksFunctionToolCall(t *testing.T) {
	r := newStreamReceiver()
	ev := &responses.ItemDoneEvent{
		OutputIndex: 4,
		Item: &responses.OutputItem{
			Union: &responses.OutputItem_FunctionToolCall{
				FunctionToolCall: &responses.ItemFunctionToolCall{
					CallId: "cid",
					Name:   "nm",
				},
			},
		},
	}
	blocks, err := r.itemDoneEventToContentBlocks(ev)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(blocks))
	assert.NotNil(t, blocks[0].FunctionToolCall)
}

func TestItemDoneEventToContentBlocksInvalid(t *testing.T) {
	r := newStreamReceiver()
	ev := &responses.ItemDoneEvent{
		Item: &responses.OutputItem{Union: nil},
	}
	_, err := r.itemDoneEventToContentBlocks(ev)
	assert.Error(t, err)
}

func TestItemDoneEventOutputMessageToContentBlockMissingProcessing(t *testing.T) {
	r := newStreamReceiver()
	ev := &responses.OutputItem_OutputMessage{
		OutputMessage: &responses.ItemOutputMessage{
			Id:     "mid",
			Status: responses.ItemStatus_completed,
		},
	}
	_, err := r.itemDoneEventOutputMessageToContentBlock(ev)
	assert.Error(t, err)
}

func TestItemDoneEventOutputMessageToContentBlockOK(t *testing.T) {
	r := newStreamReceiver()
	r.ProcessingAssistantGenTextBlockIndex["mid"] = map[int]bool{0: true, 2: true}
	ev := &responses.OutputItem_OutputMessage{
		OutputMessage: &responses.ItemOutputMessage{
			Id:     "mid",
			Status: responses.ItemStatus_completed,
		},
	}
	blocks, err := r.itemDoneEventOutputMessageToContentBlock(ev)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(blocks))
	id, ok := getItemID(blocks[0])
	assert.True(t, ok)
	assert.Equal(t, "mid", id)
	status, ok := GetItemStatus(blocks[0])
	assert.True(t, ok)
	assert.Equal(t, responses.ItemStatus_completed.String(), status)
}

func TestItemDoneEventReasoningToContentBlockNilError(t *testing.T) {
	r := newStreamReceiver()
	_, err := r.itemDoneEventReasoningToContentBlock(1, &responses.OutputItem_Reasoning{})
	assert.Error(t, err)
}

func TestItemDoneEventFunctionToolCallToContentBlockNilError(t *testing.T) {
	r := newStreamReceiver()
	_, err := r.itemDoneEventFunctionToolCallToContentBlock(1, &responses.OutputItem_FunctionToolCall{})
	assert.Error(t, err)
}

func TestItemDoneEventFunctionWebSearchToContentBlock(t *testing.T) {
	r := newStreamReceiver()
	block, err := r.itemDoneEventFunctionWebSearchToContentBlock(1, &responses.OutputItem_FunctionWebSearch{
		FunctionWebSearch: &responses.ItemFunctionWebSearch{
			Id:     "id",
			Status: responses.ItemStatus_completed,
			Action: &responses.Action{
				Type:  responses.ActionType_search,
				Query: "q",
			},
		},
	})
	assert.NoError(t, err)
	assert.NotNil(t, block.ServerToolCall)
}

func TestItemDoneEventFunctionMCPCallToContentBlocksTypeCheck(t *testing.T) {
	r := newStreamReceiver()
	blocks, err := r.itemDoneEventFunctionMCPCallToContentBlocks(1, &responses.OutputItem_FunctionMcpCall{
		FunctionMcpCall: &responses.ItemFunctionMcpCall{
			Id:          ptrOf("id"),
			ServerLabel: "server",
			Name:        "tool",
			Arguments:   "{}",
			Output:      ptrOf("out"),
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(blocks))
	assert.NotNil(t, blocks[0].StreamingMeta)
	assert.GreaterOrEqual(t, blocks[0].StreamingMeta.Index, 0)
	assert.NotNil(t, blocks[1].StreamingMeta)
	assert.GreaterOrEqual(t, blocks[1].StreamingMeta.Index, 0)
}

func TestItemDoneEventFunctionMCPListToolsToContentBlock(t *testing.T) {
	r := newStreamReceiver()
	block, err := r.itemDoneEventFunctionMCPListToolsToContentBlock(1, &responses.OutputItem_FunctionMcpListTools{
		FunctionMcpListTools: &responses.ItemFunctionMcpListTools{
			ServerLabel: "server",
		},
	})
	assert.NoError(t, err)
	assert.NotNil(t, block.MCPListToolsResult)
}

func TestItemDoneEventFunctionMCPApprovalRequestToContentBlock(t *testing.T) {
	r := newStreamReceiver()
	block, err := r.itemDoneEventFunctionMCPApprovalRequestToContentBlock(1, &responses.OutputItem_FunctionMcpApprovalRequest{
		FunctionMcpApprovalRequest: &responses.ItemFunctionMcpApprovalRequest{
			Id:          ptrOf("id"),
			ServerLabel: "server",
			Name:        "tool",
			Arguments:   "{}",
		},
	})
	assert.NoError(t, err)
	assert.NotNil(t, block.MCPToolApprovalRequest)
}

func TestContentPartAddedEventToContentBlock(t *testing.T) {
	r := newStreamReceiver()
	ev := &responses.ContentPartEvent{
		ItemId:       "mid",
		OutputIndex:  1,
		ContentIndex: 2,
		Part: &responses.OutputContentItem{
			Union: &responses.OutputContentItem_Text{Text: &responses.OutputContentItemText{}},
		},
	}
	block, err := r.contentPartAddedEventToContentBlock(ev)
	assert.NoError(t, err)
	assert.NotNil(t, block.AssistantGenText)
	id, ok := getItemID(block)
	assert.True(t, ok)
	assert.Equal(t, "mid", id)
}

func TestContentPartDoneEventToContentBlockNoIndex(t *testing.T) {
	r := newStreamReceiver()
	ev := &responses.ContentPartDoneEvent{
		ItemId:       "mid",
		OutputIndex:  1,
		ContentIndex: 2,
		Part: &responses.OutputContentItem{
			Union: &responses.OutputContentItem_Text{Text: &responses.OutputContentItemText{}},
		},
	}
	_, err := r.contentPartDoneEventToContentBlock(ev)
	assert.Error(t, err)
}

func TestContentPartDoneEventToContentBlockOK(t *testing.T) {
	r := newStreamReceiver()
	evAdd := &responses.ContentPartEvent{
		ItemId:       "mid",
		OutputIndex:  1,
		ContentIndex: 1,
		Part: &responses.OutputContentItem{
			Union: &responses.OutputContentItem_Text{Text: &responses.OutputContentItemText{}},
		},
	}
	_, _ = r.contentPartAddedEventToContentBlock(evAdd)
	evDone := &responses.ContentPartDoneEvent{
		ItemId:       "mid",
		OutputIndex:  1,
		ContentIndex: 1,
		Part: &responses.OutputContentItem{
			Union: &responses.OutputContentItem_Text{Text: &responses.OutputContentItemText{}},
		},
	}
	block, err := r.contentPartDoneEventToContentBlock(evDone)
	assert.NoError(t, err)
	assert.NotNil(t, block.AssistantGenText)
	status, ok := GetItemStatus(block)
	assert.True(t, ok)
	assert.Equal(t, responses.ItemStatus_completed.String(), status)
}

func TestEventContentPartToContentBlockInvalid(t *testing.T) {
	r := newStreamReceiver()
	_, err := r.eventContentPartToContentBlock("id", &responses.OutputContentItem{}, 1, responses.ItemStatus_in_progress)
	assert.Error(t, err)
}

func TestOutputTextDeltaEventToContentBlock(t *testing.T) {
	r := newStreamReceiver()
	block := r.outputTextDeltaEventToContentBlock(&responses.OutputTextEvent{
		Delta:        ptrOf("d"),
		ItemId:       "iid",
		OutputIndex:  1,
		ContentIndex: 1,
	})
	assert.NotNil(t, block.AssistantGenText)
	assert.Equal(t, "d", block.AssistantGenText.Text)
}

func TestAnnotationAddedEventToContentBlock(t *testing.T) {
	r := newStreamReceiver()
	title := "t"
	url := "u"
	block, err := r.annotationAddedEventToContentBlock(&responses.ResponseAnnotationAddedEvent{
		ItemId:          "iid",
		OutputIndex:     1,
		ContentIndex:    1,
		AnnotationIndex: 0,
		Annotation: &responses.Annotation{
			Type:  responses.AnnotationType_url_citation,
			Title: title,
			Url:   url,
		},
	})
	assert.NoError(t, err)
	assert.NotNil(t, block.AssistantGenText)
	assert.NotNil(t, block.AssistantGenText.Extension)
	id, ok := getItemID(block)
	assert.True(t, ok)
	assert.Equal(t, "iid", id)
}

func TestReasoningSummaryTextDeltaEventToContentBlock(t *testing.T) {
	r := newStreamReceiver()
	block := r.reasoningSummaryTextDeltaEventToContentBlock(&responses.ReasoningSummaryTextEvent{
		ItemId:       "iid",
		OutputIndex:  2,
		SummaryIndex: 0,
		Delta:        ptrOf("x"),
	})
	assert.NotNil(t, block.Reasoning)
	assert.Equal(t, "x", block.Reasoning.Text)
}

func TestFunctionCallArgumentsDeltaEventToContentBlock(t *testing.T) {
	r := newStreamReceiver()
	block := r.functionCallArgumentsDeltaEventToContentBlock(&responses.FunctionCallArgumentsEvent{
		ItemId:      "iid",
		OutputIndex: 3,
		Delta:       ptrOf("{}"),
	})
	assert.NotNil(t, block.FunctionToolCall)
	assert.Equal(t, "{}", block.FunctionToolCall.Arguments)
}

func TestMcpListToolsPhaseToContentBlock(t *testing.T) {
	r := newStreamReceiver()
	block := r.mcpListToolsPhaseToContentBlock("iid", 4, responses.ItemStatus_in_progress)
	assert.NotNil(t, block.MCPListToolsResult)
	id, ok := getItemID(block)
	assert.True(t, ok)
	assert.Equal(t, "iid", id)
	status, ok := GetItemStatus(block)
	assert.True(t, ok)
	assert.Equal(t, responses.ItemStatus_in_progress.String(), status)
}

func TestMcpCallArgumentsDeltaEventToContentBlock(t *testing.T) {
	r := newStreamReceiver()
	block := r.mcpCallArgumentsDeltaEventToContentBlock(&responses.ResponseMcpCallArgumentsDeltaEvent{
		ItemId:      "iid",
		OutputIndex: 6,
		Delta:       "{}",
	})
	assert.NotNil(t, block.MCPToolCall)
	id, ok := getItemID(block)
	assert.True(t, ok)
	assert.Equal(t, "iid", id)
}

func TestMcpCallPhaseToContentBlock(t *testing.T) {
	r := newStreamReceiver()
	block := r.mcpCallPhaseToContentBlock("iid", 7, responses.ItemStatus_failed)
	assert.NotNil(t, block.MCPToolCall)
	status, ok := GetItemStatus(block)
	assert.True(t, ok)
	assert.Equal(t, responses.ItemStatus_failed.String(), status)
}

func TestWebSearchPhaseToContentBlock(t *testing.T) {
	r := newStreamReceiver()
	block := r.webSearchPhaseToContentBlock("iid", 8, responses.ItemStatus_completed)
	assert.NotNil(t, block.ServerToolCall)
	status, ok := GetItemStatus(block)
	assert.True(t, ok)
	assert.Equal(t, responses.ItemStatus_completed.String(), status)
	assert.Equal(t, string(ServerToolNameWebSearch), block.ServerToolCall.Name)
}

func TestMakeIndexKeyFunctions(t *testing.T) {
	assert.Equal(t, "assistant_gen_text:1:2", makeAssistantGenTextIndexKey(1, 2))
	assert.Equal(t, "reasoning:3", makeReasoningIndexKey(3))
	assert.Equal(t, "function_tool_call:4", makeFunctionToolCallIndexKey(4))
	assert.Equal(t, "server_tool_call:5", makeServerToolCallIndexKey(5))
	assert.Equal(t, "mcp_list_tools_result:6", makeMCPListToolsResultIndexKey(6))
	assert.Equal(t, "mcp_tool_approval_request:7", makeMCPToolApprovalRequestIndexKey(7))
	assert.Equal(t, "mcp_tool_call:8", makeMCPToolCallIndexKey(8))
	assert.Equal(t, "mcp_tool_result:9", makeMCPToolResultIndexKey(9))
}

func TestNewCallbackSenderAndSend(t *testing.T) {
	sr, sw := schema.Pipe[*model.AgenticCallbackOutput](8)
	s := newCallbackSender(sw, &model.AgenticConfig{})
	r0 := sr.Copy(1)[0]

	// Send a meta message first
	s.sendMeta(&schema.AgenticResponseMeta{}, nil)
	ch, err := r0.Recv()
	assert.NoError(t, err)
	assert.NotNil(t, ch)
	assert.NotNil(t, ch.Message.ResponseMeta)

	// Send a block
	block := schema.NewContentBlock(&schema.AssistantGenText{Text: "x"})
	s.sendBlock(block, nil)
	ch, err = r0.Recv()
	assert.NoError(t, err)
	assert.NotNil(t, ch.Message.ContentBlocks)

	// Send an error
	s.errHeader = "h"
	s.sendMeta(nil, errors.New("e"))
	_, err = r0.Recv()
	assert.Error(t, err)
}

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
	"errors"
	"testing"

	"github.com/bytedance/mockey"
	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/openai/openai-go/v3/responses"
	"github.com/stretchr/testify/assert"
)

func TestNewStreamReceiverInit(t *testing.T) {
	r := newStreamReceiver(&model.Options{})
	assert.NotNil(t, r.ProcessingAssistantGenTextBlockIndex)
	assert.Equal(t, -1, r.MaxBlockIndex)
	assert.NotNil(t, r.IndexMapper)
	assert.NotNil(t, r.MaxReasoningSummaryIndex)
	assert.NotNil(t, r.ReasoningSummaryIndexMapper)
	assert.NotNil(t, r.TextAnnotationIndexMapper)
	assert.NotNil(t, r.MaxTextAnnotationIndex)
}

func TestGetBlockIndexAndReuse(t *testing.T) {
	r := newStreamReceiver(&model.Options{})
	a := r.getBlockIndex("k1")
	b := r.getBlockIndex("k2")
	c := r.getBlockIndex("k1")
	assert.Equal(t, a, c)
	assert.NotEqual(t, a, b)
	assert.GreaterOrEqual(t, r.MaxBlockIndex, 1)
}

func TestGetReasoningSummaryIndex(t *testing.T) {
	r := newStreamReceiver(&model.Options{})
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
	r := newStreamReceiver(&model.Options{})
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
	mockey.PatchConvey("TestItemAddedEventToContentBlockFunctionToolCall", t, func() {
		r := newStreamReceiver(&model.Options{})
		ev := responses.ResponseOutputItemAddedEvent{
			OutputIndex: 1,
		}

		mockey.Mock(responses.ResponseOutputItemUnion.AsAny).Return(responses.ResponseFunctionToolCall{
			ID:     "id1",
			CallID: "cid",
			Name:   "name",
		}).Build()

		blocks, err := r.itemAddedEventToContentBlock(ev)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(blocks))
		assert.NotNil(t, blocks[0].FunctionToolCall)
		assert.NotNil(t, blocks[0].StreamingMeta)
	})
}

func TestItemAddedEventToContentBlockIgnoredTypes(t *testing.T) {
	mockey.PatchConvey("TestItemAddedEventToContentBlockIgnoredTypes", t, func() {
		r := newStreamReceiver(&model.Options{})

		// Mock AsAny to return different types in sequence
		mockey.Mock(responses.ResponseOutputItemUnion.AsAny).Return(
			mockey.Sequence(
				responses.ResponseOutputMessage{},
			).Then(
				responses.ResponseFunctionWebSearch{},
			).Then(
				responses.ResponseOutputItemMcpListTools{},
			).Then(
				responses.ResponseOutputItemMcpApprovalRequest{},
			).Then(
				responses.ResponseOutputItemMcpCall{},
			),
		).Build()

		ignoredTypes := []string{"OutputMessage", "WebSearch", "McpListTools", "McpApprovalRequest", "McpCall"}

		for range ignoredTypes {
			ev := responses.ResponseOutputItemAddedEvent{
				OutputIndex: 1,
			}
			blocks, err := r.itemAddedEventToContentBlock(ev)
			assert.NoError(t, err)
			assert.Equal(t, 0, len(blocks))
		}
	})
}

func TestItemDoneEventToContentBlocksOutputMessage(t *testing.T) {
	mockey.PatchConvey("TestItemDoneEventToContentBlocksOutputMessage", t, func() {
		r := newStreamReceiver(&model.Options{})
		r.ProcessingAssistantGenTextBlockIndex["mid"] = map[int]bool{0: true, 2: true}

		ev := responses.ResponseOutputItemDoneEvent{
			OutputIndex: 1,
		}

		mockey.Mock(responses.ResponseOutputItemUnion.AsAny).Return(responses.ResponseOutputMessage{
			ID:     "mid",
			Status: responses.ResponseOutputMessageStatusCompleted,
		}).Build()

		blocks, err := r.itemDoneEventToContentBlocks(ev)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(blocks))
		id, ok := getItemID(blocks[0])
		assert.True(t, ok)
		assert.Equal(t, "mid", id)
	})
}

func TestItemDoneEventToContentBlocksReasoning(t *testing.T) {
	mockey.PatchConvey("TestItemDoneEventToContentBlocksReasoning", t, func() {
		r := newStreamReceiver(&model.Options{})
		ev := responses.ResponseOutputItemDoneEvent{
			OutputIndex: 3,
		}

		mockey.Mock(responses.ResponseOutputItemUnion.AsAny).Return(responses.ResponseReasoningItem{
			ID:     "rid",
			Status: responses.ResponseReasoningItemStatusCompleted,
		}).Build()

		blocks, err := r.itemDoneEventToContentBlocks(ev)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(blocks))
		assert.NotNil(t, blocks[0].Reasoning)
	})
}

func TestItemDoneEventToContentBlocksFunctionToolCall(t *testing.T) {
	mockey.PatchConvey("TestItemDoneEventToContentBlocksFunctionToolCall", t, func() {
		r := newStreamReceiver(&model.Options{})
		ev := responses.ResponseOutputItemDoneEvent{
			OutputIndex: 4,
		}

		mockey.Mock(responses.ResponseOutputItemUnion.AsAny).Return(responses.ResponseFunctionToolCall{
			ID:     "fid",
			CallID: "cid",
			Name:   "nm",
		}).Build()

		blocks, err := r.itemDoneEventToContentBlocks(ev)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(blocks))
		assert.NotNil(t, blocks[0].FunctionToolCall)
	})
}

func TestItemDoneEventToContentBlocksWebSearch(t *testing.T) {
	mockey.PatchConvey("TestItemDoneEventToContentBlocksWebSearch", t, func() {
		r := newStreamReceiver(&model.Options{})
		ev := responses.ResponseOutputItemDoneEvent{
			OutputIndex: 5,
		}

		action := responses.ResponseFunctionWebSearchActionUnion{}
		mockey.Mock(responses.ResponseFunctionWebSearchActionUnion.AsAny).Return(responses.ResponseFunctionWebSearchActionSearch{
			Query: "test",
		}).Build()

		mockey.Mock(responses.ResponseOutputItemUnion.AsAny).Return(responses.ResponseFunctionWebSearch{
			ID:     "wid",
			Status: responses.ResponseFunctionWebSearchStatusCompleted,
			Action: action,
		}).Build()

		blocks, err := r.itemDoneEventToContentBlocks(ev)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(blocks))
		assert.NotNil(t, blocks[0].ServerToolCall)
		assert.NotNil(t, blocks[1].ServerToolResult)
	})
}

func TestItemDoneEventToContentBlocksMCPCall(t *testing.T) {
	mockey.PatchConvey("TestItemDoneEventToContentBlocksMCPCall", t, func() {
		r := newStreamReceiver(&model.Options{})
		ev := responses.ResponseOutputItemDoneEvent{
			OutputIndex: 6,
		}

		mockey.Mock(responses.ResponseOutputItemUnion.AsAny).Return(responses.ResponseOutputItemMcpCall{
			ID:          "mid",
			ServerLabel: "server",
			Name:        "tool",
			Arguments:   "{}",
			Output:      "result",
		}).Build()

		blocks, err := r.itemDoneEventToContentBlocks(ev)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(blocks))
		assert.NotNil(t, blocks[0].MCPToolCall)
		assert.NotNil(t, blocks[1].MCPToolResult)
	})
}

func TestItemDoneEventToContentBlocksMCPListTools(t *testing.T) {
	mockey.PatchConvey("TestItemDoneEventToContentBlocksMCPListTools", t, func() {
		r := newStreamReceiver(&model.Options{})
		ev := responses.ResponseOutputItemDoneEvent{
			OutputIndex: 7,
		}

		mockey.Mock(responses.ResponseOutputItemUnion.AsAny).Return(responses.ResponseOutputItemMcpListTools{
			ID:          "lid",
			ServerLabel: "server",
		}).Build()

		blocks, err := r.itemDoneEventToContentBlocks(ev)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(blocks))
		assert.NotNil(t, blocks[0].MCPListToolsResult)
	})
}

func TestItemDoneEventToContentBlocksMCPApprovalRequest(t *testing.T) {
	mockey.PatchConvey("TestItemDoneEventToContentBlocksMCPApprovalRequest", t, func() {
		r := newStreamReceiver(&model.Options{})
		ev := responses.ResponseOutputItemDoneEvent{
			OutputIndex: 8,
		}

		mockey.Mock(responses.ResponseOutputItemUnion.AsAny).Return(responses.ResponseOutputItemMcpApprovalRequest{
			ID:          "aid",
			ServerLabel: "server",
			Name:        "tool",
			Arguments:   "{}",
		}).Build()

		blocks, err := r.itemDoneEventToContentBlocks(ev)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(blocks))
		assert.NotNil(t, blocks[0].MCPToolApprovalRequest)
	})
}

func TestItemDoneEventOutputMessageToContentBlockMissingProcessing(t *testing.T) {
	r := newStreamReceiver(&model.Options{})
	item := responses.ResponseOutputMessage{
		ID:     "mid",
		Status: responses.ResponseOutputMessageStatusCompleted,
	}
	_, err := r.itemDoneEventOutputMessageToContentBlock(item)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found in processing queue")
}

func TestItemDoneEventOutputMessageToContentBlockSuccess(t *testing.T) {
	r := newStreamReceiver(&model.Options{})
	r.ProcessingAssistantGenTextBlockIndex["mid"] = map[int]bool{0: true, 2: true}

	item := responses.ResponseOutputMessage{
		ID:     "mid",
		Status: responses.ResponseOutputMessageStatusCompleted,
	}
	blocks, err := r.itemDoneEventOutputMessageToContentBlock(item)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(blocks))

	for _, block := range blocks {
		id, ok := getItemID(block)
		assert.True(t, ok)
		assert.Equal(t, "mid", id)
		status, ok := GetItemStatus(block)
		assert.True(t, ok)
		assert.Equal(t, string(responses.ResponseOutputMessageStatusCompleted), status)
	}
}

func TestContentPartAddedEventToContentBlockInvalidType(t *testing.T) {
	mockey.PatchConvey("TestContentPartAddedEventToContentBlockInvalidType", t, func() {
		r := newStreamReceiver(&model.Options{})
		ev := responses.ResponseContentPartAddedEvent{}

		mockey.Mock(responses.ResponseOutputMessageContentUnion.AsAny).Return("invalid").Build()

		_, err := r.contentPartAddedEventToContentBlock(ev)
		assert.Error(t, err)
	})
}

func TestContentPartDoneEventToContentBlockNoIndex(t *testing.T) {
	mockey.PatchConvey("TestContentPartDoneEventToContentBlockNoIndex", t, func() {
		r := newStreamReceiver(&model.Options{})
		ev := responses.ResponseContentPartDoneEvent{
			ItemID:       "mid",
			OutputIndex:  1,
			ContentIndex: 2,
		}

		mockey.Mock(responses.ResponseOutputMessageContentUnion.AsAny).Return(responses.ResponseOutputText{}).Build()

		_, err := r.contentPartDoneEventToContentBlock(ev)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "has no processing assistant gen text block index")
	})
}

func TestRefusalDeltaEventToContentBlock(t *testing.T) {
	r := newStreamReceiver(&model.Options{})
	ev := responses.ResponseRefusalDeltaEvent{
		ItemID:       "iid",
		OutputIndex:  1,
		ContentIndex: 1,
		Delta:        "refused",
	}

	block := r.refusalDeltaEventToContentBlock(ev)
	assert.NotNil(t, block.AssistantGenText)
	assert.NotNil(t, block.AssistantGenText.OpenAIExtension)
	assert.NotNil(t, block.AssistantGenText.OpenAIExtension.Refusal)
	assert.Equal(t, "refused", block.AssistantGenText.OpenAIExtension.Refusal.Reason)

	id, ok := getItemID(block)
	assert.True(t, ok)
	assert.Equal(t, "iid", id)
}

func TestOutputTextDeltaEventToContentBlock(t *testing.T) {
	r := newStreamReceiver(&model.Options{})
	ev := responses.ResponseTextDeltaEvent{
		ItemID:       "iid",
		OutputIndex:  1,
		ContentIndex: 1,
		Delta:        "delta text",
	}

	block := r.outputTextDeltaEventToContentBlock(ev)
	assert.NotNil(t, block.AssistantGenText)
	assert.Equal(t, "delta text", block.AssistantGenText.Text)
	assert.NotNil(t, block.StreamingMeta)

	id, ok := getItemID(block)
	assert.True(t, ok)
	assert.Equal(t, "iid", id)
}

func TestAnnotationAddedEventToContentBlockFileCitation(t *testing.T) {
	mockey.PatchConvey("TestAnnotationAddedEventToContentBlockFileCitation", t, func() {
		r := newStreamReceiver(&model.Options{})
		ev := responses.ResponseOutputTextAnnotationAddedEvent{
			ItemID:          "iid",
			OutputIndex:     1,
			ContentIndex:    1,
			AnnotationIndex: 0,
		}

		annoData := responses.ResponseOutputTextAnnotationFileCitation{
			Index:    10,
			FileID:   "fid",
			Filename: "file.txt",
		}

		mockey.Mock(sonic.Marshal).To(func(val any) ([]byte, error) {
			return []byte(`{"type":"file_citation","index":10,"file_id":"fid","filename":"file.txt"}`), nil
		}).Build()

		mockey.Mock(sonic.Unmarshal).To(func(data []byte, v any) error {
			return nil
		}).Build()

		mockey.Mock(responses.ResponseOutputTextAnnotationUnion.AsAny).Return(annoData).Build()

		block, err := r.annotationAddedEventToContentBlock(ev)
		assert.NoError(t, err)
		assert.NotNil(t, block.AssistantGenText)
		assert.NotNil(t, block.AssistantGenText.OpenAIExtension)
		assert.Len(t, block.AssistantGenText.OpenAIExtension.Annotations, 1)
	})
}

func TestReasoningSummaryTextDeltaEventToContentBlock(t *testing.T) {
	r := newStreamReceiver(&model.Options{})
	ev := responses.ResponseReasoningSummaryTextDeltaEvent{
		ItemID:       "iid",
		OutputIndex:  2,
		SummaryIndex: 0,
		Delta:        "summary text",
	}

	block := r.reasoningSummaryTextDeltaEventToContentBlock(ev)
	assert.NotNil(t, block.Reasoning)
	assert.Equal(t, "summary text", block.Reasoning.Text)

	id, ok := getItemID(block)
	assert.True(t, ok)
	assert.Equal(t, "iid", id)
}

func TestFunctionCallArgumentsDeltaEventToContentBlock(t *testing.T) {
	r := newStreamReceiver(&model.Options{})
	ev := responses.ResponseFunctionCallArgumentsDeltaEvent{
		ItemID:      "iid",
		OutputIndex: 3,
		Delta:       `{"arg":"val"}`,
	}

	block := r.functionCallArgumentsDeltaEventToContentBlock(ev)
	assert.NotNil(t, block.FunctionToolCall)
	assert.Equal(t, `{"arg":"val"}`, block.FunctionToolCall.Arguments)

	id, ok := getItemID(block)
	assert.True(t, ok)
	assert.Equal(t, "iid", id)
}

func TestMcpListToolsPhaseToContentBlock(t *testing.T) {
	r := newStreamReceiver(&model.Options{})

	block := r.mcpListToolsPhaseToContentBlock("iid", 4, string(responses.ResponseStatusInProgress))
	assert.NotNil(t, block.MCPListToolsResult)
	assert.NotNil(t, block.StreamingMeta)

	id, ok := getItemID(block)
	assert.True(t, ok)
	assert.Equal(t, "iid", id)

	status, ok := GetItemStatus(block)
	assert.True(t, ok)
	assert.Equal(t, string(responses.ResponseStatusInProgress), status)
}

func TestMcpListToolsPhaseToContentBlockEmptyStatus(t *testing.T) {
	r := newStreamReceiver(&model.Options{})

	block := r.mcpListToolsPhaseToContentBlock("iid", 4, "")
	assert.NotNil(t, block.MCPListToolsResult)

	_, ok := GetItemStatus(block)
	assert.False(t, ok)
}

func TestMcpCallArgumentsDeltaEventToContentBlock(t *testing.T) {
	r := newStreamReceiver(&model.Options{})
	ev := responses.ResponseMcpCallArgumentsDeltaEvent{
		ItemID:      "iid",
		OutputIndex: 6,
		Delta:       `{"key":"value"}`,
	}

	block := r.mcpCallArgumentsDeltaEventToContentBlock(ev)
	assert.NotNil(t, block.MCPToolCall)
	assert.Equal(t, `{"key":"value"}`, block.MCPToolCall.Arguments)

	id, ok := getItemID(block)
	assert.True(t, ok)
	assert.Equal(t, "iid", id)
}

func TestMcpCallPhaseToContentBlock(t *testing.T) {
	r := newStreamReceiver(&model.Options{})

	block := r.mcpCallPhaseToContentBlock("iid", 7, string(responses.ResponseStatusFailed))
	assert.NotNil(t, block.MCPToolCall)

	status, ok := GetItemStatus(block)
	assert.True(t, ok)
	assert.Equal(t, string(responses.ResponseStatusFailed), status)
}

func TestWebSearchPhaseToContentBlock(t *testing.T) {
	r := newStreamReceiver(&model.Options{})

	block := r.webSearchPhaseToContentBlock("iid", 8, string(responses.ResponseStatusCompleted))
	assert.NotNil(t, block.ServerToolCall)
	assert.Equal(t, string(ServerToolNameWebSearch), block.ServerToolCall.Name)

	status, ok := GetItemStatus(block)
	assert.True(t, ok)
	assert.Equal(t, string(responses.ResponseStatusCompleted), status)
}

func TestFileSearchPhaseToContentBlock(t *testing.T) {
	r := newStreamReceiver(&model.Options{})

	block := r.fileSearchPhaseToContentBlock("fsid", 9, string(responses.ResponseStatusInProgress))
	assert.NotNil(t, block.ServerToolCall)
	assert.Equal(t, string(ServerToolNameFileSearch), block.ServerToolCall.Name)

	id, ok := getItemID(block)
	assert.True(t, ok)
	assert.Equal(t, "fsid", id)

	status, ok := GetItemStatus(block)
	assert.True(t, ok)
	assert.Equal(t, string(responses.ResponseStatusInProgress), status)
}

func TestFileSearchPhaseToContentBlockEmptyStatus(t *testing.T) {
	r := newStreamReceiver(&model.Options{})

	block := r.fileSearchPhaseToContentBlock("fsid", 10, "")
	assert.NotNil(t, block.ServerToolCall)
	assert.Equal(t, string(ServerToolNameFileSearch), block.ServerToolCall.Name)

	_, ok := GetItemStatus(block)
	assert.False(t, ok)
}

func TestItemDoneEventToContentBlocksFileSearch(t *testing.T) {
	mockey.PatchConvey("TestItemDoneEventToContentBlocksFileSearch", t, func() {
		r := newStreamReceiver(&model.Options{})
		ev := responses.ResponseOutputItemDoneEvent{
			OutputIndex: 11,
		}

		mockey.Mock(responses.ResponseOutputItemUnion.AsAny).Return(responses.ResponseFileSearchToolCall{
			ID:      "fsid",
			Status:  responses.ResponseFileSearchToolCallStatusCompleted,
			Queries: []string{"test query"},
			Results: []responses.ResponseFileSearchToolCallResult{
				{
					FileID:   "file1",
					Filename: "doc.txt",
					Score:    0.85,
					Text:     "matched content",
				},
			},
		}).Build()

		blocks, err := r.itemDoneEventToContentBlocks(ev)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(blocks))
		assert.NotNil(t, blocks[0].ServerToolCall)
		assert.NotNil(t, blocks[1].ServerToolResult)
		assert.Equal(t, string(ServerToolNameFileSearch), blocks[0].ServerToolCall.Name)
		assert.Equal(t, string(ServerToolNameFileSearch), blocks[1].ServerToolResult.Name)
	})
}

func TestItemAddedEventToContentBlockIgnoredFileSearch(t *testing.T) {
	mockey.PatchConvey("TestItemAddedEventToContentBlockIgnoredFileSearch", t, func() {
		r := newStreamReceiver(&model.Options{})

		mockey.Mock(responses.ResponseOutputItemUnion.AsAny).Return(
			responses.ResponseFileSearchToolCall{},
		).Build()

		ev := responses.ResponseOutputItemAddedEvent{
			OutputIndex: 1,
		}
		blocks, err := r.itemAddedEventToContentBlock(ev)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(blocks))
	})
}

func TestMakeIndexKeyFunctions(t *testing.T) {
	assert.Equal(t, "assistant_gen_text:1:2", makeAssistantGenTextIndexKey(1, 2))
	assert.Equal(t, "reasoning:3", makeReasoningIndexKey(3))
	assert.Equal(t, "function_tool_call:4", makeFunctionToolCallIndexKey(4))
	assert.Equal(t, "server_tool_call:5", makeServerToolCallIndexKey(5))
	assert.Equal(t, "server_tool_result:6", makeServerToolResultIndexKey(6))
	assert.Equal(t, "mcp_list_tools_result:7", makeMCPListToolsResultIndexKey(7))
	assert.Equal(t, "mcp_tool_approval_request:8", makeMCPToolApprovalRequestIndexKey(8))
	assert.Equal(t, "mcp_tool_call:9", makeMCPToolCallIndexKey(9))
	assert.Equal(t, "mcp_tool_result:10", makeMCPToolResultIndexKey(10))
}

func TestNewCallbackSender(t *testing.T) {
	_, sw := schema.Pipe[*model.AgenticCallbackOutput](8)
	config := &model.AgenticConfig{}

	s := newCallbackSender(sw, config)
	assert.NotNil(t, s)
	assert.Equal(t, sw, s.sw)
	assert.Equal(t, config, s.config)
}

func TestCallbackSenderSendMeta(t *testing.T) {
	sr, sw := schema.Pipe[*model.AgenticCallbackOutput](8)
	r := sr.Copy(1)[0]
	s := newCallbackSender(sw, &model.AgenticConfig{})

	meta := &schema.AgenticResponseMeta{}
	s.sendMeta(meta, nil)

	out, err := r.Recv()
	assert.NoError(t, err)
	assert.NotNil(t, out)
	assert.NotNil(t, out.Message.ResponseMeta)
}

func TestCallbackSenderSendBlock(t *testing.T) {
	sr, sw := schema.Pipe[*model.AgenticCallbackOutput](8)
	r := sr.Copy(1)[0]
	s := newCallbackSender(sw, &model.AgenticConfig{})

	block := schema.NewContentBlock(&schema.AssistantGenText{Text: "test"})
	s.sendBlock(block, nil)

	out, err := r.Recv()
	assert.NoError(t, err)
	assert.NotNil(t, out)
	assert.Len(t, out.Message.ContentBlocks, 1)
}

func TestCallbackSenderSendError(t *testing.T) {
	sr, sw := schema.Pipe[*model.AgenticCallbackOutput](8)
	r := sr.Copy(1)[0]
	s := newCallbackSender(sw, &model.AgenticConfig{})
	s.errHeader = "test error"

	s.sendMeta(nil, errors.New("error"))

	_, err := r.Recv()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "test error")
}

func TestItemDoneEventToContentBlocksToolSearchCall(t *testing.T) {
	mockey.PatchConvey("TestItemDoneEventToContentBlocksToolSearchCall", t, func() {
		r := newStreamReceiver(&model.Options{})
		ev := responses.ResponseOutputItemDoneEvent{
			OutputIndex: 9,
		}

		mockey.Mock(responses.ResponseOutputItemUnion.AsAny).Return(responses.ResponseToolSearchCall{
			ID:        "tsid",
			CallID:    "call1",
			Status:    "completed",
			Execution: responses.ResponseToolSearchCallExecutionServer,
			Arguments: map[string]any{"query": "find tools"},
		}).Build()

		blocks, err := r.itemDoneEventToContentBlocks(ev)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(blocks))
		assert.NotNil(t, blocks[0].ServerToolCall)
	})
}

func TestItemDoneEventToContentBlocksToolSearchOutputItem(t *testing.T) {
	mockey.PatchConvey("TestItemDoneEventToContentBlocksToolSearchOutputItem", t, func() {
		r := newStreamReceiver(&model.Options{})
		ev := responses.ResponseOutputItemDoneEvent{
			OutputIndex: 10,
		}

		mockey.Mock(responses.ResponseOutputItemUnion.AsAny).Return(responses.ResponseToolSearchOutputItem{
			ID:     "tsoid",
			CallID: "call1",
			Status: "completed",
			Tools: []responses.ToolUnion{
				{Type: "function", Name: "tool1", Description: "desc1", Parameters: map[string]any{"type": "object"}},
			},
		}).Build()

		blocks, err := r.itemDoneEventToContentBlocks(ev)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(blocks))
		assert.NotNil(t, blocks[0].ServerToolResult)
	})
}

func TestItemDoneEventToolSearchCallToContentBlocksStreamingMeta(t *testing.T) {
	mockey.PatchConvey("TestItemDoneEventToolSearchCallToContentBlocksStreamingMeta", t, func() {
		r := newStreamReceiver(&model.Options{})

		item := responses.ResponseToolSearchCall{
			ID:        "tsid",
			CallID:    "call1",
			Status:    "completed",
			Execution: responses.ResponseToolSearchCallExecutionServer,
			Arguments: map[string]any{"query": "find tools"},
		}

		block, err := r.itemDoneEventToolSearchCallToContentBlocks(0, item, &model.Options{})
		assert.NoError(t, err)
		assert.NotNil(t, block)
		assert.NotNil(t, block.StreamingMeta)
	})
}

func TestItemDoneEventToolSearchOutputItemToContentBlocksStreamingMeta(t *testing.T) {
	mockey.PatchConvey("TestItemDoneEventToolSearchOutputItemToContentBlocksStreamingMeta", t, func() {
		r := newStreamReceiver(&model.Options{})

		item := responses.ResponseToolSearchOutputItem{
			ID:     "tsoid",
			CallID: "call1",
			Status: "completed",
			Tools: []responses.ToolUnion{
				{Type: "function", Name: "tool1", Description: "desc1", Parameters: map[string]any{"type": "object"}},
			},
		}

		block, err := r.itemDoneEventToolSearchOutputItemToContentBlocks(0, item)
		assert.NoError(t, err)
		assert.NotNil(t, block)
		assert.NotNil(t, block.StreamingMeta)
	})
}

func TestMcpListToolsPhaseToContentBlockWithCache(t *testing.T) {
	r := newStreamReceiver(&model.Options{})

	k := makeMCPListToolsItemAddedEventCacheKey("iid", 4)
	r.ItemAddedEventCache[k] = responses.ResponseOutputItemMcpListTools{
		ID:          "iid",
		ServerLabel: "srv",
	}

	block := r.mcpListToolsPhaseToContentBlock("iid", 4, string(responses.ResponseStatusInProgress))
	assert.NotNil(t, block.MCPListToolsResult)
	assert.Equal(t, "srv", block.MCPListToolsResult.ServerLabel)
}

func TestMcpCallPhaseToContentBlockWithCache(t *testing.T) {
	r := newStreamReceiver(&model.Options{})

	k := makeMCPToolCallItemAddedEventCacheKey("iid", 7)
	r.ItemAddedEventCache[k] = responses.ResponseOutputItemMcpCall{
		ID:                "iid",
		ServerLabel:       "srv",
		Name:              "tool_name",
		ApprovalRequestID: "ar1",
	}

	block := r.mcpCallPhaseToContentBlock("iid", 7, string(responses.ResponseStatusInProgress))
	assert.NotNil(t, block.MCPToolCall)
	assert.Equal(t, "srv", block.MCPToolCall.ServerLabel)
	assert.Equal(t, "tool_name", block.MCPToolCall.Name)
	assert.Equal(t, "ar1", block.MCPToolCall.ApprovalRequestID)
}

func TestWebSearchPhaseToContentBlockEmptyStatus(t *testing.T) {
	r := newStreamReceiver(&model.Options{})

	block := r.webSearchPhaseToContentBlock("iid", 8, "")
	assert.NotNil(t, block.ServerToolCall)
	assert.Equal(t, string(ServerToolNameWebSearch), block.ServerToolCall.Name)

	_, ok := GetItemStatus(block)
	assert.False(t, ok)
}

func TestMcpCallPhaseToContentBlockEmptyStatus(t *testing.T) {
	r := newStreamReceiver(&model.Options{})

	block := r.mcpCallPhaseToContentBlock("iid", 7, "")
	assert.NotNil(t, block.MCPToolCall)

	_, ok := GetItemStatus(block)
	assert.False(t, ok)
}

func TestReasoningSummaryTextDeltaEventNewlinePrepend(t *testing.T) {
	r := newStreamReceiver(&model.Options{})

	// First delta at summaryIndex 0 — no newline
	ev1 := responses.ResponseReasoningSummaryTextDeltaEvent{
		ItemID:       "iid",
		OutputIndex:  2,
		SummaryIndex: 0,
		Delta:        "first",
	}
	block1 := r.reasoningSummaryTextDeltaEventToContentBlock(ev1)
	assert.Equal(t, "first", block1.Reasoning.Text)

	// Second delta at summaryIndex 1 — should prepend newline
	ev2 := responses.ResponseReasoningSummaryTextDeltaEvent{
		ItemID:       "iid",
		OutputIndex:  2,
		SummaryIndex: 1,
		Delta:        "second",
	}
	block2 := r.reasoningSummaryTextDeltaEventToContentBlock(ev2)
	assert.Equal(t, "\nsecond", block2.Reasoning.Text)

	// Third delta at summaryIndex 1 again — no newline (not new)
	ev3 := responses.ResponseReasoningSummaryTextDeltaEvent{
		ItemID:       "iid",
		OutputIndex:  2,
		SummaryIndex: 1,
		Delta:        "more",
	}
	block3 := r.reasoningSummaryTextDeltaEventToContentBlock(ev3)
	assert.Equal(t, "more", block3.Reasoning.Text)
}

func TestItemAddedEventToContentBlockReasoning(t *testing.T) {
	mockey.PatchConvey("TestItemAddedEventToContentBlockReasoning", t, func() {
		r := newStreamReceiver(&model.Options{})
		ev := responses.ResponseOutputItemAddedEvent{
			OutputIndex: 1,
		}

		mockey.Mock(responses.ResponseOutputItemUnion.AsAny).Return(responses.ResponseReasoningItem{
			ID:     "rid",
			Status: "in_progress",
		}).Build()

		blocks, err := r.itemAddedEventToContentBlock(ev)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(blocks))
		assert.NotNil(t, blocks[0].Reasoning)
		assert.NotNil(t, blocks[0].StreamingMeta)
	})
}

func TestItemAddedEventToContentBlockMcpCallCaching(t *testing.T) {
	mockey.PatchConvey("TestItemAddedEventToContentBlockMcpCallCaching", t, func() {
		r := newStreamReceiver(&model.Options{})
		ev := responses.ResponseOutputItemAddedEvent{
			OutputIndex: 2,
		}

		mcpCall := responses.ResponseOutputItemMcpCall{
			ID:          "mcpid",
			ServerLabel: "srv",
			Name:        "tool",
		}
		mockey.Mock(responses.ResponseOutputItemUnion.AsAny).Return(mcpCall).Build()

		blocks, err := r.itemAddedEventToContentBlock(ev)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(blocks))

		k := makeMCPToolCallItemAddedEventCacheKey("mcpid", 2)
		cached, ok := r.ItemAddedEventCache[k]
		assert.True(t, ok)
		cachedItem, ok := cached.(responses.ResponseOutputItemMcpCall)
		assert.True(t, ok)
		assert.Equal(t, "srv", cachedItem.ServerLabel)
	})
}

func TestItemAddedEventToContentBlockMcpListToolsCaching(t *testing.T) {
	mockey.PatchConvey("TestItemAddedEventToContentBlockMcpListToolsCaching", t, func() {
		r := newStreamReceiver(&model.Options{})
		ev := responses.ResponseOutputItemAddedEvent{
			OutputIndex: 3,
		}

		mcpList := responses.ResponseOutputItemMcpListTools{
			ID:          "listid",
			ServerLabel: "srv",
		}
		mockey.Mock(responses.ResponseOutputItemUnion.AsAny).Return(mcpList).Build()

		blocks, err := r.itemAddedEventToContentBlock(ev)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(blocks))

		k := makeMCPListToolsItemAddedEventCacheKey("listid", 3)
		cached, ok := r.ItemAddedEventCache[k]
		assert.True(t, ok)
		cachedItem, ok := cached.(responses.ResponseOutputItemMcpListTools)
		assert.True(t, ok)
		assert.Equal(t, "srv", cachedItem.ServerLabel)
	})
}

func TestCodeInterpreterPhaseToContentBlock(t *testing.T) {
	r := newStreamReceiver(&model.Options{})

	block := r.codeInterpreterPhaseToContentBlock("ciid", 9, string(responses.ResponseStatusInProgress))
	assert.NotNil(t, block.ServerToolResult)
	assert.Equal(t, string(ServerToolNameCodeInterpreter), block.ServerToolResult.Name)

	id, ok := getItemID(block)
	assert.True(t, ok)
	assert.Equal(t, "ciid", id)

	status, ok := GetItemStatus(block)
	assert.True(t, ok)
	assert.Equal(t, string(responses.ResponseStatusInProgress), status)
}

func TestCodeInterpreterPhaseToContentBlockEmptyStatus(t *testing.T) {
	r := newStreamReceiver(&model.Options{})

	block := r.codeInterpreterPhaseToContentBlock("ciid", 10, "")
	assert.NotNil(t, block.ServerToolResult)
	assert.Equal(t, string(ServerToolNameCodeInterpreter), block.ServerToolResult.Name)

	_, ok := GetItemStatus(block)
	assert.False(t, ok)
}

func TestCodeInterpreterCodeDeltaEventToContentBlock(t *testing.T) {
	r := newStreamReceiver(&model.Options{})
	ev := responses.ResponseCodeInterpreterCallCodeDeltaEvent{
		ItemID:      "ciid",
		OutputIndex: 12,
		Delta:       "print('hello",
	}

	block := r.codeInterpreterCodeDeltaEventToContentBlock(ev)
	assert.NotNil(t, block.ServerToolResult)
	assert.Equal(t, string(ServerToolNameCodeInterpreter), block.ServerToolResult.Name)

	result, ok := block.ServerToolResult.Result.(*ServerToolResult)
	assert.True(t, ok)
	assert.NotNil(t, result.CodeInterpreter)
	assert.Equal(t, "print('hello", result.CodeInterpreter.Code)

	id, ok := getItemID(block)
	assert.True(t, ok)
	assert.Equal(t, "ciid", id)
}

func TestItemDoneEventCodeInterpreterToContentBlocks(t *testing.T) {
	mockey.PatchConvey("TestItemDoneEventCodeInterpreterToContentBlocks", t, func() {
		r := newStreamReceiver(&model.Options{})
		ev := responses.ResponseOutputItemDoneEvent{
			OutputIndex: 13,
		}

		mockey.Mock(responses.ResponseOutputItemUnion.AsAny).Return(responses.ResponseCodeInterpreterToolCall{
			ID:          "ciid",
			Status:      responses.ResponseCodeInterpreterToolCallStatusCompleted,
			Code:        "x = 1 + 1\nprint(x)",
			ContainerID: "container123",
			Outputs:     []responses.ResponseCodeInterpreterToolCallOutputUnion{},
		}).Build()

		blocks, err := r.itemDoneEventToContentBlocks(ev)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(blocks))
		assert.NotNil(t, blocks[0].ServerToolCall)
		assert.NotNil(t, blocks[1].ServerToolResult)
		assert.Equal(t, string(ServerToolNameCodeInterpreter), blocks[0].ServerToolCall.Name)
		assert.Equal(t, string(ServerToolNameCodeInterpreter), blocks[1].ServerToolResult.Name)
	})
}

func TestItemAddedEventToContentBlockCodeInterpreter(t *testing.T) {
	mockey.PatchConvey("TestItemAddedEventToContentBlockCodeInterpreter", t, func() {
		r := newStreamReceiver(&model.Options{})

		mockey.Mock(responses.ResponseOutputItemUnion.AsAny).Return(
			responses.ResponseCodeInterpreterToolCall{
				ID:          "ci1",
				Code:        "print('hello')",
				ContainerID: "container123",
				Status:      "in_progress",
				Outputs: []responses.ResponseCodeInterpreterToolCallOutputUnion{
					{
						Type: "logs",
						Logs: "hello",
					},
				},
			},
		).Build()

		ev := responses.ResponseOutputItemAddedEvent{
			OutputIndex: 1,
		}
		blocks, err := r.itemAddedEventToContentBlock(ev)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(blocks))
		assert.NotNil(t, blocks[0].ServerToolCall)
		assert.NotNil(t, blocks[1].ServerToolResult)
	})
}

func TestImageGenerationPhaseToContentBlock(t *testing.T) {
	r := newStreamReceiver(&model.Options{})

	block := r.imageGenerationPhaseToContentBlock("igid", 14, string(responses.ResponseStatusInProgress))
	assert.NotNil(t, block.ServerToolResult)
	assert.Equal(t, string(ServerToolNameImageGeneration), block.ServerToolResult.Name)

	id, ok := getItemID(block)
	assert.True(t, ok)
	assert.Equal(t, "igid", id)

	status, ok := GetItemStatus(block)
	assert.True(t, ok)
	assert.Equal(t, string(responses.ResponseStatusInProgress), status)
}

func TestImageGenerationPartialImageEventToContentBlock(t *testing.T) {
	r := newStreamReceiver(&model.Options{})
	ev := responses.ResponseImageGenCallPartialImageEvent{
		ItemID:          "igid",
		OutputIndex:     15,
		PartialImageB64: "partial_base64_data",
	}

	block := r.imageGenerationPartialImageEventToContentBlock(ev)
	assert.NotNil(t, block.ServerToolResult)
	assert.Equal(t, string(ServerToolNameImageGeneration), block.ServerToolResult.Name)

	result, ok := block.ServerToolResult.Result.(*ServerToolResult)
	assert.True(t, ok)
	assert.NotNil(t, result.ImageGeneration)
	assert.Equal(t, "partial_base64_data", result.ImageGeneration.ImageBase64)

	id, ok := getItemID(block)
	assert.True(t, ok)
	assert.Equal(t, "igid", id)
}

func TestItemDoneEventImageGenerationToContentBlocks(t *testing.T) {
	mockey.PatchConvey("TestItemDoneEventImageGenerationToContentBlocks", t, func() {
		r := newStreamReceiver(&model.Options{})
		ev := responses.ResponseOutputItemDoneEvent{
			OutputIndex: 16,
		}

		mockey.Mock(responses.ResponseOutputItemUnion.AsAny).Return(responses.ResponseOutputItemImageGenerationCall{
			ID:     "igid",
			Status: "completed",
			Result: "full_base64_image",
		}).Build()

		blocks, err := r.itemDoneEventToContentBlocks(ev)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(blocks))
		assert.NotNil(t, blocks[0].ServerToolCall)
		assert.NotNil(t, blocks[1].ServerToolResult)
		assert.Equal(t, string(ServerToolNameImageGeneration), blocks[0].ServerToolCall.Name)
		assert.Equal(t, string(ServerToolNameImageGeneration), blocks[1].ServerToolResult.Name)
	})
}

func TestItemAddedEventToContentBlockIgnoredImageGeneration(t *testing.T) {
	mockey.PatchConvey("TestItemAddedEventToContentBlockIgnoredImageGeneration", t, func() {
		r := newStreamReceiver(&model.Options{})

		mockey.Mock(responses.ResponseOutputItemUnion.AsAny).Return(
			responses.ResponseOutputItemImageGenerationCall{},
		).Build()

		ev := responses.ResponseOutputItemAddedEvent{
			OutputIndex: 1,
		}
		blocks, err := r.itemAddedEventToContentBlock(ev)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(blocks))
	})
}

func TestItemDoneEventShellCallToContentBlock(t *testing.T) {
	mockey.PatchConvey("TestItemDoneEventShellCallToContentBlock", t, func() {
		r := newStreamReceiver(&model.Options{})
		ev := responses.ResponseOutputItemDoneEvent{
			OutputIndex: 20,
		}

		mockey.Mock(responses.ResponseOutputItemUnion.AsAny).Return(responses.ResponseFunctionShellToolCall{
			ID:     "shell1",
			Status: "completed",
			Action: responses.ResponseFunctionShellToolCallAction{
				Commands: []string{"ls"},
			},
			Environment: responses.ResponseFunctionShellToolCallEnvironmentUnion{
				Type: "local",
			},
		}).Build()

		blocks, err := r.itemDoneEventToContentBlocks(ev)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(blocks))
		assert.NotNil(t, blocks[0].ServerToolCall)
		assert.Equal(t, string(ServerToolNameShell), blocks[0].ServerToolCall.Name)
	})
}

func TestItemDoneEventShellOutputToContentBlock(t *testing.T) {
	mockey.PatchConvey("TestItemDoneEventShellOutputToContentBlock", t, func() {
		r := newStreamReceiver(&model.Options{})
		ev := responses.ResponseOutputItemDoneEvent{
			OutputIndex: 21,
		}

		mockey.Mock(responses.ResponseOutputItemUnion.AsAny).Return(responses.ResponseFunctionShellToolCallOutput{
			ID:     "shell_out1",
			Status: "completed",
			Output: []responses.ResponseFunctionShellToolCallOutputOutput{
				{
					Stdout: "output",
					Outcome: responses.ResponseFunctionShellToolCallOutputOutputOutcomeUnion{
						Type:     "exit",
						ExitCode: 0,
					},
				},
			},
		}).Build()

		blocks, err := r.itemDoneEventToContentBlocks(ev)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(blocks))
		assert.NotNil(t, blocks[0].ServerToolResult)
		assert.Equal(t, string(ServerToolNameShell), blocks[0].ServerToolResult.Name)
	})
}

func TestItemAddedEventToContentBlockShell(t *testing.T) {
	mockey.PatchConvey("TestItemAddedEventToContentBlockShellCall", t, func() {
		r := newStreamReceiver(&model.Options{})

		mockey.Mock(responses.ResponseOutputItemUnion.AsAny).Return(
			responses.ResponseFunctionShellToolCall{
				ID:     "shell1",
				CallID: "call_shell1",
				Status: "in_progress",
				Action: responses.ResponseFunctionShellToolCallAction{
					Commands: []string{"ls -la"},
				},
				Environment: responses.ResponseFunctionShellToolCallEnvironmentUnion{
					Type: "local",
				},
			},
		).Build()

		ev := responses.ResponseOutputItemAddedEvent{
			OutputIndex: 1,
		}
		blocks, err := r.itemAddedEventToContentBlock(ev)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(blocks))
		assert.NotNil(t, blocks[0].ServerToolCall)
		assert.Equal(t, string(ServerToolNameShell), blocks[0].ServerToolCall.Name)
	})

	mockey.PatchConvey("TestItemAddedEventToContentBlockShellOutput", t, func() {
		r := newStreamReceiver(&model.Options{})

		mockey.Mock(responses.ResponseOutputItemUnion.AsAny).Return(
			responses.ResponseFunctionShellToolCallOutput{
				ID:     "shellout1",
				CallID: "call_shell1",
				Status: "in_progress",
				Output: []responses.ResponseFunctionShellToolCallOutputOutput{
					{
						Stdout: "output",
						Outcome: responses.ResponseFunctionShellToolCallOutputOutputOutcomeUnion{
							Type:     "exit",
							ExitCode: 0,
						},
					},
				},
			},
		).Build()

		ev := responses.ResponseOutputItemAddedEvent{
			OutputIndex: 1,
		}
		blocks, err := r.itemAddedEventToContentBlock(ev)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(blocks))
		assert.NotNil(t, blocks[0].ServerToolResult)
		assert.Equal(t, string(ServerToolNameShell), blocks[0].ServerToolResult.Name)
	})
}

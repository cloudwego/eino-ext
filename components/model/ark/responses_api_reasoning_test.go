/*
 * Copyright 2025 CloudWeGo Authors
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
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model/responses"
)

func TestReasoningOutputConversion(t *testing.T) {
	t.Run("converts reasoning with ID and Status to message", func(t *testing.T) {
		cm := &ResponsesAPIChatModel{}
		reasoningID := "reasoning-123"

		resp := &responses.ResponseObject{
			Usage: &responses.Usage{},
			Output: []*responses.OutputItem{
				{
					Union: &responses.OutputItem_Reasoning{
						Reasoning: &responses.ItemReasoning{
							Id:   &reasoningID,
							Type: responses.ItemType_reasoning,
							Summary: []*responses.ReasoningSummaryPart{
								{
									Type: responses.ContentItemType_input_text,
									Text: "This is reasoning content",
								},
							},
						},
					},
				},
			},
		}

		msg, err := cm.toOutputMessage(resp, &cacheConfig{})
		require.NoError(t, err)
		assert.NotNil(t, msg)

		assert.Equal(t, "This is reasoning content", msg.ReasoningContent)

		id, ok := getReasoningID(msg)
		assert.True(t, ok)
		assert.Equal(t, "reasoning-123", id)

		order, hasOrder := getOutputItemsOrder(msg)
		assert.True(t, hasOrder)
		assert.Equal(t, []outputItemType{outputItemTypeReasoning}, order)
	})

	t.Run("converts multiple reasoning summaries", func(t *testing.T) {
		cm := &ResponsesAPIChatModel{}
		reasoningID := "reasoning-456"

		resp := &responses.ResponseObject{
			Usage: &responses.Usage{},
			Output: []*responses.OutputItem{
				{
					Union: &responses.OutputItem_Reasoning{
						Reasoning: &responses.ItemReasoning{
							Id:   &reasoningID,
							Type: responses.ItemType_reasoning,
							Summary: []*responses.ReasoningSummaryPart{
								{
									Type: responses.ContentItemType_input_text,
									Text: "First reasoning part",
								},
								{
									Type: responses.ContentItemType_input_text,
									Text: "Second reasoning part",
								},
							},
						},
					},
				},
			},
		}

		msg, err := cm.toOutputMessage(resp, &cacheConfig{})
		require.NoError(t, err)
		assert.Equal(t, "First reasoning part\n\nSecond reasoning part", msg.ReasoningContent)

		id, ok := getReasoningID(msg)
		assert.True(t, ok)
		assert.Equal(t, "reasoning-456", id)
	})

	t.Run("handles reasoning without ID or Status", func(t *testing.T) {
		cm := &ResponsesAPIChatModel{}

		resp := &responses.ResponseObject{
			Usage: &responses.Usage{},
			Output: []*responses.OutputItem{
				{
					Union: &responses.OutputItem_Reasoning{
						Reasoning: &responses.ItemReasoning{
							Type: responses.ItemType_reasoning,
							Summary: []*responses.ReasoningSummaryPart{
								{
									Type: responses.ContentItemType_input_text,
									Text: "Reasoning without metadata",
								},
							},
						},
					},
				},
			},
		}

		msg, err := cm.toOutputMessage(resp, &cacheConfig{})
		require.NoError(t, err)
		assert.Equal(t, "Reasoning without metadata", msg.ReasoningContent)

		_, ok := getReasoningID(msg)
		assert.False(t, ok)
	})
}

func TestReasoningInputConversion(t *testing.T) {
	t.Run("converts message with reasoning back to input item", func(t *testing.T) {
		cm := &ResponsesAPIChatModel{}

		msg := &schema.Message{
			Role:             schema.Assistant,
			ReasoningContent: "Reasoning content for input",
		}
		setReasoningID(msg, "reasoning-789")
		setOutputItemsOrder(msg, []outputItemType{outputItemTypeReasoning})

		req := &responses.ResponsesRequest{}
		err := cm.populateInput([]*schema.Message{msg}, req)
		require.NoError(t, err)

		inputList := req.Input.GetListValue()
		require.NotNil(t, inputList)
		require.Len(t, inputList.ListValue, 1)

		reasoningItem := inputList.ListValue[0].GetReasoning()
		require.NotNil(t, reasoningItem)
		assert.NotNil(t, reasoningItem.Id)
		assert.Equal(t, "reasoning-789", *reasoningItem.Id)
		assert.NotNil(t, reasoningItem.Status)
		assert.Equal(t, responses.ItemType_reasoning, reasoningItem.Type)
		require.Len(t, reasoningItem.Summary, 1)
		assert.Equal(t, "Reasoning content for input", reasoningItem.Summary[0].Text)
	})

	t.Run("converts message without ID and Status metadata", func(t *testing.T) {
		cm := &ResponsesAPIChatModel{}

		msg := &schema.Message{
			Role:             schema.Assistant,
			ReasoningContent: "Simple reasoning",
		}
		setOutputItemsOrder(msg, []outputItemType{outputItemTypeReasoning})

		req := &responses.ResponsesRequest{}
		err := cm.populateInput([]*schema.Message{msg}, req)
		require.NoError(t, err)

		inputList := req.Input.GetListValue()
		require.NotNil(t, inputList)
		require.Len(t, inputList.ListValue, 1)

		reasoningItem := inputList.ListValue[0].GetReasoning()
		require.NotNil(t, reasoningItem)
		assert.Nil(t, reasoningItem.Id)
		assert.Equal(t, responses.ItemType_reasoning, reasoningItem.Type)
		require.Len(t, reasoningItem.Summary, 1)
		assert.Equal(t, "Simple reasoning", reasoningItem.Summary[0].Text)
	})
}

func TestItemOrderRoundTrip(t *testing.T) {
	t.Run("preserves order of message, reasoning, and function_call", func(t *testing.T) {
		cm := &ResponsesAPIChatModel{}

		resp := &responses.ResponseObject{
			Usage: &responses.Usage{},
			Output: []*responses.OutputItem{
				{
					Union: &responses.OutputItem_Reasoning{
						Reasoning: &responses.ItemReasoning{
							Type: responses.ItemType_reasoning,
							Summary: []*responses.ReasoningSummaryPart{
								{Text: "Thinking..."},
							},
						},
					},
				},
				{
					Union: &responses.OutputItem_OutputMessage{
						OutputMessage: &responses.ItemOutputMessage{
							Type: responses.ItemType_message,
							Content: []*responses.OutputContentItem{
								{
									Union: &responses.OutputContentItem_Text{
										Text: &responses.OutputContentItemText{
											Type: responses.ContentItemType_output_text,
											Text: "Here is my response",
										},
									},
								},
							},
						},
					},
				},
				{
					Union: &responses.OutputItem_FunctionToolCall{
						FunctionToolCall: &responses.ItemFunctionToolCall{
							Type:      responses.ItemType_function_call,
							CallId:    "call-123",
							Name:      "search",
							Arguments: `{"query":"test"}`,
						},
					},
				},
			},
		}

		msg, err := cm.toOutputMessage(resp, &cacheConfig{})
		require.NoError(t, err)

		order, hasOrder := getOutputItemsOrder(msg)
		assert.True(t, hasOrder)
		assert.Equal(t, []outputItemType{
			outputItemTypeReasoning,
			outputItemTypeMessage,
			outputItemTypeFunctionCall,
		}, order)

		req := &responses.ResponsesRequest{}
		err = cm.populateInput([]*schema.Message{msg}, req)
		require.NoError(t, err)

		inputList := req.Input.GetListValue()
		require.NotNil(t, inputList)
		require.Len(t, inputList.ListValue, 3)

		assert.NotNil(t, inputList.ListValue[0].GetReasoning())
		assert.NotNil(t, inputList.ListValue[1].GetInputMessage())
		assert.NotNil(t, inputList.ListValue[2].GetFunctionToolCall())
	})

	t.Run("preserves order with multiple function calls", func(t *testing.T) {
		cm := &ResponsesAPIChatModel{}

		resp := &responses.ResponseObject{
			Usage: &responses.Usage{},
			Output: []*responses.OutputItem{
				{
					Union: &responses.OutputItem_OutputMessage{
						OutputMessage: &responses.ItemOutputMessage{
							Type: responses.ItemType_message,
							Content: []*responses.OutputContentItem{
								{
									Union: &responses.OutputContentItem_Text{
										Text: &responses.OutputContentItemText{
											Text: "Calling tools...",
										},
									},
								},
							},
						},
					},
				},
				{
					Union: &responses.OutputItem_FunctionToolCall{
						FunctionToolCall: &responses.ItemFunctionToolCall{
							CallId:    "call-1",
							Name:      "tool1",
							Arguments: `{}`,
						},
					},
				},
				{
					Union: &responses.OutputItem_FunctionToolCall{
						FunctionToolCall: &responses.ItemFunctionToolCall{
							CallId:    "call-2",
							Name:      "tool2",
							Arguments: `{}`,
						},
					},
				},
			},
		}

		msg, err := cm.toOutputMessage(resp, &cacheConfig{})
		require.NoError(t, err)

		order, hasOrder := getOutputItemsOrder(msg)
		assert.True(t, hasOrder)
		assert.Equal(t, []outputItemType{
			outputItemTypeMessage,
			outputItemTypeFunctionCall,
			outputItemTypeFunctionCall,
		}, order)

		req := &responses.ResponsesRequest{}
		err = cm.populateInput([]*schema.Message{msg}, req)
		require.NoError(t, err)

		inputList := req.Input.GetListValue()
		require.NotNil(t, inputList)
		require.Len(t, inputList.ListValue, 3)

		assert.NotNil(t, inputList.ListValue[0].GetInputMessage())
		assert.NotNil(t, inputList.ListValue[1].GetFunctionToolCall())
		assert.Equal(t, "call-1", inputList.ListValue[1].GetFunctionToolCall().CallId)
		assert.NotNil(t, inputList.ListValue[2].GetFunctionToolCall())
		assert.Equal(t, "call-2", inputList.ListValue[2].GetFunctionToolCall().CallId)
	})

	t.Run("fallback behavior without order metadata", func(t *testing.T) {
		cm := &ResponsesAPIChatModel{}

		msg := &schema.Message{
			Role:    schema.Assistant,
			Content: "Response",
			ToolCalls: []schema.ToolCall{
				{
					ID:   "call-1",
					Type: "function",
					Function: schema.FunctionCall{
						Name:      "search",
						Arguments: `{}`,
					},
				},
			},
		}

		req := &responses.ResponsesRequest{}
		err := cm.populateInput([]*schema.Message{msg}, req)
		require.NoError(t, err)

		inputList := req.Input.GetListValue()
		require.NotNil(t, inputList)
		require.Len(t, inputList.ListValue, 2)

		assert.NotNil(t, inputList.ListValue[0].GetInputMessage())
		assert.NotNil(t, inputList.ListValue[1].GetFunctionToolCall())
	})
}

func TestStreamResponseItemOrder(t *testing.T) {
	t.Run("stream sets item order correctly", func(t *testing.T) {
		msg := &schema.Message{
			Role:    schema.Assistant,
			Content: "text",
		}
		setOutputItemsOrder(msg, []outputItemType{outputItemTypeMessage})

		order, hasOrder := getOutputItemsOrder(msg)
		assert.True(t, hasOrder)
		assert.Equal(t, []outputItemType{outputItemTypeMessage}, order)
	})

	t.Run("reasoning stream chunk sets order", func(t *testing.T) {
		msg := &schema.Message{
			Role:             schema.Assistant,
			ReasoningContent: "thinking",
		}
		setReasoningContent(msg, "thinking")
		setOutputItemsOrder(msg, []outputItemType{outputItemTypeReasoning})

		order, hasOrder := getOutputItemsOrder(msg)
		assert.True(t, hasOrder)
		assert.Equal(t, []outputItemType{outputItemTypeReasoning}, order)

		content, ok := GetReasoningContent(msg)
		assert.True(t, ok)
		assert.Equal(t, "thinking", content)
	})

	t.Run("function call stream chunk sets order", func(t *testing.T) {
		msg := &schema.Message{
			Role: schema.Assistant,
			ToolCalls: []schema.ToolCall{
				{
					ID:   "call-1",
					Type: "function",
					Function: schema.FunctionCall{
						Name:      "search",
						Arguments: "{}",
					},
				},
			},
		}
		setOutputItemsOrder(msg, []outputItemType{outputItemTypeFunctionCall})

		order, hasOrder := getOutputItemsOrder(msg)
		assert.True(t, hasOrder)
		assert.Equal(t, []outputItemType{outputItemTypeFunctionCall}, order)
	})
}

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
	"testing"

	"github.com/bytedance/mockey"
	"github.com/cloudwego/eino/schema"
	openaischema "github.com/cloudwego/eino/schema/openai"
	"github.com/eino-contrib/jsonschema"
	"github.com/openai/openai-go/v3/packages/param"
	"github.com/openai/openai-go/v3/responses"
	"github.com/stretchr/testify/assert"
)

func TestToSystemRoleInputItems(t *testing.T) {
	mockey.PatchConvey("toSystemRoleInputItems", t, func() {
		mockey.PatchConvey("user_input_text", func() {
			msg := &schema.AgenticMessage{ContentBlocks: []*schema.ContentBlock{
				schema.NewContentBlock(&schema.UserInputText{Text: "hi"}),
			}}
			items, err := toSystemRoleInputItems(msg)
			assert.NoError(t, err)
			if assert.Len(t, items, 1) {
				assert.NotNil(t, items[0].OfMessage)
				assert.Equal(t, responses.EasyInputMessageRoleSystem, items[0].OfMessage.Role)
				assert.True(t, items[0].OfMessage.Content.OfString.Valid())
				assert.Equal(t, "hi", items[0].OfMessage.Content.OfString.Value)
			}
		})

		mockey.PatchConvey("invalid_block_type", func() {
			msg := &schema.AgenticMessage{ContentBlocks: []*schema.ContentBlock{
				schema.NewContentBlock(&schema.AssistantGenText{Text: "x"}),
			}}
			_, err := toSystemRoleInputItems(msg)
			assert.Error(t, err)
		})
	})
}

func TestToAssistantRoleInputItems(t *testing.T) {
	mockey.PatchConvey("toAssistantRoleInputItems", t, func() {
		msg := &schema.AgenticMessage{ContentBlocks: []*schema.ContentBlock{}}

		assistantText := schema.NewContentBlock(&schema.AssistantGenText{Text: "ok"})
		setItemID(assistantText, "msg1")
		setItemStatus(assistantText, "completed")
		msg.ContentBlocks = append(msg.ContentBlocks, assistantText)

		reasoning := schema.NewContentBlock(&schema.Reasoning{Text: "r"})
		setItemID(reasoning, "r1")
		setItemStatus(reasoning, "completed")
		msg.ContentBlocks = append(msg.ContentBlocks, reasoning)

		fc := schema.NewContentBlock(&schema.FunctionToolCall{CallID: "c1", Name: "f", Arguments: "{}"})
		setItemID(fc, "f1")
		setItemStatus(fc, "completed")
		msg.ContentBlocks = append(msg.ContentBlocks, fc)

		wsCall := schema.NewContentBlock(&schema.ServerToolCall{
			Name:      string(ServerToolNameWebSearch),
			Arguments: &ServerToolCallArguments{WebSearch: &WebSearchArguments{ActionType: WebSearchActionSearch, Search: &WebSearchQuery{Query: "q"}}},
		})
		setItemID(wsCall, "ws1")
		setItemStatus(wsCall, "in_progress")
		msg.ContentBlocks = append(msg.ContentBlocks, wsCall)

		wsRes := schema.NewContentBlock(&schema.ServerToolResult{
			Name:   string(ServerToolNameWebSearch),
			Result: &ServerToolResult{WebSearch: &WebSearchResult{ActionType: WebSearchActionSearch, Search: &WebSearchQueryResult{Sources: []*WebSearchQuerySource{{URL: "u"}}}}},
		})
		setItemID(wsRes, "ws1")
		setItemStatus(wsRes, "completed")
		msg.ContentBlocks = append(msg.ContentBlocks, wsRes)

		mcpCall := schema.NewContentBlock(&schema.MCPToolCall{ServerLabel: "srv", Name: "tool", Arguments: "{\"a\":1}"})
		setItemID(mcpCall, "m1")
		setItemStatus(mcpCall, "calling")
		msg.ContentBlocks = append(msg.ContentBlocks, mcpCall)

		mcpRes := schema.NewContentBlock(&schema.MCPToolResult{ServerLabel: "srv", Name: "tool", Result: "out"})
		setItemID(mcpRes, "m1")
		setItemStatus(mcpRes, "completed")
		msg.ContentBlocks = append(msg.ContentBlocks, mcpRes)

		items, err := toAssistantRoleInputItems(msg)
		assert.NoError(t, err)
		assert.Len(t, items, 5)

		var gotMcp *responses.ResponseInputItemMcpCallParam
		var gotWS *responses.ResponseFunctionWebSearchParam
		for i := range items {
			if items[i].OfMcpCall != nil {
				gotMcp = items[i].OfMcpCall
			}
			if items[i].OfWebSearchCall != nil {
				gotWS = items[i].OfWebSearchCall
			}
		}
		if assert.NotNil(t, gotMcp) {
			assert.Equal(t, "m1", gotMcp.ID)
			assert.Equal(t, "srv", gotMcp.ServerLabel)
			assert.Equal(t, "tool", gotMcp.Name)
			assert.Equal(t, "{\"a\":1}", gotMcp.Arguments)
			assert.True(t, gotMcp.Output.Valid())
			assert.Equal(t, "out", gotMcp.Output.Value)
		}
		if assert.NotNil(t, gotWS) {
			assert.Equal(t, "ws1", gotWS.ID)
			assert.NotNil(t, gotWS.Action.OfSearch)
			assert.Equal(t, "q", gotWS.Action.OfSearch.Query)
			assert.Len(t, gotWS.Action.OfSearch.Sources, 1)
			assert.Equal(t, "u", gotWS.Action.OfSearch.Sources[0].URL)
		}
	})
}

func TestPairMCPToolCallItems(t *testing.T) {
	mockey.PatchConvey("pairMCPToolCallItems", t, func() {
		mockey.PatchConvey("merge_call_and_result", func() {
			items := []responses.ResponseInputItemUnionParam{
				{OfMcpCall: &responses.ResponseInputItemMcpCallParam{ID: "m1", ServerLabel: "s", Name: "n", Arguments: "{}"}},
				{OfMcpCall: &responses.ResponseInputItemMcpCallParam{ID: "m1", ServerLabel: "s", Name: "n", Output: param.NewOpt("out")}},
			}
			newItems, err := pairMCPToolCallItems(items)
			assert.NoError(t, err)
			if assert.Len(t, newItems, 1) {
				m := newItems[0].OfMcpCall
				assert.NotNil(t, m)
				assert.Equal(t, "{}", m.Arguments)
				assert.True(t, m.Output.Valid())
				assert.Equal(t, "out", m.Output.Value)
			}
		})

		mockey.PatchConvey("missing_pair", func() {
			items := []responses.ResponseInputItemUnionParam{{OfMcpCall: &responses.ResponseInputItemMcpCallParam{ID: "m1", ServerLabel: "s", Name: "n", Arguments: "{}"}}}
			_, err := pairMCPToolCallItems(items)
			assert.Error(t, err)
		})
	})
}

func TestPairWebServerToolCallItems(t *testing.T) {
	mockey.PatchConvey("pairWebServerToolCallItems", t, func() {
		mockey.PatchConvey("merge_call_and_result", func() {
			items := []responses.ResponseInputItemUnionParam{
				{OfWebSearchCall: &responses.ResponseFunctionWebSearchParam{ID: "ws1", Action: responses.ResponseFunctionWebSearchActionUnionParam{OfSearch: &responses.ResponseFunctionWebSearchActionSearchParam{Query: "q"}}}},
				{OfWebSearchCall: &responses.ResponseFunctionWebSearchParam{ID: "ws1", Action: responses.ResponseFunctionWebSearchActionUnionParam{OfSearch: &responses.ResponseFunctionWebSearchActionSearchParam{Sources: []responses.ResponseFunctionWebSearchActionSearchSourceParam{{URL: "u"}}}}}},
			}
			newItems, err := pairWebServerToolCallItems(items)
			assert.NoError(t, err)
			if assert.Len(t, newItems, 1) {
				ws := newItems[0].OfWebSearchCall
				assert.NotNil(t, ws)
				assert.NotNil(t, ws.Action.OfSearch)
				assert.Equal(t, "q", ws.Action.OfSearch.Query)
				assert.Len(t, ws.Action.OfSearch.Sources, 1)
				assert.Equal(t, "u", ws.Action.OfSearch.Sources[0].URL)
			}
		})

		mockey.PatchConvey("missing_pair", func() {
			items := []responses.ResponseInputItemUnionParam{{OfWebSearchCall: &responses.ResponseFunctionWebSearchParam{ID: "ws1"}}}
			_, err := pairWebServerToolCallItems(items)
			assert.Error(t, err)
		})
	})
}

func TestPairWebSearchAction(t *testing.T) {
	mockey.PatchConvey("pairWebSearchAction", t, func() {
		a := responses.ResponseFunctionWebSearchActionUnionParam{OfSearch: &responses.ResponseFunctionWebSearchActionSearchParam{Query: "q"}}
		b := responses.ResponseFunctionWebSearchActionUnionParam{OfSearch: &responses.ResponseFunctionWebSearchActionSearchParam{Query: "q2", Sources: []responses.ResponseFunctionWebSearchActionSearchSourceParam{{URL: "u"}}}}
		merged := pairWebSearchAction(a, b)
		assert.NotNil(t, merged.OfSearch)
		assert.Equal(t, "q2", merged.OfSearch.Query)
		assert.Len(t, merged.OfSearch.Sources, 1)
		assert.Equal(t, "u", merged.OfSearch.Sources[0].URL)
	})
}

func TestToUserRoleInputItems(t *testing.T) {
	mockey.PatchConvey("toUserRoleInputItems", t, func() {
		mockey.PatchConvey("mix_user_inputs", func() {
			msg := &schema.AgenticMessage{ContentBlocks: []*schema.ContentBlock{
				schema.NewContentBlock(&schema.UserInputText{Text: "hi"}),
				schema.NewContentBlock(&schema.FunctionToolResult{CallID: "c", Result: "r"}),
				schema.NewContentBlock(&schema.MCPToolApprovalResponse{ApprovalRequestID: "a", Approve: true, Reason: "ok"}),
			}}
			items, err := toUserRoleInputItems(msg)
			assert.NoError(t, err)
			assert.Len(t, items, 3)
		})

		mockey.PatchConvey("invalid_block_type", func() {
			msg := &schema.AgenticMessage{ContentBlocks: []*schema.ContentBlock{
				schema.NewContentBlock(&schema.Reasoning{}),
			}}
			_, err := toUserRoleInputItems(msg)
			assert.Error(t, err)
		})
	})
}

func TestUserInputTextToInputItem(t *testing.T) {
	mockey.PatchConvey("userInputTextToInputItem", t, func() {
		item, err := userInputTextToInputItem(responses.EasyInputMessageRoleUser, &schema.UserInputText{Text: "hi"})
		assert.NoError(t, err)
		assert.NotNil(t, item.OfMessage)
		assert.True(t, item.OfMessage.Content.OfString.Valid())
		assert.Equal(t, "hi", item.OfMessage.Content.OfString.Value)
	})
}

func TestUserInputImageToInputItem(t *testing.T) {
	mockey.PatchConvey("userInputImageToInputItem", t, func() {
		mockey.PatchConvey("url", func() {
			item, err := userInputImageToInputItem(responses.EasyInputMessageRoleUser, &schema.UserInputImage{URL: "http://x", Detail: schema.ImageURLDetailAuto})
			assert.NoError(t, err)
			assert.NotNil(t, item.OfMessage)
			list := item.OfMessage.Content.OfInputItemContentList
			if assert.Len(t, list, 1) {
				img := list[0].OfInputImage
				assert.NotNil(t, img)
				assert.True(t, img.ImageURL.Valid())
				assert.Equal(t, "http://x", img.ImageURL.Value)
				assert.Equal(t, responses.ResponseInputImageDetailAuto, img.Detail)
			}
		})

		mockey.PatchConvey("base64_missing_mime", func() {
			_, err := userInputImageToInputItem(responses.EasyInputMessageRoleUser, &schema.UserInputImage{Base64Data: "abc"})
			assert.Error(t, err)
		})
	})
}

func TestToInputItemImageDetail(t *testing.T) {
	mockey.PatchConvey("toInputItemImageDetail", t, func() {
		mockey.PatchConvey("empty", func() {
			d, err := toInputItemImageDetail("")
			assert.NoError(t, err)
			assert.Equal(t, responses.ResponseInputImageDetail(""), d)
		})
		mockey.PatchConvey("invalid", func() {
			_, err := toInputItemImageDetail("bad")
			assert.Error(t, err)
		})
	})
}

func TestUserInputFileToInputItem(t *testing.T) {
	mockey.PatchConvey("userInputFileToInputItem", t, func() {
		mockey.PatchConvey("url", func() {
			item, err := userInputFileToInputItem(responses.EasyInputMessageRoleUser, &schema.UserInputFile{URL: "http://f", Name: "a.txt"})
			assert.NoError(t, err)
			assert.NotNil(t, item.OfMessage)
			list := item.OfMessage.Content.OfInputItemContentList
			if assert.Len(t, list, 1) {
				f := list[0].OfInputFile
				assert.NotNil(t, f)
				assert.True(t, f.FileURL.Valid())
				assert.Equal(t, "http://f", f.FileURL.Value)
				assert.True(t, f.Filename.Valid())
				assert.Equal(t, "a.txt", f.Filename.Value)
			}
		})

		mockey.PatchConvey("base64", func() {
			item, err := userInputFileToInputItem(responses.EasyInputMessageRoleUser, &schema.UserInputFile{Base64Data: "abc", MIMEType: "text/plain", Name: "a.txt"})
			assert.NoError(t, err)
			list := item.OfMessage.Content.OfInputItemContentList
			if assert.Len(t, list, 1) {
				f := list[0].OfInputFile
				assert.NotNil(t, f)
				assert.True(t, f.FileData.Valid())
				assert.Equal(t, "abc", f.FileData.Value)
				assert.False(t, f.FileURL.Valid())
			}
		})
	})
}

func TestFunctionToolResultToInputItem(t *testing.T) {
	mockey.PatchConvey("functionToolResultToInputItem", t, func() {
		item, err := functionToolResultToInputItem(&schema.FunctionToolResult{CallID: "c", Result: "r"})
		assert.NoError(t, err)
		assert.NotNil(t, item.OfFunctionCallOutput)
		assert.Equal(t, "c", item.OfFunctionCallOutput.CallID)
		assert.True(t, item.OfFunctionCallOutput.Output.OfString.Valid())
		assert.Equal(t, "r", item.OfFunctionCallOutput.Output.OfString.Value)
	})
}

func TestAssistantGenTextToInputItem(t *testing.T) {
	mockey.PatchConvey("assistantGenTextToInputItem", t, func() {
		mockey.PatchConvey("nil_content", func() {
			_, err := assistantGenTextToInputItem(&schema.ContentBlock{Type: schema.ContentBlockTypeAssistantGenText})
			assert.Error(t, err)
		})

		mockey.PatchConvey("with_annotations", func() {
			block := schema.NewContentBlock(&schema.AssistantGenText{
				Text: "t",
				OpenAIExtension: &openaischema.AssistantGenTextExtension{Annotations: []*openaischema.TextAnnotation{
					{Type: openaischema.TextAnnotationTypeURLCitation, URLCitation: &openaischema.TextAnnotationURLCitation{Title: "tt", URL: "u", StartIndex: 1, EndIndex: 2}},
				}},
			})
			setItemID(block, "msg1")
			setItemStatus(block, "completed")

			item, err := assistantGenTextToInputItem(block)
			assert.NoError(t, err)
			assert.NotNil(t, item.OfOutputMessage)
			assert.Equal(t, "msg1", item.OfOutputMessage.ID)
			assert.Equal(t, responses.ResponseOutputMessageStatus("completed"), item.OfOutputMessage.Status)
			if assert.Len(t, item.OfOutputMessage.Content, 1) {
				ot := item.OfOutputMessage.Content[0].OfOutputText
				assert.NotNil(t, ot)
				assert.Equal(t, "t", ot.Text)
				assert.Len(t, ot.Annotations, 1)
				assert.NotNil(t, ot.Annotations[0].OfURLCitation)
				assert.Equal(t, "u", ot.Annotations[0].OfURLCitation.URL)
			}
		})
	})
}

func TestTextAnnotationToOutputTextAnnotation(t *testing.T) {
	mockey.PatchConvey("textAnnotationToOutputTextAnnotation", t, func() {
		mockey.PatchConvey("file_citation", func() {
			p, err := textAnnotationToOutputTextAnnotation(&openaischema.TextAnnotation{Type: openaischema.TextAnnotationTypeFileCitation, FileCitation: &openaischema.TextAnnotationFileCitation{FileID: "f", Filename: "n", Index: 3}})
			assert.NoError(t, err)
			assert.NotNil(t, p.OfFileCitation)
			assert.Equal(t, int64(3), p.OfFileCitation.Index)
		})

		mockey.PatchConvey("invalid", func() {
			_, err := textAnnotationToOutputTextAnnotation(&openaischema.TextAnnotation{Type: "bad"})
			assert.Error(t, err)
		})
	})
}

func TestFunctionToolCallToInputItem(t *testing.T) {
	mockey.PatchConvey("functionToolCallToInputItem", t, func() {
		mockey.PatchConvey("nil_content", func() {
			_, err := functionToolCallToInputItem(&schema.ContentBlock{Type: schema.ContentBlockTypeFunctionToolCall})
			assert.Error(t, err)
		})

		mockey.PatchConvey("normal", func() {
			block := schema.NewContentBlock(&schema.FunctionToolCall{CallID: "c", Name: "n", Arguments: "{}"})
			setItemID(block, "id")
			setItemStatus(block, "completed")
			item, err := functionToolCallToInputItem(block)
			assert.NoError(t, err)
			assert.NotNil(t, item.OfFunctionCall)
			assert.True(t, item.OfFunctionCall.ID.Valid())
			assert.Equal(t, "id", item.OfFunctionCall.ID.Value)
			assert.Equal(t, "c", item.OfFunctionCall.CallID)
		})
	})
}

func TestReasoningToInputItem(t *testing.T) {
	mockey.PatchConvey("reasoningToInputItem", t, func() {
		block := schema.NewContentBlock(&schema.Reasoning{Text: "s", Signature: "e"})
		setItemID(block, "r")
		setItemStatus(block, "completed")
		item, err := reasoningToInputItem(block)
		assert.NoError(t, err)
		assert.NotNil(t, item.OfReasoning)
		assert.Equal(t, "r", item.OfReasoning.ID)
		assert.True(t, item.OfReasoning.EncryptedContent.Valid())
		assert.Equal(t, "e", item.OfReasoning.EncryptedContent.Value)
	})
}

func TestServerToolCallToInputItem(t *testing.T) {
	mockey.PatchConvey("serverToolCallToInputItem", t, func() {
		block := schema.NewContentBlock(&schema.ServerToolCall{
			Name:      string(ServerToolNameWebSearch),
			Arguments: &ServerToolCallArguments{WebSearch: &WebSearchArguments{ActionType: WebSearchActionSearch, Search: &WebSearchQuery{Query: "q"}}},
		})
		setItemID(block, "ws1")
		setItemStatus(block, "searching")
		item, err := serverToolCallToInputItem(block)
		assert.NoError(t, err)
		assert.NotNil(t, item.OfWebSearchCall)
		assert.Equal(t, "ws1", item.OfWebSearchCall.ID)
		assert.NotNil(t, item.OfWebSearchCall.Action.OfSearch)
		assert.Equal(t, "q", item.OfWebSearchCall.Action.OfSearch.Query)
	})
}

func TestGetWebSearchToolCallActionParam(t *testing.T) {
	mockey.PatchConvey("getWebSearchToolCallActionParam", t, func() {
		a, err := getWebSearchToolCallActionParam(&WebSearchArguments{ActionType: WebSearchActionFind, Find: &WebSearchFind{URL: "u", Pattern: "p"}})
		assert.NoError(t, err)
		assert.NotNil(t, a.OfFind)
		assert.Equal(t, "u", a.OfFind.URL)

		_, err = getWebSearchToolCallActionParam(&WebSearchArguments{ActionType: "bad"})
		assert.Error(t, err)
	})
}

func TestServerToolResultToInputItem(t *testing.T) {
	mockey.PatchConvey("serverToolResultToInputItem", t, func() {
		block := schema.NewContentBlock(&schema.ServerToolResult{
			Name:   string(ServerToolNameWebSearch),
			Result: &ServerToolResult{WebSearch: &WebSearchResult{ActionType: WebSearchActionSearch, Search: &WebSearchQueryResult{Sources: []*WebSearchQuerySource{{URL: "u"}}}}},
		})
		setItemID(block, "ws1")
		setItemStatus(block, "completed")
		item, err := serverToolResultToInputItem(block)
		assert.NoError(t, err)
		assert.NotNil(t, item.OfWebSearchCall)
		assert.Len(t, item.OfWebSearchCall.Action.OfSearch.Sources, 1)
		assert.Equal(t, "u", item.OfWebSearchCall.Action.OfSearch.Sources[0].URL)
	})
}

func TestGetWebSearchToolResultActionParam(t *testing.T) {
	mockey.PatchConvey("getWebSearchToolResultActionParam", t, func() {
		a, err := getWebSearchToolResultActionParam(&WebSearchResult{ActionType: WebSearchActionSearch, Search: &WebSearchQueryResult{Sources: []*WebSearchQuerySource{{URL: "u"}}}})
		assert.NoError(t, err)
		assert.NotNil(t, a.OfSearch)
		assert.Len(t, a.OfSearch.Sources, 1)
		assert.Equal(t, "u", a.OfSearch.Sources[0].URL)

		_, err = getWebSearchToolResultActionParam(&WebSearchResult{ActionType: "bad"})
		assert.Error(t, err)
	})
}

func TestMcpToolApprovalRequestToInputItem(t *testing.T) {
	mockey.PatchConvey("mcpToolApprovalRequestToInputItem", t, func() {
		block := schema.NewContentBlock(&schema.MCPToolApprovalRequest{ID: "a", Name: "n", Arguments: "{}", ServerLabel: "s"})
		setItemID(block, "a")
		item, err := mcpToolApprovalRequestToInputItem(block)
		assert.NoError(t, err)
		assert.NotNil(t, item.OfMcpApprovalRequest)
		assert.Equal(t, "a", item.OfMcpApprovalRequest.ID)
		assert.Equal(t, "n", item.OfMcpApprovalRequest.Name)
	})
}

func TestMcpToolApprovalResponseToInputItem(t *testing.T) {
	mockey.PatchConvey("mcpToolApprovalResponseToInputItem", t, func() {
		mockey.PatchConvey("empty_reason", func() {
			item, err := mcpToolApprovalResponseToInputItem(&schema.MCPToolApprovalResponse{ApprovalRequestID: "a", Approve: true})
			assert.NoError(t, err)
			assert.NotNil(t, item.OfMcpApprovalResponse)
			assert.False(t, item.OfMcpApprovalResponse.Reason.Valid())
		})

		mockey.PatchConvey("with_reason", func() {
			item, err := mcpToolApprovalResponseToInputItem(&schema.MCPToolApprovalResponse{ApprovalRequestID: "a", Approve: false, Reason: "r"})
			assert.NoError(t, err)
			assert.True(t, item.OfMcpApprovalResponse.Reason.Valid())
			assert.Equal(t, "r", item.OfMcpApprovalResponse.Reason.Value)
		})
	})
}

func TestMcpListToolsResultToInputItem(t *testing.T) {
	mockey.PatchConvey("mcpListToolsResultToInputItem", t, func() {
		block := schema.NewContentBlock(&schema.MCPListToolsResult{ServerLabel: "s", Tools: []*schema.MCPListToolsItem{{Name: "t", Description: "", InputSchema: &jsonschema.Schema{}}}})
		setItemID(block, "id")
		item, err := mcpListToolsResultToInputItem(block)
		assert.NoError(t, err)
		assert.NotNil(t, item.OfMcpListTools)
		assert.Equal(t, "id", item.OfMcpListTools.ID)
		if assert.Len(t, item.OfMcpListTools.Tools, 1) {
			assert.False(t, item.OfMcpListTools.Tools[0].Description.Valid())
		}
	})
}

func TestMcpToolCallToInputItem(t *testing.T) {
	mockey.PatchConvey("mcpToolCallToInputItem", t, func() {
		block := schema.NewContentBlock(&schema.MCPToolCall{ServerLabel: "s", Name: "n", Arguments: "{}"})
		setItemID(block, "id")
		setItemStatus(block, "calling")
		item, err := mcpToolCallToInputItem(block)
		assert.NoError(t, err)
		assert.NotNil(t, item.OfMcpCall)
		assert.Equal(t, "id", item.OfMcpCall.ID)
		assert.Equal(t, "{}", item.OfMcpCall.Arguments)
	})
}

func TestMcpToolResultToInputItem(t *testing.T) {
	mockey.PatchConvey("mcpToolResultToInputItem", t, func() {
		block := schema.NewContentBlock(&schema.MCPToolResult{ServerLabel: "s", Name: "n", Result: "out"})
		setItemID(block, "id")
		setItemStatus(block, "completed")
		item, err := mcpToolResultToInputItem(block)
		assert.NoError(t, err)
		assert.NotNil(t, item.OfMcpCall)
		assert.True(t, item.OfMcpCall.Output.Valid())
		assert.Equal(t, "out", item.OfMcpCall.Output.Value)
		assert.False(t, item.OfMcpCall.Error.Valid())
	})
}

func TestToOutputMessage(t *testing.T) {
	mockey.PatchConvey("toOutputMessage", t, func() {
		resp := &responses.Response{
			Output: []responses.ResponseOutputItemUnion{
				{
					Type:   "message",
					ID:     "m1",
					Status: "completed",
					Content: []responses.ResponseOutputMessageContentUnion{
						{Type: "output_text", Text: "hi", Annotations: []responses.ResponseOutputTextAnnotationUnion{}},
					},
				},
				{
					Type:    "reasoning",
					ID:      "r1",
					Status:  "completed",
					Summary: []responses.ResponseReasoningItemSummary{{Text: "s"}},
				},
			},
			Usage: responses.ResponseUsage{
				InputTokens:         1,
				InputTokensDetails:  responses.ResponseUsageInputTokensDetails{CachedTokens: 2},
				OutputTokens:        3,
				OutputTokensDetails: responses.ResponseUsageOutputTokensDetails{ReasoningTokens: 4},
				TotalTokens:         5,
			},
		}

		mockey.Mock(mockey.GetMethod(resp.Output[0], "AsAny")).Return(mockey.Sequence(
			responses.ResponseOutputMessage{
				Type:   "message",
				ID:     "m1",
				Status: "completed",
				Content: []responses.ResponseOutputMessageContentUnion{
					{Type: "output_text", Text: "hi", Annotations: []responses.ResponseOutputTextAnnotationUnion{}},
				},
			}).Then(responses.ResponseReasoningItem{
			Type:    "reasoning",
			ID:      "r1",
			Status:  "completed",
			Summary: []responses.ResponseReasoningItemSummary{{Text: "s"}},
		})).Build()
		msg, err := toOutputMessage(resp)
		assert.NoError(t, err)
		assert.NotNil(t, msg)
		assert.Equal(t, schema.AgenticRoleTypeAssistant, msg.Role)
		assert.Len(t, msg.ContentBlocks, 2)
		assert.NotNil(t, msg.ResponseMeta)
	})
}

func TestOutputMessageToContentBlocks(t *testing.T) {
	mockey.PatchConvey("outputMessageToContentBlocks", t, func() {
		item := responses.ResponseOutputMessage{
			ID:     "m1",
			Status: "completed",
			Content: []responses.ResponseOutputMessageContentUnion{
				{Type: "output_text", Text: "hi", Annotations: []responses.ResponseOutputTextAnnotationUnion{}},
				{Type: "refusal", Refusal: "no"},
			},
		}
		blocks, err := outputMessageToContentBlocks(item)
		assert.NoError(t, err)
		assert.Len(t, blocks, 2)
		for _, b := range blocks {
			id, ok := getItemID(b)
			assert.True(t, ok)
			assert.Equal(t, "m1", id)
		}
	})
}

func TestOutputContentTextToContentBlock(t *testing.T) {
	mockey.PatchConvey("outputContentTextToContentBlock", t, func() {
		text := responses.ResponseOutputText{Text: "hi", Annotations: []responses.ResponseOutputTextAnnotationUnion{{Type: "url_citation", Title: "t", URL: "u", StartIndex: 1, EndIndex: 2}}}
		block, err := outputContentTextToContentBlock(text)
		assert.NoError(t, err)
		assert.NotNil(t, block)
		assert.NotNil(t, block.AssistantGenText)
		assert.Equal(t, "hi", block.AssistantGenText.Text)
		if assert.NotNil(t, block.AssistantGenText.OpenAIExtension) {
			assert.Len(t, block.AssistantGenText.OpenAIExtension.Annotations, 1)
		}
	})
}

func TestOutputTextAnnotationToTextAnnotation(t *testing.T) {
	mockey.PatchConvey("outputTextAnnotationToTextAnnotation", t, func() {
		mockey.PatchConvey("file_citation_index_should_preserve", func() {
			a := responses.ResponseOutputTextAnnotationUnion{Type: "file_citation", FileID: "f", Filename: "n", Index: 5}

			mockey.Mock(responses.ResponseOutputTextAnnotationUnion.AsAny).Return(responses.ResponseOutputTextAnnotationFileCitation{
				FileID:   "f",
				Filename: "n",
				Index:    5,
			}).Build()

			ta, err := outputTextAnnotationToTextAnnotation(a)
			assert.NoError(t, err)
			assert.NotNil(t, ta)
			assert.NotNil(t, ta.FileCitation)
			assert.Equal(t, 5, ta.FileCitation.Index)
		})
	})
}

func TestFunctionToolCallToContentBlock(t *testing.T) {
	mockey.PatchConvey("functionToolCallToContentBlock", t, func() {
		item := responses.ResponseFunctionToolCall{ID: "id", Status: "completed", CallID: "c", Name: "n", Arguments: "{}"}
		block, err := functionToolCallToContentBlock(item)
		assert.NoError(t, err)
		assert.NotNil(t, block)
		assert.NotNil(t, block.FunctionToolCall)
		id, ok := getItemID(block)
		assert.True(t, ok)
		assert.Equal(t, "id", id)
	})
}

func TestWebSearchToContentBlocks(t *testing.T) {
	mockey.PatchConvey("webSearchToContentBlocks", t, func() {
		item := responses.ResponseFunctionWebSearch{
			ID:     "ws1",
			Status: "completed",
			Action: responses.ResponseFunctionWebSearchActionUnion{Type: "search", Query: "q", Sources: []responses.ResponseFunctionWebSearchActionSearchSource{{URL: "u"}}},
		}
		blocks, err := webSearchToContentBlocks(item)
		assert.NoError(t, err)
		assert.Len(t, blocks, 2)
		for _, b := range blocks {
			id, ok := getItemID(b)
			assert.True(t, ok)
			assert.Equal(t, "ws1", id)
		}
	})
}

func TestReasoningToContentBlocks(t *testing.T) {
	mockey.PatchConvey("reasoningToContentBlocks", t, func() {
		item := responses.ResponseReasoningItem{ID: "r1", Status: "completed", Summary: []responses.ResponseReasoningItemSummary{{Text: "s"}}}
		block, err := reasoningToContentBlocks(item)
		assert.NoError(t, err)
		id, ok := getItemID(block)
		assert.True(t, ok)
		assert.Equal(t, "r1", id)
		assert.NotNil(t, block.Reasoning)
		assert.Equal(t, "s", block.Reasoning.Text)
	})
}

func TestMcpCallToContentBlocks(t *testing.T) {
	mockey.PatchConvey("mcpCallToContentBlocks", t, func() {
		item := responses.ResponseOutputItemMcpCall{ID: "m1", ServerLabel: "s", Name: "n", Arguments: "{}", Output: "out"}
		blocks, err := mcpCallToContentBlocks(item)
		assert.NoError(t, err)
		assert.Len(t, blocks, 2)
		for _, b := range blocks {
			id, ok := getItemID(b)
			assert.True(t, ok)
			assert.Equal(t, "m1", id)
		}
	})
}

func TestMcpListToolsToContentBlock(t *testing.T) {
	mockey.PatchConvey("mcpListToolsToContentBlock", t, func() {
		item := responses.ResponseOutputItemMcpListTools{
			ID:          "l1",
			ServerLabel: "s",
			Tools: []responses.ResponseOutputItemMcpListToolsTool{
				{Name: "t", Description: "d", InputSchema: map[string]any{"type": "object"}},
			},
		}
		block, err := mcpListToolsToContentBlock(item)
		assert.NoError(t, err)
		assert.NotNil(t, block)
		assert.NotNil(t, block.MCPListToolsResult)
		id, ok := getItemID(block)
		assert.True(t, ok)
		assert.Equal(t, "l1", id)
		assert.Len(t, block.MCPListToolsResult.Tools, 1)
	})
}

func TestMcpApprovalRequestToContentBlock(t *testing.T) {
	mockey.PatchConvey("mcpApprovalRequestToContentBlock", t, func() {
		item := responses.ResponseOutputItemMcpApprovalRequest{ID: "a1", ServerLabel: "s", Name: "n", Arguments: "{}"}
		block, err := mcpApprovalRequestToContentBlock(item)
		assert.NoError(t, err)
		assert.NotNil(t, block)
		id, ok := getItemID(block)
		assert.True(t, ok)
		assert.Equal(t, "a1", id)
	})
}

func TestResponseObjectToResponseMeta(t *testing.T) {
	mockey.PatchConvey("responseObjectToResponseMeta", t, func() {
		resp := &responses.Response{Usage: responses.ResponseUsage{InputTokensDetails: responses.ResponseUsageInputTokensDetails{}, OutputTokensDetails: responses.ResponseUsageOutputTokensDetails{}}}
		meta := responseObjectToResponseMeta(resp)
		assert.NotNil(t, meta)
		assert.NotNil(t, meta.TokenUsage)
		assert.NotNil(t, meta.OpenAIExtension)
	})
}

func TestToTokenUsage(t *testing.T) {
	mockey.PatchConvey("toTokenUsage", t, func() {
		resp := &responses.Response{Usage: responses.ResponseUsage{InputTokens: 1, InputTokensDetails: responses.ResponseUsageInputTokensDetails{CachedTokens: 2}, OutputTokens: 3, OutputTokensDetails: responses.ResponseUsageOutputTokensDetails{ReasoningTokens: 4}, TotalTokens: 5}}
		u := toTokenUsage(resp)
		assert.NotNil(t, u)
		assert.Equal(t, 1, u.PromptTokens)
		assert.Equal(t, 2, u.PromptTokenDetails.CachedTokens)
		assert.Equal(t, 3, u.CompletionTokens)
		assert.Equal(t, 4, u.CompletionTokensDetails.ReasoningTokens)
		assert.Equal(t, 5, u.TotalTokens)
	})
}

func TestToResponseMetaExtension(t *testing.T) {
	mockey.PatchConvey("toResponseMetaExtension", t, func() {
		resp := &responses.Response{}
		resp.ID = "r"
		resp.Status = "completed"
		resp.Error.Code = "c"
		resp.Error.Message = "m"
		resp.IncompleteDetails.Reason = "x"
		resp.Reasoning.Effort = "low"
		resp.Reasoning.Summary = "sum"
		resp.ServiceTier = "auto"
		resp.CreatedAt = 123
		ext := toResponseMetaExtension(resp)
		assert.NotNil(t, ext)
		assert.Equal(t, "r", ext.ID)
		assert.NotNil(t, ext.Error)
		assert.Equal(t, openaischema.ResponseErrorCode("c"), ext.Error.Code)
		assert.NotNil(t, ext.IncompleteDetails)
		assert.Equal(t, "x", ext.IncompleteDetails.Reason)
	})
}

func TestResolveURL(t *testing.T) {
	mockey.PatchConvey("resolveURL", t, func() {
		mockey.PatchConvey("url", func() {
			u, err := resolveURL("http://x", "", "")
			assert.NoError(t, err)
			assert.Equal(t, "http://x", u)
		})

		mockey.PatchConvey("base64_without_mime", func() {
			_, err := resolveURL("", "abc", "")
			assert.Error(t, err)
		})

		mockey.PatchConvey("base64", func() {
			u, err := resolveURL("", "abc", "text/plain")
			assert.NoError(t, err)
			assert.Equal(t, "data:text/plain;base64,abc", u)
		})
	})
}

func TestEnsureDataURL(t *testing.T) {
	mockey.PatchConvey("ensureDataURL", t, func() {
		mockey.PatchConvey("already_data_url", func() {
			_, err := ensureDataURL("data:text/plain;base64,abc", "text/plain")
			assert.Error(t, err)
		})

		mockey.PatchConvey("missing_mime", func() {
			_, err := ensureDataURL("abc", "")
			assert.Error(t, err)
		})

		mockey.PatchConvey("ok", func() {
			u, err := ensureDataURL("abc", "text/plain")
			assert.NoError(t, err)
			assert.Equal(t, "data:text/plain;base64,abc", u)
		})
	})
}

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
	"github.com/cloudwego/eino/components/model"
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

func TestShellCallToContentBlock(t *testing.T) {
	mockey.PatchConvey("shellCallToContentBlock", t, func() {
		mockey.PatchConvey("with_commands", func() {
			item := responses.ResponseFunctionShellToolCall{
				ID:     "shell1",
				CallID: "call_shell1",
				Status: "completed",
				Action: responses.ResponseFunctionShellToolCallAction{
					Commands:        []string{"ls -la", "pwd"},
					TimeoutMs:       30000,
					MaxOutputLength: 1024,
				},
				Environment: responses.ResponseFunctionShellToolCallEnvironmentUnion{
					Type: "local",
				},
				CreatedBy: "user123",
			}

			block, err := shellCallToContentBlock(item)
			assert.NoError(t, err)

			id, ok := getItemID(block)
			assert.True(t, ok)
			assert.Equal(t, "shell1", id)
			assert.NotNil(t, block.ServerToolCall)
			assert.Equal(t, string(ServerToolNameShell), block.ServerToolCall.Name)
			assert.Equal(t, "call_shell1", block.ServerToolCall.CallID)

			args, ok := block.ServerToolCall.Arguments.(*ServerToolCallArguments)
			assert.True(t, ok)
			assert.NotNil(t, args.Shell)
			assert.NotNil(t, args.Shell.Action)
			assert.Equal(t, []string{"ls -la", "pwd"}, args.Shell.Action.Commands)
			assert.Equal(t, int64(30000), args.Shell.Action.TimeoutMs)
			assert.Equal(t, int64(1024), args.Shell.Action.MaxOutputLength)
			assert.NotNil(t, args.Shell.Environment)
			assert.Equal(t, ShellEnvironmentTypeLocal, args.Shell.Environment.Type)
			assert.Equal(t, "user123", args.Shell.CreatedBy)
		})

		mockey.PatchConvey("with_container_environment", func() {
			item := responses.ResponseFunctionShellToolCall{
				ID:     "shell2",
				CallID: "call_shell2",
				Status: "completed",
				Action: responses.ResponseFunctionShellToolCallAction{
					Commands: []string{"echo hello"},
				},
				Environment: responses.ResponseFunctionShellToolCallEnvironmentUnion{
					Type:        "container_reference",
					ContainerID: "container123",
				},
			}

			block, err := shellCallToContentBlock(item)
			assert.NoError(t, err)

			args, ok := block.ServerToolCall.Arguments.(*ServerToolCallArguments)
			assert.True(t, ok)
			assert.NotNil(t, args.Shell.Environment)
			assert.Equal(t, ShellEnvironmentTypeContainerReference, args.Shell.Environment.Type)
			assert.NotNil(t, args.Shell.Environment.ContainerReference)
			assert.Equal(t, "container123", args.Shell.Environment.ContainerReference.ContainerID)
		})
	})
}

func TestShellOutputToContentBlock(t *testing.T) {
	mockey.PatchConvey("shellOutputToContentBlock", t, func() {
		mockey.PatchConvey("with_exit_outcome", func() {
			item := responses.ResponseFunctionShellToolCallOutput{
				ID:     "shell_out1",
				Status: "completed",
				Output: []responses.ResponseFunctionShellToolCallOutputOutput{
					{
						Stdout: "file1.txt\nfile2.txt",
						Stderr: "",
						Outcome: responses.ResponseFunctionShellToolCallOutputOutputOutcomeUnion{
							Type:     "exit",
							ExitCode: 0,
						},
					},
				},
			}

			block, err := shellOutputToContentBlock(item)
			assert.NoError(t, err)

			id, ok := getItemID(block)
			assert.True(t, ok)
			assert.Equal(t, "shell_out1", id)
			assert.NotNil(t, block.ServerToolResult)
			assert.Equal(t, string(ServerToolNameShell), block.ServerToolResult.Name)

			result, ok := block.ServerToolResult.Result.(*ServerToolResult)
			assert.True(t, ok)
			assert.NotNil(t, result.Shell)
			assert.Len(t, result.Shell.Outputs, 1)
			assert.Equal(t, "file1.txt\nfile2.txt", result.Shell.Outputs[0].Stdout)
			assert.NotNil(t, result.Shell.Outputs[0].Outcome.Exit)
			assert.Equal(t, int64(0), result.Shell.Outputs[0].Outcome.Exit.ExitCode)
		})

		mockey.PatchConvey("with_timeout_outcome", func() {
			item := responses.ResponseFunctionShellToolCallOutput{
				ID:     "shell_out2",
				Status: "completed",
				Output: []responses.ResponseFunctionShellToolCallOutputOutput{
					{
						Stdout: "",
						Stderr: "timeout",
						Outcome: responses.ResponseFunctionShellToolCallOutputOutputOutcomeUnion{
							Type: "timeout",
						},
					},
				},
			}

			block, err := shellOutputToContentBlock(item)
			assert.NoError(t, err)

			result, ok := block.ServerToolResult.Result.(*ServerToolResult)
			assert.True(t, ok)
			assert.Nil(t, result.Shell.Outputs[0].Outcome.Exit)
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
			Arguments: &ServerToolCallArguments{WebSearch: &WebSearchArguments{ActionType: WebSearchActionSearch, Search: &WebSearchQuery{Queries: []string{"q"}}}},
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

		msg = setSelfGenerated(msg)

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
			assert.Equal(t, []string{"q"}, gotWS.Action.OfSearch.Queries)
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
	mockey.PatchConvey("pairWebSearchServerToolCallItems", t, func() {
		mockey.PatchConvey("merge_call_and_result", func() {
			items := []responses.ResponseInputItemUnionParam{
				{OfWebSearchCall: &responses.ResponseFunctionWebSearchParam{ID: "ws1", Action: responses.ResponseFunctionWebSearchActionUnionParam{OfSearch: &responses.ResponseFunctionWebSearchActionSearchParam{Queries: []string{"q"}}}}},
				{OfWebSearchCall: &responses.ResponseFunctionWebSearchParam{ID: "ws1", Action: responses.ResponseFunctionWebSearchActionUnionParam{OfSearch: &responses.ResponseFunctionWebSearchActionSearchParam{Sources: []responses.ResponseFunctionWebSearchActionSearchSourceParam{{URL: "u"}}}}}},
			}
			newItems, err := pairWebSearchServerToolCallItems(items)
			assert.NoError(t, err)
			if assert.Len(t, newItems, 1) {
				ws := newItems[0].OfWebSearchCall
				assert.NotNil(t, ws)
				assert.NotNil(t, ws.Action.OfSearch)
				assert.Equal(t, []string{"q"}, ws.Action.OfSearch.Queries)
				assert.Len(t, ws.Action.OfSearch.Sources, 1)
				assert.Equal(t, "u", ws.Action.OfSearch.Sources[0].URL)
			}
		})

		mockey.PatchConvey("missing_pair", func() {
			items := []responses.ResponseInputItemUnionParam{{OfWebSearchCall: &responses.ResponseFunctionWebSearchParam{ID: "ws1"}}}
			_, err := pairWebSearchServerToolCallItems(items)
			assert.Error(t, err)
		})
	})
}

func TestPairWebSearchAction(t *testing.T) {
	mockey.PatchConvey("pairWebSearchAction", t, func() {
		a := responses.ResponseFunctionWebSearchActionUnionParam{OfSearch: &responses.ResponseFunctionWebSearchActionSearchParam{Queries: []string{"q"}}}
		b := responses.ResponseFunctionWebSearchActionUnionParam{OfSearch: &responses.ResponseFunctionWebSearchActionSearchParam{Queries: []string{"q2"}, Sources: []responses.ResponseFunctionWebSearchActionSearchSourceParam{{URL: "u"}}}}
		merged := pairWebSearchAction(a, b)
		assert.NotNil(t, merged.OfSearch)
		assert.Equal(t, []string{"q2"}, merged.OfSearch.Queries)
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
			Arguments: &ServerToolCallArguments{WebSearch: &WebSearchArguments{ActionType: WebSearchActionSearch, Search: &WebSearchQuery{Queries: []string{"q"}}}},
		})
		setItemID(block, "ws1")
		setItemStatus(block, "searching")
		item, err := serverToolCallToInputItem(block)
		assert.NoError(t, err)
		assert.NotNil(t, item.OfWebSearchCall)
		assert.Equal(t, "ws1", item.OfWebSearchCall.ID)
		assert.NotNil(t, item.OfWebSearchCall.Action.OfSearch)
		assert.Equal(t, []string{"q"}, item.OfWebSearchCall.Action.OfSearch.Queries)
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
		msg, err := toOutputMessage(resp, &model.Options{})
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

func TestToolSearchToolCallToContentBlock(t *testing.T) {
	mockey.PatchConvey("toolSearchToolCallToContentBlock", t, func() {
		mockey.PatchConvey("client_execution", func() {
			item := responses.ResponseToolSearchCall{
				ID:        "ts1",
				CallID:    "call1",
				Status:    "completed",
				Execution: responses.ResponseToolSearchCallExecutionClient,
				Arguments: map[string]any{"query": "find tools"},
			}
			options := &model.Options{
				ToolSearchTool: &schema.ToolInfo{Name: "my_tool_search"},
			}
			block, err := toolSearchToolCallToContentBlock(item, options)
			assert.NoError(t, err)
			assert.NotNil(t, block)
			assert.Equal(t, schema.ContentBlockTypeFunctionToolCall, block.Type)
			assert.NotNil(t, block.FunctionToolCall)
			assert.Equal(t, "call1", block.FunctionToolCall.CallID)
			assert.Equal(t, "my_tool_search", block.FunctionToolCall.Name)
			id, ok := getItemID(block)
			assert.True(t, ok)
			assert.Equal(t, "ts1", id)
			assert.True(t, GetToolSearchToolCall(block))
		})

		mockey.PatchConvey("client_execution_without_tool_search_tool", func() {
			item := responses.ResponseToolSearchCall{
				ID:        "ts1",
				CallID:    "call1",
				Execution: responses.ResponseToolSearchCallExecutionClient,
				Arguments: map[string]any{"query": "find tools"},
			}
			options := &model.Options{}
			_, err := toolSearchToolCallToContentBlock(item, options)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "haven't set client tool search tool")
		})

		mockey.PatchConvey("server_execution", func() {
			item := responses.ResponseToolSearchCall{
				ID:        "ts2",
				CallID:    "call2",
				Status:    "completed",
				Execution: responses.ResponseToolSearchCallExecutionServer,
				Arguments: map[string]any{"query": "find tools"},
			}
			options := &model.Options{}
			block, err := toolSearchToolCallToContentBlock(item, options)
			assert.NoError(t, err)
			assert.NotNil(t, block)
			assert.Equal(t, schema.ContentBlockTypeServerToolCall, block.Type)
			assert.NotNil(t, block.ServerToolCall)
			assert.Equal(t, "call2", block.ServerToolCall.CallID)
			id, ok := getItemID(block)
			assert.True(t, ok)
			assert.Equal(t, "ts2", id)
		})

		mockey.PatchConvey("invalid_execution", func() {
			item := responses.ResponseToolSearchCall{
				ID:        "ts3",
				Execution: "invalid",
				Arguments: map[string]any{},
			}
			_, err := toolSearchToolCallToContentBlock(item, &model.Options{})
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "invalid tool search execution type")
		})
	})
}

func TestToolSearchToolResultToContentBlock(t *testing.T) {
	mockey.PatchConvey("toolSearchToolResultToContentBlock", t, func() {
		item := responses.ResponseToolSearchOutputItem{
			ID:     "tsr1",
			CallID: "call1",
			Status: "completed",
			Tools: []responses.ToolUnion{
				{Type: "function", Name: "tool1", Description: "desc1", Parameters: map[string]any{"type": "object"}},
			},
		}
		block, err := toolSearchToolResultToContentBlock(item)
		assert.NoError(t, err)
		assert.NotNil(t, block)
		assert.Equal(t, schema.ContentBlockTypeServerToolResult, block.Type)
		assert.NotNil(t, block.ServerToolResult)
		assert.Equal(t, "call1", block.ServerToolResult.CallID)
		result, ok := block.ServerToolResult.Result.(*ServerToolResult)
		assert.True(t, ok)
		assert.NotNil(t, result.ToolSearch)
		assert.Len(t, result.ToolSearch.Tools, 1)
		assert.Equal(t, "tool1", result.ToolSearch.Tools[0].Name)

		id, ok := getItemID(block)
		assert.True(t, ok)
		assert.Equal(t, "tsr1", id)
	})
}

func TestToolSearchToolResultToInputItem(t *testing.T) {
	mockey.PatchConvey("toolSearchToolResultToInputItem", t, func() {
		mockey.PatchConvey("success", func() {
			block := &schema.ToolSearchFunctionToolResult{
				CallID: "call1",
				Result: &schema.ToolSearchResult{
					Tools: []*schema.ToolInfo{
						{Name: "tool1", Desc: "desc1", ParamsOneOf: schema.NewParamsOneOfByJSONSchema(&jsonschema.Schema{Type: "object"})},
					},
				},
			}
			item, err := toolSearchToolResultToInputItem(block)
			assert.NoError(t, err)
			assert.NotNil(t, item.OfToolSearchOutput)
			assert.True(t, item.OfToolSearchOutput.CallID.Valid())
			assert.Equal(t, "call1", item.OfToolSearchOutput.CallID.Value)
			assert.Equal(t, responses.ResponseToolSearchOutputItemParamStatusCompleted, item.OfToolSearchOutput.Status)
			assert.Equal(t, responses.ResponseToolSearchOutputItemParamExecutionClient, item.OfToolSearchOutput.Execution)
			assert.Len(t, item.OfToolSearchOutput.Tools, 1)
		})

		mockey.PatchConvey("nil_result", func() {
			block := &schema.ToolSearchFunctionToolResult{
				CallID: "call1",
				Result: nil,
			}
			_, err := toolSearchToolResultToInputItem(block)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "tool search result should not be nil")
		})
	})
}

func TestToolSearchInUserRoleInputItems(t *testing.T) {
	mockey.PatchConvey("toolSearchInUserRoleInputItems", t, func() {
		msg := &schema.AgenticMessage{ContentBlocks: []*schema.ContentBlock{
			schema.NewContentBlock(&schema.UserInputText{Text: "hi"}),
			schema.NewContentBlock(&schema.ToolSearchFunctionToolResult{
				CallID: "call1",
				Result: &schema.ToolSearchResult{
					Tools: []*schema.ToolInfo{
						{Name: "tool1", Desc: "desc1", ParamsOneOf: schema.NewParamsOneOfByJSONSchema(&jsonschema.Schema{Type: "object"})},
					},
				},
			}),
		}}
		items, err := toUserRoleInputItems(msg)
		assert.NoError(t, err)
		assert.Len(t, items, 2)
		assert.NotNil(t, items[0].OfMessage)
		assert.NotNil(t, items[1].OfToolSearchOutput)
	})
}

func TestGetToolSearchResultActionParam(t *testing.T) {
	mockey.PatchConvey("getToolSearchResultActionParam", t, func() {
		ts := &ToolSearchResult{
			Tools: []*schema.ToolInfo{
				{Name: "tool1", Desc: "desc1", ParamsOneOf: schema.NewParamsOneOfByJSONSchema(&jsonschema.Schema{Type: "object"})},
			},
		}
		result, err := getToolSearchResultActionParam(ts)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, responses.ResponseToolSearchOutputItemParamExecution("server"), result.Execution)
		assert.Len(t, result.Tools, 1)
	})
}

func TestServerToolResultToInputItemWithToolSearch(t *testing.T) {
	mockey.PatchConvey("serverToolResultToInputItemWithToolSearch", t, func() {
		block := schema.NewContentBlock(&schema.ServerToolResult{
			CallID: "call1",
			Name:   "tool_search",
			Result: &ServerToolResult{ToolSearch: &ToolSearchResult{
				Tools: []*schema.ToolInfo{
					{Name: "tool1", Desc: "desc1", ParamsOneOf: schema.NewParamsOneOfByJSONSchema(&jsonschema.Schema{Type: "object"})},
				},
			}},
		})
		setItemID(block, "ts1")
		setItemStatus(block, "completed")
		item, err := serverToolResultToInputItem(block)
		assert.NoError(t, err)
		assert.NotNil(t, item.OfToolSearchOutput)
		assert.True(t, item.OfToolSearchOutput.CallID.Valid())
		assert.Equal(t, "call1", item.OfToolSearchOutput.CallID.Value)
		assert.True(t, item.OfToolSearchOutput.ID.Valid())
		assert.Equal(t, "ts1", item.OfToolSearchOutput.ID.Value)
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

func TestFileSearchToolCallToInputItem(t *testing.T) {
	mockey.PatchConvey("fileSearchToolCallToInputItem", t, func() {
		block := schema.NewContentBlock(&schema.ServerToolCall{
			Name:      string(ServerToolNameFileSearch),
			Arguments: &ServerToolCallArguments{FileSearch: &FileSearchArguments{Queries: []string{"query1", "query2"}}},
		})
		setItemID(block, "fs1")
		setItemStatus(block, "searching")

		item, err := fileSearchToolCallToInputItem(&FileSearchArguments{Queries: []string{"query1", "query2"}}, block)
		assert.NoError(t, err)
		assert.NotNil(t, item.OfFileSearchCall)
		assert.Equal(t, "fs1", item.OfFileSearchCall.ID)
		assert.Equal(t, responses.ResponseFileSearchToolCallStatus("searching"), item.OfFileSearchCall.Status)
		assert.Equal(t, []string{"query1", "query2"}, item.OfFileSearchCall.Queries)
	})
}

func TestFileSearchToolResultToInputItem(t *testing.T) {
	mockey.PatchConvey("fileSearchToolResultToInputItem", t, func() {
		mockey.PatchConvey("with_results_and_attributes", func() {
			strVal := "test_string"
			floatVal := 0.95
			boolVal := true
			result := &FileSearchResult{
				Results: []*FileSearchResultItem{
					{
						FileID:   "file1",
						FileName: "test.txt",
						Score:    0.85,
						Text:     "matching text",
						Attributes: map[string]*FileSearchAttribute{
							"str_attr":   {OfString: &strVal},
							"float_attr": {OfFloat: &floatVal},
							"bool_attr":  {OfBool: &boolVal},
						},
					},
				},
			}
			block := schema.NewContentBlock(&schema.ServerToolResult{
				Name:   string(ServerToolNameFileSearch),
				Result: &ServerToolResult{FileSearch: result},
			})
			setItemID(block, "fs1")
			setItemStatus(block, "completed")

			item, err := fileSearchToolResultToInputItem(result, block)
			assert.NoError(t, err)
			assert.NotNil(t, item.OfFileSearchCall)
			assert.Equal(t, "fs1", item.OfFileSearchCall.ID)
			assert.Equal(t, responses.ResponseFileSearchToolCallStatus("completed"), item.OfFileSearchCall.Status)
			if assert.Len(t, item.OfFileSearchCall.Results, 1) {
				r := item.OfFileSearchCall.Results[0]
				assert.True(t, r.FileID.Valid())
				assert.Equal(t, "file1", r.FileID.Value)
				assert.True(t, r.Filename.Valid())
				assert.Equal(t, "test.txt", r.Filename.Value)
				assert.Equal(t, 0.85, r.Score.Value)
				assert.True(t, r.Text.Valid())
				assert.Equal(t, "matching text", r.Text.Value)
				assert.Len(t, r.Attributes, 3)
			}
		})

		mockey.PatchConvey("empty_results", func() {
			result := &FileSearchResult{Results: []*FileSearchResultItem{}}
			block := schema.NewContentBlock(&schema.ServerToolResult{
				Name:   string(ServerToolNameFileSearch),
				Result: &ServerToolResult{FileSearch: result},
			})
			setItemID(block, "fs2")
			setItemStatus(block, "completed")

			item, err := fileSearchToolResultToInputItem(result, block)
			assert.NoError(t, err)
			assert.NotNil(t, item.OfFileSearchCall)
			assert.Len(t, item.OfFileSearchCall.Results, 0)
		})
	})
}

func TestServerToolCallToInputItemFileSearch(t *testing.T) {
	mockey.PatchConvey("serverToolCallToInputItem_fileSearch", t, func() {
		block := schema.NewContentBlock(&schema.ServerToolCall{
			Name:      string(ServerToolNameFileSearch),
			Arguments: &ServerToolCallArguments{FileSearch: &FileSearchArguments{Queries: []string{"search query"}}},
		})
		setItemID(block, "fs1")
		setItemStatus(block, "in_progress")

		item, err := serverToolCallToInputItem(block)
		assert.NoError(t, err)
		assert.NotNil(t, item.OfFileSearchCall)
		assert.Equal(t, "fs1", item.OfFileSearchCall.ID)
		assert.Equal(t, []string{"search query"}, item.OfFileSearchCall.Queries)
	})
}

func TestServerToolResultToInputItemFileSearch(t *testing.T) {
	mockey.PatchConvey("serverToolResultToInputItem_fileSearch", t, func() {
		block := schema.NewContentBlock(&schema.ServerToolResult{
			Name: string(ServerToolNameFileSearch),
			Result: &ServerToolResult{FileSearch: &FileSearchResult{
				Results: []*FileSearchResultItem{
					{FileID: "f1", FileName: "doc.pdf", Score: 0.9, Text: "content"},
				},
			}},
		})
		setItemID(block, "fs1")
		setItemStatus(block, "completed")

		item, err := serverToolResultToInputItem(block)
		assert.NoError(t, err)
		assert.NotNil(t, item.OfFileSearchCall)
		assert.Equal(t, "fs1", item.OfFileSearchCall.ID)
		if assert.Len(t, item.OfFileSearchCall.Results, 1) {
			assert.True(t, item.OfFileSearchCall.Results[0].FileID.Valid())
			assert.Equal(t, "f1", item.OfFileSearchCall.Results[0].FileID.Value)
		}
	})
}

func TestFileSearchToContentBlocks(t *testing.T) {
	mockey.PatchConvey("fileSearchToContentBlocks", t, func() {
		mockey.PatchConvey("with_results", func() {
			item := responses.ResponseFileSearchToolCall{
				ID:      "fs1",
				Status:  "completed",
				Queries: []string{"query1"},
				Results: []responses.ResponseFileSearchToolCallResult{
					{
						FileID:   "file1",
						Filename: "test.txt",
						Score:    0.85,
						Text:     "matched text",
						Attributes: map[string]responses.ResponseFileSearchToolCallResultAttributeUnion{
							"attr1": {OfString: "val1"},
						},
					},
				},
			}

			blocks, err := fileSearchToContentBlocks(item)
			assert.NoError(t, err)
			assert.Len(t, blocks, 2)

			callBlock := blocks[0]
			id, ok := getItemID(callBlock)
			assert.True(t, ok)
			assert.Equal(t, "fs1", id)
			assert.NotNil(t, callBlock.ServerToolCall)
			assert.Equal(t, string(ServerToolNameFileSearch), callBlock.ServerToolCall.Name)

			resBlock := blocks[1]
			id, ok = getItemID(resBlock)
			assert.True(t, ok)
			assert.Equal(t, "fs1", id)
			assert.NotNil(t, resBlock.ServerToolResult)
			assert.Equal(t, string(ServerToolNameFileSearch), resBlock.ServerToolResult.Name)
		})

		mockey.PatchConvey("empty_results", func() {
			item := responses.ResponseFileSearchToolCall{
				ID:      "fs2",
				Status:  "completed",
				Queries: []string{"query2"},
				Results: []responses.ResponseFileSearchToolCallResult{},
			}

			blocks, err := fileSearchToContentBlocks(item)
			assert.NoError(t, err)
			assert.Len(t, blocks, 2)
		})

		mockey.PatchConvey("with_float_attribute", func() {
			item := responses.ResponseFileSearchToolCall{
				ID:      "fs3",
				Status:  "completed",
				Queries: []string{"q"},
				Results: []responses.ResponseFileSearchToolCallResult{
					{
						FileID: "f1",
						Attributes: map[string]responses.ResponseFileSearchToolCallResultAttributeUnion{
							"score_attr": {OfFloat: 0.95},
						},
					},
				},
			}

			blocks, err := fileSearchToContentBlocks(item)
			assert.NoError(t, err)
			assert.Len(t, blocks, 2)
		})

		mockey.PatchConvey("with_bool_attribute", func() {
			item := responses.ResponseFileSearchToolCall{
				ID:      "fs4",
				Status:  "completed",
				Queries: []string{"q"},
				Results: []responses.ResponseFileSearchToolCallResult{
					{
						FileID: "f1",
						Attributes: map[string]responses.ResponseFileSearchToolCallResultAttributeUnion{
							"is_valid": {OfBool: true},
						},
					},
				},
			}

			blocks, err := fileSearchToContentBlocks(item)
			assert.NoError(t, err)
			assert.Len(t, blocks, 2)
		})
	})
}

func TestCodeInterpreterToContentBlocks(t *testing.T) {
	mockey.PatchConvey("codeInterpreterToContentBlocks", t, func() {
		mockey.PatchConvey("with_logs_output", func() {
			item := responses.ResponseCodeInterpreterToolCall{
				ID:          "ci1",
				Status:      "completed",
				Code:        "print('hello world')",
				ContainerID: "container123",
				Outputs: []responses.ResponseCodeInterpreterToolCallOutputUnion{
					{
						Type: "logs",
						Logs: "print('hello world')",
					},
				},
			}

			blocks, err := codeInterpreterToContentBlocks(item)
			assert.NoError(t, err)
			assert.Len(t, blocks, 2)

			callBlock := blocks[0]
			id, ok := getItemID(callBlock)
			assert.True(t, ok)
			assert.Equal(t, "ci1", id)
			assert.NotNil(t, callBlock.ServerToolCall)
			assert.Equal(t, string(ServerToolNameCodeInterpreter), callBlock.ServerToolCall.Name)

			resBlock := blocks[1]
			id, ok = getItemID(resBlock)
			assert.True(t, ok)
			assert.Equal(t, "ci1", id)
			assert.NotNil(t, resBlock.ServerToolResult)
			assert.Equal(t, string(ServerToolNameCodeInterpreter), resBlock.ServerToolResult.Name)
		})

		mockey.PatchConvey("with_image_output", func() {
			item := responses.ResponseCodeInterpreterToolCall{
				ID:          "ci2",
				Status:      "completed",
				Code:        "import matplotlib.pyplot as plt; plt.plot([1,2,3]); plt.show()",
				ContainerID: "container456",
				Outputs: []responses.ResponseCodeInterpreterToolCallOutputUnion{
					{
						Type: "image",
						URL:  "https://example.com/image.png",
					},
				},
			}

			blocks, err := codeInterpreterToContentBlocks(item)
			assert.NoError(t, err)
			assert.Len(t, blocks, 2)
		})

		mockey.PatchConvey("empty_outputs", func() {
			item := responses.ResponseCodeInterpreterToolCall{
				ID:          "ci3",
				Status:      "completed",
				Code:        "x = 1 + 1",
				ContainerID: "container789",
				Outputs:     []responses.ResponseCodeInterpreterToolCallOutputUnion{},
			}

			blocks, err := codeInterpreterToContentBlocks(item)
			assert.NoError(t, err)
			assert.Len(t, blocks, 2)

			resBlock := blocks[1]
			result, ok := resBlock.ServerToolResult.Result.(*ServerToolResult)
			assert.True(t, ok)
			assert.NotNil(t, result.CodeInterpreter)
			assert.Equal(t, "x = 1 + 1", result.CodeInterpreter.Code)
			assert.Equal(t, "container789", result.CodeInterpreter.ContainerID)
			assert.Empty(t, result.CodeInterpreter.Outputs)
		})
	})
}

func TestImageGenerationToContentBlocks(t *testing.T) {
	mockey.PatchConvey("imageGenerationToContentBlocks", t, func() {
		mockey.PatchConvey("with_result", func() {
			item := responses.ResponseOutputItemImageGenerationCall{
				ID:     "ig1",
				Status: "completed",
				Result: "base64_image_data_here",
			}

			blocks, err := imageGenerationToContentBlocks(item)
			assert.NoError(t, err)
			assert.Len(t, blocks, 2)

			callBlock := blocks[0]
			id, ok := getItemID(callBlock)
			assert.True(t, ok)
			assert.Equal(t, "ig1", id)
			assert.NotNil(t, callBlock.ServerToolCall)
			assert.Equal(t, string(ServerToolNameImageGeneration), callBlock.ServerToolCall.Name)

			resBlock := blocks[1]
			id, ok = getItemID(resBlock)
			assert.True(t, ok)
			assert.Equal(t, "ig1", id)
			assert.NotNil(t, resBlock.ServerToolResult)
			assert.Equal(t, string(ServerToolNameImageGeneration), resBlock.ServerToolResult.Name)

			result, ok := resBlock.ServerToolResult.Result.(*ServerToolResult)
			assert.True(t, ok)
			assert.NotNil(t, result.ImageGeneration)
			assert.Equal(t, "base64_image_data_here", result.ImageGeneration.ImageBase64)
		})

		mockey.PatchConvey("in_progress_status", func() {
			item := responses.ResponseOutputItemImageGenerationCall{
				ID:     "ig2",
				Status: "in_progress",
				Result: "",
			}

			blocks, err := imageGenerationToContentBlocks(item)
			assert.NoError(t, err)
			assert.Len(t, blocks, 2)

			status, ok := GetItemStatus(blocks[0])
			assert.True(t, ok)
			assert.Equal(t, "in_progress", status)
		})
	})
}

func TestCodeInterpreterToolCallToInputItem(t *testing.T) {
	mockey.PatchConvey("codeInterpreterToolCallToInputItem", t, func() {
		block := schema.NewContentBlock(&schema.ServerToolCall{
			Name:      string(ServerToolNameCodeInterpreter),
			Arguments: &ServerToolCallArguments{CodeInterpreter: &CodeInterpreterArguments{}},
		})
		setItemID(block, "ci1")
		setItemStatus(block, "completed")

		item, err := codeInterpreterToolCallToInputItem(&CodeInterpreterArguments{}, block)
		assert.NoError(t, err)
		assert.NotNil(t, item.OfCodeInterpreterCall)
		assert.Equal(t, "ci1", item.OfCodeInterpreterCall.ID)
		assert.Equal(t, responses.ResponseCodeInterpreterToolCallStatus("completed"), item.OfCodeInterpreterCall.Status)
	})
}

func TestCodeInterpreterToolResultToInputItem(t *testing.T) {
	mockey.PatchConvey("codeInterpreterToolResultToInputItem", t, func() {
		mockey.PatchConvey("with_logs_and_image_outputs", func() {
			result := &CodeInterpreterResult{
				Code:        "print('hello')",
				ContainerID: "ctr1",
				Outputs: []*CodeInterpreterOutput{
					{
						Type: CodeInterpreterOutputTypeLogs,
						Logs: &CodeInterpreterOutputLogs{Logs: "hello"},
					},
					{
						Type:  CodeInterpreterOutputTypeImage,
						Image: &CodeInterpreterOutputImage{URL: "https://example.com/img.png"},
					},
				},
			}

			block := schema.NewContentBlock(&schema.ServerToolResult{
				Name:   string(ServerToolNameCodeInterpreter),
				Result: &ServerToolResult{CodeInterpreter: result},
			})
			setItemID(block, "ci1")
			setItemStatus(block, "completed")

			item, err := codeInterpreterToolResultToInputItem(result, block)
			assert.NoError(t, err)
			assert.NotNil(t, item.OfCodeInterpreterCall)
			assert.Equal(t, "ci1", item.OfCodeInterpreterCall.ID)
			assert.Equal(t, "ctr1", item.OfCodeInterpreterCall.ContainerID)
			assert.True(t, item.OfCodeInterpreterCall.Code.Valid())
			assert.Equal(t, "print('hello')", item.OfCodeInterpreterCall.Code.Value)
			assert.Len(t, item.OfCodeInterpreterCall.Outputs, 2)
			assert.NotNil(t, item.OfCodeInterpreterCall.Outputs[0].OfLogs)
			assert.Equal(t, "hello", item.OfCodeInterpreterCall.Outputs[0].OfLogs.Logs)
			assert.NotNil(t, item.OfCodeInterpreterCall.Outputs[1].OfImage)
			assert.Equal(t, "https://example.com/img.png", item.OfCodeInterpreterCall.Outputs[1].OfImage.URL)
		})

		mockey.PatchConvey("unknown_output_type", func() {
			result := &CodeInterpreterResult{
				Outputs: []*CodeInterpreterOutput{
					{Type: "unknown_type"},
				},
			}
			block := schema.NewContentBlock(&schema.ServerToolResult{
				Name:   string(ServerToolNameCodeInterpreter),
				Result: &ServerToolResult{CodeInterpreter: result},
			})
			_, err := codeInterpreterToolResultToInputItem(result, block)
			assert.Error(t, err)
		})
	})
}

func TestImageGenerationToolCallToInputItem(t *testing.T) {
	mockey.PatchConvey("imageGenerationToolCallToInputItem", t, func() {
		block := schema.NewContentBlock(&schema.ServerToolCall{
			Name:      string(ServerToolNameImageGeneration),
			Arguments: &ServerToolCallArguments{ImageGeneration: &ImageGenerationArguments{}},
		})
		setItemID(block, "ig1")
		setItemStatus(block, "in_progress")

		item, err := imageGenerationToolCallToInputItem(&ImageGenerationArguments{}, block)
		assert.NoError(t, err)
		assert.NotNil(t, item.OfImageGenerationCall)
		assert.Equal(t, "ig1", item.OfImageGenerationCall.ID)
		assert.Equal(t, "in_progress", item.OfImageGenerationCall.Status)
	})
}

func TestImageGenerationToolResultToInputItem(t *testing.T) {
	mockey.PatchConvey("imageGenerationToolResultToInputItem", t, func() {
		result := &ImageGenerationResult{ImageBase64: "abc123base64"}
		block := schema.NewContentBlock(&schema.ServerToolResult{
			Name:   string(ServerToolNameImageGeneration),
			Result: &ServerToolResult{ImageGeneration: result},
		})
		setItemID(block, "ig1")
		setItemStatus(block, "completed")

		item, err := imageGenerationToolResultToInputItem(result, block)
		assert.NoError(t, err)
		assert.NotNil(t, item.OfImageGenerationCall)
		assert.Equal(t, "ig1", item.OfImageGenerationCall.ID)
		assert.Equal(t, "completed", item.OfImageGenerationCall.Status)
		assert.True(t, item.OfImageGenerationCall.Result.Valid())
		assert.Equal(t, "abc123base64", item.OfImageGenerationCall.Result.Value)
	})
}

func TestShellToolCallToInputItem(t *testing.T) {
	mockey.PatchConvey("shellToolCallToInputItem", t, func() {
		mockey.PatchConvey("local_environment_with_skills", func() {
			args := &ShellArguments{
				Action: &ShellAction{
					Commands:        []string{"ls -la", "pwd"},
					TimeoutMs:       30000,
					MaxOutputLength: 1024,
				},
				Environment: &ShellEnvironment{
					Type: ShellEnvironmentTypeLocal,
					Local: &ShellEnvironmentLocal{
						Skills: []*ShellEnvironmentLocalSkill{
							{Name: "go", Description: "Go language", Path: "/usr/local/go"},
						},
					},
				},
				CreatedBy: "user1",
			}
			block := schema.NewContentBlock(&schema.ServerToolCall{
				Name:      string(ServerToolNameShell),
				CallID:    "call_1",
				Arguments: &ServerToolCallArguments{Shell: args},
			})
			setItemID(block, "sh1")
			setItemStatus(block, "completed")

			item, err := shellToolCallToInputItem(args, block)
			assert.NoError(t, err)
			assert.NotNil(t, item.OfShellCall)
			assert.True(t, item.OfShellCall.ID.Valid())
			assert.Equal(t, "sh1", item.OfShellCall.ID.Value)
			assert.Equal(t, "call_1", item.OfShellCall.CallID)
			assert.Equal(t, "completed", item.OfShellCall.Status)
			assert.Equal(t, []string{"ls -la", "pwd"}, item.OfShellCall.Action.Commands)
			assert.True(t, item.OfShellCall.Action.TimeoutMs.Valid())
			assert.Equal(t, int64(30000), item.OfShellCall.Action.TimeoutMs.Value)
			assert.True(t, item.OfShellCall.Action.MaxOutputLength.Valid())
			assert.Equal(t, int64(1024), item.OfShellCall.Action.MaxOutputLength.Value)
			assert.NotNil(t, item.OfShellCall.Environment.OfLocal)
			assert.Len(t, item.OfShellCall.Environment.OfLocal.Skills, 1)
			assert.Equal(t, "go", item.OfShellCall.Environment.OfLocal.Skills[0].Name)
		})

		mockey.PatchConvey("container_environment", func() {
			args := &ShellArguments{
				Action: &ShellAction{
					Commands: []string{"echo hello"},
				},
				Environment: &ShellEnvironment{
					Type: ShellEnvironmentTypeContainerReference,
					ContainerReference: &ShellEnvironmentContainerReference{
						ContainerID: "ctr123",
					},
				},
			}
			block := schema.NewContentBlock(&schema.ServerToolCall{
				Name:      string(ServerToolNameShell),
				CallID:    "call_2",
				Arguments: &ServerToolCallArguments{Shell: args},
			})
			setItemID(block, "sh2")

			item, err := shellToolCallToInputItem(args, block)
			assert.NoError(t, err)
			assert.NotNil(t, item.OfShellCall)
			assert.NotNil(t, item.OfShellCall.Environment.OfContainerReference)
			assert.Equal(t, "ctr123", item.OfShellCall.Environment.OfContainerReference.ContainerID)
		})

		mockey.PatchConvey("unknown_environment_type", func() {
			args := &ShellArguments{
				Action: &ShellAction{Commands: []string{"echo"}},
				Environment: &ShellEnvironment{
					Type: "unknown",
				},
			}
			block := schema.NewContentBlock(&schema.ServerToolCall{
				Name:      string(ServerToolNameShell),
				CallID:    "call_3",
				Arguments: &ServerToolCallArguments{Shell: args},
			})
			_, err := shellToolCallToInputItem(args, block)
			assert.Error(t, err)
		})
	})
}

func TestShellToolResultToInputItem(t *testing.T) {
	mockey.PatchConvey("shellToolResultToInputItem", t, func() {
		mockey.PatchConvey("with_exit_outcome", func() {
			result := &ShellResult{
				MaxOutputLength: 2048,
				CreatedBy:       "user1",
				Outputs: []*ShellOutputItem{
					{
						Stdout: "file.txt",
						Stderr: "",
						Outcome: &ShellOutputOutcome{
							Type: ShellOutputOutcomeTypeExit,
							Exit: &ShellOutputOutcomeExit{ExitCode: 0},
						},
					},
				},
			}
			block := schema.NewContentBlock(&schema.ServerToolResult{
				Name:   string(ServerToolNameShell),
				CallID: "call_1",
				Result: &ServerToolResult{Shell: result},
			})
			setItemID(block, "sh1")
			setItemStatus(block, "completed")

			item, err := shellToolResultToInputItem(result, block)
			assert.NoError(t, err)
			assert.NotNil(t, item.OfShellCallOutput)
			assert.True(t, item.OfShellCallOutput.ID.Valid())
			assert.Equal(t, "sh1", item.OfShellCallOutput.ID.Value)
			assert.Equal(t, "call_1", item.OfShellCallOutput.CallID)
			assert.Equal(t, "completed", item.OfShellCallOutput.Status)
			assert.True(t, item.OfShellCallOutput.MaxOutputLength.Valid())
			assert.Equal(t, int64(2048), item.OfShellCallOutput.MaxOutputLength.Value)
			assert.Len(t, item.OfShellCallOutput.Output, 1)
			assert.Equal(t, "file.txt", item.OfShellCallOutput.Output[0].Stdout)
			assert.NotNil(t, item.OfShellCallOutput.Output[0].Outcome.OfExit)
			assert.Equal(t, int64(0), item.OfShellCallOutput.Output[0].Outcome.OfExit.ExitCode)
		})

		mockey.PatchConvey("with_timeout_outcome", func() {
			result := &ShellResult{
				Outputs: []*ShellOutputItem{
					{
						Stdout: "partial",
						Stderr: "timed out",
						Outcome: &ShellOutputOutcome{
							Type: ShellOutputOutcomeTypeTimeout,
						},
					},
				},
			}
			block := schema.NewContentBlock(&schema.ServerToolResult{
				Name:   string(ServerToolNameShell),
				CallID: "call_2",
				Result: &ServerToolResult{Shell: result},
			})
			setItemID(block, "sh2")

			item, err := shellToolResultToInputItem(result, block)
			assert.NoError(t, err)
			assert.NotNil(t, item.OfShellCallOutput)
			assert.Len(t, item.OfShellCallOutput.Output, 1)
			assert.NotNil(t, item.OfShellCallOutput.Output[0].Outcome.OfTimeout)
		})

		mockey.PatchConvey("unknown_outcome_type", func() {
			result := &ShellResult{
				Outputs: []*ShellOutputItem{
					{
						Outcome: &ShellOutputOutcome{Type: "unknown"},
					},
				},
			}
			block := schema.NewContentBlock(&schema.ServerToolResult{
				Name:   string(ServerToolNameShell),
				CallID: "call_3",
				Result: &ServerToolResult{Shell: result},
			})
			_, err := shellToolResultToInputItem(result, block)
			assert.Error(t, err)
		})
	})
}

func TestServerToolCallToInputItem_CodeInterpreter(t *testing.T) {
	mockey.PatchConvey("serverToolCallToInputItem_code_interpreter", t, func() {
		block := schema.NewContentBlock(&schema.ServerToolCall{
			Name:      string(ServerToolNameCodeInterpreter),
			Arguments: &ServerToolCallArguments{CodeInterpreter: &CodeInterpreterArguments{}},
		})
		setItemID(block, "ci1")
		item, err := serverToolCallToInputItem(block)
		assert.NoError(t, err)
		assert.NotNil(t, item.OfCodeInterpreterCall)
		assert.Equal(t, "ci1", item.OfCodeInterpreterCall.ID)
	})
}

func TestServerToolCallToInputItem_ImageGeneration(t *testing.T) {
	mockey.PatchConvey("serverToolCallToInputItem_image_generation", t, func() {
		block := schema.NewContentBlock(&schema.ServerToolCall{
			Name:      string(ServerToolNameImageGeneration),
			Arguments: &ServerToolCallArguments{ImageGeneration: &ImageGenerationArguments{}},
		})
		setItemID(block, "ig1")
		item, err := serverToolCallToInputItem(block)
		assert.NoError(t, err)
		assert.NotNil(t, item.OfImageGenerationCall)
		assert.Equal(t, "ig1", item.OfImageGenerationCall.ID)
	})
}

func TestServerToolCallToInputItem_Shell(t *testing.T) {
	mockey.PatchConvey("serverToolCallToInputItem_shell", t, func() {
		block := schema.NewContentBlock(&schema.ServerToolCall{
			Name:   string(ServerToolNameShell),
			CallID: "call_sh",
			Arguments: &ServerToolCallArguments{Shell: &ShellArguments{
				Action: &ShellAction{Commands: []string{"ls"}},
			}},
		})
		setItemID(block, "sh1")
		item, err := serverToolCallToInputItem(block)
		assert.NoError(t, err)
		assert.NotNil(t, item.OfShellCall)
		assert.Equal(t, "call_sh", item.OfShellCall.CallID)
	})
}

func TestServerToolCallToInputItem_Nil(t *testing.T) {
	mockey.PatchConvey("serverToolCallToInputItem_nil", t, func() {
		block := schema.NewContentBlock(&schema.ServerToolCall{
			Name:      string(ServerToolNameCodeInterpreter),
			Arguments: &ServerToolCallArguments{},
		})
		_, err := serverToolCallToInputItem(block)
		assert.Error(t, err)
	})
}

func TestServerToolResultToInputItem_CodeInterpreter(t *testing.T) {
	mockey.PatchConvey("serverToolResultToInputItem_code_interpreter", t, func() {
		block := schema.NewContentBlock(&schema.ServerToolResult{
			Name: string(ServerToolNameCodeInterpreter),
			Result: &ServerToolResult{CodeInterpreter: &CodeInterpreterResult{
				Code:        "x = 1",
				ContainerID: "ctr1",
			}},
		})
		setItemID(block, "ci1")
		setItemStatus(block, "completed")
		item, err := serverToolResultToInputItem(block)
		assert.NoError(t, err)
		assert.NotNil(t, item.OfCodeInterpreterCall)
		assert.Equal(t, "ci1", item.OfCodeInterpreterCall.ID)
		assert.Equal(t, "ctr1", item.OfCodeInterpreterCall.ContainerID)
	})
}

func TestServerToolResultToInputItem_ImageGeneration(t *testing.T) {
	mockey.PatchConvey("serverToolResultToInputItem_image_generation", t, func() {
		block := schema.NewContentBlock(&schema.ServerToolResult{
			Name: string(ServerToolNameImageGeneration),
			Result: &ServerToolResult{ImageGeneration: &ImageGenerationResult{
				ImageBase64: "abc",
			}},
		})
		setItemID(block, "ig1")
		item, err := serverToolResultToInputItem(block)
		assert.NoError(t, err)
		assert.NotNil(t, item.OfImageGenerationCall)
		assert.True(t, item.OfImageGenerationCall.Result.Valid())
		assert.Equal(t, "abc", item.OfImageGenerationCall.Result.Value)
	})
}

func TestServerToolResultToInputItem_Shell(t *testing.T) {
	mockey.PatchConvey("serverToolResultToInputItem_shell", t, func() {
		block := schema.NewContentBlock(&schema.ServerToolResult{
			Name:   string(ServerToolNameShell),
			CallID: "call_sh",
			Result: &ServerToolResult{Shell: &ShellResult{
				Outputs: []*ShellOutputItem{
					{Stdout: "ok"},
				},
			}},
		})
		setItemID(block, "sh1")
		item, err := serverToolResultToInputItem(block)
		assert.NoError(t, err)
		assert.NotNil(t, item.OfShellCallOutput)
		assert.Equal(t, "call_sh", item.OfShellCallOutput.CallID)
		assert.Len(t, item.OfShellCallOutput.Output, 1)
		assert.Equal(t, "ok", item.OfShellCallOutput.Output[0].Stdout)
	})
}

func TestServerToolResultToInputItem_Nil(t *testing.T) {
	mockey.PatchConvey("serverToolResultToInputItem_nil", t, func() {
		block := schema.NewContentBlock(&schema.ServerToolResult{
			Name:   string(ServerToolNameCodeInterpreter),
			Result: &ServerToolResult{},
		})
		_, err := serverToolResultToInputItem(block)
		assert.Error(t, err)
	})
}

func TestPairFileSearchToolCallItems(t *testing.T) {
	mockey.PatchConvey("pairFileSearchToolCallItems", t, func() {
		mockey.PatchConvey("merge_call_and_result", func() {
			call := responses.ResponseInputItemUnionParam{
				OfFileSearchCall: &responses.ResponseFileSearchToolCallParam{
					ID:      "fs1",
					Status:  "searching",
					Queries: []string{"query1"},
				},
			}
			result := responses.ResponseInputItemUnionParam{
				OfFileSearchCall: &responses.ResponseFileSearchToolCallParam{
					ID:     "fs1",
					Status: "completed",
					Results: []responses.ResponseFileSearchToolCallResultParam{
						{
							Filename: param.NewOpt("file.txt"),
							Score:    param.NewOpt(0.9),
							Text:     param.NewOpt("content"),
						},
					},
				},
			}
			other := responses.ResponseInputItemUnionParam{
				OfWebSearchCall: &responses.ResponseFunctionWebSearchParam{ID: "ws1"},
			}

			items, err := pairFileSearchToolCallItems([]responses.ResponseInputItemUnionParam{call, other, result})
			assert.NoError(t, err)
			assert.Len(t, items, 2)

			fs := items[0].OfFileSearchCall
			assert.NotNil(t, fs)
			assert.Equal(t, "fs1", fs.ID)
			assert.Equal(t, []string{"query1"}, fs.Queries)
			assert.Len(t, fs.Results, 1)

			assert.NotNil(t, items[1].OfWebSearchCall)
		})

		mockey.PatchConvey("unpaired_error", func() {
			call := responses.ResponseInputItemUnionParam{
				OfFileSearchCall: &responses.ResponseFileSearchToolCallParam{
					ID: "fs1",
				},
			}
			_, err := pairFileSearchToolCallItems([]responses.ResponseInputItemUnionParam{call})
			assert.Error(t, err)
		})
	})
}

func TestPairCodeInterpreterToolCallItems(t *testing.T) {
	mockey.PatchConvey("pairCodeInterpreterToolCallItems", t, func() {
		mockey.PatchConvey("merge_call_and_result", func() {
			call := responses.ResponseInputItemUnionParam{
				OfCodeInterpreterCall: &responses.ResponseCodeInterpreterToolCallParam{
					ID: "ci1",
				},
			}
			result := responses.ResponseInputItemUnionParam{
				OfCodeInterpreterCall: &responses.ResponseCodeInterpreterToolCallParam{
					ID:          "ci1",
					Status:      "completed",
					ContainerID: "ctr1",
					Code:        param.NewOpt("print('hi')"),
					Outputs: []responses.ResponseCodeInterpreterToolCallOutputUnionParam{
						{OfLogs: &responses.ResponseCodeInterpreterToolCallOutputLogsParam{Logs: "hi"}},
					},
				},
			}
			other := responses.ResponseInputItemUnionParam{
				OfWebSearchCall: &responses.ResponseFunctionWebSearchParam{ID: "ws1"},
			}

			items, err := pairCodeInterpreterToolCallItems([]responses.ResponseInputItemUnionParam{call, other, result})
			assert.NoError(t, err)
			assert.Len(t, items, 2)

			ci := items[0].OfCodeInterpreterCall
			assert.NotNil(t, ci)
			assert.Equal(t, "ci1", ci.ID)
			assert.Equal(t, "ctr1", ci.ContainerID)
			assert.True(t, ci.Code.Valid())
			assert.Equal(t, "print('hi')", ci.Code.Value)
			assert.Len(t, ci.Outputs, 1)
			assert.Equal(t, responses.ResponseCodeInterpreterToolCallStatus("completed"), ci.Status)

			assert.NotNil(t, items[1].OfWebSearchCall)
		})

		mockey.PatchConvey("unpaired_error", func() {
			call := responses.ResponseInputItemUnionParam{
				OfCodeInterpreterCall: &responses.ResponseCodeInterpreterToolCallParam{
					ID: "ci1",
				},
			}
			_, err := pairCodeInterpreterToolCallItems([]responses.ResponseInputItemUnionParam{call})
			assert.Error(t, err)
		})
	})
}

func TestPairImageGenerationToolCallItems(t *testing.T) {
	mockey.PatchConvey("pairImageGenerationToolCallItems", t, func() {
		mockey.PatchConvey("merge_call_and_result", func() {
			call := responses.ResponseInputItemUnionParam{
				OfImageGenerationCall: &responses.ResponseInputItemImageGenerationCallParam{
					ID: "ig1",
				},
			}
			result := responses.ResponseInputItemUnionParam{
				OfImageGenerationCall: &responses.ResponseInputItemImageGenerationCallParam{
					ID:     "ig1",
					Status: "completed",
					Result: param.NewOpt("base64data"),
				},
			}
			other := responses.ResponseInputItemUnionParam{
				OfWebSearchCall: &responses.ResponseFunctionWebSearchParam{ID: "ws1"},
			}

			items, err := pairImageGenerationToolCallItems([]responses.ResponseInputItemUnionParam{call, other, result})
			assert.NoError(t, err)
			assert.Len(t, items, 2)

			ig := items[0].OfImageGenerationCall
			assert.NotNil(t, ig)
			assert.Equal(t, "ig1", ig.ID)
			assert.Equal(t, "completed", ig.Status)
			assert.True(t, ig.Result.Valid())
			assert.Equal(t, "base64data", ig.Result.Value)

			assert.NotNil(t, items[1].OfWebSearchCall)
		})

		mockey.PatchConvey("unpaired_error", func() {
			call := responses.ResponseInputItemUnionParam{
				OfImageGenerationCall: &responses.ResponseInputItemImageGenerationCallParam{
					ID: "ig1",
				},
			}
			_, err := pairImageGenerationToolCallItems([]responses.ResponseInputItemUnionParam{call})
			assert.Error(t, err)
		})
	})
}

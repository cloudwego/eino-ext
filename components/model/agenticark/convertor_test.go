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

	"github.com/bytedance/mockey"
	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model/responses"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestToSystemRoleInputItems(t *testing.T) {
	msg := &schema.AgenticMessage{
		ContentBlocks: []*schema.ContentBlock{
			schema.NewContentBlock(&schema.UserInputText{Text: "hello"}),
			schema.NewContentBlock(&schema.UserInputImage{
				URL:      "http://example.com/image.png",
				MIMEType: "image/png",
				Detail:   schema.ImageURLDetailHigh,
			}),
		},
	}

	items, err := toSystemRoleInputItems(msg)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(items))
	assert.Equal(t, responses.MessageRole_system, items[0].GetInputMessage().Role)

	msgInvalid := &schema.AgenticMessage{
		ContentBlocks: []*schema.ContentBlock{
			{Type: "invalid"},
		},
	}
	_, err = toSystemRoleInputItems(msgInvalid)
	assert.Error(t, err)
}

func TestToAssistantRoleInputItems(t *testing.T) {
	msg := &schema.AgenticMessage{
		ContentBlocks: []*schema.ContentBlock{
			schema.NewContentBlock(&schema.AssistantGenText{Text: "answer"}),
			schema.NewContentBlock(&schema.Reasoning{
				Text: "reason",
			}),
		},
	}
	setItemID(msg.ContentBlocks[1], "id-1")
	setItemStatus(msg.ContentBlocks[1], responses.ItemStatus_completed.String())

	items, err := toAssistantRoleInputItems(msg)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(items))
	assert.Equal(t, responses.MessageRole_assistant, items[0].GetInputMessage().Role)
	assert.NotNil(t, items[1].GetReasoning())
}

func TestPairMCPToolCallItems(t *testing.T) {
	id := "call-1"
	out := "result"
	errStr := "err"

	call := &responses.InputItem{
		Union: &responses.InputItem_FunctionMcpCall{
			FunctionMcpCall: &responses.ItemFunctionMcpCall{
				Type:        responses.ItemType_mcp_call,
				Id:          &id,
				ServerLabel: "server",
				Name:        "tool",
			},
		},
	}
	result := &responses.InputItem{
		Union: &responses.InputItem_FunctionMcpCall{
			FunctionMcpCall: &responses.ItemFunctionMcpCall{
				Type:        responses.ItemType_mcp_call,
				Id:          &id,
				ServerLabel: "server",
				Name:        "tool",
				Output:      &out,
				Error:       &errStr,
			},
		},
	}

	items, err := pairMCPToolCallItems([]*responses.InputItem{call, result})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(items))
	mcp := items[0].GetFunctionMcpCall()
	assert.NotNil(t, mcp)
	assert.Equal(t, out, mcp.GetOutput())
	assert.Equal(t, errStr, mcp.GetError())

	onlyCall := []*responses.InputItem{call}
	_, err = pairMCPToolCallItems(onlyCall)
	assert.Error(t, err)
}

func TestToUserRoleInputItems(t *testing.T) {
	msg := &schema.AgenticMessage{
		ContentBlocks: []*schema.ContentBlock{
			schema.NewContentBlock(&schema.UserInputText{Text: "u"}),
			schema.NewContentBlock(&schema.UserInputVideo{
				URL:      "http://example.com/video.mp4",
				MIMEType: "video/mp4",
			}),
			schema.NewContentBlock(&schema.FunctionToolResult{
				CallID: "c1",
				Name:   "n1",
				Result: "r1",
			}),
			schema.NewContentBlock(&schema.MCPToolApprovalResponse{
				ApprovalRequestID: "ar1",
				Approve:           true,
			}),
		},
	}

	items, err := toUserRoleInputItems(msg)
	assert.NoError(t, err)
	assert.Equal(t, 4, len(items))
	assert.Equal(t, responses.MessageRole_user, items[0].GetInputMessage().Role)
	assert.NotNil(t, items[1].GetInputMessage().Content[0].GetVideo())
	assert.NotNil(t, items[2].GetFunctionToolCallOutput())
	assert.NotNil(t, items[3].GetMcpApprovalResponse())
}

func TestUserInputTextToInputItem(t *testing.T) {
	block := &schema.UserInputText{Text: "hello"}
	item, err := userInputTextToInputItem(responses.MessageRole_user, block)
	assert.NoError(t, err)
	assert.Equal(t, "hello", item.GetInputMessage().Content[0].GetText().Text)
}

func TestUserInputImageToInputItem(t *testing.T) {
	block := &schema.UserInputImage{
		URL:      "http://example.com/image.png",
		MIMEType: "image/png",
		Detail:   schema.ImageURLDetailLow,
	}
	item, err := userInputImageToInputItem(responses.MessageRole_user, block)
	assert.NoError(t, err)
	img := item.GetInputMessage().Content[0].GetImage()
	assert.NotNil(t, img)
	assert.NotNil(t, img.ImageUrl)
	assert.Equal(t, block.URL, *img.ImageUrl)

	blockInvalid := &schema.UserInputImage{
		Base64Data: "xxx",
		MIMEType:   "",
		Detail:     "invalid",
	}
	_, err = userInputImageToInputItem(responses.MessageRole_user, blockInvalid)
	assert.Error(t, err)
}

func TestToContentItemImageDetail(t *testing.T) {
	tests := []struct {
		in    schema.ImageURLDetail
		valid bool
	}{
		{schema.ImageURLDetailHigh, true},
		{schema.ImageURLDetailLow, true},
		{schema.ImageURLDetailAuto, true},
		{"invalid", false},
	}
	for _, tt := range tests {
		detail, err := toContentItemImageDetail(tt.in)
		if tt.valid {
			assert.NoError(t, err)
			assert.NotNil(t, detail)
		} else {
			assert.Error(t, err)
		}
	}
}

func TestUserInputVideoToInputItem(t *testing.T) {
	video := &schema.UserInputVideo{
		URL:      "http://example.com/video.mp4",
		MIMEType: "video/mp4",
	}
	item, err := userInputVideoToInputItem(responses.MessageRole_user, video)
	assert.NoError(t, err)
	assert.Equal(t, video.URL, item.GetInputMessage().Content[0].GetVideo().VideoUrl)
}

func TestUserInputFileToInputItem(t *testing.T) {
	tests := []struct {
		name   string
		block  *schema.UserInputFile
		hasURL bool
	}{
		{
			name: "with_url",
			block: &schema.UserInputFile{
				Name: "file.txt",
				URL:  "http://example.com/file.txt",
			},
			hasURL: true,
		},
		{
			name: "with_base64",
			block: &schema.UserInputFile{
				Name:       "file.bin",
				Base64Data: "ZGF0YQ==",
			},
			hasURL: false,
		},
	}

	for _, tt := range tests {
		item, err := userInputFileToInputItem(responses.MessageRole_user, tt.block)
		assert.NoError(t, err)
		msg := item.GetInputMessage()
		assert.Equal(t, responses.MessageRole_user, msg.Role)
		assert.Len(t, msg.Content, 1)
		file := msg.Content[0].GetFile()
		assert.NotNil(t, file)
		assert.Equal(t, responses.ContentItemType_input_file, file.Type)
		assert.NotNil(t, file.Filename)
		assert.Equal(t, tt.block.Name, *file.Filename)
		if tt.hasURL {
			assert.NotNil(t, file.FileUrl)
			assert.Equal(t, tt.block.URL, *file.FileUrl)
			assert.Nil(t, file.FileData)
		} else {
			assert.NotNil(t, file.FileData)
			assert.Equal(t, tt.block.Base64Data, *file.FileData)
			assert.Nil(t, file.FileUrl)
		}
	}
}

func TestFunctionToolResultToInputItem(t *testing.T) {
	block := &schema.FunctionToolResult{
		CallID: "c1",
		Name:   "n1",
		Result: "r1",
	}
	item, err := functionToolResultToInputItem(block)
	assert.NoError(t, err)
	out := item.GetFunctionToolCallOutput()
	assert.NotNil(t, out)
	assert.Equal(t, "c1", out.CallId)
	assert.Equal(t, "r1", out.Output)
}

func TestAssistantGenTextToInputItem(t *testing.T) {
	block := schema.NewContentBlock(&schema.AssistantGenText{
		Text: "answer"},
	)
	item, err := assistantGenTextToInputItem(block)
	assert.NoError(t, err)
	msg := item.GetInputMessage()
	assert.Equal(t, responses.MessageRole_assistant, msg.Role)
	assert.Equal(t, "answer", msg.Content[0].GetText().Text)
}

func TestFunctionToolCallToInputItem(t *testing.T) {
	block := &schema.ContentBlock{
		FunctionToolCall: &schema.FunctionToolCall{
			CallID:    "cid",
			Name:      "name",
			Arguments: "{}",
		},
	}
	item, err := functionToolCallToInputItem(block)
	assert.NoError(t, err)
	call := item.GetFunctionToolCall()
	assert.NotNil(t, call)
	assert.Equal(t, "cid", call.CallId)
	assert.Equal(t, "name", call.Name)
}

func TestReasoningToInputItem(t *testing.T) {
	block := schema.NewContentBlock(&schema.Reasoning{
		Text: "r",
	})

	item, err := reasoningToInputItem(block)
	assert.NoError(t, err)
	reason := item.GetReasoning()
	assert.NotNil(t, reason)
	assert.Equal(t, 1, len(reason.Summary))
	assert.Equal(t, "r", reason.Summary[0].Text)
}

func TestServerToolCallToInputItem(t *testing.T) {
	mockey.PatchConvey("TestServerToolCallToInputItem", t, func() {
		block := schema.NewContentBlock(&schema.ServerToolCall{
			Name: string(ServerToolNameWebSearch),
			Arguments: &ServerToolCallArguments{
				WebSearch: &WebSearchArguments{
					ActionType: WebSearchActionSearch,
					Search:     &WebSearchQuery{Query: "q"},
				},
			},
		})

		item, err := serverToolCallToInputItem(block)
		assert.NoError(t, err)
		ws := item.GetFunctionWebSearchCall()
		assert.NotNil(t, ws)
		assert.NotNil(t, ws.Action)
		assert.Equal(t, "q", ws.Action.Query)

		mockey.Mock(getServerToolCallArguments).Return(nil, errors.New("mock")).Build()
		_, err = serverToolCallToInputItem(block)
		assert.Error(t, err)
	})
}

func TestMcpToolApprovalRequestToInputItem(t *testing.T) {
	req := schema.NewContentBlock(&schema.MCPToolApprovalRequest{
		ID:          "id",
		ServerLabel: "server",
		Name:        "name",
		Arguments:   "{}",
	})

	item, err := mcpToolApprovalRequestToInputItem(req)
	assert.NoError(t, err)
	ap := item.GetMcpApprovalRequest()
	assert.NotNil(t, ap)
	assert.NotEmpty(t, ap.GetId())
	assert.Equal(t, "server", ap.ServerLabel)
}

func TestMcpToolApprovalResponseToInputItem(t *testing.T) {
	resp := &schema.MCPToolApprovalResponse{
		ApprovalRequestID: "rid",
		Approve:           true,
		Reason:            "ok",
	}
	item, err := mcpToolApprovalResponseToInputItem(resp)
	assert.NoError(t, err)
	ap := item.GetMcpApprovalResponse()
	assert.NotNil(t, ap)
	assert.True(t, ap.Approve)
	assert.Equal(t, "rid", ap.ApprovalRequestId)
}

func TestMcpListToolsResultToInputItem(t *testing.T) {
	sc := &jsonschema.Schema{
		Title:       "t",
		Description: "d",
	}

	content := schema.NewContentBlock(&schema.MCPListToolsResult{
		ServerLabel: "server",
		Tools: []*schema.MCPListToolsItem{
			{
				Name:        "tool",
				Description: "desc",
				InputSchema: sc,
			},
		},
		Error: "err",
	})

	item, err := mcpListToolsResultToInputItem(content)
	assert.NoError(t, err)
	list := item.GetMcpListTools()
	assert.NotNil(t, list)
	assert.Equal(t, 1, len(list.Tools))
	assert.Equal(t, "tool", list.Tools[0].Name)
}

func TestMcpToolCallToInputItem(t *testing.T) {
	call := schema.NewContentBlock(&schema.MCPToolCall{
		ServerLabel:       "server",
		Name:              "name",
		Arguments:         "{}",
		ApprovalRequestID: "ar",
	})

	item, err := mcpToolCallToInputItem(call)
	assert.NoError(t, err)
	mcp := item.GetFunctionMcpCall()
	assert.NotNil(t, mcp)
	assert.Equal(t, "server", mcp.ServerLabel)
	assert.Equal(t, "ar", mcp.GetApprovalRequestId())
}

func TestMcpToolResultToInputItem(t *testing.T) {
	res := schema.NewContentBlock(&schema.MCPToolResult{
		ServerLabel: "server",
		Name:        "name",
		Result:      "r",
		Error:       &schema.MCPToolCallError{Message: "e"},
	})

	item, err := mcpToolResultToInputItem(res)
	assert.NoError(t, err)
	mcp := item.GetFunctionMcpCall()
	assert.NotNil(t, mcp)
	assert.Equal(t, "server", mcp.ServerLabel)
	assert.Equal(t, "r", mcp.GetOutput())
}

func TestToOutputMessage(t *testing.T) {
	outputText := &responses.OutputContentItemText{
		Text: "answer",
	}
	outMsg := &responses.OutputItem{
		Union: &responses.OutputItem_OutputMessage{
			OutputMessage: &responses.ItemOutputMessage{
				Content: []*responses.OutputContentItem{
					{Union: &responses.OutputContentItem_Text{Text: outputText}},
				},
			},
		},
	}

	id := "mid"
	mcpCall := &responses.OutputItem{
		Union: &responses.OutputItem_FunctionMcpCall{
			FunctionMcpCall: &responses.ItemFunctionMcpCall{
				Type:        responses.ItemType_mcp_call,
				Id:          &id,
				ServerLabel: "server",
				Name:        "tool",
				Output:      ptrOf("out"),
			},
		},
	}

	resp := &responses.ResponseObject{
		Output: []*responses.OutputItem{outMsg, mcpCall},
	}

	msg, err := toOutputMessage(resp)
	assert.NoError(t, err)
	assert.Equal(t, schema.AgenticRoleTypeAssistant, msg.Role)
	assert.Greater(t, len(msg.ContentBlocks), 0)
	assert.NotNil(t, msg.ContentBlocks[0].AssistantGenText)
	assert.Equal(t, "answer", msg.ContentBlocks[0].AssistantGenText.Text)
}

func TestOutputMessageToContentBlocks(t *testing.T) {
	out := &responses.ItemOutputMessage{
		Id:     "id",
		Status: responses.ItemStatus_completed,
		Content: []*responses.OutputContentItem{
			{
				Union: &responses.OutputContentItem_Text{
					Text: &responses.OutputContentItemText{Text: "a"},
				},
			},
		},
	}
	blocks, err := outputMessageToContentBlocks(&responses.OutputItem_OutputMessage{OutputMessage: out})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(blocks))
	assert.NotNil(t, blocks[0].AssistantGenText)

	_, err = outputMessageToContentBlocks(&responses.OutputItem_OutputMessage{})
	assert.Error(t, err)
}

func TestOutputContentTextToContentBlock(t *testing.T) {
	title := "t"
	url := "u"
	anno := &responses.Annotation{
		Type:  responses.AnnotationType_url_citation,
		Title: title,
		Url:   url,
	}
	block, err := outputContentTextToContentBlock(&responses.OutputContentItemText{
		Text:        "a",
		Annotations: []*responses.Annotation{anno},
	})
	assert.NoError(t, err)
	assert.NotNil(t, block.AssistantGenText)
	assert.Equal(t, "a", block.AssistantGenText.Text)
}

func TestOutputTextAnnotationToTextAnnotation(t *testing.T) {
	docID := "d"
	docName := "n"
	a := &responses.Annotation{
		Type:    responses.AnnotationType_doc_citation,
		DocId:   &docID,
		DocName: &docName,
		ChunkId: ptrOf[int32](1),
		ChunkAttachment: []*structpb.Struct{
			structpb.NewStructValue(&structpb.Struct{}).GetStructValue(),
		},
	}
	ta, err := outputTextAnnotationToTextAnnotation(a)
	assert.NoError(t, err)
	assert.NotNil(t, ta)
	assert.NotNil(t, ta.DocCitation)
	assert.Equal(t, "d", ta.DocCitation.DocID)

	invalid := &responses.Annotation{
		Type: responses.AnnotationType_unspecified,
	}
	_, err = outputTextAnnotationToTextAnnotation(invalid)
	assert.Error(t, err)
}

func TestFunctionToolCallToContentBlock(t *testing.T) {
	id := "id"
	item := &responses.OutputItem_FunctionToolCall{
		FunctionToolCall: &responses.ItemFunctionToolCall{
			CallId: "cid",
			Name:   "name",
			Status: responses.ItemStatus_completed.Enum(),
			Id:     &id,
		},
	}
	block, err := functionToolCallToContentBlock(item)
	assert.NoError(t, err)
	assert.NotNil(t, block.FunctionToolCall)
	assert.Equal(t, "cid", block.FunctionToolCall.CallID)

	_, err = functionToolCallToContentBlock(&responses.OutputItem_FunctionToolCall{})
	assert.Error(t, err)
}

func TestWebSearchToContentBlock(t *testing.T) {
	item := &responses.OutputItem_FunctionWebSearch{
		FunctionWebSearch: &responses.ItemFunctionWebSearch{
			Id:     "id",
			Status: responses.ItemStatus_completed,
			Action: &responses.Action{
				Type:  responses.ActionType_search,
				Query: "q",
			},
		},
	}
	block, err := webSearchToContentBlock(item)
	assert.NoError(t, err)
	assert.NotNil(t, block.ServerToolCall)
	args := block.ServerToolCall.Arguments.(*ServerToolCallArguments)
	assert.NotNil(t, args.WebSearch)
	assert.Equal(t, "q", args.WebSearch.Search.Query)

	itemInvalid := &responses.OutputItem_FunctionWebSearch{
		FunctionWebSearch: &responses.ItemFunctionWebSearch{
			Action: &responses.Action{
				Type: responses.ActionType_unspecified,
			},
		},
	}
	_, err = webSearchToContentBlock(itemInvalid)
	assert.Error(t, err)
}

func TestImageProcessToContentBlocks(t *testing.T) {
	item := &responses.OutputItem_FunctionImageProcess{
		FunctionImageProcess: &responses.ItemFunctionImageProcess{
			Id:     "ip-id",
			Status: responses.ItemStatus_completed,
			Arguments: &responses.ResponseImageProcessArgs{
				Union: &responses.ResponseImageProcessArgs_PointArgs{
					PointArgs: &responses.ResponseImageProcessPointArgs{
						ImageIndex: 0,
						Points:     "[[100,200]]",
						DrawLine:   true,
					},
				},
			},
			Action: &responses.ResponseImageProcessAction{
				Type:           string(ImageProcessActionPoint),
				ResultImageUrl: ptrOf("http://example.com/result.png"),
			},
		},
	}
	blocks, err := imageProcessToContentBlocks(item)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(blocks))

	assert.NotNil(t, blocks[0].ServerToolCall)
	assert.Equal(t, string(ServerToolNameImageProcess), blocks[0].ServerToolCall.Name)
	args := blocks[0].ServerToolCall.Arguments.(*ServerToolCallArguments)
	assert.NotNil(t, args.ImageProcess)
	assert.Equal(t, ImageProcessActionPoint, args.ImageProcess.ActionType)
	assert.NotNil(t, args.ImageProcess.Point)
	assert.Equal(t, "[[100,200]]", args.ImageProcess.Point.Points)
	assert.True(t, args.ImageProcess.Point.DrawLine)

	assert.NotNil(t, blocks[1].ServerToolResult)
	assert.Equal(t, string(ServerToolNameImageProcess), blocks[1].ServerToolResult.Name)
	result := blocks[1].ServerToolResult.Result.(*ServerToolResult)
	assert.NotNil(t, result.ImageProcess)
	assert.Equal(t, ImageProcessActionPoint, result.ImageProcess.Action.Type)
	assert.Equal(t, "http://example.com/result.png", result.ImageProcess.Action.ResultImageURL)

	_, err = imageProcessToContentBlocks(&responses.OutputItem_FunctionImageProcess{})
	assert.Error(t, err)
}

func TestImageProcessToContentBlocks_AllActionTypes(t *testing.T) {
	tests := []struct {
		name       string
		actionType ImageProcessAction
		args       *responses.ResponseImageProcessArgs
	}{
		{
			name:       "grounding",
			actionType: ImageProcessActionGrounding,
			args: &responses.ResponseImageProcessArgs{
				Union: &responses.ResponseImageProcessArgs_GroundingArgs{
					GroundingArgs: &responses.ResponseImageProcessGroundingArgs{
						ImageIndex: 0,
						BboxStr:    "[[10,20,30,40]]",
						Crop:       true,
					},
				},
			},
		},
		{
			name:       "rotate",
			actionType: ImageProcessActionRotate,
			args: &responses.ResponseImageProcessArgs{
				Union: &responses.ResponseImageProcessArgs_RotateArgs{
					RotateArgs: &responses.ResponseImageProcessRotateArgs{
						ImageIndex: 0,
						Degree:     90,
					},
				},
			},
		},
		{
			name:       "zoom",
			actionType: ImageProcessActionZoom,
			args: &responses.ResponseImageProcessArgs{
				Union: &responses.ResponseImageProcessArgs_ZoomArgs{
					ZoomArgs: &responses.ResponseImageProcessZoomArgs{
						ImageIndex: 0,
						BboxStr:    "[[0,0,100,100]]",
						Scale:      1.5,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := &responses.OutputItem_FunctionImageProcess{
				FunctionImageProcess: &responses.ItemFunctionImageProcess{
					Id:        "ip-id",
					Status:    responses.ItemStatus_completed,
					Arguments: tt.args,
					Action: &responses.ResponseImageProcessAction{
						Type:           string(tt.actionType),
						ResultImageUrl: ptrOf("http://example.com/result.png"),
					},
				},
			}
			blocks, err := imageProcessToContentBlocks(item)
			assert.NoError(t, err)
			assert.Equal(t, 2, len(blocks))

			args := blocks[0].ServerToolCall.Arguments.(*ServerToolCallArguments)
			assert.Equal(t, tt.actionType, args.ImageProcess.ActionType)
		})
	}
}

func TestImageProcessToContentBlocks_WithError(t *testing.T) {
	item := &responses.OutputItem_FunctionImageProcess{
		FunctionImageProcess: &responses.ItemFunctionImageProcess{
			Id:     "ip-id",
			Status: responses.ItemStatus_failed,
			Arguments: &responses.ResponseImageProcessArgs{
				Union: &responses.ResponseImageProcessArgs_PointArgs{
					PointArgs: &responses.ResponseImageProcessPointArgs{
						ImageIndex: 0,
					},
				},
			},
			Action: &responses.ResponseImageProcessAction{
				Type: string(ImageProcessActionPoint),
			},
			Error: &responses.ResponseImageProcessError{
				Message: "processing failed",
			},
		},
	}
	blocks, err := imageProcessToContentBlocks(item)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(blocks))

	result := blocks[1].ServerToolResult.Result.(*ServerToolResult)
	assert.NotNil(t, result.ImageProcess.Error)
	assert.Equal(t, "processing failed", result.ImageProcess.Error.Message)
}

func TestImageProcessArgumentsToInputItem(t *testing.T) {
	tests := []struct {
		name   string
		args   *ImageProcessArguments
		errMsg string
	}{
		{
			name: "point",
			args: &ImageProcessArguments{
				ActionType: ImageProcessActionPoint,
				Point: &ImageProcessPoint{
					ImageIndex: 0,
					Points:     "[[100,200]]",
					DrawLine:   true,
				},
			},
		},
		{
			name: "grounding",
			args: &ImageProcessArguments{
				ActionType: ImageProcessActionGrounding,
				Grounding: &ImageProcessGrounding{
					ImageIndex: 0,
					BboxStr:    "[[10,20,30,40]]",
					Crop:       true,
				},
			},
		},
		{
			name: "rotate",
			args: &ImageProcessArguments{
				ActionType: ImageProcessActionRotate,
				Rotate: &ImageProcessRotate{
					ImageIndex: 0,
					Degree:     90,
				},
			},
		},
		{
			name: "zoom",
			args: &ImageProcessArguments{
				ActionType: ImageProcessActionZoom,
				Zoom: &ImageProcessZoom{
					ImageIndex: 0,
					BboxStr:    "[[0,0,100,100]]",
					Scale:      1.5,
				},
			},
		},
		{
			name:   "nil_args",
			args:   nil,
			errMsg: "image process arguments is nil",
		},
		{
			name: "nil_point",
			args: &ImageProcessArguments{
				ActionType: ImageProcessActionPoint,
				Point:      nil,
			},
			errMsg: "point arguments is nil",
		},
		{
			name: "nil_grounding",
			args: &ImageProcessArguments{
				ActionType: ImageProcessActionGrounding,
				Grounding:  nil,
			},
			errMsg: "grounding arguments is nil",
		},
		{
			name: "nil_rotate",
			args: &ImageProcessArguments{
				ActionType: ImageProcessActionRotate,
				Rotate:     nil,
			},
			errMsg: "rotate arguments is nil",
		},
		{
			name: "nil_zoom",
			args: &ImageProcessArguments{
				ActionType: ImageProcessActionZoom,
				Zoom:       nil,
			},
			errMsg: "zoom arguments is nil",
		},
		{
			name: "unknown_action",
			args: &ImageProcessArguments{
				ActionType: "unknown",
			},
			errMsg: "unknown image process action type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item, err := imageProcessArgumentsToInputItem("id", "completed", tt.args)
			if tt.errMsg != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, item)
				ip := item.GetImageProcess()
				assert.NotNil(t, ip)
				assert.Equal(t, "id", ip.Id)
			}
		})
	}
}

func TestReasoningToContentBlocks(t *testing.T) {
	id := "id"
	item := &responses.OutputItem_Reasoning{
		Reasoning: &responses.ItemReasoning{
			Id:     &id,
			Status: responses.ItemStatus_completed,
			Summary: []*responses.ReasoningSummaryPart{
				{Text: "r"},
			},
		},
	}
	block, err := reasoningToContentBlocks(item)
	assert.NoError(t, err)
	assert.NotNil(t, block.Reasoning)
	assert.Equal(t, "r", block.Reasoning.Text)

	_, err = reasoningToContentBlocks(&responses.OutputItem_Reasoning{})
	assert.Error(t, err)
}

func TestMcpCallToContentBlocks(t *testing.T) {
	id := "id"
	item := &responses.OutputItem_FunctionMcpCall{
		FunctionMcpCall: &responses.ItemFunctionMcpCall{
			Id:          &id,
			ServerLabel: "server",
			Name:        "tool",
			Arguments:   "{}",
			Output:      ptrOf("out"),
		},
	}
	blocks, err := mcpCallToContentBlocks(item)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(blocks))
	assert.NotNil(t, blocks[0].MCPToolCall)
	assert.NotNil(t, blocks[1].MCPToolResult)

	_, err = mcpCallToContentBlocks(&responses.OutputItem_FunctionMcpCall{})
	assert.Error(t, err)
}

func TestMcpListToolsToContentBlock(t *testing.T) {
	toolSchema, err := structpb.NewStruct(map[string]any{"type": "object"})
	assert.NoError(t, err)
	id := "id"
	item := &responses.OutputItem_FunctionMcpListTools{
		FunctionMcpListTools: &responses.ItemFunctionMcpListTools{
			Id:          &id,
			ServerLabel: "server",
			Tools: []*responses.McpTool{
				{
					Name:        "tool",
					Description: "desc",
					InputSchema: toolSchema,
				},
			},
		},
	}
	block, err := mcpListToolsToContentBlock(item)
	assert.NoError(t, err)
	assert.NotNil(t, block.MCPListToolsResult)
	assert.Equal(t, 1, len(block.MCPListToolsResult.Tools))

	_, err = mcpListToolsToContentBlock(&responses.OutputItem_FunctionMcpListTools{})
	assert.Error(t, err)
}

func TestMcpApprovalRequestToContentBlock(t *testing.T) {
	item := &responses.OutputItem_FunctionMcpApprovalRequest{
		FunctionMcpApprovalRequest: &responses.ItemFunctionMcpApprovalRequest{
			Id:          ptrOf("id"),
			ServerLabel: "server",
			Name:        "tool",
			Arguments:   "{}",
		},
	}
	block, err := mcpApprovalRequestToContentBlock(item)
	assert.NoError(t, err)
	assert.NotNil(t, block.MCPToolApprovalRequest)
	assert.NotEmpty(t, block.MCPToolApprovalRequest.ID)

	_, err = mcpApprovalRequestToContentBlock(&responses.OutputItem_FunctionMcpApprovalRequest{})
	assert.Error(t, err)
}

func TestResponseObjectToResponseMeta(t *testing.T) {
	resp := &responses.ResponseObject{
		Id: "id",
	}
	meta := responseObjectToResponseMeta(resp)
	assert.NotNil(t, meta)
	assert.NotNil(t, meta.Extension)
}

func TestToTokenUsage(t *testing.T) {
	assert.Nil(t, toTokenUsage(&responses.ResponseObject{}))
}

func TestToResponseMetaExtension(t *testing.T) {
	resp := &responses.ResponseObject{
		Id: "id",
		IncompleteDetails: &responses.IncompleteDetails{
			Reason: "r",
			ContentFilter: &responses.ContentFilter{
				Type:    "t",
				Details: "d",
			},
		},
		Error: &responses.Error{
			Code:    "c",
			Message: "m",
		},
		Thinking: &responses.ResponsesThinking{
			Type: responses.ThinkingType_enabled.Enum(),
		},
		ServiceTier: responses.ResponsesServiceTier_default.Enum(),
		Status:      responses.ResponseStatus_completed,
	}
	ext := toResponseMetaExtension(resp)
	assert.NotNil(t, ext)
	assert.Equal(t, "id", ext.ID)
	assert.NotNil(t, ext.IncompleteDetails)
	assert.NotNil(t, ext.Error)
	assert.NotNil(t, ext.Thinking)
	assert.Nil(t, toResponseMetaExtension(nil))
}

func TestResolveURL(t *testing.T) {
	u, err := resolveURL("http://example.com/image.png", "", "")
	assert.NoError(t, err)
	assert.Equal(t, "http://example.com/image.png", u)

	u, err = resolveURL("", "abcd", "image/png")
	assert.NoError(t, err)
	assert.NotEmpty(t, u)

	_, err = resolveURL("", "abcd", "")
	assert.Error(t, err)
}

func TestEnsureDataURL(t *testing.T) {
	_, err := ensureDataURL("data:xxx", "image/png")
	assert.Error(t, err)
	u, err := ensureDataURL("abcd", "image/png")
	assert.NoError(t, err)
	assert.Equal(t, "data:image/png;base64,abcd", u)
	_, err = ensureDataURL("abcd", "")
	assert.Error(t, err)
}

func TestDoubaoAppCallToContentBlocks(t *testing.T) {
	id := "id"
	feature := "ai_search"
	item := &responses.OutputItem_FunctionDoubaoAppCall{
		FunctionDoubaoAppCall: &responses.ItemDoubaoAppCall{
			Id:      &id,
			Status:  responses.ItemStatus_completed,
			Feature: &feature,
			Blocks: []*responses.DoubaoAppCallBlock{
				{
					Union: &responses.DoubaoAppCallBlock_OutputText{
						OutputText: &responses.DoubaoAppCallBlockOutputText{
							Id:   ptrOf("text_id"),
							Text: "output text",
						},
					},
				},
				{
					Union: &responses.DoubaoAppCallBlock_ReasoningText{
						ReasoningText: &responses.DoubaoAppCallBlockReasoningText{
							Id:            ptrOf("reasoning_id"),
							ReasoningText: "reasoning text",
						},
					},
				},
				{
					Union: &responses.DoubaoAppCallBlock_Search{
						Search: &responses.DoubaoAppCallBlockSearch{
							Id:      ptrOf("search_id"),
							Summary: ptrOf("search summary"),
							Queries: []string{"query1", "query2"},
							Results: []*responses.DoubaoAppSearchResult{
								{
									TextCard: &responses.DoubaoAppSearchTextItem{
										Title:    "title",
										Url:      "url",
										Sitename: "site",
									},
								},
							},
						},
					},
				},
				{
					Union: &responses.DoubaoAppCallBlock_ReasoningSearch{
						ReasoningSearch: &responses.DoubaoAppCallBlockReasoningSearch{
							Id:      ptrOf("reasoning_search_id"),
							Summary: ptrOf("reasoning search summary"),
							Queries: []string{"rq1"},
						},
					},
				},
				nil,
			},
		},
	}

	blocks, err := doubaoAppCallToContentBlocks(item)
	assert.NoError(t, err)
	assert.Len(t, blocks, 2)

	assert.NotNil(t, blocks[0].ServerToolCall)
	assert.Equal(t, string(ServerToolNameDoubaoApp), blocks[0].ServerToolCall.Name)
	args := blocks[0].ServerToolCall.Arguments.(*ServerToolCallArguments)
	assert.NotNil(t, args.DoubaoApp)
	assert.Equal(t, DoubaoAppFeature("ai_search"), args.DoubaoApp.Feature)

	assert.NotNil(t, blocks[1].ServerToolResult)
	result := blocks[1].ServerToolResult.Result.(*ServerToolResult)
	assert.NotNil(t, result.DoubaoApp)
	assert.Len(t, result.DoubaoApp.Blocks, 4)

	assert.Equal(t, DoubaoAppBlockTypeOutputText, result.DoubaoApp.Blocks[0].Type)
	assert.Equal(t, "output text", result.DoubaoApp.Blocks[0].OutputText.Text)

	assert.Equal(t, DoubaoAppBlockTypeReasoningText, result.DoubaoApp.Blocks[1].Type)
	assert.Equal(t, "reasoning text", result.DoubaoApp.Blocks[1].ReasoningText.ReasoningText)

	assert.Equal(t, DoubaoAppBlockTypeSearch, result.DoubaoApp.Blocks[2].Type)
	assert.Equal(t, "search summary", result.DoubaoApp.Blocks[2].Search.Summary)
	assert.Len(t, result.DoubaoApp.Blocks[2].Search.Results, 1)
	assert.Equal(t, "title", result.DoubaoApp.Blocks[2].Search.Results[0].Title)

	assert.Equal(t, DoubaoAppBlockTypeReasoningSearch, result.DoubaoApp.Blocks[3].Type)
	assert.Equal(t, "reasoning search summary", result.DoubaoApp.Blocks[3].ReasoningSearch.Summary)

	_, err = doubaoAppCallToContentBlocks(&responses.OutputItem_FunctionDoubaoAppCall{})
	assert.Error(t, err)
}

func TestKnowledgeSearchCallToContentBlocks(t *testing.T) {
	id := "id"
	item := &responses.OutputItem_FunctionKnowledgeSearch{
		FunctionKnowledgeSearch: &responses.ItemFunctionKnowledgeSearch{
			Id:                  &id,
			Status:              responses.ItemStatus_completed,
			KnowledgeResourceId: "resource_id",
			Queries:             []string{"q1", "q2"},
		},
	}

	blocks, err := knowledgeSearchCallToContentBlocks(item)
	assert.NoError(t, err)
	assert.Len(t, blocks, 1)

	assert.NotNil(t, blocks[0].ServerToolCall)
	assert.Equal(t, string(ServerToolNameKnowledgeSearch), blocks[0].ServerToolCall.Name)
	args := blocks[0].ServerToolCall.Arguments.(*ServerToolCallArguments)
	assert.NotNil(t, args.KnowledgeSearch)
	assert.Equal(t, "resource_id", args.KnowledgeSearch.KnowledgeResourceID)
	assert.Equal(t, []string{"q1", "q2"}, args.KnowledgeSearch.Queries)

	_, err = knowledgeSearchCallToContentBlocks(&responses.OutputItem_FunctionKnowledgeSearch{})
	assert.Error(t, err)
}

func TestConvertDoubaoAppSearchResults(t *testing.T) {
	results := []*responses.DoubaoAppSearchResult{
		{
			TextCard: &responses.DoubaoAppSearchTextItem{
				Title:    "title1",
				Url:      "url1",
				Sitename: "site1",
			},
		},
		nil,
		{
			TextCard: nil,
		},
		{
			TextCard: &responses.DoubaoAppSearchTextItem{
				Title: "title2",
				Url:   "url2",
			},
		},
	}

	ret := convertDoubaoAppSearchResults(results)
	assert.Len(t, ret, 2)
	assert.Equal(t, "title1", ret[0].Title)
	assert.Equal(t, "url1", ret[0].URL)
	assert.Equal(t, "site1", ret[0].SiteName)
	assert.Equal(t, "title2", ret[1].Title)

	assert.Nil(t, convertDoubaoAppSearchResults(nil))
	assert.Nil(t, convertDoubaoAppSearchResults([]*responses.DoubaoAppSearchResult{}))
}

func TestServerToolResultToInputItem_ImageProcess(t *testing.T) {
	block := schema.NewContentBlock(&schema.ServerToolResult{
		Name: string(ServerToolNameImageProcess),
		Result: &ServerToolResult{
			ImageProcess: &ImageProcessResult{
				Action: &ImageProcessResultAction{
					Type:           ImageProcessActionPoint,
					ResultImageURL: "http://example.com/result.png",
				},
			},
		},
	})
	setItemID(block, "ip-id")
	setItemStatus(block, responses.ItemStatus_completed.String())

	item, err := serverToolResultToInputItem(block)
	assert.NoError(t, err)
	ip := item.GetImageProcess()
	assert.NotNil(t, ip)
	assert.Equal(t, "ip-id", ip.Id)
	assert.NotNil(t, ip.Action)
	assert.Equal(t, string(ImageProcessActionPoint), ip.Action.Type)
	assert.Equal(t, "http://example.com/result.png", ip.Action.GetResultImageUrl())
}

func TestServerToolResultToInputItem_ImageProcess_WithError(t *testing.T) {
	block := schema.NewContentBlock(&schema.ServerToolResult{
		Name: string(ServerToolNameImageProcess),
		Result: &ServerToolResult{
			ImageProcess: &ImageProcessResult{
				Error: &ImageProcessResultError{
					Message: "processing failed",
				},
			},
		},
	})
	setItemID(block, "ip-id")
	setItemStatus(block, responses.ItemStatus_failed.String())

	item, err := serverToolResultToInputItem(block)
	assert.NoError(t, err)
	ip := item.GetImageProcess()
	assert.NotNil(t, ip)
	assert.NotNil(t, ip.Error)
	assert.Equal(t, "processing failed", ip.Error.Message)
}

func TestServerToolResultToInputItem_DoubaoApp(t *testing.T) {
	block := schema.NewContentBlock(&schema.ServerToolResult{
		Name: string(ServerToolNameDoubaoApp),
		Result: &ServerToolResult{
			DoubaoApp: &DoubaoAppResult{
				Blocks: []*DoubaoAppBlock{
					{
						Type: DoubaoAppBlockTypeOutputText,
						OutputText: &DoubaoAppOutputText{
							ID:   "text_id",
							Text: "output text",
						},
					},
					{
						Type: DoubaoAppBlockTypeReasoningText,
						ReasoningText: &DoubaoAppReasoningText{
							ID:            "reasoning_id",
							ReasoningText: "reasoning text",
						},
					},
					{
						Type: DoubaoAppBlockTypeSearch,
						Search: &DoubaoAppSearch{
							ID:      "search_id",
							Summary: "search summary",
							Queries: []string{"q1"},
							Results: []*DoubaoAppSearchResult{
								{Title: "title", URL: "url", SiteName: "site"},
							},
						},
					},
					{
						Type: DoubaoAppBlockTypeReasoningSearch,
						ReasoningSearch: &DoubaoAppReasoningSearch{
							ID:      "rs_id",
							Summary: "rs summary",
							Queries: []string{"rq1"},
						},
					},
				},
			},
		},
	})
	setItemID(block, "da-id")
	setItemStatus(block, responses.ItemStatus_completed.String())

	item, err := serverToolResultToInputItem(block)
	assert.NoError(t, err)
	da := item.GetFunctionDoubaoAppCall()
	assert.NotNil(t, da)
	assert.Equal(t, "da-id", da.GetId())
	assert.Len(t, da.Blocks, 4)

	assert.Equal(t, "output text", da.Blocks[0].GetOutputText().Text)
	assert.Equal(t, "reasoning text", da.Blocks[1].GetReasoningText().ReasoningText)
	assert.Equal(t, "search summary", da.Blocks[2].GetSearch().GetSummary())
	assert.Len(t, da.Blocks[2].GetSearch().Results, 1)
	assert.Equal(t, "title", da.Blocks[2].GetSearch().Results[0].TextCard.Title)
	assert.Equal(t, "rs summary", da.Blocks[3].GetReasoningSearch().GetSummary())
}

func TestServerToolResultToInputItem_NilResult(t *testing.T) {
	block := schema.NewContentBlock(&schema.ServerToolResult{
		Name:   string(ServerToolNameImageProcess),
		Result: nil,
	})

	_, err := serverToolResultToInputItem(block)
	assert.Error(t, err)
}

func TestServerToolResultToInputItem_UnknownName(t *testing.T) {
	block := schema.NewContentBlock(&schema.ServerToolResult{
		Name:   "unknown",
		Result: &ServerToolResult{},
	})

	_, err := serverToolResultToInputItem(block)
	assert.Error(t, err)
}

func TestImageProcessResultToInputItem_Nil(t *testing.T) {
	_, err := imageProcessResultToInputItem("id", "completed", nil)
	assert.Error(t, err)
}

func TestDoubaoAppResultToInputItem_Nil(t *testing.T) {
	_, err := doubaoAppResultToInputItem("id", "completed", nil)
	assert.Error(t, err)
}

func TestDoubaoAppArgumentsToInputItem(t *testing.T) {
	item, err := doubaoAppArgumentsToInputItem("da-id", "completed", &DoubaoAppArguments{
		Feature: DoubaoAppFeature("ai_search"),
	})
	assert.NoError(t, err)
	da := item.GetFunctionDoubaoAppCall()
	assert.NotNil(t, da)
	assert.Equal(t, "da-id", da.GetId())
	assert.Equal(t, "ai_search", da.GetFeature())

	_, err = doubaoAppArgumentsToInputItem("id", "completed", nil)
	assert.Error(t, err)
}

func TestKnowledgeSearchArgumentsToInputItem(t *testing.T) {
	item, err := knowledgeSearchArgumentsToInputItem("ks-id", "completed", &KnowledgeSearchArguments{
		KnowledgeResourceID: "resource_id",
		Queries:             []string{"q1", "q2"},
	})
	assert.NoError(t, err)
	ks := item.GetFunctionKnowledgeSearch()
	assert.NotNil(t, ks)
	assert.Equal(t, "ks-id", ks.GetId())
	assert.Equal(t, "resource_id", ks.KnowledgeResourceId)
	assert.Equal(t, []string{"q1", "q2"}, ks.Queries)

	_, err = knowledgeSearchArgumentsToInputItem("id", "completed", nil)
	assert.Error(t, err)
}

func TestPairImageProcessToolCallItems(t *testing.T) {
	call := &responses.InputItem{
		Union: &responses.InputItem_ImageProcess{
			ImageProcess: &responses.ItemFunctionImageProcess{
				Type: responses.ItemType_image_process,
				Id:   "ip-id",
				Arguments: &responses.ResponseImageProcessArgs{
					Union: &responses.ResponseImageProcessArgs_PointArgs{
						PointArgs: &responses.ResponseImageProcessPointArgs{
							ImageIndex: 0,
							Points:     "[[100,200]]",
						},
					},
				},
			},
		},
	}
	result := &responses.InputItem{
		Union: &responses.InputItem_ImageProcess{
			ImageProcess: &responses.ItemFunctionImageProcess{
				Type:   responses.ItemType_image_process,
				Id:     "ip-id",
				Status: responses.ItemStatus_completed,
				Action: &responses.ResponseImageProcessAction{
					Type:           string(ImageProcessActionPoint),
					ResultImageUrl: ptrOf("http://example.com/result.png"),
				},
			},
		},
	}
	other := &responses.InputItem{
		Union: &responses.InputItem_FunctionWebSearchCall{
			FunctionWebSearchCall: &responses.ItemFunctionWebSearch{
				Id: "ws-id",
			},
		},
	}

	items, err := pairImageProcessToolCallItems([]*responses.InputItem{call, other, result})
	assert.NoError(t, err)
	assert.Len(t, items, 2)

	ip := items[0].GetImageProcess()
	assert.NotNil(t, ip)
	assert.Equal(t, "ip-id", ip.Id)
	assert.NotNil(t, ip.Arguments)
	assert.NotNil(t, ip.Action)
	assert.Equal(t, responses.ItemStatus_completed, ip.Status)

	assert.NotNil(t, items[1].GetFunctionWebSearchCall())

	// Error case: unpaired
	_, err = pairImageProcessToolCallItems([]*responses.InputItem{call})
	assert.Error(t, err)
}

func TestPairDoubaoAppToolCallItems(t *testing.T) {
	id := "da-id"
	feature := "ai_search"
	call := &responses.InputItem{
		Union: &responses.InputItem_FunctionDoubaoAppCall{
			FunctionDoubaoAppCall: &responses.ItemDoubaoAppCall{
				Type:    responses.ItemType_doubao_app_call,
				Id:      &id,
				Feature: &feature,
			},
		},
	}
	result := &responses.InputItem{
		Union: &responses.InputItem_FunctionDoubaoAppCall{
			FunctionDoubaoAppCall: &responses.ItemDoubaoAppCall{
				Type:   responses.ItemType_doubao_app_call,
				Id:     &id,
				Status: responses.ItemStatus_completed,
				Blocks: []*responses.DoubaoAppCallBlock{
					{
						Union: &responses.DoubaoAppCallBlock_OutputText{
							OutputText: &responses.DoubaoAppCallBlockOutputText{
								Text: "output",
							},
						},
					},
				},
			},
		},
	}

	items, err := pairDoubaoAppToolCallItems([]*responses.InputItem{call, result})
	assert.NoError(t, err)
	assert.Len(t, items, 1)

	da := items[0].GetFunctionDoubaoAppCall()
	assert.NotNil(t, da)
	assert.Equal(t, "da-id", da.GetId())
	assert.Equal(t, "ai_search", da.GetFeature())
	assert.Len(t, da.Blocks, 1)
	assert.Equal(t, "output", da.Blocks[0].GetOutputText().Text)
	assert.Equal(t, responses.ItemStatus_completed, da.Status)

	// Error case: unpaired
	_, err = pairDoubaoAppToolCallItems([]*responses.InputItem{call})
	assert.Error(t, err)
}

func TestConvertDoubaoAppSearchResultsToProto(t *testing.T) {
	results := []*DoubaoAppSearchResult{
		{Title: "t1", URL: "u1", SiteName: "s1"},
		nil,
		{Title: "t2", URL: "u2", SiteName: "s2"},
	}

	ret := convertDoubaoAppSearchResultsToProto(results)
	assert.Len(t, ret, 2)
	assert.Equal(t, "t1", ret[0].TextCard.Title)
	assert.Equal(t, "u1", ret[0].TextCard.Url)
	assert.Equal(t, "s1", ret[0].TextCard.Sitename)
	assert.Equal(t, "t2", ret[1].TextCard.Title)
}

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
	"fmt"
	"strings"
	"sync"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model/responses"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/types/known/structpb"
)

func toSystemRoleInputItems(msg *schema.AgenticMessage) (items []*responses.InputItem, err error) {
	items = make([]*responses.InputItem, 0, len(msg.ContentBlocks))

	for _, block := range msg.ContentBlocks {
		var item *responses.InputItem

		switch block.Type {
		case schema.ContentBlockTypeUserInputText:
			item, err = userInputTextToInputItem(responses.MessageRole_system, block.UserInputText)
			if err != nil {
				return nil, fmt.Errorf("failed to convert user input text to input item, err: %w", err)
			}

		case schema.ContentBlockTypeUserInputImage:
			item, err = userInputImageToInputItem(responses.MessageRole_system, block.UserInputImage)
			if err != nil {
				return nil, fmt.Errorf("failed to convert user input image to input item, err: %w", err)
			}

		default:
			return nil, fmt.Errorf("invalid content block type %q with system role", block.Type)
		}

		items = append(items, item)
	}

	return items, nil
}

func toAssistantRoleInputItems(msg *schema.AgenticMessage) (items []*responses.InputItem, err error) {
	items = make([]*responses.InputItem, 0, len(msg.ContentBlocks))

	isSelfGenerated := isSelfGeneratedMessage(msg)

	for _, block := range msg.ContentBlocks {
		// For non-self-generated messages, skip block types that are not in the whitelist.
		// These types are model-specific and cannot be safely converted.
		if !isSelfGenerated && !isAllowedNonSelfGeneratedBlockType(block.Type) {
			continue
		}

		var item *responses.InputItem

		switch block.Type {
		case schema.ContentBlockTypeAssistantGenText:
			item, err = assistantGenTextToInputItem(block)
			if err != nil {
				return nil, fmt.Errorf("failed to convert assistant generated text to input item: %w", err)
			}

		case schema.ContentBlockTypeReasoning:
			item, err = reasoningToInputItem(block)
			if err != nil {
				return nil, fmt.Errorf("failed to convert reasoning to input item: %w", err)
			}

		case schema.ContentBlockTypeFunctionToolCall:
			item, err = functionToolCallToInputItem(block)
			if err != nil {
				return nil, fmt.Errorf("failed to convert function tool call to input item: %w", err)
			}

		case schema.ContentBlockTypeServerToolCall:
			item, err = serverToolCallToInputItem(block)
			if err != nil {
				return nil, fmt.Errorf("failed to convert server tool call to input item: %w", err)
			}

		case schema.ContentBlockTypeServerToolResult:
			item, err = serverToolResultToInputItem(block)
			if err != nil {
				return nil, fmt.Errorf("failed to convert server tool result to input item: %w", err)
			}

		case schema.ContentBlockTypeMCPToolApprovalRequest:
			item, err = mcpToolApprovalRequestToInputItem(block)
			if err != nil {
				return nil, fmt.Errorf("failed to convert MCP tool approval request to input item: %w", err)
			}

		case schema.ContentBlockTypeMCPListToolsResult:
			item, err = mcpListToolsResultToInputItem(block)
			if err != nil {
				return nil, fmt.Errorf("failed to convert MCP list tools result to input item: %w", err)
			}

		case schema.ContentBlockTypeMCPToolCall:
			item, err = mcpToolCallToInputItem(block)
			if err != nil {
				return nil, fmt.Errorf("failed to convert MCP tool call to input item: %w", err)
			}

		case schema.ContentBlockTypeMCPToolResult:
			item, err = mcpToolResultToInputItem(block)
			if err != nil {
				return nil, fmt.Errorf("failed to convert MCP tool result to input item: %w", err)
			}

		default:
			return nil, fmt.Errorf("invalid content block type %q with assistant role", block.Type)
		}

		items = append(items, item)
	}

	items, err = pairToolCallItems(items)
	if err != nil {
		return nil, err
	}

	return items, nil
}

func pairToolCallItems(items []*responses.InputItem) (newItems []*responses.InputItem, err error) {
	items, err = pairMCPToolCallItems(items)
	if err != nil {
		return nil, fmt.Errorf("failed to pair MCP tool call items: %w", err)
	}

	items, err = pairImageProcessToolCallItems(items)
	if err != nil {
		return nil, fmt.Errorf("failed to pair image process tool call items: %w", err)
	}

	items, err = pairDoubaoAppToolCallItems(items)
	if err != nil {
		return nil, fmt.Errorf("failed to pair doubao app tool call items: %w", err)
	}

	return items, nil
}

func pairMCPToolCallItems(items []*responses.InputItem) (newItems []*responses.InputItem, err error) {
	processed := make(map[int]bool)
	itemIDIndices := make(map[string][]int)

	for i, item := range items {
		call := item.GetFunctionMcpCall()
		if call == nil {
			continue
		}
		id := call.GetId()
		if id == "" {
			return nil, fmt.Errorf("found MCP tool call item with empty ID at index %d", i)
		}
		itemIDIndices[id] = append(itemIDIndices[id], i)
	}

	for id, indices := range itemIDIndices {
		if len(indices) != 2 {
			return nil, fmt.Errorf("MCP tool call %q should have exactly 2 items (call and result), "+
				"but found %d", id, len(indices))
		}
	}

	for i, item := range items {
		if processed[i] {
			continue
		}

		call := item.GetFunctionMcpCall()
		if call == nil {
			newItems = append(newItems, item)
			continue
		}

		id := call.GetId()
		indices := itemIDIndices[id]

		var pairIndex int
		if indices[0] == i {
			pairIndex = indices[1]
		} else {
			pairIndex = indices[0]
		}

		pairMcpCall := items[pairIndex].GetFunctionMcpCall()

		mergedItem := &responses.InputItem{
			Union: &responses.InputItem_FunctionMcpCall{
				FunctionMcpCall: &responses.ItemFunctionMcpCall{
					Type:              responses.ItemType_mcp_call,
					Id:                &id,
					ServerLabel:       call.ServerLabel,
					ApprovalRequestId: coalesce(call.ApprovalRequestId, pairMcpCall.ApprovalRequestId),
					Name:              call.Name,
					Arguments:         coalesce(call.Arguments, pairMcpCall.Arguments),
					Output:            coalesce(call.Output, pairMcpCall.Output),
					Error:             coalesce(call.Error, pairMcpCall.Error),
				},
			},
		}

		newItems = append(newItems, mergedItem)

		processed[i] = true
		processed[pairIndex] = true
	}

	return newItems, nil
}

func pairImageProcessToolCallItems(items []*responses.InputItem) (newItems []*responses.InputItem, err error) {
	processed := make(map[int]bool)
	itemIDIndices := make(map[string][]int)

	for i, item := range items {
		call := item.GetImageProcess()
		if call == nil {
			continue
		}
		if call.Id == "" {
			return nil, fmt.Errorf("found image process tool call item with empty ID at index %d", i)
		}
		itemIDIndices[call.Id] = append(itemIDIndices[call.Id], i)
	}

	for id, indices := range itemIDIndices {
		if len(indices) != 2 {
			return nil, fmt.Errorf("image process tool call %q should have exactly 2 items (call and result), "+
				"but found %d", id, len(indices))
		}
	}

	for i, item := range items {
		if processed[i] {
			continue
		}

		call := item.GetImageProcess()
		if call == nil {
			newItems = append(newItems, item)
			continue
		}

		indices := itemIDIndices[call.Id]

		var pairIndex int
		if indices[0] == i {
			pairIndex = indices[1]
		} else {
			pairIndex = indices[0]
		}

		pairCall := items[pairIndex].GetImageProcess()

		mergedItem := &responses.InputItem{
			Union: &responses.InputItem_ImageProcess{
				ImageProcess: &responses.ItemFunctionImageProcess{
					Type:      responses.ItemType_image_process,
					Id:        call.Id,
					Status:    coalesce(call.Status, pairCall.Status),
					Arguments: coalesce(call.Arguments, pairCall.Arguments),
					Action:    coalesce(call.Action, pairCall.Action),
					Error:     coalesce(call.Error, pairCall.Error),
				},
			},
		}

		newItems = append(newItems, mergedItem)

		processed[i] = true
		processed[pairIndex] = true
	}

	return newItems, nil
}

func pairDoubaoAppToolCallItems(items []*responses.InputItem) (newItems []*responses.InputItem, err error) {
	processed := make(map[int]bool)
	itemIDIndices := make(map[string][]int)

	for i, item := range items {
		call := item.GetFunctionDoubaoAppCall()
		if call == nil {
			continue
		}
		id := call.GetId()
		if id == "" {
			return nil, fmt.Errorf("found doubao app tool call item with empty ID at index %d", i)
		}
		itemIDIndices[id] = append(itemIDIndices[id], i)
	}

	for id, indices := range itemIDIndices {
		if len(indices) != 2 {
			return nil, fmt.Errorf("doubao app tool call %q should have exactly 2 items (call and result), "+
				"but found %d", id, len(indices))
		}
	}

	for i, item := range items {
		if processed[i] {
			continue
		}

		call := item.GetFunctionDoubaoAppCall()
		if call == nil {
			newItems = append(newItems, item)
			continue
		}

		id := call.GetId()
		indices := itemIDIndices[id]

		var pairIndex int
		if indices[0] == i {
			pairIndex = indices[1]
		} else {
			pairIndex = indices[0]
		}

		pairCall := items[pairIndex].GetFunctionDoubaoAppCall()

		mergedItem := &responses.InputItem{
			Union: &responses.InputItem_FunctionDoubaoAppCall{
				FunctionDoubaoAppCall: &responses.ItemDoubaoAppCall{
					Type:    responses.ItemType_doubao_app_call,
					Id:      call.Id,
					Status:  coalesce(call.Status, pairCall.Status),
					Feature: coalesce(call.Feature, pairCall.Feature),
					Blocks:  coalesce(call.Blocks, pairCall.Blocks),
				},
			},
		}

		newItems = append(newItems, mergedItem)

		processed[i] = true
		processed[pairIndex] = true
	}

	return newItems, nil
}

func toUserRoleInputItems(msg *schema.AgenticMessage) (items []*responses.InputItem, err error) {
	items = make([]*responses.InputItem, 0, len(msg.ContentBlocks))

	for _, block := range msg.ContentBlocks {
		var item *responses.InputItem

		switch block.Type {
		case schema.ContentBlockTypeUserInputText:
			item, err = userInputTextToInputItem(responses.MessageRole_user, block.UserInputText)
			if err != nil {
				return nil, fmt.Errorf("failed to convert user input text to input item, err: %w", err)
			}

		case schema.ContentBlockTypeUserInputImage:
			item, err = userInputImageToInputItem(responses.MessageRole_user, block.UserInputImage)
			if err != nil {
				return nil, fmt.Errorf("failed to convert user input image to input item, err: %w", err)
			}

		case schema.ContentBlockTypeUserInputVideo:
			item, err = userInputVideoToInputItem(responses.MessageRole_user, block.UserInputVideo)
			if err != nil {
				return nil, fmt.Errorf("failed to convert user input video to input item: %w", err)
			}

		case schema.ContentBlockTypeFunctionToolResult:
			item, err = functionToolResultToInputItem(block.FunctionToolResult)
			if err != nil {
				return nil, fmt.Errorf("failed to convert function tool result to input item: %w", err)
			}

		case schema.ContentBlockTypeMCPToolApprovalResponse:
			item, err = mcpToolApprovalResponseToInputItem(block.MCPToolApprovalResponse)
			if err != nil {
				return nil, fmt.Errorf("failed to convert MCP tool approval response to input item: %w", err)
			}

		case schema.ContentBlockTypeUserInputFile:
			item, err = userInputFileToInputItem(responses.MessageRole_user, block.UserInputFile)
			if err != nil {
				return nil, fmt.Errorf("failed to convert user input file to input item: %w", err)
			}

		default:
			return nil, fmt.Errorf("invalid content block type %q with user role", block.Type)
		}

		items = append(items, item)
	}

	return items, nil
}

func userInputTextToInputItem(role responses.MessageRole_Enum, block *schema.UserInputText) (inputItem *responses.InputItem, err error) {
	item := &responses.ContentItem{
		Union: &responses.ContentItem_Text{
			Text: &responses.ContentItemText{
				Type: responses.ContentItemType_input_text,
				Text: block.Text,
			},
		},
	}

	inputItem = &responses.InputItem{
		Union: &responses.InputItem_InputMessage{
			InputMessage: &responses.ItemInputMessage{
				Type:    ptrOf(responses.ItemType_message),
				Role:    role,
				Content: []*responses.ContentItem{item},
			},
		},
	}

	return inputItem, nil
}

func userInputImageToInputItem(role responses.MessageRole_Enum, block *schema.UserInputImage) (inputItem *responses.InputItem, err error) {
	imageURL, err := resolveURL(block.URL, block.Base64Data, block.MIMEType)
	if err != nil {
		return nil, err
	}

	detail, err := toContentItemImageDetail(block.Detail)
	if err != nil {
		return nil, err
	}

	item := &responses.ContentItem{
		Union: &responses.ContentItem_Image{
			Image: &responses.ContentItemImage{
				Type:     responses.ContentItemType_input_image,
				ImageUrl: &imageURL,
				Detail:   detail,
			},
		},
	}

	inputItem = &responses.InputItem{
		Union: &responses.InputItem_InputMessage{
			InputMessage: &responses.ItemInputMessage{
				Type:    ptrOf(responses.ItemType_message),
				Role:    role,
				Content: []*responses.ContentItem{item},
			},
		},
	}

	return inputItem, nil
}

func toContentItemImageDetail(detail schema.ImageURLDetail) (*responses.ContentItemImageDetail_Enum, error) {
	switch detail {
	case schema.ImageURLDetailHigh:
		return responses.ContentItemImageDetail_high.Enum(), nil
	case schema.ImageURLDetailLow:
		return responses.ContentItemImageDetail_low.Enum(), nil
	case schema.ImageURLDetailAuto:
		return responses.ContentItemImageDetail_auto.Enum(), nil
	default:
		return nil, fmt.Errorf("invalid image detail: %s", detail)
	}
}

func userInputVideoToInputItem(role responses.MessageRole_Enum, block *schema.UserInputVideo) (inputItem *responses.InputItem, err error) {
	videoURL, err := resolveURL(block.URL, block.Base64Data, block.MIMEType)
	if err != nil {
		return nil, err
	}

	var fpsPtr *float32
	if fps, ok := GetUserInputVideoFPS(block); ok {
		fpsPtr = ptrOf(float32(fps))
	}

	contentItem := &responses.ContentItem{
		Union: &responses.ContentItem_Video{
			Video: &responses.ContentItemVideo{
				Type:     responses.ContentItemType_input_video,
				VideoUrl: videoURL,
				Fps:      fpsPtr,
			},
		},
	}

	inputItem = &responses.InputItem{
		Union: &responses.InputItem_InputMessage{
			InputMessage: &responses.ItemInputMessage{
				Type:    ptrOf(responses.ItemType_message),
				Role:    role,
				Content: []*responses.ContentItem{contentItem},
			},
		},
	}

	return inputItem, nil
}

func userInputFileToInputItem(role responses.MessageRole_Enum, block *schema.UserInputFile) (inputItem *responses.InputItem, err error) {
	fileItem := &responses.ContentItemFile{
		Type:     responses.ContentItemType_input_file,
		Filename: &block.Name,
	}

	if block.URL != "" {
		fileItem.FileUrl = &block.URL
	} else if block.Base64Data != "" {
		fileItem.FileData = &block.Base64Data
	} else {
		return nil, fmt.Errorf("file input must have either URL or Base64Data")
	}

	contentItem := &responses.ContentItem{
		Union: &responses.ContentItem_File{
			File: fileItem,
		},
	}

	inputItem = &responses.InputItem{
		Union: &responses.InputItem_InputMessage{
			InputMessage: &responses.ItemInputMessage{
				Type:    ptrOf(responses.ItemType_message),
				Role:    role,
				Content: []*responses.ContentItem{contentItem},
			},
		},
	}

	return inputItem, nil
}

func functionToolResultToInputItem(block *schema.FunctionToolResult) (item *responses.InputItem, err error) {
	item = &responses.InputItem{
		Union: &responses.InputItem_FunctionToolCallOutput{
			FunctionToolCallOutput: &responses.ItemFunctionToolCallOutput{
				Type:   responses.ItemType_function_call_output,
				CallId: block.CallID,
				Output: block.Result,
			},
		},
	}

	return item, nil
}

func assistantGenTextToInputItem(block *schema.ContentBlock) (item *responses.InputItem, err error) {
	content := block.AssistantGenText
	if content == nil {
		return item, fmt.Errorf("assistant generated text is nil")
	}

	id, _ := getItemID(block)
	status, _ := GetItemStatus(block)

	contentItem := &responses.ContentItem{
		Union: &responses.ContentItem_Text{
			Text: &responses.ContentItemText{
				Type: responses.ContentItemType_output_text,
				Text: content.Text,
			},
		},
	}

	item = &responses.InputItem{
		Union: &responses.InputItem_InputMessage{
			InputMessage: &responses.ItemInputMessage{
				Type: ptrOf(responses.ItemType_message),
				Id:   ptrIfNonZero(id),
				Status: func() *responses.ItemStatus_Enum {
					if status == "" {
						return nil
					}
					return ptrOf(responses.ItemStatus_Enum(responses.ItemStatus_Enum_value[status]))
				}(),
				Role:    responses.MessageRole_assistant,
				Content: []*responses.ContentItem{contentItem},
			},
		},
	}

	return item, nil
}

func functionToolCallToInputItem(block *schema.ContentBlock) (item *responses.InputItem, err error) {
	content := block.FunctionToolCall
	if content == nil {
		return item, fmt.Errorf("function tool call is nil")
	}

	id, _ := getItemID(block)
	status, _ := GetItemStatus(block)

	item = &responses.InputItem{
		Union: &responses.InputItem_FunctionToolCall{
			FunctionToolCall: &responses.ItemFunctionToolCall{
				Type: responses.ItemType_function_call,
				Id:   ptrIfNonZero(id),
				Status: func() *responses.ItemStatus_Enum {
					if status == "" {
						return nil
					}
					return ptrOf(responses.ItemStatus_Enum(responses.ItemStatus_Enum_value[status]))
				}(),
				CallId:    content.CallID,
				Name:      content.Name,
				Arguments: content.Arguments,
			},
		},
	}

	return item, nil
}

func reasoningToInputItem(block *schema.ContentBlock) (item *responses.InputItem, err error) {
	content := block.Reasoning
	if content == nil {
		return item, fmt.Errorf("reasoning is nil")
	}

	id, _ := getItemID(block)
	status, _ := GetItemStatus(block)

	item = &responses.InputItem{
		Union: &responses.InputItem_Reasoning{
			Reasoning: &responses.ItemReasoning{
				Type:   responses.ItemType_reasoning,
				Id:     ptrIfNonZero(id),
				Status: responses.ItemStatus_Enum(responses.ItemStatus_Enum_value[status]),
				Summary: []*responses.ReasoningSummaryPart{
					{Text: content.Text},
				},
			},
		},
	}

	return item, nil
}

func serverToolCallToInputItem(block *schema.ContentBlock) (item *responses.InputItem, err error) {
	content := block.ServerToolCall
	if content == nil {
		return item, fmt.Errorf("server tool call is nil")
	}

	id, _ := getItemID(block)
	status, _ := GetItemStatus(block)

	arguments, err := getServerToolCallArguments(content)
	if err != nil {
		return nil, err
	}

	switch ServerToolName(content.Name) {
	case ServerToolNameWebSearch:
		return webSearchArgumentsToInputItem(id, status, arguments.WebSearch)
	case ServerToolNameImageProcess:
		return imageProcessArgumentsToInputItem(id, status, arguments.ImageProcess)
	case ServerToolNameDoubaoApp:
		return doubaoAppArgumentsToInputItem(id, status, arguments.DoubaoApp)
	case ServerToolNameKnowledgeSearch:
		return knowledgeSearchArgumentsToInputItem(id, status, arguments.KnowledgeSearch)
	default:
		return nil, fmt.Errorf("unknown server tool name: %s", content.Name)
	}
}

func serverToolResultToInputItem(block *schema.ContentBlock) (item *responses.InputItem, err error) {
	content := block.ServerToolResult
	if content == nil {
		return item, fmt.Errorf("server tool result is nil")
	}

	id, _ := getItemID(block)
	status, _ := GetItemStatus(block)

	result, err := getServerToolResult(content)
	if err != nil {
		return nil, err
	}

	switch ServerToolName(content.Name) {
	case ServerToolNameImageProcess:
		return imageProcessResultToInputItem(id, status, result.ImageProcess)
	case ServerToolNameDoubaoApp:
		return doubaoAppResultToInputItem(id, status, result.DoubaoApp)
	default:
		return nil, fmt.Errorf("unknown server tool result name: %s", content.Name)
	}
}

func imageProcessResultToInputItem(id string, status string, result *ImageProcessResult) (item *responses.InputItem, err error) {
	if result == nil {
		return nil, fmt.Errorf("image process result is nil")
	}

	var action *responses.ResponseImageProcessAction
	if result.Action != nil {
		action = &responses.ResponseImageProcessAction{
			Type:           string(result.Action.Type),
			ResultImageUrl: &result.Action.ResultImageURL,
		}
	}

	var ipError *responses.ResponseImageProcessError
	if result.Error != nil {
		ipError = &responses.ResponseImageProcessError{
			Message: result.Error.Message,
		}
	}

	item = &responses.InputItem{
		Union: &responses.InputItem_ImageProcess{
			ImageProcess: &responses.ItemFunctionImageProcess{
				Type:   responses.ItemType_image_process,
				Id:     id,
				Status: responses.ItemStatus_Enum(responses.ItemStatus_Enum_value[status]),
				Action: action,
				Error:  ipError,
			},
		},
	}

	return item, nil
}

func doubaoAppResultToInputItem(id string, status string, result *DoubaoAppResult) (item *responses.InputItem, err error) {
	if result == nil {
		return nil, fmt.Errorf("doubao app result is nil")
	}

	blocks := make([]*responses.DoubaoAppCallBlock, 0, len(result.Blocks))
	for _, b := range result.Blocks {
		if b == nil {
			continue
		}

		block := &responses.DoubaoAppCallBlock{}

		switch b.Type {
		case DoubaoAppBlockTypeOutputText:
			block.Union = &responses.DoubaoAppCallBlock_OutputText{
				OutputText: &responses.DoubaoAppCallBlockOutputText{
					Id:       ptrIfNonZero(b.OutputText.ID),
					ParentId: ptrIfNonZero(b.OutputText.ParentID),
					Type:     responses.DoubaoAppBlockType_output_text,
					Text:     b.OutputText.Text,
				},
			}
		case DoubaoAppBlockTypeReasoningText:
			block.Union = &responses.DoubaoAppCallBlock_ReasoningText{
				ReasoningText: &responses.DoubaoAppCallBlockReasoningText{
					Id:            ptrIfNonZero(b.ReasoningText.ID),
					ParentId:      ptrIfNonZero(b.ReasoningText.ParentID),
					Type:          responses.DoubaoAppBlockType_reasoning_text,
					ReasoningText: b.ReasoningText.ReasoningText,
				},
			}
		case DoubaoAppBlockTypeSearch:
			block.Union = &responses.DoubaoAppCallBlock_Search{
				Search: &responses.DoubaoAppCallBlockSearch{
					Id:       ptrIfNonZero(b.Search.ID),
					ParentId: ptrIfNonZero(b.Search.ParentID),
					Type:     responses.DoubaoAppBlockType_search,
					Summary:  ptrIfNonZero(b.Search.Summary),
					Queries:  b.Search.Queries,
					Results:  convertDoubaoAppSearchResultsToProto(b.Search.Results),
				},
			}
		case DoubaoAppBlockTypeReasoningSearch:
			block.Union = &responses.DoubaoAppCallBlock_ReasoningSearch{
				ReasoningSearch: &responses.DoubaoAppCallBlockReasoningSearch{
					Id:       ptrIfNonZero(b.ReasoningSearch.ID),
					ParentId: ptrIfNonZero(b.ReasoningSearch.ParentID),
					Type:     responses.DoubaoAppBlockType_reasoning_search,
					Summary:  ptrIfNonZero(b.ReasoningSearch.Summary),
					Queries:  b.ReasoningSearch.Queries,
					Results:  convertDoubaoAppSearchResultsToProto(b.ReasoningSearch.Results),
				},
			}
		default:
			return nil, fmt.Errorf("unknown doubao app block type: %s", b.Type)
		}
		blocks = append(blocks, block)
	}

	item = &responses.InputItem{
		Union: &responses.InputItem_FunctionDoubaoAppCall{
			FunctionDoubaoAppCall: &responses.ItemDoubaoAppCall{
				Type:   responses.ItemType_doubao_app_call,
				Id:     ptrIfNonZero(id),
				Status: responses.ItemStatus_Enum(responses.ItemStatus_Enum_value[status]),
				Blocks: blocks,
			},
		},
	}

	return item, nil
}

func convertDoubaoAppSearchResultsToProto(results []*DoubaoAppSearchResult) []*responses.DoubaoAppSearchResult {
	ret := make([]*responses.DoubaoAppSearchResult, 0, len(results))
	for _, r := range results {
		if r == nil {
			continue
		}
		ret = append(ret, &responses.DoubaoAppSearchResult{
			TextCard: &responses.DoubaoAppSearchTextItem{
				Title:    r.Title,
				Url:      r.URL,
				Sitename: r.SiteName,
			},
		})
	}
	return ret
}

func webSearchArgumentsToInputItem(id string, status string, ws *WebSearchArguments) (item *responses.InputItem, err error) {
	if ws == nil {
		return nil, fmt.Errorf("web search arguments is nil")
	}

	var action *responses.Action
	switch ws.ActionType {
	case WebSearchActionSearch:
		action = &responses.Action{
			Type:  responses.ActionType_search,
			Query: ws.Search.Query,
		}
	default:
		return nil, fmt.Errorf("unknown web search action type: %s", ws.ActionType)
	}

	item = &responses.InputItem{
		Union: &responses.InputItem_FunctionWebSearchCall{
			FunctionWebSearchCall: &responses.ItemFunctionWebSearch{
				Type:   responses.ItemType_web_search_call,
				Id:     id,
				Status: responses.ItemStatus_Enum(responses.ItemStatus_Enum_value[status]),
				Action: action,
			},
		},
	}

	return item, nil
}

func imageProcessArgumentsToInputItem(id string, status string, ip *ImageProcessArguments) (item *responses.InputItem, err error) {
	if ip == nil {
		return nil, fmt.Errorf("image process arguments is nil")
	}

	var args *responses.ResponseImageProcessArgs
	switch ip.ActionType {
	case ImageProcessActionPoint:
		if ip.Point == nil {
			return nil, fmt.Errorf("point arguments is nil")
		}
		args = &responses.ResponseImageProcessArgs{
			Union: &responses.ResponseImageProcessArgs_PointArgs{
				PointArgs: &responses.ResponseImageProcessPointArgs{
					ImageIndex: ip.Point.ImageIndex,
					Points:     ip.Point.Points,
					DrawLine:   ip.Point.DrawLine,
				},
			},
		}
	case ImageProcessActionGrounding:
		if ip.Grounding == nil {
			return nil, fmt.Errorf("grounding arguments is nil")
		}
		args = &responses.ResponseImageProcessArgs{
			Union: &responses.ResponseImageProcessArgs_GroundingArgs{
				GroundingArgs: &responses.ResponseImageProcessGroundingArgs{
					ImageIndex: ip.Grounding.ImageIndex,
					BboxStr:    ip.Grounding.BboxStr,
					Crop:       ip.Grounding.Crop,
				},
			},
		}
	case ImageProcessActionRotate:
		if ip.Rotate == nil {
			return nil, fmt.Errorf("rotate arguments is nil")
		}
		args = &responses.ResponseImageProcessArgs{
			Union: &responses.ResponseImageProcessArgs_RotateArgs{
				RotateArgs: &responses.ResponseImageProcessRotateArgs{
					ImageIndex: ip.Rotate.ImageIndex,
					Degree:     ip.Rotate.Degree,
				},
			},
		}
	case ImageProcessActionZoom:
		if ip.Zoom == nil {
			return nil, fmt.Errorf("zoom arguments is nil")
		}
		args = &responses.ResponseImageProcessArgs{
			Union: &responses.ResponseImageProcessArgs_ZoomArgs{
				ZoomArgs: &responses.ResponseImageProcessZoomArgs{
					ImageIndex: ip.Zoom.ImageIndex,
					BboxStr:    ip.Zoom.BboxStr,
					Scale:      ip.Zoom.Scale,
				},
			},
		}
	default:
		return nil, fmt.Errorf("unknown image process action type: %s", ip.ActionType)
	}

	item = &responses.InputItem{
		Union: &responses.InputItem_ImageProcess{
			ImageProcess: &responses.ItemFunctionImageProcess{
				Type:      responses.ItemType_image_process,
				Id:        id,
				Status:    responses.ItemStatus_Enum(responses.ItemStatus_Enum_value[status]),
				Arguments: args,
			},
		},
	}

	return item, nil
}

func doubaoAppArgumentsToInputItem(id string, status string, da *DoubaoAppArguments) (item *responses.InputItem, err error) {
	if da == nil {
		return nil, fmt.Errorf("doubao app arguments is nil")
	}

	item = &responses.InputItem{
		Union: &responses.InputItem_FunctionDoubaoAppCall{
			FunctionDoubaoAppCall: &responses.ItemDoubaoAppCall{
				Type:    responses.ItemType_doubao_app_call,
				Id:      ptrIfNonZero(id),
				Status:  responses.ItemStatus_Enum(responses.ItemStatus_Enum_value[status]),
				Feature: ptrIfNonZero(string(da.Feature)),
			},
		},
	}

	return item, nil
}

func knowledgeSearchArgumentsToInputItem(id string, status string, ks *KnowledgeSearchArguments) (item *responses.InputItem, err error) {
	if ks == nil {
		return nil, fmt.Errorf("knowledge search arguments is nil")
	}

	item = &responses.InputItem{
		Union: &responses.InputItem_FunctionKnowledgeSearch{
			FunctionKnowledgeSearch: &responses.ItemFunctionKnowledgeSearch{
				Type:                responses.ItemType_knowledge_search_call,
				Id:                  ptrIfNonZero(id),
				Status:              responses.ItemStatus_Enum(responses.ItemStatus_Enum_value[status]),
				Queries:             ks.Queries,
				KnowledgeResourceId: ks.KnowledgeResourceID,
			},
		},
	}

	return item, nil
}

func mcpToolApprovalRequestToInputItem(block *schema.ContentBlock) (item *responses.InputItem, err error) {
	content := block.MCPToolApprovalRequest
	if content == nil {
		return item, fmt.Errorf("mcp tool approval request is nil")
	}

	item = &responses.InputItem{
		Union: &responses.InputItem_McpApprovalRequest{
			McpApprovalRequest: &responses.ItemFunctionMcpApprovalRequest{
				Type:        responses.ItemType_mcp_approval_request,
				Id:          ptrIfNonZero(content.ID),
				ServerLabel: content.ServerLabel,
				Arguments:   content.Arguments,
				Name:        content.Name,
			},
		},
	}

	return item, nil
}

func mcpToolApprovalResponseToInputItem(block *schema.MCPToolApprovalResponse) (item *responses.InputItem, err error) {
	item = &responses.InputItem{
		Union: &responses.InputItem_McpApprovalResponse{
			McpApprovalResponse: &responses.ItemFunctionMcpApprovalResponse{
				Type:              responses.ItemType_mcp_approval_response,
				Approve:           block.Approve,
				ApprovalRequestId: block.ApprovalRequestID,
				Reason: func() *string {
					if block.Reason == "" {
						return nil
					}
					return &block.Reason
				}(),
			},
		},
	}

	return item, nil
}

func mcpListToolsResultToInputItem(block *schema.ContentBlock) (item *responses.InputItem, err error) {
	content := block.MCPListToolsResult
	if content == nil {
		return item, fmt.Errorf("mcp list tools result is nil")
	}

	tools := make([]*responses.McpTool, 0, len(content.Tools))
	for i := range content.Tools {
		tool := content.Tools[i]

		sc, err := jsonschemaToMap(tool.InputSchema)
		if err != nil {
			return nil, fmt.Errorf("failed to convert tool input schema to map: %w", err)
		}

		sc_, err := structpb.NewStruct(sc)
		if err != nil {
			return nil, fmt.Errorf("failed to create structpb struct: %w", err)
		}

		tools = append(tools, &responses.McpTool{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: sc_,
		})
	}

	id, _ := getItemID(block)

	item = &responses.InputItem{
		Union: &responses.InputItem_McpListTools{
			McpListTools: &responses.ItemFunctionMcpListTools{
				Type:        responses.ItemType_mcp_list_tools,
				ServerLabel: content.ServerLabel,
				Tools:       tools,
				Id:          ptrIfNonZero(id),
				Error:       ptrIfNonZero(content.Error),
			},
		},
	}

	return item, nil
}

func mcpToolCallToInputItem(block *schema.ContentBlock) (item *responses.InputItem, err error) {
	content := block.MCPToolCall
	if content == nil {
		return item, fmt.Errorf("mcp tool call is nil")
	}

	id, _ := getItemID(block)

	item = &responses.InputItem{
		Union: &responses.InputItem_FunctionMcpCall{
			FunctionMcpCall: &responses.ItemFunctionMcpCall{
				Type:              responses.ItemType_mcp_call,
				Id:                ptrIfNonZero(id),
				ServerLabel:       content.ServerLabel,
				ApprovalRequestId: ptrIfNonZero(content.ApprovalRequestID),
				Arguments:         content.Arguments,
				Name:              content.Name,
			},
		},
	}

	return item, nil
}

func mcpToolResultToInputItem(block *schema.ContentBlock) (item *responses.InputItem, err error) {
	content := block.MCPToolResult
	if content == nil {
		return nil, fmt.Errorf("MCP tool result is nil")
	}

	id, _ := getItemID(block)

	item = &responses.InputItem{
		Union: &responses.InputItem_FunctionMcpCall{
			FunctionMcpCall: &responses.ItemFunctionMcpCall{
				Type:        responses.ItemType_mcp_call,
				Id:          ptrIfNonZero(id),
				ServerLabel: content.ServerLabel,
				Name:        content.Name,
				Output:      ptrIfNonZero(content.Result),
				Error: func() *string {
					if content.Error == nil {
						return nil
					}
					return &content.Error.Message
				}(),
			},
		},
	}

	return item, nil
}

func toOutputMessage(resp *responses.ResponseObject) (msg *schema.AgenticMessage, err error) {
	blocks := make([]*schema.ContentBlock, 0, len(resp.Output))

	for _, item := range resp.Output {
		var tmpBlocks []*schema.ContentBlock

		switch t := item.Union.(type) {
		case *responses.OutputItem_Reasoning:
			block, err := reasoningToContentBlocks(t)
			if err != nil {
				return nil, fmt.Errorf("failed to convert reasoning to content block: %w", err)
			}

			tmpBlocks = append(tmpBlocks, block)

		case *responses.OutputItem_OutputMessage:
			tmpBlocks, err = outputMessageToContentBlocks(t)
			if err != nil {
				return nil, fmt.Errorf("failed to convert output message to content blocks: %w", err)
			}

		case *responses.OutputItem_FunctionToolCall:
			block, err := functionToolCallToContentBlock(t)
			if err != nil {
				return nil, fmt.Errorf("failed to convert function tool call to content block: %w", err)
			}

			tmpBlocks = append(tmpBlocks, block)

		case *responses.OutputItem_FunctionMcpListTools:
			block, err := mcpListToolsToContentBlock(t)
			if err != nil {
				return nil, fmt.Errorf("failed to convert MCP list tools to content block: %w", err)
			}

			tmpBlocks = append(tmpBlocks, block)

		case *responses.OutputItem_FunctionMcpCall:
			tmpBlocks, err = mcpCallToContentBlocks(t)
			if err != nil {
				return nil, fmt.Errorf("failed to convert MCP call to content blocks: %w", err)
			}

			tmpBlocks = append(tmpBlocks, tmpBlocks...)

		case *responses.OutputItem_FunctionMcpApprovalRequest:
			block, err := mcpApprovalRequestToContentBlock(t)
			if err != nil {
				return nil, fmt.Errorf("failed to convert MCP approval request to content block: %w", err)
			}

			tmpBlocks = append(tmpBlocks, block)

		case *responses.OutputItem_FunctionWebSearch:
			block, err := webSearchToContentBlock(t)
			if err != nil {
				return nil, fmt.Errorf("failed to convert web search to content block: %w", err)
			}

			tmpBlocks = append(tmpBlocks, block)

		case *responses.OutputItem_FunctionImageProcess:
			bs, err := imageProcessToContentBlocks(t)
			if err != nil {
				return nil, fmt.Errorf("failed to convert image process to content blocks: %w", err)
			}

			tmpBlocks = append(tmpBlocks, bs...)

		case *responses.OutputItem_FunctionDoubaoAppCall:
			bs, err := doubaoAppCallToContentBlocks(t)
			if err != nil {
				return nil, fmt.Errorf("failed to convert doubao app call to content blocks: %w", err)
			}

			tmpBlocks = append(tmpBlocks, bs...)

		case *responses.OutputItem_FunctionKnowledgeSearch:
			bs, err := knowledgeSearchCallToContentBlocks(t)
			if err != nil {
				return nil, fmt.Errorf("failed to convert knowledge search call to content blocks: %w", err)
			}

			tmpBlocks = append(tmpBlocks, bs...)

		default:
			return nil, fmt.Errorf("unknown output item type: %T", t)
		}

		blocks = append(blocks, tmpBlocks...)
	}

	msg = &schema.AgenticMessage{
		Role:          schema.AgenticRoleTypeAssistant,
		ContentBlocks: blocks,
		ResponseMeta:  responseObjectToResponseMeta(resp),
	}

	msg = setSelfGenerated(msg)

	return msg, nil
}

func outputMessageToContentBlocks(item *responses.OutputItem_OutputMessage) (blocks []*schema.ContentBlock, err error) {
	outputMsg := item.OutputMessage
	if outputMsg == nil {
		return nil, fmt.Errorf("received empty output message")
	}

	blocks = make([]*schema.ContentBlock, 0, len(outputMsg.Content))

	for _, content := range outputMsg.Content {
		var block *schema.ContentBlock

		switch t := content.Union.(type) {
		case *responses.OutputContentItem_Text:
			block, err = outputContentTextToContentBlock(t.Text)
			if err != nil {
				return nil, fmt.Errorf("failed to convert output text to content block: %w", err)
			}

		default:
			return nil, fmt.Errorf("unknown output content item type: %T", t)
		}

		setItemID(block, outputMsg.Id)
		setItemStatus(block, outputMsg.Status.String())

		blocks = append(blocks, block)
	}

	return blocks, nil
}

func outputContentTextToContentBlock(text *responses.OutputContentItemText) (block *schema.ContentBlock, err error) {
	annotations := make([]*TextAnnotation, 0, len(text.Annotations))
	for _, anno := range text.Annotations {
		ta, err := outputTextAnnotationToTextAnnotation(anno)
		if err != nil {
			return nil, fmt.Errorf("failed to convert text annotation: %w", err)
		}
		annotations = append(annotations, ta)
	}

	block = schema.NewContentBlock(&schema.AssistantGenText{
		Text: text.Text,
		Extension: &AssistantGenTextExtension{
			Annotations: annotations,
		},
	})

	return block, nil
}

func outputTextAnnotationToTextAnnotation(anno *responses.Annotation) (*TextAnnotation, error) {
	var ta *TextAnnotation
	switch anno.Type {
	case responses.AnnotationType_url_citation:
		var coverImage *CoverImage
		if anno.CoverImage != nil {
			coverImage = &CoverImage{
				URL:    anno.GetCoverImage().GetUrl(),
				Width:  anno.CoverImage.Width,
				Height: anno.CoverImage.Height,
			}
		}

		ta = &TextAnnotation{
			Type: TextAnnotationTypeURLCitation,
			URLCitation: &URLCitation{
				Title:         anno.GetTitle(),
				URL:           anno.GetUrl(),
				LogoURL:       anno.GetLogoUrl(),
				MobileURL:     anno.GetMobileUrl(),
				SiteName:      anno.GetSiteName(),
				PublishTime:   anno.GetPublishTime(),
				CoverImage:    coverImage,
				Summary:       anno.GetSummary(),
				FreshnessInfo: anno.GetFreshnessInfo(),
			},
		}

	case responses.AnnotationType_doc_citation:
		var chunkAttachment []map[string]any
		for _, ca := range anno.ChunkAttachment {
			chunkAttachment = append(chunkAttachment, ca.AsMap())
		}

		ta = &TextAnnotation{
			Type: TextAnnotationTypeDocCitation,
			DocCitation: &DocCitation{
				DocID:           anno.GetDocId(),
				DocName:         anno.GetDocName(),
				ChunkID:         anno.ChunkId,
				ChunkAttachment: chunkAttachment,
			},
		}

	default:
		return nil, fmt.Errorf("unknown annotation type: %s", anno.Type.String())
	}

	return ta, nil
}

func functionToolCallToContentBlock(item *responses.OutputItem_FunctionToolCall) (block *schema.ContentBlock, err error) {
	toolCall := item.FunctionToolCall
	if toolCall == nil {
		return nil, fmt.Errorf("received empty function tool call")
	}

	block = schema.NewContentBlock(&schema.FunctionToolCall{
		CallID:    toolCall.CallId,
		Name:      toolCall.Name,
		Arguments: toolCall.Arguments,
	})

	if toolCall.Id != nil {
		setItemID(block, *toolCall.Id)
	}
	if toolCall.Status != nil {
		setItemStatus(block, toolCall.Status.String())
	}

	return block, nil
}

func imageProcessToContentBlocks(item *responses.OutputItem_FunctionImageProcess) (blocks []*schema.ContentBlock, err error) {
	ip := item.FunctionImageProcess
	if ip == nil {
		return nil, fmt.Errorf("received empty function image process")
	}

	var args *ImageProcessArguments
	if ipArgs := ip.Arguments; ipArgs != nil {
		args = &ImageProcessArguments{}
		switch t := ipArgs.Union.(type) {
		case *responses.ResponseImageProcessArgs_PointArgs:
			args.ActionType = ImageProcessActionPoint
			if t.PointArgs != nil {
				args.Point = &ImageProcessPoint{
					ImageIndex: t.PointArgs.ImageIndex,
					Points:     t.PointArgs.Points,
					DrawLine:   t.PointArgs.DrawLine,
				}
			}
		case *responses.ResponseImageProcessArgs_GroundingArgs:
			args.ActionType = ImageProcessActionGrounding
			if t.GroundingArgs != nil {
				args.Grounding = &ImageProcessGrounding{
					ImageIndex: t.GroundingArgs.ImageIndex,
					BboxStr:    t.GroundingArgs.BboxStr,
					Crop:       t.GroundingArgs.Crop,
				}
			}
		case *responses.ResponseImageProcessArgs_RotateArgs:
			args.ActionType = ImageProcessActionRotate
			if t.RotateArgs != nil {
				args.Rotate = &ImageProcessRotate{
					ImageIndex: t.RotateArgs.ImageIndex,
					Degree:     t.RotateArgs.Degree,
				}
			}
		case *responses.ResponseImageProcessArgs_ZoomArgs:
			args.ActionType = ImageProcessActionZoom
			if t.ZoomArgs != nil {
				args.Zoom = &ImageProcessZoom{
					ImageIndex: t.ZoomArgs.ImageIndex,
					BboxStr:    t.ZoomArgs.BboxStr,
					Scale:      t.ZoomArgs.Scale,
				}
			}
		default:
			return nil, fmt.Errorf("unknown image process args type: %T", t)
		}
	}

	callBlock := schema.NewContentBlock(&schema.ServerToolCall{
		Name:      string(ServerToolNameImageProcess),
		Arguments: &ServerToolCallArguments{ImageProcess: args},
	})

	result := &ImageProcessResult{
		Action: &ImageProcessResultAction{
			Type:           ImageProcessAction(ip.Action.GetType()),
			ResultImageURL: ip.Action.GetResultImageUrl(),
		},
		Error: func() *ImageProcessResultError {
			if ip.Error == nil {
				return nil
			}
			return &ImageProcessResultError{
				Message: ip.Error.Message,
			}
		}(),
	}

	resultBlock := schema.NewContentBlock(&schema.ServerToolResult{
		Name:   string(ServerToolNameImageProcess),
		Result: &ServerToolResult{ImageProcess: result},
	})

	setItemID(callBlock, ip.Id)
	setItemID(resultBlock, ip.Id)
	setItemStatus(callBlock, ip.Status.String())
	setItemStatus(resultBlock, ip.Status.String())

	blocks = []*schema.ContentBlock{callBlock, resultBlock}

	return blocks, nil
}

func doubaoAppCallToContentBlocks(item *responses.OutputItem_FunctionDoubaoAppCall) (blocks []*schema.ContentBlock, err error) {
	dac := item.FunctionDoubaoAppCall
	if dac == nil {
		return nil, fmt.Errorf("received empty doubao app call")
	}

	callBlock := schema.NewContentBlock(&schema.ServerToolCall{
		Name: string(ServerToolNameDoubaoApp),
		Arguments: &ServerToolCallArguments{
			DoubaoApp: &DoubaoAppArguments{
				Feature: DoubaoAppFeature(dac.GetFeature()),
			},
		},
	})

	result := &DoubaoAppResult{
		Blocks: make([]*DoubaoAppBlock, 0, len(dac.Blocks)),
	}

	for _, block := range dac.Blocks {
		if block == nil {
			continue
		}
		daBlock := &DoubaoAppBlock{}
		switch b := block.Union.(type) {
		case *responses.DoubaoAppCallBlock_OutputText:
			daBlock.Type = DoubaoAppBlockTypeOutputText
			daBlock.OutputText = &DoubaoAppOutputText{
				ID:       b.OutputText.GetId(),
				ParentID: b.OutputText.GetParentId(),
				Text:     b.OutputText.GetText(),
				Status:   b.OutputText.GetStatus().String(),
			}
		case *responses.DoubaoAppCallBlock_ReasoningText:
			daBlock.Type = DoubaoAppBlockTypeReasoningText
			daBlock.ReasoningText = &DoubaoAppReasoningText{
				ID:            b.ReasoningText.GetId(),
				ParentID:      b.ReasoningText.GetParentId(),
				ReasoningText: b.ReasoningText.GetReasoningText(),
				Status:        b.ReasoningText.GetStatus().String(),
			}
		case *responses.DoubaoAppCallBlock_Search:
			daBlock.Type = DoubaoAppBlockTypeSearch
			daBlock.Search = &DoubaoAppSearch{
				ID:       b.Search.GetId(),
				ParentID: b.Search.GetParentId(),
				Summary:  b.Search.GetSummary(),
				Queries:  b.Search.GetQueries(),
				Results:  convertDoubaoAppSearchResults(b.Search.GetResults()),
				Status:   b.Search.GetStatus().String(),
			}
		case *responses.DoubaoAppCallBlock_ReasoningSearch:
			daBlock.Type = DoubaoAppBlockTypeReasoningSearch
			daBlock.ReasoningSearch = &DoubaoAppReasoningSearch{
				ID:       b.ReasoningSearch.GetId(),
				ParentID: b.ReasoningSearch.GetParentId(),
				Summary:  b.ReasoningSearch.GetSummary(),
				Queries:  b.ReasoningSearch.GetQueries(),
				Results:  convertDoubaoAppSearchResults(b.ReasoningSearch.GetResults()),
				Status:   b.ReasoningSearch.GetStatus().String(),
			}
		default:
			continue
		}
		result.Blocks = append(result.Blocks, daBlock)
	}

	resultBlock := schema.NewContentBlock(&schema.ServerToolResult{
		Name:   string(ServerToolNameDoubaoApp),
		Result: &ServerToolResult{DoubaoApp: result},
	})

	setItemID(callBlock, dac.GetId())
	setItemID(resultBlock, dac.GetId())
	setItemStatus(callBlock, dac.Status.String())
	setItemStatus(resultBlock, dac.Status.String())

	return []*schema.ContentBlock{callBlock, resultBlock}, nil
}

func convertDoubaoAppSearchResults(results []*responses.DoubaoAppSearchResult) []*DoubaoAppSearchResult {
	if len(results) == 0 {
		return nil
	}
	ret := make([]*DoubaoAppSearchResult, 0, len(results))
	for _, r := range results {
		if r == nil || r.TextCard == nil {
			continue
		}
		ret = append(ret, &DoubaoAppSearchResult{
			Title:    r.TextCard.Title,
			URL:      r.TextCard.Url,
			SiteName: r.TextCard.Sitename,
		})
	}
	return ret
}

func knowledgeSearchCallToContentBlocks(item *responses.OutputItem_FunctionKnowledgeSearch) (blocks []*schema.ContentBlock, err error) {
	ks := item.FunctionKnowledgeSearch
	if ks == nil {
		return nil, fmt.Errorf("received empty knowledge search call")
	}

	args := &KnowledgeSearchArguments{
		KnowledgeResourceID: ks.KnowledgeResourceId,
		Queries:             ks.Queries,
	}

	block := schema.NewContentBlock(&schema.ServerToolCall{
		Name: string(ServerToolNameKnowledgeSearch),
		Arguments: &ServerToolCallArguments{
			KnowledgeSearch: args,
		},
	})

	setItemID(block, ks.GetId())
	setItemStatus(block, ks.Status.String())

	return []*schema.ContentBlock{block}, nil
}

func webSearchToContentBlock(item *responses.OutputItem_FunctionWebSearch) (block *schema.ContentBlock, err error) {
	webSearch := item.FunctionWebSearch
	if webSearch == nil {
		return nil, fmt.Errorf("received empty function web search")
	}

	var args *ServerToolCallArguments
	if action := webSearch.Action; action != nil {
		switch action_ := WebSearchAction(action.Type.String()); action_ {
		case WebSearchActionSearch:
			args = &ServerToolCallArguments{
				WebSearch: &WebSearchArguments{
					ActionType: action_,
					Search: &WebSearchQuery{
						Query: webSearch.Action.Query,
					},
				},
			}

		default:
			return nil, fmt.Errorf("unknown web search action type: %s", action_)
		}
	}

	block = schema.NewContentBlock(&schema.ServerToolCall{
		Name:      string(ServerToolNameWebSearch),
		Arguments: args,
	})

	setItemID(block, webSearch.Id)
	setItemStatus(block, webSearch.Status.String())

	return block, nil
}

func reasoningToContentBlocks(item *responses.OutputItem_Reasoning) (block *schema.ContentBlock, err error) {
	reasoning := item.Reasoning
	if reasoning == nil {
		return nil, fmt.Errorf("received empty reasoning")
	}

	var text strings.Builder
	for i, s := range reasoning.Summary {
		if i != 0 {
			text.WriteString("\n")
		}
		text.WriteString(s.Text)
	}

	block = schema.NewContentBlock(&schema.Reasoning{
		Text: text.String(),
	})

	if reasoning.Id != nil {
		setItemID(block, *reasoning.Id)
	}
	setItemStatus(block, reasoning.Status.String())

	return block, nil
}

func mcpCallToContentBlocks(item *responses.OutputItem_FunctionMcpCall) (blocks []*schema.ContentBlock, err error) {
	mcpCall := item.FunctionMcpCall
	if mcpCall == nil {
		return nil, fmt.Errorf("received empty MCP call")
	}

	callBlock := schema.NewContentBlock(&schema.MCPToolCall{
		ServerLabel:       mcpCall.ServerLabel,
		ApprovalRequestID: mcpCall.GetApprovalRequestId(),
		Name:              mcpCall.Name,
		Arguments:         mcpCall.Arguments,
	})

	resultBlock := schema.NewContentBlock(&schema.MCPToolResult{
		ServerLabel: mcpCall.ServerLabel,
		Name:        mcpCall.Name,
		Result:      mcpCall.GetOutput(),
		Error: func() *schema.MCPToolCallError {
			if mcpCall.Error == nil {
				return nil
			}
			return &schema.MCPToolCallError{
				Message: mcpCall.GetError(),
			}
		}(),
	})

	if mcpCall.Id != nil {
		setItemID(callBlock, *mcpCall.Id)
		setItemID(resultBlock, *mcpCall.Id)
	}
	if resultBlock.MCPToolResult.Error == nil {
		setItemStatus(resultBlock, responses.ItemStatus_completed.String())
	} else {
		setItemStatus(resultBlock, responses.ItemStatus_failed.String())
	}

	blocks = []*schema.ContentBlock{callBlock, resultBlock}

	return blocks, nil
}

func mcpListToolsToContentBlock(item *responses.OutputItem_FunctionMcpListTools) (block *schema.ContentBlock, err error) {
	mcpListTools := item.FunctionMcpListTools
	if mcpListTools == nil {
		return nil, fmt.Errorf("received empty MCP list tools")
	}

	group := &errgroup.Group{}
	group.SetLimit(5)
	mu := sync.Mutex{}

	tools := make([]*schema.MCPListToolsItem, 0, len(mcpListTools.Tools))
	for i := range mcpListTools.Tools {
		tool := mcpListTools.Tools[i]

		group.Go(func() error {
			b, err := sonic.Marshal(tool.InputSchema)
			if err != nil {
				return fmt.Errorf("failed to marshal tool input schema: %w", err)
			}

			sc := &jsonschema.Schema{}
			if err := sonic.Unmarshal(b, sc); err != nil {
				return fmt.Errorf("failed to unmarshal tool input schema: %w", err)
			}

			mu.Lock()
			defer mu.Unlock()

			tools = append(tools, &schema.MCPListToolsItem{
				Name:        tool.Name,
				Description: tool.Description,
				InputSchema: sc,
			})

			return nil
		})
	}

	if err = group.Wait(); err != nil {
		return nil, err
	}

	block = schema.NewContentBlock(&schema.MCPListToolsResult{
		ServerLabel: mcpListTools.ServerLabel,
		Tools:       tools,
		Error:       mcpListTools.GetError(),
	})

	if mcpListTools.Id != nil {
		setItemID(block, *mcpListTools.Id)
	}

	return block, nil
}

func mcpApprovalRequestToContentBlock(item *responses.OutputItem_FunctionMcpApprovalRequest) (block *schema.ContentBlock, err error) {
	apReq := item.FunctionMcpApprovalRequest
	if apReq == nil {
		return nil, fmt.Errorf("received empty MCP approval request")
	}

	block = schema.NewContentBlock(&schema.MCPToolApprovalRequest{
		ID:          apReq.GetId(),
		ServerLabel: apReq.ServerLabel,
		Name:        apReq.Name,
		Arguments:   apReq.Arguments,
	})

	if apReq.Id != nil {
		setItemID(block, *apReq.Id)
	}

	return block, nil
}

func responseObjectToResponseMeta(obj *responses.ResponseObject) *schema.AgenticResponseMeta {
	return &schema.AgenticResponseMeta{
		TokenUsage: toTokenUsage(obj),
		Extension:  toResponseMetaExtension(obj),
	}
}

func toTokenUsage(resp *responses.ResponseObject) (tokenUsage *schema.TokenUsage) {
	if resp.Usage == nil {
		return nil
	}

	usage := &schema.TokenUsage{
		PromptTokens: int(resp.Usage.InputTokens),
		PromptTokenDetails: schema.PromptTokenDetails{
			CachedTokens: int(resp.Usage.InputTokensDetails.GetCachedTokens()),
		},
		CompletionTokens: int(resp.Usage.OutputTokens),
		CompletionTokensDetails: schema.CompletionTokensDetails{
			ReasoningTokens: int(resp.Usage.OutputTokensDetails.GetReasoningTokens()),
		},
		TotalTokens: int(resp.Usage.TotalTokens),
	}

	return usage
}

func toResponseMetaExtension(resp *responses.ResponseObject) *ResponseMetaExtension {
	if resp == nil {
		return nil
	}

	var incompleteDetails *IncompleteDetails
	if details := resp.IncompleteDetails; details != nil {
		var contentFilter *ContentFilter
		if filter := details.ContentFilter; filter != nil {
			contentFilter = &ContentFilter{
				Type:    filter.Type,
				Details: filter.Details,
			}
		}
		incompleteDetails = &IncompleteDetails{
			Reason:        details.Reason,
			ContentFilter: contentFilter,
		}
	}

	var respErr *ResponseError
	if e := resp.Error; e != nil {
		respErr = &ResponseError{
			Code:    e.Code,
			Message: e.Message,
		}
	}

	var thinking *ResponseThinking
	if t := resp.Thinking; t != nil {
		thinking = &ResponseThinking{
			Type: ThinkingType(t.Type.String()),
		}
	}

	var serviceTier ServiceTier
	if s := resp.ServiceTier; s != nil {
		serviceTier = ServiceTier(s.String())
	}

	var status ResponseStatus
	if s := resp.Status; s != responses.ResponseStatus_unspecified {
		status = ResponseStatus(s.String())
	}

	extension := &ResponseMetaExtension{
		ID:                 resp.Id,
		Status:             status,
		IncompleteDetails:  incompleteDetails,
		Error:              respErr,
		PreviousResponseID: resp.GetPreviousResponseId(),
		Thinking:           thinking,
		ExpireAt:           resp.ExpireAt,
		ServiceTier:        serviceTier,
	}

	return extension
}

func resolveURL(url string, base64Data string, mimeType string) (real string, err error) {
	if url != "" {
		return url, nil
	}

	if mimeType == "" {
		return "", fmt.Errorf("mimeType is required when using base64Data")
	}

	real, err = ensureDataURL(base64Data, mimeType)
	if err != nil {
		return "", err
	}

	return real, nil
}

func ensureDataURL(base64Data, mimeType string) (string, error) {
	if strings.HasPrefix(base64Data, "data:") {
		return "", fmt.Errorf("base64Data field must be a raw base64 string, but got a string with prefix 'data:'")
	}
	if mimeType == "" {
		return "", fmt.Errorf("mimeType is required")
	}
	return fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data), nil
}

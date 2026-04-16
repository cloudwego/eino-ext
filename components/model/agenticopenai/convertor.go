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
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/eino/schema/openai"
	"github.com/eino-contrib/jsonschema"
	"github.com/openai/openai-go/v3/packages/param"
	"github.com/openai/openai-go/v3/responses"
	"golang.org/x/sync/errgroup"
)

func toSystemRoleInputItems(msg *schema.AgenticMessage) (items []responses.ResponseInputItemUnionParam, err error) {
	items = make([]responses.ResponseInputItemUnionParam, 0, len(msg.ContentBlocks))

	for _, block := range msg.ContentBlocks {
		var item responses.ResponseInputItemUnionParam

		switch block.Type {
		case schema.ContentBlockTypeUserInputText:
			item, err = userInputTextToInputItem(responses.EasyInputMessageRoleSystem, block.UserInputText)
			if err != nil {
				return nil, fmt.Errorf("failed to convert user input text to input item: %w", err)
			}

		case schema.ContentBlockTypeUserInputImage:
			item, err = userInputImageToInputItem(responses.EasyInputMessageRoleSystem, block.UserInputImage)
			if err != nil {
				return nil, fmt.Errorf("failed to convert user input image to input item: %w", err)
			}

		default:
			return nil, fmt.Errorf("invalid content block type %q with system role", block.Type)
		}

		items = append(items, item)
	}

	return items, nil
}

func toAssistantRoleInputItems(msg *schema.AgenticMessage) (items []responses.ResponseInputItemUnionParam, err error) {
	items = make([]responses.ResponseInputItemUnionParam, 0, len(msg.ContentBlocks))

	isSelfGenerated := isSelfGeneratedMessage(msg)

	for _, block := range msg.ContentBlocks {
		// For non-self-generated messages, skip block types that are not in the whitelist.
		// These types are model-specific and cannot be safely converted.
		if !isSelfGenerated && !isAllowedNonSelfGeneratedBlockType(block.Type) {
			continue
		}

		var item responses.ResponseInputItemUnionParam

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

func pairToolCallItems(items []responses.ResponseInputItemUnionParam) (newItems []responses.ResponseInputItemUnionParam, err error) {
	items, err = pairMCPToolCallItems(items)
	if err != nil {
		return nil, fmt.Errorf("failed to pair MCP tool call items: %w", err)
	}

	items, err = pairWebSearchServerToolCallItems(items)
	if err != nil {
		return nil, fmt.Errorf("failed to pair web server tool call items: %w", err)
	}

	items, err = pairFileSearchToolCallItems(items)
	if err != nil {
		return nil, fmt.Errorf("failed to pair file search tool call items: %w", err)
	}

	items, err = pairCodeInterpreterToolCallItems(items)
	if err != nil {
		return nil, fmt.Errorf("failed to pair code interpreter tool call items: %w", err)
	}

	items, err = pairImageGenerationToolCallItems(items)
	if err != nil {
		return nil, fmt.Errorf("failed to pair image generation tool call items: %w", err)
	}

	return items, nil
}

func pairMCPToolCallItems(items []responses.ResponseInputItemUnionParam) (newItems []responses.ResponseInputItemUnionParam, err error) {
	processed := make(map[int]bool)
	mcpCallItemIDIndices := make(map[string][]int)

	for i, item := range items {
		mcpCall := item.OfMcpCall
		if mcpCall == nil {
			continue
		}

		id := mcpCall.ID
		if id == "" {
			return nil, fmt.Errorf("found MCP tool call item with empty ID")
		}

		mcpCallItemIDIndices[id] = append(mcpCallItemIDIndices[id], i)
	}

	for id, indices := range mcpCallItemIDIndices {
		if len(indices) != 2 {
			return nil, fmt.Errorf("MCP tool call %q should have exactly 2 items (call and result), "+
				"but found %d", id, len(indices))
		}
	}

	for i, item := range items {
		if processed[i] {
			continue
		}

		mcpCall := item.OfMcpCall
		if mcpCall == nil {
			newItems = append(newItems, item)
			continue
		}

		id := mcpCall.ID
		indices := mcpCallItemIDIndices[id]

		var pairIndex int
		if indices[0] == i {
			pairIndex = indices[1]
		} else {
			pairIndex = indices[0]
		}

		pairMcpCall := items[pairIndex].OfMcpCall

		mergedItem := responses.ResponseInputItemUnionParam{
			OfMcpCall: &responses.ResponseInputItemMcpCallParam{
				ID:                mcpCall.ID,
				ServerLabel:       coalesce(mcpCall.ServerLabel, pairMcpCall.ServerLabel),
				ApprovalRequestID: coalesce(mcpCall.ApprovalRequestID, pairMcpCall.ApprovalRequestID),
				Name:              mcpCall.Name,
				Arguments:         coalesce(mcpCall.Arguments, pairMcpCall.Arguments),
				Output:            coalesce(mcpCall.Output, pairMcpCall.Output),
				Error:             coalesce(mcpCall.Error, pairMcpCall.Error),
				Status:            coalesce(mcpCall.Status, pairMcpCall.Status),
			},
		}

		newItems = append(newItems, mergedItem)

		processed[i] = true
		processed[pairIndex] = true
	}

	return newItems, nil
}

func pairWebSearchServerToolCallItems(items []responses.ResponseInputItemUnionParam) (newItems []responses.ResponseInputItemUnionParam, err error) {
	processed := make(map[int]bool)
	itemIDIndices := make(map[string][]int)

	for i, item := range items {
		call := item.OfWebSearchCall
		if call == nil {
			continue
		}
		if call.ID == "" {
			return nil, fmt.Errorf("found server tool call item with empty ID at index %d", i)
		}
		itemIDIndices[call.ID] = append(itemIDIndices[call.ID], i)
	}

	for id, indices := range itemIDIndices {
		if len(indices) != 2 {
			return nil, fmt.Errorf("server tool call %q should have exactly 2 items (call and result), "+
				"but found %d", id, len(indices))
		}
	}

	for i, item := range items {
		if processed[i] {
			continue
		}

		call := item.OfWebSearchCall
		if call == nil {
			newItems = append(newItems, item)
			continue
		}

		indices := itemIDIndices[call.ID]

		var pairIndex int
		if indices[0] == i {
			pairIndex = indices[1]
		} else {
			pairIndex = indices[0]
		}

		pairCall := items[pairIndex].OfWebSearchCall

		mergedItem := responses.ResponseInputItemUnionParam{
			OfWebSearchCall: &responses.ResponseFunctionWebSearchParam{
				ID:     call.ID,
				Action: pairWebSearchAction(call.Action, pairCall.Action),
				Status: coalesce(call.Status, pairCall.Status),
			},
		}

		newItems = append(newItems, mergedItem)

		processed[i] = true
		processed[pairIndex] = true
	}

	return newItems, nil
}

func pairWebSearchAction(action, pairAction responses.ResponseFunctionWebSearchActionUnionParam) responses.ResponseFunctionWebSearchActionUnionParam {
	ret := responses.ResponseFunctionWebSearchActionUnionParam{}

	if action.OfFind != nil {
		ret.OfFind = action.OfFind
	} else if pairAction.OfFind != nil {
		ret.OfFind = pairAction.OfFind
	}

	if action.OfOpenPage != nil {
		ret.OfOpenPage = action.OfOpenPage
	} else if pairAction.OfOpenPage != nil {
		ret.OfOpenPage = pairAction.OfOpenPage
	}

	if action.OfSearch == nil {
		ret.OfSearch = pairAction.OfSearch
	}
	if pairAction.OfSearch == nil {
		ret.OfSearch = action.OfSearch
	}
	if action.OfSearch != nil && pairAction.OfSearch != nil {
		ret.OfSearch = action.OfSearch
		if len(pairAction.OfSearch.Queries) > 0 {
			ret.OfSearch.Queries = pairAction.OfSearch.Queries
		}
		if len(pairAction.OfSearch.Sources) > 0 {
			ret.OfSearch.Sources = pairAction.OfSearch.Sources
		}
	}

	return ret
}

func pairFileSearchToolCallItems(items []responses.ResponseInputItemUnionParam) (newItems []responses.ResponseInputItemUnionParam, err error) {
	processed := make(map[int]bool)
	itemIDIndices := make(map[string][]int)

	for i, item := range items {
		call := item.OfFileSearchCall
		if call == nil {
			continue
		}
		if call.ID == "" {
			return nil, fmt.Errorf("found file search tool call item with empty ID at index %d", i)
		}
		itemIDIndices[call.ID] = append(itemIDIndices[call.ID], i)
	}

	for id, indices := range itemIDIndices {
		if len(indices) != 2 {
			return nil, fmt.Errorf("file search tool call %q should have exactly 2 items (call and result), "+
				"but found %d", id, len(indices))
		}
	}

	for i, item := range items {
		if processed[i] {
			continue
		}

		call := item.OfFileSearchCall
		if call == nil {
			newItems = append(newItems, item)
			continue
		}

		indices := itemIDIndices[call.ID]

		var pairIndex int
		if indices[0] == i {
			pairIndex = indices[1]
		} else {
			pairIndex = indices[0]
		}

		pairCall := items[pairIndex].OfFileSearchCall

		mergedItem := responses.ResponseInputItemUnionParam{
			OfFileSearchCall: &responses.ResponseFileSearchToolCallParam{
				ID:      call.ID,
				Status:  coalesce(call.Status, pairCall.Status),
				Queries: coalesce(call.Queries, pairCall.Queries),
				Results: coalesce(call.Results, pairCall.Results),
			},
		}

		newItems = append(newItems, mergedItem)

		processed[i] = true
		processed[pairIndex] = true
	}

	return newItems, nil
}

func pairCodeInterpreterToolCallItems(items []responses.ResponseInputItemUnionParam) (newItems []responses.ResponseInputItemUnionParam, err error) {
	processed := make(map[int]bool)
	itemIDIndices := make(map[string][]int)

	for i, item := range items {
		call := item.OfCodeInterpreterCall
		if call == nil {
			continue
		}
		if call.ID == "" {
			return nil, fmt.Errorf("found code interpreter tool call item with empty ID at index %d", i)
		}
		itemIDIndices[call.ID] = append(itemIDIndices[call.ID], i)
	}

	for id, indices := range itemIDIndices {
		if len(indices) != 2 {
			return nil, fmt.Errorf("code interpreter tool call %q should have exactly 2 items (call and result), "+
				"but found %d", id, len(indices))
		}
	}

	for i, item := range items {
		if processed[i] {
			continue
		}

		call := item.OfCodeInterpreterCall
		if call == nil {
			newItems = append(newItems, item)
			continue
		}

		indices := itemIDIndices[call.ID]

		var pairIndex int
		if indices[0] == i {
			pairIndex = indices[1]
		} else {
			pairIndex = indices[0]
		}

		pairCall := items[pairIndex].OfCodeInterpreterCall

		mergedItem := responses.ResponseInputItemUnionParam{
			OfCodeInterpreterCall: &responses.ResponseCodeInterpreterToolCallParam{
				ID:          call.ID,
				ContainerID: coalesce(call.ContainerID, pairCall.ContainerID),
				Status:      coalesce(call.Status, pairCall.Status),
				Code:        coalesce(call.Code, pairCall.Code),
				Outputs:     coalesce(call.Outputs, pairCall.Outputs),
			},
		}

		newItems = append(newItems, mergedItem)

		processed[i] = true
		processed[pairIndex] = true
	}

	return newItems, nil
}

func pairImageGenerationToolCallItems(items []responses.ResponseInputItemUnionParam) (newItems []responses.ResponseInputItemUnionParam, err error) {
	processed := make(map[int]bool)
	itemIDIndices := make(map[string][]int)

	for i, item := range items {
		call := item.OfImageGenerationCall
		if call == nil {
			continue
		}
		if call.ID == "" {
			return nil, fmt.Errorf("found image generation tool call item with empty ID at index %d", i)
		}
		itemIDIndices[call.ID] = append(itemIDIndices[call.ID], i)
	}

	for id, indices := range itemIDIndices {
		if len(indices) != 2 {
			return nil, fmt.Errorf("image generation tool call %q should have exactly 2 items (call and result), "+
				"but found %d", id, len(indices))
		}
	}

	for i, item := range items {
		if processed[i] {
			continue
		}

		call := item.OfImageGenerationCall
		if call == nil {
			newItems = append(newItems, item)
			continue
		}

		indices := itemIDIndices[call.ID]

		var pairIndex int
		if indices[0] == i {
			pairIndex = indices[1]
		} else {
			pairIndex = indices[0]
		}

		pairCall := items[pairIndex].OfImageGenerationCall

		mergedItem := responses.ResponseInputItemUnionParam{
			OfImageGenerationCall: &responses.ResponseInputItemImageGenerationCallParam{
				ID:     call.ID,
				Status: coalesce(call.Status, pairCall.Status),
				Result: coalesce(call.Result, pairCall.Result),
			},
		}

		newItems = append(newItems, mergedItem)

		processed[i] = true
		processed[pairIndex] = true
	}

	return newItems, nil
}

func toUserRoleInputItems(msg *schema.AgenticMessage) (items []responses.ResponseInputItemUnionParam, err error) {
	items = make([]responses.ResponseInputItemUnionParam, 0, len(msg.ContentBlocks))

	for _, block := range msg.ContentBlocks {
		var item responses.ResponseInputItemUnionParam

		switch block.Type {
		case schema.ContentBlockTypeUserInputText:
			item, err = userInputTextToInputItem(responses.EasyInputMessageRoleUser, block.UserInputText)
			if err != nil {
				return nil, fmt.Errorf("failed to convert user input text to input item: %w", err)
			}

		case schema.ContentBlockTypeUserInputImage:
			item, err = userInputImageToInputItem(responses.EasyInputMessageRoleUser, block.UserInputImage)
			if err != nil {
				return nil, fmt.Errorf("failed to convert user input image to input item: %w", err)
			}

		case schema.ContentBlockTypeUserInputFile:
			item, err = userInputFileToInputItem(responses.EasyInputMessageRoleUser, block.UserInputFile)
			if err != nil {
				return nil, fmt.Errorf("failed to convert user input file to input item: %w", err)
			}

		case schema.ContentBlockTypeFunctionToolResult:
			item, err = functionToolResultToInputItem(block.FunctionToolResult)
			if err != nil {
				return nil, fmt.Errorf("failed to convert function tool result to input item: %w", err)
			}

		case schema.ContentBlockTypeToolSearchResult:
			item, err = toolSearchToolResultToInputItem(block.ToolSearchFunctionToolResult)
			if err != nil {
				return nil, fmt.Errorf("failed to convert tool search function tool result to input item: %w", err)
			}

		case schema.ContentBlockTypeMCPToolApprovalResponse:
			item, err = mcpToolApprovalResponseToInputItem(block.MCPToolApprovalResponse)
			if err != nil {
				return nil, fmt.Errorf("failed to convert MCP tool approval response to input item: %w", err)
			}

		default:
			return nil, fmt.Errorf("invalid content block type %q with user role", block.Type)
		}

		items = append(items, item)
	}

	return items, nil
}

func userInputTextToInputItem(role responses.EasyInputMessageRole, block *schema.UserInputText) (item responses.ResponseInputItemUnionParam, err error) {
	item = responses.ResponseInputItemUnionParam{
		OfMessage: &responses.EasyInputMessageParam{
			Role: role,
			Content: responses.EasyInputMessageContentUnionParam{
				OfString: param.NewOpt(block.Text),
			},
		},
	}

	return item, nil
}

func userInputImageToInputItem(role responses.EasyInputMessageRole, block *schema.UserInputImage) (item responses.ResponseInputItemUnionParam, err error) {
	imageURL, err := resolveURL(block.URL, block.Base64Data, block.MIMEType)
	if err != nil {
		return item, err
	}

	detail, err := toInputItemImageDetail(block.Detail)
	if err != nil {
		return item, err
	}

	contentItem := responses.ResponseInputContentUnionParam{
		OfInputImage: &responses.ResponseInputImageParam{
			ImageURL: newOpenaiStrOpt(imageURL),
			Detail:   detail,
		},
	}

	msgItem := &responses.EasyInputMessageParam{
		Role: role,
		Content: responses.EasyInputMessageContentUnionParam{
			OfInputItemContentList: []responses.ResponseInputContentUnionParam{
				contentItem,
			},
		},
	}

	item = responses.ResponseInputItemUnionParam{
		OfMessage: msgItem,
	}

	return item, nil
}

func toInputItemImageDetail(detail schema.ImageURLDetail) (responses.ResponseInputImageDetail, error) {
	if detail == "" {
		return "", nil
	}
	switch detail {
	case schema.ImageURLDetailHigh:
		return responses.ResponseInputImageDetailHigh, nil
	case schema.ImageURLDetailLow:
		return responses.ResponseInputImageDetailLow, nil
	case schema.ImageURLDetailAuto:
		return responses.ResponseInputImageDetailAuto, nil
	default:
		return "", fmt.Errorf("invalid image detail: %s", detail)
	}
}

func userInputFileToInputItem(role responses.EasyInputMessageRole, block *schema.UserInputFile) (item responses.ResponseInputItemUnionParam, err error) {
	fileURl, err := resolveURL(block.URL, block.Base64Data, block.MIMEType)
	if err != nil {
		return item, err
	}

	contentItem := responses.ResponseInputContentUnionParam{
		OfInputFile: &responses.ResponseInputFileParam{
			Filename: newOpenaiStrOpt(block.Name),
		},
	}
	if block.URL != "" {
		contentItem.OfInputFile.FileURL = newOpenaiStrOpt(fileURl)
	} else if block.Base64Data != "" {
		contentItem.OfInputFile.FileData = newOpenaiStrOpt(block.Base64Data)
	}

	item = responses.ResponseInputItemUnionParam{
		OfMessage: &responses.EasyInputMessageParam{
			Role: role,
			Content: responses.EasyInputMessageContentUnionParam{
				OfInputItemContentList: []responses.ResponseInputContentUnionParam{
					contentItem,
				},
			},
		},
	}

	return item, nil
}

func toolSearchToolResultToInputItem(block *schema.ToolSearchFunctionToolResult) (item responses.ResponseInputItemUnionParam, err error) {
	if block.Result == nil {
		return item, fmt.Errorf("tool search result should not be nil")
	}
	tools, err := toDeferredFunctionTools(block.Result.Tools)
	if err != nil {
		return item, err
	}
	item = responses.ResponseInputItemUnionParam{
		OfToolSearchOutput: &responses.ResponseToolSearchOutputItemParam{
			Tools:     tools,
			CallID:    param.NewOpt(block.CallID),
			Status:    responses.ResponseToolSearchOutputItemParamStatusCompleted,
			Execution: responses.ResponseToolSearchOutputItemParamExecutionClient,
		},
	}
	return item, nil
}

func functionToolResultToInputItem(block *schema.FunctionToolResult) (item responses.ResponseInputItemUnionParam, err error) {
	item = responses.ResponseInputItemUnionParam{
		OfFunctionCallOutput: &responses.ResponseInputItemFunctionCallOutputParam{
			CallID: block.CallID,
			Output: responses.ResponseInputItemFunctionCallOutputOutputUnionParam{
				OfString: param.NewOpt(block.Result),
			},
		},
	}

	return item, nil
}

func assistantGenTextToInputItem(block *schema.ContentBlock) (item responses.ResponseInputItemUnionParam, err error) {
	content := block.AssistantGenText
	if content == nil {
		return item, fmt.Errorf("assistant generated text is nil")
	}

	var annotations []responses.ResponseOutputTextAnnotationUnionParam
	if content.OpenAIExtension != nil {
		annotations = make([]responses.ResponseOutputTextAnnotationUnionParam, 0, len(content.OpenAIExtension.Annotations))
		for _, anno := range content.OpenAIExtension.Annotations {
			if anno == nil {
				return item, fmt.Errorf("text annotation is nil")
			}
			anno_, err := textAnnotationToOutputTextAnnotation(anno)
			if err != nil {
				return item, fmt.Errorf("failed to convert text annotation to output text annotation: %w", err)
			}
			annotations = append(annotations, anno_)
		}
	}

	id, _ := getItemID(block)
	status, _ := GetItemStatus(block)

	contentItem := responses.ResponseOutputMessageContentUnionParam{
		OfOutputText: &responses.ResponseOutputTextParam{
			Annotations: annotations,
			Text:        content.Text,
		},
	}

	item = responses.ResponseInputItemUnionParam{
		OfOutputMessage: &responses.ResponseOutputMessageParam{
			ID:      id,
			Status:  responses.ResponseOutputMessageStatus(status),
			Content: []responses.ResponseOutputMessageContentUnionParam{contentItem},
		},
	}

	return item, nil
}

func textAnnotationToOutputTextAnnotation(annotation *openai.TextAnnotation) (param responses.ResponseOutputTextAnnotationUnionParam, err error) {
	switch annotation.Type {
	case openai.TextAnnotationTypeFileCitation:
		citation := annotation.FileCitation
		if citation == nil {
			return param, fmt.Errorf("file citation is nil")
		}
		return responses.ResponseOutputTextAnnotationUnionParam{
			OfFileCitation: &responses.ResponseOutputTextAnnotationFileCitationParam{
				Index:    int64(citation.Index),
				FileID:   citation.FileID,
				Filename: citation.Filename,
			},
		}, nil

	case openai.TextAnnotationTypeURLCitation:
		citation := annotation.URLCitation
		if citation == nil {
			return param, fmt.Errorf("url citation is nil")
		}
		return responses.ResponseOutputTextAnnotationUnionParam{
			OfURLCitation: &responses.ResponseOutputTextAnnotationURLCitationParam{
				Title:      citation.Title,
				URL:        citation.URL,
				StartIndex: int64(citation.StartIndex),
				EndIndex:   int64(citation.EndIndex),
			},
		}, nil

	case openai.TextAnnotationTypeContainerFileCitation:
		citation := annotation.ContainerFileCitation
		if citation == nil {
			return param, fmt.Errorf("container file citation is nil")
		}
		return responses.ResponseOutputTextAnnotationUnionParam{
			OfContainerFileCitation: &responses.ResponseOutputTextAnnotationContainerFileCitationParam{
				ContainerID: citation.ContainerID,
				StartIndex:  int64(citation.StartIndex),
				EndIndex:    int64(citation.EndIndex),
				FileID:      citation.FileID,
				Filename:    citation.Filename,
			},
		}, nil

	case openai.TextAnnotationTypeFilePath:
		filePath := annotation.FilePath
		if filePath == nil {
			return param, fmt.Errorf("file path is nil")
		}
		return responses.ResponseOutputTextAnnotationUnionParam{
			OfFilePath: &responses.ResponseOutputTextAnnotationFilePathParam{
				FileID: filePath.FileID,
				Index:  int64(filePath.Index),
			},
		}, nil

	default:
		return param, fmt.Errorf("invalid text annotation type: %s", annotation.Type)
	}
}

func functionToolCallToInputItem(block *schema.ContentBlock) (item responses.ResponseInputItemUnionParam, err error) {
	content := block.FunctionToolCall
	if content == nil {
		return item, fmt.Errorf("function tool call is nil")
	}

	id, _ := getItemID(block)
	status, _ := GetItemStatus(block)
	namespace, _ := getNamespace(block)

	if GetToolSearchToolCall(block) {
		// client tool search call
		item = responses.ResponseInputItemUnionParam{
			OfToolSearchCall: &responses.ResponseInputItemToolSearchCallParam{
				Arguments: json.RawMessage(content.Arguments),
				ID:        newOpenaiStrOpt(id),
				CallID:    param.NewOpt(content.CallID),
				Status:    status,
				Execution: "client",
			},
		}
		return item, nil
	}

	item = responses.ResponseInputItemUnionParam{
		OfFunctionCall: &responses.ResponseFunctionToolCallParam{
			ID:        newOpenaiStrOpt(id),
			Status:    responses.ResponseFunctionToolCallStatus(status),
			CallID:    content.CallID,
			Name:      content.Name,
			Arguments: content.Arguments,
			Namespace: newOpenaiStrOpt(namespace),
		},
	}

	return item, nil
}

func reasoningToInputItem(block *schema.ContentBlock) (item responses.ResponseInputItemUnionParam, err error) {
	content := block.Reasoning
	if content == nil {
		return item, fmt.Errorf("reasoning is nil")
	}

	id, _ := getItemID(block)
	status, _ := GetItemStatus(block)

	item = responses.ResponseInputItemUnionParam{
		OfReasoning: &responses.ResponseReasoningItemParam{
			ID:     id,
			Status: responses.ResponseReasoningItemStatus(status),
			Summary: []responses.ResponseReasoningItemSummaryParam{
				{Text: content.Text},
			},
			EncryptedContent: newOpenaiStrOpt(content.Signature),
		},
	}

	return item, nil
}

func serverToolCallToInputItem(block *schema.ContentBlock) (item responses.ResponseInputItemUnionParam, err error) {
	content := block.ServerToolCall
	if content == nil {
		return item, fmt.Errorf("server tool call is nil")
	}

	arguments, err := getServerToolCallArguments(content)
	if err != nil {
		return item, err
	}

	switch {
	case arguments.WebSearch != nil:
		return webSearchToolCallToInputItem(arguments.WebSearch, block)
	case arguments.FileSearch != nil:
		return fileSearchToolCallToInputItem(arguments.FileSearch, block)
	case arguments.CodeInterpreter != nil:
		return codeInterpreterToolCallToInputItem(arguments.CodeInterpreter, block)
	case arguments.ImageGeneration != nil:
		return imageGenerationToolCallToInputItem(arguments.ImageGeneration, block)
	case arguments.Shell != nil:
		return shellToolCallToInputItem(arguments.Shell, block)
	case arguments.ToolSearch != nil:
		id, _ := getItemID(block)
		status, _ := GetItemStatus(block)
		var action *responses.ResponseInputItemToolSearchCallParam
		action, err = getToolSearchCallActionParam(arguments.ToolSearch)
		if err != nil {
			return item, err
		}
		action.CallID = newOpenaiStrOpt(content.CallID)
		action.ID = param.NewOpt(id)
		action.Status = status
		return responses.ResponseInputItemUnionParam{
			OfToolSearchCall: action,
		}, nil
	default:
		return item, fmt.Errorf("server tool call arguments are nil")
	}
}

func webSearchToolCallToInputItem(args *WebSearchArguments, block *schema.ContentBlock) (item responses.ResponseInputItemUnionParam, err error) {
	id, _ := getItemID(block)
	status, _ := GetItemStatus(block)

	action, err := getWebSearchToolCallActionParam(args)
	if err != nil {
		return item, err
	}

	item = responses.ResponseInputItemUnionParam{
		OfWebSearchCall: &responses.ResponseFunctionWebSearchParam{
			ID:     id,
			Status: responses.ResponseFunctionWebSearchStatus(status),
			Action: action,
		},
	}

	return item, nil
}

func getWebSearchToolCallActionParam(ws *WebSearchArguments) (action responses.ResponseFunctionWebSearchActionUnionParam, err error) {
	switch ws.ActionType {
	case WebSearchActionSearch:
		return responses.ResponseFunctionWebSearchActionUnionParam{
			OfSearch: &responses.ResponseFunctionWebSearchActionSearchParam{
				Queries: ws.Search.Queries,
			},
		}, nil

	case WebSearchActionOpenPage:
		return responses.ResponseFunctionWebSearchActionUnionParam{
			OfOpenPage: &responses.ResponseFunctionWebSearchActionOpenPageParam{
				URL: newOpenaiStrOpt(ws.OpenPage.URL),
			},
		}, nil

	case WebSearchActionFind:
		return responses.ResponseFunctionWebSearchActionUnionParam{
			OfFind: &responses.ResponseFunctionWebSearchActionFindParam{
				URL:     ws.Find.URL,
				Pattern: ws.Find.Pattern,
			},
		}, nil

	default:
		return action, fmt.Errorf("unknown web search action type: %s", ws.ActionType)
	}
}

func fileSearchToolCallToInputItem(args *FileSearchArguments, block *schema.ContentBlock) (item responses.ResponseInputItemUnionParam, err error) {
	id, _ := getItemID(block)
	status, _ := GetItemStatus(block)

	item = responses.ResponseInputItemUnionParam{
		OfFileSearchCall: &responses.ResponseFileSearchToolCallParam{
			ID:      id,
			Status:  responses.ResponseFileSearchToolCallStatus(status),
			Queries: args.Queries,
		},
	}

	return item, nil
}

func codeInterpreterToolCallToInputItem(args *CodeInterpreterArguments, block *schema.ContentBlock) (item responses.ResponseInputItemUnionParam, err error) {
	id, _ := getItemID(block)
	status, _ := GetItemStatus(block)

	item = responses.ResponseInputItemUnionParam{
		OfCodeInterpreterCall: &responses.ResponseCodeInterpreterToolCallParam{
			ID:     id,
			Status: responses.ResponseCodeInterpreterToolCallStatus(status),
		},
	}

	return item, nil
}

func imageGenerationToolCallToInputItem(args *ImageGenerationArguments, block *schema.ContentBlock) (item responses.ResponseInputItemUnionParam, err error) {
	id, _ := getItemID(block)
	status, _ := GetItemStatus(block)

	item = responses.ResponseInputItemUnionParam{
		OfImageGenerationCall: &responses.ResponseInputItemImageGenerationCallParam{
			ID:     id,
			Status: status,
		},
	}

	return item, nil
}

func shellToolCallToInputItem(args *ShellArguments, block *schema.ContentBlock) (item responses.ResponseInputItemUnionParam, err error) {
	id, _ := getItemID(block)
	status, _ := GetItemStatus(block)
	callID := block.ServerToolCall.CallID

	action := responses.ResponseInputItemShellCallActionParam{
		Commands: args.Action.Commands,
	}
	if args.Action.TimeoutMs > 0 {
		action.TimeoutMs = param.NewOpt(args.Action.TimeoutMs)
	}
	if args.Action.MaxOutputLength > 0 {
		action.MaxOutputLength = param.NewOpt(args.Action.MaxOutputLength)
	}

	var env responses.ResponseInputItemShellCallEnvironmentUnionParam
	if args.Environment != nil {
		switch args.Environment.Type {
		case ShellEnvironmentTypeLocal:
			skills := make([]responses.LocalSkillParam, 0, len(args.Environment.Local.Skills))
			for _, s := range args.Environment.Local.Skills {
				skills = append(skills, responses.LocalSkillParam{
					Name:        s.Name,
					Description: s.Description,
					Path:        s.Path,
				})
			}
			env = responses.ResponseInputItemShellCallEnvironmentUnionParam{
				OfLocal: &responses.LocalEnvironmentParam{
					Skills: skills,
				},
			}
		case ShellEnvironmentTypeContainerReference:
			env = responses.ResponseInputItemShellCallEnvironmentUnionParam{
				OfContainerReference: &responses.ContainerReferenceParam{
					ContainerID: args.Environment.ContainerReference.ContainerID,
				},
			}
		default:
			return item, fmt.Errorf("unknown shell environment type: %s", args.Environment.Type)
		}
	}

	item = responses.ResponseInputItemUnionParam{
		OfShellCall: &responses.ResponseInputItemShellCallParam{
			ID:          newOpenaiStrOpt(id),
			CallID:      callID,
			Status:      status,
			Action:      action,
			Environment: env,
		},
	}

	return item, nil
}

func getToolSearchCallActionParam(ts *ToolSearchCall) (*responses.ResponseInputItemToolSearchCallParam, error) {
	return &responses.ResponseInputItemToolSearchCallParam{
		Arguments: ts.Arguments,
		Execution: "server",
	}, nil
}

func serverToolResultToInputItem(block *schema.ContentBlock) (item responses.ResponseInputItemUnionParam, err error) {
	content := block.ServerToolResult
	if content == nil {
		return item, fmt.Errorf("server tool result is nil")
	}

	result, err := getServerToolResult(content)
	if err != nil {
		return item, err
	}

	switch {
	case result.WebSearch != nil:
		return webSearchToolResultToInputItem(result.WebSearch, block)
	case result.FileSearch != nil:
		return fileSearchToolResultToInputItem(result.FileSearch, block)
	case result.CodeInterpreter != nil:
		return codeInterpreterToolResultToInputItem(result.CodeInterpreter, block)
	case result.ImageGeneration != nil:
		return imageGenerationToolResultToInputItem(result.ImageGeneration, block)
	case result.Shell != nil:
		return shellToolResultToInputItem(result.Shell, block)
	case result.ToolSearch != nil:
		id, _ := getItemID(block)
		status, _ := GetItemStatus(block)
		var action *responses.ResponseToolSearchOutputItemParam
		action, err = getToolSearchResultActionParam(result.ToolSearch)
		if err != nil {
			return item, err
		}
		action.CallID = newOpenaiStrOpt(content.CallID)
		action.ID = param.NewOpt(id)
		action.Status = responses.ResponseToolSearchOutputItemParamStatus(status)

		return responses.ResponseInputItemUnionParam{
			OfToolSearchOutput: action,
		}, nil
	default:
		return item, fmt.Errorf("server tool result is nil")
	}
}

func webSearchToolResultToInputItem(result *WebSearchResult, block *schema.ContentBlock) (item responses.ResponseInputItemUnionParam, err error) {
	id, _ := getItemID(block)
	status, _ := GetItemStatus(block)

	action, err := getWebSearchToolResultActionParam(result)
	if err != nil {
		return item, err
	}

	item = responses.ResponseInputItemUnionParam{
		OfWebSearchCall: &responses.ResponseFunctionWebSearchParam{
			ID:     id,
			Status: responses.ResponseFunctionWebSearchStatus(status),
			Action: action,
		},
	}

	return item, nil
}

func getWebSearchToolResultActionParam(ws *WebSearchResult) (action responses.ResponseFunctionWebSearchActionUnionParam, err error) {
	switch ws.ActionType {
	case WebSearchActionSearch:
		sources := make([]responses.ResponseFunctionWebSearchActionSearchSourceParam, 0, len(ws.Search.Sources))
		for _, s := range ws.Search.Sources {
			sources = append(sources, responses.ResponseFunctionWebSearchActionSearchSourceParam{
				URL: s.URL,
			})
		}
		return responses.ResponseFunctionWebSearchActionUnionParam{
			OfSearch: &responses.ResponseFunctionWebSearchActionSearchParam{
				Sources: sources,
			},
		}, nil

	default:
		return action, fmt.Errorf("unknown web search result action type: %s", ws.ActionType)
	}
}

func fileSearchToolResultToInputItem(fs *FileSearchResult, block *schema.ContentBlock) (item responses.ResponseInputItemUnionParam, err error) {
	id, _ := getItemID(block)
	status, _ := GetItemStatus(block)

	results := make([]responses.ResponseFileSearchToolCallResultParam, 0, len(fs.Results))
	for _, r := range fs.Results {
		attrs := make(map[string]responses.ResponseFileSearchToolCallResultAttributeUnionParam, len(r.Attributes))
		for k, v := range r.Attributes {
			attrs[k] = responses.ResponseFileSearchToolCallResultAttributeUnionParam{
				OfString: newOpenaiOpt(v.OfString),
				OfFloat:  newOpenaiOpt(v.OfFloat),
				OfBool:   newOpenaiOpt(v.OfBool),
			}
		}

		results = append(results, responses.ResponseFileSearchToolCallResultParam{
			FileID:     newOpenaiStrOpt(r.FileID),
			Filename:   newOpenaiStrOpt(r.FileName),
			Score:      param.NewOpt(r.Score),
			Text:       newOpenaiStrOpt(r.Text),
			Attributes: attrs,
		})
	}

	item = responses.ResponseInputItemUnionParam{
		OfFileSearchCall: &responses.ResponseFileSearchToolCallParam{
			ID:      id,
			Status:  responses.ResponseFileSearchToolCallStatus(status),
			Results: results,
		},
	}

	return item, nil
}

func codeInterpreterToolResultToInputItem(result *CodeInterpreterResult, block *schema.ContentBlock) (item responses.ResponseInputItemUnionParam, err error) {
	id, _ := getItemID(block)
	status, _ := GetItemStatus(block)

	outputs := make([]responses.ResponseCodeInterpreterToolCallOutputUnionParam, 0, len(result.Outputs))
	for _, o := range result.Outputs {
		switch o.Type {
		case CodeInterpreterOutputTypeLogs:
			if o.Logs != nil {
				outputs = append(outputs, responses.ResponseCodeInterpreterToolCallOutputUnionParam{
					OfLogs: &responses.ResponseCodeInterpreterToolCallOutputLogsParam{
						Logs: o.Logs.Logs,
					},
				})
			}
		case CodeInterpreterOutputTypeImage:
			if o.Image != nil {
				outputs = append(outputs, responses.ResponseCodeInterpreterToolCallOutputUnionParam{
					OfImage: &responses.ResponseCodeInterpreterToolCallOutputImageParam{
						URL: o.Image.URL,
					},
				})
			}
		default:
			return item, fmt.Errorf("unknown code interpreter output type: %s", o.Type)
		}
	}

	item = responses.ResponseInputItemUnionParam{
		OfCodeInterpreterCall: &responses.ResponseCodeInterpreterToolCallParam{
			ID:          id,
			ContainerID: result.ContainerID,
			Status:      responses.ResponseCodeInterpreterToolCallStatus(status),
			Code:        newOpenaiStrOpt(result.Code),
			Outputs:     outputs,
		},
	}

	return item, nil
}

func imageGenerationToolResultToInputItem(result *ImageGenerationResult, block *schema.ContentBlock) (item responses.ResponseInputItemUnionParam, err error) {
	id, _ := getItemID(block)
	status, _ := GetItemStatus(block)

	item = responses.ResponseInputItemUnionParam{
		OfImageGenerationCall: &responses.ResponseInputItemImageGenerationCallParam{
			ID:     id,
			Status: status,
			Result: newOpenaiStrOpt(result.ImageBase64),
		},
	}

	return item, nil
}

func shellToolResultToInputItem(result *ShellResult, block *schema.ContentBlock) (item responses.ResponseInputItemUnionParam, err error) {
	id, _ := getItemID(block)
	status, _ := GetItemStatus(block)
	callID := block.ServerToolResult.CallID

	output := make([]responses.ResponseFunctionShellCallOutputContentParam, 0, len(result.Outputs))
	for _, o := range result.Outputs {
		var outcome responses.ResponseFunctionShellCallOutputContentOutcomeUnionParam
		if o.Outcome != nil {
			switch o.Outcome.Type {
			case ShellOutputOutcomeTypeTimeout:
				outcome = responses.ResponseFunctionShellCallOutputContentOutcomeUnionParam{
					OfTimeout: &responses.ResponseFunctionShellCallOutputContentOutcomeTimeoutParam{},
				}
			case ShellOutputOutcomeTypeExit:
				if o.Outcome.Exit != nil {
					outcome = responses.ResponseFunctionShellCallOutputContentOutcomeUnionParam{
						OfExit: &responses.ResponseFunctionShellCallOutputContentOutcomeExitParam{
							ExitCode: o.Outcome.Exit.ExitCode,
						},
					}
				}
			default:
				return item, fmt.Errorf("unknown shell output outcome type: %s", o.Outcome.Type)
			}
		}

		output = append(output, responses.ResponseFunctionShellCallOutputContentParam{
			Stdout:  o.Stdout,
			Stderr:  o.Stderr,
			Outcome: outcome,
		})
	}

	shellOutput := &responses.ResponseInputItemShellCallOutputParam{
		ID:     newOpenaiStrOpt(id),
		CallID: callID,
		Status: status,
		Output: output,
	}
	if result.MaxOutputLength > 0 {
		shellOutput.MaxOutputLength = param.NewOpt(result.MaxOutputLength)
	}

	item = responses.ResponseInputItemUnionParam{
		OfShellCallOutput: shellOutput,
	}

	return item, nil
}

func getToolSearchResultActionParam(ts *ToolSearchResult) (*responses.ResponseToolSearchOutputItemParam, error) {
	tools, err := toDeferredFunctionTools(ts.Tools)
	if err != nil {
		return nil, err
	}
	return &responses.ResponseToolSearchOutputItemParam{
		Tools:     tools,
		Execution: "server",
	}, nil
}

func mcpToolApprovalRequestToInputItem(block *schema.ContentBlock) (item responses.ResponseInputItemUnionParam, err error) {
	content := block.MCPToolApprovalRequest
	if content == nil {
		return item, fmt.Errorf("mcp tool approval request is nil")
	}

	id, _ := getItemID(block)

	item = responses.ResponseInputItemUnionParam{
		OfMcpApprovalRequest: &responses.ResponseInputItemMcpApprovalRequestParam{
			ID:          id,
			ServerLabel: content.ServerLabel,
			Name:        content.Name,
			Arguments:   content.Arguments,
		},
	}

	return item, nil
}

func mcpToolApprovalResponseToInputItem(block *schema.MCPToolApprovalResponse) (item responses.ResponseInputItemUnionParam, err error) {
	item = responses.ResponseInputItemUnionParam{
		OfMcpApprovalResponse: &responses.ResponseInputItemMcpApprovalResponseParam{
			ApprovalRequestID: block.ApprovalRequestID,
			Approve:           block.Approve,
			Reason:            newOpenaiStrOpt(block.Reason),
		},
	}

	return item, nil
}

func mcpListToolsResultToInputItem(block *schema.ContentBlock) (item responses.ResponseInputItemUnionParam, err error) {
	content := block.MCPListToolsResult
	if content == nil {
		return item, fmt.Errorf("mcp list tools result is nil")
	}

	tools := make([]responses.ResponseInputItemMcpListToolsToolParam, 0, len(content.Tools))
	for i := range content.Tools {
		tool := content.Tools[i]

		tools = append(tools, responses.ResponseInputItemMcpListToolsToolParam{
			Name:        tool.Name,
			Description: newOpenaiStrOpt(tool.Description),
			InputSchema: tool.InputSchema,
		})
	}

	id, _ := getItemID(block)

	item = responses.ResponseInputItemUnionParam{
		OfMcpListTools: &responses.ResponseInputItemMcpListToolsParam{
			ID:          id,
			ServerLabel: content.ServerLabel,
			Tools:       tools,
			Error:       newOpenaiStrOpt(content.Error),
		},
	}

	return item, nil
}

func mcpToolCallToInputItem(block *schema.ContentBlock) (item responses.ResponseInputItemUnionParam, err error) {
	content := block.MCPToolCall
	if content == nil {
		return item, fmt.Errorf("mcp tool call is nil")
	}

	id, _ := getItemID(block)
	status, _ := GetItemStatus(block)

	item = responses.ResponseInputItemUnionParam{
		OfMcpCall: &responses.ResponseInputItemMcpCallParam{
			ID:                id,
			ApprovalRequestID: newOpenaiStrOpt(content.ApprovalRequestID),
			ServerLabel:       content.ServerLabel,
			Arguments:         content.Arguments,
			Name:              content.Name,
			Status:            status,
		},
	}

	return item, nil
}

func mcpToolResultToInputItem(block *schema.ContentBlock) (item responses.ResponseInputItemUnionParam, err error) {
	content := block.MCPToolResult
	if content == nil {
		return item, fmt.Errorf("MCP tool result is nil")
	}

	id, _ := getItemID(block)
	status, _ := GetItemStatus(block)

	var errorMsg string
	if content.Error != nil {
		errorMsg = content.Error.Message
	}

	item = responses.ResponseInputItemUnionParam{
		OfMcpCall: &responses.ResponseInputItemMcpCallParam{
			ID:          id,
			ServerLabel: content.ServerLabel,
			Name:        content.Name,
			Error:       newOpenaiStrOpt(errorMsg),
			Output:      newOpenaiStrOpt(content.Result),
			Status:      status,
		},
	}

	return item, nil
}

func toOutputMessage(resp *responses.Response, options *model.Options) (msg *schema.AgenticMessage, err error) {
	blocks := make([]*schema.ContentBlock, 0, len(resp.Output))

	for _, item := range resp.Output {
		var tmpBlocks []*schema.ContentBlock

		switch variant := item.AsAny().(type) {
		case responses.ResponseReasoningItem:
			block, err := reasoningToContentBlocks(variant)
			if err != nil {
				return nil, fmt.Errorf("failed to convert reasoning to content block: %w", err)
			}

			tmpBlocks = append(tmpBlocks, block)

		case responses.ResponseOutputMessage:
			tmpBlocks, err = outputMessageToContentBlocks(variant)
			if err != nil {
				return nil, fmt.Errorf("failed to convert output message to content blocks: %w", err)
			}

		case responses.ResponseFunctionToolCall:
			block, err := functionToolCallToContentBlock(variant)
			if err != nil {
				return nil, fmt.Errorf("failed to convert function tool call to content block: %w", err)
			}

			tmpBlocks = append(tmpBlocks, block)

		case responses.ResponseOutputItemMcpListTools:
			block, err := mcpListToolsToContentBlock(variant)
			if err != nil {
				return nil, fmt.Errorf("failed to convert MCP list tools to content block: %w", err)
			}

			tmpBlocks = append(tmpBlocks, block)

		case responses.ResponseOutputItemMcpCall:
			tmpBlocks, err = mcpCallToContentBlocks(variant)
			if err != nil {
				return nil, fmt.Errorf("failed to convert MCP call to content block: %w", err)
			}

		case responses.ResponseOutputItemMcpApprovalRequest:
			block, err := mcpApprovalRequestToContentBlock(variant)
			if err != nil {
				return nil, fmt.Errorf("failed to convert MCP approval request to content block: %w", err)
			}

			tmpBlocks = append(tmpBlocks, block)

		case responses.ResponseFunctionWebSearch:
			tmpBlocks, err = webSearchToContentBlocks(variant)
			if err != nil {
				return nil, fmt.Errorf("failed to convert web search to content block: %w", err)
			}

		case responses.ResponseFileSearchToolCall:
			tmpBlocks, err = fileSearchToContentBlocks(variant)
			if err != nil {
				return nil, fmt.Errorf("failed to convert file search to content block: %w", err)
			}

		case responses.ResponseCodeInterpreterToolCall:
			tmpBlocks, err = codeInterpreterToContentBlocks(variant)
			if err != nil {
				return nil, fmt.Errorf("failed to convert code interpreter to content block: %w", err)
			}

		case responses.ResponseOutputItemImageGenerationCall:
			tmpBlocks, err = imageGenerationToContentBlocks(variant)
			if err != nil {
				return nil, fmt.Errorf("failed to convert image generation to content block: %w", err)
			}

		case responses.ResponseFunctionShellToolCall:
			block, err := shellCallToContentBlock(variant)
			if err != nil {
				return nil, fmt.Errorf("failed to convert function shell call to content block: %w", err)
			}
			tmpBlocks = []*schema.ContentBlock{block}

		case responses.ResponseFunctionShellToolCallOutput:
			block, err := shellOutputToContentBlock(variant)
			if err != nil {
				return nil, fmt.Errorf("failed to convert function shell output to content block: %w", err)
			}
			tmpBlocks = []*schema.ContentBlock{block}
		case responses.ResponseToolSearchCall:
			block, err := toolSearchToolCallToContentBlock(variant, options)
			if err != nil {
				return nil, fmt.Errorf("failed to convert tool search call to content block: %w", err)
			}
			tmpBlocks = append(tmpBlocks, block)

		case responses.ResponseToolSearchOutputItem:
			block, err := toolSearchToolResultToContentBlock(variant)
			if err != nil {
				return nil, fmt.Errorf("failed to convert tool search output item to content block: %w", err)
			}
			tmpBlocks = append(tmpBlocks, block)

		default:
			return nil, fmt.Errorf("unknown output item type: %T", variant)
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

func outputMessageToContentBlocks(item responses.ResponseOutputMessage) (blocks []*schema.ContentBlock, err error) {
	blocks = make([]*schema.ContentBlock, 0, len(item.Content))

	for _, content := range item.Content {
		var block *schema.ContentBlock

		switch variant := content.AsAny().(type) {
		case responses.ResponseOutputText:
			block, err = outputContentTextToContentBlock(variant)
			if err != nil {
				return nil, fmt.Errorf("failed to convert output text to content block: %w", err)
			}

		case responses.ResponseOutputRefusal:
			block = schema.NewContentBlock(&schema.AssistantGenText{
				OpenAIExtension: &openai.AssistantGenTextExtension{
					Refusal: &openai.OutputRefusal{
						Reason: variant.Refusal,
					},
				},
			})

		default:
			return nil, fmt.Errorf("unknown output message content type: %s", content.Type)
		}

		setItemID(block, item.ID)
		if s := string(item.Status); s != "" {
			setItemStatus(block, s)
		}

		blocks = append(blocks, block)
	}

	return blocks, nil
}

func outputContentTextToContentBlock(text responses.ResponseOutputText) (block *schema.ContentBlock, err error) {
	annotations := make([]*openai.TextAnnotation, 0, len(text.Annotations))
	for _, union := range text.Annotations {
		anno, err := outputTextAnnotationToTextAnnotation(union)
		if err != nil {
			return nil, fmt.Errorf("failed to convert text annotation: %w", err)
		}
		annotations = append(annotations, anno)
	}

	block = schema.NewContentBlock(&schema.AssistantGenText{
		Text: text.Text,
		OpenAIExtension: &openai.AssistantGenTextExtension{
			Annotations: annotations,
		},
	})

	return block, nil
}

func outputTextAnnotationToTextAnnotation(anno responses.ResponseOutputTextAnnotationUnion) (*openai.TextAnnotation, error) {
	switch variant := anno.AsAny().(type) {
	case responses.ResponseOutputTextAnnotationFileCitation:
		return &openai.TextAnnotation{
			Type: openai.TextAnnotationTypeFileCitation,
			FileCitation: &openai.TextAnnotationFileCitation{
				Index:    int(variant.Index),
				FileID:   variant.FileID,
				Filename: variant.Filename,
			},
		}, nil

	case responses.ResponseOutputTextAnnotationURLCitation:
		return &openai.TextAnnotation{
			Type: openai.TextAnnotationTypeURLCitation,
			URLCitation: &openai.TextAnnotationURLCitation{
				Title:      variant.Title,
				URL:        variant.URL,
				StartIndex: int(variant.StartIndex),
				EndIndex:   int(variant.EndIndex),
			},
		}, nil

	case responses.ResponseOutputTextAnnotationContainerFileCitation:
		return &openai.TextAnnotation{
			Type: openai.TextAnnotationTypeContainerFileCitation,
			ContainerFileCitation: &openai.TextAnnotationContainerFileCitation{
				ContainerID: variant.ContainerID,
				FileID:      variant.FileID,
				Filename:    variant.Filename,
				StartIndex:  int(variant.StartIndex),
				EndIndex:    int(variant.EndIndex),
			},
		}, nil

	case responses.ResponseOutputTextAnnotationFilePath:
		return &openai.TextAnnotation{
			Type: openai.TextAnnotationTypeFilePath,
			FilePath: &openai.TextAnnotationFilePath{
				FileID: variant.FileID,
				Index:  int(variant.Index),
			},
		}, nil

	default:
		return nil, fmt.Errorf("unknown annotation type: %s", anno.Type)
	}
}

func functionToolCallToContentBlock(item responses.ResponseFunctionToolCall) (block *schema.ContentBlock, err error) {
	block = schema.NewContentBlock(&schema.FunctionToolCall{
		CallID:    item.CallID,
		Name:      item.Name,
		Arguments: item.Arguments,
	})

	setItemID(block, item.ID)
	if s := string(item.Status); s != "" {
		setItemStatus(block, s)
	}
	if len(item.Namespace) > 0 {
		setNamespace(block, item.Namespace)
	}

	return block, nil
}

func webSearchToContentBlocks(item responses.ResponseFunctionWebSearch) (blocks []*schema.ContentBlock, err error) {
	var (
		args *ServerToolCallArguments
		res  *ServerToolResult
	)

	switch variant := item.Action.AsAny().(type) {
	case responses.ResponseFunctionWebSearchActionSearch:
		args = &ServerToolCallArguments{
			WebSearch: &WebSearchArguments{
				ActionType: WebSearchActionSearch,
				Search: &WebSearchQuery{
					Queries: variant.Queries,
				},
			},
		}

		sources := make([]*WebSearchQuerySource, 0, len(variant.Sources))
		for _, src := range variant.Sources {
			sources = append(sources, &WebSearchQuerySource{
				URL: src.URL,
			})
		}
		res = &ServerToolResult{
			WebSearch: &WebSearchResult{
				ActionType: WebSearchActionSearch,
				Search: &WebSearchQueryResult{
					Sources: sources,
				},
			},
		}

	case responses.ResponseFunctionWebSearchActionOpenPage:
		args = &ServerToolCallArguments{
			WebSearch: &WebSearchArguments{
				ActionType: WebSearchActionOpenPage,
				OpenPage: &WebSearchOpenPage{
					URL: variant.URL,
				},
			},
		}

	case responses.ResponseFunctionWebSearchActionFind:
		args = &ServerToolCallArguments{
			WebSearch: &WebSearchArguments{
				ActionType: WebSearchActionFind,
				Find: &WebSearchFind{
					URL:     variant.URL,
					Pattern: variant.Pattern,
				},
			},
		}

	default:
		return nil, fmt.Errorf("unknown web search variant type: %s", item.Type)
	}

	callBlock := schema.NewContentBlock(&schema.ServerToolCall{
		Name:      string(ServerToolNameWebSearch),
		Arguments: args,
	})
	setItemID(callBlock, item.ID)
	if s := string(item.Status); s != "" {
		setItemStatus(callBlock, s)
	}

	resBlock := schema.NewContentBlock(&schema.ServerToolResult{
		Name:   string(ServerToolNameWebSearch),
		Result: res,
	})
	setItemID(resBlock, item.ID)
	if s := string(item.Status); s != "" {
		setItemStatus(resBlock, s)
	}

	blocks = []*schema.ContentBlock{callBlock, resBlock}

	return blocks, nil
}

func toolSearchToolCallToContentBlock(item responses.ResponseToolSearchCall, options *model.Options) (*schema.ContentBlock, error) {
	args, err := json.Marshal(item.Arguments)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal tool search arguments: %w", err)
	}

	switch item.Execution {
	case responses.ResponseToolSearchCallExecutionClient:
		if options.ToolSearchTool == nil {
			return nil, fmt.Errorf("haven't set client tool search tool, but get a client tool search tool call")
		}
		block := schema.NewContentBlock(&schema.FunctionToolCall{
			CallID:    item.CallID,
			Name:      options.ToolSearchTool.Name,
			Arguments: string(args),
		})
		setItemID(block, item.ID)
		setItemStatus(block, string(item.Status))
		setToolSearchToolCall(block)
		return block, nil

	case responses.ResponseToolSearchCallExecutionServer:
		// server tool call
		block := schema.NewContentBlock(&schema.ServerToolCall{
			CallID: item.CallID,
			Arguments: &ServerToolCallArguments{
				ToolSearch: &ToolSearchCall{
					Arguments: args,
				},
			},
		})
		setItemID(block, item.ID)
		setItemStatus(block, string(item.Status))
		return block, nil

	default:
		return nil, fmt.Errorf("invalid tool search execution type: %s", item.Execution)
	}
}

func toolSearchToolResultToContentBlock(item responses.ResponseToolSearchOutputItem) (blocks *schema.ContentBlock, err error) {
	tools, err := fromFunctionTools(item.Tools)
	if err != nil {
		return nil, err
	}
	block := schema.NewContentBlock(&schema.ServerToolResult{
		CallID: item.CallID,
		Result: &ServerToolResult{
			ToolSearch: &ToolSearchResult{
				Tools: tools,
			},
		},
	})
	setItemID(block, item.ID)
	setItemStatus(block, string(item.Status))
	return block, nil
}

func fileSearchToContentBlocks(item responses.ResponseFileSearchToolCall) (blocks []*schema.ContentBlock, err error) {
	args := &ServerToolCallArguments{
		FileSearch: &FileSearchArguments{
			Queries: item.Queries,
		},
	}

	results := make([]*FileSearchResultItem, 0, len(item.Results))
	for _, res := range item.Results {
		attrs := make(map[string]*FileSearchAttribute, len(res.Attributes))
		for k, v := range res.Attributes {
			switch {
			case !param.IsNull(v.OfString):
				attrs[k] = &FileSearchAttribute{
					OfString: ptrOf(v.AsString()),
				}
			case !param.IsNull(v.OfFloat):
				attrs[k] = &FileSearchAttribute{
					OfFloat: ptrOf(v.AsFloat()),
				}
			case !param.IsNull(v.OfBool):
				attrs[k] = &FileSearchAttribute{
					OfBool: ptrOf(v.AsBool()),
				}
			}
		}

		results = append(results, &FileSearchResultItem{
			FileID:     res.FileID,
			FileName:   res.Filename,
			Score:      res.Score,
			Text:       res.Text,
			Attributes: attrs,
		})
	}

	res := &ServerToolResult{
		FileSearch: &FileSearchResult{
			Results: results,
		},
	}

	callBlock := schema.NewContentBlock(&schema.ServerToolCall{
		Name:      string(ServerToolNameFileSearch),
		Arguments: args,
	})
	setItemID(callBlock, item.ID)
	if s := string(item.Status); s != "" {
		setItemStatus(callBlock, s)
	}

	resBlock := schema.NewContentBlock(&schema.ServerToolResult{
		Name:   string(ServerToolNameFileSearch),
		Result: res,
	})
	setItemID(resBlock, item.ID)
	if s := string(item.Status); s != "" {
		setItemStatus(resBlock, s)
	}

	blocks = []*schema.ContentBlock{callBlock, resBlock}

	return blocks, nil
}

func codeInterpreterToContentBlocks(item responses.ResponseCodeInterpreterToolCall) (blocks []*schema.ContentBlock, err error) {
	args := &ServerToolCallArguments{
		CodeInterpreter: &CodeInterpreterArguments{},
	}

	outputs := make([]*CodeInterpreterOutput, 0, len(item.Outputs))
	for _, out := range item.Outputs {
		switch CodeInterpreterOutputType(out.Type) {
		case "":
			// empty type in streaming events, output is nil
		case CodeInterpreterOutputTypeLogs:
			outputs = append(outputs, &CodeInterpreterOutput{
				Type: CodeInterpreterOutputTypeLogs,
				Logs: &CodeInterpreterOutputLogs{
					Logs: out.Logs,
				},
			})
		case CodeInterpreterOutputTypeImage:
			outputs = append(outputs, &CodeInterpreterOutput{
				Type: CodeInterpreterOutputTypeImage,
				Image: &CodeInterpreterOutputImage{
					URL: out.URL,
				},
			})
		default:
			return nil, fmt.Errorf("unknown code interpreter output type: %s", out.Type)
		}
	}

	res := &ServerToolResult{
		CodeInterpreter: &CodeInterpreterResult{
			Code:        item.Code,
			ContainerID: item.ContainerID,
			Outputs:     outputs,
		},
	}

	callBlock := schema.NewContentBlock(&schema.ServerToolCall{
		Name:      string(ServerToolNameCodeInterpreter),
		Arguments: args,
	})
	setItemID(callBlock, item.ID)
	if s := string(item.Status); s != "" {
		setItemStatus(callBlock, s)
	}

	resBlock := schema.NewContentBlock(&schema.ServerToolResult{
		Name:   string(ServerToolNameCodeInterpreter),
		Result: res,
	})
	setItemID(resBlock, item.ID)
	if s := string(item.Status); s != "" {
		setItemStatus(resBlock, s)
	}

	blocks = []*schema.ContentBlock{callBlock, resBlock}

	return blocks, nil
}

func imageGenerationToContentBlocks(item responses.ResponseOutputItemImageGenerationCall) (blocks []*schema.ContentBlock, err error) {
	args := &ServerToolCallArguments{
		ImageGeneration: &ImageGenerationArguments{},
	}

	res := &ServerToolResult{
		ImageGeneration: &ImageGenerationResult{
			ImageBase64: item.Result,
		},
	}

	callBlock := schema.NewContentBlock(&schema.ServerToolCall{
		Name:      string(ServerToolNameImageGeneration),
		Arguments: args,
	})
	setItemID(callBlock, item.ID)
	if item.Status != "" {
		setItemStatus(callBlock, item.Status)
	}

	resBlock := schema.NewContentBlock(&schema.ServerToolResult{
		Name:   string(ServerToolNameImageGeneration),
		Result: res,
	})
	setItemID(resBlock, item.ID)
	if item.Status != "" {
		setItemStatus(resBlock, item.Status)
	}

	blocks = []*schema.ContentBlock{callBlock, resBlock}

	return blocks, nil
}

func shellCallToContentBlock(item responses.ResponseFunctionShellToolCall) (block *schema.ContentBlock, err error) {
	var env *ShellEnvironment
	switch ShellEnvironmentType(item.Environment.Type) {
	case "":
		// empty type in streaming events, env is nil
	case ShellEnvironmentTypeLocal:
		env = &ShellEnvironment{
			Type: ShellEnvironmentTypeLocal,
		}
	case ShellEnvironmentTypeContainerReference:
		env = &ShellEnvironment{
			Type: ShellEnvironmentTypeContainerReference,
			ContainerReference: &ShellEnvironmentContainerReference{
				ContainerID: item.Environment.ContainerID,
			},
		}
	default:
		return nil, fmt.Errorf("unknown shell environment type: %s", item.Environment.Type)
	}

	var action *ShellAction
	if len(item.Action.Commands) > 0 {
		action = &ShellAction{
			Commands:        item.Action.Commands,
			TimeoutMs:       item.Action.TimeoutMs,
			MaxOutputLength: item.Action.MaxOutputLength,
		}
	}

	args := &ServerToolCallArguments{
		Shell: &ShellArguments{
			Action:      action,
			Environment: env,
			CreatedBy:   item.CreatedBy,
		},
	}

	block = schema.NewContentBlock(&schema.ServerToolCall{
		Name:      string(ServerToolNameShell),
		CallID:    item.CallID,
		Arguments: args,
	})
	setItemID(block, item.ID)
	if item.Status != "" {
		setItemStatus(block, string(item.Status))
	}

	return block, nil
}

func shellOutputToContentBlock(item responses.ResponseFunctionShellToolCallOutput) (block *schema.ContentBlock, err error) {
	outputs := make([]*ShellOutputItem, 0, len(item.Output))
	for _, o := range item.Output {
		var outcome *ShellOutputOutcome
		switch ShellOutputOutcomeType(o.Outcome.Type) {
		case "":
			// empty type in streaming events, outcome is nil
		case ShellOutputOutcomeTypeTimeout:
			outcome = &ShellOutputOutcome{
				Type: ShellOutputOutcomeTypeTimeout,
			}
		case ShellOutputOutcomeTypeExit:
			outcome = &ShellOutputOutcome{
				Type: ShellOutputOutcomeTypeExit,
				Exit: &ShellOutputOutcomeExit{
					ExitCode: o.Outcome.ExitCode,
				},
			}
		default:
			return nil, fmt.Errorf("unknown shell output outcome type: %s", o.Outcome.Type)
		}

		outputs = append(outputs, &ShellOutputItem{
			Stdout:    o.Stdout,
			Stderr:    o.Stderr,
			Outcome:   outcome,
			CreatedBy: o.CreatedBy,
		})
	}

	res := &ServerToolResult{
		Shell: &ShellResult{
			MaxOutputLength: item.MaxOutputLength,
			Outputs:         outputs,
			CreatedBy:       item.CreatedBy,
		},
	}

	block = schema.NewContentBlock(&schema.ServerToolResult{
		Name:   string(ServerToolNameShell),
		CallID: item.CallID,
		Result: res,
	})
	setItemID(block, item.ID)
	if item.Status != "" {
		setItemStatus(block, string(item.Status))
	}

	return block, nil
}

func reasoningToContentBlocks(item responses.ResponseReasoningItem) (block *schema.ContentBlock, err error) {
	var text strings.Builder
	for i, s := range item.Summary {
		if i != 0 {
			text.WriteString("\n")
		}
		text.WriteString(s.Text)
	}

	block = schema.NewContentBlock(&schema.Reasoning{
		Text: text.String(),
	})

	setItemID(block, item.ID)
	if s := string(item.Status); s != "" {
		setItemStatus(block, s)
	}

	return block, nil
}

func mcpCallToContentBlocks(item responses.ResponseOutputItemMcpCall) (blocks []*schema.ContentBlock, err error) {
	callBlock := schema.NewContentBlock(&schema.MCPToolCall{
		ServerLabel:       item.ServerLabel,
		ApprovalRequestID: item.ApprovalRequestID,
		Name:              item.Name,
		Arguments:         item.Arguments,
	})
	setItemID(callBlock, item.ID)

	resultBlock := schema.NewContentBlock(&schema.MCPToolResult{
		ServerLabel: item.ServerLabel,
		Name:        item.Name,
		Result:      item.Output,
		Error: func() *schema.MCPToolCallError {
			if item.Error == "" {
				return nil
			}
			return &schema.MCPToolCallError{
				Message: item.Error,
			}
		}(),
	})
	setItemID(resultBlock, item.ID)

	blocks = []*schema.ContentBlock{callBlock, resultBlock}

	return blocks, nil
}

func mcpListToolsToContentBlock(item responses.ResponseOutputItemMcpListTools) (block *schema.ContentBlock, err error) {
	group := &errgroup.Group{}
	group.SetLimit(5)
	mu := sync.Mutex{}

	tools := make([]*schema.MCPListToolsItem, 0, len(item.Tools))
	for i := range item.Tools {
		tool := item.Tools[i]

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
		ServerLabel: item.ServerLabel,
		Tools:       tools,
		Error:       item.Error,
	})

	setItemID(block, item.ID)

	return block, nil
}

func mcpApprovalRequestToContentBlock(item responses.ResponseOutputItemMcpApprovalRequest) (block *schema.ContentBlock, err error) {
	block = schema.NewContentBlock(&schema.MCPToolApprovalRequest{
		ID:          item.ID,
		ServerLabel: item.ServerLabel,
		Name:        item.Name,
		Arguments:   item.Arguments,
	})

	setItemID(block, item.ID)

	return block, nil
}

func responseObjectToResponseMeta(res *responses.Response) *schema.AgenticResponseMeta {
	return &schema.AgenticResponseMeta{
		TokenUsage:      toTokenUsage(res),
		OpenAIExtension: toResponseMetaExtension(res),
	}
}

func toTokenUsage(resp *responses.Response) (tokenUsage *schema.TokenUsage) {
	usage := &schema.TokenUsage{
		PromptTokens: int(resp.Usage.InputTokens),
		PromptTokenDetails: schema.PromptTokenDetails{
			CachedTokens: int(resp.Usage.InputTokensDetails.CachedTokens),
		},
		CompletionTokens: int(resp.Usage.OutputTokens),
		CompletionTokensDetails: schema.CompletionTokensDetails{
			ReasoningTokens: int(resp.Usage.OutputTokensDetails.ReasoningTokens),
		},
		TotalTokens: int(resp.Usage.TotalTokens),
	}

	return usage
}

func toResponseMetaExtension(resp *responses.Response) *openai.ResponseMetaExtension {
	var incompleteDetails *openai.IncompleteDetails
	if resp.IncompleteDetails.Reason != "" {
		incompleteDetails = &openai.IncompleteDetails{
			Reason: resp.IncompleteDetails.Reason,
		}
	}

	var respErr *openai.ResponseError
	if resp.Error.Code != "" || resp.Error.Message != "" {
		respErr = &openai.ResponseError{
			Code:    openai.ResponseErrorCode(resp.Error.Code),
			Message: resp.Error.Message,
		}
	}

	reasoning := &openai.Reasoning{
		Effort:  openai.ReasoningEffort(resp.Reasoning.Effort),
		Summary: openai.ReasoningSummary(resp.Reasoning.Summary),
	}

	extension := &openai.ResponseMetaExtension{
		ID:                   resp.ID,
		Status:               openai.ResponseStatus(resp.Status),
		Error:                respErr,
		IncompleteDetails:    incompleteDetails,
		PreviousResponseID:   resp.PreviousResponseID,
		Reasoning:            reasoning,
		ServiceTier:          openai.ServiceTier(resp.ServiceTier),
		CreatedAt:            int64(resp.CreatedAt),
		PromptCacheRetention: openai.PromptCacheRetention(resp.PromptCacheRetention),
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

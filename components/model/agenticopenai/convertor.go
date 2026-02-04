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
	"fmt"
	"strings"
	"sync"

	"github.com/bytedance/sonic"
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

	for _, block := range msg.ContentBlocks {
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

	items, err = pairMCPToolCallItems(items)
	if err != nil {
		return nil, fmt.Errorf("failed to pair MCP tool call items: %w", err)
	}

	items, err = pairWebServerToolCallItems(items)
	if err != nil {
		return nil, fmt.Errorf("failed to pair web server tool call items: %w", err)
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

func pairWebServerToolCallItems(items []responses.ResponseInputItemUnionParam) (newItems []responses.ResponseInputItemUnionParam, err error) {
	processed := make(map[int]bool)
	serverCallItemIDIndices := make(map[string][]int)

	for i, item := range items {
		serverCall := item.OfWebSearchCall
		if serverCall == nil {
			continue
		}

		id := serverCall.ID
		if id == "" {
			return nil, fmt.Errorf("found server tool call item with empty ID at index %d", i)
		}

		serverCallItemIDIndices[id] = append(serverCallItemIDIndices[id], i)
	}

	for id, indices := range serverCallItemIDIndices {
		if len(indices) != 2 {
			return nil, fmt.Errorf("server tool call %q should have exactly 2 items (call and result), "+
				"but found %d", id, len(indices))
		}
	}

	for i, item := range items {
		if processed[i] {
			continue
		}

		serverCall := item.OfWebSearchCall
		if serverCall == nil {
			newItems = append(newItems, item)
			continue
		}

		id := serverCall.ID
		indices := serverCallItemIDIndices[id]

		var pairIndex int
		if indices[0] == i {
			pairIndex = indices[1]
		} else {
			pairIndex = indices[0]
		}

		pairServerCall := items[pairIndex].OfWebSearchCall

		mergedItem := responses.ResponseInputItemUnionParam{
			OfWebSearchCall: &responses.ResponseFunctionWebSearchParam{
				ID:     serverCall.ID,
				Action: pairWebSearchAction(serverCall.Action, pairServerCall.Action),
				Status: coalesce(serverCall.Status, pairServerCall.Status),
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
		if pairAction.OfSearch.Query != "" {
			ret.OfSearch.Query = pairAction.OfSearch.Query
		}
		if len(pairAction.OfSearch.Sources) > 0 {
			ret.OfSearch.Sources = pairAction.OfSearch.Sources
		}
	}

	return ret
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

	item = responses.ResponseInputItemUnionParam{
		OfFunctionCall: &responses.ResponseFunctionToolCallParam{
			ID:        newOpenaiStrOpt(id),
			Status:    responses.ResponseFunctionToolCallStatus(status),
			CallID:    content.CallID,
			Name:      content.Name,
			Arguments: content.Arguments,
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

	id, _ := getItemID(block)
	status, _ := GetItemStatus(block)

	arguments, err := getServerToolCallArguments(content)
	if err != nil {
		return item, err
	}

	var action responses.ResponseFunctionWebSearchActionUnionParam
	switch {
	case arguments.WebSearch != nil:
		action, err = getWebSearchToolCallActionParam(arguments.WebSearch)
	default:
		return item, fmt.Errorf("server tool call arguments are nil")
	}
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
				Query: ws.Search.Query,
			},
		}, nil

	case WebSearchActionOpenPage:
		return responses.ResponseFunctionWebSearchActionUnionParam{
			OfOpenPage: &responses.ResponseFunctionWebSearchActionOpenPageParam{
				URL: ws.OpenPage.URL,
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
		return action, fmt.Errorf("invalid web search action type: %s", ws.ActionType)
	}
}

func serverToolResultToInputItem(block *schema.ContentBlock) (item responses.ResponseInputItemUnionParam, err error) {
	content := block.ServerToolResult
	if content == nil {
		return item, fmt.Errorf("server tool result is nil")
	}

	id, _ := getItemID(block)
	status, _ := GetItemStatus(block)

	result, err := getServerToolResult(content)
	if err != nil {
		return item, err
	}

	var action responses.ResponseFunctionWebSearchActionUnionParam
	switch {
	case result.WebSearch != nil:
		action, err = getWebSearchToolResultActionParam(result.WebSearch)
	default:
		return item, fmt.Errorf("server tool result is nil")
	}
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
		return action, fmt.Errorf("invalid web search result action type: %s", ws.ActionType)
	}
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

func toOutputMessage(resp *responses.Response) (msg *schema.AgenticMessage, err error) {
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

		default:
			return nil, fmt.Errorf("invalid output item type: %T", variant)
		}

		blocks = append(blocks, tmpBlocks...)
	}

	msg = &schema.AgenticMessage{
		Role:          schema.AgenticRoleTypeAssistant,
		ContentBlocks: blocks,
		ResponseMeta:  responseObjectToResponseMeta(resp),
	}

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
			return nil, fmt.Errorf("invalid output message content type: %s", content.Type)
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
		return nil, fmt.Errorf("invalid annotation type: %s", anno.Type)
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
					Query: variant.Query,
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
		return nil, fmt.Errorf("invalid web search variant type: %s", item.Type)
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

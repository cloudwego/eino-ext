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

package agenticclaude

import (
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/eino/schema/claude"
)

func toAnthropicMessages(input []*schema.AgenticMessage) (systemBlocks []anthropic.TextBlockParam, msgParams []anthropic.MessageParam, err error) {
	if len(input) == 0 {
		return nil, nil, fmt.Errorf("input is empty")
	}

	systemDone := false

	for _, msg := range input {
		switch msg.Role {
		case schema.AgenticRoleTypeSystem:
			if systemDone {
				return nil, nil, fmt.Errorf("system message must appear before all non-system messages")
			}
			blocks, err := toSystemBlocks(msg)
			if err != nil {
				return nil, nil, err
			}
			systemBlocks = append(systemBlocks, blocks...)

		case schema.AgenticRoleTypeUser:
			systemDone = true
			msgParam, err := toUserMessageParam(msg)
			if err != nil {
				return nil, nil, err
			}
			msgParams = append(msgParams, msgParam)

		case schema.AgenticRoleTypeAssistant:
			systemDone = true
			msgParam, err := toAssistantMessageParam(msg)
			if err != nil {
				return nil, nil, err
			}
			msgParams = append(msgParams, msgParam)

		default:
			return nil, nil, fmt.Errorf("invalid role %q", msg.Role)
		}
	}

	return systemBlocks, msgParams, nil
}

func toSystemBlocks(msg *schema.AgenticMessage) ([]anthropic.TextBlockParam, error) {
	blockParams := make([]anthropic.TextBlockParam, 0, len(msg.ContentBlocks))

	for _, block := range msg.ContentBlocks {
		if block.Type != schema.ContentBlockTypeUserInputText {
			return nil, fmt.Errorf("system message only supports text blocks, got %q", block.Type)
		}
		textBlock := anthropic.TextBlockParam{
			Text: block.UserInputText.Text,
		}
		if hasCacheControlOnContentBlock(block) {
			textBlock.CacheControl = *getContentBlockCacheControl(block)
		}
		blockParams = append(blockParams, textBlock)
	}

	return blockParams, nil
}

func toUserMessageParam(msg *schema.AgenticMessage) (msgParam anthropic.MessageParam, err error) {
	blockParams := make([]anthropic.ContentBlockParamUnion, 0, len(msg.ContentBlocks))

	for _, block := range msg.ContentBlocks {
		var blockParam anthropic.ContentBlockParamUnion

		switch block.Type {
		case schema.ContentBlockTypeUserInputText:
			blockParam, err = userInputTextToBlockParam(block.UserInputText)
		case schema.ContentBlockTypeUserInputImage:
			blockParam, err = userInputImageToBlockParam(block.UserInputImage)
		case schema.ContentBlockTypeUserInputFile:
			blockParam, err = userInputFileToBlockParam(block.UserInputFile)
		case schema.ContentBlockTypeFunctionToolResult:
			blockParam, err = functionToolResultToBlockParam(block.FunctionToolResult)
		case schema.ContentBlockTypeToolSearchResult:
			blockParam, err = toolSearchResultToBlockParam(block.ToolSearchFunctionToolResult)
		default:
			err = fmt.Errorf("invalid content block type %q with user role", block.Type)
		}

		if err != nil {
			return anthropic.MessageParam{}, err
		}

		if hasCacheControlOnContentBlock(block) {
			*blockParam.GetCacheControl() = *getContentBlockCacheControl(block)
		}

		blockParams = append(blockParams, blockParam)
	}

	return anthropic.MessageParam{
		Role:    anthropic.MessageParamRoleUser,
		Content: blockParams,
	}, nil
}

func toAssistantMessageParam(msg *schema.AgenticMessage) (msgParam anthropic.MessageParam, err error) {
	blockParams := make([]anthropic.ContentBlockParamUnion, 0, len(msg.ContentBlocks))

	for _, block := range msg.ContentBlocks {
		var blockParam anthropic.ContentBlockParamUnion

		switch block.Type {
		case schema.ContentBlockTypeAssistantGenText:
			blockParam, err = assistantGenTextToBlockParam(block.AssistantGenText)
		case schema.ContentBlockTypeReasoning:
			blockParam, err = reasoningToBlockParam(block.Reasoning)
		case schema.ContentBlockTypeFunctionToolCall:
			blockParam, err = functionToolCallToBlockParam(block.FunctionToolCall)
		case schema.ContentBlockTypeServerToolCall:
			blockParam, err = serverToolCallToBlockParam(block.ServerToolCall)
		case schema.ContentBlockTypeServerToolResult:
			blockParam, err = serverToolResultToBlockParam(block)
		default:
			err = fmt.Errorf("invalid content block type %q with assistant role", block.Type)
		}

		if err != nil {
			return anthropic.MessageParam{}, err
		}

		if hasCacheControlOnContentBlock(block) {
			*blockParam.GetCacheControl() = *getContentBlockCacheControl(block)
		}

		blockParams = append(blockParams, blockParam)
	}

	return anthropic.MessageParam{
		Role:    anthropic.MessageParamRoleAssistant,
		Content: blockParams,
	}, nil
}

func userInputTextToBlockParam(text *schema.UserInputText) (anthropic.ContentBlockParamUnion, error) {
	return anthropic.ContentBlockParamUnion{
		OfText: &anthropic.TextBlockParam{Text: text.Text},
	}, nil
}

func userInputImageToBlockParam(image *schema.UserInputImage) (anthropic.ContentBlockParamUnion, error) {
	return imageToBlockParam(image)
}

func userInputFileToBlockParam(file *schema.UserInputFile) (anthropic.ContentBlockParamUnion, error) {
	documentBlockParam, err := documentToBlockParam(file)
	if err != nil {
		return anthropic.ContentBlockParamUnion{}, err
	}
	return documentBlockParam, nil
}

func assistantGenTextToBlockParam(text *schema.AssistantGenText) (anthropic.ContentBlockParamUnion, error) {
	textBlockParam := anthropic.TextBlockParam{
		Text: text.Text,
	}

	if text.ClaudeExtension != nil {
		citations, err := toAnthropicTextCitations(text.ClaudeExtension.Citations)
		if err != nil {
			return anthropic.ContentBlockParamUnion{}, err
		}
		textBlockParam.Citations = citations
	}

	return anthropic.ContentBlockParamUnion{OfText: &textBlockParam}, nil
}

func reasoningToBlockParam(reasoning *schema.Reasoning) (anthropic.ContentBlockParamUnion, error) {
	return anthropic.NewThinkingBlock(reasoning.Signature, reasoning.Text), nil
}

func functionToolCallToBlockParam(call *schema.FunctionToolCall) (anthropic.ContentBlockParamUnion, error) {
	input := json.RawMessage("{}")
	if call.Arguments != "" {
		input = json.RawMessage(call.Arguments)
	}
	return anthropic.NewToolUseBlock(call.CallID, input, call.Name), nil
}

func serverToolCallToBlockParam(call *schema.ServerToolCall) (anthropic.ContentBlockParamUnion, error) {
	args, err := getServerToolCallArguments(call)
	if err != nil {
		return anthropic.ContentBlockParamUnion{}, err
	}

	switch {
	case args.WebSearch != nil:
		return anthropic.NewServerToolUseBlock(call.CallID, args.WebSearch, anthropic.ServerToolUseBlockParamNameWebSearch), nil
	case args.WebFetch != nil:
		return anthropic.NewServerToolUseBlock(call.CallID, args.WebFetch, anthropic.ServerToolUseBlockParamNameWebFetch), nil
	case args.CodeExecution != nil:
		return anthropic.NewServerToolUseBlock(call.CallID, args.CodeExecution, anthropic.ServerToolUseBlockParamNameCodeExecution), nil
	case args.BashCodeExecution != nil:
		return anthropic.NewServerToolUseBlock(call.CallID, args.BashCodeExecution, anthropic.ServerToolUseBlockParamNameBashCodeExecution), nil
	case args.TextEditorCodeExecution != nil:
		return anthropic.NewServerToolUseBlock(call.CallID, args.TextEditorCodeExecution, anthropic.ServerToolUseBlockParamNameTextEditorCodeExecution), nil
	case args.ToolSearchToolBm25 != nil:
		return anthropic.NewServerToolUseBlock(call.CallID, args.ToolSearchToolBm25, anthropic.ServerToolUseBlockParamNameToolSearchToolBm25), nil
	case args.ToolSearchToolRegex != nil:
		return anthropic.NewServerToolUseBlock(call.CallID, args.ToolSearchToolRegex, anthropic.ServerToolUseBlockParamNameToolSearchToolRegex), nil
	default:
		return anthropic.ContentBlockParamUnion{}, fmt.Errorf("server tool call arguments are nil")
	}
}

func serverToolResultToBlockParam(block *schema.ContentBlock) (blockParam anthropic.ContentBlockParamUnion, err error) {
	content, err := getServerToolResult(block.ServerToolResult)
	if err != nil {
		return blockParam, err
	}

	switch {
	case content.WebSearch != nil:
		return webSearchToolResultToBlockParam(content.WebSearch, block.ServerToolResult.CallID, block)
	case content.WebFetch != nil:
		return webFetchToolResultToBlockParam(content.WebFetch, block.ServerToolResult.CallID, block)
	case content.CodeExecution != nil:
		return codeExecutionToolResultToBlockParam(content.CodeExecution, block.ServerToolResult.CallID)
	case content.BashCodeExecution != nil:
		return bashCodeExecutionToolResultToBlockParam(content.BashCodeExecution, block.ServerToolResult.CallID)
	case content.TextEditorCodeExecution != nil:
		return textEditorCodeExecutionToolResultToBlockParam(content.TextEditorCodeExecution, block.ServerToolResult.CallID)
	case content.ToolSearchToolBm25 != nil:
		return toolSearchToolResultToBlockParam(content.ToolSearchToolBm25, block.ServerToolResult.CallID)
	case content.ToolSearchToolRegex != nil:
		return toolSearchToolResultToBlockParam(content.ToolSearchToolRegex, block.ServerToolResult.CallID)
	default:
		return blockParam, fmt.Errorf("server tool result content is nil")
	}
}

func webSearchToolResultToBlockParam(result *WebSearchResult, callID string, block *schema.ContentBlock) (anthropic.ContentBlockParamUnion, error) {
	caller, err := toWebSearchResultCallerParam(block)
	if err != nil {
		return anthropic.ContentBlockParamUnion{}, err
	}

	switch result.Type {
	case WebSearchResultTypeError:
		blockParam := anthropic.NewWebSearchToolResultBlock(anthropic.WebSearchToolRequestErrorParam{
			ErrorCode: anthropic.WebSearchToolResultErrorCode(result.Error.Code),
		}, callID)
		blockParam.OfWebSearchToolResult.Caller = caller
		return blockParam, nil

	case WebSearchResultTypeResult:
		resultBlockParams := make([]anthropic.WebSearchResultBlockParam, 0, len(result.Result.Content))
		for _, item := range result.Result.Content {
			resultBlockParam := anthropic.WebSearchResultBlockParam{
				Title:            item.Title,
				URL:              item.URL,
				EncryptedContent: item.EncryptedContent,
				PageAge:          param.NewOpt(item.PageAge),
			}
			resultBlockParams = append(resultBlockParams, resultBlockParam)
		}

		blockParam := anthropic.NewWebSearchToolResultBlock(resultBlockParams, callID)
		blockParam.OfWebSearchToolResult.Caller = caller
		return blockParam, nil

	default:
		return anthropic.ContentBlockParamUnion{}, fmt.Errorf("invalid web search result type %q", result.Type)
	}
}

func webFetchToolResultToBlockParam(result *WebFetchResult, callID string, block *schema.ContentBlock) (anthropic.ContentBlockParamUnion, error) {
	caller, err := toWebFetchResultCallerParam(block)
	if err != nil {
		return anthropic.ContentBlockParamUnion{}, err
	}

	switch result.Type {
	case WebFetchResultTypeError:
		blockParam := anthropic.NewWebFetchToolResultBlock(anthropic.WebFetchToolResultErrorBlockParam{
			ErrorCode: anthropic.WebFetchToolResultErrorCode(result.Error.Code),
		}, callID)
		blockParam.OfWebFetchToolResult.Caller = caller
		return blockParam, nil

	case WebFetchResultTypeResult:
		documentBlockParam, err := toWebFetchDocumentBlockParam(result.Result.Content)
		if err != nil {
			return anthropic.ContentBlockParamUnion{}, err
		}

		blockParam := anthropic.NewWebFetchToolResultBlock(anthropic.WebFetchBlockParam{
			URL:         result.Result.URL,
			RetrievedAt: param.NewOpt(result.Result.RetrievedAt),
			Content:     documentBlockParam,
		}, callID)
		blockParam.OfWebFetchToolResult.Caller = caller
		return blockParam, nil

	default:
		return anthropic.ContentBlockParamUnion{}, fmt.Errorf("invalid web fetch result type %q", result.Type)
	}
}

func functionToolResultToBlockParam(result *schema.FunctionToolResult) (anthropic.ContentBlockParamUnion, error) {
	content := make([]anthropic.ToolResultBlockParamContentUnion, 0, len(result.Content))
	for _, block := range result.Content {
		item, err := functionToolResultContentToBlockParam(block)
		if err != nil {
			return anthropic.ContentBlockParamUnion{}, err
		}
		content = append(content, item)
	}
	return anthropic.ContentBlockParamUnion{
		OfToolResult: &anthropic.ToolResultBlockParam{
			ToolUseID: result.CallID,
			Content:   content,
		},
	}, nil
}

func toolSearchResultToBlockParam(result *schema.ToolSearchFunctionToolResult) (anthropic.ContentBlockParamUnion, error) {
	content := make([]anthropic.ToolResultBlockParamContentUnion, 0, len(result.Result.Tools))
	for _, tool := range result.Result.Tools {
		content = append(content, anthropic.ToolResultBlockParamContentUnion{
			OfToolReference: &anthropic.ToolReferenceBlockParam{
				ToolName: tool.Name,
			},
		})
	}

	return anthropic.ContentBlockParamUnion{
		OfToolResult: &anthropic.ToolResultBlockParam{
			ToolUseID: result.CallID,
			Content:   content,
		},
	}, nil
}

func functionToolResultContentToBlockParam(block *schema.FunctionToolResultContentBlock) (blockParam anthropic.ToolResultBlockParamContentUnion, err error) {
	switch block.Type {
	case schema.FunctionToolResultContentBlockTypeText:
		blockParam.OfText = &anthropic.TextBlockParam{Text: block.Text.Text}
		return blockParam, nil

	case schema.FunctionToolResultContentBlockTypeImage:
		imageBlockParam, err := imageToBlockParam(block.Image)
		if err != nil {
			return blockParam, err
		}
		blockParam.OfImage = imageBlockParam.OfImage
		return blockParam, nil

	case schema.FunctionToolResultContentBlockTypeFile:
		documentBlockParam, err := documentToBlockParam(block.File)
		if err != nil {
			return blockParam, err
		}
		blockParam.OfDocument = documentBlockParam.OfDocument
		return blockParam, nil

	default:
		return blockParam, fmt.Errorf("invalid function tool result content block type %q", block.Type)
	}
}

func imageToBlockParam(image *schema.UserInputImage) (anthropic.ContentBlockParamUnion, error) {
	switch {
	case image.URL != "":
		return anthropic.NewImageBlock(anthropic.URLImageSourceParam{URL: image.URL}), nil
	case image.Base64Data != "":
		mediaType, err := toAnthropicImageMediaType(image.MIMEType)
		if err != nil {
			return anthropic.ContentBlockParamUnion{}, err
		}
		return anthropic.NewImageBlock(anthropic.Base64ImageSourceParam{
			Data:      image.Base64Data,
			MediaType: mediaType,
		}), nil
	default:
		return anthropic.ContentBlockParamUnion{}, fmt.Errorf("image input requires either URL or Base64Data")
	}
}

func documentToBlockParam(file *schema.UserInputFile) (anthropic.ContentBlockParamUnion, error) {
	switch {
	case file.URL != "":
		return anthropic.NewDocumentBlock(anthropic.URLPDFSourceParam{URL: file.URL}), nil
	case file.Base64Data != "":
		return anthropic.NewDocumentBlock(anthropic.Base64PDFSourceParam{Data: file.Base64Data}), nil
	default:
		return anthropic.ContentBlockParamUnion{}, fmt.Errorf("file input requires either URL or Base64Data")
	}
}

func toAnthropicImageMediaType(mimeType string) (anthropic.Base64ImageSourceMediaType, error) {
	switch mimeType {
	case "image/jpeg", "image/jpg":
		return anthropic.Base64ImageSourceMediaTypeImageJPEG, nil
	case "image/png":
		return anthropic.Base64ImageSourceMediaTypeImagePNG, nil
	case "image/gif":
		return anthropic.Base64ImageSourceMediaTypeImageGIF, nil
	case "image/webp":
		return anthropic.Base64ImageSourceMediaTypeImageWebP, nil
	default:
		return "", fmt.Errorf("invalid image mime type %q", mimeType)
	}
}

func toFunctionTools(functionTools []*schema.ToolInfo) ([]anthropic.ToolUnionParam, error) {
	tools := make([]anthropic.ToolUnionParam, 0, len(functionTools))
	for _, tool := range functionTools {
		s, err := tool.ToJSONSchema()
		if err != nil {
			return nil, fmt.Errorf("failed to convert tool %q parameters to JSON schema: %w", tool.Name, err)
		}
		toolParam := &anthropic.ToolParam{
			Type:        anthropic.ToolTypeCustom,
			Name:        tool.Name,
			Description: newClaudeStrOpt(tool.Desc),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: s.Properties,
				Required:   s.Required,
			},
		}
		if hasCacheControlOnToolInfo(tool) {
			toolParam.CacheControl = *getToolInfoCacheControl(tool)
		}
		tools = append(tools, anthropic.ToolUnionParam{OfTool: toolParam})
	}
	return tools, nil
}

func toDeferredFunctionTools(functionTools []*schema.ToolInfo) ([]anthropic.ToolUnionParam, error) {
	tools, err := toFunctionTools(functionTools)
	if err != nil {
		return nil, err
	}
	for i := range tools {
		tools[i].OfTool.DeferLoading = param.NewOpt(true)
	}
	return tools, nil
}

func toAgenticMessage(resp *anthropic.Message) (*schema.AgenticMessage, error) {
	if resp == nil {
		return nil, fmt.Errorf("response is nil")
	}

	blocks := make([]*schema.ContentBlock, 0, len(resp.Content))
	for _, item := range resp.Content {
		contentBlock, err := toAgenticContentBlock(item.AsAny())
		if err != nil {
			return nil, err
		}
		if contentBlock == nil {
			continue
		}
		blocks = append(blocks, contentBlock)
	}

	return &schema.AgenticMessage{
		Role:          schema.AgenticRoleTypeAssistant,
		ContentBlocks: blocks,
		ResponseMeta:  toAgenticResponseMeta(resp),
	}, nil
}

func toAgenticContentBlock(block any) (*schema.ContentBlock, error) {
	switch item := block.(type) {
	case anthropic.TextBlock:
		return schema.NewContentBlock(&schema.AssistantGenText{
			Text:            item.Text,
			ClaudeExtension: toAssistantTextExtension(item.Citations),
		}), nil

	case anthropic.ThinkingBlock:
		return schema.NewContentBlock(&schema.Reasoning{
			Text:      item.Thinking,
			Signature: item.Signature,
		}), nil

	case anthropic.ToolUseBlock:
		return schema.NewContentBlock(&schema.FunctionToolCall{
			CallID:    item.ID,
			Name:      item.Name,
			Arguments: string(item.Input),
		}), nil

	case anthropic.RedactedThinkingBlock:
		// Drop redacted thinking block
		return nil, nil

	case anthropic.ServerToolUseBlock:
		contentBlock, err := serverToolUseToContentBlock(item)
		if err != nil {
			return nil, err
		}
		return contentBlock, nil

	case anthropic.WebSearchToolResultBlock:
		contentBlock, err := webSearchToolResultToContentBlock(item)
		if err != nil {
			return nil, err
		}
		return contentBlock, nil

	case anthropic.WebFetchToolResultBlock:
		contentBlock, err := webFetchToolResultToContentBlock(item)
		if err != nil {
			return nil, err
		}
		return contentBlock, nil

	case anthropic.CodeExecutionToolResultBlock:
		contentBlock, err := codeExecutionToolResultToContentBlock(item)
		if err != nil {
			return nil, err
		}
		return contentBlock, nil

	case anthropic.BashCodeExecutionToolResultBlock:
		contentBlock, err := bashCodeExecutionToolResultToContentBlock(item)
		if err != nil {
			return nil, err
		}
		return contentBlock, nil

	case anthropic.TextEditorCodeExecutionToolResultBlock:
		contentBlock, err := textEditorCodeExecutionToolResultToContentBlock(item)
		if err != nil {
			return nil, err
		}
		return contentBlock, nil

	default:
		return nil, fmt.Errorf("invalid output block type %T", block)
	}
}

func toAgenticResponseMeta(resp *anthropic.Message) *schema.AgenticResponseMeta {
	return &schema.AgenticResponseMeta{
		TokenUsage: toTokenUsage(resp.Usage),
		ClaudeExtension: &claude.ResponseMetaExtension{
			ID:           resp.ID,
			StopReason:   string(resp.StopReason),
			StopSequence: resp.StopSequence,
			StopDetails:  toClaudeStopDetails(resp.StopDetails),
		},
	}
}

func serverToolUseToContentBlock(block anthropic.ServerToolUseBlock) (*schema.ContentBlock, error) {
	switch ServerToolName(block.Name) {
	case ServerToolNameWebSearch:
		return webSearchToolUseToContentBlock(block)
	case ServerToolNameWebFetch:
		return webFetchToolUseToContentBlock(block)
	case ServerToolNameCodeExecution:
		return codeExecutionToolUseToContentBlock(block)
	case ServerToolNameBashCodeExecution:
		return bashCodeExecutionToolUseToContentBlock(block)
	case ServerToolNameTextEditorCodeExecution:
		return textEditorCodeExecutionToolUseToContentBlock(block)
	case ServerToolNameToolSearchToolBm25:
		return toolSearchToolBm25UseToContentBlock(block)
	case ServerToolNameToolSearchToolRegex:
		return toolSearchToolRegexUseToContentBlock(block)
	default:
		return nil, fmt.Errorf("invalid server tool %q", block.Name)
	}
}

func newServerToolCallContentBlock(callID, name string, args *ServerToolCallArguments) *schema.ContentBlock {
	return schema.NewContentBlock(&schema.ServerToolCall{
		CallID:    callID,
		Name:      name,
		Arguments: args,
	})
}

func webSearchToolUseToContentBlock(block anthropic.ServerToolUseBlock) (*schema.ContentBlock, error) {
	args := &WebSearchArguments{}
	if block.Input == nil {
		return newServerToolCallContentBlock(block.ID, string(ServerToolNameWebSearch), &ServerToolCallArguments{
			WebSearch: args,
		}), nil
	}

	m, ok := block.Input.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T for web search input", block.Input)
	}
	if err := setStringArgument(m, "query", &args.Query); err != nil {
		return nil, fmt.Errorf("invalid web search query: %w", err)
	}

	return newServerToolCallContentBlock(block.ID, string(ServerToolNameWebSearch), &ServerToolCallArguments{
		WebSearch: args,
	}), nil
}

func webFetchToolUseToContentBlock(block anthropic.ServerToolUseBlock) (*schema.ContentBlock, error) {
	args := &WebFetchArguments{}
	if block.Input == nil {
		return newServerToolCallContentBlock(block.ID, string(ServerToolNameWebFetch), &ServerToolCallArguments{
			WebFetch: args,
		}), nil
	}

	m, ok := block.Input.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T for web fetch input", block.Input)
	}
	if err := setStringArgument(m, "url", &args.URL); err != nil {
		return nil, fmt.Errorf("invalid web fetch url: %w", err)
	}

	return newServerToolCallContentBlock(block.ID, string(ServerToolNameWebFetch), &ServerToolCallArguments{
		WebFetch: args,
	}), nil
}

func codeExecutionToolUseToContentBlock(block anthropic.ServerToolUseBlock) (*schema.ContentBlock, error) {
	args := &CodeExecutionArguments{}
	if block.Input == nil {
		return newServerToolCallContentBlock(block.ID, string(ServerToolNameCodeExecution), &ServerToolCallArguments{
			CodeExecution: args,
		}), nil
	}

	m, ok := block.Input.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T for code execution input", block.Input)
	}
	if err := setStringArgument(m, "code", &args.Code); err != nil {
		return nil, fmt.Errorf("invalid code execution code: %w", err)
	}

	return newServerToolCallContentBlock(block.ID, string(ServerToolNameCodeExecution), &ServerToolCallArguments{
		CodeExecution: args,
	}), nil
}

func bashCodeExecutionToolUseToContentBlock(block anthropic.ServerToolUseBlock) (*schema.ContentBlock, error) {
	args := &BashCodeExecutionArguments{}
	if block.Input == nil {
		return newServerToolCallContentBlock(block.ID, string(ServerToolNameBashCodeExecution), &ServerToolCallArguments{
			BashCodeExecution: args,
		}), nil
	}

	m, ok := block.Input.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T for bash code execution input", block.Input)
	}
	if err := setStringArgument(m, "command", &args.Command); err != nil {
		return nil, fmt.Errorf("invalid bash code execution command: %w", err)
	}

	return newServerToolCallContentBlock(block.ID, string(ServerToolNameBashCodeExecution), &ServerToolCallArguments{
		BashCodeExecution: args,
	}), nil
}

func textEditorCodeExecutionToolUseToContentBlock(block anthropic.ServerToolUseBlock) (*schema.ContentBlock, error) {
	args := &TextEditorCodeExecutionArguments{}
	if block.Input == nil {
		return newServerToolCallContentBlock(block.ID, string(ServerToolNameTextEditorCodeExecution), &ServerToolCallArguments{
			TextEditorCodeExecution: args,
		}), nil
	}

	m, ok := block.Input.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T for text editor code execution input", block.Input)
	}
	if err := setStringArgument(m, "command", &args.Command); err != nil {
		return nil, fmt.Errorf("invalid text editor code execution command: %w", err)
	}
	if err := setStringArgument(m, "path", &args.Path); err != nil {
		return nil, fmt.Errorf("invalid text editor code execution path: %w", err)
	}
	if err := setStringArgument(m, "file_text", &args.FileText); err != nil {
		return nil, fmt.Errorf("invalid text editor code execution file_text: %w", err)
	}
	if err := setStringArgument(m, "old_str", &args.OldStr); err != nil {
		return nil, fmt.Errorf("invalid text editor code execution old_str: %w", err)
	}
	if err := setStringArgument(m, "new_str", &args.NewStr); err != nil {
		return nil, fmt.Errorf("invalid text editor code execution new_str: %w", err)
	}

	return newServerToolCallContentBlock(block.ID, string(ServerToolNameTextEditorCodeExecution), &ServerToolCallArguments{
		TextEditorCodeExecution: args,
	}), nil
}

func toolSearchToolBm25UseToContentBlock(block anthropic.ServerToolUseBlock) (*schema.ContentBlock, error) {
	args := &ToolSearchToolBm25Arguments{}
	if block.Input == nil {
		return newServerToolCallContentBlock(block.ID, string(ServerToolNameToolSearchToolBm25), &ServerToolCallArguments{
			ToolSearchToolBm25: args,
		}), nil
	}

	m, ok := block.Input.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T for tool search bm25 input", block.Input)
	}
	if err := setStringArgument(m, "query", &args.Query); err != nil {
		return nil, fmt.Errorf("invalid tool search bm25 query: %w", err)
	}

	return newServerToolCallContentBlock(block.ID, string(ServerToolNameToolSearchToolBm25), &ServerToolCallArguments{
		ToolSearchToolBm25: args,
	}), nil
}

func toolSearchToolRegexUseToContentBlock(block anthropic.ServerToolUseBlock) (*schema.ContentBlock, error) {
	args := &ToolSearchToolRegexArguments{}
	if block.Input == nil {
		return newServerToolCallContentBlock(block.ID, string(ServerToolNameToolSearchToolRegex), &ServerToolCallArguments{
			ToolSearchToolRegex: args,
		}), nil
	}

	m, ok := block.Input.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T for tool search regex input", block.Input)
	}
	if err := setStringArgument(m, "query", &args.Query); err != nil {
		return nil, fmt.Errorf("invalid tool search regex query: %w", err)
	}

	return newServerToolCallContentBlock(block.ID, string(ServerToolNameToolSearchToolRegex), &ServerToolCallArguments{
		ToolSearchToolRegex: args,
	}), nil
}

func webSearchToolResultToContentBlock(block anthropic.WebSearchToolResultBlock) (*schema.ContentBlock, error) {
	result := &WebSearchResult{}

	switch WebSearchResultType(block.Content.Type) {
	case WebSearchResultTypeError:
		result.Type = WebSearchResultTypeError
		result.Error = &WebSearchResultError{
			Code: string(block.Content.ErrorCode),
		}

	case "":
		result.Type = WebSearchResultTypeResult
		items := block.Content.AsWebSearchResultBlockArray()
		result.Result = &WebSearchResultBlock{
			Content: make([]*WebSearchResultItem, 0, len(items)),
		}
		for i := range items {
			item := &items[i]
			result.Result.Content = append(result.Result.Content, &WebSearchResultItem{
				Title:            item.Title,
				URL:              item.URL,
				EncryptedContent: item.EncryptedContent,
				PageAge:          item.PageAge,
			})
		}

	default:
		return nil, fmt.Errorf("invalid web search result type %q", block.Content.Type)
	}

	contentBlock := schema.NewContentBlock(&schema.ServerToolResult{
		CallID: block.ToolUseID,
		Name:   string(ServerToolNameWebSearch),
		Content: &ServerToolResult{
			WebSearch: result,
		},
	})
	setWebSearchResultCaller(contentBlock, block.Caller)

	return contentBlock, nil
}

func webFetchToolResultToContentBlock(block anthropic.WebFetchToolResultBlock) (*schema.ContentBlock, error) {
	result := &WebFetchResult{}

	switch WebFetchResultType(block.Content.Type) {
	case WebFetchResultTypeError:
		result.Type = WebFetchResultTypeError
		result.Error = &WebFetchResultError{
			Code: string(block.Content.ErrorCode),
		}

	case WebFetchResultTypeResult:
		result.Type = WebFetchResultTypeResult
		document, err := toWebFetchDocument(block.Content.Content)
		if err != nil {
			return nil, err
		}
		result.Result = &WebFetchResultBlock{
			URL:         block.Content.URL,
			RetrievedAt: block.Content.RetrievedAt,
			Content:     document,
		}

	default:
		return nil, fmt.Errorf("invalid web fetch result type %q", block.Content.Type)
	}

	contentBlock := schema.NewContentBlock(&schema.ServerToolResult{
		CallID: block.ToolUseID,
		Name:   string(ServerToolNameWebFetch),
		Content: &ServerToolResult{
			WebFetch: result,
		},
	})
	setWebFetchResultCaller(contentBlock, block.Caller)

	return contentBlock, nil
}

func codeExecutionToolResultToContentBlock(block anthropic.CodeExecutionToolResultBlock) (*schema.ContentBlock, error) {
	result := &CodeExecutionResult{}
	switch CodeExecutionResultType(block.Content.Type) {
	case CodeExecutionResultTypeError:
		result.Type = CodeExecutionResultTypeError
		result.Error = &CodeExecutionResultError{
			Code: string(block.Content.ErrorCode),
		}

	case CodeExecutionResultTypeEncrypted:
		result.Type = CodeExecutionResultTypeEncrypted
		encrypted := block.Content.AsResponseEncryptedCodeExecutionResultBlock()
		result.EncryptedResult = &EncryptedCodeExecutionResultBlock{
			Content:         toCodeExecutionOutputs(encrypted.Content),
			EncryptedStdout: encrypted.EncryptedStdout,
			Stderr:          encrypted.Stderr,
			ReturnCode:      encrypted.ReturnCode,
		}

	case CodeExecutionResultTypeResult:
		result.Type = CodeExecutionResultTypeResult
		content := block.Content.AsResponseCodeExecutionResultBlock()
		result.Result = &CodeExecutionResultBlock{
			Content:    toCodeExecutionOutputs(content.Content),
			Stdout:     content.Stdout,
			Stderr:     content.Stderr,
			ReturnCode: content.ReturnCode,
		}

	default:
		return nil, fmt.Errorf("invalid code execution result type %q", block.Content.Type)
	}

	return schema.NewContentBlock(&schema.ServerToolResult{
		CallID: block.ToolUseID,
		Name:   string(ServerToolNameCodeExecution),
		Content: &ServerToolResult{
			CodeExecution: result,
		},
	}), nil
}

func bashCodeExecutionToolResultToContentBlock(block anthropic.BashCodeExecutionToolResultBlock) (*schema.ContentBlock, error) {
	result := &BashCodeExecutionResult{}
	switch BashCodeExecutionResultType(block.Content.Type) {
	case BashCodeExecutionResultTypeError:
		result.Type = BashCodeExecutionResultTypeError
		result.Error = &BashCodeExecutionResultError{
			Code: string(block.Content.ErrorCode),
		}

	case BashCodeExecutionResultTypeResult:
		result.Type = BashCodeExecutionResultTypeResult
		content := block.Content.AsResponseBashCodeExecutionResultBlock()
		result.Result = &BashCodeExecutionResultBlock{
			Content:    toBashCodeExecutionOutputs(content.Content),
			Stdout:     content.Stdout,
			Stderr:     content.Stderr,
			ReturnCode: content.ReturnCode,
		}

	default:
		return nil, fmt.Errorf("invalid bash code execution result type %q", block.Content.Type)
	}

	return schema.NewContentBlock(&schema.ServerToolResult{
		CallID: block.ToolUseID,
		Name:   string(ServerToolNameBashCodeExecution),
		Content: &ServerToolResult{
			BashCodeExecution: result,
		},
	}), nil
}

func textEditorCodeExecutionToolResultToContentBlock(block anthropic.TextEditorCodeExecutionToolResultBlock) (*schema.ContentBlock, error) {
	result := &TextEditorCodeExecutionResult{}
	switch TextEditorCodeExecutionResultType(block.Content.Type) {
	case TextEditorCodeExecutionResultTypeError:
		result.Type = TextEditorCodeExecutionResultTypeError
		errBlock := block.Content.AsResponseTextEditorCodeExecutionToolResultError()
		result.Error = &TextEditorCodeExecutionResultError{
			Code:    string(errBlock.ErrorCode),
			Message: errBlock.ErrorMessage,
		}

	case TextEditorCodeExecutionResultTypeCreate:
		result.Type = TextEditorCodeExecutionResultTypeCreate
		content := block.Content.AsResponseTextEditorCodeExecutionCreateResultBlock()
		result.Create = &TextEditorCodeExecutionCreateResult{
			IsFileUpdate: content.IsFileUpdate,
		}

	case TextEditorCodeExecutionResultTypeStrReplace:
		result.Type = TextEditorCodeExecutionResultTypeStrReplace
		content := block.Content.AsResponseTextEditorCodeExecutionStrReplaceResultBlock()
		result.StrReplace = &TextEditorCodeExecutionStrReplaceResult{
			OldStart: content.OldStart,
			OldLines: content.OldLines,
			NewStart: content.NewStart,
			NewLines: content.NewLines,
			Lines:    content.Lines,
		}

	case TextEditorCodeExecutionResultTypeView:
		result.Type = TextEditorCodeExecutionResultTypeView
		content := block.Content.AsResponseTextEditorCodeExecutionViewResultBlock()
		result.View = &TextEditorCodeExecutionViewResult{
			FileType:   string(content.FileType),
			Content:    content.Content,
			NumLines:   content.NumLines,
			StartLine:  content.StartLine,
			TotalLines: content.TotalLines,
		}

	default:
		return nil, fmt.Errorf("invalid text editor code execution result type %q", block.Content.Type)
	}

	return schema.NewContentBlock(&schema.ServerToolResult{
		CallID: block.ToolUseID,
		Name:   string(ServerToolNameTextEditorCodeExecution),
		Content: &ServerToolResult{
			TextEditorCodeExecution: result,
		},
	}), nil
}

func toCodeExecutionOutputs(outputs []anthropic.CodeExecutionOutputBlock) []*CodeExecutionOutput {
	items := make([]*CodeExecutionOutput, 0, len(outputs))
	for _, output := range outputs {
		items = append(items, &CodeExecutionOutput{FileID: output.FileID})
	}
	return items
}

func toBashCodeExecutionOutputs(outputs []anthropic.BashCodeExecutionOutputBlock) []*CodeExecutionOutput {
	items := make([]*CodeExecutionOutput, 0, len(outputs))
	for _, output := range outputs {
		items = append(items, &CodeExecutionOutput{FileID: output.FileID})
	}
	return items
}

func codeExecutionToolResultToBlockParam(result *CodeExecutionResult, callID string) (anthropic.ContentBlockParamUnion, error) {
	switch result.Type {
	case CodeExecutionResultTypeError:
		return anthropic.NewCodeExecutionToolResultBlock(anthropic.CodeExecutionToolResultErrorParam{
			ErrorCode: anthropic.CodeExecutionToolResultErrorCode(result.Error.Code),
		}, callID), nil

	case CodeExecutionResultTypeEncrypted:
		outputs := toCodeExecutionOutputBlockParams(result.EncryptedResult.Content)
		return anthropic.NewCodeExecutionToolResultBlock(anthropic.EncryptedCodeExecutionResultBlockParam{
			Content:         outputs,
			EncryptedStdout: result.EncryptedResult.EncryptedStdout,
			Stderr:          result.EncryptedResult.Stderr,
			ReturnCode:      result.EncryptedResult.ReturnCode,
		}, callID), nil

	case CodeExecutionResultTypeResult:
		outputs := toCodeExecutionOutputBlockParams(result.Result.Content)
		return anthropic.NewCodeExecutionToolResultBlock(anthropic.CodeExecutionResultBlockParam{
			Content:    outputs,
			Stdout:     result.Result.Stdout,
			Stderr:     result.Result.Stderr,
			ReturnCode: result.Result.ReturnCode,
		}, callID), nil

	default:
		return anthropic.ContentBlockParamUnion{}, fmt.Errorf("invalid code execution result type %q", result.Type)
	}
}

func bashCodeExecutionToolResultToBlockParam(result *BashCodeExecutionResult, callID string) (anthropic.ContentBlockParamUnion, error) {
	switch result.Type {
	case BashCodeExecutionResultTypeError:
		return anthropic.NewBashCodeExecutionToolResultBlock(anthropic.BashCodeExecutionToolResultErrorParam{
			ErrorCode: anthropic.BashCodeExecutionToolResultErrorCode(result.Error.Code),
		}, callID), nil

	case BashCodeExecutionResultTypeResult:
		return anthropic.NewBashCodeExecutionToolResultBlock(anthropic.BashCodeExecutionResultBlockParam{
			Content:    toBashCodeExecutionOutputBlockParams(result.Result.Content),
			Stdout:     result.Result.Stdout,
			Stderr:     result.Result.Stderr,
			ReturnCode: result.Result.ReturnCode,
		}, callID), nil

	default:
		return anthropic.ContentBlockParamUnion{}, fmt.Errorf("invalid bash code execution result type %q", result.Type)
	}
}

func textEditorCodeExecutionToolResultToBlockParam(result *TextEditorCodeExecutionResult, callID string) (anthropic.ContentBlockParamUnion, error) {
	switch result.Type {
	case TextEditorCodeExecutionResultTypeError:
		blockParam := anthropic.NewTextEditorCodeExecutionToolResultBlock(anthropic.TextEditorCodeExecutionToolResultErrorParam{
			ErrorCode:    anthropic.TextEditorCodeExecutionToolResultErrorCode(result.Error.Code),
			ErrorMessage: param.NewOpt(result.Error.Message),
		}, callID)
		return blockParam, nil

	case TextEditorCodeExecutionResultTypeCreate:
		if result.Create == nil {
			return anthropic.ContentBlockParamUnion{}, fmt.Errorf("text editor code execution create result is nil")
		}
		return anthropic.NewTextEditorCodeExecutionToolResultBlock(anthropic.TextEditorCodeExecutionCreateResultBlockParam{
			IsFileUpdate: result.Create.IsFileUpdate,
		}, callID), nil

	case TextEditorCodeExecutionResultTypeStrReplace:
		if result.StrReplace == nil {
			return anthropic.ContentBlockParamUnion{}, fmt.Errorf("text editor code execution str replace result is nil")
		}
		return anthropic.NewTextEditorCodeExecutionToolResultBlock(anthropic.TextEditorCodeExecutionStrReplaceResultBlockParam{
			OldStart: param.NewOpt(result.StrReplace.OldStart),
			OldLines: param.NewOpt(result.StrReplace.OldLines),
			NewStart: param.NewOpt(result.StrReplace.NewStart),
			NewLines: param.NewOpt(result.StrReplace.NewLines),
			Lines:    result.StrReplace.Lines,
		}, callID), nil

	case TextEditorCodeExecutionResultTypeView:
		if result.View == nil {
			return anthropic.ContentBlockParamUnion{}, fmt.Errorf("text editor code execution view result is nil")
		}
		return anthropic.NewTextEditorCodeExecutionToolResultBlock(anthropic.TextEditorCodeExecutionViewResultBlockParam{
			FileType:   anthropic.TextEditorCodeExecutionViewResultBlockParamFileType(result.View.FileType),
			Content:    result.View.Content,
			NumLines:   param.NewOpt(result.View.NumLines),
			StartLine:  param.NewOpt(result.View.StartLine),
			TotalLines: param.NewOpt(result.View.TotalLines),
		}, callID), nil

	default:
		return anthropic.ContentBlockParamUnion{}, fmt.Errorf("invalid text editor code execution result type %q", result.Type)
	}
}

func toolSearchToolResultToBlockParam(result *ToolSearchToolResult, callID string) (anthropic.ContentBlockParamUnion, error) {
	switch result.Type {
	case ToolSearchToolResultTypeError:
		return anthropic.NewToolSearchToolResultBlock(anthropic.ToolSearchToolResultErrorParam{
			ErrorCode: anthropic.ToolSearchToolResultErrorCode(result.Error.Code),
		}, callID), nil

	case ToolSearchToolResultTypeSearchResult:
		return anthropic.NewToolSearchToolResultBlock(anthropic.ToolSearchToolSearchResultBlockParam{
			ToolReferences: toToolReferenceBlockParams(result.SearchResult.ToolReferences),
		}, callID), nil

	default:
		return anthropic.ContentBlockParamUnion{}, fmt.Errorf("invalid tool search tool result type %q", result.Type)
	}
}

func toCodeExecutionOutputBlockParams(outputs []*CodeExecutionOutput) []anthropic.CodeExecutionOutputBlockParam {
	items := make([]anthropic.CodeExecutionOutputBlockParam, 0, len(outputs))
	for _, output := range outputs {
		if output == nil {
			continue
		}
		items = append(items, anthropic.CodeExecutionOutputBlockParam{FileID: output.FileID})
	}
	return items
}

func toBashCodeExecutionOutputBlockParams(outputs []*CodeExecutionOutput) []anthropic.BashCodeExecutionOutputBlockParam {
	items := make([]anthropic.BashCodeExecutionOutputBlockParam, 0, len(outputs))
	for _, output := range outputs {
		if output == nil {
			continue
		}
		items = append(items, anthropic.BashCodeExecutionOutputBlockParam{FileID: output.FileID})
	}
	return items
}

func toToolReferenceBlockParams(references []*ToolSearchToolReference) []anthropic.ToolReferenceBlockParam {
	items := make([]anthropic.ToolReferenceBlockParam, 0, len(references))
	for _, reference := range references {
		if reference == nil {
			continue
		}
		items = append(items, anthropic.ToolReferenceBlockParam{
			ToolName: reference.ToolName,
		})
	}
	return items
}

func toWebFetchDocument(content anthropic.DocumentBlock) (*WebFetchDocument, error) {
	document := &WebFetchDocument{Title: content.Title}
	if content.Citations.Enabled {
		document.Citations = &WebFetchDocumentCitations{
			Enabled: content.Citations.Enabled,
		}
	}

	switch source := content.Source.AsAny().(type) {
	case anthropic.PlainTextSource:
		document.Source = &WebFetchDocumentSource{
			MIMEType: string(source.MediaType),
			Data:     source.Data,
		}
	case anthropic.Base64PDFSource:
		document.Source = &WebFetchDocumentSource{
			MIMEType: string(source.MediaType),
			Data:     source.Data,
		}
	default:
		return nil, fmt.Errorf("invalid web fetch content source %T", content.Source.AsAny())
	}

	return document, nil
}

func toWebFetchDocumentBlockParam(document *WebFetchDocument) (anthropic.DocumentBlockParam, error) {
	if document == nil {
		return anthropic.DocumentBlockParam{}, fmt.Errorf("web fetch content is nil")
	}

	documentBlockParam := anthropic.DocumentBlockParam{
		Title: param.NewOpt(document.Title),
	}
	if document.Citations != nil {
		documentBlockParam.Citations.Enabled = param.NewOpt(document.Citations.Enabled)
	}

	if document.Source == nil {
		return anthropic.DocumentBlockParam{}, fmt.Errorf("web fetch content source is nil")
	}

	switch document.Source.MIMEType {
	case "", "text/plain":
		documentBlockParam.Source.OfText = &anthropic.PlainTextSourceParam{
			Data: document.Source.Data,
		}
	case "application/pdf":
		documentBlockParam.Source.OfBase64 = &anthropic.Base64PDFSourceParam{
			Data: document.Source.Data,
		}
	default:
		return anthropic.DocumentBlockParam{}, fmt.Errorf("invalid web fetch content mime type %q", document.Source.MIMEType)
	}

	return documentBlockParam, nil
}

func toDeltaResponseMeta(delta anthropic.MessageDeltaEvent) *schema.AgenticResponseMeta {
	promptTokens := int(delta.Usage.InputTokens + delta.Usage.CacheReadInputTokens + delta.Usage.CacheCreationInputTokens)
	completionTokens := int(delta.Usage.OutputTokens)

	return &schema.AgenticResponseMeta{
		TokenUsage: &schema.TokenUsage{
			PromptTokens: promptTokens,
			PromptTokenDetails: schema.PromptTokenDetails{
				CachedTokens: int(delta.Usage.CacheReadInputTokens),
			},
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
		},
		ClaudeExtension: &claude.ResponseMetaExtension{
			StopReason:   string(delta.Delta.StopReason),
			StopSequence: delta.Delta.StopSequence,
			StopDetails:  toClaudeStopDetails(delta.Delta.StopDetails),
		},
	}
}

func toClaudeStopDetails(details anthropic.RefusalStopDetails) *claude.StopDetails {
	if details.Category == "" && details.Explanation == "" {
		return nil
	}

	return &claude.StopDetails{
		Category:    string(details.Category),
		Explanation: details.Explanation,
	}
}

func toTokenUsage(usage anthropic.Usage) *schema.TokenUsage {
	promptTokens := int(usage.InputTokens + usage.CacheReadInputTokens + usage.CacheCreationInputTokens)
	completionTokens := int(usage.OutputTokens)
	return &schema.TokenUsage{
		PromptTokens: promptTokens,
		PromptTokenDetails: schema.PromptTokenDetails{
			CachedTokens: int(usage.CacheReadInputTokens),
		},
		CompletionTokens: completionTokens,
		TotalTokens:      promptTokens + completionTokens,
	}
}

func toAssistantTextExtension(citations []anthropic.TextCitationUnion) *claude.AssistantGenTextExtension {
	if len(citations) == 0 {
		return nil
	}

	items := make([]*claude.TextCitation, 0, len(citations))
	for _, citation := range citations {
		item := toClaudeTextCitation(citation)
		if item == nil {
			continue
		}
		items = append(items, item)
	}
	if len(items) == 0 {
		return nil
	}

	return &claude.AssistantGenTextExtension{
		Citations: items,
	}
}

func toAnthropicTextCitations(citations []*claude.TextCitation) ([]anthropic.TextCitationParamUnion, error) {
	if len(citations) == 0 {
		return nil, nil
	}

	result := make([]anthropic.TextCitationParamUnion, 0, len(citations))
	for _, citation := range citations {
		switch citation.Type {
		case claude.TextCitationTypeCharLocation:
			result = append(result, anthropic.TextCitationParamUnion{
				OfCharLocation: &anthropic.CitationCharLocationParam{
					CitedText:      citation.CharLocation.CitedText,
					DocumentTitle:  newClaudeStrOpt(citation.CharLocation.DocumentTitle),
					DocumentIndex:  int64(citation.CharLocation.DocumentIndex),
					StartCharIndex: int64(citation.CharLocation.StartCharIndex),
					EndCharIndex:   int64(citation.CharLocation.EndCharIndex),
				},
			})

		case claude.TextCitationTypePageLocation:
			result = append(result, anthropic.TextCitationParamUnion{
				OfPageLocation: &anthropic.CitationPageLocationParam{
					CitedText:       citation.PageLocation.CitedText,
					DocumentTitle:   newClaudeStrOpt(citation.PageLocation.DocumentTitle),
					DocumentIndex:   int64(citation.PageLocation.DocumentIndex),
					StartPageNumber: int64(citation.PageLocation.StartPageNumber),
					EndPageNumber:   int64(citation.PageLocation.EndPageNumber),
				},
			})

		case claude.TextCitationTypeContentBlockLocation:
			result = append(result, anthropic.TextCitationParamUnion{
				OfContentBlockLocation: &anthropic.CitationContentBlockLocationParam{
					CitedText:       citation.ContentBlockLocation.CitedText,
					DocumentTitle:   newClaudeStrOpt(citation.ContentBlockLocation.DocumentTitle),
					DocumentIndex:   int64(citation.ContentBlockLocation.DocumentIndex),
					StartBlockIndex: int64(citation.ContentBlockLocation.StartBlockIndex),
					EndBlockIndex:   int64(citation.ContentBlockLocation.EndBlockIndex),
				},
			})

		case claude.TextCitationTypeWebSearchResultLocation:
			result = append(result, anthropic.TextCitationParamUnion{
				OfWebSearchResultLocation: &anthropic.CitationWebSearchResultLocationParam{
					CitedText:      citation.WebSearchResultLocation.CitedText,
					Title:          newClaudeStrOpt(citation.WebSearchResultLocation.Title),
					URL:            citation.WebSearchResultLocation.URL,
					EncryptedIndex: citation.WebSearchResultLocation.EncryptedIndex,
				},
			})

		default:
			return nil, fmt.Errorf("invalid text citation type %q", citation.Type)
		}
	}

	return result, nil
}

type claudeTextCitationFields struct {
	typ             string
	citedText       string
	documentTitle   string
	documentIndex   int64
	startCharIndex  int64
	endCharIndex    int64
	startPageNumber int64
	endPageNumber   int64
	startBlockIndex int64
	endBlockIndex   int64
	title           string
	url             string
	encryptedIndex  string
}

func toClaudeTextCitation(citation anthropic.TextCitationUnion) *claude.TextCitation {
	return toClaudeTextCitationFields(claudeTextCitationFields{
		typ:             citation.Type,
		citedText:       citation.CitedText,
		documentTitle:   citation.DocumentTitle,
		documentIndex:   citation.DocumentIndex,
		startCharIndex:  citation.StartCharIndex,
		endCharIndex:    citation.EndCharIndex,
		startPageNumber: citation.StartPageNumber,
		endPageNumber:   citation.EndPageNumber,
		startBlockIndex: citation.StartBlockIndex,
		endBlockIndex:   citation.EndBlockIndex,
		title:           citation.Title,
		url:             citation.URL,
		encryptedIndex:  citation.EncryptedIndex,
	})
}

func toClaudeTextCitationFromDelta(citation anthropic.CitationsDeltaCitationUnion) *claude.TextCitation {
	return toClaudeTextCitationFields(claudeTextCitationFields{
		typ:             citation.Type,
		citedText:       citation.CitedText,
		documentTitle:   citation.DocumentTitle,
		documentIndex:   citation.DocumentIndex,
		startCharIndex:  citation.StartCharIndex,
		endCharIndex:    citation.EndCharIndex,
		startPageNumber: citation.StartPageNumber,
		endPageNumber:   citation.EndPageNumber,
		startBlockIndex: citation.StartBlockIndex,
		endBlockIndex:   citation.EndBlockIndex,
		title:           citation.Title,
		url:             citation.URL,
		encryptedIndex:  citation.EncryptedIndex,
	})
}

func toClaudeTextCitationFields(fields claudeTextCitationFields) *claude.TextCitation {
	item := &claude.TextCitation{
		Type: claude.TextCitationType(fields.typ),
	}

	switch claude.TextCitationType(fields.typ) {
	case claude.TextCitationTypeCharLocation:
		item.CharLocation = &claude.CitationCharLocation{
			CitedText:      fields.citedText,
			DocumentTitle:  fields.documentTitle,
			DocumentIndex:  int(fields.documentIndex),
			StartCharIndex: int(fields.startCharIndex),
			EndCharIndex:   int(fields.endCharIndex),
		}

	case claude.TextCitationTypePageLocation:
		item.PageLocation = &claude.CitationPageLocation{
			CitedText:       fields.citedText,
			DocumentTitle:   fields.documentTitle,
			DocumentIndex:   int(fields.documentIndex),
			StartPageNumber: int(fields.startPageNumber),
			EndPageNumber:   int(fields.endPageNumber),
		}

	case claude.TextCitationTypeContentBlockLocation:
		item.ContentBlockLocation = &claude.CitationContentBlockLocation{
			CitedText:       fields.citedText,
			DocumentTitle:   fields.documentTitle,
			DocumentIndex:   int(fields.documentIndex),
			StartBlockIndex: int(fields.startBlockIndex),
			EndBlockIndex:   int(fields.endBlockIndex),
		}

	case claude.TextCitationTypeWebSearchResultLocation:
		item.WebSearchResultLocation = &claude.CitationWebSearchResultLocation{
			CitedText:      fields.citedText,
			Title:          fields.title,
			URL:            fields.url,
			EncryptedIndex: fields.encryptedIndex,
		}

	default:
		return nil
	}

	return item
}

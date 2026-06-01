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
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/eino/schema/claude"
)

type streamConverter struct {
	blockKinds      map[int64]schema.ContentBlockType
	serverToolNames map[int64]ServerToolName
}

func newStreamConverter() *streamConverter {
	return &streamConverter{
		blockKinds:      make(map[int64]schema.ContentBlockType),
		serverToolNames: make(map[int64]ServerToolName),
	}
}

func (c *streamConverter) toMessageStreamingChunk(event anthropic.MessageStreamEventUnion) (*schema.AgenticMessage, error) {
	switch e := event.AsAny().(type) {
	case anthropic.MessageStartEvent:
		return toAgenticMessage(&e.Message)
	case anthropic.MessageDeltaEvent:
		return &schema.AgenticMessage{
			Role:         schema.AgenticRoleTypeAssistant,
			ResponseMeta: toDeltaResponseMeta(e),
		}, nil
	case anthropic.ContentBlockStartEvent:
		contentBlock, err := c.toStreamingStartBlock(e.Index, e.ContentBlock.AsAny())
		if err != nil {
			return nil, err
		}
		return newAssistantStreamingChunk(contentBlock), nil
	case anthropic.ContentBlockDeltaEvent:
		contentBlock, err := c.toStreamingDeltaBlock(e.Index, e.Delta.AsAny())
		if err != nil {
			return nil, err
		}
		return newAssistantStreamingChunk(contentBlock), nil
	default:
		return nil, nil
	}
}

func newAssistantStreamingChunk(contentBlock *schema.ContentBlock) *schema.AgenticMessage {
	if contentBlock == nil {
		return nil
	}
	return &schema.AgenticMessage{
		Role:          schema.AgenticRoleTypeAssistant,
		ContentBlocks: []*schema.ContentBlock{contentBlock},
	}
}

func (c *streamConverter) toStreamingStartBlock(index int64, block any) (contentBlock *schema.ContentBlock, err error) {
	switch item := block.(type) {
	case anthropic.TextBlock:
		return c.toStreamingTextStartBlock(index, item), nil
	case anthropic.ThinkingBlock:
		return c.toStreamingThinkingStartBlock(index, item), nil
	case anthropic.ToolUseBlock:
		return c.toStreamingFunctionToolCallStartBlock(index, item), nil
	case anthropic.ServerToolUseBlock:
		return c.toStreamingServerToolCallStartBlock(index, item)
	case anthropic.WebSearchToolResultBlock:
		return c.toStreamingWebSearchToolResultStartBlock(index, item)
	case anthropic.WebFetchToolResultBlock:
		return c.toStreamingWebFetchToolResultStartBlock(index, item)
	case anthropic.CodeExecutionToolResultBlock:
		return c.toStreamingCodeExecutionToolResultStartBlock(index, item)
	case anthropic.BashCodeExecutionToolResultBlock:
		return c.toStreamingBashCodeExecutionToolResultStartBlock(index, item)
	case anthropic.TextEditorCodeExecutionToolResultBlock:
		return c.toStreamingTextEditorCodeExecutionToolResultStartBlock(index, item)
	default:
		return nil, nil
	}
}

func (c *streamConverter) toStreamingDeltaBlock(index int64, delta any) (contentBlock *schema.ContentBlock, err error) {
	meta := &schema.StreamingMeta{Index: int(index)}

	switch item := delta.(type) {
	case anthropic.TextDelta:
		return schema.NewContentBlockChunk(&schema.AssistantGenText{Text: item.Text}, meta), nil

	case anthropic.CitationsDelta:
		citation := toClaudeTextCitationFromDelta(item.Citation)
		if citation == nil {
			return nil, nil
		}
		return schema.NewContentBlockChunk(&schema.AssistantGenText{
			ClaudeExtension: &claude.AssistantGenTextExtension{
				Citations: []*claude.TextCitation{citation},
			},
		}, meta), nil

	case anthropic.ThinkingDelta:
		return schema.NewContentBlockChunk(&schema.Reasoning{Text: item.Thinking}, meta), nil

	case anthropic.SignatureDelta:
		return schema.NewContentBlockChunk(&schema.Reasoning{Signature: item.Signature}, meta), nil

	case anthropic.InputJSONDelta:
		if c.blockKinds[index] == schema.ContentBlockTypeServerToolCall {
			return c.toStreamingServerToolCallDeltaBlock(index, item.PartialJSON, meta)
		}
		return schema.NewContentBlockChunk(&schema.FunctionToolCall{Arguments: item.PartialJSON}, meta), nil

	default:
		return nil, fmt.Errorf("invalid stream delta type %T", delta)
	}
}

func (c *streamConverter) toStreamingServerToolCallDeltaBlock(index int64, partialJSON string, meta *schema.StreamingMeta) (contentBlock *schema.ContentBlock, err error) {
	switch c.serverToolNames[index] {
	case ServerToolNameWebSearch:
		return toStreamingWebSearchToolCallDeltaBlock(partialJSON, meta)
	case ServerToolNameWebFetch:
		return toStreamingWebFetchToolCallDeltaBlock(partialJSON, meta)
	case ServerToolNameCodeExecution:
		return toStreamingCodeExecutionToolCallDeltaBlock(partialJSON, meta)
	case ServerToolNameBashCodeExecution:
		return toStreamingBashCodeExecutionToolCallDeltaBlock(partialJSON, meta)
	case ServerToolNameTextEditorCodeExecution:
		return toStreamingTextEditorCodeExecutionToolCallDeltaBlock(partialJSON, meta)
	case ServerToolNameToolSearchToolBm25:
		return toStreamingToolSearchToolBm25CallDeltaBlock(partialJSON, meta)
	case ServerToolNameToolSearchToolRegex:
		return toStreamingToolSearchToolRegexCallDeltaBlock(partialJSON, meta)
	default:
		return nil, fmt.Errorf("invalid server tool name %q", c.serverToolNames[index])
	}
}

func toStreamingWebSearchToolCallDeltaBlock(partialJSON string, meta *schema.StreamingMeta) (contentBlock *schema.ContentBlock, err error) {
	args := &WebSearchArguments{}
	if err := sonic.UnmarshalString(partialJSON, args); err != nil {
		return nil, fmt.Errorf("failed to decode %s server tool arguments: %w", ServerToolNameWebSearch, err)
	}

	return schema.NewContentBlockChunk(&schema.ServerToolCall{
		Arguments: &ServerToolCallArguments{
			WebSearch: args,
		},
	}, meta), nil
}

func toStreamingWebFetchToolCallDeltaBlock(partialJSON string, meta *schema.StreamingMeta) (contentBlock *schema.ContentBlock, err error) {
	args := &WebFetchArguments{}
	if err := sonic.UnmarshalString(partialJSON, args); err != nil {
		return nil, fmt.Errorf("failed to decode %s server tool arguments: %w", ServerToolNameWebFetch, err)
	}

	return schema.NewContentBlockChunk(&schema.ServerToolCall{
		Arguments: &ServerToolCallArguments{
			WebFetch: args,
		},
	}, meta), nil
}

func toStreamingCodeExecutionToolCallDeltaBlock(partialJSON string, meta *schema.StreamingMeta) (contentBlock *schema.ContentBlock, err error) {
	args := &CodeExecutionArguments{}
	if err := sonic.UnmarshalString(partialJSON, args); err != nil {
		return nil, fmt.Errorf("failed to decode %s server tool arguments: %w", ServerToolNameCodeExecution, err)
	}

	return schema.NewContentBlockChunk(&schema.ServerToolCall{
		Arguments: &ServerToolCallArguments{
			CodeExecution: args,
		},
	}, meta), nil
}

func toStreamingBashCodeExecutionToolCallDeltaBlock(partialJSON string, meta *schema.StreamingMeta) (contentBlock *schema.ContentBlock, err error) {
	args := &BashCodeExecutionArguments{}
	if err := sonic.UnmarshalString(partialJSON, args); err != nil {
		return nil, fmt.Errorf("failed to decode %s server tool arguments: %w", ServerToolNameBashCodeExecution, err)
	}

	return schema.NewContentBlockChunk(&schema.ServerToolCall{
		Arguments: &ServerToolCallArguments{
			BashCodeExecution: args,
		},
	}, meta), nil
}

func toStreamingTextEditorCodeExecutionToolCallDeltaBlock(partialJSON string, meta *schema.StreamingMeta) (contentBlock *schema.ContentBlock, err error) {
	args := &TextEditorCodeExecutionArguments{}
	if err := sonic.UnmarshalString(partialJSON, args); err != nil {
		return nil, fmt.Errorf("failed to decode %s server tool arguments: %w", ServerToolNameTextEditorCodeExecution, err)
	}

	return schema.NewContentBlockChunk(&schema.ServerToolCall{
		Arguments: &ServerToolCallArguments{
			TextEditorCodeExecution: args,
		},
	}, meta), nil
}

func toStreamingToolSearchToolBm25CallDeltaBlock(partialJSON string, meta *schema.StreamingMeta) (contentBlock *schema.ContentBlock, err error) {
	args := &ToolSearchToolBm25Arguments{}
	if err := sonic.UnmarshalString(partialJSON, args); err != nil {
		return nil, fmt.Errorf("failed to decode %s server tool arguments: %w", ServerToolNameToolSearchToolBm25, err)
	}

	return schema.NewContentBlockChunk(&schema.ServerToolCall{
		Arguments: &ServerToolCallArguments{
			ToolSearchToolBm25: args,
		},
	}, meta), nil
}

func toStreamingToolSearchToolRegexCallDeltaBlock(partialJSON string, meta *schema.StreamingMeta) (contentBlock *schema.ContentBlock, err error) {
	args := &ToolSearchToolRegexArguments{}
	if err := sonic.UnmarshalString(partialJSON, args); err != nil {
		return nil, fmt.Errorf("failed to decode %s server tool arguments: %w", ServerToolNameToolSearchToolRegex, err)
	}

	return schema.NewContentBlockChunk(&schema.ServerToolCall{
		Arguments: &ServerToolCallArguments{
			ToolSearchToolRegex: args,
		},
	}, meta), nil
}

func (c *streamConverter) toStreamingServerToolCallStartBlock(index int64, block anthropic.ServerToolUseBlock) (contentBlock *schema.ContentBlock, err error) {
	contentBlock, err = serverToolUseToContentBlock(block)
	if err != nil {
		return nil, err
	}

	c.blockKinds[index] = schema.ContentBlockTypeServerToolCall
	c.serverToolNames[index] = ServerToolName(block.Name)

	return schema.NewContentBlockChunk(contentBlock.ServerToolCall, &schema.StreamingMeta{Index: int(index)}), nil
}

func (c *streamConverter) toStreamingTextStartBlock(index int64, block anthropic.TextBlock) *schema.ContentBlock {
	c.blockKinds[index] = schema.ContentBlockTypeAssistantGenText
	return schema.NewContentBlockChunk(&schema.AssistantGenText{
		Text:            block.Text,
		ClaudeExtension: toAssistantTextExtension(block.Citations),
	}, &schema.StreamingMeta{Index: int(index)})
}

func (c *streamConverter) toStreamingThinkingStartBlock(index int64, block anthropic.ThinkingBlock) *schema.ContentBlock {
	c.blockKinds[index] = schema.ContentBlockTypeReasoning
	return schema.NewContentBlockChunk(&schema.Reasoning{
		Text:      block.Thinking,
		Signature: block.Signature,
	}, &schema.StreamingMeta{Index: int(index)})
}

func (c *streamConverter) toStreamingFunctionToolCallStartBlock(index int64, block anthropic.ToolUseBlock) *schema.ContentBlock {
	c.blockKinds[index] = schema.ContentBlockTypeFunctionToolCall
	return schema.NewContentBlockChunk(&schema.FunctionToolCall{
		CallID: block.ID,
		Name:   block.Name,
	}, &schema.StreamingMeta{Index: int(index)})
}

func (c *streamConverter) toStreamingWebSearchToolResultStartBlock(index int64, block anthropic.WebSearchToolResultBlock) (contentBlock *schema.ContentBlock, err error) {
	c.blockKinds[index] = schema.ContentBlockTypeServerToolResult
	contentBlock, err = webSearchToolResultToContentBlock(block)
	if err != nil {
		return nil, err
	}
	return schema.NewContentBlockChunk(contentBlock.ServerToolResult, &schema.StreamingMeta{Index: int(index)}), nil
}

func (c *streamConverter) toStreamingWebFetchToolResultStartBlock(index int64, block anthropic.WebFetchToolResultBlock) (contentBlock *schema.ContentBlock, err error) {
	c.blockKinds[index] = schema.ContentBlockTypeServerToolResult
	contentBlock, err = webFetchToolResultToContentBlock(block)
	if err != nil {
		return nil, err
	}
	return schema.NewContentBlockChunk(contentBlock.ServerToolResult, &schema.StreamingMeta{Index: int(index)}), nil
}

func (c *streamConverter) toStreamingCodeExecutionToolResultStartBlock(index int64, block anthropic.CodeExecutionToolResultBlock) (contentBlock *schema.ContentBlock, err error) {
	c.blockKinds[index] = schema.ContentBlockTypeServerToolResult
	contentBlock, err = codeExecutionToolResultToContentBlock(block)
	if err != nil {
		return nil, err
	}
	return schema.NewContentBlockChunk(contentBlock.ServerToolResult, &schema.StreamingMeta{Index: int(index)}), nil
}

func (c *streamConverter) toStreamingBashCodeExecutionToolResultStartBlock(index int64, block anthropic.BashCodeExecutionToolResultBlock) (contentBlock *schema.ContentBlock, err error) {
	c.blockKinds[index] = schema.ContentBlockTypeServerToolResult
	contentBlock, err = bashCodeExecutionToolResultToContentBlock(block)
	if err != nil {
		return nil, err
	}
	return schema.NewContentBlockChunk(contentBlock.ServerToolResult, &schema.StreamingMeta{Index: int(index)}), nil
}

func (c *streamConverter) toStreamingTextEditorCodeExecutionToolResultStartBlock(index int64, block anthropic.TextEditorCodeExecutionToolResultBlock) (contentBlock *schema.ContentBlock, err error) {
	c.blockKinds[index] = schema.ContentBlockTypeServerToolResult
	contentBlock, err = textEditorCodeExecutionToolResultToContentBlock(block)
	if err != nil {
		return nil, err
	}
	return schema.NewContentBlockChunk(contentBlock.ServerToolResult, &schema.StreamingMeta{Index: int(index)}), nil
}

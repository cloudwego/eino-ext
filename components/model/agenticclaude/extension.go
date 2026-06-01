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

	"github.com/cloudwego/eino/schema"
	"github.com/go-viper/mapstructure/v2"
)

type ServerToolCallArguments struct {
	WebSearch               *WebSearchArguments               `json:"web_search,omitempty" mapstructure:"web_search,omitempty"`
	WebFetch                *WebFetchArguments                `json:"web_fetch,omitempty" mapstructure:"web_fetch,omitempty"`
	CodeExecution           *CodeExecutionArguments           `json:"code_execution,omitempty" mapstructure:"code_execution,omitempty"`
	BashCodeExecution       *BashCodeExecutionArguments       `json:"bash_code_execution,omitempty" mapstructure:"bash_code_execution,omitempty"`
	TextEditorCodeExecution *TextEditorCodeExecutionArguments `json:"text_editor_code_execution,omitempty" mapstructure:"text_editor_code_execution,omitempty"`
	ToolSearchToolBm25      *ToolSearchToolBm25Arguments      `json:"tool_search_tool_bm25,omitempty" mapstructure:"tool_search_tool_bm25,omitempty"`
	ToolSearchToolRegex     *ToolSearchToolRegexArguments     `json:"tool_search_tool_regex,omitempty" mapstructure:"tool_search_tool_regex,omitempty"`
}

type ServerToolResult struct {
	WebSearch               *WebSearchResult               `json:"web_search,omitempty" mapstructure:"web_search,omitempty"`
	WebFetch                *WebFetchResult                `json:"web_fetch,omitempty" mapstructure:"web_fetch,omitempty"`
	CodeExecution           *CodeExecutionResult           `json:"code_execution,omitempty" mapstructure:"code_execution,omitempty"`
	BashCodeExecution       *BashCodeExecutionResult       `json:"bash_code_execution,omitempty" mapstructure:"bash_code_execution,omitempty"`
	TextEditorCodeExecution *TextEditorCodeExecutionResult `json:"text_editor_code_execution,omitempty" mapstructure:"text_editor_code_execution,omitempty"`
	ToolSearchToolBm25      *ToolSearchToolResult          `json:"tool_search_tool_bm25,omitempty" mapstructure:"tool_search_tool_bm25,omitempty"`
	ToolSearchToolRegex     *ToolSearchToolResult          `json:"tool_search_tool_regex,omitempty" mapstructure:"tool_search_tool_regex,omitempty"`
}

type WebSearchArguments struct {
	Query string `json:"query,omitempty" mapstructure:"query,omitempty"`
}

type WebSearchResult struct {
	Type   WebSearchResultType   `json:"type,omitempty" mapstructure:"type,omitempty"`
	Result *WebSearchResultBlock `json:"result,omitempty" mapstructure:"result,omitempty"`
	Error  *WebSearchResultError `json:"error,omitempty" mapstructure:"error,omitempty"`
}

type WebSearchResultBlock struct {
	Content []*WebSearchResultItem `json:"content,omitempty" mapstructure:"content,omitempty"`
}

type WebSearchResultItem struct {
	Title            string `json:"title,omitempty" mapstructure:"title,omitempty"`
	URL              string `json:"url,omitempty" mapstructure:"url,omitempty"`
	EncryptedContent string `json:"encrypted_content,omitempty" mapstructure:"encrypted_content,omitempty"`
	PageAge          string `json:"page_age,omitempty" mapstructure:"page_age,omitempty"`
}

type WebFetchArguments struct {
	URL string `json:"url,omitempty" mapstructure:"url,omitempty"`
}

type CodeExecutionArguments struct {
	Code string `json:"code,omitempty" mapstructure:"code,omitempty"`
}

type BashCodeExecutionArguments struct {
	Command string `json:"command,omitempty" mapstructure:"command,omitempty"`
}

type TextEditorCodeExecutionArguments struct {
	Command  string `json:"command,omitempty" mapstructure:"command,omitempty"`
	Path     string `json:"path,omitempty" mapstructure:"path,omitempty"`
	FileText string `json:"file_text,omitempty" mapstructure:"file_text,omitempty"`
	OldStr   string `json:"old_str,omitempty" mapstructure:"old_str,omitempty"`
	NewStr   string `json:"new_str,omitempty" mapstructure:"new_str,omitempty"`
}

type ToolSearchToolBm25Arguments struct {
	Query string `json:"query,omitempty" mapstructure:"query,omitempty"`
}

type ToolSearchToolRegexArguments struct {
	Query string `json:"query,omitempty" mapstructure:"query,omitempty"`
}

type WebFetchResult struct {
	Type   WebFetchResultType   `json:"type,omitempty" mapstructure:"type,omitempty"`
	Result *WebFetchResultBlock `json:"result,omitempty" mapstructure:"result,omitempty"`
	Error  *WebFetchResultError `json:"error,omitempty" mapstructure:"error,omitempty"`
}

type CodeExecutionOutput struct {
	FileID string `json:"file_id,omitempty" mapstructure:"file_id,omitempty"`
}

type CodeExecutionResult struct {
	Type            CodeExecutionResultType            `json:"type,omitempty" mapstructure:"type,omitempty"`
	Result          *CodeExecutionResultBlock          `json:"result,omitempty" mapstructure:"result,omitempty"`
	EncryptedResult *EncryptedCodeExecutionResultBlock `json:"encrypted_result,omitempty" mapstructure:"encrypted_result,omitempty"`
	Error           *CodeExecutionResultError          `json:"error,omitempty" mapstructure:"error,omitempty"`
}

type BashCodeExecutionResult struct {
	Type   BashCodeExecutionResultType   `json:"type,omitempty" mapstructure:"type,omitempty"`
	Result *BashCodeExecutionResultBlock `json:"result,omitempty" mapstructure:"result,omitempty"`
	Error  *BashCodeExecutionResultError `json:"error,omitempty" mapstructure:"error,omitempty"`
}

type TextEditorCodeExecutionResult struct {
	Type       TextEditorCodeExecutionResultType        `json:"type,omitempty" mapstructure:"type,omitempty"`
	View       *TextEditorCodeExecutionViewResult       `json:"view,omitempty" mapstructure:"view,omitempty"`
	Create     *TextEditorCodeExecutionCreateResult     `json:"create,omitempty" mapstructure:"create,omitempty"`
	StrReplace *TextEditorCodeExecutionStrReplaceResult `json:"str_replace,omitempty" mapstructure:"str_replace,omitempty"`
	Error      *TextEditorCodeExecutionResultError      `json:"error,omitempty" mapstructure:"error,omitempty"`
}

type ToolSearchToolResult struct {
	Type         ToolSearchToolResultType    `json:"type,omitempty" mapstructure:"type,omitempty"`
	SearchResult *ToolSearchToolSearchResult `json:"search_result,omitempty" mapstructure:"search_result,omitempty"`
	Error        *ToolSearchToolResultError  `json:"error,omitempty" mapstructure:"error,omitempty"`
}

type ToolSearchToolSearchResult struct {
	ToolReferences []*ToolSearchToolReference `json:"tool_references,omitempty" mapstructure:"tool_references,omitempty"`
}

type ToolSearchToolReference struct {
	ToolName string `json:"tool_name,omitempty" mapstructure:"tool_name,omitempty"`
}

type WebFetchDocument struct {
	Source    *WebFetchDocumentSource    `json:"source,omitempty" mapstructure:"source,omitempty"`
	Title     string                     `json:"title,omitempty" mapstructure:"title,omitempty"`
	Citations *WebFetchDocumentCitations `json:"citations,omitempty" mapstructure:"citations,omitempty"`
}

type WebSearchResultError struct {
	Code string `json:"code,omitempty" mapstructure:"code,omitempty"`
}

type WebFetchResultError struct {
	Code string `json:"code,omitempty" mapstructure:"code,omitempty"`
}

type CodeExecutionResultError struct {
	Code string `json:"code,omitempty" mapstructure:"code,omitempty"`
}

type BashCodeExecutionResultError struct {
	Code string `json:"code,omitempty" mapstructure:"code,omitempty"`
}

type TextEditorCodeExecutionResultError struct {
	Code    string `json:"code,omitempty" mapstructure:"code,omitempty"`
	Message string `json:"message,omitempty" mapstructure:"message,omitempty"`
}

type ToolSearchToolResultError struct {
	Code    string `json:"code,omitempty" mapstructure:"code,omitempty"`
	Message string `json:"message,omitempty" mapstructure:"message,omitempty"`
}

type WebFetchResultBlock struct {
	URL         string            `json:"url,omitempty" mapstructure:"url,omitempty"`
	RetrievedAt string            `json:"retrieved_at,omitempty" mapstructure:"retrieved_at,omitempty"`
	Content     *WebFetchDocument `json:"content,omitempty" mapstructure:"content,omitempty"`
}

type WebFetchDocumentSource struct {
	MIMEType string `json:"media_type,omitempty" mapstructure:"media_type,omitempty"`
	Data     string `json:"data,omitempty" mapstructure:"data,omitempty"`
}

type WebFetchDocumentCitations struct {
	Enabled bool `json:"enabled,omitempty" mapstructure:"enabled,omitempty"`
}

type CodeExecutionResultBlock struct {
	Content    []*CodeExecutionOutput `json:"content,omitempty" mapstructure:"content,omitempty"`
	Stdout     string                 `json:"stdout,omitempty" mapstructure:"stdout,omitempty"`
	Stderr     string                 `json:"stderr,omitempty" mapstructure:"stderr,omitempty"`
	ReturnCode int64                  `json:"return_code,omitempty" mapstructure:"return_code,omitempty"`
}

type EncryptedCodeExecutionResultBlock struct {
	Content         []*CodeExecutionOutput `json:"content,omitempty" mapstructure:"content,omitempty"`
	EncryptedStdout string                 `json:"encrypted_stdout,omitempty" mapstructure:"encrypted_stdout,omitempty"`
	Stderr          string                 `json:"stderr,omitempty" mapstructure:"stderr,omitempty"`
	ReturnCode      int64                  `json:"return_code,omitempty" mapstructure:"return_code,omitempty"`
}

type BashCodeExecutionResultBlock struct {
	Content    []*CodeExecutionOutput `json:"content,omitempty" mapstructure:"content,omitempty"`
	Stdout     string                 `json:"stdout,omitempty" mapstructure:"stdout,omitempty"`
	Stderr     string                 `json:"stderr,omitempty" mapstructure:"stderr,omitempty"`
	ReturnCode int64                  `json:"return_code,omitempty" mapstructure:"return_code,omitempty"`
}

type TextEditorCodeExecutionViewResult struct {
	FileType   string `json:"file_type,omitempty" mapstructure:"file_type,omitempty"`
	Content    string `json:"content,omitempty" mapstructure:"content,omitempty"`
	NumLines   int64  `json:"numLines,omitempty" mapstructure:"numLines,omitempty"`
	StartLine  int64  `json:"startLine,omitempty" mapstructure:"startLine,omitempty"`
	TotalLines int64  `json:"totalLines,omitempty" mapstructure:"totalLines,omitempty"`
}

type TextEditorCodeExecutionCreateResult struct {
	IsFileUpdate bool `json:"is_file_update,omitempty" mapstructure:"is_file_update,omitempty"`
}

type TextEditorCodeExecutionStrReplaceResult struct {
	OldStart int64    `json:"oldStart,omitempty" mapstructure:"oldStart,omitempty"`
	OldLines int64    `json:"oldLines,omitempty" mapstructure:"oldLines,omitempty"`
	NewStart int64    `json:"newStart,omitempty" mapstructure:"newStart,omitempty"`
	NewLines int64    `json:"newLines,omitempty" mapstructure:"newLines,omitempty"`
	Lines    []string `json:"lines,omitempty" mapstructure:"lines,omitempty"`
}

func getServerToolCallArguments(call *schema.ServerToolCall) (*ServerToolCallArguments, error) {
	if call == nil || call.Arguments == nil {
		return nil, fmt.Errorf("server tool call arguments are nil")
	}
	if args, ok := call.Arguments.(*ServerToolCallArguments); ok {
		return args, nil
	}
	if m, ok := call.Arguments.(map[string]any); ok {
		args := &ServerToolCallArguments{}
		if err := mapstructure.Decode(m, args); err != nil {
			return nil, fmt.Errorf("failed to decode server tool call arguments: %w", err)
		}
		return args, nil
	}
	return nil, fmt.Errorf("unexpected type %T for server tool call arguments", call.Arguments)
}

func getServerToolResult(res *schema.ServerToolResult) (*ServerToolResult, error) {
	if res == nil || res.Content == nil {
		return nil, fmt.Errorf("server tool result is nil")
	}
	if result, ok := res.Content.(*ServerToolResult); ok {
		return result, nil
	}
	if m, ok := res.Content.(map[string]any); ok {
		result := &ServerToolResult{}
		if err := mapstructure.Decode(m, result); err != nil {
			return nil, fmt.Errorf("failed to decode server tool result: %w", err)
		}
		return result, nil
	}
	return nil, fmt.Errorf("unexpected type %T for server tool result", res.Content)
}

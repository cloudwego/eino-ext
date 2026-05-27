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

type ServerToolName string

const (
	ServerToolNameWebSearch               ServerToolName = "web_search"
	ServerToolNameWebFetch                ServerToolName = "web_fetch"
	ServerToolNameCodeExecution           ServerToolName = "code_execution"
	ServerToolNameBashCodeExecution       ServerToolName = "bash_code_execution"
	ServerToolNameTextEditorCodeExecution ServerToolName = "text_editor_code_execution"
	ServerToolNameToolSearchToolBm25      ServerToolName = "tool_search_tool_bm25"
	ServerToolNameToolSearchToolRegex     ServerToolName = "tool_search_tool_regex"
)

type WebSearchResultType string

const (
	WebSearchResultTypeResult WebSearchResultType = "web_search_result"
	WebSearchResultTypeError  WebSearchResultType = "web_search_tool_result_error"
)

type WebFetchResultType string

const (
	WebFetchResultTypeResult WebFetchResultType = "web_fetch_result"
	WebFetchResultTypeError  WebFetchResultType = "web_fetch_tool_result_error"
)

type CodeExecutionResultType string

const (
	CodeExecutionResultTypeResult    CodeExecutionResultType = "code_execution_result"
	CodeExecutionResultTypeEncrypted CodeExecutionResultType = "encrypted_code_execution_result"
	CodeExecutionResultTypeError     CodeExecutionResultType = "code_execution_tool_result_error"
)

type BashCodeExecutionResultType string

const (
	BashCodeExecutionResultTypeResult BashCodeExecutionResultType = "bash_code_execution_result"
	BashCodeExecutionResultTypeError  BashCodeExecutionResultType = "bash_code_execution_tool_result_error"
)

type TextEditorCodeExecutionResultType string

const (
	TextEditorCodeExecutionResultTypeError      TextEditorCodeExecutionResultType = "text_editor_code_execution_tool_result_error"
	TextEditorCodeExecutionResultTypeCreate     TextEditorCodeExecutionResultType = "text_editor_code_execution_create_result"
	TextEditorCodeExecutionResultTypeStrReplace TextEditorCodeExecutionResultType = "text_editor_code_execution_str_replace_result"
	TextEditorCodeExecutionResultTypeView       TextEditorCodeExecutionResultType = "text_editor_code_execution_view_result"
)

type ToolSearchToolResultType string

const (
	ToolSearchToolResultTypeSearchResult ToolSearchToolResultType = "tool_search_tool_search_result"
	ToolSearchToolResultTypeError        ToolSearchToolResultType = "tool_search_tool_result_error"
)

const (
	serverToolVersionWebSearch20260209           = "web_search_20260209"
	serverToolVersionWebFetch20260309            = "web_fetch_20260309"
	serverToolVersionCodeExecution20260120       = "code_execution_20260120"
	serverToolVersionToolSearchToolBm25_20251119 = "tool_search_tool_bm25_20251119"
	serverToolVersionToolSearchToolRegex20251119 = "tool_search_tool_regex_20251119"
)

const (
	headerAnthropicBeta        = "anthropic-beta"
	betaHeaderWebFetch20260309 = "web-fetch-2026-03-09"
)

const implType = "AgenticClaude"

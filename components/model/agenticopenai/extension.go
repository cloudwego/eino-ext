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

	"github.com/cloudwego/eino/schema"
)

type ServerToolCallArguments struct {
	WebSearch *WebSearchArguments `json:"web_search,omitempty"`
}

type ServerToolResult struct {
	WebSearch *WebSearchResult `json:"web_search,omitempty"`
}

type WebSearchArguments struct {
	ActionType WebSearchAction `json:"action_type,omitempty"`

	Search   *WebSearchQuery    `json:"search,omitempty"`
	OpenPage *WebSearchOpenPage `json:"open_page,omitempty"`
	Find     *WebSearchFind     `json:"find,omitempty"`
}

type WebSearchQuery struct {
	Query string `json:"query,omitempty"`
}

type WebSearchOpenPage struct {
	URL string `json:"url,omitempty"`
}

type WebSearchFind struct {
	URL     string `json:"url,omitempty"`
	Pattern string `json:"pattern,omitempty"`
}

type WebSearchQueryResult struct {
	Sources []*WebSearchQuerySource `json:"sources,omitempty"`
}

type WebSearchQuerySource struct {
	URL string `json:"url,omitempty"`
}

type WebSearchResult struct {
	ActionType WebSearchAction `json:"action_type,omitempty"`

	Search *WebSearchQueryResult `json:"search,omitempty"`
}

func getServerToolCallArguments(call *schema.ServerToolCall) (*ServerToolCallArguments, error) {
	if call == nil || call.Arguments == nil {
		return nil, fmt.Errorf("server tool call arguments are nil")
	}
	arguments, ok := call.Arguments.(*ServerToolCallArguments)
	if !ok {
		return nil, fmt.Errorf("expected %T, but got %T", &ServerToolCallArguments{}, call.Arguments)
	}
	return arguments, nil
}

func getServerToolResult(content *schema.ServerToolResult) (*ServerToolResult, error) {
	if content == nil || content.Result == nil {
		return nil, fmt.Errorf("server tool result is nil")
	}
	result, ok := content.Result.(*ServerToolResult)
	if !ok {
		return nil, fmt.Errorf("expected %T, but got %T", &ServerToolResult{}, content.Result)
	}
	return result, nil
}

func concatServerToolCallArguments(chunks []*ServerToolCallArguments) (ret *ServerToolCallArguments, err error) {
	if len(chunks) == 0 {
		return nil, fmt.Errorf("no server tool call arguments found")
	}
	if len(chunks) == 1 {
		return chunks[0], nil
	}
	return nil, fmt.Errorf("cannot concat multiple server tool call arguments")
}

func concatServerToolResult(chunks []*ServerToolResult) (ret *ServerToolResult, err error) {
	if len(chunks) == 0 {
		return nil, fmt.Errorf("no server tool result found")
	}
	if len(chunks) == 1 {
		return chunks[0], nil
	}
	return nil, fmt.Errorf("cannot concat multiple server tool result")
}

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

	"github.com/cloudwego/eino/schema"
	"github.com/openai/openai-go/v3/packages/param"
	"github.com/openai/openai-go/v3/responses"
)

type blockExtraMCPErrorJSON string

const mcpErrorJSONKey = "openai-mcp-error-json"

// Eino's MCPToolCallError has only Code/Message, while the SDK now models
// protocol, HTTP and tool-execution errors (the last carries arbitrary content).
// Preserve the wire union in block extras for lossless history replay, alongside
// a useful projection for callers. Do not infer the union type from its code:
// an HTTP status and an MCP protocol code belong to different namespaces.
func mcpErrorFromResponse(item responses.ResponseOutputItemMcpCall) (*schema.MCPToolCallError, string, error) {
	value := item.Error
	// The outer field preserves legacy string values that the structured
	// union decoder cannot populate, as well as explicit null.
	raw := item.JSON.Error.Raw()
	if raw == "" {
		raw = value.RawJSON()
	}
	if raw == "null" {
		return nil, "", nil
	}
	if raw == "" {
		if value.Type == "" && value.Message == "" && value.Code == 0 && value.Content == nil {
			return nil, "", nil
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, "", fmt.Errorf("encode MCP error: %w", err)
		}
		raw = string(encoded)
	}
	var legacy string
	if json.Unmarshal([]byte(raw), &legacy) == nil {
		if legacy == "" {
			return nil, "", nil
		}
		return &schema.MCPToolCallError{Message: legacy}, raw, nil
	}
	out := &schema.MCPToolCallError{Message: value.Message}
	switch value.Type {
	case "mcp_protocol_error", "http_error":
		out.Code = ptrOf(value.Code)
	case "mcp_tool_execution_error":
		encoded, err := json.Marshal(value.Content)
		if err != nil {
			return nil, "", fmt.Errorf("encode MCP tool execution error content: %w", err)
		}
		out.Message = string(encoded)
	default:
		// Unknown future variants remain errors and retain their full payload.
		if out.Message == "" {
			out.Message = raw
		}
	}
	return out, raw, nil
}

func mcpErrorToInputParam(block *schema.ContentBlock) (responses.McpToolCallErrorUnionParam, error) {
	var empty responses.McpToolCallErrorUnionParam
	value := block.MCPToolResult.Error
	if value == nil {
		return empty, nil
	}
	raw, ok := getBlockExtraValue[blockExtraMCPErrorJSON](block, mcpErrorJSONKey)
	if !ok {
		// JSON-persisted extras decode private string aliases as plain strings.
		text, _ := getBlockExtraValue[string](block, mcpErrorJSONKey)
		raw = blockExtraMCPErrorJSON(text)
	}
	if raw != "" {
		if !json.Valid([]byte(raw)) {
			return empty, fmt.Errorf("invalid MCP error replay metadata")
		}
		return param.Override[responses.McpToolCallErrorUnionParam](json.RawMessage(raw)), nil
	}
	// Hand-built or older Eino history lacks a provider variant. Preserve the
	// legacy string wire shape rather than inventing an HTTP/protocol error.
	// A caller that needs structured errors should replay original blocks.
	if value.Message == "" {
		return empty, nil
	}
	return param.Override[responses.McpToolCallErrorUnionParam](value.Message), nil
}

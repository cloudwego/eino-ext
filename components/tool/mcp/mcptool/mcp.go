/*
 * Copyright 2025 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package mcptool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/bytedance/sonic"
	"github.com/eino-contrib/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

type Config struct {
	// Sess is the MCP (Model Control Protocol) session, ref: https://github.com/modelcontextprotocol/go-sdk?tab=readme-ov-file#tools
	// Notice: should Initialize with server before use
	Sess *mcp.ClientSession

	// ToolNameList specifies which tools to fetch from MCP server
	// If empty, all available tools will be fetched
	ToolNameList []string
	// ToolCallResultHandler is a function that processes the result after a tool call completes
	// It can be used for custom processing of tool call results
	// If nil, no additional processing will be performed
	ToolCallResultHandler func(ctx context.Context, name string, result *mcp.CallToolResult) (*mcp.CallToolResult, error)

	// Cursor specifies the cursor passed to MCP server when requesting.
	// An opaque token representing the current pagination position. If provided,
	// the server should return results starting after this cursor.
	Cursor string
}

func GetTools(ctx context.Context, conf *Config) ([]tool.BaseTool, error) {
	if conf.Sess == nil {
		return nil, errors.New("mcp client session is nil")
	}

	listResults, err := conf.Sess.ListTools(ctx, &mcp.ListToolsParams{
		Cursor: conf.Cursor,
	})
	if err != nil {
		return nil, fmt.Errorf("list mcp tools fail: %w", err)
	}

	nameSet := make(map[string]struct{})
	for _, name := range conf.ToolNameList {
		nameSet[name] = struct{}{}
	}

	ret := make([]tool.BaseTool, 0, len(listResults.Tools))
	for _, t := range listResults.Tools {
		if len(conf.ToolNameList) > 0 {
			if _, ok := nameSet[t.Name]; !ok {
				continue
			}
		}

		marshaledInputSchema, err := sonic.Marshal(t.InputSchema)
		if err != nil {
			return nil, fmt.Errorf("conv mcp tool input schema fail(marshal): %w, tool name: %s", err, t.Name)
		}
		inputSchema := &jsonschema.Schema{}
		err = sonic.Unmarshal(marshaledInputSchema, inputSchema)
		if err != nil {
			return nil, fmt.Errorf("conv mcp tool input schema fail(unmarshal): %w, tool name: %s", err, t.Name)
		}

		ret = append(ret, &toolHelper{
			sess: conf.Sess,
			info: &schema.ToolInfo{
				Name:        t.Name,
				Desc:        t.Description,
				ParamsOneOf: schema.NewParamsOneOfByJSONSchema(inputSchema),
			},
			toolCallResultHandler: conf.ToolCallResultHandler,
		})
	}

	return ret, nil
}

type toolHelper struct {
	sess                  *mcp.ClientSession
	info                  *schema.ToolInfo
	toolCallResultHandler func(ctx context.Context, name string, result *mcp.CallToolResult) (*mcp.CallToolResult, error)
}

func (m *toolHelper) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return m.info, nil
}

func (m *toolHelper) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	result, err := m.sess.CallTool(ctx, &mcp.CallToolParams{
		Name:      m.info.Name,
		Arguments: json.RawMessage(argumentsInJSON),
	})
	if err != nil {
		return "", fmt.Errorf("failed to call mcp tool: %w", err)
	}

	if m.toolCallResultHandler != nil {
		result, err = m.toolCallResultHandler(ctx, m.info.Name, result)
		if err != nil {
			return "", fmt.Errorf("failed to execute mcp tool call result handler: %w", err)
		}
	}

	marshaledResult, err := sonic.MarshalString(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal mcp tool result: %w", err)
	}
	if result.IsError {
		return "", fmt.Errorf("failed to call mcp tool, mcp server return error: %s", marshaledResult)
	}

	return marshaledResult, nil
}

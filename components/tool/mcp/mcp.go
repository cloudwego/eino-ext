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

package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/bytedance/sonic"
	"github.com/eino-contrib/jsonschema"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	omcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

type Config struct {
	// Cli is the non-official MCP (Model Control Protocol) client, ref: https://github.com/mark3labs/mcp-go?tab=readme-ov-file#tools
	// Notice: should Initialize with server before use
	Cli client.MCPClient

	// MCPSess is the session provided by the official MCP (Model Control Protocol) SDK, ref: https://github.com/modelcontextprotocol/go-sdk?tab=readme-ov-file#tools
	// Notice: should Initialize with server before use, use MCPSess first, if MCPSess is nil, use Cli
	MCPSess *omcp.ClientSession

	// ToolNameList specifies which tools to fetch from MCP server
	// If empty, all available tools will be fetched
	ToolNameList []string
	// ToolCallResultHandler is a function that processes the result after a tool call completes
	// It can be used for custom processing of tool call results
	// If nil, no additional processing will be performed
	// Notice: when using MCPSess, ToolCallResultHandler will be ignored
	ToolCallResultHandler func(ctx context.Context, name string, result *mcp.CallToolResult) (*mcp.CallToolResult, error)

	// MCPToolCallResultHandler is a function that processes the result after a tool call completes
	// It can be used for custom processing of tool call results
	// If nil, no additional processing will be performed
	// Notice: when using Cli, MCPToolCallResultHandler will be ignored
	MCPToolCallResultHandler func(ctx context.Context, name string, result *omcp.CallToolResult) (*omcp.CallToolResult, error)

	// CustomHeaders specifies the http headers passed to mcp server when requesting.
	// Notice: when using MCPSess, CustomHeaders will be ignored
	CustomHeaders map[string]string
}

func GetTools(ctx context.Context, conf *Config) ([]tool.BaseTool, error) {
	infoTools, err := getMCPTools(ctx, conf)
	if err != nil {
		return nil, fmt.Errorf("get mcp tools fail: %w", err)
	}

	nameSet := make(map[string]struct{})
	for _, name := range conf.ToolNameList {
		nameSet[name] = struct{}{}
	}

	ret := make([]tool.BaseTool, 0, len(infoTools))
	for _, t := range infoTools {
		if len(conf.ToolNameList) > 0 {
			if _, ok := nameSet[t.Name]; !ok {
				continue
			}
		}

		ret = append(ret, &toolHelper{
			cli: conf.Cli,
			sess: conf.MCPSess,
			info: t,
			customHeaders: conf.CustomHeaders,
			toolCallResultHandler: conf.ToolCallResultHandler,
			officialToolCallResultHandler: conf.MCPToolCallResultHandler,
		})
	}

	return ret, nil
}

func getMCPTools(ctx context.Context, conf *Config) ([]*schema.ToolInfo, error) {
	var tools []*schema.ToolInfo
	var err error
	if conf.MCPSess != nil {
		tools, err = getOfficialMCPTools(ctx, conf)
	} else if conf.Cli != nil {
		tools, err = getNonOfficialMCPTools(ctx, conf)
	} else {
		err = errors.New("mcp config error: both MCPSess and Cli are nil")
	}
	return tools, err
}

func getNonOfficialMCPTools(ctx context.Context, conf *Config) ([]*schema.ToolInfo, error) {
	header := http.Header{}
	if conf.CustomHeaders != nil {
		for k, v := range conf.CustomHeaders {
			header.Set(k, v)
		}
	}

	listResults, err := conf.Cli.ListTools(ctx, mcp.ListToolsRequest{
		Header: header,
	})
	if err != nil {
		return nil, fmt.Errorf("list mcp tools fail: %w", err)
	}

	ret := make([]*schema.ToolInfo, 0, len(listResults.Tools))
	for _, t := range listResults.Tools {
		marshaledInputSchema, err := sonic.Marshal(t.InputSchema)
		if err != nil {
			return nil, fmt.Errorf("conv mcp tool input schema fail(marshal): %w, tool name: %s", err, t.Name)
		}
		inputSchema := &jsonschema.Schema{}
		err = sonic.Unmarshal(marshaledInputSchema, inputSchema)
		if err != nil {
			return nil, fmt.Errorf("conv mcp tool input schema fail(unmarshal): %w, tool name: %s", err, t.Name)
		}
		ret = append(ret, &schema.ToolInfo{
			Name: t.Name,
			Desc: t.Description,
			ParamsOneOf: schema.NewParamsOneOfByJSONSchema(inputSchema),
		})
	}
	return ret, nil
}

func getOfficialMCPTools(ctx context.Context, conf *Config) ([]*schema.ToolInfo, error) {
	listResults, err := conf.MCPSess.ListTools(ctx, &omcp.ListToolsParams{})
	if err != nil {
		return nil, fmt.Errorf("list mcp tools fail: %w", err)
	}
	ret := make([]*schema.ToolInfo, 0, len(listResults.Tools))
	for _, t := range listResults.Tools {
		marshaledInputSchema, err := sonic.Marshal(t.InputSchema)
		if err != nil {
			return nil, fmt.Errorf("conv mcp tool input schema fail(marshal): %w, tool name: %s", err, t.Name)
		}
		inputSchema := &jsonschema.Schema{}
		err = sonic.Unmarshal(marshaledInputSchema, inputSchema)
		if err != nil {
			return nil, fmt.Errorf("conv mcp tool input schema fail(unmarshal): %w, tool name: %s", err, t.Name)
		}
		ret = append(ret, &schema.ToolInfo{
			Name: t.Name,
			Desc: t.Description,
			ParamsOneOf: schema.NewParamsOneOfByJSONSchema(inputSchema),
		})
	}

	return ret, nil
}

type toolHelper struct {
	cli                   client.MCPClient
	sess                  *omcp.ClientSession
	info                  *schema.ToolInfo
	customHeaders         map[string]string
	toolCallResultHandler func(ctx context.Context, name string, result *mcp.CallToolResult) (*mcp.CallToolResult, error)
	officialToolCallResultHandler func(ctx context.Context, name string, result *omcp.CallToolResult) (*omcp.CallToolResult, error)
}

func (m *toolHelper) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return m.info, nil
}

func (m *toolHelper) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var result string
	var err error
	if m.sess != nil {
		result, err = m.callOfficialMCPTool(ctx, argumentsInJSON, opts...)
	} else if m.cli != nil {
		result, err = m.callNonOfficialMCPTool(ctx, argumentsInJSON, opts...)
	} else {
		err = errors.New("mcp config error: both Sess and Cli are nil")
	}
	return result, err
}

func (m *toolHelper) callNonOfficialMCPTool(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
specOptions := tool.GetImplSpecificOptions(&mcpOptions{
		customHeaders: m.customHeaders,
	}, opts...)

	headers := http.Header{}
	if specOptions.customHeaders != nil {
		for k, v := range specOptions.customHeaders {
			headers.Set(k, v)
		}
	}

	result, err := m.cli.CallTool(ctx, mcp.CallToolRequest{
		Request: mcp.Request{
			Method: "tools/call",
		},
		Header: headers,
		Params: mcp.CallToolParams{
			Name:      m.info.Name,
			Arguments: json.RawMessage(argumentsInJSON),
		},
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

	if result == nil {
		return "", fmt.Errorf("failed to call mcp tool, mcp server return nil result")
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

func (m *toolHelper) callOfficialMCPTool(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	result, err := m.sess.CallTool(ctx, &omcp.CallToolParams{
		Name:      m.info.Name,
		Arguments: json.RawMessage(argumentsInJSON),
	})
	if err != nil {
		return "", fmt.Errorf("failed to call mcp tool: %w", err)
	}

	if m.officialToolCallResultHandler != nil {
		result, err = m.officialToolCallResultHandler(ctx, m.info.Name, result)
		if err != nil {
			return "", fmt.Errorf("failed to execute mcp tool call result handler: %w", err)
		}
	}

	if result == nil {
		return "", fmt.Errorf("failed to call mcp tool, mcp server return nil result")
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

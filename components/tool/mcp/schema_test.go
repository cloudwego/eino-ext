/*
 * Copyright 2026 CloudWeGo Authors
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
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetToolsPreservesInputSchema(t *testing.T) {
	for _, version := range []string{mcp.LATEST_PROTOCOL_VERSION, "2025-06-18"} {
		t.Run(version, func(t *testing.T) {
			testGetToolsPreservesInputSchema(t, version)
		})
	}
}

func testGetToolsPreservesInputSchema(t *testing.T, protocolVersion string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	source, err := os.ReadFile("README.md")
	require.NoError(t, err)
	files := httptest.NewServer(http.FileServer(http.Dir(".")))
	t.Cleanup(files.Close)

	inputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"$ref": "#/$defs/source"},
			"url": {"$ref": "#/$defs/source"}
		},
		"additionalProperties": false,
		"oneOf": [{"required": ["path"]}, {"required": ["url"]}],
		"anyOf": [{"required": ["path"]}, {"required": ["url"]}],
		"allOf": [{"minProperties": 1}, {"maxProperties": 1}],
		"not": {"required": ["blocked"]},
		"$defs": {"source": {"type": "string", "minLength": 1}}
	}`)
	svr := server.NewMCPServer("file-reader", "1.0.0", server.WithToolCapabilities(false))
	readFile := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if path := request.GetString("path", ""); path != "" {
			content, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}
			return mcp.NewToolResultText(string(content)), nil
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, request.GetString("url", ""), nil)
		if err != nil {
			return nil, err
		}
		response, err := files.Client().Do(req)
		if err != nil {
			return nil, err
		}
		defer response.Body.Close()
		content, err := io.ReadAll(response.Body)
		if err != nil {
			return nil, err
		}
		return mcp.NewToolResultText(string(content)), nil
	}
	svr.AddTool(mcp.NewToolWithRawSchema("read_file", "Read a local file or URL", inputSchema), readFile)

	var draft07 map[string]any
	require.NoError(t, json.Unmarshal(inputSchema, &draft07))
	draft07["definitions"] = draft07["$defs"]
	delete(draft07, "$defs")
	draft07["properties"] = map[string]any{
		"path": map[string]any{"$ref": "#/definitions/source"},
		"url":  map[string]any{"$ref": "#/definitions/source"},
	}
	legacySchema, err := json.Marshal(draft07)
	require.NoError(t, err)
	svr.AddTool(mcp.NewToolWithRawSchema("read_file_draft07", "Read a local file or URL", legacySchema), readFile)

	cli, err := client.NewInProcessClient(svr)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, cli.Close()) })
	require.NoError(t, cli.Start(ctx))
	init := mcp.InitializeRequest{}
	init.Params.ProtocolVersion = protocolVersion
	init.Params.ClientInfo = mcp.Implementation{Name: "schema-client", Version: "1.0.0"}
	_, err = cli.Initialize(ctx, init)
	require.NoError(t, err)

	tools, err := GetTools(ctx, &Config{Cli: cli})
	require.NoError(t, err)
	require.Len(t, tools, 2)
	for _, remoteTool := range tools {
		info, err := remoteTool.Info(ctx)
		require.NoError(t, err)
		params, err := info.ParamsOneOf.ToJSONSchema()
		require.NoError(t, err)
		encoded, err := json.Marshal(params)
		require.NoError(t, err)
		assert.JSONEq(t, string(inputSchema), string(encoded), info.Name)

		for _, arguments := range []map[string]string{
			{"path": "README.md"},
			{"url": files.URL + "/README.md"},
		} {
			encoded, err := json.Marshal(arguments)
			require.NoError(t, err)
			result, err := remoteTool.(tool.InvokableTool).InvokableRun(ctx, string(encoded))
			require.NoError(t, err)
			var output mcp.CallToolResult
			require.NoError(t, json.Unmarshal([]byte(result), &output))
			require.Len(t, output.Content, 1)
			assert.Equal(t, string(source), output.Content[0].(mcp.TextContent).Text)
		}
	}
}

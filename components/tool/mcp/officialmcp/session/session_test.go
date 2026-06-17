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

package session

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	officialmcp "github.com/cloudwego/eino-ext/components/tool/mcp/officialmcp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type addParams struct {
	X int `json:"x"`
	Y int `json:"y"`
}

func TestConnectSSEAndCloseIdempotent(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "server", Version: "v1.0.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "add", Description: "add two numbers"}, func(ctx context.Context, req *mcp.CallToolRequest, args addParams) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("%d", args.X+args.Y)}},
		}, nil, nil
	})
	httpServer := httptest.NewServer(mcp.NewSSEHandler(func(*http.Request) *mcp.Server { return server }, nil))
	defer httpServer.Close()

	managed, err := Connect(context.Background(), ServerConfig{
		Name: "test",
		Transport: TransportConfig{
			Type: TransportSSE,
			URL:  httpServer.URL,
		},
	})
	require.NoError(t, err)
	require.NotNil(t, managed)

	result, err := managed.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "add",
		Arguments: map[string]any{"x": 1, "y": 2},
	})
	require.NoError(t, err)
	assert.Equal(t, "3", result.Content[0].(*mcp.TextContent).Text)
	assert.NoError(t, managed.Close())
	assert.NoError(t, managed.Close())
}

func TestConnectStreamableHTTPWithHeaders(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "server", Version: "v1.0.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "add", Description: "add two numbers"}, func(ctx context.Context, req *mcp.CallToolRequest, args addParams) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("%d", args.X+args.Y)}},
		}, nil, nil
	})
	handler := mcp.NewStreamableHTTPHandler(func(req *http.Request) *mcp.Server {
		assert.Equal(t, "token", req.Header.Get("Authorization"))
		return server
	}, nil)
	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()

	managed, err := Connect(context.Background(), ServerConfig{
		Name: "test",
		Transport: TransportConfig{
			Type:    TransportStreamableHTTP,
			URL:     httpServer.URL,
			Headers: map[string]string{"Authorization": "token"},
		},
	})
	require.NoError(t, err)
	defer managed.Close()

	result, err := managed.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "add",
		Arguments: map[string]any{"x": 2, "y": 3},
	})
	require.NoError(t, err)
	assert.Equal(t, "5", result.Content[0].(*mcp.TextContent).Text)
}

func TestNewTransportStdioConfig(t *testing.T) {
	transport, err := newTransport(TransportConfig{
		Type:    TransportStdio,
		Command: "echo",
		Args:    []string{"hello"},
		Env:     map[string]string{"MCP_TEST_ENV": "value"},
		CWD:     "/tmp",
	})
	require.NoError(t, err)

	commandTransport := transport.(*mcp.CommandTransport)
	assert.Equal(t, "echo", filepath.Base(commandTransport.Command.Path))
	assert.Equal(t, []string{"echo", "hello"}, commandTransport.Command.Args)
	assert.Equal(t, "/tmp", commandTransport.Command.Dir)
	assert.Contains(t, commandTransport.Command.Env, "MCP_TEST_ENV=value")
}

func TestNewTransportRejectsEmptyStdioCommand(t *testing.T) {
	_, err := newTransport(TransportConfig{Type: TransportStdio})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stdio command is empty")
}

func TestSessionCloseNil(t *testing.T) {
	var managed *Session
	assert.NoError(t, managed.Close())
}

func TestConnectRejectsInvalidURL(t *testing.T) {
	_, err := Connect(context.Background(), ServerConfig{
		Name: "bad",
		Transport: TransportConfig{
			Type: TransportSSE,
			URL:  "/relative",
		},
	})
	require.Error(t, err)
	var startupErr *StartupError
	assert.ErrorAs(t, err, &startupErr)
	assert.Contains(t, err.Error(), "transport URL must be absolute")
}

func TestConnectUnsupportedTransport(t *testing.T) {
	_, err := Connect(context.Background(), ServerConfig{
		Name:      "bad",
		Transport: TransportConfig{Type: "unknown"},
	})
	require.Error(t, err)
	var startupErr *StartupError
	require.ErrorAs(t, err, &startupErr)
	assert.True(t, officialmcp.IsErrorKind(startupErr.Err, ErrorKindUnsupportedTransport))
}

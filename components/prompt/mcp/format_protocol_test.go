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
	"os"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/require"
)

func TestFormatWithMCPServer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	source, err := os.ReadFile("README.md")
	require.NoError(t, err)

	svr := server.NewMCPServer("file-prompt", "1.0.0", server.WithPromptCapabilities(false))
	svr.AddPrompt(mcp.NewPrompt("read_file", mcp.WithArgument("path", mcp.RequiredArgument())),
		func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			content, err := os.ReadFile(request.Params.Arguments["path"])
			if err != nil {
				return nil, err
			}
			return &mcp.GetPromptResult{
				Messages: []mcp.PromptMessage{{
					Role:    mcp.RoleUser,
					Content: mcp.TextContent{Type: "text", Text: string(content)},
				}},
			}, nil
		})

	cli, err := client.NewInProcessClient(svr)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, cli.Close()) })
	require.NoError(t, cli.Start(ctx))
	init := mcp.InitializeRequest{}
	init.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	init.Params.ClientInfo = mcp.Implementation{Name: "prompt-client", Version: "1.0.0"}
	_, err = cli.Initialize(ctx, init)
	require.NoError(t, err)

	tpl, err := NewPromptTemplate(ctx, &Config{Cli: cli, Name: "read_file"})
	require.NoError(t, err)
	messages, err := tpl.Format(ctx, map[string]any{"path": "README.md"})
	require.NoError(t, err)
	require.Len(t, messages, 1)
	require.Equal(t, schema.User, messages[0].Role)
	require.Equal(t, string(source), messages[0].Content)
}

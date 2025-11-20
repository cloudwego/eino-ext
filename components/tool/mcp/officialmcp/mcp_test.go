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

package officialmcp

import (
	"context"
	"fmt"
	"testing"

	"github.com/cloudwego/eino/components/tool"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
)

var testImpl = &mcp.Implementation{Name: "test", Version: "v1.0.0"}

type SayHiParams struct {
	Name string `json:"name"`
}

func SayHi(ctx context.Context, req *mcp.CallToolRequest, args SayHiParams) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "Hi " + args.Name},
		},
	}, nil, nil
}

func SayHello(ctx context.Context, req *mcp.CallToolRequest, args SayHiParams) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "Hello " + args.Name},
		},
	}, nil, nil
}

func TestTool(t *testing.T) {
	ctx := context.Background()

	server := mcp.NewServer(testImpl, nil)
	client := mcp.NewClient(testImpl, nil)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()

	// add tools to server
	mcp.AddTool(server, &mcp.Tool{Name: "greet", Description: "say hi"}, SayHi)
	mcp.AddTool(server, &mcp.Tool{Name: "hello", Description: "say hello"}, SayHello)

	// get tools from client, only greet tool
	tools, err := GetTools(ctx, &Config{Cli: clientSession, ToolNameList: []string{"greet"}})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(tools))
	info, err := tools[0].Info(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "greet", info.Name)

	result, err := tools[0].(tool.InvokableTool).InvokableRun(ctx, "{\"name\": \"eino\"}")
	assert.NoError(t, err)
	fmt.Println(result)
	assert.Equal(t, "{\"content\":[{\"type\":\"text\",\"text\":\"Hi eino\"}]}", result)
}

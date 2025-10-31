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
	"fmt"
	"testing"

	"github.com/cloudwego/eino/components/tool"
	"github.com/mark3labs/mcp-go/mcp"
	omcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
)

func TestTool(t *testing.T) {
	cli := &mockMCPClient{}

	ctx := context.Background()

	tools, err := GetTools(ctx, &Config{Cli: cli, ToolNameList: []string{"name"}})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(tools))
	info, err := tools[0].Info(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "name", info.Name)

	options := []tool.Option{
		WithCustomHeaders(map[string]string{"key": "value"}),
	}

	result, err := tools[0].(tool.InvokableTool).InvokableRun(ctx, "{\"input\": \"123\"}", options...)
	assert.NoError(t, err)
	assert.Equal(t, "{\"content\":[{\"type\":\"text\",\"text\":\"hello\"}]}", result)
}

type mockMCPClient struct{}

func (m *mockMCPClient) ListResourcesByPage(ctx context.Context, request mcp.ListResourcesRequest) (*mcp.ListResourcesResult, error) {
	//TODO implement me
	panic("implement me")
}

func (m *mockMCPClient) ListResourceTemplatesByPage(ctx context.Context, request mcp.ListResourceTemplatesRequest) (*mcp.ListResourceTemplatesResult, error) {
	//TODO implement me
	panic("implement me")
}

func (m *mockMCPClient) ListPromptsByPage(ctx context.Context, request mcp.ListPromptsRequest) (*mcp.ListPromptsResult, error) {
	//TODO implement me
	panic("implement me")
}

func (m *mockMCPClient) ListToolsByPage(ctx context.Context, request mcp.ListToolsRequest) (*mcp.ListToolsResult, error) {
	//TODO implement me
	panic("implement me")
}

func (m *mockMCPClient) Initialize(ctx context.Context, request mcp.InitializeRequest) (*mcp.InitializeResult, error) {
	panic("implement me")
}

func (m *mockMCPClient) Ping(ctx context.Context) error {
	panic("implement me")
}

func (m *mockMCPClient) ListResources(ctx context.Context, request mcp.ListResourcesRequest) (*mcp.ListResourcesResult, error) {
	panic("implement me")
}

func (m *mockMCPClient) ListResourceTemplates(ctx context.Context, request mcp.ListResourceTemplatesRequest) (*mcp.ListResourceTemplatesResult, error) {
	panic("implement me")
}

func (m *mockMCPClient) ReadResource(ctx context.Context, request mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	panic("implement me")
}

func (m *mockMCPClient) Subscribe(ctx context.Context, request mcp.SubscribeRequest) error {
	panic("implement me")
}

func (m *mockMCPClient) Unsubscribe(ctx context.Context, request mcp.UnsubscribeRequest) error {
	panic("implement me")
}

func (m *mockMCPClient) ListPrompts(ctx context.Context, request mcp.ListPromptsRequest) (*mcp.ListPromptsResult, error) {
	panic("implement me")
}

func (m *mockMCPClient) GetPrompt(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	panic("implement me")
}

func (m *mockMCPClient) ListTools(ctx context.Context, request mcp.ListToolsRequest) (*mcp.ListToolsResult, error) {
	return &mcp.ListToolsResult{
		Tools: []mcp.Tool{
			{
				Name:        "name",
				Description: "description",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"input": map[string]interface{}{"type": "string"},
					},
					Required: []string{"input"},
				},
			},
			{
				Name:        "name2",
				Description: "description",
			},
		},
	}, nil
}

func (m *mockMCPClient) CallTool(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: "hello",
			},
		},
		IsError: false,
	}, nil
}

func (m *mockMCPClient) SetLevel(ctx context.Context, request mcp.SetLevelRequest) error {
	panic("implement me")
}

func (m *mockMCPClient) Complete(ctx context.Context, request mcp.CompleteRequest) (*mcp.CompleteResult, error) {
	panic("implement me")
}

func (m *mockMCPClient) Close() error {
	panic("implement me")
}

func (m *mockMCPClient) OnNotification(handler func(notification mcp.JSONRPCNotification)) {
	panic("implement me")
}

var testImpl = &omcp.Implementation{Name: "test", Version: "v1.0.0"}

type SayHiParams struct {
	Name string `json:"name"`
}

func SayHi(ctx context.Context, req *omcp.CallToolRequest, args SayHiParams) (*omcp.CallToolResult, any, error) {
	return &omcp.CallToolResult{
		Content: []omcp.Content{
			&omcp.TextContent{Text: "Hi " + args.Name},
		},
	}, nil, nil
}

func SayHello(ctx context.Context, req *omcp.CallToolRequest, args SayHiParams) (*omcp.CallToolResult, any, error) {
	return &omcp.CallToolResult{
		Content: []omcp.Content{
			&omcp.TextContent{Text: "Hello " + args.Name},
		},
	}, nil, nil
}

func TestOfficialMCPTool(t *testing.T) {
	ctx := context.Background()

	server := omcp.NewServer(testImpl, nil)
	client := omcp.NewClient(testImpl, nil)
	serverTransport, clientTransport := omcp.NewInMemoryTransports()
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
	omcp.AddTool(server, &omcp.Tool{Name: "greet", Description: "say hi"}, SayHi)
	omcp.AddTool(server, &omcp.Tool{Name: "hello", Description: "say hello"}, SayHello)

	// get tools from client, only greet tool
	tools, err := GetTools(ctx, &Config{MCPSess: clientSession, ToolNameList: []string{"greet"}})
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

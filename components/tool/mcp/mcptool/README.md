# MCP Tool

A MCP Tool implementation for [Eino](https://github.com/cloudwego/eino) that implements the `Tool` interface. This enables seamless integration with Eino's LLM capabilities for enhanced natural language processing and generation.

## Features

- Implements `github.com/cloudwego/eino/components/tool.BaseTool`
- Easy integration with Eino's tool system
- Support for get&call mcp tools

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/tool/mcp/mcptool@latest
```

## Quick Start

Here's a quick example of how to use the mcp tool:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	mcpp "github.com/cloudwego/eino-ext/components/tool/mcp/mcptool"
)

type AddParams struct {
	X int `json:"x"`
	Y int `json:"y"`
}

func Add(ctx context.Context, req *mcp.CallToolRequest, args AddParams) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("%d", args.X+args.Y)},
		},
	}, nil, nil
}

func main() {
	httpServer := startMCPServer()
	time.Sleep(1 * time.Second)
	ctx := context.Background()

	sess := getMCPClientSession(ctx, httpServer.URL)
	defer sess.Close()

	mcpTools, err := mcpp.GetTools(ctx, &mcpp.Config{Sess: sess})
	if err != nil {
		log.Fatal(err)
	}

	for i, mcpTool := range mcpTools {
		fmt.Println(i, ":")
		info, err := mcpTool.Info(ctx)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("Name:", info.Name)
		fmt.Println("Desc:", info.Desc)
		result, err := mcpTool.(tool.InvokableTool).InvokableRun(ctx, `{"x":1, "y":1}`)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("Result:", result)
		fmt.Println()
	}
}

func getMCPClientSession(ctx context.Context, addr string) *mcp.ClientSession {
	transport := &mcp.SSEClientTransport{Endpoint: addr}
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v1.0.0"}, nil)
	sess, err := client.Connect(ctx, transport, nil)
	if err != nil {
		log.Fatal(err)
	}
	return sess
}

func startMCPServer() *httptest.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "adder", Version: "v0.0.1"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "add", Description: "add two numbers"}, Add)

	handler := mcp.NewSSEHandler(func(*http.Request) *mcp.Server { return server }, nil)

	httpServer := httptest.NewServer(handler)
	return httpServer
}
```

## Configuration

The tool can be configured using the `mcp.Config` struct:

```go
type Config struct {
	// Sess is the MCP (Model Control Protocol) session, ref: https://github.com/modelcontextprotocol/go-sdk?tab=readme-ov-file#tools
	// Notice: should Initialize with server before use
	Sess *mcp.ClientSession
	// ToolNameList specifies which tools to fetch from MCP server
	// If empty, all available tools will be fetched
	ToolNameList []string
}
```

## For More Details

- [Eino Documentation](https://www.cloudwego.io/zh/docs/eino/)
- [MCP Documentation](https://modelcontextprotocol.io/introduction)
- [Official MCP SDK Documentation](https://github.com/modelcontextprotocol/go-sdk?tab=readme-ov-file#tools)

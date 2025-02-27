# MCP Tool

A MCP Tool implementation for [Eino](https://github.com/cloudwego/eino) that implements the `Tool` interface. This enables seamless integration with Eino's LLM capabilities for enhanced natural language processing and generation.

## Features

- Implements `github.com/cloudwego/eino/components/tool.BaseTool`
- Easy integration with Eino's tool system
- Support for get&call mcp tools

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/tool/mcp@latest
```

## Quick Start

Here's a quick example of how to use the mcp tool:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	mcpp "github.com/cloudwego/eino-ext/components/tool/mcp"
)

func main() {
	startMCPServer()
	time.Sleep(1 * time.Second)
	ctx := context.Background()

	mcpTools := getMCPTool(ctx)

	cm := getChatModel(ctx)

	runner, err := react.NewAgent(ctx, &react.AgentConfig{
		Model:       cm,
		ToolsConfig: compose.ToolsNodeConfig{Tools: mcpTools},
	})
	if err != nil {
		log.Fatal(err)
	}

	result, err := runner.Generate(ctx, []*schema.Message{schema.UserMessage("What is the sum of 1 and 2?")})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(result.Content)
}

func getChatModel(ctx context.Context) model.ChatModel {
	cm, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey: os.Getenv("OPENAI_API_KEY"),
		Model:  "gpt-4o",
	})
	if err != nil {
		log.Fatal(err)
	}
	return cm
}

func getMCPTool(ctx context.Context) []tool.BaseTool {
	cli, err := client.NewSSEMCPClient("http://localhost:12345/sse")
	if err != nil {
		log.Fatal(err)
	}
	err = cli.Start(ctx)
	if err != nil {
		log.Fatal(err)
	}

	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "example-client",
		Version: "1.0.0",
	}

	_, err = cli.Initialize(ctx, initRequest)
	if err != nil {
		log.Fatal(err)
	}

	tools, err := mcpp.GetTools(ctx, &mcpp.Config{Cli: cli})
	if err != nil {
		log.Fatal(err)
	}

	return tools
}

func startMCPServer() {
	svr := server.NewMCPServer("demo", mcp.LATEST_PROTOCOL_VERSION)
	svr.AddTool(mcp.NewTool("calculate",
		mcp.WithDescription("Perform basic arithmetic operations"),
		mcp.WithString("operation",
			mcp.Required(),
			mcp.Description("The operation to perform (add, subtract, multiply, divide)"),
			mcp.Enum("add", "subtract", "multiply", "divide"),
		),
		mcp.WithNumber("x",
			mcp.Required(),
			mcp.Description("First number"),
		),
		mcp.WithNumber("y",
			mcp.Required(),
			mcp.Description("Second number"),
		),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		op := request.Params.Arguments["operation"].(string)
		x := request.Params.Arguments["x"].(float64)
		y := request.Params.Arguments["y"].(float64)

		var result float64
		switch op {
		case "add":
			result = x + y
		case "subtract":
			result = x - y
		case "multiply":
			result = x * y
		case "divide":
			if y == 0 {
				return mcp.NewToolResultError("Cannot divide by zero"), nil
			}
			result = x / y
		}
		log.Printf("Calculated result: %.2f", result)
		return mcp.NewToolResultText(fmt.Sprintf("%.2f", result)), nil
	})
	go func() {
		defer func() {
			e := recover()
			if e != nil {
				fmt.Println(e)
			}
		}()

		err := server.NewSSEServer(svr, "http://localhost:12345").Start("localhost:12345")

		if err != nil {
			log.Fatal(err)
		}
	}()
}
```

## Configuration

The tool can be configured using the `mcp.Config` struct:

```go
type Config struct {
    // Cli is the MCP (Model Control Protocol) client, ref: https://github.com/mark3labs/mcp-go?tab=readme-ov-file#tools
    // Notice: should Initialize with server before use
    Cli client.MCPClient
	// ToolNameList specifies which tools to fetch from MCP server
	// If empty, all available tools will be fetched
	ToolNameList []string
}
```
 
## For More Details

- [Eino Documentation](https://github.com/cloudwego/eino)
- [MCP Documentation](https://modelcontextprotocol.io/introduction)
- [MCP SDK Documentation](https://github.com/mark3labs/mcp-go?tab=readme-ov-file#tools)
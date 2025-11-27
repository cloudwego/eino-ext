# MCP Prompt

A MCP Prompt implementation for [Eino](https://github.com/cloudwego/eino) that implements the `ChatTemplate` interface. This enables seamless integration with Eino's LLM capabilities for enhanced natural language processing and generation.

## Features

- Implements `github.com/cloudwego/eino/components/prompt.ChatTemplate`
- Easy integration with Eino's chat template system
- Support for get mcp prompt

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/prompt/mcp@latest
```

## Quick Start

Here's a quick example of how to use the mcp prompt:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/cloudwego/eino/components/prompt"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	mcpp "github.com/cloudwego/eino-ext/components/prompt/mcp"
)

func main() {
	startMCPServer()
	time.Sleep(1 * time.Second)
	ctx := context.Background()

	mcpPrompt := getMCPPrompt(ctx)

	result, err := mcpPrompt.Format(ctx, map[string]interface{}{"persona": "Describe the content of the image"})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(result)
}

func getMCPPrompt(ctx context.Context) prompt.ChatTemplate {
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

	p, err := mcpp.NewPromptTemplate(ctx, &mcpp.Config{Cli: cli, Name: "test"})
	if err != nil {
		log.Fatal(err)
	}

	return p
}

func startMCPServer() {
	svr := server.NewMCPServer("demo", mcp.LATEST_PROTOCOL_VERSION, server.WithPromptCapabilities(false))
	svr.AddPrompt(mcp.Prompt{
		Name: "test",
	}, func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return &mcp.GetPromptResult{
			Messages: []mcp.PromptMessage{
				mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(request.Params.Arguments["persona"])),
				mcp.NewPromptMessage(mcp.RoleUser, mcp.NewImageContent("https://upload.wikimedia.org/wikipedia/commons/3/3a/Cat03.jpg", "image/jpeg")),
				mcp.NewPromptMessage(mcp.RoleUser, mcp.NewEmbeddedResource(mcp.TextResourceContents{
					URI:      "https://upload.wikimedia.org/wikipedia/commons/3/3a/Cat03.jpg",
					MIMEType: "image/jpeg",
					Text:     "resource",
				})),
			},
		}, nil
	})
	go func() {
		defer func() {
			e := recover()
			if e != nil {
				fmt.Println(e)
			}
		}()

		err := server.NewSSEServer(svr, server.WithBaseURL("http://localhost:12345")).Start("localhost:12345")

		if err != nil {
			log.Fatal(err)
		}
	}()
}


```

## Configuration

The prompt can be configured using the `mcp.Config` struct:

```go
type Config struct {
    // Cli is the MCP (Model Control Protocol) client, ref: https://github.com/mark3labs/mcp-go
    // Notice: should Initialize with server before use
    // Required
    Cli client.MCPClient
    // Name specifies the prompt name to use from MCP service
    // Required
    Name string
}
```

## For More Details

- [Eino Documentation](https://www.cloudwego.io/zh/docs/eino/)
- [MCP Documentation](https://modelcontextprotocol.io/introduction)
- [MCP SDK Documentation](https://github.com/mark3labs/mcp-go?tab=readme-ov-file#prompts)

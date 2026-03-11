# MCP Prompt

一个为 [Eino](https://github.com/cloudwego/eino) 实现的 MCP Prompt 组件，实现了 `ChatTemplate` 接口。这使得能够无缝集成 Eino 的 LLM 功能，以增强自然语言处理和生成能力。

## 特性

- 实现 `github.com/cloudwego/eino/components/prompt.ChatTemplate`
- 易于与 Eino 的聊天模板系统集成
- 支持获取 MCP 提示词

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/prompt/mcp@latest
```

## 快速开始

以下是如何使用 MCP prompt 的快速示例：

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

## 配置

prompt 可以使用 `mcp.Config` 结构体进行配置：

```go
type Config struct {
    // Cli 是 MCP（Model Control Protocol）客户端，参考：https://github.com/mark3labs/mcp-go
    // 注意：使用前应先与服务器进行初始化
    // 必需
    Cli client.MCPClient
    // Name 指定从 MCP 服务使用的提示词名称
    // 必需
    Name string
}
```

## 示例

查看 [examples](./examples/) 目录获取完整的使用示例。

## 更多详情

- [Eino 文档](https://www.cloudwego.io/zh/docs/eino/)
- [MCP 文档](https://modelcontextprotocol.io/introduction)
- [MCP SDK 文档](https://github.com/mark3labs/mcp-go?tab=readme-ov-file#prompts)

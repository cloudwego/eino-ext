# MCP Tool

一个为 [Eino](https://github.com/cloudwego/eino) 实现的 MCP Tool 组件，实现了 `Tool` 接口。这使得能够无缝集成 Eino 的 LLM 功能，以增强自然语言处理和生成能力。

## 特性

- 实现 `github.com/cloudwego/eino/components/tool.BaseTool`
- 易于与 Eino 的工具系统集成
- 支持获取和调用 MCP 工具

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/tool/mcp@latest
```

## 快速开始

以下是如何使用 MCP 工具的快速示例：

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/cloudwego/eino/components/tool"
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

	for i, mcpTool := range mcpTools {
		fmt.Println(i, ":")
		info, err := mcpTool.Info(ctx)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("Name:", info.Name)
		fmt.Println("Desc:", info.Desc)
		fmt.Println()
	}
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
				return mcp.NewToolResultText("Cannot divide by zero"), nil
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

		err := server.NewSSEServer(svr, server.WithBaseURL("http://localhost:12345")).Start("localhost:12345")

		if err != nil {
			log.Fatal(err)
		}
	}()
}
```

## 配置

工具可以使用 `mcp.Config` 结构体进行配置：

```go
type Config struct {
    // Cli 是 MCP（Model Control Protocol）客户端，参考：https://github.com/mark3labs/mcp-go?tab=readme-ov-file#tools
    // 注意：使用前应先与服务器进行初始化
    Cli client.MCPClient
	// ToolNameList 指定从 MCP 服务器获取哪些工具
	// 如果为空，将获取所有可用工具
	ToolNameList []string
}
```

## 示例

查看 [examples](./examples/) 目录获取完整的使用示例。

## 更多详情

- [Eino 文档](https://www.cloudwego.io/zh/docs/eino/)
- [MCP 文档](https://modelcontextprotocol.io/introduction)
- [MCP SDK 文档](https://github.com/mark3labs/mcp-go?tab=readme-ov-file#tools)

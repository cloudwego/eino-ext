# Official MCP Tool

一个为 [Eino](https://github.com/cloudwego/eino) 实现的 MCP Tool 组件，实现了 `Tool` 接口。这使得能够无缝集成 Eino 的 LLM 功能，以增强自然语言处理和生成能力。基于 Official MCP SDK 实现。

## 特性

- 实现 `github.com/cloudwego/eino/components/tool.BaseTool`
- 易于与 Eino 的工具系统集成
- 支持获取和调用 MCP 工具

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/tool/mcp/officialmcp@latest
```

## 快速开始

以下是如何使用官方 MCP 工具的快速示例：

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

	omcp "github.com/cloudwego/eino-ext/components/tool/mcp/officialmcp"
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

	cli := getMCPClient(ctx, httpServer.URL)
	defer cli.Close()

	mcpTools, err := omcp.GetTools(ctx, &omcp.Config{Cli: cli})
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

func getMCPClient(ctx context.Context, addr string) *mcp.ClientSession {
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

## 配置

工具可以使用 `mcp.Config` 结构体进行配置：

```go
type Config struct {
	// Cli 是 MCP（Model Control Protocol）客户端，参考：https://github.com/modelcontextprotocol/go-sdk?tab=readme-ov-file#tools
	// 注意：使用前应先与服务器进行初始化
	Cli *mcp.ClientSession

	ServerName string

	// ToolNameList 指定从 MCP 服务器获取哪些工具
	// 如果为空，将获取所有可用工具
	ToolNameList []string

	Cursor        string
	ListToolsMode ListToolsMode
	MaxToolPages  int

	ToolNameMapper ToolNameMapper
	MetadataMode   MetadataMode

	DescriptionPolicy *DescriptionPolicy
	ResultPolicy      *ResultPolicy

	// ToolCallResultHandler 是工具调用完成后处理调用结果的函数
	// 可用于自定义处理工具调用结果
	// 如果为 nil，将不执行额外处理
	ToolCallResultHandler func(ctx context.Context, name string, result *mcp.CallToolResult) (*mcp.CallToolResult, error)

	// ToolCallResultHandlerV2 接收稳定的 MCP 原始/暴露工具标识
	// 如果两个 handler 都已设置，将先执行旧版 handler，再执行 V2
	ToolCallResultHandlerV2 func(ctx context.Context, info ToolCallInfo, result *mcp.CallToolResult) (*mcp.CallToolResult, error)
}
```

## 示例

查看 [examples](./examples/) 目录获取完整的使用示例。

## 更多详情

- [Eino 文档](https://www.cloudwego.io/zh/docs/eino/)
- [MCP 文档](https://modelcontextprotocol.io/introduction)
- [Official MCP SDK 文档](https://github.com/modelcontextprotocol/go-sdk?tab=readme-ov-file#tools)

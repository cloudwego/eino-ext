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
	// ToolNameList 指定从 MCP 服务器获取哪些工具
	// 如果为空，将获取所有可用工具
	ToolNameList []string
}
```

## 示例

查看 [examples](./examples/) 目录获取完整的使用示例。

## 动态凭据注入（streamable-http / SSE）

如果授权在带外完成——例如由凭据保险库（Vault）对每个 server 预授权、在请求时返回
已就绪的鉴权 header——你并不需要交互式 OAuth，只需要在每次建连请求时注入 header 的
能力。相比配置阶段就固定的静态 header map，按请求调用的 provider 可以让凭据更晚解析
（减少 secret 在内存中的停留）并跨请求刷新（例如会过期的 token）。

officialmcp 消费的是调用方构建的 `*mcp.ClientSession`，因此这一步在调用方交给 transport
的 `http.Client` 上接线。这个 seam 就是一个小的 `http.RoundTripper`，在每次请求前向
provider 取 header：

```go
type CredentialProvider interface {
    Credentials(ctx context.Context) (headers map[string]string, err error)
}

type credentialRoundTripper struct {
    base     http.RoundTripper
    headers  map[string]string  // 静态底
    provider CredentialProvider // 动态，覆盖同名静态项；可为 nil
}

func (rt credentialRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
    clone := req.Clone(req.Context()) // 不要改动传入的请求
    for k, v := range rt.headers {
        clone.Header.Set(k, v)
    }
    if rt.provider != nil {
        dyn, err := rt.provider.Credentials(req.Context()) // 继承请求的 deadline
        if err != nil {
            return nil, err
        }
        for k, v := range dyn {
            clone.Header.Set(k, v)
        }
    }
    base := rt.base
    if base == nil {
        base = http.DefaultTransport
    }
    return base.RoundTrip(clone)
}

client := &http.Client{Transport: credentialRoundTripper{base: http.DefaultTransport, provider: p}}
transport := &mcp.StreamableClientTransport{Endpoint: serverURL, HTTPClient: client}
session, _ := mcp.NewClient(impl, nil).Connect(ctx, transport, nil)
tools, _ := omcp.GetTools(ctx, &omcp.Config{Cli: session})
```

provider 为 nil 时即退化为纯静态 header 行为，可与现有配置无缝组合。它不涉及 401 处理、
不依赖 `oauthex`、不需要 build tag，并且适用于任何暴露 `Transport.HTTPClient` 的 go-sdk
版本。SSE server 用 `SSEClientTransport{Endpoint, HTTPClient}`。完整程序见
[examples/credentialprovider](./examples/credentialprovider/)。

## 远程（streamable-http）服务器的 OAuth 授权

> 这是更重的*交互式*路径（授权码流程、401 challenge）。如果你的凭据已在带外解析完成，
> 优先使用上面的 CredentialProvider seam——两者是正交的。

许多托管的 MCP 服务器（Notion、Linear、GitHub 等）要求 OAuth 授权，而非静态
token。officialmcp 消费的是调用方构建的 `*mcp.ClientSession`，因此 OAuth 在调用方
构建的 transport 上接线——而不在 officialmcp 内部。

go-sdk 自身驱动整个流程：正常发送请求，仅当服务端返回 `401`/`403` + `WWW-Authenticate`
时才触发 OAuth（challenge-driven，符合 MCP Authorization 规范）。不需要授权的服务器
永远不会触发它，因此无需提前判断，现有服务器的行为也完全不变。

在 `mcp.StreamableClientTransport` 上设置 `OAuthHandler` 即可接入：

```go
oauthHandler, err := auth.NewAuthorizationCodeHandler(&auth.AuthorizationCodeHandlerConfig{
    RedirectURL: redirectURL,
    DynamicClientRegistrationConfig: &auth.DynamicClientRegistrationConfig{
        Metadata: &oauthex.ClientRegistrationMetadata{
            ClientName:   "my-app",
            RedirectURIs: []string{redirectURL},
        },
    },
    // 唯一需要你提供的交互：把 args.URL 呈现给用户，并返回重定向得到的 code/state。
    // 使用带超时的 ctx——授权流程期间 transport 会持锁。
    AuthorizationCodeFetcher: fetchAuthorizationCode,
})
// ...
transport := &mcp.StreamableClientTransport{Endpoint: serverURL, OAuthHandler: oauthHandler}
session, _ := mcp.NewClient(impl, nil).Connect(ctx, transport, nil)
tools, _ := omcp.GetTools(ctx, &omcp.Config{Cli: session})
```

`auth.AuthorizationCodeHandler` 是 SDK 内置的 handler，实现了 PRM 发现 → Auth Server
Metadata → 可选的动态客户端注册（DCR）→ 授权码 + PKCE，你只需提供交互步骤。如需更底层
的控制，`oauthex` 包直接暴露了发现/注册原语。完整程序见
[examples/oauth](./examples/oauth/)。

> 需要 go-sdk v1.6.x+：该版本将 `OAuthHandler` 设为 transport 的一等字段，且不再需要
> build tag。`SSEClientTransport` 没有 OAuth 字段——OAuth 请使用 streamable-http。

## 更多详情

- [Eino 文档](https://www.cloudwego.io/zh/docs/eino/)
- [MCP 文档](https://modelcontextprotocol.io/introduction)
- [Official MCP SDK 文档](https://github.com/modelcontextprotocol/go-sdk?tab=readme-ov-file#tools)

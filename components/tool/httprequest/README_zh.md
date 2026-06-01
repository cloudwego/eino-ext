# HTTP Request Tools

一组为 [Eino](https://github.com/cloudwego/eino) 实现的 HTTP 请求工具，实现了 `InvokableTool` 接口。这些工具允许您轻松执行 GET、POST、PUT 和 DELETE 请求，并将它们与 Eino 的聊天模型交互系统和 `ToolsNode` 集成以增强功能。

## 特性

- 实现 `github.com/cloudwego/eino/components/tool.InvokableTool`
- 支持 GET、POST、PUT 和 DELETE 请求
- 可配置的请求头和 HttpClient
- 简单地与 Eino 的工具系统集成

## 安装

使用 `go get` 安装包（根据您的项目结构调整模块路径）：

```bash
go get github.com/cloudwego/eino-ext/components/tool/httprequest
```

## 快速开始

以下是两个示例，演示如何单独使用 GET 和 POST 工具。

### GET 请求示例

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/bytedance/sonic"
	req "github.com/cloudwego/eino-ext/components/tool/httprequest/get"
)

func main() {
	// 配置 GET 工具
	config := &req.Config{
		// Headers 是可选的
		Headers: map[string]string{
			"User-Agent": "MyCustomAgent",
		},
		// HttpClient 是可选的
		HttpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: &http.Transport{},
		},
	}

	ctx := context.Background()

	// 创建 GET 工具
	tool, err := req.NewTool(ctx, config)
	if err != nil {
		log.Fatalf("Failed to create tool: %v", err)
	}

	// 准备 GET 请求负载
	request := &req.GetRequest{
		URL: "https://jsonplaceholder.typicode.com/posts",
	}

	jsonReq, err := sonic.Marshal(request)
	if err != nil {
		log.Fatalf("Error marshaling JSON: %v", err)
	}

	// 使用 InvokableTool 接口执行 GET 请求
	resp, err := tool.InvokableRun(ctx, string(jsonReq))
	if err != nil {
		log.Fatalf("GET request failed: %v", err)
	}

	fmt.Println(resp)
}
```

### POST 请求示例

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/bytedance/sonic"
	post "github.com/cloudwego/eino-ext/components/tool/httprequest/post"
)

func main() {
	config := &post.Config{}

	ctx := context.Background()

	tool, err := post.NewTool(ctx, config)
	if err != nil {
		log.Fatalf("Failed to create tool: %v", err)
	}

	request := &post.PostRequest{
		URL:  "https://jsonplaceholder.typicode.com/posts",
		Body: `{"title": "my title","body": "my body","userId": 1}`,
	}

	jsonReq, err := sonic.Marshal(request)

	if err != nil {
		log.Fatalf("Error marshaling JSON: %v", err)
	}

	resp, err := tool.InvokableRun(ctx, string(jsonReq))
	if err != nil {
		log.Fatalf("Post failed: %v", err)
	}

	fmt.Println(resp)
}

```

## 配置

GET、POST、PUT 和 DELETE 工具共享在各自的 `Config` 结构体中定义的类似配置参数。例如：

```go
// Config 表示 HTTP 请求工具的通用配置
type Config struct {
	// 受 LangChain 项目的 "Requests" 工具启发，特别是 RequestsGetTool
	// 详情请访问：https://python.langchain.com/docs/integrations/tools/requests/
	// 可选。默认值："request_get"
	ToolName string `json:"tool_name"`
	// 可选。默认值："A portal to the internet. Use this tool when you need to fetch specific content from a website.
	// Input should be a URL (e.g., https://www.google.com). The output will be the text response from the GET request."
	ToolDesc string `json:"tool_desc"`

	// Headers 是一个将 HTTP 头名称映射到其对应值的映射
	// 这些头将包含在工具发出的每个请求中
	Headers map[string]string `json:"headers"`

	// HttpClient 是用于执行请求的 HTTP 客户端
	// 如果未提供，将初始化并使用具有 30 秒超时和标准传输的默认客户端
	HttpClient *http.Client
}
```

对于 GET 工具，请求架构定义为：

```go
type GetRequest struct {
	URL string `json:"url" jsonschema_description:"The URL to perform the GET request"`
}
```

对于 POST 工具，请求架构为：

```go
type PostRequest struct {
	URL  string `json:"url" jsonschema_description:"The URL to perform the POST request"`
	Body string `json:"body" jsonschema_description:"The request body to be sent in the POST request"`
}
```

## Agent 示例

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino-ext/components/model/openai"
	req "github.com/cloudwego/eino-ext/components/tool/httprequest/get"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// float32Ptr 是一个辅助函数，用于返回 float32 值的指针
func float32Ptr(f float32) *float32 {
	return &f
}

func main() {
	// 从环境变量加载 OpenAI API 密钥
	openAIAPIKey := os.Getenv("OPENAI_API_KEY")
	if openAIAPIKey == "" {
		log.Fatal("OPENAI_API_KEY not set")
	}

	ctx := context.Background()

	// 设置 GET 工具配置
	config := &req.Config{
		Headers: map[string]string{
			"User-Agent": "MyCustomAgent",
		},
	}

	// 实例化 GET 工具
	getTool, err := req.NewTool(ctx, config)
	if err != nil {
		log.Fatalf("Failed to create GET tool: %v", err)
	}

	// 检索工具信息以将其绑定到 ChatModel
	toolInfo, err := getTool.Info(ctx)
	if err != nil {
		log.Fatalf("Failed to get tool info: %v", err)
	}

	// 使用 OpenAI 创建 ChatModel
	chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		Model:       "gpt-4o", // 或其他支持的模型
		APIKey:      openAIAPIKey,
		Temperature: float32Ptr(0.7),
	})
	if err != nil {
		log.Fatalf("Failed to create ChatModel: %v", err)
	}

	// 将工具绑定到 ChatModel
	err = chatModel.BindTools([]*schema.ToolInfo{toolInfo})
	if err != nil {
		log.Fatalf("Failed to bind tool to ChatModel: %v", err)
	}

	// 使用 GET 工具创建 Tools 节点
	toolsNode, err := compose.NewToolNode(ctx, &compose.ToolsNodeConfig{
		Tools: []tool.BaseTool{getTool},
	})
	if err != nil {
		log.Fatalf("Failed to create ToolNode: %v", err)
	}

	// 使用 ChatModel 和 Tools 节点构建链
	chain := compose.NewChain[[]*schema.Message, []*schema.Message]()
	chain.
		AppendChatModel(chatModel, compose.WithNodeName("chat_model")).
		AppendToolsNode(toolsNode, compose.WithNodeName("tools"))

	// 编译链以获取 agent
	agent, err := chain.Compile(ctx)
	if err != nil {
		log.Fatalf("Failed to compile chain: %v", err)
	}

	// 定义 OpenAPI (YAML) 格式的 API 规范（api_spec）
	apiSpec := `
openapi: "3.0.0"
info:
  title: JSONPlaceholder API
  version: "1.0.0"
servers:
  - url: https://jsonplaceholder.typicode.com
paths:
  /posts:
    get:
      summary: Get posts
      parameters:
        - name: _limit
          in: query
          required: false
          schema:
            type: integer
            example: 2
          description: Limit the number of results
      responses:
        "200":
          description: Successful response
          content:
            application/json:
              schema:
                type: array
                items:
                  type: object
                  properties:
                    userId:
                      type: integer
                    id:
                      type: integer
                    title:
                      type: string
                    body:
                      type: string
  /comments:
    get:
      summary: Get comments
      parameters:
        - name: _limit
          in: query
          required: false
          schema:
            type: integer
            example: 2
          description: Limit the number of results
      responses:
        "200":
          description: Successful response
          content:
            application/json:
              schema:
                type: array
                items:
                  type: object
                  properties:
                    postId:
                      type: integer
                    id:
                      type: integer
                    name:
                      type: string
                    email:
                      type: string
                    body:
                      type: string
`

	// 创建包含 API 文档的系统消息
	systemMessage := fmt.Sprintf(`You have access to an API to help answer user queries.
Here is documentation on the API:
%s`, apiSpec)

	// 定义初始消息（系统和用户）
	messages := []*schema.Message{
		{
			Role:    schema.System,
			Content: systemMessage,
		},
		{
			Role:    schema.User,
			Content: "Fetch the top two posts. What are their titles?",
		},
	}

	// 使用消息调用 agent
	resp, err := agent.Invoke(ctx, messages)
	if err != nil {
		log.Fatalf("Failed to invoke agent: %v", err)
	}

	// 输出响应消息
	for idx, msg := range resp {
		fmt.Printf("Message %d: %s: %s\n", idx, msg.Role, msg.Content)
	}
}
```

## 更多详情
- [Eino 文档](https://www.cloudwego.io/zh/docs/eino/)
- [InvokableTool 接口参考](https://pkg.go.dev/github.com/cloudwego/eino/components/tool)
- [langchain_community 参考](https://python.langchain.com/docs/integrations/tools/requests/)
## 示例

查看以下示例了解更多用法：

- [GET 请求](./examples/get/)
- [POST 请求](./examples/post/)


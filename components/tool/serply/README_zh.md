# Serply 搜索工具

[English](README.md) | 简体中文

这是一个为 [Eino](https://github.com/cloudwego/eino) 实现的 [Serply](https://serply.io) 搜索工具。该工具实现了 `InvokableTool` 接口，可以与 Eino 的 ChatModel 交互系统和 `ToolsNode` 无缝集成。Serply 是一个 SERP API，以 JSON 形式返回 Google 网页、Google News 和 Google Scholar 的搜索结果，可用于网页搜索、新闻监控或文献检索。

## 特性

- 实现了 `github.com/cloudwego/eino/components/tool.InvokableTool` 接口
- 易于与 Eino 工具系统集成
- 通过 `Config.SearchType` 在网页、新闻和学术搜索之间切换
- 精简的搜索结果，包含标题、链接和摘要（新闻结果附带来源和发布时间，学术结果附带作者和引用数）
- 支持自定义请求头、超时时间和 HTTP 客户端

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/tool/serply
```

## 前置条件

使用该工具前需要一个 Serply API Key：

1. 在 [serply.io](https://serply.io) 注册并创建 API Key
2. 将其设置为环境变量 `SERPLY_API_KEY`（或直接传入 `Config.APIKey`）

API 参考文档见 [serply.io/docs](https://serply.io/docs)。

## 快速开始

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/tool/serply"
)

func main() {
	ctx := context.Background()

	// 创建搜索工具
	searchTool, err := serply.NewTool(ctx, &serply.Config{
		APIKey:     os.Getenv("SERPLY_API_KEY"),
		SearchType: serply.SearchTypeWeb, // 也可以是 serply.SearchTypeNews 或 serply.SearchTypeScholar
		Num:        5,
	})
	if err != nil {
		log.Fatal(err)
	}

	args, _ := json.Marshal(serply.SearchRequest{Query: "CloudWeGo Eino"})

	resp, err := searchTool.InvokableRun(ctx, string(args))
	if err != nil {
		log.Fatal(err)
	}

	var searchResp serply.SearchResponse
	_ = json.Unmarshal([]byte(resp), &searchResp)

	for i, result := range searchResp.Results {
		fmt.Printf("%d. %s\n   %s\n\n", i+1, result.Title, result.Link)
	}
}
```

## 配置

工具可以通过 `Config` 结构体进行配置：

```go
type Config struct {
    // Eino 工具设置
    ToolName string `json:"tool_name"` // 可选，默认值: "serply_search"
    ToolDesc string `json:"tool_desc"` // 可选，默认值取决于 SearchType

    // APIKey 是访问 Serply API 所必需的，通过 X-Api-Key 请求头发送。
    APIKey string `json:"api_key"`

    // BaseURL 是 Serply API 的基础 URL。
    // 可选，默认值: "https://api.serply.io"
    BaseURL string `json:"base_url"`

    // SearchType 选择搜索类型: SearchTypeWeb、SearchTypeNews 或 SearchTypeScholar。
    // 可选，默认值: SearchTypeWeb
    SearchType SearchType `json:"search_type"`

    // Num 是默认返回的结果数量，范围 1 到 100，可在请求中覆盖。
    // 可选，默认值: 10
    Num int `json:"num"`

    // Headers 指定随每个请求发送的自定义 HTTP 请求头，
    // 例如 "User-Agent"，或用 "X-Proxy-Location" 指定结果的地区。
    // 可选，默认值: map[string]string{}
    Headers map[string]string `json:"headers"`

    // Timeout 指定单次请求的最长时间。设置了 HttpClient 时忽略。
    // 可选，默认值: 30 * time.Second
    Timeout time.Duration `json:"timeout"`

    // HttpClient 指定自定义的 HTTP 客户端。
    // 可选，默认值: &http.Client{Timeout: Timeout}
    HttpClient *http.Client `json:"http_client"`
}
```

## 搜索

### 请求 Schema

```go
type SearchRequest struct {
    Query string `json:"query" jsonschema:"required" jsonschema_description:"The query to search for"`
    Num   int    `json:"num,omitempty" jsonschema_description:"Number of results to return, between 1 and 100. Defaults to the configured value"`
}
```

### 响应 Schema

```go
type SearchResponse struct {
    Query   string          `json:"query"`
    Results []*SearchResult `json:"results"`
}

type SearchResult struct {
    Title       string   `json:"title"`
    Link        string   `json:"link"`
    Description string   `json:"description,omitempty"` // 网页和学术搜索
    Source      string   `json:"source,omitempty"`      // 新闻: 来源
    Published   string   `json:"published,omitempty"`   // 新闻: 发布时间
    Authors     []string `json:"authors,omitempty"`     // 学术: 作者
    CitedBy     int      `json:"cited_by,omitempty"`    // 学术: 引用数
}
```

## 示例

完整的使用示例请参见 [examples](./examples/) 目录。

运行示例：
```bash
export SERPLY_API_KEY="your-api-key"
cd examples && go run main.go
```

## 更多详情

- [Serply API 文档](https://serply.io/docs)
- [Eino 文档](https://www.cloudwego.io/zh/docs/eino/)

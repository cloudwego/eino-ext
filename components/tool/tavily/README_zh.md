# Tavily Search Tool

[English](README.md) | 简体中文

这是一个为 [Eino](https://github.com/cloudwego/eino) 实现的 Tavily 搜索工具。该工具实现了 `InvokableTool` 接口，可以与 Eino 的 ChatModel 交互系统和 `ToolsNode` 无缝集成。

## 特性

- 实现了 `github.com/cloudwego/eino/components/tool.InvokableTool` 接口
- 易于与 Eino 工具系统集成
- 可配置的搜索参数
- 支持 AI 生成答案
- 域名过滤（包含/排除）
- 可配置的搜索深度和主题

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/tool/tavily
```

## 快速开始

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino-ext/components/tool/tavily"
)

func main() {
	// 从环境变量获取 Tavily API 密钥
	tavilyAPIKey := os.Getenv("TAVILY_API_KEY")

	// 创建上下文
	ctx := context.Background()

	// 创建 Tavily Search 工具
	tavilyTool, err := tavily.NewTool(ctx, &tavily.Config{
		APIKey:        tavilyAPIKey,
		IncludeAnswer: true,
	})
	if err != nil {
		log.Fatalf("创建工具失败: %v", err)
	}

	// 创建搜索请求
	request := &tavily.SearchRequest{
		Query: "什么是 Eino 框架？",
	}

	jsonReq, _ := sonic.Marshal(request)

	// 执行搜索
	resp, err := tavilyTool.InvokableRun(ctx, string(jsonReq))
	if err != nil {
		log.Fatalf("搜索失败: %v", err)
	}

	fmt.Println(resp)
}
```

## 配置

工具可以通过 `Config` 结构体进行配置：

```go
type Config struct {
    // Eino 工具设置
    ToolName string `json:"tool_name"` // 可选，默认值: "tavily_search"
    ToolDesc string `json:"tool_desc"` // 可选，默认值: "search web for information by tavily"

    // Tavily API 设置
    APIKey            string      `json:"api_key"`             // 必填
    BaseURL           string      `json:"base_url"`            // 可选，默认值: "https://api.tavily.com"
    SearchDepth       SearchDepth `json:"search_depth"`        // 可选，默认值: SearchDepthBasic
    Topic             Topic       `json:"topic"`               // 可选，默认值: TopicGeneral
    MaxResults        int         `json:"max_results"`         // 可选，默认值: 10
    IncludeAnswer     bool        `json:"include_answer"`      // 可选，默认值: false
    IncludeRawContent bool        `json:"include_raw_content"` // 可选，默认值: false
    IncludeDomains    []string    `json:"include_domains"`     // 可选，默认值: nil
    ExcludeDomains    []string    `json:"exclude_domains"`     // 可选，默认值: nil

    // HTTP 客户端设置
    Timeout    time.Duration `json:"timeout"`     // 可选，默认值: 30 * time.Second
    ProxyURL   string        `json:"proxy_url"`   // 可选，默认值: ""
    MaxRetries int           `json:"max_retries"` // 可选，默认值: 3
}
```

### 搜索深度选项

- `SearchDepthBasic`: 基础搜索，速度更快但结果较简略
- `SearchDepthAdvanced`: 高级搜索，速度较慢但结果更全面

### 主题选项

- `TopicGeneral`: 通用网页搜索
- `TopicNews`: 新闻专项搜索

## 搜索

### 请求 Schema

```go
type SearchRequest struct {
    Query string `json:"query" jsonschema_description:"The query to search the web for"`
}
```

### 响应 Schema

```go
type SearchResponse struct {
    Query   string          `json:"query,omitempty"`   // 原始搜索查询
    Answer  string          `json:"answer,omitempty"`  // AI 生成的答案（如果启用）
    Results []*SearchResult `json:"results"`           // 搜索结果列表
}

type SearchResult struct {
    Title      string  `json:"title"`                 // 搜索结果标题
    URL        string  `json:"url"`                   // 搜索结果 URL
    Content    string  `json:"content"`               // 内容摘要
    Score      float64 `json:"score,omitempty"`       // 相关度评分
    RawContent string  `json:"raw_content,omitempty"` // 原始内容（如果启用）
}
```

## 高级用法

### 使用域名过滤

```go
tool, err := tavily.NewTool(ctx, &tavily.Config{
    APIKey:         apiKey,
    IncludeDomains: []string{"github.com", "stackoverflow.com"},
    ExcludeDomains: []string{"pinterest.com"},
})
```

### 启用 AI 答案

```go
tool, err := tavily.NewTool(ctx, &tavily.Config{
    APIKey:        apiKey,
    IncludeAnswer: true,
    SearchDepth:   tavily.SearchDepthAdvanced,
})
```

### 新闻搜索

```go
tool, err := tavily.NewTool(ctx, &tavily.Config{
    APIKey: apiKey,
    Topic:  tavily.TopicNews,
})
```

## 更多详情

- [Tavily API 文档](https://docs.tavily.com/)
- [Eino 框架](https://github.com/cloudwego/eino)

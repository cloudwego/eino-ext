# Bing Search Tool

[English](README.md) | 简体中文

这是一个为 [Eino](https://github.com/cloudwego/eino) 实现的 Bing 搜索工具。该工具实现了 `InvokableTool` 接口，可以与 Eino 的 ChatModel 交互系统和 `ToolsNode` 无缝集成。

## 特性

- 实现了 `github.com/cloudwego/eino/components/tool.InvokableTool` 接口
- 易于与 Eino 工具系统集成
- 可配置的搜索参数

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/tool/bingsearch
```

## 快速开始

```go
package main

import (
	"context"
	"log"
	"os"
	"time"
	
	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino-ext/components/tool/bingsearch"
)

func main() {
	// 设置 Bing Search API 密钥
	bingSearchAPIKey := os.Getenv("BING_SEARCH_API_KEY")
	
	// 创建上下文
	ctx := context.Background()
	
	// 创建 Bing Search 工具
	bingSearchTool, err := bingsearch.NewTool(ctx, &bingsearch.Config{
		APIKey: bingSearchAPIKey,
		Cache:  5 * time.Minute,
	})
	
	if err != nil {
		log.Fatalf("Failed to create tool: %v", err)
	}
    // ... 配置并使用 ToolsNode
```

## 配置

工具可以通过 `Config` 结构体进行配置：

```go
// Config represents the Bing search tool configuration.
type Config struct {
// Config represents the Bing search tool configuration.
type Config struct {
    ToolName string `json:"tool_name"` // optional, default is "bing_search"
    ToolDesc string `json:"tool_desc"` // optional, default is "search web for information by bing"

    APIKey     string     `json:"api_key"`     // required
    Region     Region     `json:"region"`      // optional, default: ""
    MaxResults int        `json:"max_results"` // optional, default: 10
    SafeSearch SafeSearch `json:"safe_search"` // optional, default: SafeSearchModerate
    TimeRange  TimeRange  `json:"time_range"`  // optional, default: nil

    Headers    map[string]string `json:"headers"`     // optional, default: map[string]string{}
    Timeout    time.Duration     `json:"timeout"`     // optional, default: 30 * time.Second
    ProxyURL   string            `json:"proxy_url"`   // optional, default: ""
    Cache      time.Duration     `json:"cache"`       // optional, default: 0 (disabled)
    MaxRetries int               `json:"max_retries"` // optional, default: 3
}
```

## Search

### 请求 Schema
```go
type SearchRequest struct {
    Query  string `json:"query" jsonschema_description:"The query to search the web for"`
    Offset int    `json:"page" jsonschema_description:"The index of the first result to return, default is 0"`
}
```

### 响应 Schema
```go
type SearchResponse struct {
    Results []*searchResult `json:"results" jsonschema_description:"The results of the search"`
}

type searchResult struct {
    Title       string `json:"title" jsonschema_description:"The title of the search result"`
    URL         string `json:"url" jsonschema_description:"The link of the search result"`
    Description string `json:"description" jsonschema_description:"The description of the search result"`
}
```

## 更多详情

- [Eino 文档](https://github.com/cloudwego/eino) 
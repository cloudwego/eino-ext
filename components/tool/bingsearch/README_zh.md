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
	"fmt"
	"log"
	"os"
	
	"github.com/bytedance/sonic"
	
	"github.com/cloudwego/eino-ext/components/tool/bingsearch"
	"github.com/cloudwego/eino-ext/components/tool/bingsearch/bingcore"
)

func main() {
    // 设置 Bing 搜索的 API key
    bingSearchAPIKey := os.Getenv("BING_SEARCH_API_KEY")
    
    // 创建上下文
    ctx := context.Background()
    
    // 创建 Bing 搜索工具
    bingSearchTool, err := bingsearch.NewTool(ctx, &bingsearch.Config{
        APIKey: bingSearchAPIKey,
        BingConfig: &bingcore.Config{
            Cache: true,
        },
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
    ToolName string `json:"tool_name"` // default: bing_search
    ToolDesc string `json:"tool_desc"` // default: "search web for information by bing"

    APIKey     string              `json:"api_key"`     // default: ""
    Region     bingcore.Region     `json:"region"`      // default: "wt-wt"
    MaxResults int                 `json:"max_results"` // default: 10
    SafeSearch bingcore.SafeSearch `json:"safe_search"` // default: bingcore.SafeSearchModerate
    TimeRange  bingcore.TimeRange  `json:"time_range"`  // default: nil

    // Bing search configuration
    BingConfig struct{
        // Headers specifies custom HTTP headers to be sent with each request.
        Headers map[string]string `json:"headers"`

        // Timeout specifies the maximum duration for a single request.
        Timeout time.Duration `json:"timeout"`  // default: 10s

        // ProxyURL specifies the proxy server URL for all requests.
        ProxyURL string `json:"proxy_url"`

        // Cache enables in-memory caching of search results.
        Cache bool `json:"cache"`

        // MaxRetries specifies the maximum number of retry attempts for failed requests.
        MaxRetries int `json:"max_retries"` // default: 3
    }
}
```

## Search

### 请求 Schema
```go
type SearchRequest struct {
    Query string `json:"query" jsonschema_description:"The query to search the web for"`
    Page  int    `json:"page" jsonschema_description:"The page number to search for, default: 1"`
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

- [DuckDuckGo 搜索库文档](ddgsearch/README_zh.md)
- [Eino 文档](https://github.com/cloudwego/eino) 
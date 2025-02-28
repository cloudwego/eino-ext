# DuckDuckGo 搜索工具

[English](README.md) | 简体中文

这是一个为 [Eino](https://github.com/cloudwego/eino) 实现的 DuckDuckGo 搜索工具。该工具实现了 `InvokableTool` 接口，可以与 Eino 的 ChatModel 交互系统和 `ToolsNode` 无缝集成。

## 特性

- 实现了 `github.com/cloudwego/eino/components/tool.InvokableTool` 接口
- 易于与 Eino 工具系统集成
- 可配置的搜索参数

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/tool/duckduckgo
```

## 快速开始

```go
package main

import (
    "context"
    "log"

    "github.com/cloudwego/eino-ext/components/tool/duckduckgo"
    "github.com/cloudwego/eino-ext/components/tool/duckduckgo/ddgsearch"
    "github.com/cloudwego/eino/components/tool"
)

func main() {
    // 创建工具配置
    cfg := &duckduckgo.Config{ // 下面所有这些参数都是默认值，仅作用法展示
        ToolName:   "duckduckgo_search",
        ToolDesc:   "search web for information by duckduckgo",
        Region:     ddgsearch.RegionWT,
        Retries:    3,
        Timeout:    10,
        MaxResults: 10,
    }

    // 创建搜索工具
    searchTool, err := duckduckgo.NewTool(context.Background(), cfg)
    if err != nil {
        log.Fatalf("NewTool of duckduckgo failed, err=%v", err)
    }

    // 与 Eino 的 ToolsNode 一起使用
    tools := []tool.BaseTool{searchTool}
    // ... 配置并使用 ToolsNode
}
```

## 配置

工具可以通过 `Config` 结构体进行配置：

```go
type Config struct {
    ToolName    string           // 用于 LLM 交互的工具名称（默认："duckduckgo_search"）
    ToolDesc    string           // 工具描述（默认："search web for information by duckduckgo"）
    Region      ddgsearch.Region // 搜索地区（默认："wt-wt"）
    Retries     int             // 重试次数（默认：3）
    Timeout     int             // 最大超时时间（秒）（默认：10）
    MaxResults  int             // 每次搜索的最大结果数（默认：10）
    Proxy       string          // 可选的代理 URL
}
```

## Search

### 请求 Schema
```go
type SearchRequest struct {
    Query string `json:"query" jsonschema_description:"要搜索的查询内容"`
    Page  int    `json:"page" jsonschema_description:"要搜索的页码，默认：1"`
}
```

### 响应 Schema
```go
type SearchResponse struct {
    Results []SearchResult `json:"results" jsonschema_description:"搜索结果列��"`
}

type SearchResult struct {
    Title       string `json:"title" jsonschema_description:"搜索结果的标题"`
    Description string `json:"description" jsonschema_description:"搜索结果的描述"`
    Link        string `json:"link" jsonschema_description:"搜索结果的链接"`
}
```

## 更多详情

- [DuckDuckGo 搜索库文档](ddgsearch/README_zh.md)
- [Eino 文档](https://github.com/cloudwego/eino) 
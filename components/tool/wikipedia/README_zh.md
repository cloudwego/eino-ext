# 维基百科搜索工具

[English](README.md) | 简体中文

这是一个为 [Eino](https://github.com/cloudwego/eino) 实现的维基百科搜索工具。该工具实现了 `InvokableTool` 接口，可以与 Eino 的 ChatModel 交互系统和 `ToolsNode` 无缝集成。

## 特性

- 实现了 `github.com/cloudwego/eino/components/tool.InvokableTool` 接口
- 易于与 Eino 工具系统集成
- 可配置的搜索参数

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/tool/wikipedia
```

## 快速开始

```go
package main

import (
	"context"
	"github.com/cloudwego/eino-ext/components/tool/wikipedia"
	"github.com/cloudwego/eino/components/tool"
	"log"
	"time"
)

func main() {
	ctx := context.Background()

	// 创建工具配置
	// 下面所有这些参数都是默认值，仅作用法展示
	config := &wikipedia.Config{
		UserAgent:   "eino (https://github.com/cloudwego/eino)",
		DocMaxChars: 2000,
		Timeout:     15 * time.Second,
		TopK:        3,
		MaxRedirect: 3,
		Language:    "en",
	}

	// 创建搜索工具
	t, err := wikipedia.NewTool(ctx, config)
	if err != nil {
		log.Fatal("Failed to create tool:", err)
	}

	// 与 Eino 的 ToolsNode 一起使用
	tools := []tool.BaseTool{t}
	// ... 配置并使用 ToolsNode
}
```

## 配置

工具可以通过 `Config` 结构体进行配置：

```go
type Config struct {
    // BaseURL 是 Wikipedia API 的基础 URL。
    // 格式: https://<language>.wikipedia.org/w/api.php
    // URL 语言依赖于您为 Language 字段设置的语言。
    // 可选。默认值: "https://en.wikipedia.org/w/api.php"。
    BaseURL string
    
    // UserAgent 是 HTTP 客户端使用的用户代理。
    // 可选，但强烈建议覆盖默认值并使用您项目的相关信息。
    // 建议遵循 Wikipedia 的机器人规范：
    // https://foundation.wikimedia.org/wiki/Policy:Wikimedia_Foundation_User-Agent_Policy
    // 可选。默认值: "eino (https://github.com/cloudwego/eino)"
    UserAgent string `json:"user_agent"`
    
    // DocMaxChars 是返回页面内容的最大字符数。
    // 如果内容超过此值，将会被截断。
    // 可选。默认值: 15s。
    DocMaxChars int `json:"doc_max_chars"`
    
    // Timeout 是 HTTP 客户端返回响应的最大等待时间。
    // 可选。默认值: 15s。
    Timeout time.Duration `json:"timeout"`
    
    // TopK 是返回的搜索结果数量。
    // 可选。默认值: 3。
    TopK int `json:"top_k"`
    
    // MaxRedirect 是最大允许的重定向次数。
    // 可选。默认值: 3。
    MaxRedirect int `json:"max_redirect"`
    
    // Language 是用于 Wikipedia 搜索的语言。
    // 可选。默认值: "en"。
    Language string `json:"language"`
	
    ToolName string `json:"tool_name"` // 可选。默认值: "wikipedia_search"。
    ToolDesc string `json:"tool_desc"` // 可选。默认值: "this tool provides quick and efficient access to information from the Wikipedia"。
}

```

## Search

### 请求 Schema

```go
type SearchRequest struct {
    // Query 是搜索的查询字符串。
    Query string `json:"query" jsonschema_description:"The query to search the web for"`
}
```

### 响应 Schema

```go
type SearchResponse struct {
    // Results 是搜索结果的列表。
    Results []*Result `json:"results" jsonschema_description:"The results of the search"`
}
type SearchResult struct {
    // Title 是搜索结果的标题。 
    Title   string `json:"title" jsonschema_description:"The title of the search result"`
    // URL 是搜索结果的 URL。 
    URL     string `json:"url" jsonschema_description:"The url of the search result"`
    // Extract 是搜索结果的摘要。
    Extract string `json:"extract" jsonschema_description:"The extract of the search result"`
    // Snippet 是搜索结果的片段。
    Snippet string `json:"snippet" jsonschema_description:"The snippet of the search result"`
}
```

## 更多详情

- [Eino 文档](https://github.com/cloudwego/eino)

# DuckDuckGo 文本搜索工具

一个为 [Eino](https://github.com/cloudwego/eino) 实现的 DuckDuckGo 文本搜索工具，实现了 `InvokableTool` 接口。这使得能够无缝集成 Eino 的 ChatModel 交互系统和 `ToolsNode`，以增强搜索功能。

这**不推荐用于生产环境**。DuckDuckGO 工具不使用标准的 OpenAPI。服务接口可能随时更改，并且无法保证可靠性。

## 特性

- 实现 `github.com/cloudwego/eino/components/tool.InvokableTool`
- 易于与 Eino 的工具系统集成
- 可配置的搜索参数

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/tool/duckduckgo/v2
```

## 快速开始

```go
package main

import (
	"context"
	"log"

	"github.com/cloudwego/eino-ext/components/tool/duckduckgo/v2"
	"github.com/cloudwego/eino/components/tool"
)

func main() {
	// 创建工具配置
	cfg := &duckduckgo.Config{ // 所有这些参数都是默认值，仅用于演示
		Region:     duckduckgo.RegionWT,
		Timeout:    10,
		MaxResults: 10,
	}

	// 创建搜索工具
	searchTool, err := duckduckgo.NewTextSearchTool(context.Background(), cfg)
	if err != nil {
		log.Fatalf("NewTextSearchTool of duckduckgo failed, err=%v", err)
	}

	// 与 Eino 的 ToolsNode 一起使用
	tools := []tool.BaseTool{searchTool}
	// ... 配置并使用 ToolsNode
}
```

## 配置

工具可以使用 `Config` 结构体进行配置：

```go
type Config struct {
    // ToolName 是工具的名称
    // 默认值：duckduckgo_search
    ToolName string `json:"tool_name"`
    // ToolDesc 是工具的描述
    // 默认值：search web for information by duckduckgo
    ToolDesc string `json:"tool_desc"`
    
    // Timeout 指定单个请求的最大持续时间
    // 默认值：30 秒
    Timeout time.Duration
    
    // HTTPClient 指定用于发送 HTTP 请求的客户端
    // 如果设置了 HTTPClient，则不会使用 Timeout
    // 可选。默认值：&http.Client{Timeout: Timeout}
    HTTPClient *http.Client `json:"http_client"`
    
    // MaxResults 限制返回的结果数量
    // 默认值：10
    MaxResults int `json:"max_results"`
    
    // Region 是结果的地理区域
    // 默认值：RegionWT，表示所有区域
    // 参考：https://duckduckgo.com/duckduckgo-help-pages/settings/params
    Region Region `json:"region"`
}
```

## 搜索

### 请求结构

```go
type TextSearchRequest struct {
	// Query 是用户的搜索查询
    Query string `json:"query"`
    // TimeRange 是搜索时间范围
    // 默认值：TimeRangeAny
    TimeRange TimeRange `json:"time_range"`
}
```

### 响应结构

```go
type TextSearchResponse struct {
    // Message 是给模型的简短状态消息
    Message string `json:"message"`
    // Results 包含搜索结果列表
    Results []*TextSearchResult `json:"results,omitempty"`
}

type TextSearchResult struct {
    // Title 是搜索结果的标题
    Title string `json:"title"`
    // URL 是结果的网址
    URL string `json:"url"`
    // Summary 是结果内容的摘要
    Summary string `json:"summary"`
}
```

## 示例

查看 [examples](./examples/) 目录获取完整的使用示例。

## 更多详情

- [Eino 文档](https://www.cloudwego.io/zh/docs/eino/)

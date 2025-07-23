# SearXNG 搜索工具

[English](README.md) | 简体中文

这是一个为 [Eino](https://github.com/cloudwego/eino) 实现的 SearXNG 搜索工具。该工具实现了 `InvokableTool` 和 `StreamableTool` 接口，可以与 Eino 的 ChatModel 交互系统和 `ToolsNode` 无缝集成，使用 SearXNG 实例提供增强的搜索功能。

## 特性

- 实现了 `github.com/cloudwego/eino/components/tool.InvokableTool` 接口
- 实现了 `github.com/cloudwego/eino/components/tool.StreamableTool` 接口
- 易于与 Eino 工具系统集成
- 可配置的搜索参数
- 支持自定义 SearXNG 实例
- 内置重试机制和错误处理
- 代理支持
- 自定义请求头支持

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/tool/searxng
```

## 快速开始

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/cloudwego/eino-ext/components/tool/searxng"
    "github.com/cloudwego/eino/components/tool"
)

func main() {
    // 创建搜索请求配置
    requestConfig := &searxng.SearchRequestConfig{
        TimeRange:  searxng.TimeRangeMonth,
        Language:   searxng.LanguageZhCN,
        SafeSearch: searxng.SafeSearchModerate,
        Engines:    []searxng.Engine{searxng.EngineBaidu, searxng.EngineBing},
    }

    // 创建客户端配置
    cfg := &searxng.ClientConfig{
        BaseUrl: "https://searx.example.com/search", // 你的 SearXNG 实例 URL
        Timeout: 30 * time.Second,
        Headers: map[string]string{
            "User-Agent": "MyApp/1.0",
        },
        MaxRetries: 3,
        RequestConfig: requestConfig, // 将请求配置添加到客户端配置
    }

    // 创建搜索工具
    searchTool, err := searxng.BuildSearchInvokeTool(cfg)
    if err != nil {
        log.Fatalf("BuildSearchInvokeTool failed, err=%v", err)
    }

    // 与 Eino 的 ToolsNode 一起使用
    tools := []tool.BaseTool{searchTool}
    // ... 配置并使用 ToolsNode
}
```

## 配置

工具可以通过 `ClientConfig` 结构体进行配置：

```go
type ClientConfig struct {
    BaseUrl        string                // SearXNG 实例的基础 URL（必需）
    Headers        map[string]string     // 自定义 HTTP 请求头
    Timeout        time.Duration         // 请求超时时间（默认：30 秒）
    ProxyURL       string                // 代理服务器 URL
    MaxRetries     int                   // 最大重试次数（默认：3）
    HttpClient     *http.Client          // 自定义 HTTP 客户端（可选）
     RequestConfig  *SearchRequestConfig  // 默认搜索请求配置
}
```

### 自定义 HTTP 客户端

您可以提供自己的 HTTP 客户端进行高级配置：

```go
import (
    "crypto/tls"
    "net/http"
    "time"
)

// 创建具有自定义设置的 HTTP 客户端
customClient := &http.Client{
    Timeout: 60 * time.Second,
    Transport: &http.Transport{
        TLSClientConfig: &tls.Config{
            InsecureSkipVerify: true, // 仅用于测试
        },
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 10,
    },
}

// 在配置中使用自定义客户端
cfg := &searxng.ClientConfig{
    BaseUrl:    "https://searx.example.com/search",
    HttpClient: customClient, // 使用自定义 HTTP 客户端
    RequestConfig: &searxng.SearchRequestConfig{
        Language: searxng.LanguageEn,
    },
}

searchTool, err := searxng.BuildSearchInvokeTool(cfg)
```

## 搜索

### 请求 Schema
```go
type SearchRequest struct {
    Query  string `json:"query"` // 搜索查询（必需）
    PageNo int    `json:"pageno"` // 页码（默认：1）
}

type SearchRequestConfig struct {
    TimeRange  TimeRange       `json:"time_range,omitempty"`  // 时间范围："day"、"month"、"year"
    Language   Language        `json:"language,omitempty"`    // 语言代码（默认："all"）
    SafeSearch SafeSearchLevel `json:"safesearch,omitempty"` // 安全搜索级别：0、1、2（默认：0）
    Engines    []Engine        `json:"engines,omitempty"`     // 搜索引擎列表
}
```

#### 支持的语言
- `all` - 所有语言（默认）
- `en` - 英语
- `zh` - 中文（简体）
- `zh-CN` - 中文（简体，中国）
- `zh-TW` - 中文（繁体，台湾）
- `fr` - 法语
- `de` - 德语
- `es` - 西班牙语
- `ja` - 日语
- `ko` - 韩语
- `ru` - 俄语
- `ar` - 阿拉伯语
- `pt` - 葡萄牙语
- `it` - 意大利语
- `nl` - 荷兰语
- `pl` - 波兰语
- `tr` - 土耳其语

#### 支持的搜索引擎
- `google` - Google 搜索
- `duckduckgo` - DuckDuckGo
- `baidu` - 百度（中文搜索引擎）
- `bing` - 微软必应
- `360search` - 360搜索（中文）
- `yahoo` - 雅虎搜索
- `quark` - 夸克搜索

你可以通过逗号分隔指定多个引擎，例如：`"google,duckduckgo,bing"`

### 响应 Schema
```go
type SearchResponse struct {
    Query           string          `json:"query"`             // 搜索查询
    NumberOfResults int             `json:"number_of_results"` // 结果数量
    Results         []*SearchResult `json:"results"`           // 搜索结果
}

type SearchResult struct {
    Title   string `json:"title"`   // 搜索结果的标题
    Content string `json:"content"` // 结果的内容/描述
    URL     string `json:"url"`     // 搜索结果的 URL
    Engine  string `json:"engine"`  // 搜索结果的来源引擎
}
```

## 使用示例

### 基础搜索
```go
ctx := context.Background()
request := &searxng.SearchRequest{
    Query:  "人工智能",
    PageNo: 1,
}

response, err := client.Search(ctx, request)
if err != nil {
    log.Printf("搜索失败: %v", err)
    return
}

for _, result := range response.Results {
    fmt.Printf("标题: %s\nURL: %s\n内容: %s\n引擎: %s\n\n",
        result.Title, result.URL, result.Content, result.Engine)
}
```

### 带过滤器的高级搜索
```go
// 创建请求配置
requestConfig := &searxng.SearchRequestConfig{
    TimeRange:  searxng.TimeRangeMonth,
    Language:   searxng.LanguageZhCN,
    SafeSearch: searxng.SafeSearchModerate,
    Engines:    []searxng.Engine{searxng.EngineBaidu, searxng.EngineBing},
}

// 创建带请求配置的客户端配置
cfg := &searxng.ClientConfig{
    BaseUrl:       "https://searx.example.com/search",
    Timeout:       30 * time.Second,
    MaxRetries:    3,
    RequestConfig: requestConfig, // 将请求配置添加到客户端配置
}

// 创建客户端
client, err := searxng.NewClient(cfg)
if err != nil {
    log.Fatalf("NewClient failed, err=%v", err)
}

// 创建搜索请求
request := &searxng.SearchRequest{
    Query:  "机器学习教程",
    PageNo: 1,
}

response, err := client.Search(ctx, request)
// 处理响应...
```

### 英文搜索示例
```go
// 创建英文搜索的请求配置
requestConfig := &searxng.SearchRequestConfig{
    Language: searxng.LanguageEn,
    Engines:  []searxng.Engine{searxng.EngineGoogle, searxng.EngineDuckDuckGo}, // 使用国际搜索引擎
}

// 更新客户端配置中的请求配置
cfg.RequestConfig = requestConfig

// 使用更新后的配置创建新客户端
client, err = searxng.NewClient(cfg)
if err != nil {
    log.Fatalf("NewClient failed, err=%v", err)
}

// 创建搜索请求
request := &searxng.SearchRequest{
    Query:  "artificial intelligence tutorial",
    PageNo: 1,
}

response, err := client.Search(ctx, request)
// 处理响应...
```



## 错误处理

该工具包含针对常见场景的内置错误处理：

- 网络超时和连接错误
- 速率限制（HTTP 429）
- 无效的搜索参数
- 空搜索结果
- SearXNG 实例不可用

## 更多详情

- [Eino 文档](https://github.com/cloudwego/eino)
- [SearXNG 文档](https://docs.searxng.org/)
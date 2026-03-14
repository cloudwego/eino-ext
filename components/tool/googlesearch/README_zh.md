# Google Search Tool

[English](README.md) | 简体中文

这是一个为 [Eino](https://github.com/cloudwego/eino) 实现的 Google 自定义搜索工具。该工具实现了 `InvokableTool` 接口，可以使用 Google 的自定义搜索 JSON API 与 Eino 的 ChatModel 交互系统和 `ToolsNode` 无缝集成，提供增强的搜索功能。

## 特性

- 实现了 `github.com/cloudwego/eino/components/tool.InvokableTool` 接口
- 易于与 Eino 工具系统集成
- 可配置的搜索参数（语言、结果数量、偏移量）
- 简化的搜索结果，包含标题、链接、摘要和描述
- 支持自定义基础 URL 配置

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/tool/googlesearch
```

## 前置条件

使用此工具之前，您需要：

1. **获取 Google API 密钥**：
   - 访问 [Google Cloud Console](https://console.cloud.google.com/)
   - 创建或选择一个项目
   - 启用自定义搜索 API
   - 创建凭据（API 密钥）

2. **创建自定义搜索引擎**：
   - 访问 [可编程搜索引擎](https://programmablesearchengine.google.com/)
   - 创建一个新的搜索引擎
   - 获取您的搜索引擎 ID（cx 参数）

## 快速开始

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "os"
    
    "github.com/cloudwego/eino-ext/components/tool/googlesearch"
)

func main() {
    ctx := context.Background()
    
    googleAPIKey := os.Getenv("GOOGLE_API_KEY")
    googleSearchEngineID := os.Getenv("GOOGLE_SEARCH_ENGINE_ID")
    
    if googleAPIKey == "" || googleSearchEngineID == "" {
        log.Fatal("必须设置 GOOGLE_API_KEY 和 GOOGLE_SEARCH_ENGINE_ID")
    }
    
    searchTool, err := googlesearch.NewTool(ctx, &googlesearch.Config{
        APIKey:         googleAPIKey,
        SearchEngineID: googleSearchEngineID,
        Lang:           "zh-CN",
        Num:            10,
    })
    if err != nil {
        log.Fatal(err)
    }
    
    req := googlesearch.SearchRequest{
        Query: "Go语言编程",
        Num:   5,
        Lang:  "zh-CN",
    }
    
    args, _ := json.Marshal(req)
    
    resp, err := searchTool.InvokableRun(ctx, string(args))
    if err != nil {
        log.Fatal(err)
    }
    
    var searchResp googlesearch.SearchResult
    json.Unmarshal([]byte(resp), &searchResp)
    
    for i, result := range searchResp.Items {
        fmt.Printf("%d. %s\n   %s\n\n", i+1, result.Title, result.Link)
    }
}
```

## 配置

工具可以通过 `Config` 结构体进行配置：

```go
type Config struct {
    APIKey         string // 必需：Google API 密钥
    SearchEngineID string // 必需：Google 自定义搜索引擎 ID（cx 参数）
    BaseURL        string // 可选：自定义基础 URL（默认：https://customsearch.googleapis.com）
    Num            int    // 可选：默认返回的结果数量（1-10）
    Lang           string // 可选：默认语言（ISO 639-1 代码，例如 "en"、"ja"、"zh-CN"）
    
    ToolName string // 可选：LLM 交互时的工具名称（默认："google_search"）
    ToolDesc string // 可选：工具描述（默认："custom search json api of google search engine"）
}
```

### 配置选项

- **APIKey**（必需）：启用了自定义搜索 API 的 Google API 密钥
- **SearchEngineID**（必需）：您的自定义搜索引擎 ID（cx 参数）
- **BaseURL**（可选）：自定义 API 端点。默认值：`https://customsearch.googleapis.com`
- **Num**（可选）：默认搜索结果数量（1-10）。可以在每个请求中覆盖
- **Lang**（可选）：搜索结果的默认语言（ISO 639-1 代码）
- **ToolName**（可选）：LLM 调用此工具时使用的名称。默认值：`"google_search"`
- **ToolDesc**（可选）：LLM 使用的描述。默认值：`"custom search json api of google search engine"`

## 搜索

### 请求 Schema

```go
type SearchRequest struct {
    Query  string // 必需：搜索查询字符串
    Num    int    // 可选：返回的结果数量（1-10），覆盖配置默认值
    Offset int    // 可选：返回的第一个结果的索引（用于分页）
    Lang   string // 可选：结果语言（ISO 639-1 代码），覆盖配置默认值
}
```

### 响应 Schema

```go
type SearchResult struct {
    Query string                  // 搜索查询
    Items []*SimplifiedSearchItem // 搜索结果数组
}

type SimplifiedSearchItem struct {
    Link    string // 搜索结果的 URL
    Title   string // 搜索结果的标题
    Snippet string // 页面的简短摘要
    Desc    string // 来自页面元数据的详细描述
}
```

## 示例

### 示例 1：基本搜索

```go
searchTool, _ := googlesearch.NewTool(ctx, &googlesearch.Config{
    APIKey:         apiKey,
    SearchEngineID: engineID,
})

req := googlesearch.SearchRequest{
    Query: "人工智能",
}
args, _ := json.Marshal(req)
resp, _ := searchTool.InvokableRun(ctx, string(args))
```

### 示例 2：带语言和限制的搜索

```go
searchTool, _ := googlesearch.NewTool(ctx, &googlesearch.Config{
    APIKey:         apiKey,
    SearchEngineID: engineID,
    Lang:           "zh-CN",
    Num:            5,
})

req := googlesearch.SearchRequest{
    Query: "Go并发编程",
    Num:   3,
    Lang:  "zh-CN",
}
args, _ := json.Marshal(req)
resp, _ := searchTool.InvokableRun(ctx, string(args))
```

### 示例 3：分页

```go
req := googlesearch.SearchRequest{
    Query:  "机器学习",
    Num:    10,
    Offset: 10, // 获取结果 11-20
}
args, _ := json.Marshal(req)
resp, _ := searchTool.InvokableRun(ctx, string(args))
```

### 示例 4：与 Eino ToolsNode 集成

```go
import (
    "github.com/cloudwego/eino/components/tool"
)

searchTool, _ := googlesearch.NewTool(ctx, &googlesearch.Config{
    APIKey:         apiKey,
    SearchEngineID: engineID,
})

tools := []tool.BaseTool{searchTool}
// 在您的工作流中与 Eino 的 ToolsNode 一起使用
```

### 完整示例

完整的工作示例请参见 [examples/main.go](examples/main.go)

运行示例：
```bash
export GOOGLE_API_KEY="your-api-key"
export GOOGLE_SEARCH_ENGINE_ID="your-search-engine-id"
cd examples && go run main.go
```

## 工作原理

1. **工具创建**：使用您的 Google API 凭据和配置初始化工具。

2. **请求处理**：调用时，工具接收带有查询参数的 JSON 格式 `SearchRequest`。

3. **API 调用**：工具使用指定的参数调用 Google 的自定义搜索 JSON API。

4. **响应简化**：原始 Google API 响应被简化为仅包含基本字段（标题、链接、摘要、描述）。

5. **JSON 响应**：简化的结果作为 JSON 字符串返回，便于使用。

## API 限制

请注意 Google 自定义搜索 API 的限制：
- 免费层级：每天 100 次查询
- 付费层级：每天最多 10,000 次查询
- 每次查询最多 10 个结果

## 更多详情

- [Google 自定义搜索 API 文档](https://developers.google.com/custom-search/v1/overview)
- [Eino 文档](https://www.cloudwego.io/zh/docs/eino/)
- [示例代码](examples/main.go)

# SearXNG Search Tool

English | [简体中文](README_zh.md)

A SearXNG search tool implementation for [Eino](https://github.com/cloudwego/eino) that implements the `InvokableTool` and `StreamableTool` interfaces. This enables seamless integration with Eino's ChatModel interaction system and `ToolsNode` for enhanced search capabilities using SearXNG instances.

## Features

- Implements `github.com/cloudwego/eino/components/tool.InvokableTool`
- Implements `github.com/cloudwego/eino/components/tool.StreamableTool`
- Easy integration with Eino's tool system
- Configurable search parameters
- Support for custom SearXNG instances
- Built-in retry mechanism and error handling
- Proxy support
- Custom headers support

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/tool/searxng
```

## Quick Start

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
    // Create client config
    cfg := &searxng.ClientConfig{
        BaseUrl: "https://searx.example.com/search", // Your SearXNG instance URL
        Timeout: 30 * time.Second,
        Headers: map[string]string{
            "User-Agent": "MyApp/1.0",
        },
        MaxRetries: 3,
    }

    // Create the search tool
    // Create request config (optional)
    requestConfig := &searxng.SearchRequestConfig{
        TimeRange:  searxng.TimeRangeMonth,
        Language:   searxng.LanguageEn,
        SafeSearch: searxng.SafeSearchModerate,
        Engines:    []searxng.Engine{searxng.EngineGoogle, searxng.EngineBing},
    }

    // Create the search tool with request config
    searchTool, err := searxng.BuildSearchInvokeTool(cfg, requestConfig)
    if err != nil {
        log.Fatalf("BuildSearchInvokeTool failed, err=%v", err)
    }

    // Use with Eino's ToolsNode
    tools := []tool.BaseTool{searchTool}
    // ... configure and use with ToolsNode
}
```

## Configuration

The tool can be configured using the `ClientConfig` struct:

```go
type ClientConfig struct {
    BaseUrl    string            // Base URL of the SearXNG instance (required)
    Headers    map[string]string // Custom HTTP headers
    Timeout    time.Duration     // Request timeout (default: 30 seconds)
    ProxyURL   string           // Proxy server URL
    MaxRetries int              // Maximum retry attempts (default: 3)
}
```

## Search

### Request Schema
```go
type SearchRequest struct {
    Query  string `json:"query"` // The search query (required)
    PageNo int    `json:"pageno"` // Page number (default: 1)
}

type SearchRequestConfig struct {
    TimeRange  TimeRange       `json:"time_range,omitempty"`  // Time range: "day", "month", "year"
    Language   Language        `json:"language,omitempty"`    // Language code (default: "all")
    SafeSearch SafeSearchLevel `json:"safesearch,omitempty"` // Safe search level: 0, 1, 2 (default: 0)
    Engines    []Engine        `json:"engines,omitempty"`     // List of search engines
}
```

#### Supported Languages
- `all` - All languages (default)
- `en` - English
- `zh` - Chinese (simplified)
- `zh-CN` - Chinese (simplified, China)
- `zh-TW` - Chinese (traditional, Taiwan)
- `fr` - French
- `de` - German
- `es` - Spanish
- `ja` - Japanese
- `ko` - Korean
- `ru` - Russian
- `ar` - Arabic
- `pt` - Portuguese
- `it` - Italian
- `nl` - Dutch
- `pl` - Polish
- `tr` - Turkish

#### Supported Search Engines
- `google` - Google Search
- `duckduckgo` - DuckDuckGo
- `baidu` - Baidu (Chinese search engine)
- `bing` - Microsoft Bing
- `360search` - 360 Search (Chinese)
- `yahoo` - Yahoo Search
- `quark` - Quark Search

You can specify multiple engines by separating them with commas, e.g., `"google,duckduckgo,bing"`

### Response Schema
```go
type SearchResponse struct {
    Query           string          `json:"query"`             // The search query
    NumberOfResults int             `json:"number_of_results"` // Number of results
    Results         []*SearchResult `json:"results"`           // Search results
}

type SearchResult struct {
    Title   string `json:"title"`   // Title of the search result
    Content string `json:"content"` // Content/description of the result
    URL     string `json:"url"`     // URL of the search result
    Engine  string `json:"engine"`  // The engine of the search result
}
```

## Usage Examples

### Basic Search
```go
ctx := context.Background()
request := &searxng.SearchRequest{
    Query:  "artificial intelligence",
    PageNo: 1,
}

response, err := client.Search(ctx, request)
if err != nil {
    log.Printf("Search failed: %v", err)
    return
}

for _, result := range response.Results {
    fmt.Printf("Title: %s\nURL: %s\nContent: %s\n\n", 
        result.Title, result.URL, result.Content)
}
```

### Advanced Search with Filters
```go
// Create request config
requestConfig := &searxng.SearchRequestConfig{
    TimeRange:  searxng.TimeRangeMonth,
    Language:   searxng.LanguageEn,
    SafeSearch: searxng.SafeSearchModerate,
    Engines:    []searxng.Engine{searxng.EngineGoogle, searxng.EngineDuckDuckGo},
}

// Create client with request config
client, err := searxng.NewClient(cfg, requestConfig)
if err != nil {
    log.Fatalf("NewClient failed, err=%v", err)
}

// Create search request
request := &searxng.SearchRequest{
    Query:  "machine learning tutorials",
    PageNo: 1,
}

response, err := client.Search(ctx, request)
// Handle response...
```

### Chinese Search Example
```go
language := "zh-CN"
engines := "baidu,bing" // Use Chinese-friendly search engines

request := &searxng.SearchRequest{
    Query:    "人工智能教程",
    PageNo:   1,
    Language: &language,
    Engines:  &engines,
}

response, err := client.Search(ctx, request)
// Handle response...
```



## Error Handling

The tool includes built-in error handling for common scenarios:

- Network timeouts and connection errors
- Rate limiting (HTTP 429)
- Invalid search parameters
- Empty search results
- SearXNG instance unavailability

## For More Details

- [Eino Documentation](https://github.com/cloudwego/eino)
- [SearXNG Documentation](https://docs.searxng.org/)
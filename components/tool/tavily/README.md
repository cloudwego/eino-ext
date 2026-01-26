# Tavily Search Tool

English | [简体中文](README_zh.md)

A Tavily search tool implementation for [Eino](https://github.com/cloudwego/eino) that implements the `InvokableTool` interface. This enables seamless integration with Eino's ChatModel interaction system and `ToolsNode` for enhanced search capabilities.

## Features

- Implements `github.com/cloudwego/eino/components/tool.InvokableTool`
- Easy integration with Eino's tool system
- Configurable search parameters
- Support for AI-generated answers
- Domain filtering (include/exclude)
- Configurable search depth and topic

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/tool/tavily
```

## Quick Start

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
	// Get the Tavily API key from environment
	tavilyAPIKey := os.Getenv("TAVILY_API_KEY")

	// Create a context
	ctx := context.Background()

	// Create the Tavily Search tool
	tavilyTool, err := tavily.NewTool(ctx, &tavily.Config{
		APIKey:        tavilyAPIKey,
		IncludeAnswer: true,
	})
	if err != nil {
		log.Fatalf("Failed to create tool: %v", err)
	}

	// Create a search request
	request := &tavily.SearchRequest{
		Query: "What is Eino framework?",
	}

	jsonReq, _ := sonic.Marshal(request)

	// Execute the search
	resp, err := tavilyTool.InvokableRun(ctx, string(jsonReq))
	if err != nil {
		log.Fatalf("Search failed: %v", err)
	}

	fmt.Println(resp)
}
```

## Configuration

The tool can be configured using the `Config` struct:

```go
type Config struct {
    // Eino tool settings
    ToolName string `json:"tool_name"` // optional, default is "tavily_search"
    ToolDesc string `json:"tool_desc"` // optional, default is "search web for information by tavily"

    // Tavily API settings
    APIKey            string      `json:"api_key"`             // required
    BaseURL           string      `json:"base_url"`            // optional, default: "https://api.tavily.com"
    SearchDepth       SearchDepth `json:"search_depth"`        // optional, default: SearchDepthBasic
    Topic             Topic       `json:"topic"`               // optional, default: TopicGeneral
    MaxResults        int         `json:"max_results"`         // optional, default: 10
    IncludeAnswer     bool        `json:"include_answer"`      // optional, default: false
    IncludeRawContent bool        `json:"include_raw_content"` // optional, default: false
    IncludeDomains    []string    `json:"include_domains"`     // optional, default: nil
    ExcludeDomains    []string    `json:"exclude_domains"`     // optional, default: nil

    // HTTP client settings
    Timeout    time.Duration `json:"timeout"`     // optional, default: 30 * time.Second
    ProxyURL   string        `json:"proxy_url"`   // optional, default: ""
    MaxRetries int           `json:"max_retries"` // optional, default: 3
}
```

### Search Depth Options

- `SearchDepthBasic`: Basic search, faster but less comprehensive
- `SearchDepthAdvanced`: Advanced search, slower but more comprehensive results

### Topic Options

- `TopicGeneral`: General web search
- `TopicNews`: News-specific search

## Search

### Request Schema

```go
type SearchRequest struct {
    Query string `json:"query" jsonschema_description:"The query to search the web for"`
}
```

### Response Schema

```go
type SearchResponse struct {
    Query   string          `json:"query,omitempty"`   // The original search query
    Answer  string          `json:"answer,omitempty"`  // AI-generated answer (if enabled)
    Results []*SearchResult `json:"results"`           // The list of search results
}

type SearchResult struct {
    Title      string  `json:"title"`                 // The title of the search result
    URL        string  `json:"url"`                   // The URL of the search result
    Content    string  `json:"content"`               // The content snippet
    Score      float64 `json:"score,omitempty"`       // The relevance score
    RawContent string  `json:"raw_content,omitempty"` // Raw content (if enabled)
}
```

## Advanced Usage

### With Domain Filtering

```go
tool, err := tavily.NewTool(ctx, &tavily.Config{
    APIKey:         apiKey,
    IncludeDomains: []string{"github.com", "stackoverflow.com"},
    ExcludeDomains: []string{"pinterest.com"},
})
```

### With AI Answer

```go
tool, err := tavily.NewTool(ctx, &tavily.Config{
    APIKey:        apiKey,
    IncludeAnswer: true,
    SearchDepth:   tavily.SearchDepthAdvanced,
})
```

### News Search

```go
tool, err := tavily.NewTool(ctx, &tavily.Config{
    APIKey: apiKey,
    Topic:  tavily.TopicNews,
})
```

## For More Details

- [Tavily API Documentation](https://docs.tavily.com/)
- [Eino Framework](https://github.com/cloudwego/eino)

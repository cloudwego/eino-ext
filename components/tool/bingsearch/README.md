# Bing Search Tool

English | [简体中文](README_zh.md)

A Bing search tool implementation for [Eino](https://github.com/cloudwego/eino) that implements the `InvokableTool` interface. This enables seamless integration with Eino's ChatModel interaction system and `ToolsNode` for enhanced search capabilities.

## Features

- Implements `github.com/cloudwego/eino/components/tool.InvokableTool`
- Easy integration with Eino's tool system
- Configurable search parameters

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/tool/bingsearch
```

## Quick Start

```go
package main

import (
	"context"
	"fmt"
	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino-ext/components/tool/bingsearch"
	"github.com/cloudwego/eino-ext/components/tool/bingsearch/bingcore"
	"log"
	"os"
)

func main() {
	// Set the Bing Search API key
	bingSearchAPIKey := os.Getenv("BING_SEARCH_API_KEY")
	
	// Create a context
	ctx := context.Background()
	
	// Create the Bing Search tool
	bingSearchTool, err := bingsearch.NewTool(ctx, &bingsearch.Config{
		APIKey: bingSearchAPIKey,
		BingConfig: &bingcore.Config{
			Cache: true,
		},
	})
	if err != nil {
		log.Fatalf("Failed to create tool: %v", err)
	}
	// ... configure and use with ToolsNode
```

## Configuration

The tool can be configured using the `Config` struct:

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

### Request Schema
```go
type SearchRequest struct {
    Query string `json:"query" jsonschema_description:"The query to search the web for"`
    Page  int    `json:"page" jsonschema_description:"The page number to search for, default: 1"`
}
```

### Response Schema
```go
type SearchResponse struct {
    Results []*SearchResult `json:"results" jsonschema_description:"The results of the search"`
}

type SearchResult struct {
    Title       string `json:"title" jsonschema_description:"The title of the search result"`
    URL         string `json:"url" jsonschema_description:"The link of the search result"`
    Description string `json:"description" jsonschema_description:"The description of the search result"`
}
```

## For More Details

- [DuckDuckGo Search Library Documentation](ddgsearch/README.md)
- [Eino Documentation](https://github.com/cloudwego/eino)

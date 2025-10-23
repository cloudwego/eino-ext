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
	"log"
	"os"
	"time"
	
	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino-ext/components/tool/bingsearch"
)

func main() {
	// Set the Bing Search API key
	bingSearchAPIKey := os.Getenv("BING_SEARCH_API_KEY")
	
	// Create a context
	ctx := context.Background()
	
	// Create the Bing Search tool
	bingSearchTool, err := bingsearch.NewTool(ctx, &bingsearch.Config{
		APIKey: bingSearchAPIKey,
		Cache:  5 * time.Minute,
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

### Request Schema
```go
type SearchRequest struct {
    Query  string `json:"query" jsonschema_description:"The query to search the web for"`
    Offset int    `json:"page" jsonschema_description:"The index of the first result to return, default is 0"`
}
```

### Response Schema
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

## For More Details

- [Eino Documentation](https://www.cloudwego.io/zh/docs/eino/)

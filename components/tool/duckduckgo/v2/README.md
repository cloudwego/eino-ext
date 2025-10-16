# DuckDuckGo Text Search Tool

A DuckDuckGo text search tool implementation for [Eino](https://github.com/cloudwego/eino) that implements the `InvokableTool` interface. This enables seamless integration with Eino's ChatModel interaction system and `ToolsNode` for enhanced search capabilities.

This is **not recommended for production** use. The DuckDuckGO tool doesn't use a standard OpenAPI. The service interface may change at any time, and reliability cannot be guaranteed.

## Features

- Implements `github.com/cloudwego/eino/components/tool.InvokableTool`
- Easy integration with Eino's tool system
- Configurable search parameters

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/tool/duckduckgo/v2
```

## Quick Start

```go
package main

import (
	"context"
	"log"

	"github.com/cloudwego/eino-ext/components/tool/duckduckgo/v2"
	"github.com/cloudwego/eino/components/tool"
)

func main() {
	// Create tool config
	cfg := &duckduckgo.Config{ // All of these parameters are default values, for demonstration purposes only
		Region:     duckduckgo.RegionWT,
		Timeout:    10,
		MaxResults: 10,
	}

	// Create the search tool
	searchTool, err := duckduckgo.NewTextSearchTool(context.Background(), cfg)
	if err != nil {
		log.Fatalf("NewTextSearchTool of duckduckgo failed, err=%v", err)
	}

	// Use with Eino's ToolsNode
	tools := []tool.BaseTool{searchTool}
	// ... configure and use with ToolsNode
}
```

## Configuration

The tool can be configured using the `Config` struct:

```go
type Config struct {
    // ToolName is the name of the tool
    // Default: duckduckgo_search
    ToolName string `json:"tool_name"`
    // ToolDesc is the description of the tool
    // Default: search web for information by duckduckgo
    ToolDesc string `json:"tool_desc"`
    
    // Timeout specifies the maximum duration for a single request.
    // Default: 30 seconds
    Timeout time.Duration
    
    // HTTPClient specifies the client to send HTTP requests.
    // If HTTPClient is set, Timeout will not be used.
    // Optional. Default &http.Client{Timeout: Timeout}
    HTTPClient *http.Client `json:"http_client"`
    
    // MaxResults limits the number of results returned
    // Default: 10
    MaxResults int `json:"max_results"`
    
    // Region is the geographical region for results
    // Default: RegionWT, means all regions
    // Reference: https://duckduckgo.com/duckduckgo-help-pages/settings/params
    Region Region `json:"region"`
}
```

## Search

### Request Schema
```go
type TextSearchRequest struct {
	// Query is the user's search query
    Query string `json:"query"`
    // TimeRange is the search time range
    // Default: TimeRangeAny
    TimeRange TimeRange `json:"time_range"`
}
```

### Response Schema
```go
type TextSearchResponse struct {
    // Message is a brief status message for the model
    Message string `json:"message"`
    // Results contains the list of search results
    Results []*TextSearchResult `json:"results,omitempty"`
}

type TextSearchResult struct {
    // Title is the title of the search result
    Title string `json:"title"`
    // URL is the web address of the result
    URL string `json:"url"`
    // Summary is the summary of the result content
    Summary string `json:"summary"`
}
```

## For More Details

- [Eino Documentation](https://github.com/cloudwego/eino)

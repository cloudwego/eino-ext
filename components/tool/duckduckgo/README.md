# (Deprecated) DuckDuckGo Search Tool

English | [简体中文](README_zh.md)

Please note that this implementation is **deprecated**. Please use the [DuckDuckGo V2](https://github.com/cloudwego/eino-ext/tree/main/components/tool/duckduckgo/v2).

A DuckDuckGo search tool implementation for [Eino](https://github.com/cloudwego/eino) that implements the `InvokableTool` interface. This enables seamless integration with Eino's ChatModel interaction system and `ToolsNode` for enhanced search capabilities.

## Features

- Implements `github.com/cloudwego/eino/components/tool.InvokableTool`
- Easy integration with Eino's tool system
- Configurable search parameters

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/tool/duckduckgo
```

## Quick Start

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
    // Create tool config
    cfg := &duckduckgo.Config{ // All of these parameters are default values, for demonstration purposes only
        ToolName:   "duckduckgo_search",
        ToolDesc:   "search web for information by duckduckgo",
        Region:     ddgsearch.RegionWT,
        Retries:    3,
        Timeout:    10,
        MaxResults: 10,
    }

    // Create the search tool
    searchTool, err := duckduckgo.NewTool(context.Background(), cfg)
    if err != nil {
        log.Fatalf("NewTool of duckduckgo failed, err=%v", err)
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
    ToolName    string           // Tool name for LLM interaction (default: "duckduckgo_search")
    ToolDesc    string           // Tool description (default: "search web for information by duckduckgo")
    Region      ddgsearch.Region // Search region (default: "wt-wt" of no specified region)
    Retries     int             // Number of retries (default: 3)
    Timeout     int             // Max timeout in seconds (default: 10)
    MaxResults  int             // Maximum results per search (default: 10)
    Proxy       string          // Optional proxy URL
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
    Results []SearchResult `json:"results" jsonschema_description:"The results of the search"`
}

type SearchResult struct {
    Title       string `json:"title" jsonschema_description:"The title of the search result"`
    Description string `json:"description" jsonschema_description:"The description of the search result"`
    Link        string `json:"link" jsonschema_description:"The link of the search result"`
}
```

## For More Details

- [DuckDuckGo Search Library Documentation](ddgsearch/README.md)
- [Eino Documentation](https://www.cloudwego.io/zh/docs/eino/)

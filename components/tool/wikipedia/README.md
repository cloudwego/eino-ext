# Wikipedia Search tool

English | [简体中文](README_zh.md)

A Wikipedia search tool implementation for [Eino](https://github.com/cloudwego/eino) that implements the `InvokableTool` interface. This enables seamless integration with Eino's ChatModel interaction system and `ToolsNode` for enhanced search capabilities.

## Features    

- Implements `github.com/cloudwego/eino/components/tool.InvokableTool`
- Easy integration with Eino's tool system
- Configurable search parameters

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/tool/wikipedia
```

## Quick Start

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

	// Create tool configuration
	// All of these parameters are default values, used here for demonstration purposes
	config := &wikipedia.Config{
		UserAgent:   "eino (https://github.com/cloudwego/eino)",
		DocMaxChars: 2000,
		Timeout:     15 * time.Second,
		TopK:        3,
		MaxRedirect: 3,
		Language:    "en",
	}

	// Create the search tool
	t, err := wikipedia.NewTool(ctx, config)
	if err != nil {
		log.Fatal("Failed to create tool:", err)
	}

	// Use with Eino's ToolsNode
	tools := []tool.BaseTool{t}
	// ... Configure and use ToolsNode
}
```

## Configuration

The tool can be configured using the `Config` struct:

```go
type Config struct {
    // BaseURL is the base url of the wikipedia api.
    // format: https://<language>.wikipedia.org/w/api.php
    // The URL language depends on the settings you have set for the Language field
    // Optional. Default: "https://en.wikipedia.org/w/api.php".
    BaseURL string
    
    // UserAgent is the user agent to use for the http client.
    // Optional but HIGHLY RECOMMENDED to override the default with your project's info.
    // It is recommended to follow Wikipedia's robot specification:
    // https://foundation.wikimedia.org/wiki/Policy:Wikimedia_Foundation_User-Agent_Policy
    // Optional. Default: "eino (https://github.com/cloudwego/eino)"
    UserAgent string `json:"user_agent"`
    // DocMaxChars is the maximum number of characters as extract for returning in the page content.
    // If the content is longer than this, it will be truncated.
    // Optional. Default: 15s.
    DocMaxChars int `json:"doc_max_chars"`
    // Timeout is the maximum time to wait for the http client to return a response.
    // Optional. Default: 15s.
    Timeout time.Duration `json:"timeout"`
    // TopK is the number of search results to return.
    // Optional. Default: 3.
    TopK int `json:"top_k"`
    // MaxRedirect is the maximum number of redirects to follow.
    // Optional. Default: 3.
    MaxRedirect int `json:"max_redirect"`
    // Language is the language to use for the wikipedia search.
    // Optional. Default: "en".
    Language string `json:"language"`

	ToolName string `json:"tool_name"` // Optional. Default: "wikipedia_search".
    ToolDesc string `json:"tool_desc"` // Optional. Default: "this tool provides quick and efficient access to information from the Wikipedia"
}
```

## Search

### Request Schema
```go
type SearchRequest struct {
    // Query is the query to search the web for.
    Query string `json:"query" jsonschema_description:"The query to search the web for"`
}
```

### Response Schema
```go
type SearchResponse struct {
    // Results is the list of search results.
    Results []*Result `json:"results" jsonschema_description:"The results of the search"`
}

type SearchResult struct {
    // Title is the title of the search result.
    Title   string `json:"title" jsonschema_description:"The title of the search result"`
    // URL is the URL of the search result.
    URL     string `json:"url" jsonschema_description:"The url of the search result"`
    // Extract is the summary of the search result.
    Extract string `json:"extract" jsonschema_description:"The extract of the search result"`
    // Snippet is the snippet of the search result.
    Snippet string `json:"snippet" jsonschema_description:"The snippet of the search result"`
}
```

## For More Details

- [Eino Documentation](https://www.cloudwego.io/zh/docs/eino/)

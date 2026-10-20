# Serply Search Tool

English | [简体中文](README_zh.md)

A [Serply](https://serply.io) search tool implementation for [Eino](https://github.com/cloudwego/eino) that implements the `InvokableTool` interface. Serply is a SERP API that returns Google web, Google News and Google Scholar results as JSON, so the tool can be plugged into Eino's ChatModel interaction system and `ToolsNode` for web search, news monitoring or literature lookup.

## Features

- Implements `github.com/cloudwego/eino/components/tool.InvokableTool`
- Easy integration with Eino's tool system
- Web, news and scholar search through one `Config.SearchType` switch
- Simplified results with title, link and description (plus source and publish time for news, authors and citation count for scholar)
- Custom headers, timeout and HTTP client

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/tool/serply
```

## Prerequisites

Before using this tool, you need a Serply API key:

1. Sign up at [serply.io](https://serply.io) and create an API key
2. Export it as `SERPLY_API_KEY` (or pass it to `Config.APIKey` directly)

The API reference is at [serply.io/docs](https://serply.io/docs).

## Quick Start

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/tool/serply"
)

func main() {
	ctx := context.Background()

	// Create the search tool
	searchTool, err := serply.NewTool(ctx, &serply.Config{
		APIKey:     os.Getenv("SERPLY_API_KEY"),
		SearchType: serply.SearchTypeWeb, // or serply.SearchTypeNews, serply.SearchTypeScholar
		Num:        5,
	})
	if err != nil {
		log.Fatal(err)
	}

	args, _ := json.Marshal(serply.SearchRequest{Query: "CloudWeGo Eino"})

	resp, err := searchTool.InvokableRun(ctx, string(args))
	if err != nil {
		log.Fatal(err)
	}

	var searchResp serply.SearchResponse
	_ = json.Unmarshal([]byte(resp), &searchResp)

	for i, result := range searchResp.Results {
		fmt.Printf("%d. %s\n   %s\n\n", i+1, result.Title, result.Link)
	}
}
```

## Configuration

The tool can be configured using the `Config` struct:

```go
type Config struct {
    // Eino tool settings
    ToolName string `json:"tool_name"` // optional, default is "serply_search"
    ToolDesc string `json:"tool_desc"` // optional, default depends on SearchType

    // APIKey is required to access the Serply API. It is sent as the X-Api-Key header.
    APIKey string `json:"api_key"`

    // BaseURL is the base URL of the Serply API.
    // Optional, default: "https://api.serply.io"
    BaseURL string `json:"base_url"`

    // SearchType selects the endpoint: SearchTypeWeb, SearchTypeNews or SearchTypeScholar.
    // Optional, default: SearchTypeWeb
    SearchType SearchType `json:"search_type"`

    // Num is the default number of results to return, between 1 and 100.
    // The request may override it.
    // Optional, default: 10
    Num int `json:"num"`

    // Headers specifies custom HTTP headers to be sent with each request,
    // e.g. "User-Agent" or "X-Proxy-Location" to localize results.
    // Optional, default: map[string]string{}
    Headers map[string]string `json:"headers"`

    // Timeout specifies the maximum duration for a single request.
    // Ignored when HttpClient is set.
    // Optional, default: 30 * time.Second
    Timeout time.Duration `json:"timeout"`

    // HttpClient specifies the custom HTTP client to be used.
    // Optional, default: &http.Client{Timeout: Timeout}
    HttpClient *http.Client `json:"http_client"`
}
```

## Search

### Request Schema

```go
type SearchRequest struct {
    Query string `json:"query" jsonschema:"required" jsonschema_description:"The query to search for"`
    Num   int    `json:"num,omitempty" jsonschema_description:"Number of results to return, between 1 and 100. Defaults to the configured value"`
}
```

### Response Schema

```go
type SearchResponse struct {
    Query   string          `json:"query"`
    Results []*SearchResult `json:"results"`
}

type SearchResult struct {
    Title       string   `json:"title"`
    Link        string   `json:"link"`
    Description string   `json:"description,omitempty"` // web and scholar
    Source      string   `json:"source,omitempty"`      // news: publisher
    Published   string   `json:"published,omitempty"`   // news: publish time
    Authors     []string `json:"authors,omitempty"`     // scholar
    CitedBy     int      `json:"cited_by,omitempty"`    // scholar
}
```

## Examples

See the [examples](./examples/) directory for a complete usage example.

Run the example:
```bash
export SERPLY_API_KEY="your-api-key"
cd examples && go run main.go
```

## For More Details

- [Serply API Documentation](https://serply.io/docs)
- [Eino Documentation](https://www.cloudwego.io/zh/docs/eino/)

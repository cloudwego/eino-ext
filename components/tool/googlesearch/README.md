# Google Search Tool

English | [简体中文](README_zh.md)

A Google Custom Search tool implementation for [Eino](https://github.com/cloudwego/eino) that implements the `InvokableTool` interface. This enables seamless integration with Eino's ChatModel interaction system and `ToolsNode` for enhanced search capabilities using Google's Custom Search JSON API.

## Features

- Implements `github.com/cloudwego/eino/components/tool.InvokableTool`
- Easy integration with Eino's tool system
- Configurable search parameters (language, number of results, offset)
- Simplified search results with title, link, snippet, and description
- Support for custom base URL configuration

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/tool/googlesearch
```

## Prerequisites

Before using this tool, you need to:

1. **Get a Google API Key**: 
   - Visit [Google Cloud Console](https://console.cloud.google.com/)
   - Create or select a project
   - Enable the Custom Search API
   - Create credentials (API Key)

2. **Create a Custom Search Engine**:
   - Visit [Programmable Search Engine](https://programmablesearchengine.google.com/)
   - Create a new search engine
   - Get your Search Engine ID (cx parameter)

## Quick Start

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
        log.Fatal("GOOGLE_API_KEY and GOOGLE_SEARCH_ENGINE_ID must be set")
    }
    
    searchTool, err := googlesearch.NewTool(ctx, &googlesearch.Config{
        APIKey:         googleAPIKey,
        SearchEngineID: googleSearchEngineID,
        Lang:           "en",
        Num:            10,
    })
    if err != nil {
        log.Fatal(err)
    }
    
    req := googlesearch.SearchRequest{
        Query: "Golang programming",
        Num:   5,
        Lang:  "en",
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

## Configuration

The tool can be configured using the `Config` struct:

```go
type Config struct {
    APIKey         string // required: Google API key
    SearchEngineID string // required: Google Custom Search Engine ID (cx parameter)
    BaseURL        string // optional: Custom base URL (default: https://customsearch.googleapis.com)
    Num            int    // optional: Default number of results to return (1-10)
    Lang           string // optional: Default language (ISO 639-1 code, e.g., "en", "ja", "zh-CN")
    
    ToolName string // optional: Tool name for LLM interaction (default: "google_search")
    ToolDesc string // optional: Tool description (default: "custom search json api of google search engine")
}
```

### Configuration Options

- **APIKey** (required): Your Google API key with Custom Search API enabled
- **SearchEngineID** (required): The ID of your custom search engine (cx parameter)
- **BaseURL** (optional): Custom API endpoint. Default: `https://customsearch.googleapis.com`
- **Num** (optional): Default number of search results (1-10). Can be overridden per request
- **Lang** (optional): Default language for search results (ISO 639-1 code)
- **ToolName** (optional): Name used when LLM calls this tool. Default: `"google_search"`
- **ToolDesc** (optional): Description used by LLM. Default: `"custom search json api of google search engine"`

## Search

### Request Schema

```go
type SearchRequest struct {
    Query  string // required: The search query string
    Num    int    // optional: Number of results to return (1-10), overrides config default
    Offset int    // optional: Index of the first result to return (for pagination)
    Lang   string // optional: Language for results (ISO 639-1 code), overrides config default
}
```

### Response Schema

```go
type SearchResult struct {
    Query string                  // The search query
    Items []*SimplifiedSearchItem // Array of search results
}

type SimplifiedSearchItem struct {
    Link    string // URL of the search result
    Title   string // Title of the search result
    Snippet string // Short snippet from the page
    Desc    string // Detailed description from page metadata
}
```

## Examples

### Example 1: Basic Search

```go
searchTool, _ := googlesearch.NewTool(ctx, &googlesearch.Config{
    APIKey:         apiKey,
    SearchEngineID: engineID,
})

req := googlesearch.SearchRequest{
    Query: "artificial intelligence",
}
args, _ := json.Marshal(req)
resp, _ := searchTool.InvokableRun(ctx, string(args))
```

### Example 2: Search with Language and Limit

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

### Example 3: Pagination

```go
req := googlesearch.SearchRequest{
    Query:  "machine learning",
    Num:    10,
    Offset: 10, // Get results 11-20
}
args, _ := json.Marshal(req)
resp, _ := searchTool.InvokableRun(ctx, string(args))
```

### Example 4: Integration with Eino ToolsNode

```go
import (
    "github.com/cloudwego/eino/components/tool"
)

searchTool, _ := googlesearch.NewTool(ctx, &googlesearch.Config{
    APIKey:         apiKey,
    SearchEngineID: engineID,
})

tools := []tool.BaseTool{searchTool}
// Use with Eino's ToolsNode in your workflow
```

### Complete Example

For a complete working example, see [examples/main.go](examples/main.go)

Run the example:
```bash
export GOOGLE_API_KEY="your-api-key"
export GOOGLE_SEARCH_ENGINE_ID="your-search-engine-id"
cd examples && go run main.go
```

## How It Works

1. **Tool Creation**: The tool is initialized with your Google API credentials and configuration.

2. **Request Processing**: When invoked, the tool receives a JSON-formatted `SearchRequest` with query parameters.

3. **API Call**: The tool calls Google's Custom Search JSON API with the specified parameters.

4. **Response Simplification**: The raw Google API response is simplified to include only essential fields (title, link, snippet, description).

5. **JSON Response**: The simplified results are returned as a JSON string for easy consumption.

## API Limits

Be aware of Google Custom Search API limits:
- Free tier: 100 queries per day
- Paid tier: Up to 10,000 queries per day
- Maximum 10 results per query

## For More Details

- [Google Custom Search API Documentation](https://developers.google.com/custom-search/v1/overview)
- [Eino Documentation](https://www.cloudwego.io/zh/docs/eino/)
- [Example Code](examples/main.go)

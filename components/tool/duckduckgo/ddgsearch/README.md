# DDGSearch

English | [简体中文](README_zh.md)

A native Go library for DuckDuckGo search functionality. This library provides a simple and efficient way to perform searches using DuckDuckGo's search engine.

## Why DuckDuckGo?

DuckDuckGo offers several advantages:
- **No Authentication Required**: Unlike other search engines, DuckDuckGo's API can be used without any API keys or authentication
- Privacy-focused search results
- No rate limiting for reasonable usage
- Support for multiple regions and languages
- Clean and relevant search results

## Features

- Clean and idiomatic Go implementation
- Comprehensive error handling
- Configurable search parameters
- In-memory caching with TTL
- Support for:
  - Multiple regions (us-en, uk-en, de-de, etc.)
  - Safe search levels (strict, moderate, off)
  - Time-based filtering (day, week, month, year)
  - Result pagination
  - Custom HTTP headers
  - Proxy configuration

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/tool/duckduckgo
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/cloudwego/eino-ext/components/tool/duckduckgo/ddgsearch"
)

func main() {
    // Create a new client with configuration
    cfg := &ddgsearch.Config{
        Timeout:    30 * time.Second,
        MaxRetries: 3,
        Cache:      true,
    }
    client, err := ddgsearch.New(cfg)
    if err != nil {
        log.Fatalf("New of ddgsearch failed, err=%v", err)
    }

    // Configure search parameters
    params := &ddgsearch.SearchParams{
        Query:      "what is golang",
        Region:     ddgsearch.RegionUSEN,
        SafeSearch: ddgsearch.SafeSearchModerate,
        TimeRange:  ddgsearch.TimeRangeMonth,
        MaxResults: 10,
    }

    // Perform search
    response, err := client.Search(context.Background(), params)
    if err != nil {
        log.Fatalf("Search of ddgsearch failed, err=%v", err)
    }

    // Print results
    for i, result := range response.Results {
        fmt.Printf("%d. %s\n   URL: %s\n   Description: %s\n\n", 
            i+1, result.Title, result.URL, result.Description)
    }
}
```

## Advanced Usage

### Configuration

```go
// Create client with custom configuration
cfg := &ddgsearch.Config{
    Timeout:    20 * time.Second,
    MaxRetries: 3,
    Proxy:      "http://proxy:8080",
    Cache:      true,
    Headers: map[string]string{
        "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
    },
}
client, err := ddgsearch.New(cfg)
```

### Search Parameters

```go
params := &ddgsearch.SearchParams{
    Query:      "golang tutorial",       // Search query
    Region:     ddgsearch.RegionUS,    // Region for results (us-en, uk-en, etc.)
    SafeSearch: ddgsearch.SafeSearchModerate, // Safe search level
    TimeRange:  ddgsearch.TimeRangeWeek, // Time filter
    MaxResults: 10,                      // Maximum results to return
}
```

Available regions:
- RegionUS (United States)
- RegionUK (United Kingdom)
- RegionDE (Germany)
- RegionFR (France)
- RegionJP (Japan)
- RegionCN (China)
- RegionRU (Russia)

Safe search levels:
- SafeSearchStrict
- SafeSearchModerate
- SafeSearchOff

Time range options:
- TimeRangeDay
- TimeRangeWeek
- TimeRangeMonth
- TimeRangeYear

### Proxy Support

```go
// HTTP proxy
cfg := &ddgsearch.Config{
    Proxy: "http://proxy:8080",
}
client, err := ddgsearch.New(cfg)

// SOCKS5 proxy
cfg := &ddgsearch.Config{
    Proxy: "socks5://proxy:1080",
}
client, err := ddgsearch.New(cfg)
```

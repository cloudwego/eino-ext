# Search Tools

## Google Search

```go
import "github.com/cloudwego/eino-ext/components/tool/googlesearch"

tool, err := googlesearch.NewTool(ctx, &googlesearch.Config{
    APIKey:         "your-google-api-key",
    SearchEngineID: "your-cse-id",
    NumResults:     5,
})
```

## DuckDuckGo Search (v2)

```go
import "github.com/cloudwego/eino-ext/components/tool/duckduckgo/v2"

tool, err := duckduckgo.NewTool(ctx, &duckduckgo.Config{
    MaxResults: 5,
})
```

## Bing Search

```go
import "github.com/cloudwego/eino-ext/components/tool/bingsearch"

tool, err := bingsearch.NewTool(ctx, &bingsearch.Config{
    APIKey:     "your-bing-api-key",
    MaxResults: 5,
})
```

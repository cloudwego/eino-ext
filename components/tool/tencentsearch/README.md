# Tencent Cloud Web Search Tool

The Web Search API (WSA) originates from Sogou Search and is built on publicly available internet content. It provides end-to-end search enhancement from indexing and recall to ranking. Partners can send search terms to the API and receive ranked search results and structured fields in JSON format, which can be used to enrich downstream business scenarios and improve query satisfaction.

This is a custom search tool implemented for [Eino](https://github.com/cloudwego/eino). The tool implements the `InvokableTool` interface and integrates with Eino's ChatModel interaction system and `ToolsNode` using Tencent Cloud Web Search API (WSA).

## Features

- Implements `github.com/cloudwego/eino/components/tool.InvokableTool` interface
- Easy integration with Eino tool system
- Configurable search parameters (query, result count, site, time range, industry filter, and search mode)
- Simplified search results containing titles, links, passages, and contents
- Supports custom Tencent Cloud API endpoint configuration

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/tool/tencentsearch
```

## Prerequisites

Before using this tool, you need to:

1. **Get Tencent Cloud Web Search API Keys**:
   - Reference documentation: <https://cloud.tencent.com/document/product/1806/121812>
   - Enable Tencent Cloud "Web Search (WSA)" service
   - Obtain your `SecretKey` and `SecretID`

## Configuration (Config)

```go
type Config struct {
	SecretID  string `json:"secret_id"`  // Required: Tencent Cloud SecretID
	SecretKey string `json:"secret_key"` // Required: Tencent Cloud SecretKey
	Endpoint  string `json:"endpoint"`   // default: "wsa.tencentcloudapi.com"
	Mode      int64  `json:"mode"`       // default: 0
	Cnt       uint64 `json:"cnt"`        // default: 10

	ToolName string `json:"tool_name"`
	ToolDesc string `json:"tool_desc"`
}
```

- `SecretID`: Tencent Cloud API SecretID.
- `SecretKey`: Tencent Cloud API SecretKey.
- `Endpoint`: API endpoint (default: `wsa.tencentcloudapi.com`).
- `Mode`: Result type, 0-natural, 1-VR, 2-mixed (default: `0`).
- `Cnt`: Default number of results to fetch per request. Valid values are `10/20/30/40/50`; invalid values fall back to `10`.
- `ToolName`: Override default tool name (default: `tencent_search`).
- `ToolDesc`: Override default tool description.

`SecretID` and `SecretKey` are required when initializing the tool. Missing credentials return `secret_id is required` or `secret_key is required`.

## Search Request

```go
type SearchRequest struct {
	Query    string  `json:"query" jsonschema:"required" jsonschema_description:"queried string to the search engine"`
	Mode     *int64  `json:"mode,omitempty" jsonschema_description:"0-natural, 1-VR, 2-mixed"`
	Site     *string `json:"site,omitempty" jsonschema_description:"site domain to search within"`
	FromTime *int64  `json:"from_time,omitempty" jsonschema_description:"start time timestamp in seconds"`
	ToTime   *int64  `json:"to_time,omitempty" jsonschema_description:"end time timestamp in seconds"`
	Industry *string `json:"industry,omitempty" jsonschema_description:"industry filter, one of gov/news/acad/finance"`
	Cnt      *uint64 `json:"cnt,omitempty" jsonschema_description:"number of search results to return, 10/20/30/40/50"`
}
```

- `Query`: Required search query. Empty values return `query is required`.
- `Mode`: Optional result type, `0` natural, `1` VR, `2` mixed.
- `Site`: Optional site domain to search within.
- `FromTime`: Optional start time, Unix timestamp in seconds.
- `ToTime`: Optional end time, Unix timestamp in seconds.
- `Industry`: Optional industry filter. Valid values are `gov/news/acad/finance`.
- `Cnt`: Optional number of results. Valid values are `10/20/30/40/50`; invalid values fall back to `10`.

## Examples

### Example 1: Basic Search

```go
searchTool, err := tencentsearch.NewTool(ctx, &tencentsearch.Config{
	SecretID:  os.Getenv("TENCENTCLOUD_SECRET_ID"),
	SecretKey: os.Getenv("TENCENTCLOUD_SECRET_KEY"),
})
if err != nil {
	return err
}

req := tencentsearch.SearchRequest{
	Query: "Artificial Intelligence",
}
args, _ := json.Marshal(req)
resp, err := searchTool.InvokableRun(ctx, string(args))
if err != nil {
	return err
}
```

### Example 2: Search with Limits and Industry

```go
searchTool, err := tencentsearch.NewTool(ctx, &tencentsearch.Config{
	SecretID:  os.Getenv("TENCENTCLOUD_SECRET_ID"),
	SecretKey: os.Getenv("TENCENTCLOUD_SECRET_KEY"),
	Cnt:       10, // Supports returning 10/20/30/40/50 results per request
})
if err != nil {
	return err
}

cnt := uint64(20)
industry := "news"
req := tencentsearch.SearchRequest{
	Query:    "latest finance news",
	Industry: &industry,
	Cnt:      &cnt,
}
args, _ := json.Marshal(req)
resp, err := searchTool.InvokableRun(ctx, string(args))
if err != nil {
	return err
}
```

### Example 3: Integrate with Eino ToolsNode

```go
import (
	"github.com/cloudwego/eino/components/tool"
)

searchTool, err := tencentsearch.NewTool(ctx, &tencentsearch.Config{
	SecretID:  os.Getenv("TENCENTCLOUD_SECRET_ID"),
	SecretKey: os.Getenv("TENCENTCLOUD_SECRET_KEY"),
})
if err != nil {
	return err
}

tools := []tool.BaseTool{searchTool}
// Use with Eino's ToolsNode in your workflow
```

### Full Example

See [examples/main.go](examples/main.go) for a complete working example.

Run the example:
```bash
export TENCENTCLOUD_SECRET_KEY="your-api-key"
export TENCENTCLOUD_SECRET_ID="your-secret-id"
cd examples && go run main.go
```

## How it Works

1. **Tool Creation**: Initialize the tool using your Tencent Cloud API credentials and configuration.

2. **Request Processing**: When invoked, the tool receives a JSON-formatted `SearchRequest` with query parameters.

3. **API Call**: The tool calls the Tencent Cloud Web Search API using the specified parameters.

4. **Response Simplification**: The raw Tencent Cloud API response is simplified to contain only essential fields (title, link, passage, content, etc.).

5. **JSON Response**: The simplified results are returned as a JSON string for easy consumption.

## API Limitations

Please note the limitations of the Tencent Cloud Web Search API:
- `Query` must not be empty
- Each query can return 10/20/30/40/50 results
- `Industry` is only available for the premium edition
- Invalid `Cnt` values are normalized by this tool to `10` before calling the API

## More Details

- [Tencent Cloud Web Search API Documentation](https://cloud.tencent.com/document/api/1806/121812)
- [Eino Documentation](https://www.cloudwego.io/docs/eino/)
- [Example Code](examples/main.go)

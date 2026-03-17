# Sougou Search Tool

This is a custom search tool implemented for [Eino](https://github.com/cloudwego/eino) (powered by Sougou Search). The tool implements the `InvokableTool` interface and seamlessly integrates with Eino's ChatModel interaction system and `ToolsNode` using TencentCloud's Web Search API (WSA) JSON API to provide enhanced search capabilities.

## Features

- Implements `github.com/cloudwego/eino/components/tool.InvokableTool` interface
- Easy integration with Eino tool system
- Configurable search parameters (query, result count, pagination, search mode)
- Simplified search results containing titles, links, passages, and contents
- Supports custom base URL endpoint configuration

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/tool/sougousearch
go get github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common
go get github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/wsa
```

## Prerequisites

Before using this tool, you need to:

1. **Get Tencent Cloud Web Search API Keys**:
   - Reference documentation: <https://cloud.tencent.com/document/product/1806/121811>
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
- `Cnt`: Default number of results to fetch per request (default: `10`).
- `ToolName`: Override default tool name (default: `sougou_search`).
- `ToolDesc`: Override default tool description.

## Examples

### Example 1: Basic Search

```go
searchTool, _ := sougousearch.NewTool(ctx, &sougousearch.Config{
	SecretID:  os.Getenv("TENCENTCLOUD_SECRET_ID"),
	SecretKey: os.Getenv("TENCENTCLOUD_SECRET_KEY"),
})

req := sougousearch.SearchRequest{
	Query: "Artificial Intelligence",
}
args, _ := json.Marshal(req)
resp, _ := searchTool.InvokableRun(ctx, string(args))
```

### Example 2: Search with Limits

```go
searchTool, _ := sougousearch.NewTool(ctx, &sougousearch.Config{
	SecretID:  os.Getenv("TENCENTCLOUD_SECRET_ID"),
	SecretKey: os.Getenv("TENCENTCLOUD_SECRET_KEY"),
	Cnt:       10, 					// Supports returning 10/20/30/40/50 results per request
})

cnt := uint64(3)
req := sougousearch.SearchRequest{
	Query: "Go concurrent programming",
	Cnt:   &cnt,
}
args, _ := json.Marshal(req)
resp, _ := searchTool.InvokableRun(ctx, string(args))
```

### Example 3: Integrate with Eino ToolsNode

```go
import (
	"github.com/cloudwego/eino/components/tool"
)

searchTool, _ := sougousearch.NewTool(ctx, &sougousearch.Config{
	SecretID:  os.Getenv("TENCENTCLOUD_SECRET_ID"),
	SecretKey: os.Getenv("TENCENTCLOUD_SECRET_KEY"),
})

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

1. **Tool Creation**: Initialize the tool using your TencentCloud API credentials and configuration.

2. **Request Processing**: When invoked, the tool receives a JSON-formatted `SearchRequest` with query parameters.

3. **API Call**: The tool calls the TencentCloud Web Search API using the specified parameters.

4. **Response Simplification**: The raw TencentCloud API response is simplified to contain only essential fields (title, link, passage, content, etc.).

5. **JSON Response**: The simplified results are returned as a JSON string for easy consumption.

## API Limitations

Please note the limitations of the TencentCloud Web Search API:
- Paid tiers: 30 RMB/1000 requests, 46 RMB/1000 requests
- Each query can return 10/20/30/40/50 results

## More Details

- [Tencent Cloud Web Search API Documentation](https://cloud.tencent.com/document/api/1806/121812)
- [Eino Documentation](https://www.cloudwego.io/docs/eino/)
- [Example Code](examples/main.go)
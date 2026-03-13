# Tool Reference

Tools are functions that a ChatModel can invoke. Eino provides several tool interfaces and pre-built tool implementations.

## Interfaces

```go
// github.com/cloudwego/eino/components/tool

type BaseTool interface {
    Info(ctx context.Context) (*schema.ToolInfo, error)
}

type InvokableTool interface {
    BaseTool
    InvokableRun(ctx context.Context, argumentsInJSON string, opts ...Option) (string, error)
}

type StreamableTool interface {
    BaseTool
    StreamableRun(ctx context.Context, argumentsInJSON string, opts ...Option) (*schema.StreamReader[string], error)
}

// Enhanced variants accept/return structured multimodal data
type EnhancedInvokableTool interface {
    BaseTool
    InvokableRun(ctx context.Context, toolArgument *schema.ToolArgument, opts ...Option) (*schema.ToolResult, error)
}

type EnhancedStreamableTool interface {
    BaseTool
    StreamableRun(ctx context.Context, toolArgument *schema.ToolArgument, opts ...Option) (*schema.StreamReader[*schema.ToolResult], error)
}
```

## ToolInfo Schema

```go
toolInfo := &schema.ToolInfo{
    Name: "get_weather",
    Desc: "Get current weather for a city",
    ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
        "city": {
            Type:     "string",
            Desc:     "City name",
            Required: true,
        },
        "unit": {
            Type:     "string",
            Desc:     "Temperature unit",
            Enum:     []string{"celsius", "fahrenheit"},
            Required: false,
        },
    }),
}
```

## MCP Tool Integration

The MCP (Model Context Protocol) component converts MCP server tools into Eino tools.

```go
import (
    "github.com/mark3labs/mcp-go/client"
    "github.com/mark3labs/mcp-go/mcp"
    mcpp "github.com/cloudwego/eino-ext/components/tool/mcp"
)

// 1. Create and initialize MCP client
cli, _ := client.NewSSEMCPClient("http://localhost:12345/sse")
cli.Start(ctx)

initReq := mcp.InitializeRequest{}
initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
initReq.Params.ClientInfo = mcp.Implementation{Name: "my-app", Version: "1.0.0"}
cli.Initialize(ctx, initReq)

// 2. Get tools from MCP server
tools, err := mcpp.GetTools(ctx, &mcpp.Config{
    Cli: cli,
    // ToolNameList: []string{"calculate"},  // Optional: filter specific tools
})

// 3. Each tool implements InvokableTool -- use with ToolsNode or ChatModel
for _, t := range tools {
    info, _ := t.Info(ctx)
    fmt.Println(info.Name, info.Desc)
}
```

For stdio-based MCP servers:

```go
cli, _ := client.NewStdioMCPClient("npx", nil, "-y", "@modelcontextprotocol/server-xxx")
```

## Search Tools

### Google Search

```go
import "github.com/cloudwego/eino-ext/components/tool/googlesearch"

tool, err := googlesearch.NewTool(ctx, &googlesearch.Config{
    APIKey:         "your-google-api-key",
    SearchEngineID: "your-cse-id",
    NumResults:     5,
})
// Implements InvokableTool
```

### DuckDuckGo Search (v2)

```go
import "github.com/cloudwego/eino-ext/components/tool/duckduckgo/v2"

tool, err := duckduckgo.NewTool(ctx, &duckduckgo.Config{
    MaxResults: 5,
})
```

### Bing Search

```go
import "github.com/cloudwego/eino-ext/components/tool/bingsearch"

tool, err := bingsearch.NewTool(ctx, &bingsearch.Config{
    APIKey:     "your-bing-api-key",
    MaxResults: 5,
})
```

## Utility Tools

### HTTP Request

```go
import "github.com/cloudwego/eino-ext/components/tool/httprequest"

tool, err := httprequest.NewTool(ctx, &httprequest.Config{})
// Makes HTTP requests based on model-generated parameters
```

### Command Line

```go
import "github.com/cloudwego/eino-ext/components/tool/commandline"

tool, err := commandline.NewTool(ctx, &commandline.Config{})
// Executes shell commands
```

### Browser Use

```go
import "github.com/cloudwego/eino-ext/components/tool/browseruse"

tool, err := browseruse.NewTool(ctx, &browseruse.Config{})
// Browser automation tool
```

## Custom Tool Implementation

### Using utils.NewTool (recommended)

```go
import "github.com/cloudwego/eino/components/tool/utils"

type WeatherInput struct {
    City string `json:"city" jsonschema:"required" jsonschema_description:"City name"`
    Unit string `json:"unit" jsonschema:"enum=celsius|fahrenheit" jsonschema_description:"Temperature unit"`
}

weatherTool, _ := utils.InferTool(
    "get_weather",
    "Get current weather for a city",
    func(ctx context.Context, input *WeatherInput) (string, error) {
        return fmt.Sprintf("Weather in %s: 22 %s", input.City, input.Unit), nil
    },
)
```

### Manual implementation

```go
type MyTool struct{}

func (t *MyTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
    return &schema.ToolInfo{
        Name: "my_tool",
        Desc: "Does something useful",
        ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
            "input": {Type: "string", Desc: "The input", Required: true},
        }),
    }, nil
}

func (t *MyTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
    var args struct{ Input string `json:"input"` }
    json.Unmarshal([]byte(argumentsInJSON), &args)
    return "result for: " + args.Input, nil
}
```

## Using Tools with ChatModel

```go
// 1. Collect tool infos
var toolInfos []*schema.ToolInfo
for _, t := range tools {
    info, _ := t.Info(ctx)
    toolInfos = append(toolInfos, info)
}

// 2. Bind to model
modelWithTools, _ := chatModel.WithTools(toolInfos)

// 3. Generate -- model may produce tool calls
resp, _ := modelWithTools.Generate(ctx, messages)

// 4. Handle tool calls
for _, tc := range resp.ToolCalls {
    // Find and execute the matching tool
    result, _ := matchingTool.InvokableRun(ctx, tc.Function.Arguments)
    // Append tool result as a message and call Generate again
    messages = append(messages, resp) // assistant message with tool calls
    messages = append(messages, &schema.Message{
        Role:       schema.Tool,
        Content:    result,
        ToolCallID: tc.ID,
    })
}
resp, _ = modelWithTools.Generate(ctx, messages) // final answer
```

For automated tool execution loops, use `ToolsNode` in eino or the ReAct agent pattern.

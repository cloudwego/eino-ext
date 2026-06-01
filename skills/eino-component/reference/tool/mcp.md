# MCP Tool Integration

The MCP (Model Context Protocol) component converts MCP server tools into Eino tools.

```
import mcpp "github.com/cloudwego/eino-ext/components/tool/mcp"
```

## SSE-based MCP Server

```go
import (
    "github.com/mark3labs/mcp-go/client"
    "github.com/mark3labs/mcp-go/mcp"
    mcpp "github.com/cloudwego/eino-ext/components/tool/mcp"
)

cli, _ := client.NewSSEMCPClient("http://localhost:12345/sse")
cli.Start(ctx)

initReq := mcp.InitializeRequest{}
initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
initReq.Params.ClientInfo = mcp.Implementation{Name: "my-app", Version: "1.0.0"}
cli.Initialize(ctx, initReq)

tools, err := mcpp.GetTools(ctx, &mcpp.Config{
    Cli: cli,
    // ToolNameList: []string{"calculate"},  // Optional: filter specific tools
})

// Each tool implements InvokableTool
for _, t := range tools {
    info, _ := t.Info(ctx)
    fmt.Println(info.Name, info.Desc)
}
```

## Stdio-based MCP Server

```go
cli, _ := client.NewStdioMCPClient("npx", nil, "-y", "@modelcontextprotocol/server-xxx")
```

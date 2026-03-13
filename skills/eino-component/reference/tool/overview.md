# Tool Overview

Tools are functions that a ChatModel can invoke.

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
        "city": {Type: "string", Desc: "City name", Required: true},
        "unit": {Type: "string", Desc: "Temperature unit", Enum: []string{"celsius", "fahrenheit"}},
    }),
}
```

## Custom Tool Creation

### Using utils.InferTool (recommended)

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
    result, _ := matchingTool.InvokableRun(ctx, tc.Function.Arguments)
    messages = append(messages, resp)
    messages = append(messages, &schema.Message{
        Role:       schema.Tool,
        Content:    result,
        ToolCallID: tc.ID,
    })
}
resp, _ = modelWithTools.Generate(ctx, messages) // final answer
```

For automated tool execution loops, use `ToolsNode` in eino or the ReAct agent pattern.

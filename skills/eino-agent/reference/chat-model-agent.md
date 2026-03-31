ChatModelAgent is the core ADK agent that uses the ReAct pattern: LLM reasons, generates tool calls, executes tools, feeds results back, and loops until done.

## Import

```go
import "github.com/cloudwego/eino/adk"
```

## ReAct Pattern

1. Call ChatModel (Reason)
2. LLM returns tool call requests (Action)
3. ChatModelAgent executes tools (Act)
4. Tool results fed back to ChatModel (Observation)
5. Loop until ChatModel decides no more tool calls are needed

When no tools are configured, ChatModelAgent degrades to a single ChatModel call.

## ChatModelAgentConfig

```go
type ChatModelAgentConfig struct {
    // Name of the agent. Should be unique across all agents.
    Name string

    // Description of capabilities. Helps other agents decide whether to delegate.
    Description string

    // Instruction used as system prompt. Supports f-string placeholders for session values:
    // "The current time is {Time}. The user is {User}."
    Instruction string

    // The LLM model. Must support tool calling.
    Model model.ToolCallingChatModel

    // Tool configuration
    ToolsConfig ToolsConfig

    // Custom function to transform instruction + input into model messages.
    // Optional. Defaults to prepending instruction as system message.
    GenModelInput GenModelInput

    // Exit tool. When called, agent terminates immediately.
    // Use the built-in ExitTool: adk.ExitTool{}
    Exit tool.BaseTool

    // Max ReAct iterations. Default: 20. Agent errors if exceeded.
    MaxIterations int

    // Retry config for ChatModel failures. Optional.
    ModelRetryConfig *ModelRetryConfig

    // Middleware list
    Handlers []ChatModelAgentMiddleware
}
```

## ToolsConfig

```go
type ToolsConfig struct {
    compose.ToolsNodeConfig

    // Tools whose results cause the agent to return immediately (skip further ReAct loops).
    ReturnDirectly map[string]bool

    // When true, internal events from AgentTool sub-agents are emitted to the parent.
    EmitInternalEvents bool
}
```

`ToolsNodeConfig` comes from `github.com/cloudwego/eino/compose`:

```go
type ToolsNodeConfig struct {
    Tools              []tool.BaseTool
    ToolCallMiddlewares []compose.ToolCallMiddleware
}
```

## Creating Tools

Use `utils.InferTool` for quick tool creation:

```go
import (
    "github.com/cloudwego/eino/components/tool/utils"
)

type SearchInput struct {
    Query string `json:"query" jsonschema_description:"Search query"`
}

type SearchOutput struct {
    Results []string `json:"results"`
}

searchTool, err := utils.InferTool("web_search", "Search the web",
    func(ctx context.Context, input *SearchInput) (*SearchOutput, error) {
        return &SearchOutput{Results: []string{"result1"}}, nil
    })
```

For tools that accept options (needed for interrupt/resume):

```go
optionableTool, err := utils.InferOptionableTool("ask_user", "Ask user for input",
    func(ctx context.Context, input *AskInput, opts ...tool.Option) (string, error) {
        o := tool.GetImplSpecificOptions[myOptions](nil, opts...)
        if o.NewInput == nil {
            return "", compose.NewInterruptAndRerunErr(input.Question)
        }
        return *o.NewInput, nil
    })
```

## Complete Example with Streaming

```go
import (
    "context"
    "fmt"
    "io"
    "log"

    "github.com/cloudwego/eino-ext/components/model/openai"
    "github.com/cloudwego/eino/adk"
    "github.com/cloudwego/eino/components/tool"
    "github.com/cloudwego/eino/components/tool/utils"
    "github.com/cloudwego/eino/compose"
    "github.com/cloudwego/eino/schema"
)

func main() {
    ctx := context.Background()

    // Create model
    cm, _ := openai.NewChatModel(ctx, &openai.ChatModelConfig{
        APIKey: "your-key", Model: "gpt-4o",
    })

    // Create tools
    weatherTool, _ := utils.InferTool("get_weather", "Get weather for a city",
        func(ctx context.Context, input *struct {
            City string `json:"city" jsonschema_description:"City name"`
        }) (string, error) {
            return fmt.Sprintf("25C in %s", input.City), nil
        })

    calcTool, _ := utils.InferTool("calculator", "Basic math operations",
        func(ctx context.Context, input *struct {
            Expression string `json:"expression" jsonschema_description:"Math expression"`
        }) (string, error) {
            return "42", nil
        })

    // Create agent
    agent, _ := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
        Name:        "Assistant",
        Description: "A helpful assistant with weather and calculator tools",
        Instruction: "You are a helpful assistant. Use tools when needed.",
        Model:       cm,
        ToolsConfig: adk.ToolsConfig{
            ToolsNodeConfig: compose.ToolsNodeConfig{
                Tools: []tool.BaseTool{weatherTool, calcTool},
            },
        },
        MaxIterations: 10,
    })

    // Run with streaming
    runner := adk.NewRunner(ctx, adk.RunnerConfig{
        Agent:          agent,
        EnableStreaming: true,
    })

    iter := runner.Query(ctx, "What's the weather in Tokyo?")
    for {
        event, ok := iter.Next()
        if !ok {
            break
        }
        if event.Err != nil {
            log.Fatal(event.Err)
        }

        if event.Output != nil && event.Output.MessageOutput != nil {
            mv := event.Output.MessageOutput
            if mv.IsStreaming {
                // Handle streaming response
                for {
                    msg, err := mv.MessageStream.Recv()
                    if err == io.EOF {
                        break
                    }
                    if err != nil {
                        log.Fatal(err)
                    }
                    fmt.Print(msg.Content)
                }
                fmt.Println()
            } else {
                // Handle non-streaming response
                fmt.Printf("[%s] %s\n", mv.Role, mv.Message.Content)
            }
        }
    }
}
```

## Middleware

Add middleware via the `Handlers` field:

```go
agent, _ := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
    // ...
    Handlers: []adk.ChatModelAgentMiddleware{
        patchToolCallsMW,
        summarizationMW,
        filesystemMW,
    },
})
```

## ModelRetryConfig

Configure automatic retry on model failures:

```go
agent, _ := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
    // ...
    ModelRetryConfig: &adk.ModelRetryConfig{
        MaxRetries: 3,
        // Additional retry policy fields...
    },
})
```

When a streaming response fails and will be retried, the stream returns a `WillRetryError`. Handle it to show retry status to the user.

## SessionValues

SessionValues provide cross-agent key-value storage within a single run:

```go
// Set before running
runner.Run(ctx, msgs, adk.WithSessionValues(map[string]any{"user": "Alice"}))

// Access in Instruction via f-string
Instruction: "The current user is {user}."
```

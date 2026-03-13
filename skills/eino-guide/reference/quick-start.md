Three complete working examples: ChatModel Generate, ChatModelAgent with Runner, and a simple Graph chain.

## Prerequisites

- Go 1.18+
- An API key for OpenAI (or Ark, Ollama, etc.)

```bash
go get github.com/cloudwego/eino@latest
go get github.com/cloudwego/eino-ext/components/model/openai@latest
```

## Example 1: ChatModel Generate

Create a ChatModel, send messages, get a response. This is the simplest way to use Eino.

```go
package main

import (
    "context"
    "errors"
    "fmt"
    "io"
    "log"

    "github.com/cloudwego/eino-ext/components/model/openai"
    "github.com/cloudwego/eino/schema"
)

func main() {
    ctx := context.Background()

    // Create an OpenAI ChatModel
    model, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
        Model:  "gpt-4o",
        APIKey: "your-api-key",
    })
    if err != nil {
        log.Fatal(err)
    }

    // Construct input messages
    messages := []*schema.Message{
        schema.SystemMessage("You are a helpful assistant."),
        schema.UserMessage("Explain Go interfaces in one sentence."),
    }

    // Option A: Non-streaming (Generate)
    resp, err := model.Generate(ctx, messages)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println("Generate:", resp.Content)

    // Option B: Streaming (Stream)
    stream, err := model.Stream(ctx, messages)
    if err != nil {
        log.Fatal(err)
    }
    defer stream.Close()

    fmt.Print("Stream: ")
    for {
        chunk, err := stream.Recv()
        if errors.Is(err, io.EOF) {
            break
        }
        if err != nil {
            log.Fatal(err)
        }
        fmt.Print(chunk.Content)
    }
    fmt.Println()
}
```

Key points:
- `Generate()` returns a complete `*schema.Message`.
- `Stream()` returns a `*schema.StreamReader[*schema.Message]` -- always `defer stream.Close()`.
- Switch providers by changing the import and config (e.g., `ark.NewChatModel`, `ollama.NewChatModel`).

## Example 2: ChatModelAgent with Runner (Multi-Turn)

Use ADK to build a multi-turn conversational agent with tool calling.

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/cloudwego/eino/adk"
    "github.com/cloudwego/eino/components/tool"
    "github.com/cloudwego/eino/compose"
    "github.com/cloudwego/eino/schema"
    "github.com/cloudwego/eino-ext/components/model/openai"
    toolutils "github.com/cloudwego/eino/components/tool/utils"
)

// Define a simple tool
func getWeather(ctx context.Context, req *WeatherRequest) (string, error) {
    return fmt.Sprintf("Weather in %s: sunny, 22C", req.City), nil
}

type WeatherRequest struct {
    City string `json:"city" jsonschema_description:"The city to get weather for"`
}

func main() {
    ctx := context.Background()

    // Create ChatModel
    model, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
        Model:  "gpt-4o",
        APIKey: "your-api-key",
    })
    if err != nil {
        log.Fatal(err)
    }

    // Create tool from function
    weatherTool, err := toolutils.InferTool("get_weather", "Get current weather for a city", getWeather)
    if err != nil {
        log.Fatal(err)
    }

    // Create ChatModelAgent with tools
    agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
        Name:        "weather_assistant",
        Description: "An assistant that can check weather",
        Instruction: "You are a helpful weather assistant.",
        Model:       model,
        ToolsConfig: adk.ToolsConfig{
            ToolsNodeConfig: compose.ToolsNodeConfig{
                Tools: []tool.BaseTool{weatherTool},
            },
        },
    })
    if err != nil {
        log.Fatal(err)
    }

    // Create Runner
    runner := adk.NewRunner(ctx, adk.RunnerConfig{
        Agent:          agent,
        EnableStreaming: true,
    })

    // Multi-turn conversation
    history := make([]*schema.Message, 0)

    queries := []string{
        "What's the weather in Beijing?",
        "How about Tokyo?",
    }

    for _, query := range queries {
        fmt.Printf("\nUser: %s\n", query)
        history = append(history, schema.UserMessage(query))

        // Run agent
        events := runner.Run(ctx, history)
        var content string
        for {
            event, ok := events.Next()
            if !ok {
                break
            }
            if event.Err != nil {
                log.Fatal(event.Err)
            }
            if event.Output != nil && event.Output.MessageOutput != nil {
                msg := event.Output.MessageOutput.Message
                if msg != nil {
                    content += msg.Content
                    fmt.Print(msg.Content)
                }
            }
        }
        fmt.Println()

        // Append assistant response to history
        history = append(history, schema.AssistantMessage(content, nil))
    }
}
```

Key points:
- `tool/utils.InferTool` creates a tool from a Go function with automatic JSON schema inference.
- `Runner.Run()` returns an `AsyncIterator[*AgentEvent]` for streaming consumption.
- The agent handles the ReAct loop internally: model -> tool call -> tool result -> model -> response.
- Multi-turn is achieved by maintaining a `history` slice across `Run()` calls.

## Example 3: Simple Graph (Template -> Model -> Output)

Use the compose package to build a chain: ChatTemplate feeds into ChatModel.

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/cloudwego/eino/compose"
    "github.com/cloudwego/eino/components/prompt"
    "github.com/cloudwego/eino/schema"
    "github.com/cloudwego/eino-ext/components/model/openai"
)

func main() {
    ctx := context.Background()

    // Create ChatModel
    model, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
        Model:  "gpt-4o",
        APIKey: "your-api-key",
    })
    if err != nil {
        log.Fatal(err)
    }

    // Create ChatTemplate
    tpl, err := prompt.FromMessages(schema.Jinja2,
        schema.SystemMessage("You are a {{role}} expert."),
        schema.UserMessage("{{query}}"),
    )
    if err != nil {
        log.Fatal(err)
    }

    // Build a Chain: template -> model
    chain, err := compose.NewChain[map[string]any, *schema.Message]().
        AppendChatTemplate(tpl).
        AppendChatModel(model).
        Compile(ctx)
    if err != nil {
        log.Fatal(err)
    }

    // Invoke the chain
    result, err := chain.Invoke(ctx, map[string]any{
        "role":  "Go programming",
        "query": "What are the best practices for error handling in Go?",
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(result.Content)

    // Or use Stream for streaming output
    stream, err := chain.Stream(ctx, map[string]any{
        "role":  "Go programming",
        "query": "Explain goroutines in 3 sentences.",
    })
    if err != nil {
        log.Fatal(err)
    }
    defer stream.Close()

    for {
        chunk, err := stream.Recv()
        if err != nil {
            break
        }
        fmt.Print(chunk.Content)
    }
    fmt.Println()
}
```

Key points:
- `compose.NewChain` creates a linear pipeline with type-safe connections.
- `AppendChatTemplate` and `AppendChatModel` are convenience methods for adding component nodes.
- Compiled chains/graphs support all four execution modes: Invoke, Stream, Collect, Transform.
- Template variables are passed as `map[string]any` and rendered using Jinja2 syntax.

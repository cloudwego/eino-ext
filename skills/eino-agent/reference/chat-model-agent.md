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
type TypedChatModelAgentConfig[M MessageType] struct {
    // Name of the agent. Should be unique across all agents.
    Name string

    // Description of capabilities. Helps other agents decide whether to delegate.
    Description string

    // Instruction used as system prompt. Supports f-string placeholders for session values:
    // "The current time is {Time}. The user is {User}."
    Instruction string

    // The LLM model. Must support tool calling.
    Model model.BaseModel[M] // model.BaseChatModel or model.AgenticModel

    // Tool configuration (value type, not pointer)
    ToolsConfig ToolsConfig

    // Custom function to transform instruction + input into model messages.
    // Optional. Defaults to prepending instruction as system message.
    GenModelInput TypedGenModelInput[M]

    // Max ReAct iterations. Default: 20. Agent errors if exceeded.
    MaxIterations int

    // Retry config for ChatModel failures. Optional.
    ModelRetryConfig *TypedModelRetryConfig[M]

    // Failover config for model failures. Optional.
    ModelFailoverConfig *ModelFailoverConfig[M]

    // Middleware list (replaces deprecated Middlewares field)
    Handlers []TypedChatModelAgentMiddleware[M]
}

type ChatModelAgentConfig = TypedChatModelAgentConfig[*schema.Message]
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
    Tools               []tool.BaseTool
    ToolAliases         map[string]ToolAliasConfig
    UnknownToolsHandler func(ctx context.Context, name, input string) (string, error)
    ExecuteSequentially bool
    ToolArgumentsHandler func(ctx context.Context, name, arguments string) (string, error)
    ToolCallMiddlewares  []ToolMiddleware
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

See the "ModelRetryConfig (v0.9 Enhanced)" section below for the full retry API with output-based decisions, modified inputs, and backoff control.

When a streaming response will be retried, the stream emits a `WillRetryError` via `Recv()`. Handle it to show retry status to the user:

```go
// WillRetryError is returned by stream.Recv() when an attempt is rejected and will be retried.
// The stream continues after this error — call Recv() again to get the next attempt's chunks.
chunk, err := stream.Recv()
if err != nil {
    var willRetry *adk.WillRetryError
    if errors.As(err, &willRetry) {
        fmt.Printf("Retrying (attempt %d): %s\n", willRetry.RetryAttempt, willRetry.ErrStr)
        reason := willRetry.RejectReason() // custom reason from RetryDecision
        continue // next Recv() will return chunks from the retry attempt
    }
}
```

## ModelRetryConfig (v0.9 Enhanced)

Output-based retry with full decision control:

```go
agent, _ := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
    // ...
    ModelRetryConfig: &adk.ModelRetryConfig{
        MaxRetries: 3,
        ShouldRetry: func(ctx context.Context, retryCtx *adk.RetryContext) *adk.RetryDecision {
            // Access the full output message for decision
            if retryCtx.Err != nil {
                return &adk.RetryDecision{Retry: true}
            }
            if retryCtx.OutputMessage == nil || retryCtx.OutputMessage.Content == "" {
                return &adk.RetryDecision{
                    Retry:                 true,
                    ModifiedInputMessages: append(retryCtx.InputMessages, schema.UserMessage("Please provide a non-empty response")),
                    PersistModifiedInputMessages: true,
                    Backoff:               2 * time.Second,
                }
            }
            return &adk.RetryDecision{Retry: false}
        },
        // BackoffFunc provides the default delay between retries.
        // ShouldRetry can override this per-attempt via RetryDecision.Backoff (non-zero takes precedence).
        BackoffFunc: func(ctx context.Context, attempt int) time.Duration {
            return time.Duration(attempt) * time.Second // linear backoff
        },
    },
})
```

RetryContext fields:
- `RetryAttempt int` -- current retry attempt (1-based: first retry = 1)
- `InputMessages []M` -- messages sent to the model
- `OutputMessage M` -- full concatenated response (stream fully consumed for streaming)
- `Err error` -- error from model call (nil if output-based retry)
- `Options []model.Option` -- model options used

RetryDecision fields:
- `Retry bool` -- whether to retry
- `RewriteError error` -- replace the original error
- `ModifiedInputMessages []M` -- modified input for retry
- `PersistModifiedInputMessages bool` -- keep modified input in agent state
- `AdditionalOptions []model.Option` -- extra model options for retry
- `Backoff time.Duration` -- wait before retry (overrides BackoffFunc)
- `RejectReason any` -- attached to WillRetryError for stream consumers

## ModelFailoverConfig

Dynamic model switching when the primary model fails:

```go
agent, _ := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
    // ...
    ModelFailoverConfig: &adk.ModelFailoverConfig[*schema.Message]{
        MaxRetries: 2,
        ShouldFailover: func(ctx context.Context, outputMessage *schema.Message, outputErr error) bool {
            return outputErr != nil
        },
        GetFailoverModel: func(ctx context.Context, failoverCtx *adk.FailoverContext[*schema.Message]) (
            model.BaseModel[*schema.Message], []*schema.Message, error) {
            return backupModel, failoverCtx.InputMessages, nil
        },
    },
})
```

FailoverContext fields:
- `FailoverAttempt uint` -- current failover attempt
- `InputMessages []M` -- original input messages
- `LastOutputMessage M` -- last model output (may be nil)
- `LastErr error` -- last error (may be `*RetryExhaustedError` if retry is also configured)

## Cancel

Cancel provides safe, controllable termination during a run:

```go
cancelOpt, cancelFn := adk.WithCancel()
iter := runner.Query(ctx, "do something", cancelOpt)

// Later: cancel at safe point
handle, ok := cancelFn(
    adk.WithAgentCancelMode(adk.CancelAfterToolCalls),
    adk.WithAgentCancelTimeout(5*time.Second),
    adk.WithRecursive(),
)
if ok {
    err := handle.Wait()
    // err is *CancelError with checkpoint for resume
}
```

CancelMode values (bitmask, combinable):
- `CancelImmediate` (0) -- abort now, stream terminated with StreamCanceledError
- `CancelAfterChatModel` -- wait for model call to complete
- `CancelAfterToolCalls` -- wait for tool execution to complete

On cancel, a `CancelError` is delivered via the event stream. It contains `InterruptContexts` for checkpoint-based resumption.

## WithAfterToolCallsHook

Execute a callback after all tool calls in a ReAct iteration complete, before the next ChatModel call:

```go
iter := runner.Query(ctx, "do work",
    adk.WithAfterToolCallsHook(func(ctx context.Context) error {
        // Fires after tool execution finishes, before the next model call.
        // Useful for TurnLoop Push+Preempt patterns where pushed items
        // must be visible to the next turn's GenInput.
        fmt.Println("All tool calls completed")
        return nil
    }),
)
```

## SessionValues

SessionValues provide cross-agent key-value storage within a single run:

```go
// Set before running
runner.Run(ctx, msgs, adk.WithSessionValues(map[string]any{"user": "Alice"}))

// Access in Instruction via f-string
Instruction: "The current user is {user}."
```

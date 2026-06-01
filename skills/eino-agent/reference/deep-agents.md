DeepAgent is a pre-built agent on top of ChatModelAgent that provides planning, filesystem access, shell execution, and sub-agent delegation out of the box.

## Import

Requires eino >= v0.5.14:

```go
import "github.com/cloudwego/eino/adk/prebuilt/deep"
```

## When to Use

Use DeepAgent instead of plain ChatModelAgent when you need:

- Built-in task planning (WriteTodos tool)
- File system operations (read/write/edit/glob/grep)
- Shell command execution
- Sub-agent delegation with context isolation
- Auto-summarization for long conversations

Use plain ChatModelAgent when:

- You need fine-grained control over tools and prompts
- The task is simple (single tool, no planning needed)
- You want to minimize token cost (DeepAgent's planning adds overhead)

## Architecture

```
MainAgent (ChatModelAgent + built-in tools + prompt)
    |
    +-- WriteTodos tool (task planning)
    +-- Built-in tools (read_file, write_file, edit_file, glob, grep, execute)
    +-- TaskTool -> SubAgents
            |
            +-- GeneralPurpose (same tools as main, no TaskTool)
            +-- Custom SubAgents
```

- MainAgent receives user input, plans via WriteTodos, delegates via TaskTool
- SubAgents have isolated context (no shared history with MainAgent)
- GeneralPurpose sub-agent is added by default for generic tasks

## Configuration

```go
type TypedConfig[M adk.MessageType] struct {
    // Name is the identifier for the Deep agent.
    Name string
    // Description provides a brief explanation of the agent's purpose.
    Description string

    // Required: the LLM model.
    // If tools are used, it must support the model.WithTools call option.
    ChatModel model.BaseModel[M]

    // Optional: tools and tool-calling configuration.
    ToolsConfig adk.ToolsConfig

    // Optional: filesystem backend for file operations.
    Backend filesystem.Backend

    // Optional: shell execution (mutually exclusive with StreamingShell).
    Shell filesystem.Shell

    // Optional: streaming shell execution.
    StreamingShell filesystem.StreamingShell

    // Optional: custom sub-agents.
    // M = *schema.Message accepts standard agents; M = *schema.AgenticMessage accepts agentic agents.
    SubAgents []adk.TypedAgent[M]

    // Optional: custom system prompt (replaces built-in prompt when non-empty).
    Instruction string

    // Optional: max reasoning iterations.
    MaxIteration int

    // Optional: disable WriteTodos tool.
    WithoutWriteTodos bool

    // Optional: disable the default general-purpose sub-agent.
    WithoutGeneralSubAgent bool

    // Optional: custom TaskTool description generator.
    TaskToolDescriptionGenerator func(ctx context.Context, availableAgents []adk.TypedAgent[M]) (string, error)

    // Optional: interface-based middleware (recommended).
    Handlers []adk.TypedChatModelAgentMiddleware[M]

    // Optional: model retry configuration.
    ModelRetryConfig *adk.TypedModelRetryConfig[M]
    // Optional: model failover configuration.
    ModelFailoverConfig *adk.ModelFailoverConfig[M]
}

type Config = TypedConfig[*schema.Message]
```

## Quick Start

```go
import (
    "context"
    "fmt"
    "log"

    "github.com/cloudwego/eino-ext/components/model/openai"
    "github.com/cloudwego/eino/adk"
    "github.com/cloudwego/eino/adk/filesystem"
    "github.com/cloudwego/eino/adk/prebuilt/deep"
)

func main() {
    ctx := context.Background()

    cm, _ := openai.NewChatModel(ctx, &openai.ChatModelConfig{
        APIKey: "your-key", Model: "gpt-4o",
    })

    backend := filesystem.NewInMemoryBackend()

    agent, err := deep.New(ctx, &deep.Config{
        ChatModel: cm,
        Backend:   backend,
    })
    if err != nil {
        log.Fatal(err)
    }

    runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent})
    iter := runner.Query(ctx, "Analyze the CSV file at /data/sales.csv and create a summary report")

    for {
        event, ok := iter.Next()
        if !ok {
            break
        }
        if event.Err != nil {
            log.Fatal(event.Err)
        }
        if event.Output != nil && event.Output.MessageOutput != nil {
            msg, _ := event.Output.MessageOutput.GetMessage()
            fmt.Printf("[%s] %s\n", event.AgentName, msg.Content)
        }
    }
}
```

## Built-in Capabilities

### WriteTodos

A planning tool that lets the agent create and track a structured task list. The agent calls WriteTodos to decompose complex tasks, then updates progress as it works.

Disable with `WithoutWriteTodos: true`.

### File System Tools

When `Backend` is configured, the agent gets: `read_file`, `write_file`, `edit_file`, `glob`, `grep`.

When `Shell` or `StreamingShell` is configured, the agent also gets: `execute`.

### Task Delegation (TaskTool)

When `SubAgents` are configured (or the default general-purpose sub-agent is enabled), the agent gets a `task` tool. The agent specifies which sub-agent to call and what task to perform.

Sub-agents run in isolated context -- they only receive the task description, not the full conversation history.

## Comparison with Other Patterns

| Feature | DeepAgent | Plain ReAct | Plan-Execute |
|---------|-----------|-------------|--------------|
| Planning | Built-in (WriteTodos) | Manual | Separate planner agent |
| Context isolation | Yes (sub-agents) | No | Depends |
| Token cost | Higher (planning overhead) | Lower | Higher (separate plan) |
| Model requirements | Higher (must plan well) | Lower | Medium |

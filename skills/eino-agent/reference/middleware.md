ChatModelAgentMiddleware extends ChatModelAgent behavior at various execution stages.

## Interface

```go
type TypedChatModelAgentMiddleware[M MessageType] interface {
    BeforeAgent(ctx context.Context, runCtx *ChatModelAgentContext) (context.Context, *ChatModelAgentContext, error)
    AfterAgent(ctx context.Context, state *TypedChatModelAgentState[M]) (context.Context, error)
    BeforeModelRewriteState(ctx context.Context, state *TypedChatModelAgentState[M], mc *TypedModelContext[M]) (context.Context, *TypedChatModelAgentState[M], error)
    AfterModelRewriteState(ctx context.Context, state *TypedChatModelAgentState[M], mc *TypedModelContext[M]) (context.Context, *TypedChatModelAgentState[M], error)
    WrapInvokableToolCall(ctx context.Context, endpoint InvokableToolCallEndpoint, tCtx *ToolContext) (InvokableToolCallEndpoint, error)
    WrapStreamableToolCall(ctx context.Context, endpoint StreamableToolCallEndpoint, tCtx *ToolContext) (StreamableToolCallEndpoint, error)
    WrapEnhancedInvokableToolCall(ctx context.Context, endpoint EnhancedInvokableToolCallEndpoint, tCtx *ToolContext) (EnhancedInvokableToolCallEndpoint, error)
    WrapEnhancedStreamableToolCall(ctx context.Context, endpoint EnhancedStreamableToolCallEndpoint, tCtx *ToolContext) (EnhancedStreamableToolCallEndpoint, error)
    WrapModel(ctx context.Context, m model.BaseModel[M], mc *TypedModelContext[M]) (model.BaseModel[M], error)
}

// Convenience alias for classic message path
type ChatModelAgentMiddleware = TypedChatModelAgentMiddleware[*schema.Message]
```

Embed `*adk.BaseChatModelAgentMiddleware` to get default no-op implementations and only override what you need.

## Execution Flow

```
Agent.Run(input)
  -> BeforeAgent (once per run: modify instruction, tools)
  -> [ReAct Loop]
       -> BeforeModelRewriteState (modify messages before model call)
       -> WrapModel (wrap model for logging, metrics, etc.)
       -> Model.Generate/Stream
       -> AfterModelRewriteState (modify messages after model response)
       -> If tool calls:
            -> WrapInvokableToolCall / WrapStreamableToolCall
            -> Tool.Run()
            -> Results added to messages
       -> Continue loop
  -> AfterAgent (once per run: cleanup, final state processing)
  -> Agent.Run() ends
```

## Configuring Middleware

```go
agent, _ := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
    Handlers: []adk.ChatModelAgentMiddleware{mw1, mw2, mw3},
})
```

---

## FileSystem Middleware

Package: `github.com/cloudwego/eino/adk/middlewares/filesystem`

Provides file system access and shell execution tools to the agent.

```go
import "github.com/cloudwego/eino/adk/middlewares/filesystem"

mw, err := filesystem.New(ctx, &filesystem.MiddlewareConfig{
    Backend: myBackend,           // Required: filesystem.Backend implementation
    Shell:   myShell,             // Optional: shell execution (mutually exclusive with StreamingShell)
    StreamingShell: myStreamShell, // Optional: streaming shell
})
```

Injected tools: `ls`, `read_file`, `write_file`, `edit_file`, `glob`, `grep`, `execute` (if Shell/StreamingShell configured).

Backend implementations:
- `filesystem.NewInMemoryBackend()` -- in-memory (for testing)
- `github.com/cloudwego/eino-ext/adk/backend/local` -- local filesystem (Unix/macOS)
- `github.com/cloudwego/eino-ext/adk/backend/agentkit` -- Volcengine sandbox

---

## ToolSearch Middleware

Package: `github.com/cloudwego/eino/adk/middlewares/dynamictool/toolsearch`

Dynamic tool selection for large tool libraries. Adds a `tool_search` meta-tool that accepts regex to find tools by name.

```go
import "github.com/cloudwego/eino/adk/middlewares/dynamictool/toolsearch"

mw, err := toolsearch.New(ctx, &toolsearch.Config{
    DynamicTools: []tool.BaseTool{weatherTool, stockTool, currencyTool},
})
```

Flow:
1. Initially, only `tool_search` is visible to the model
2. Model calls `tool_search(regex_pattern="weather.*")`
3. Matched tools become available for subsequent model calls
4. Multiple searches accumulate results

---

## ToolReduction Middleware

Package: `github.com/cloudwego/eino/adk/middlewares/reduction`

Controls token usage from tool results via two strategies:

- **Truncation**: Immediately truncate oversized tool results, save full content to backend
- **Clear**: When total tokens exceed threshold, offload old tool results to files

```go
import "github.com/cloudwego/eino/adk/middlewares/reduction"

mw, err := reduction.New(ctx, &reduction.Config{
    Backend:           myBackend,     // Required: storage for offloaded content
    MaxLengthForTrunc: 50000,         // Default: 50000 chars
    MaxTokensForClear: 160000,        // Default: 160000 tokens
    SkipTruncation:    false,         // Set true to skip truncation
    SkipClear:         false,         // Set true to skip clearing
})
```

---

## Summarization Middleware

Package: `github.com/cloudwego/eino/adk/middlewares/summarization`

Automatically compresses conversation history when token count exceeds a threshold.

```go
import "github.com/cloudwego/eino/adk/middlewares/summarization"

mw, err := summarization.New(ctx, &summarization.Config{
    Model: summarizationModel,  // Required: model used to generate summaries
    Trigger: &summarization.TriggerCondition{
        ContextTokens: 160000,  // Default: 160000
    },
    TranscriptFilePath: "/path/to/transcript.txt",  // Optional: save full transcript
    PreserveUserMessages: &summarization.PreserveUserMessages{
        Enabled:   true,       // Default: true (when nil)
        MaxTokens: 30000,      // Default: 30000. Keep recent user messages up to this limit
    },
})
```

How it works:
1. In `BeforeModelRewriteState`, counts tokens in current messages
2. If tokens exceed threshold, calls the summary model to compress history
3. Replaces old messages with a summary message + preserved recent user messages

---

## PlanTask Middleware

Package: `github.com/cloudwego/eino/adk/middlewares/plantask`

Injects task management tools for the agent to create and track tasks.

```go
import "github.com/cloudwego/eino/adk/middlewares/plantask"

mw, err := plantask.New(ctx, &plantask.Config{
    Backend: myBackend,  // Required: storage backend (should be session-scoped)
    BaseDir: "/tasks",   // Required: directory for task files
})
```

Injected tools:
- `TaskCreate` -- create a new task with subject, description
- `TaskGet` -- get task details by ID
- `TaskUpdate` -- update status, add dependencies, set owner
- `TaskList` -- list all tasks with status

Task status flow: `pending` -> `in_progress` -> `completed` (or `deleted` from any state).

Tasks support dependency management (`blocks`/`blockedBy`) with circular dependency detection.

---

## Skill Middleware

Package: `github.com/cloudwego/eino/adk/middlewares/skill`

Enables progressive disclosure of skills. At startup, the agent sees skill names and descriptions. When a task matches, it loads the full SKILL.md content.

```go
import (
    "github.com/cloudwego/eino/adk/middlewares/skill"
    "github.com/cloudwego/eino-ext/adk/backend/local"
)

// Create filesystem backend
be, _ := local.NewBackend(ctx, &local.Config{})

// Create skill backend from filesystem
skillBackend, _ := skill.NewBackendFromFilesystem(ctx, &skill.BackendFromFilesystemConfig{
    Backend: be,
    BaseDir: "/path/to/skills",  // Directory containing skill folders
})

// Create middleware
mw, _ := skill.NewMiddleware(ctx, &skill.Config{
    Backend: skillBackend,
})
```

Skill directory structure:
```
skills/
  my-skill/
    SKILL.md          # Required: frontmatter (name, description) + instructions
    scripts/          # Optional: executable code
    references/       # Optional: reference docs
```

Context modes in SKILL.md frontmatter:
- (empty) -- inline: skill content returned as tool result
- `fork` -- new agent with clean context, discarding parent message history
- `fork_with_context` -- new agent carrying over parent message history

---

## PatchToolCalls Middleware

Package: `github.com/cloudwego/eino/adk/middlewares/patchtoolcalls`

Fixes "dangling tool calls" -- assistant messages with tool calls that lack corresponding tool response messages. Common in interrupted sessions or human-in-the-loop.

```go
import "github.com/cloudwego/eino/adk/middlewares/patchtoolcalls"

mw, _ := patchtoolcalls.New(ctx, nil)  // nil config uses defaults

// Custom placeholder message
mw, _ := patchtoolcalls.New(ctx, &patchtoolcalls.Config{
    PatchedContentGenerator: func(ctx context.Context, toolName, toolCallID string) (string, error) {
        return fmt.Sprintf("Tool %s (call %s) was cancelled.", toolName, toolCallID), nil
    },
})
```

Place this middleware first in the chain to ensure clean message history for other middleware.

---

## Agents.md Middleware

Package: `github.com/cloudwego/eino/adk/middlewares/agentsmd`

Injects Agents.md file contents into model input as transient context. The injected content is excluded from summarization to avoid polluting compressed history.

```go
import "github.com/cloudwego/eino/adk/middlewares/agentsmd"

mw, err := agentsmd.New(ctx, &agentsmd.Config{
    Backend:       myBackend,                        // Required: file access backend
    AgentsMDFiles: []string{"/path/to/Agents.md"},   // Ordered list of files to load
})
```

Use this middleware when you want to provide persistent reference documentation to the agent without it being summarized away. The content is re-injected fresh on each model call.

---

## Run-Local State

Middleware can persist key-value state that survives interrupt/resume cycles:

```go
// Inside any middleware method
adk.SetRunLocalValue(ctx, "myKey", myValue)

// Read later (even after resume)
val, ok, err := adk.GetRunLocalValue(ctx, "myKey")

// Delete
adk.DeleteRunLocalValue(ctx, "myKey")
```

Values must be gob-serializable. Register custom types in `init()`.

---

## Event Emission from Middleware

Middleware can emit custom events to the agent's event stream:

```go
adk.SendEvent(ctx, &adk.AgentEvent{
    AgentName: "MyAgent",
    Output: &adk.AgentOutput{
        CustomizedOutput: myData,
    },
})
```

---

## Recommended Middleware Order

```go
Handlers: []adk.ChatModelAgentMiddleware{
    patchToolCallsMW,    // 1. Fix message history first
    agentsMdMW,          // 2. Inject reference docs
    summarizationMW,     // 3. Compress if needed
    reductionMW,         // 4. Handle large tool results
    filesystemMW,        // 5. Add file tools
    skillMW,             // 6. Add skill discovery
    planTaskMW,          // 7. Add task management
}
```

## Language Support

All built-in middleware supports English (default) and Chinese prompts:

```go
adk.SetLanguage(adk.LanguageChinese)  // Switch to Chinese
adk.SetLanguage(adk.LanguageEnglish)  // Switch to English (default)
```

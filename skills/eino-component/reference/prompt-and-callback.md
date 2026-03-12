# Prompt and Callback Reference

ChatTemplate formats prompt messages; Callback handlers provide observability and tracing.

## ChatTemplate Interface

```go
// github.com/cloudwego/eino/components/prompt
type ChatTemplate interface {
    Format(ctx context.Context, vs map[string]any, opts ...Option) ([]*schema.Message, error)
}
```

## Creating Templates

### FromMessages

Build a template from multiple message templates:

```go
import "github.com/cloudwego/eino/components/prompt"

template := prompt.FromMessages(ctx,
    schema.SystemMessage("You are a {role}. {instructions}"),
    schema.UserMessage("{user_input}"),
)

messages, err := template.Format(ctx, map[string]any{
    "role":         "helpful assistant",
    "instructions": "Be concise.",
    "user_input":   "What is Eino?",
})
// Returns []*schema.Message with variables substituted
```

### FString Format

Uses `{variable}` syntax -- simple and direct:

```go
msg := &schema.Message{
    Role:    schema.System,
    Content: "You are a {role}. Help the user with {task}.",
}
// schema.Message implements ChatTemplate
messages, err := msg.Format(ctx, map[string]any{
    "role": "code reviewer",
    "task": "reviewing Go code",
})
```

### GoTemplate Format

Uses Go `text/template` syntax for complex logic:

```go
template := prompt.FromMessages(ctx,
    &schema.Message{
        Role:    schema.System,
        Content: "{{if .expert}}As an expert{{end}} help with {{.topic}}",
        // Template type is inferred from syntax
    },
)
```

### Jinja2 Format

```go
// Uses Jinja2 syntax
msg := &schema.Message{
    Role:    schema.System,
    Content: "{% if level == 'expert' %}Expert mode{% endif %} Topic: {{topic}}",
}
```

### Message Helpers

```go
schema.SystemMessage("system prompt")
schema.UserMessage("user question")
schema.AssistantMessage("assistant response")
schema.ToolMessage("tool result", "tool-call-id")
```

## Callback Handlers

Callback handlers observe component execution for tracing and monitoring. They implement `callbacks.Handler` from `github.com/cloudwego/eino/callbacks`.

### Available Implementations

| Handler | Import Path | Description |
|---------|-------------|-------------|
| Langfuse | `github.com/cloudwego/eino-ext/callbacks/langfuse` | Langfuse observability platform |
| Langsmith | `github.com/cloudwego/eino-ext/callbacks/langsmith` | LangSmith tracing |
| CozeLoop | `github.com/cloudwego/eino-ext/callbacks/cozeloop` | Coze observability |
| APMPlus | `github.com/cloudwego/eino-ext/callbacks/apmplus` | ByteDance APM |

### Langfuse Example

```go
import (
    "github.com/cloudwego/eino-ext/callbacks/langfuse"
    "github.com/cloudwego/eino/callbacks"
)

cbh, flusher := langfuse.NewLangfuseHandler(&langfuse.Config{
    Host:        "https://cloud.langfuse.com",
    PublicKey:   "pk-lf-...",
    SecretKey:   "sk-lf-...",
    ServiceName: "my-eino-app",
})

// Register globally -- all components will be traced
callbacks.AppendGlobalHandlers(cbh)

// Flush before exit
defer flusher.Flush(ctx)
```

### Langsmith Example

```go
import (
    "github.com/cloudwego/eino-ext/callbacks/langsmith"
    "github.com/cloudwego/eino/callbacks"
)

cbh := langsmith.NewLangsmithHandler(&langsmith.Config{
    APIKey:  "ls-...",
    BaseURL: "https://api.smith.langchain.com",
    Project: "my-project",
})

callbacks.AppendGlobalHandlers(cbh)
```

### Registration Pattern

```go
// Global registration -- applies to all components
callbacks.AppendGlobalHandlers(handler1, handler2)

// Per-run registration -- pass via context in Compose graphs
ctx = callbacks.CtxWithHandlers(ctx, handler)
```

Callbacks automatically capture: component type, input/output, latency, errors, and streaming data.

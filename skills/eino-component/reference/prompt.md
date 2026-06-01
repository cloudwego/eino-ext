# Prompt Reference

ChatTemplate formats prompt messages with variable substitution.

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

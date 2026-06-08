<!--
Copyright 2026 CloudWeGo Authors
-->

# OpenAI AgenticModel

Use `agenticopenai` when you need the `model.AgenticModel` path backed by OpenAI Responses API and `*schema.AgenticMessage`.

```go
import "github.com/cloudwego/eino-ext/components/model/agenticopenai"
```

## Configuration

```go
am, err := agenticopenai.New(ctx, &agenticopenai.Config{
    APIKey: "your-key", // Required
    Model:  "gpt-4o",   // Required
})
```

`agenticopenai.Config` fields:

| Field | Type | Notes |
|-------|------|-------|
| `ByAzure` | `bool` | Use Azure OpenAI authentication when true |
| `BaseURL` | `string` | Optional custom endpoint |
| `APIKey` | `string` | Required |
| `Timeout` | `*time.Duration` | Optional request timeout |
| `HTTPClient` | `*http.Client` | Optional custom HTTP client |
| `MaxRetries` | `*int` | Optional SDK retry count |
| `Model` | `string` | Required model ID |
| `MaxTokens` | `*int` | Optional maximum output tokens |
| `Temperature` | `*float32` | Optional, range 0.0 to 2.0 |
| `TopP` | `*float32` | Optional, range 0.0 to 1.0 |
| `ServiceTier` | `*responses.ResponseNewParamsServiceTier` | Optional service tier |
| `Text` | `*responses.ResponseTextConfigParam` | Optional text output config |
| `Reasoning` | `*responses.ReasoningParam` | Optional reasoning config |
| `Store` | `*bool` | Optional server-side response storage |
| `MaxToolCalls` | `*int` | Optional maximum tool calls |
| `ParallelToolCalls` | `*bool` | Optional parallel tool call switch |
| `Include` | `[]responses.ResponseIncludable` | Optional additional response fields |
| `ServerTools` | `[]*agenticopenai.ServerToolConfig` | Optional hosted tools |
| `MCPTools` | `[]*responses.ToolMcpParam` | Optional MCP tools |
| `Truncation` | `*responses.ResponseNewParamsTruncation` | Optional truncation behavior |
| `CustomHeaders` | `map[string]string` | Optional request headers |
| `ExtraFields` | `map[string]any` | Optional raw JSON fields added to the request |

`ServerToolConfig` supports `WebSearch`, `FileSearch`, `CodeInterpreter`, and `Shell`.

## Call Options

Provider-specific options:

```go
resp, err := am.Generate(ctx, messages,
    agenticopenai.WithReasoning(reasoning),
    agenticopenai.WithMaxToolCalls(4),
    agenticopenai.WithParallelToolCalls(true),
    agenticopenai.WithServerTools(serverTools),
    agenticopenai.WithMCPTools(mcpTools),
    agenticopenai.WithPromptCacheKey("stable-prefix-key"),
    agenticopenai.WithCustomHeaders(map[string]string{"x-trace-id": traceID}),
    agenticopenai.WithExtraFields(map[string]any{"metadata": map[string]string{"env": "prod"}}),
)
```

Available provider options: `WithStore`, `WithPromptCacheKey`, `WithReasoning`, `WithText`, `WithMaxToolCalls`, `WithParallelToolCalls`, `WithServerTools`, `WithMCPTools`, `WithCustomHeaders`, `WithExtraFields`, `WithTruncation`.

Common model options also apply, including `model.WithModel`, `model.WithMaxTokens`, `model.WithTemperature`, `model.WithTopP`, `model.WithTools`, `model.WithDeferredTools`, `model.WithToolSearchTool`, and `model.WithAgenticToolChoice`.

## Tools

For code that works against the `model.AgenticModel` interface, pass tools at call time:

```go
resp, err := am.Generate(ctx,
    []*schema.AgenticMessage{schema.UserAgenticMessage("search the web")},
    model.WithTools(toolInfos),
)
```

Do not rely on concrete-only methods when documenting general AgenticModel usage.

## Notes

- `model.WithStop` and classic `model.WithToolChoice` are rejected by this implementation.
- Use `model.WithAgenticToolChoice` for agentic tool-choice control.
- `model.WithDeferredTools` automatically adds hosted tool search when no explicit tool-search tool is provided.

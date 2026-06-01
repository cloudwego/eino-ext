<!--
Copyright 2026 CloudWeGo Authors
-->

# Ark AgenticModel

Use `agenticark` when you need the `model.AgenticModel` path backed by Volcengine Ark Responses API and `*schema.AgenticMessage`.

```go
import "github.com/cloudwego/eino-ext/components/model/agenticark"
```

## Configuration

```go
am, err := agenticark.New(ctx, &agenticark.Config{
    APIKey: "your-key",    // Required unless AccessKey + SecretKey are set
    Model:  "endpoint-id", // Required: Ark endpoint ID
})
```

`agenticark.Config` fields:

| Field | Type | Notes |
|-------|------|-------|
| `Timeout` | `*time.Duration` | Optional; ignored when `HTTPClient` is set |
| `HTTPClient` | `*http.Client` | Optional custom HTTP client |
| `RetryTimes` | `*int` | Optional retry count |
| `BaseURL` | `string` | Optional custom Ark endpoint |
| `Region` | `string` | Optional region |
| `APIKey` | `string` | Preferred authentication |
| `AccessKey` | `string` | Alternative authentication, used with `SecretKey` |
| `SecretKey` | `string` | Alternative authentication, used with `AccessKey` |
| `Model` | `string` | Required model endpoint ID |
| `MaxTokens` | `*int` | Optional maximum output tokens |
| `Temperature` | `*float32` | Optional, range 0.0 to 2.0 |
| `TopP` | `*float32` | Optional, range 0.0 to 1.0 |
| `ServiceTier` | `*responses.ResponsesServiceTier_Enum` | Optional service tier |
| `Text` | `*responses.ResponsesText` | Optional text output config |
| `Thinking` | `*responses.ResponsesThinking` | Optional thinking mode config |
| `Reasoning` | `*responses.ResponsesReasoning` | Optional reasoning config |
| `EnablePassBackReasoning` | `*bool` | Optional; default true |
| `MaxToolCalls` | `*int64` | Optional maximum tool calls |
| `ParallelToolCalls` | `*bool` | Optional parallel tool call switch |
| `ServerTools` | `[]*agenticark.ServerToolConfig` | Optional server-side tools |
| `MCPTools` | `[]*responses.ToolMcp` | Optional MCP tools |
| `Cache` | `*agenticark.CacheConfig` | Optional session-cache config |
| `ContextManagement` | `*contextmanagement.ContextManagement` | Optional context management |
| `CustomHeaders` | `map[string]string` | Optional request headers |

`ServerToolConfig` supports `WebSearch`, `ImageProcess`, `DoubaoApp`, and `KnowledgeSearch`.

## Call Options

Provider-specific options:

```go
resp, err := am.Generate(ctx, messages,
    agenticark.WithThinking(thinking),
    agenticark.WithReasoning(reasoning),
    agenticark.WithMaxToolCalls(4),
    agenticark.WithParallelToolCalls(true),
    agenticark.WithServerTools(serverTools),
    agenticark.WithMCPTools(mcpTools),
    agenticark.WithCache(cacheOpt),
    agenticark.WithContextManagement(contextManagement),
    agenticark.WithCustomHeaders(map[string]string{"x-trace-id": traceID}),
)
```

Available provider options: `WithReasoning`, `WithThinking`, `WithText`, `WithMaxToolCalls`, `WithParallelToolCalls`, `WithServerTools`, `WithMCPTools`, `WithCustomHeaders`, `WithCache`, `WithContextManagement`.

Common model options also apply, including `model.WithModel`, `model.WithMaxTokens`, `model.WithTemperature`, `model.WithTopP`, `model.WithTools`, and `model.WithAgenticToolChoice`.

## Cache

Session cache is configured by `CacheConfig` or overridden per request with `WithCache`:

```go
resp, err := am.Generate(ctx, messages,
    agenticark.WithCache(&agenticark.CacheOption{
        HeadPreviousResponseID: previousResponseID,
        SessionCache: &agenticark.SessionCacheConfig{
            EnableCache: true,
            ExpireAtSec: expireAt,
        },
    }),
)
```

`CreatePrefixCache(ctx, prefix, expireAtSec, opts...)` creates server-side prefix context and returns `*agenticark.CacheInfo`.

## Tools

For code that works against the `model.AgenticModel` interface, pass tools at call time:

```go
resp, err := am.Generate(ctx,
    []*schema.AgenticMessage{schema.UserAgenticMessage("use a function tool")},
    model.WithTools(toolInfos),
)
```

## Notes

- `model.WithStop` and classic `model.WithToolChoice` are rejected by this implementation.
- Use `model.WithAgenticToolChoice` for agentic tool-choice control.
- When a cached previous response is used, function/server/MCP tool population and tool choice are skipped for that request.

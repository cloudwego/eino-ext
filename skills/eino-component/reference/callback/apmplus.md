# APMPlus Callback

APMPlus provides tracing, metrics, and token usage tracking for Eino applications on the ByteDance APM platform.

```go
import (
    "github.com/cloudwego/eino-ext/callbacks/apmplus"
    "github.com/cloudwego/eino/callbacks"
)
```

## Setup

```go
ctx := context.Background()

cbh, shutdown, err := apmplus.NewApmplusHandler(&apmplus.Config{
    Host:        "apmplus-cn-beijing.volces.com:4317",
    AppKey:      "appkey-xxx",
    ServiceName: "my-eino-app",
})
if err != nil {
    log.Fatal(err)
}
defer shutdown(ctx)

callbacks.AppendGlobalHandlers(cbh)
```

## Constructor

```go
func NewApmplusHandler(cfg *Config) (handler callbacks.Handler, shutdown func(ctx context.Context) error, err error)
```

Returns three values:
- `handler` -- the callback handler
- `shutdown` -- cleanup function, must be called to flush traces/metrics before exit
- `err` -- initialization error

## Config

```go
type Config struct {
    // Host is the APMPlus URL (required)
    Host string

    // AppKey is the authentication key (required)
    AppKey string

    // ServiceName identifies your service (required)
    ServiceName string

    // Release is the version identifier (optional)
    Release string

    // ResourceAttributes are custom attributes (optional)
    ResourceAttributes map[string]string
}
```

All of `Host`, `AppKey`, and `ServiceName` are required.

## Session Support

Associate multiple requests with a session for grouped tracing:

```go
ctx = apmplus.SetSession(ctx,
    apmplus.WithSessionID("session_abc"),
    apmplus.WithUserID("user_123"),
)

// Subsequent Eino calls with this ctx are grouped under the session
resp, _ := chatModel.Generate(ctx, messages)
```

## Full Example

```go
func main() {
    ctx := context.Background()

    cbh, shutdown, err := apmplus.NewApmplusHandler(&apmplus.Config{
        Host:        "apmplus-cn-beijing.volces.com:4317",
        AppKey:      "appkey-xxx",
        ServiceName: "eino-app",
        Release:     "v1.0.0",
    })
    if err != nil {
        log.Fatal(err)
    }
    defer shutdown(ctx)

    callbacks.AppendGlobalHandlers(cbh)

    // Set session for request grouping
    ctx = apmplus.SetSession(ctx,
        apmplus.WithSessionID("session_001"),
        apmplus.WithUserID("user_001"),
    )

    // All Eino component calls are now traced with APMPlus
    chatModel, _ := openai.NewChatModel(ctx, &openai.ChatModelConfig{
        Model: "gpt-4o",
    })
    resp, _ := chatModel.Generate(ctx, []*schema.Message{
        {Role: schema.User, Content: "Hello"},
    })
}
```

Features tracked: traces, metrics, token usage, streaming performance, and error reporting.

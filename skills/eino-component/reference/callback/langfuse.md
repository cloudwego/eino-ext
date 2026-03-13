# Langfuse Callback

Langfuse provides open-source observability and tracing for LLM applications.

```go
import (
    "github.com/cloudwego/eino-ext/callbacks/langfuse"
    "github.com/cloudwego/eino/callbacks"
)
```

## Setup

```go
cbh, flusher := langfuse.NewLangfuseHandler(&langfuse.Config{
    Host:      "https://cloud.langfuse.com",
    PublicKey: "pk-lf-...",
    SecretKey: "sk-lf-...",
})

callbacks.AppendGlobalHandlers(cbh)
defer flusher()
```

## Constructor

```go
func NewLangfuseHandler(cfg *Config) (handler *CallbackHandler, flusher func())
```

Returns:
- `handler` -- the callback handler
- `flusher` -- flush function, call before exit to send remaining events

## Config

```go
type Config struct {
    Host             string            // Langfuse server URL (required)
    PublicKey        string            // Public key (required)
    SecretKey        string            // Secret key (required)
    Threads          int               // Concurrent workers (default: 1)
    Timeout          time.Duration     // HTTP timeout
    MaxTaskQueueSize int               // Event buffer size (default: 100)
    FlushAt          int               // Batch size before sending (default: 15)
    FlushInterval    time.Duration     // Auto-flush interval (default: 500ms)
    SampleRate       float64           // Event sampling rate (default: 1.0)
    MaskFunc         func(string) string // Mask sensitive data
    MaxRetry         uint64            // Max retry attempts (default: 3)
    Name             string            // Trace name
    UserID           string            // User identifier
    SessionID        string            // Session identifier
    Release          string            // Version identifier
    Tags             []string          // Labels attached to trace
    Public           bool              // Publicly accessible (default: false)
}
```

# Langsmith Callback

LangSmith provides tracing and evaluation for LLM applications.

```go
import (
    "github.com/cloudwego/eino-ext/callbacks/langsmith"
    "github.com/cloudwego/eino/callbacks"
)
```

## Setup

```go
cbh, err := langsmith.NewLangsmithHandler(&langsmith.Config{
    APIKey: "ls-...",
    APIURL: "https://api.smith.langchain.com",
})
if err != nil {
    log.Fatal(err)
}

callbacks.AppendGlobalHandlers(cbh)
```

## Constructor

```go
func NewLangsmithHandler(cfg *Config) (*CallbackHandler, error)
```

## Config

```go
type Config struct {
    APIKey   string                           // LangSmith API key (required)
    APIURL   string                           // API URL (default: https://api.smith.langchain.com)
    RunIDGen func(ctx context.Context) string // Custom run_id generator (optional)
}
```

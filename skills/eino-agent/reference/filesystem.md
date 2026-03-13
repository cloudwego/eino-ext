# Filesystem Backend

The `adk/filesystem` package defines a pluggable file backend interface used by DeepAgent and filesystem-related middleware.

Interface: `github.com/cloudwego/eino/adk/filesystem`
Implementations: `github.com/cloudwego/eino-ext/adk/backend/{local,agentkit}`

## Backend Interface

```go
// github.com/cloudwego/eino/adk/filesystem
type Backend interface {
    LsInfo(ctx context.Context, req *LsInfoRequest) ([]FileInfo, error)
    Read(ctx context.Context, req *ReadRequest) (*FileContent, error)
    GrepRaw(ctx context.Context, req *GrepRequest) ([]GrepMatch, error)
    GlobInfo(ctx context.Context, req *GlobInfoRequest) ([]FileInfo, error)
    Write(ctx context.Context, req *WriteRequest) error
    Edit(ctx context.Context, req *EditRequest) error
}

type Shell interface {
    Execute(ctx context.Context, input *ExecuteRequest) (*ExecuteResponse, error)
}

type StreamingShell interface {
    ExecuteStreaming(ctx context.Context, input *ExecuteRequest) (*schema.StreamReader[*ExecuteResponse], error)
}
```

Backend provides file operations (ls, read, grep, glob, write, edit). Shell and StreamingShell provide command execution. DeepAgent uses all three.

## Implementations

### Local

Local filesystem backend. Operates directly on the host's file system.

```go
import "github.com/cloudwego/eino-ext/adk/backend/local"

backend, err := local.NewBackend(ctx, &local.Config{
    RootDir: "/path/to/workspace",
})
```

### AgentKit (Sandbox)

Remote sandbox backend via ByteDance AgentKit. Runs file operations and code execution in an isolated cloud environment.

```go
import "github.com/cloudwego/eino-ext/adk/backend/agentkit"

backend, err := agentkit.NewBackend(ctx, &agentkit.Config{
    AccessKeyID:    "your-access-key",
    SecretAccessKey: "your-secret-key",
    Region:         agentkit.RegionOfBeijing,
    SandboxID:      "sandbox-id",
})
```

## Usage with DeepAgent

```go
import (
    "github.com/cloudwego/eino/adk/prebuilt/deep"
    "github.com/cloudwego/eino-ext/adk/backend/local"
)

backend, _ := local.NewBackend(ctx, &local.Config{
    RootDir: "./workspace",
})

agent, _ := deep.New(ctx, &deep.Config{
    Model:   chatModel,
    Backend: backend,
})
```

## Key Types

```go
type FileInfo struct {
    Path       string // file/directory path
    IsDir      bool
    Size       int64  // bytes
    ModifiedAt string // ISO 8601 format
}

type ReadRequest struct {
    FilePath string
    Offset   int // 1-based line number (default: 1)
    Limit    int // max lines to read (default: 2000)
}

type GrepRequest struct {
    Pattern         string // regex pattern (ripgrep syntax)
    Path            string // search scope directory
    Glob            string // file path filter (e.g., "*.go")
    FileType        string // file type filter (e.g., "go", "py")
    CaseInsensitive bool
    EnableMultiline bool
    AfterLines      int // context lines after match
    BeforeLines     int // context lines before match
}

type GlobInfoRequest struct {
    Pattern string // glob expression (e.g., "**/*.go")
    Path    string // base directory
}

type WriteRequest struct {
    FilePath string
    Content  string
}

type EditRequest struct {
    FilePath   string
    OldString  string // must be non-empty, matched literally
    NewString  string // must differ from OldString
    ReplaceAll bool   // false: fail if not exactly one match
}
```

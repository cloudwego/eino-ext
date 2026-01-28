# AIO Sandbox

AIO Sandbox is an implementation of the `commandline.Operator` interface that provides a remote sandboxed environment using the [AIO Sandbox](https://github.com/agent-infra/sandbox).

## Features

- **Remote Execution**: Execute commands in a cloud-based sandbox environment
- **Session Persistence**: Optionally maintain shell session state across commands
- **File Operations**: Read, write, and manage files in the sandbox
- **Secure**: Path traversal protection and isolated execution environment
- **Compatible**: Works seamlessly with eino's `StrReplaceEditor` and `PyExecutor` tools

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/tool/commandline/aiosandbox
```

## Usage

### Basic Usage

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/cloudwego/eino-ext/components/tool/commandline/aiosandbox"
)

func main() {
    ctx := context.Background()

    // Create AIO Sandbox
    sandbox, err := aiosandbox.NewAIOSandbox(ctx, &aiosandbox.Config{
        BaseURL:     "https://xxxx.apigateway-cn-beijing.volceapi.com",
        Token:       "your-api-token",
        WorkDir:     "/workspace",
        Timeout:     120,
        KeepSession: true,
    })
    if err != nil {
        log.Fatal(err)
    }
    defer sandbox.Close(ctx)

    // Execute a command
    output, err := sandbox.RunCommand(ctx, []string{"python3", "--version"})
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Python version: %s\n", output.Stdout)

    // Write a file
    err = sandbox.WriteFile(ctx, "/workspace/hello.py", `print("Hello, World!")`)
    if err != nil {
        log.Fatal(err)
    }

    // Read a file
    content, err := sandbox.ReadFile(ctx, "/workspace/hello.py")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("File content: %s\n", content)
}
```

### Integration with Eino Tools

```go
package main

import (
    "context"
    "log"

    "github.com/cloudwego/eino-ext/components/tool/commandline"
    "github.com/cloudwego/eino-ext/components/tool/commandline/aiosandbox"
    "github.com/cloudwego/eino/components/tool"
)

func main() {
    ctx := context.Background()

    // Create AIO Sandbox
    sandbox, err := aiosandbox.NewAIOSandbox(ctx, &aiosandbox.Config{
        BaseURL:     "https://xxxx.apigateway-cn-beijing.volceapi.com",
        WorkDir:     "/workspace",
        KeepSession: true,
    })
    if err != nil {
        log.Fatal(err)
    }
    defer sandbox.Close(ctx)

    // Create StrReplaceEditor with AIO Sandbox
    editor, err := commandline.NewStrReplaceEditor(ctx, &commandline.EditorConfig{
        Operator: sandbox,
    })
    if err != nil {
        log.Fatal(err)
    }

    // Create PyExecutor with AIO Sandbox
    pyExecutor, err := commandline.NewPyExecutor(ctx, &commandline.PyExecutorConfig{
        Command:  "python3",
        Operator: sandbox,
    })
    if err != nil {
        log.Fatal(err)
    }

    // Use tools in your agent
    tools := []tool.BaseTool{editor, pyExecutor}
    _ = tools
}
```

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `BaseURL` | string | (required) | AIO Sandbox API endpoint |
| `Token` | string | (required) | Authentication token |
| `WorkDir` | string | `/tmp` | Working directory in sandbox |
| `Timeout` | float64 | `60.0` | Command timeout in seconds |
| `KeepSession` | bool | `false` | Reuse shell sessions for stateful execution |

## API Reference

### Methods

#### `NewAIOSandbox(ctx, config) (*AIOSandbox, error)`
Creates a new AIO Sandbox instance.

#### `RunCommand(ctx, command []string) (*CommandOutput, error)`
Executes a command in the sandbox.

#### `ReadFile(ctx, path string) (string, error)`
Reads file content from the sandbox.

#### `WriteFile(ctx, path string, content string) error`
Writes content to a file in the sandbox.

#### `Exists(ctx, path string) (bool, error)`
Checks if a path exists.

#### `IsDirectory(ctx, path string) (bool, error)`
Checks if a path is a directory.

#### `Close(ctx) error`
Releases sandbox resources.

#### `SetWorkDir(dir string)`
Updates the working directory.

#### `GetSessionID() string`
Returns the current shell session ID.

## Comparison with DockerSandbox

| Feature | AIOSandbox | DockerSandbox |
|---------|------------|---------------|
| Dependency | Remote API | Local Docker |
| Setup | API credentials | Docker installed |
| Isolation | Cloud sandbox | Container |
| Session | API-managed | Container lifecycle |
| Resource limits | Server-side | Local config |
| Network access | Server-controlled | Local config |

## Testing

Run unit tests:
```bash
go test ./...
```

Run integration tests (requires AIO Sandbox credentials):
```bash
export AIO_SANDBOX_BASE_URL=https://xxxx.apigateway-cn-beijing.volceapi.com
go test -v ./...
```

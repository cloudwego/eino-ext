# Local Backend

A filesystem backend for EINO ADK that operates directly on the local machine's filesystem using standard Go packages.

## Quick Start

### Installation

```bash
go get github.com/cloudwego/eino-ext/adk/backend/local
```

### Basic Usage

```go
import (
    "context"
    "github.com/cloudwego/eino-ext/adk/backend/local"
    "github.com/cloudwego/eino/adk/middlewares/filesystem"
)

// Initialize backend
backend, err := local.NewLocalBackend(context.Background(), &local.Config{})
if err != nil {
    panic(err)
}

// Write a file
err = backend.Write(ctx, &filesystem.WriteRequest{
    FilePath: "/path/to/file.txt",
    Content:  "Hello, World!",
})

// Read a file
content, err := backend.Read(ctx, &filesystem.ReadRequest{
    FilePath: "/path/to/file.txt",
})
```

## Features

- **Zero Configuration** - Works out of the box with no setup required
- **Direct Filesystem Access** - Operates on local files with native performance
- **Full Backend Implementation** - Supports all `filesystem.Backend` operations
- **Path Security** - Enforces absolute paths to prevent directory traversal
- **Safe Write** - Prevents accidental file overwrites by default

## Configuration

```go
type Config struct {
    // Optional: Command validator for Execute() method security
    // Recommended for production use to prevent command injection
    ValidateCommand func(string) error
}
```

### Command Validation Example

```go
func validateCommand(cmd string) error {
    allowed := map[string]bool{"ls": true, "cat": true, "grep": true}
    parts := strings.Fields(cmd)
    if len(parts) == 0 || !allowed[parts[0]] {
        return fmt.Errorf("command not allowed: %s", cmd)
    }
    return nil
}

backend, _ := local.NewLocalBackend(ctx, &local.Config{
    ValidateCommand: validateCommand,
})
```

## Examples

### Example 1: File Operations

Complete example demonstrating write, read, edit, and list operations.

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "path/filepath"
    
    "github.com/cloudwego/eino-ext/adk/backend/local"
    "github.com/cloudwego/eino/adk/middlewares/filesystem"
)

func main() {
    ctx := context.Background()
    
    // Create temporary directory
    tempDir, _ := os.MkdirTemp("", "example-*")
    defer os.RemoveAll(tempDir)
    
    // Initialize backend
    backend, err := local.NewLocalBackend(ctx, &local.Config{})
    if err != nil {
        log.Fatal(err)
    }
    
    filePath := filepath.Join(tempDir, "test.txt")
    
    // Write file
    backend.Write(ctx, &filesystem.WriteRequest{
        FilePath: filePath,
        Content:  "Hello, Local Backend!",
    })
    
    // Read file
    content, _ := backend.Read(ctx, &filesystem.ReadRequest{
        FilePath: filePath,
    })
    fmt.Println("Content:", content)
    
    // Edit file
    backend.Edit(ctx, &filesystem.EditRequest{
        FilePath:  filePath,
        OldString: "Hello",
        NewString: "Hi",
    })
    
    // List directory
    files, _ := backend.LsInfo(ctx, &filesystem.LsInfoRequest{
        Path: tempDir,
    })
    fmt.Printf("Found %d files\n", len(files))
}
```

**Output:**
```
Content:      1	Hello, Local Backend!
Found 1 files
```

### Example 2: Search Operations

Search file content and find files by pattern.

```go
// Search for pattern in files
matches, _ := backend.GrepRaw(ctx, &filesystem.GrepRequest{
    Path:    "/path/to/directory",
    Pattern: "search-term",
    Glob:    "*.txt",  // Only search .txt files
})

for _, match := range matches {
    fmt.Printf("%s:%d - %s\n", match.Path, match.Line, match.Content)
}

// Find files by glob pattern
files, _ := backend.GlobInfo(ctx, &filesystem.GlobInfoRequest{
    Path:    "/path/to/directory",
    Pattern: "**/*.go",  // Recursive search for .go files
})
```

**Output:**
```
/path/to/file.txt:5 - This line contains search-term
/path/to/another.txt:12 - Another match for search-term
```

### Example 3: Agent Integration

Integrate with EINO Agent for AI-powered filesystem operations.

```go
import (
    "github.com/cloudwego/eino/adk"
    "github.com/cloudwego/eino/components/model/openai"
)

// Create backend
backend, _ := local.NewLocalBackend(ctx, &local.Config{})

// Create middleware
middleware, _ := filesystem.NewMiddleware(ctx, &filesystem.Config{
    Backend: backend,
})

// Create chat model
chatModel, _ := openai.NewChatModel(ctx, &openai.ChatModelConfig{
    APIKey: os.Getenv("OPENAI_API_KEY"),
    Model:  "gpt-4",
})

// Create agent
agent, _ := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
    Name:        "FileSystemAgent",
    Description: "AI agent for filesystem operations",
    Model:       chatModel,
    Middlewares: []adk.AgentMiddleware{middleware},
})

// Run agent
input := &adk.AgentInput{
    Messages: []*schema.Message{
        schema.UserMessage("List all .go files in the current directory"),
    },
}

for event := range agent.Run(ctx, input) {
    // Process agent events
}
```

## API Reference

### Core Methods

- **`LsInfo(ctx, req)`** - List directory contents
- **`Read(ctx, req)`** - Read file with optional line offset/limit
- **`Write(ctx, req)`** - Create new file (fails if exists)
- **`Edit(ctx, req)`** - Search and replace in file
- **`GrepRaw(ctx, req)`** - Search pattern in files
- **`GlobInfo(ctx, req)`** - Find files by glob pattern

### Additional Methods

- **`Execute(ctx, req)`** - Execute shell command (requires validation)
- **`ExecuteStreaming(ctx, req)`** - Execute with streaming output

**Note:** All paths must be absolute. Use `filepath.Abs()` to convert relative paths.

## Security

### Best Practices

- ✅ Always validate user input before file operations
- ✅ Use absolute paths to prevent directory traversal
- ✅ Implement `ValidateCommand` for command execution
- ✅ Run with minimal necessary permissions
- ✅ Monitor filesystem operations in production

### Command Injection Prevention

The `Execute()` method requires command validation:

```go
// Bad: No validation
backend, _ := local.NewLocalBackend(ctx, &local.Config{})
// Command injection risk!

// Good: With validation
backend, _ := local.NewLocalBackend(ctx, &local.Config{
    ValidateCommand: myValidator,
})
```

## FAQ

**Q: Why do all paths need to be absolute?**  
A: This prevents directory traversal attacks. Use `filepath.Abs()` to convert relative paths.

**Q: Why does Write fail if the file exists?**  
A: This is a safety feature to prevent accidental data loss. Use `Edit()` to modify existing files.

**Q: Can I use this in production?**  
A: Yes, but ensure proper input validation, command validation, and appropriate permissions.


## License

Licensed under the Apache License, Version 2.0. See the [LICENSE](../../LICENSE) file for details.

# Ark Sandbox Backend

A secure filesystem backend for EINO ADK that executes operations in Volcengine's isolated Ark Sandbox environment.

## Quick Start

### Installation

```bash
go get github.com/cloudwego/eino-ext/adk/backend/arksandbox
```

### Basic Usage

```go
import (
    "context"
    "os"
    "time"
    
    "github.com/cloudwego/eino-ext/adk/backend/arksandbox"
    "github.com/cloudwego/eino/adk/middlewares/filesystem"
)

// Configure with credentials
config := &arksandbox.Config{
    AccessKeyID:     os.Getenv("VOLC_ACCESS_KEY_ID"),
    SecretAccessKey: os.Getenv("VOLC_SECRET_ACCESS_KEY"),
    ToolID:          os.Getenv("VOLC_TOOL_ID"),
    UserSessionID:   "session-" + time.Now().Format("20060102-150405"),
    Region:          arksandbox.RegionOfBeijing,
}

// Initialize backend
backend, err := arksandbox.NewArkSandboxBackend(config)
if err != nil {
    panic(err)
}

// Use the backend
backend.Write(ctx, &filesystem.WriteRequest{
    FilePath: "/home/gem/file.txt",
    Content:  "Hello, Sandbox!",
})
```

## Features

- **Secure Execution** - All operations run in isolated sandbox environment
- **Session Management** - Supports session isolation with configurable TTL
- **Full Backend Implementation** - Supports all `filesystem.Backend` operations
- **Request Signing** - Automatic AK/SK authentication with Volcengine
- **Remote Execution** - Python-based sandbox operations with result streaming

## Configuration

```go
type Config struct {
    // Required: Volcengine credentials
    AccessKeyID     string
    SecretAccessKey string
    ToolID          string  // Sandbox tool ID from Volcengine console
    UserSessionID   string  // Unique session ID for isolation
    
    // Optional: Defaults provided
    Region        Region        // Default: RegionOfBeijing
    SessionTTL    int          // Default: 1800 seconds (30 min)
    ExecutionTimeout int       
    Timeout       time.Duration // HTTP client timeout
}
```

### Environment Setup

```bash
# Set credentials as environment variables
export VOLC_ACCESS_KEY_ID="your_access_key"
export VOLC_SECRET_ACCESS_KEY="your_secret_key"
export VOLC_TOOL_ID="your_tool_id"
```

**Get Credentials:**
1. Log in to [Volcengine Console](https://console.volcengine.com/)
2. Navigate to IAM → Access Keys
3. Create Ark Sandbox tool in Ark Platform
4. Copy credentials and tool ID

## Examples

### Example 1: File Operations

Basic file operations in the sandbox environment.

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "time"
    
    "github.com/cloudwego/eino-ext/adk/backend/arksandbox"
    "github.com/cloudwego/eino/adk/middlewares/filesystem"
)

func main() {
    ctx := context.Background()
    
    // Initialize backend
    backend, err := arksandbox.NewArkSandboxBackend(&arksandbox.Config{
        AccessKeyID:     os.Getenv("VOLC_ACCESS_KEY_ID"),
        SecretAccessKey: os.Getenv("VOLC_SECRET_ACCESS_KEY"),
        ToolID:          os.Getenv("VOLC_TOOL_ID"),
        UserSessionID:   "example-" + time.Now().Format("20060102-150405"),
        Region:          arksandbox.RegionOfBeijing,
    })
    if err != nil {
        log.Fatal(err)
    }
    
    // Write file
    backend.Write(ctx, &filesystem.WriteRequest{
        FilePath: "/home/gem/test.txt",
        Content:  "Hello from Ark Sandbox!",
    })
    
    // Read file
    content, _ := backend.Read(ctx, &filesystem.ReadRequest{
        FilePath: "/home/gem/test.txt",
    })
    fmt.Println("Content:", content)
    
    // List directory
    files, _ := backend.LsInfo(ctx, &filesystem.LsInfoRequest{
        Path: "/home/gem",
    })
    fmt.Printf("Found %d files\n", len(files))
}
```

**Output:**
```
Content:      1	Hello from Ark Sandbox!
Found 1 files
```

### Example 2: Search Operations

Search and pattern matching in sandbox files.

```go
// Search for pattern
matches, _ := backend.GrepRaw(ctx, &filesystem.GrepRequest{
    Path:    "/home/gem",
    Pattern: "Sandbox",
    Glob:    "*.txt",
})

for _, match := range matches {
    fmt.Printf("%s:%d - %s\n", match.Path, match.Line, match.Content)
}

// Find files by pattern
files, _ := backend.GlobInfo(ctx, &filesystem.GlobInfoRequest{
    Path:    "/home/gem",
    Pattern: "**/*.txt",
})

fmt.Printf("Found %d .txt files\n", len(files))
```

**Output:**
```
/home/gem/test.txt:1 - Hello from Ark Sandbox!
Found 1 .txt files
```

### Example 3: Agent Integration

Build AI-powered filesystem assistant with sandbox backend.

```go
import (
    "github.com/cloudwego/eino/adk"
    "github.com/cloudwego/eino/components/model/openai"
)

// Create backend
backend, _ := arksandbox.NewArkSandboxBackend(config)

// Create middleware
middleware, _ := filesystem.NewMiddleware(ctx, &filesystem.Config{
    Backend: backend,
})

// Create agent with ChatModel
chatModel, _ := openai.NewChatModel(ctx, &openai.ChatModelConfig{
    APIKey: os.Getenv("OPENAI_API_KEY"),
    Model:  "gpt-4",
})

agent, _ := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
    Name:        "SandboxAgent",
    Description: "AI agent for secure filesystem operations",
    Model:       chatModel,
    Middlewares: []adk.AgentMiddleware{middleware},
})

// Execute user query
input := &adk.AgentInput{
    Messages: []*schema.Message{
        schema.UserMessage("List all files in /home/gem and show their details"),
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

**Note:** Use `/home/gem` directory for file operations. The default `gem` user has limited permissions on system paths.

## Security

### Best Practices

- ✅ Store credentials in environment variables, never in code
- ✅ Use unique session IDs for each execution context
- ✅ Set appropriate timeouts to prevent resource exhaustion
- ✅ Monitor sandbox resource usage in production
- ✅ Implement proper error handling and retry logic

### Session Isolation

```go
// Each user/context should have unique session ID
config := &arksandbox.Config{
    UserSessionID: fmt.Sprintf("user-%s-%d", userID, time.Now().Unix()),
    SessionTTL:    3600,  // 1 hour
}
```

## Troubleshooting

**File Already Exists**
- `Write()` fails if file exists (safety feature)
- Delete file first or use unique filenames

**Authentication Errors**
- Verify credentials are correct
- Check environment variables are set
- Ensure Volcengine account has Ark Sandbox access

**Timeout Errors**
- Increase `Timeout` or `ExecutionTimeout` in config
- Check network connectivity
- Verify Ark Sandbox service availability

## FAQ

**Q: What's the difference from Local backend?**  
A: Ark Sandbox executes in isolated remote environment (secure, sandboxed). Local backend accesses local filesystem directly (fast, simple).

**Q: Can I use this in production?**  
A: Yes. Ensure proper error handling, timeouts, unique session IDs, and resource monitoring.

**Q: What are the rate limits?**  
A: Limits depend on your Volcengine account tier. Check Volcengine documentation for details.

**Q: How long do sessions last?**  
A: Default is 30 minutes (1800 seconds). Configure with `SessionTTL` (range: 60-86400 seconds).

## License

Licensed under the Apache License, Version 2.0. See the [LICENSE](../../LICENSE) file for details.

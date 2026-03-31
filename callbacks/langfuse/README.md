# Langfuse Callbacks

English | [简体中文](README_zh.md)

A Langfuse callback implementation for [Eino](https://github.com/cloudwego/eino) that implements the `Handler` interface. This enables seamless integration with Eino's application for enhanced observability and tracing.

## Features

- Implements `github.com/cloudwego/eino/callbacks.Handler` interface
- Full support for Langfuse trace, span, and generation tracking
- Automatic handling of streaming inputs and outputs
- Flexible trace configuration with session, user, and metadata support
- Built-in error handling and recovery
- Configurable batching, sampling, and retry mechanisms
- Easy integration with Eino's application

## Installation

```bash
go get github.com/cloudwego/eino-ext/callbacks/langfuse
```

## Quick Start

```go
package main

import (
	"context"
	"log"

	"github.com/cloudwego/eino-ext/callbacks/langfuse"
	"github.com/cloudwego/eino/callbacks"
)

func main() {
	ctx := context.Background()
	
	cbh, flusher := langfuse.NewLangfuseHandler(&langfuse.Config{
		Host:        "https://cloud.langfuse.com",
		PublicKey:   "pk-lf-...",
		SecretKey:   "sk-lf-...",
		ServiceName: "eino-app",
		Release:     "v1.0.0",
	})
	
	callbacks.AppendGlobalHandlers(cbh)
	
	g := NewGraph[string, string]()
	runner, _ := g.Compile(ctx)
	
	ctx = langfuse.SetTrace(ctx, 
		langfuse.WithSessionID("session-123"), 
		langfuse.WithUserID("user-456"),
	)
	
	result, _ := runner.Invoke(ctx, "input")
	
	flusher()
}
```

## Configuration

The callback can be configured using the `Config` struct:

```go
type Config struct {
    // Host is the Langfuse server URL (Required)
    // Example: "https://cloud.langfuse.com"
    Host string
    
    // PublicKey is the public key for authentication (Required)
    // Example: "pk-lf-..."
    PublicKey string
    
    // SecretKey is the secret key for authentication (Required)
    // Example: "sk-lf-..."
    SecretKey string
    
    // Threads is the number of concurrent workers (Optional)
    // Default: 1
    Threads int
    
    // Timeout is the HTTP request timeout (Optional)
    // Default: no timeout
    Timeout time.Duration
    
    // MaxTaskQueueSize is the max number of events to buffer (Optional)
    // Default: 100
    MaxTaskQueueSize int
    
    // FlushAt is the number of events to batch before sending (Optional)
    // Default: 15
    FlushAt int
    
    // FlushInterval is how often to flush events automatically (Optional)
    // Default: 500ms
    FlushInterval time.Duration
    
    // SampleRate is the percentage of events to send (Optional)
    // Default: 1.0 (100%)
    SampleRate float64
    
    // LogMessage is the message prefix for logs (Optional)
    LogMessage string
    
    // MaskFunc is a function to mask sensitive data (Optional)
    MaskFunc func(string) string
    
    // MaxRetry is the maximum number of retry attempts (Optional)
    // Default: 3
    MaxRetry uint64
    
    // Name is the default trace name (Optional)
    Name string
    
    // UserID is the default user identifier (Optional)
    UserID string
    
    // SessionID is the default session identifier (Optional)
    SessionID string
    
    // Release is the version identifier (Optional)
    Release string
    
    // Tags are labels attached to traces (Optional)
    Tags []string
    
    // Public determines if traces are publicly accessible (Optional)
    Public bool
}
```

## Trace Options

You can customize individual traces using the `SetTrace` function:

```go
ctx = langfuse.SetTrace(ctx,
    langfuse.WithID("trace-id"),
    langfuse.WithName("custom-trace"),
    langfuse.WithUserID("user-123"),
    langfuse.WithSessionID("session-456"),
    langfuse.WithTags("production", "feature-x"),
    langfuse.WithMetadata(map[string]string{"key": "value"}),
    langfuse.WithEnvironment("production"),
    langfuse.WithVersion("v1.0.0"),
    langfuse.WithPublic(true),
)
```

## For More Details

- [Langfuse Documentation](https://langfuse.com/docs)
- [Eino Documentation](https://www.cloudwego.io/zh/docs/eino/)

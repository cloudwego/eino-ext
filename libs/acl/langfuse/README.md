# Langfuse ACL (API Client Library)

English | [简体中文](README_zh.md)

## Introduction

This is a low-level Langfuse API client library for Go. It provides direct access to the Langfuse API for creating traces, spans, generations, and events. This library is used internally by the higher-level `callbacks/langfuse` package.

**Note**: For most use cases with Eino, you should use the [callbacks/langfuse](../../callbacks/langfuse) package instead, which provides a simpler interface integrated with Eino's callback system.

## Features

- Direct Langfuse API client implementation
- Support for traces, spans, generations, and events
- Automatic batching and queueing of events
- Configurable flush intervals and batch sizes
- Retry logic for failed API calls
- Sampling rate control for event filtering
- Thread-safe operations

## Installation

```bash
go get github.com/cloudwego/eino-ext/libs/acl/langfuse
```

## Quick Start

```go
package main

import (
	"time"

	"github.com/cloudwego/eino-ext/libs/acl/langfuse"
)

func main() {
	client := langfuse.NewLangfuse(
		"https://cloud.langfuse.com",
		"pk-lf-...",
		"sk-lf-...",
		langfuse.WithThreads(5),
		langfuse.WithFlushInterval(10*time.Second),
	)

	traceID, _ := client.CreateTrace(&langfuse.TraceEventBody{
		BaseEventBody: langfuse.BaseEventBody{
			Name: "my-trace",
		},
		TimeStamp: time.Now(),
	})

	spanID, _ := client.CreateSpan(&langfuse.SpanEventBody{
		BaseObservationEventBody: langfuse.BaseObservationEventBody{
			BaseEventBody: langfuse.BaseEventBody{
				Name: "my-span",
			},
			TraceID:   traceID,
			StartTime: time.Now(),
		},
	})

	_ = client.EndSpan(&langfuse.SpanEventBody{
		BaseObservationEventBody: langfuse.BaseObservationEventBody{
			BaseEventBody: langfuse.BaseEventBody{
				ID: spanID,
			},
		},
		EndTime: time.Now(),
	})

	client.Flush()
}
```

## Configuration Options

The client can be configured using the following options:

```go
// WithThreads sets the number of concurrent worker threads
// Default: 1
langfuse.WithThreads(5)

// WithTimeout sets the HTTP request timeout
// Default: no timeout
langfuse.WithTimeout(30 * time.Second)

// WithMaxTaskQueueSize sets the maximum number of events to buffer
// Default: 100
langfuse.WithMaxTaskQueueSize(1000)

// WithFlushAt sets the number of events to batch before sending
// Default: 15
langfuse.WithFlushAt(50)

// WithFlushInterval sets how often to flush events automatically
// Default: 500ms
langfuse.WithFlushInterval(10 * time.Second)

// WithSampleRate sets the percentage of events to send (0.0-1.0)
// Default: 1.0 (100%)
langfuse.WithSampleRate(0.5)

// WithLogMessage sets the prefix for log messages
langfuse.WithLogMessage("langfuse:")

// WithMaskFunc sets a function to mask sensitive data
langfuse.WithMaskFunc(func(s string) string {
    return strings.ReplaceAll(s, "secret", "***")
})

// WithMaxRetry sets the maximum number of retry attempts
// Default: 3
langfuse.WithMaxRetry(5)
```

## API Methods

### CreateTrace

Creates a new trace:

```go
traceID, err := client.CreateTrace(&langfuse.TraceEventBody{
    BaseEventBody: langfuse.BaseEventBody{
        ID:   "custom-trace-id", // Optional, auto-generated if empty
        Name: "my-trace",
    },
    TimeStamp: time.Now(),
    UserID:    "user-123",
    SessionID: "session-456",
})
```

### CreateSpan

Creates a new span within a trace:

```go
spanID, err := client.CreateSpan(&langfuse.SpanEventBody{
    BaseObservationEventBody: langfuse.BaseObservationEventBody{
        BaseEventBody: langfuse.BaseEventBody{
            Name: "my-span",
        },
        TraceID:   traceID,
        StartTime: time.Now(),
        Input:     "span input data",
    },
})
```

### EndSpan

Completes a span:

```go
err := client.EndSpan(&langfuse.SpanEventBody{
    BaseObservationEventBody: langfuse.BaseObservationEventBody{
        BaseEventBody: langfuse.BaseEventBody{
            ID: spanID,
        },
        Output: "span output data",
    },
    EndTime: time.Now(),
})
```

### CreateGeneration

Creates a generation (LLM call):

```go
generationID, err := client.CreateGeneration(&langfuse.GenerationEventBody{
    BaseObservationEventBody: langfuse.BaseObservationEventBody{
        BaseEventBody: langfuse.BaseEventBody{
            Name: "llm-call",
        },
        TraceID:   traceID,
        StartTime: time.Now(),
    },
    Model:      "gpt-4",
    InMessages: messages,
})
```

### EndGeneration

Completes a generation:

```go
err := client.EndGeneration(&langfuse.GenerationEventBody{
    BaseObservationEventBody: langfuse.BaseObservationEventBody{
        BaseEventBody: langfuse.BaseEventBody{
            ID: generationID,
        },
    },
    OutMessage: responseMessage,
    EndTime:    time.Now(),
    Usage: &langfuse.Usage{
        PromptTokens:     10,
        CompletionTokens: 20,
        TotalTokens:      30,
    },
})
```

### CreateEvent

Creates a custom event:

```go
eventID, err := client.CreateEvent(&langfuse.EventEventBody{
    BaseObservationEventBody: langfuse.BaseObservationEventBody{
        BaseEventBody: langfuse.BaseEventBody{
            Name: "custom-event",
        },
        TraceID:   traceID,
        StartTime: time.Now(),
    },
})
```

### Flush

Manually flush all pending events:

```go
client.Flush()
```

## Important Notes

- **Automatic Batching**: Events are automatically batched and sent periodically
- **Thread Safety**: All methods are thread-safe
- **Flush on Exit**: Always call `Flush()` before exiting to ensure all events are sent
- **Error Handling**: Errors are logged but don't block the main application flow

## Use Cases

This library is typically used:

1. **Internal Implementation**: As the underlying client for higher-level packages
2. **Direct Integration**: When you need fine-grained control over Langfuse API calls
3. **Custom Solutions**: When building custom observability solutions

For most Eino integrations, use [callbacks/langfuse](../../callbacks/langfuse) instead.

## License

This project is licensed under the Apache License 2.0 - see the LICENSE file for details.

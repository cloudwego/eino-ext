# Langfuse ACL（API 客户端库）

[English](README.md) | 简体中文

## 简介

这是一个用于 Go 的低级 Langfuse API 客户端库。它提供对 Langfuse API 的直接访问，用于创建跟踪、跨度、生成和事件。此库由更高级别的 `callbacks/langfuse` 包内部使用。

**注意**：对于大多数 Eino 用例，您应该使用 [callbacks/langfuse](../../callbacks/langfuse) 包，它提供了与 Eino 回调系统集成的更简单接口。

## 特性

- 直接的 Langfuse API 客户端实现
- 支持跟踪、跨度、生成和事件
- 事件的自动批处理和排队
- 可配置的刷新间隔和批处理大小
- 失败 API 调用的重试逻辑
- 用于事件过滤的采样率控制
- 线程安全操作

## 安装

```bash
go get github.com/cloudwego/eino-ext/libs/acl/langfuse
```

## 快速开始

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

## 配置选项

客户端可以使用以下选项进行配置：

```go
// WithThreads 设置并发工作线程数
// 默认值: 1
langfuse.WithThreads(5)

// WithTimeout 设置 HTTP 请求超时
// 默认值: 无超时
langfuse.WithTimeout(30 * time.Second)

// WithMaxTaskQueueSize 设置要缓冲的最大事件数
// 默认值: 100
langfuse.WithMaxTaskQueueSize(1000)

// WithFlushAt 设置发送前批处理的事件数
// 默认值: 15
langfuse.WithFlushAt(50)

// WithFlushInterval 设置自动刷新事件的频率
// 默认值: 500ms
langfuse.WithFlushInterval(10 * time.Second)

// WithSampleRate 设置要发送的事件百分比 (0.0-1.0)
// 默认值: 1.0 (100%)
langfuse.WithSampleRate(0.5)

// WithLogMessage 设置日志消息的前缀
langfuse.WithLogMessage("langfuse:")

// WithMaskFunc 设置一个函数来屏蔽敏感数据
langfuse.WithMaskFunc(func(s string) string {
    return strings.ReplaceAll(s, "secret", "***")
})

// WithMaxRetry 设置最大重试次数
// 默认值: 3
langfuse.WithMaxRetry(5)
```

## API 方法

### CreateTrace

创建新的跟踪：

```go
traceID, err := client.CreateTrace(&langfuse.TraceEventBody{
    BaseEventBody: langfuse.BaseEventBody{
        ID:   "custom-trace-id", // 可选，如果为空则自动生成
        Name: "my-trace",
    },
    TimeStamp: time.Now(),
    UserID:    "user-123",
    SessionID: "session-456",
})
```

### CreateSpan

在跟踪中创建新的跨度：

```go
spanID, err := client.CreateSpan(&langfuse.SpanEventBody{
    BaseObservationEventBody: langfuse.BaseObservationEventBody{
        BaseEventBody: langfuse.BaseEventBody{
            Name: "my-span",
        },
        TraceID:   traceID,
        StartTime: time.Now(),
        Input:     "跨度输入数据",
    },
})
```

### EndSpan

完成跨度：

```go
err := client.EndSpan(&langfuse.SpanEventBody{
    BaseObservationEventBody: langfuse.BaseObservationEventBody{
        BaseEventBody: langfuse.BaseEventBody{
            ID: spanID,
        },
        Output: "跨度输出数据",
    },
    EndTime: time.Now(),
})
```

### CreateGeneration

创建生成（LLM 调用）：

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

完成生成：

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

创建自定义事件：

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

手动刷新所有待处理的事件：

```go
client.Flush()
```

## 重要说明

- **自动批处理**：事件会自动批处理并定期发送
- **线程安全**：所有方法都是线程安全的
- **退出时刷新**：在退出前始终调用 `Flush()` 以确保所有事件都已发送
- **错误处理**：错误会被记录但不会阻止主应用程序流程

## 用例

此库通常用于：

1. **内部实现**：作为更高级别包的底层客户端
2. **直接集成**：当您需要对 Langfuse API 调用进行细粒度控制时
3. **自定义解决方案**：构建自定义可观测性解决方案时

对于大多数 Eino 集成，请改用 [callbacks/langfuse](../../callbacks/langfuse)。

## 许可证

本项目采用 Apache License 2.0 许可证 - 详见 LICENSE 文件。

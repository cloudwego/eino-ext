# Langfuse 回调

[English](README.md) | 简体中文

这是一个为 [Eino](https://github.com/cloudwego/eino) 实现的 Langfuse 回调。该工具实现了 `Handler` 接口，可以与 Eino 的应用无缝集成以提供增强的可观测和追踪能力。

## 特性

- 实现了 `github.com/cloudwego/eino/callbacks.Handler` 接口
- 全面支持 Langfuse 的 trace、span 和 generation 追踪
- 自动处理流式输入和输出
- 灵活的追踪配置，支持会话、用户和元数据
- 内置错误处理和恢复机制
- 可配置的批处理、采样和重试机制
- 易于与 Eino 应用集成

## 安装

```bash
go get github.com/cloudwego/eino-ext/callbacks/langfuse
```

## 快速开始

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

## 配置

回调可以通过 `Config` 结构体进行配置：

```go
type Config struct {
    // Langfuse 服务器地址 (必填)
    // 例子: "https://cloud.langfuse.com"
    Host string
    
    // 公钥，用于认证 (必填)
    // 例子: "pk-lf-..."
    PublicKey string
    
    // 私钥，用于认证 (必填)
    // 例子: "sk-lf-..."
    SecretKey string
    
    // 并发工作线程数 (选填)
    // 默认值: 1
    Threads int
    
    // HTTP 请求超时时间 (选填)
    // 默认值: 无超时
    Timeout time.Duration
    
    // 事件缓冲区最大大小 (选填)
    // 默认值: 100
    MaxTaskQueueSize int
    
    // 批量发送前的事件数量 (选填)
    // 默认值: 15
    FlushAt int
    
    // 自动刷新事件的时间间隔 (选填)
    // 默认值: 500ms
    FlushInterval time.Duration
    
    // 事件采样率 (选填)
    // 默认值: 1.0 (100%)
    SampleRate float64
    
    // 日志消息前缀 (选填)
    LogMessage string
    
    // 敏感数据脱敏函数 (选填)
    MaskFunc func(string) string
    
    // 最大重试次数 (选填)
    // 默认值: 3
    MaxRetry uint64
    
    // 默认追踪名称 (选填)
    Name string
    
    // 默认用户标识 (选填)
    UserID string
    
    // 默认会话标识 (选填)
    SessionID string
    
    // 版本标识 (选填)
    Release string
    
    // 追踪标签 (选填)
    Tags []string
    
    // 是否公开可访问 (选填)
    Public bool
}
```

## 追踪选项

您可以使用 `SetTrace` 函数自定义单个追踪：

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

## 更多详情

- [Langfuse 文档](https://langfuse.com/docs)
- [Eino 文档](https://www.cloudwego.io/zh/docs/eino/)

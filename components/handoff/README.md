# Eino Handoff

一个用于 AI 会话间工作交接的 Eino 扩展组件。生成人类可读的交接文档，让任何 AI 代理都能无缝继续工作。

## 特性

- 🤖 **AI 友好**：YAML frontmatter + Markdown body，任何 AI 都能理解
- 📊 **智能摘要**：自动提取关键决策、代码状态、下一步行动
- 🔌 **非侵入集成**：通过 callback 机制集成，不修改现有代码
- 🎨 **可扩展**：支持自定义摘要器、代码追踪器、格式化器
- 📝 **人类可读**：Markdown 格式，便于人工审查和编辑

## 安装

```bash
go get github.com/bytedance/eino-ext/components/handoff
```

## 快速开始

### 方式一：使用 Handler（底层 API）

```go
package main

import (
    "context"
    "os"

    "github.com/bytedance/eino/adk"
    "github.com/bytedance/eino-ext/components/handoff"
)

func main() {
    ctx := context.Background()

    // 创建 handoff handler
    handler := handoff.NewHandler(&handoff.HandlerConfig{
        SessionID: "my-session",
    })

    // 创建 agent
    agent := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
        Model: model,
        Tools: tools,
    })

    // 运行 agent 时使用 handoff callback
    input := &adk.AgentInput{Input: "帮我实现用户认证"}
    events := agent.Run(ctx, input, adk.WithCallbacks(handler))

    // 处理事件...

    // 生成 handoff 文档
    doc, err := handler.Generate(ctx, &handoff.GenerateOptions{
        TaskTitle:    "实现用户认证",
        TaskStatus:   handoff.TaskStatusInProgress,
        TaskProgress: 60,
    })
    if err != nil {
        panic(err)
    }

    // 保存文档
    doc.Save("handoff.md")
}
```

### 方式二：使用 Wrapper（快速集成）

```go
// 包装现有 agent
wrapped, err := handoff.Wrap(agent, &handoff.WrapConfig{
    SessionID:  "my-session",
    OutputPath: "./handoffs/",
})

// 使用方式与原有 agent 相同
events := wrapped.Run(ctx, input)

// 手动触发 handoff 生成
outputPath, err := wrapped.GenerateHandoff(ctx, &handoff.GenerateOptions{
    TaskTitle: "当前任务",
})
```

### 方式三：使用 Builder（流式 API）

```go
handler := handoff.NewHandlerBuilder().
    WithSessionID("my-session").
    WithSessionDescription("用户认证系统重构").
    WithMaxEvents(100).
    WithCodeTracker(handoff.NewDefaultCodeTracker()).
    Build()
```

## Handoff 文档格式

生成的文档格式：

```yaml
---
handoff_version: "1.0"
session:
  id: "sess_abc123"
  started_at: "2024-01-15T09:00:00Z"
  handoff_at: "2024-01-15T10:30:00Z"
current_task:
  title: "实现 JWT 登录"
  status: "in_progress"
  progress: 45
stats:
  total_messages: 24
  tool_calls: 8
---

# 任务摘要

正在将用户认证系统从 session 迁移到 JWT...

## 关键决策

| 时间 | 决策 | 理由 | 状态 |
|------|------|------|------|
| 09:35 | 使用 RS256 | 更安全 | ✅ 已实施 |

## 代码状态

### 工作文件
- `src/service/auth.go` (第 45 行) 📝

## 下一步

1. **高**: 完成密码验证
2. **中**: 编写单元测试

## 待解决问题

- [ ] 旧版 session 如何迁移？
```

## 高级用法

### 手动标记决策

```go
handler.MarkDecision(handoff.Decision{
    Title:     "使用 bcrypt 加密",
    Reasoning: "比 md5 更安全，行业推荐",
    Status:    "decided",
    Time:      time.Now(),
})
```

### 手动标记里程碑

```go
handler.MarkMilestone(handoff.Milestone{
    Title:       "完成接口设计",
    Description: "定义了所有公共接口",
    CompletedAt: time.Now(),
})
```

### 加载和解析 Handoff 文档

```go
loader := handoff.NewLoader()
doc, err := loader.Load("handoff.md")

// 获取摘要
summary := loader.ExtractSummary(doc)

// 获取高优先级任务
steps := loader.GetNextSteps(doc, handoff.PriorityHigh)

// 检查是否有阻塞问题
if doc.HasBlockingIssues() {
    blockers := loader.GetBlockingIssues(doc)
    // 处理阻塞问题...
}
```

### 自定义代码追踪器

```go
// 使用 Git 对比特定 commit
tracker := handoff.NewGitDiffCodeTracker("HEAD~5")

// 或使用静态文件列表
tracker := &handoff.StaticCodeTracker{
    Files: []handoff.WorkingFile{
        {Path: "main.go", Status: "editing"},
    },
}

// 然后传给 handler
handler := handoff.NewHandler(&handoff.HandlerConfig{
    CodeTracker: tracker,
})
```

### 使用 LLM 精炼摘要

```go
summarizer := handoff.NewDefaultSummarizer()
summarizer.UseLLM = true
summarizer.LLM = myLLMClient  // 实现 handoff.LLMClient 接口

handler := handoff.NewHandler(&handoff.HandlerConfig{
    Summarizer: summarizer,
})
```

## 架构

```
┌─────────────────────────────────────────────────────┐
│                     Handoff                         │
├─────────────────────────────────────────────────────┤
│  Handler → Collector → Summarizer → Formatter       │
│      ↓           ↓          ↓           ↓           │
│  Callback    Events    Content    YAML+Markdown     │
└─────────────────────────────────────────────────────┘
```

## API 层级

| 层级 | API | 适用场景 |
|------|-----|----------|
| Level 3 | CLI | 终端用户直接使用 |
| Level 2 | Wrapper | 快速集成到现有 agent |
| Level 1 | Handler | 完全控制，自定义逻辑 |

## 设计文档

详细设计文档请参见 [DESIGN.md](./DESIGN.md)。

## 许可证

Apache License 2.0

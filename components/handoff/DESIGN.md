# Eino Handoff 设计文档

> **让 AI 会话可交接、可延续、可协作**

---

## 目录

1. [为什么要做 Handoff](#1-为什么要做-handoff)
   - 1.1 [我们面临的真实问题](#11-我们面临的真实问题)
   - 1.2 [现有方案为什么不够](#12-现有方案为什么不够)
   - 1.3 [核心价值主张](#13-核心价值主张)
2. [设计哲学](#2-设计哲学)
   - 2.1 [传递意图，而非状态](#21-传递意图而非状态)
   - 2.2 [人类可读是第一优先级](#22-人类可读是第一优先级)
   - 2.3 [框架集成，而非工具绑定](#23-框架集成而非工具绑定)
3. [架构设计](#3-架构设计)
   - 3.1 [整体架构](#31-整体架构)
   - 3.2 [核心组件详解](#32-核心组件详解)
   - 3.3 [数据流](#33-数据流)
4. [详细设计](#4-详细设计)
   - 4.1 [Document Schema](#41-document-schema)
   - 4.2 [事件收集机制](#42-事件收集机制)
   - 4.3 [智能摘要算法](#43-智能摘要算法)
5. [使用指南](#5-使用指南)
   - 5.1 [三种使用模式](#51-三种使用模式)
   - 5.2 [完整示例](#52-完整示例)
   - 5.3 [最佳实践](#53-最佳实践)
6. [实际案例分析](#6-实际案例分析)
   - 6.1 [个人开发跨会话](#61-个人开发跨会话)
   - 6.2 [团队协作](#62-团队协作)
   - 6.3 [工具切换](#63-工具切换)
7. [与现有方案对比](#7-与现有方案对比)
8. [未来演进](#8-未来演进)

---

## 1. 为什么要做 Handoff

### 1.1 我们面临的真实问题

#### 问题一：上下文窗口的"硬天花板"

现代 AI 模型虽然上下文窗口越来越大（32K、100K、200K tokens），但**有效上下文**却是另一回事：

```
实际观察到的现象：
- 超过 8K tokens 后，模型开始"遗忘"早期的细节
- 超过 20K tokens 后，对复杂依赖关系的理解明显下降
- 代码生成任务中，超过 1 小时的连续会话，错误率显著上升
```

**真实场景**：
> 小明正在用 Claude 重构一个微服务架构。他已经和 AI 交互了 45 分钟，完成了接口设计和部分实现。此时，他需要去开会。会后继续时，发现：
> - 早期关于"为什么用 gRPC 而非 REST"的讨论被埋在历史消息中
> - AI 开始建议一些与之前决策冲突的方案
> - 为了恢复上下文，他不得不花费 10 分钟重新解释背景

#### 问题二：工具锁定的隐性成本

AI 辅助开发工具正在快速演进：

| 工具 | 优势 | 劣势 |
|------|------|------|
| Claude | 推理能力强，代码质量高 | 无原生 IDE 集成 |
| Cursor | IDE 集成好，响应快 | 复杂逻辑处理较弱 |
| GitHub Copilot | 补全能力强 | 缺乏深度交互 |
| Gemini | 上下文窗口大 | 代码能力参差不齐 |

**真实场景**：
> 小红在 Cursor 中完成了项目的框架搭建，但遇到一个复杂的算法问题，想切换到 Claude 寻求帮助。她需要：
> 1. 手动整理当前代码结构和关键决策
> 2. 在 Claude 中重新描述项目背景
> 3. 解释已经在 Cursor 中做的设计选择
> 这个过程耗时 15-20 分钟，而且总会遗漏一些细节。

#### 问题三：团队协作的"黑盒"问题

传统软件开发中，知识的传递通过：
- 代码注释
- 设计文档
- 代码审查
- 站会同步

但在 AI 辅助开发中，大量关键信息存在于**会话历史**中：
- 为什么选择了方案 A 而非方案 B？
- 尝试过哪些方案并被否决？
- 当前代码的已知问题和限制是什么？

这些信息**不会自动沉淀**到代码或文档中。

**真实场景**：
> 团队中的高级工程师用 AI 完成了一个模块的原型，然后交给初级工程师完善。原型代码看起来完整，但：
> - 缺少错误处理的边界情况（因为原型阶段和 AI 约定"先不管错误处理"）
> - 使用了一个临时方案，计划后续替换（但没有记录）
> - 有一些隐含的架构假设（如"假定用户服务总是可用"）
>
> 初级工程师在完善过程中引入了 bug，因为他不知道这些隐含假设。

#### 问题四：长周期项目的"记忆断层"

复杂项目往往需要多天的持续开发：

```
Day 1: 需求分析 + 架构设计（2 小时 AI 会话）
Day 2: 核心模块实现（3 小时 AI 会话）
Day 3: 集成测试 + 优化（2 小时 AI 会话）
```

每一天都是一个新的会话，昨天讨论的关键决策、妥协方案、技术债务，今天都需要重新唤起。

### 1.2 现有方案为什么不够

#### 方案一：人工写交接文档

**做法**：会话结束时，手动写一个 Markdown 文档总结。

**问题**：
```
1. 负担重：需要花 10-15 分钟整理
2. 不完整：人会选择性记忆，遗漏关键细节
3. 不一致：不同人写的格式、详细程度差异大
4. 不及时：经常忘记写，或者过了几天才补
```

#### 方案二：Eino Checkpoint/Resume

**做法**：使用 eino 的 `checkpoint` 和 `resume` 机制保存状态。

**问题**：
```
1. 二进制格式：gob 序列化，人类不可读
2. 版本敏感：代码变更后 checkpoint 可能失效
3. 工具绑定：只能在相同框架版本内使用
4. 黑盒问题：不知道 checkpoint 里有什么，不敢依赖
```

**适用场景**：程序自动化恢复，不适用于人类交接。

#### 方案三：Claude Handoff（社区工具）

**做法**：Python 脚本生成 handoff.md 模板。

**问题**：
```
1. 无框架集成：需要手动复制粘贴会话历史
2. 无智能摘要：只是模板填充，不分析内容
3. 无代码感知：不知道实际改了哪些文件
4. 静态工具：无法追踪会话过程中的事件
```

### 1.3 核心价值主张

Eino Handoff 要解决的核心问题：**让 AI 会话像代码一样可版本化、可审查、可交接**。

#### 价值一：降低上下文恢复成本

| 方式 | 恢复时间 | 信息完整性 |
|------|----------|-----------|
| 重新解释 | 10-15 分钟 | 60-70% |
| 人工文档 | 5-10 分钟 | 70-80% |
| **Handoff** | **1-2 分钟** | **90%+** |

#### 价值二：实现真正的工具自由

```
用户旅程：
1. 在 Cursor 中开始项目框架（利用 IDE 集成优势）
2. 导出 Handoff
3. 在 Claude 中解决复杂算法问题（利用推理能力优势）
4. 导出 Handoff
5. 在 Cursor 中继续完善（利用 IDE 集成优势）

关键：Handoff 是通用格式，任何 AI 工具都能理解
```

#### 价值三：沉淀团队知识

```
传统方式：
代码 + 口头交流 → 信息随时间流失

Handoff 方式：
代码 + Handoff 文档 → 可审查的决策历史

每完成一个任务，自动生成一个"决策记录"，包括：
- 为什么这样设计
- 尝试过哪些方案
- 已知的技术债务
- 下一步的计划
```

#### 价值四：支持渐进式开发

复杂项目可以分解为多个 Handoff 周期：

```
Handoff 1: 需求分析完成
  ↓
Handoff 2: 架构设计完成
  ↓
Handoff 3: 核心模块完成
  ↓
Handoff 4: 测试优化完成

每个 Handoff 都是可交付的 checkpoint
```

---

## 2. 设计哲学

### 2.1 传递意图，而非状态

**关键区分**：

```
状态（State）：
- 变量值
- 内存结构
- 执行位置
- 适合：程序自动化恢复

意图（Intent）：
- 目标是什么
- 已完成什么
- 为什么选择某方案
- 待解决什么问题
- 适合：人类理解、跨工具使用
```

**Eino Handoff 传递的是意图**。

#### 为什么意图比状态更重要

**场景对比**：

```
状态传递（Checkpoint）：
┌─────────────────────────────────────────┐
│ 当前执行位置: line 245                  │
│ 变量值: x=10, y=20                      │
│ 内存状态: 0x7ff3...                     │
└─────────────────────────────────────────┘
接手者："这是什么？我能做什么？"

意图传递（Handoff）：
┌─────────────────────────────────────────┐
│ 目标: 实现用户认证系统                   │
│ 已完成: 数据库设计、接口定义             │
│ 当前: 正在实现登录逻辑（第 2 个函数）     │
│ 决策: 使用 JWT 而非 Session             │
│ 待办: 1. 密码加密 2. 错误处理            │
└─────────────────────────────────────────┘
接手者："明白了，我可以继续实现密码加密"
```

### 2.2 人类可读是第一优先级

**设计决策**：即使牺牲一些机器效率，也要确保人类可读。

#### 格式选择：YAML + Markdown

```yaml
---
# 机器可解析的结构化数据
handoff_version: "1.0"
current_task:
  title: "实现登录功能"
  progress: 45
---

# 人类可读的详细描述

## 当前状态

正在实现 `AuthService.Login` 方法，已完成参数验证，待实现密码比对逻辑。

## 关键决策

- **使用 bcrypt**: 比 md5 更安全，Go 标准库支持
- **JWT 有效期 15 分钟**: 平衡安全性和用户体验
```

**为什么不选 JSON？**
```
JSON:
{
  "current_task": {
    "title": "实现登录功能",
    "progress": 45
  }
}

问题：
1. 不支持注释，无法添加说明
2. 对人类不友好，难以快速扫读
3. 没有标准化的多行字符串格式
```

**为什么不选纯 Markdown？**
```
纯 Markdown:
# 任务标题
实现登录功能

# 进度
45%

问题：
1. 机器解析困难（需要约定格式）
2. 类型信息丢失
3. 难以验证 Schema
```

**YAML + Markdown 的优势**：
```
1. Frontmatter: 结构化、类型安全、可验证
2. Body: 自由格式、支持任何 Markdown 特性
3. 兼容性: 任何文本编辑器都能打开
4. 版本控制: diff 友好
```

### 2.3 框架集成，而非工具绑定

**核心原则**：与 Eino 框架深度集成，但不绑定特定 AI 工具。

#### 集成层次

```
Level 1: Eino Core
  ↓ callbacks.Handler 接口
Level 2: Handoff Collector
  ↓ 事件收集、状态累积
Level 3: Document Generation
  ↓ YAML + Markdown
Level 4: 任何 AI 工具都能理解
```

**为什么这样设计？**

```
如果绑定特定工具（如只支持 Claude）：
┌────────────────────────────────────────┐
│ Claude → Handoff → Claude              │
└────────────────────────────────────────┘
局限：不能切换到其他工具

框架级设计：
┌────────────────────────────────────────┐
│ Claude ──┐                             │
│ Cursor ──┼→ Handoff → 任何 AI 工具     │
│ Copilot ─┘                             │
└────────────────────────────────────────┘
优势：真正的工具自由
```

#### 非侵入式集成

**关键设计**：通过 `callbacks.Handler` 收集事件，不修改 agent 逻辑。

```go
// 你的原有代码完全不变
agent := adk.NewChatModelAgent(...)

// 只需添加一个 callback 选项
events := agent.Run(ctx, input,
    adk.WithCallbacks(handoffHandler),  // ← 就这一行
)
```

**好处**：
```
1. 零侵入：不改任何业务逻辑
2. 零成本：不需要时直接去掉 callback
3. 可组合：和其他 callback（日志、监控）共存
```

---

## 3. 架构设计

### 3.1 整体架构

```
┌──────────────────────────────────────────────────────────────────────┐
│                        Eino Handoff 架构                             │
├──────────────────────────────────────────────────────────────────────┤
│                                                                       │
│  ┌───────────────────────────────────────────────────────────────┐   │
│  │                     User Layer                                │   │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐    │   │
│  │  │  CLI Tool    │  │   Wrapper    │  │ Handler (Direct) │    │   │
│  │  │  (终端用户)   │  │  (快速集成)   │  │  (底层控制)       │    │   │
│  │  └──────┬───────┘  └──────┬───────┘  └────────┬─────────┘    │   │
│  │         │                  │                    │             │   │
│  │         └──────────────────┴────────────────────┘             │   │
│  │                            │                                   │   │
│  └────────────────────────────┼───────────────────────────────────┘   │
│                               ▼                                       │
│  ┌───────────────────────────────────────────────────────────────┐   │
│  │                    Handoff Core                               │   │
│  │                                                               │   │
│  │   ┌──────────┐    ┌──────────┐    ┌──────────┐              │   │
│  │   │Collector │───→│Generator │───→│Formatter │              │   │
│  │   │          │    │          │    │          │              │   │
│  │   │ - Events │    │ - Summarize│   │ - YAML   │              │   │
│  │   │ - Filter │    │ - CodeState│   │ - Markdown              │   │
│  │   │ - State  │    │ - Content │    │ - Output │              │   │
│  │   └────┬─────┘    └────┬─────┘    └────┬─────┘              │   │
│  │        │               │               │                     │   │
│  │        └───────────────┴───────────────┘                     │   │
│  │                        │                                      │   │
│  │                   ┌────┴────┐                                 │   │
│  │                   │ Document│                                 │   │
│  │                   └────┬────┘                                 │   │
│  └────────────────────────┼──────────────────────────────────────┘   │
│                          ▼                                            │
│  ┌───────────────────────────────────────────────────────────────┐   │
│  │                    Extensions                                 │   │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐      │   │
│  │  │Summarizer│  │CodeTrack │  │ Formatter│  │  Loader  │      │   │
│  │  │ (可替换)  │  │ (可替换)  │  │ (可替换)  │  │ (可替换)  │      │   │
│  │  └──────────┘  └──────────┘  └──────────┘  └──────────┘      │   │
│  └───────────────────────────────────────────────────────────────┘   │
│                                                                       │
│  ┌───────────────────────────────────────────────────────────────┐   │
│  │                    Eino Framework                             │   │
│  │         callbacks.Handler / adk.Agent / compose.Graph        │   │
│  └───────────────────────────────────────────────────────────────┘   │
│                                                                       │
└──────────────────────────────────────────────────────────────────────┘
```

### 3.2 核心组件详解

#### 3.2.1 Collector（事件收集器）

**职责**：收集并管理事件流

```go
type Collector struct {
    config *CollectorConfig    // 配置（最大事件数、过滤器等）
    state  *State              // 累积状态
    mu     sync.RWMutex        // 线程安全
}
```

**工作原理**：

```
Eino Agent 执行
     │
     ▼
┌─────────────────┐
│  callbacks.OnStart│
│  callbacks.OnEnd  │ ←── 自动触发
│  callbacks.OnError│
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│   Collector     │
│  ┌───────────┐  │
│  │  Convert  │  │  ←── 转换为 Event 结构
│  │   to Event│  │
│  └─────┬─────┘  │
│        │        │
│  ┌─────▼─────┐  │
│  │   Filter  │  │  ←── 应用 EventFilter
│  └─────┬─────┘  │
│        │        │
│  ┌─────▼─────┐  │
│  │   Store   │  │  ←── 存储到 State
│  │   in State│  │
│  └───────────┘  │
└─────────────────┘
```

**关键设计点**：

1. **自动收集**：通过 `callbacks.Handler` 自动捕获所有 agent 事件
2. **手动标记**：提供 API 手动标记决策、里程碑
3. **线程安全**：支持并发事件处理
4. **可过滤**：通过 `EventFilter` 过滤噪声事件

#### 3.2.2 Generator（文档生成器）

**职责**：将事件和状态转换为结构化的 Document

```go
type Generator struct {
    summarizer  Summarizer   // 智能摘要
    codeTracker CodeTracker  // 代码状态追踪
    formatter   Formatter    // 格式化输出
}
```

**生成流程**：

```
┌──────────┐     ┌──────────────┐     ┌──────────────┐
│  State   │────→│  Summarizer  │────→│   Content    │
│  (Events)│     │              │     │  - Summary   │
│  (Files) │     │ 1. 规则提取   │     │  - Decisions │
│  (Custom)│     │ 2. 统计计算   │     │  - NextSteps │
└──────────┘     │ 3. LLM精炼   │     │  - Issues    │
                 └──────────────┘     └──────┬───────┘
                                              │
┌──────────┐     ┌──────────────┐            │
│ CodeTrack│────→│  CodeState   │────────────┤
│ (git/fs) │     │  - Files     │            │
└──────────┘     │  - Changes   │            │
                 └──────────────┘            │
                                             ▼
                                      ┌──────────────┐
                                      │   Document   │
                                      │  (结构化的)   │
                                      └──────┬───────┘
                                             │
                                             ▼
                                      ┌──────────────┐
                                      │   Formatter  │
                                      │  (YAML + MD) │
                                      └──────┬───────┘
                                             │
                                             ▼
                                      ┌──────────────┐
                                      │   handoff.md │
                                      └──────────────┘
```

#### 3.2.3 Formatter（格式化器）

**职责**：将 Document 格式化为 YAML + Markdown

**输出示例**：

```yaml
---
handoff_version: "1.0"
session:
  id: "sess_20240115_001"
  started_at: "2024-01-15T09:00:00Z"
  handoff_at: "2024-01-15T10:30:00Z"
current_task:
  title: "实现用户认证系统"
  status: "in_progress"
  progress: 45
stats:
  total_messages: 24
  tool_calls: 8
  file_changes: 3
---

# 任务摘要

正在将用户认证系统从 session 迁移到 JWT...

## 关键决策

| 时间 | 决策 | 理由 | 状态 |
|------|------|------|------|
| 09:35 | 使用 RS256 | 比 HS256 更安全 | ✅ 已实施 |

## 代码状态

### 工作文件
- `src/service/auth.go` (第 45 行) 📝
- `src/middleware/jwt.go` ✅

## 下一步

### 高优先级
1. **完成密码验证逻辑**
2. **实现错误处理**

### 中优先级
3. 编写单元测试

## 待解决问题

- [ ] **阻塞** 旧版 session 如何迁移？
```

### 3.3 数据流

#### 完整数据流图

```
┌──────────────────────────────────────────────────────────────────────┐
│                          数据流                                      │
├──────────────────────────────────────────────────────────────────────┤
│                                                                       │
│  Phase 1: 收集阶段 (Collection)                                      │
│  ═══════════════════════════════                                     │
│                                                                       │
│  Agent 运行 ──→ Callback ──→ Event ──→ Filter ──→ State.Events      │
│       │                                                                  │
│       ├── User Message ──→ EventTypeMessage                          │
│       ├── Tool Call ─────→ EventTypeToolCall                         │
│       ├── Tool Result ───→ EventTypeToolResult                       │
│       ├── Error ─────────→ EventTypeError                            │
│       └── Custom ────────→ EventTypeCustom                           │
│                                                                       │
│  手动标记:                                                            │
│  MarkDecision() ─────────────────────────→ State.Decisions           │
│  MarkMilestone() ────────────────────────→ State.Milestones          │
│  AddCustomData() ────────────────────────→ State.CustomData          │
│                                                                       │
│  Phase 2: 生成阶段 (Generation)                                      │
│  ═══════════════════════════════                                     │
│                                                                       │
│  Trigger: handler.Generate()                                          │
│       │                                                               │
│       ▼                                                               │
│  ┌─────────────────┐                                                  │
│  │  Build Metadata │                                                  │
│  │  - Session info │                                                  │
│  │  - Task info    │                                                  │
│  │  - Stats        │                                                  │
│  └────────┬────────┘                                                  │
│           │                                                           │
│           ▼                                                           │
│  ┌─────────────────┐     ┌─────────────────┐                         │
│  │  Summarize()    │←────│  CodeTracker()  │                         │
│  │  - Events       │     │  - Git status   │                         │
│  │  - Decisions    │     │  - File changes │                         │
│  │  - Rules        │     │  - Repo info    │                         │
│  └────────┬────────┘     └─────────────────┘                         │
│           │                                                           │
│           ▼                                                           │
│  ┌─────────────────┐                                                  │
│  │  Content        │                                                  │
│  │  - Summary      │                                                  │
│  │  - Decisions    │                                                  │
│  │  - CodeState    │                                                  │
│  │  - NextSteps    │                                                  │
│  │  - Issues       │                                                  │
│  └────────┬────────┘                                                  │
│           │                                                           │
│           ▼                                                           │
│  ┌─────────────────┐                                                  │
│  │  Document       │                                                  │
│  │  (结构化数据)    │                                                  │
│  └────────┬────────┘                                                  │
│           │                                                           │
│  Phase 3: 输出阶段 (Output)                                          │
│  ═══════════════════════════                                         │
│           │                                                           │
│           ▼                                                           │
│  ┌─────────────────┐                                                  │
│  │  Format()       │                                                  │
│  │  - YAML header  │                                                  │
│  │  - Markdown body│                                                  │
│  └────────┬────────┘                                                  │
│           │                                                           │
│           ▼                                                           │
│  ┌─────────────────┐                                                  │
│  │  handoff.md     │                                                  │
│  │  (最终输出)      │                                                  │
│  └─────────────────┘                                                  │
│                                                                       │
└──────────────────────────────────────────────────────────────────────┘
```

---

## 4. 详细设计

### 4.1 Document Schema

#### Schema 设计原则

1. **稳定性**：使用 `handoff_version` 管理版本
2. **完整性**：覆盖任务、代码、决策、下一步四大维度
3. **可扩展**：`custom_fields` 允许应用特定数据
4. **可验证**：必需字段检查，类型安全

#### 完整 Schema

```yaml
---
# 版本信息 (必需)
handoff_version: "1.0"           # Schema 版本

# 会话信息 (必需)
session:
  id: "string"                   # 会话唯一 ID
  started_at: "timestamp"        # 会话开始时间
  handoff_at: "timestamp"        # handoff 生成时间
  description: "string"          # 会话描述 (可选)

# 当前任务 (必需)
current_task:
  title: "string"                # 任务标题
  description: "string"          # 任务描述 (可选)
  status: "enum"                 # 状态: in_progress/blocked/review_needed/completed
  progress: 0-100                # 完成百分比
  started_at: "timestamp"        # 任务开始时间 (可选)
  estimated_remaining: "string"  # 预估剩余时间 (可选)

# 上下文信息 (可选)
context:
  parent_goal: "string"          # 父级目标
  completed_milestones:          # 已完成的里程碑
    - title: "string"
      completed_at: "timestamp"
      description: "string"
  dependencies:                  # 依赖条件
    - "string"
  custom_fields:                 # 自定义字段
    key: "value"

# 统计信息 (自动计算)
stats:
  total_messages: 0              # 消息总数
  tool_calls: 0                  # 工具调用数
  file_changes: 0                # 文件变更数
  llm_tokens_in: 0               # 输入 token 数 (可选)
  llm_tokens_out: 0              # 输出 token 数 (可选)
---

# Markdown 内容 (人类可读)
```

#### 字段详细说明

**current_task.status**

```
in_progress    - 正在进行中，可以接手继续
blocked        - 被阻塞，需要解决阻塞问题
review_needed  - 需要审查，完成后可合并
completed      - 已完成，可以开始新任务
```

**stats 的意义**

```
total_messages: 反映会话的活跃程度
                20+ 表示深入的讨论

tool_calls:     反映代码操作的频率
                高 = 大量文件操作

file_changes:   反映代码变更范围
                帮助接手者了解影响面
```

### 4.2 事件收集机制

#### Event 类型系统

```go
type EventType string

const (
    EventTypeMessage    = "message"      // 用户/助手消息
    EventTypeToolCall   = "tool_call"    // 工具调用
    EventTypeToolResult = "tool_result"  // 工具结果
    EventTypeDecision   = "decision"     // 决策标记
    EventTypeFileChange = "file_change"  // 文件变更
    EventTypeError      = "error"        // 错误
    EventTypeCustom     = "custom"       // 自定义
)
```

#### 自动事件转换

```go
// Eino Callback Input → Event

// Chat Model Input → Message Event
model.CallbackInput{
    Messages: [...]
}
↓
Event{
    Type: EventTypeMessage,
    Data: EventData{
        Role:    "user/assistant/system",
        Content: "message content",
    },
}

// Tool Call → Tool Event
compose.ToolInvokeInput{
    Name: "read_file",
    Arguments: {"path": "main.go"},
}
↓
Event{
    Type: EventTypeToolCall,
    Data: EventData{
        ToolName: "read_file",
        Input:    {"path": "main.go"},
    },
}
↓
// 同时更新 State.FilesTouched
```

#### 手动事件标记

```go
// 标记关键决策
handler.MarkDecision(Decision{
    Title:     "使用 gRPC 而非 REST",
    Reasoning: "更高的性能，强类型契约",
    Status:    "decided",
    Time:      time.Now(),
})
↓
Event{
    Type: EventTypeDecision,
    Data: EventData{Decision: ...},
}
State.Decisions = append(...)

// 标记里程碑
handler.MarkMilestone(Milestone{
    Title:       "完成接口设计",
    Description: "定义了所有服务接口",
    CompletedAt: time.Now(),
})
↓
State.CompletedMilestones = append(...)
```

### 4.3 智能摘要算法

#### 算法架构

```
┌───────────────────────────────────────────────────────┐
│                    Summarizer                         │
├───────────────────────────────────────────────────────┤
│                                                       │
│  Input: []Event + State                               │
│       │                                               │
│       ▼                                               │
│  ┌─────────────────────────────────────────────┐     │
│  │  Stage 1: Rule-Based Extraction             │     │
│  │  ─────────────────────────────              │     │
│  │  • Count event types (messages, tools)      │     │
│  │  • Extract file operations                  │     │
│  │  • Identify decision patterns               │     │
│  │  • Find TODO/FIXME markers                  │     │
│  │  • Detect questions (?)                     │     │
│  └────────────────────┬────────────────────────┘     │
│                       │                               │
│                       ▼                               │
│  ┌─────────────────────────────────────────────┐     │
│  │  Stage 2: Content Generation                │     │
│  │  ────────────────────────────               │     │
│  │  • Generate summary from stats              │     │
│  │  • Format decisions table                   │     │
│  │  • Build next steps list                    │     │
│  │  • Extract open issues                      │     │
│  └────────────────────┬────────────────────────┘     │
│                       │                               │
│                       ▼                               │
│  ┌─────────────────────────────────────────────┐     │
│  │  Stage 3: LLM Refinement (Optional)         │     │
│  │  ─────────────────────────────────          │     │
│  │  • Build prompt from extracted data         │     │
│  │  • Call LLM for refinement                  │     │
│  │  • Parse and merge results                  │     │
│  └────────────────────┬────────────────────────┘     │
│                       │                               │
│                       ▼                               │
│  Output: *Content                                     │
│                                                       │
└───────────────────────────────────────────────────────┘
```

#### 规则提取详解

**文件操作提取**：

```go
// 从工具调用中提取文件变更
func extractFileChanges(events []Event) []FileChange {
    fileTools := []string{
        "read_file", "write_file", "edit_file",
        "apply_diff", "create_file", "delete_file",
    }

    for _, event := range events {
        if event.Type == EventTypeToolCall {
            if contains(fileTools, event.Data.ToolName) {
                path := event.Data.Input["path"]
                // 记录文件变更
                changes = append(changes, FileChange{
                    Path:       path,
                    ChangeType: mapToolToChangeType(event.Data.ToolName),
                    Time:       event.Timestamp,
                })
            }
        }
    }
}
```

**TODO 提取**：

```go
// 从消息中提取待办事项
func extractTODOs(events []Event) []NextStep {
    patterns := []string{"TODO", "FIXME", "XXX", "HACK"}

    for _, event := range events {
        if event.Type == EventTypeMessage {
            content := event.Data.Content
            for _, pattern := range patterns {
                if strings.Contains(content, pattern) {
                    todo := extractLineWithPattern(content, pattern)
                    todos = append(todos, NextStep{
                        Priority: PriorityHigh,
                        Title:    todo,
                    })
                }
            }
        }
    }
}
```

**决策提取**：

```go
// 识别决策性语句
func extractDecisions(events []Event) []Decision {
    decisionPatterns := []string{
        "we decided", "let's use", "we'll go with",
        "选择", "决定使用", "采用",
    }

    for _, event := range events {
        if event.Type == EventTypeMessage {
            content := strings.ToLower(event.Data.Content)
            for _, pattern := range decisionPatterns {
                if strings.Contains(content, pattern) {
                    // 提取决策语句
                    decision := parseDecisionStatement(event.Data.Content)
                    decisions = append(decisions, decision)
                }
            }
        }
    }
}
```

#### LLM 精炼流程

```go
// 构建精炼提示
func buildRefinementPrompt(content *Content, events []Event) string {
    return fmt.Sprintf(`
你正在协助总结一次编程会话。请基于以下信息，
撰写一个清晰、简洁的摘要（2-3句话），说明已完成的工作和当前状态。

当前摘要：
%s

关键决策：
%s

工作文件：
%s

请提供一个精炼后的摘要：
`, content.Summary,
   formatDecisions(content.Decisions),
   formatFiles(content.CodeState.WorkingFiles))
}

// 精炼流程
func (s *LLMSummarizer) Refine(content *Content, events []Event) (*Content, error) {
    prompt := buildRefinementPrompt(content, events)

    // 调用 LLM
    refined, err := s.llmClient.Generate(prompt)
    if err != nil {
        return nil, err
    }

    // 更新摘要
    content.Summary = refined

    return content, nil
}
```

---

## 5. 使用指南

### 5.1 三种使用模式

#### 模式一：Handler（底层控制）

**适用场景**：
- 需要完全控制 handoff 生成时机
- 需要自定义 summarizer/code tracker
- 需要在特定条件下触发 handoff

**代码示例**：

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/bytedance/eino/adk"
    "github.com/bytedance/eino-ext/components/handoff"
)

func main() {
    ctx := context.Background()

    // 1. 创建 Handler
    handler := handoff.NewHandler(&handoff.HandlerConfig{
        SessionID:          "my-session-001",
        SessionDescription: "实现订单系统",
        CodeTracker:        handoff.NewDefaultCodeTracker(),
    })

    // 2. 创建 Agent
    agent := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
        Model: myModel,
        Tools: myTools,
    })

    // 3. 运行 Agent，传入 Handler 作为 Callback
    input := &adk.AgentInput{Input: "帮我设计订单表结构"}
    events := agent.Run(ctx, input, adk.WithCallbacks(handler))

    // 4. 处理事件...
    processEvents(events)

    // 5. 手动标记关键决策
    handler.MarkDecision(handoff.Decision{
        Title:     "使用雪花算法生成订单ID",
        Reasoning: "分布式环境下保证唯一性",
        Status:    "decided",
    })

    // 6. 手动标记里程碑
    handler.MarkMilestone(handoff.Milestone{
        Title:       "完成数据库设计",
        Description: "定义了 orders 表和 order_items 表",
    })

    // 7. 生成 Handoff 文档
    doc, err := handler.Generate(ctx, &handoff.GenerateOptions{
        TaskTitle:       "订单系统数据库设计",
        TaskDescription: "设计订单相关的数据库表结构",
        TaskStatus:      handoff.TaskStatusInProgress,
        TaskProgress:    30,
    })
    if err != nil {
        panic(err)
    }

    // 8. 保存文档
    if err := doc.Save("handoff.md"); err != nil {
        panic(err)
    }

    fmt.Println("Handoff 文档已生成: handoff.md")
}
```

#### 模式二：Wrapper（快速集成）

**适用场景**：
- 已有 Agent，想快速添加 handoff 能力
- 不需要精细控制生成过程
- 希望自动保存到指定目录

**代码示例**：

```go
package main

import (
    "context"
    "fmt"

    "github.com/bytedance/eino/adk"
    "github.com/bytedance/eino-ext/components/handoff"
)

func main() {
    ctx := context.Background()

    // 1. 创建你的 Agent
    myAgent := createMyAgent()

    // 2. 包装 Agent
    wrapped, err := handoff.Wrap(myAgent, &handoff.WrapConfig{
        SessionID:  "order-system",
        OutputPath: "./handoffs/",
        OnBeforeHandoff: func(ctx context.Context, h *handoff.HandoffContext) error {
            fmt.Println("正在生成 handoff...")
            return nil
        },
        OnAfterHandoff: func(ctx context.Context, path string) {
            fmt.Printf("Handoff 已保存: %s\n", path)
        },
    })
    if err != nil {
        panic(err)
    }

    // 3. 像正常使用 Agent 一样使用 Wrapped Agent
    input := &adk.AgentInput{Input: "帮我实现订单创建接口"}
    events := wrapped.Run(ctx, input)

    // 4. 处理事件...
    processEvents(events)

    // 5. 在需要时生成 Handoff
    path, err := wrapped.GenerateHandoff(ctx, &handoff.GenerateOptions{
        TaskTitle:    "实现订单创建接口",
        TaskStatus:   handoff.TaskStatusInProgress,
        TaskProgress: 50,
    })
    if err != nil {
        panic(err)
    }

    fmt.Printf("Handoff 文档: %s\n", path)
}
```

#### 模式三：Builder（流式 API）

**适用场景**：
- 喜欢链式调用风格
- 配置项较多
- 希望代码更可读

**代码示例**：

```go
package main

import (
    "context"

    "github.com/bytedance/eino-ext/components/handoff"
)

func main() {
    ctx := context.Background()

    // 使用 Builder 创建 Handler
    handler := handoff.NewHandlerBuilder().
        WithSessionID("builder-example").
        WithSessionDescription("使用 Builder 模式创建").
        WithMaxEvents(200).
        WithCodeTracker(handoff.NewDefaultCodeTracker()).
        WithSummarizer(handoff.NewDefaultSummarizer()).
        WithContext(&handoff.ContextInfo{
            ParentGoal: "重构整个后端服务",
        }).
        Build()

    // 使用 Handler...
    // (同模式一)

    doc, _ := handler.Generate(ctx, &handoff.GenerateOptions{
        TaskTitle: "Builder 模式示例",
    })

    doc.Save("handoff.md")
}
```

### 5.2 完整示例

#### 示例：多人协作开发

**场景**：小红设计架构，小明实现细节

**小红的代码**：

```go
// xiaohong.go
func main() {
    ctx := context.Background()

    // 创建 Handler
    handler := handoff.NewHandler(&handoff.HandlerConfig{
        SessionID: "order-system-arch",
        Context: &handoff.ContextInfo{
            ParentGoal: "构建完整的订单系统",
        },
    })

    // 架构设计工作...
    agent := adk.NewChatModelAgent(...)
    agent.Run(ctx, input, adk.WithCallbacks(handler))

    // 记录关键架构决策
    handler.MarkDecision(handoff.Decision{
        Title:     "使用事件驱动架构",
        Reasoning: "支持异步处理，提高系统吞吐量",
        Status:    "decided",
    })

    handler.MarkDecision(handoff.Decision{
        Title:     "订单状态使用状态机模式",
        Reasoning: "清晰的流转逻辑，易于维护",
        Status:    "decided",
    })

    // 标记完成的里程碑
    handler.MarkMilestone(handoff.Milestone{
        Title:       "完成架构设计",
        Description: "定义了系统模块划分和接口规范",
    })

    // 生成 Handoff
    doc, _ := handler.Generate(ctx, &handoff.GenerateOptions{
        TaskTitle:       "订单系统架构设计",
        TaskDescription: "完成整体架构设计和模块划分",
        TaskStatus:      handoff.TaskStatusCompleted,
        TaskProgress:    100,
    })

    doc.Save("handoff-from-xiaohong.md")
}
```

**生成的 handoff-from-xiaohong.md**：

```yaml
---
handoff_version: "1.0"
session:
  id: "order-system-arch"
  started_at: "2024-01-15T09:00:00Z"
  handoff_at: "2024-01-15T11:30:00Z"
current_task:
  title: "订单系统架构设计"
  status: "completed"
  progress: 100
context:
  parent_goal: "构建完整的订单系统"
  completed_milestones:
    - title: "完成架构设计"
      description: "定义了系统模块划分和接口规范"
---

# 任务摘要

已完成订单系统的整体架构设计，确定了事件驱动架构和状态机模式。

## 关键决策

| 时间 | 决策 | 理由 | 状态 |
|------|------|------|------|
| 09:30 | 使用事件驱动架构 | 支持异步处理，提高系统吞吐量 | ✅ 已实施 |
| 10:15 | 订单状态使用状态机模式 | 清晰的流转逻辑，易于维护 | ✅ 已实施 |

## 架构概览

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   API Gateway│────→│ Order Service│────→│  Event Bus  │
└─────────────┘     └─────────────┘     └──────┬──────┘
                                                │
                       ┌────────────────────────┼────────────────────────┐
                       ▼                        ▼                        ▼
                ┌─────────────┐          ┌─────────────┐          ┌─────────────┐
                │Inventory Svc│          │ Payment Svc │          │ Shipping Svc│
                └─────────────┘          └─────────────┘          └─────────────┘
```

## 接口规范

### OrderService

```go
type OrderService interface {
    CreateOrder(ctx context.Context, req *CreateOrderRequest) (*Order, error)
    GetOrder(ctx context.Context, orderID string) (*Order, error)
    CancelOrder(ctx context.Context, orderID string) error
}
```

## 下一步

1. **高**: 实现 OrderService 的核心方法
2. **高**: 定义领域事件结构
3. **中**: 实现事件发布和订阅
```

**小明的代码**：

```go
// xiaoming.go
func main() {
    ctx := context.Background()

    // 1. 加载小红的 Handoff
    loader := handoff.NewLoader()
    prevDoc, _ := loader.Load("handoff-from-xiaohong.md")

    // 2. 创建新的 Handler，继承上下文
    handler := handoff.NewHandler(&handoff.HandlerConfig{
        SessionID:          "order-system-impl",
        SessionDescription: "基于小红的设计进行实现",
        Context: &handoff.ContextInfo{
            ParentGoal:           prevDoc.Context.ParentGoal,
            Dependencies:         []string{"基于架构设计文档: handoff-from-xiaohong.md"},
            CompletedMilestones:  prevDoc.Context.CompletedMilestones,
        },
    })

    // 3. 基于小红的架构进行实现...
    agent := adk.NewChatModelAgent(...)
    agent.Run(ctx, input, adk.WithCallbacks(handler))

    // 4. 生成新的 Handoff
    doc, _ := handler.Generate(ctx, &handoff.GenerateOptions{
        TaskTitle:       "实现 OrderService",
        TaskDescription: "基于架构设计实现核心业务逻辑",
        TaskStatus:      handoff.TaskStatusInProgress,
        TaskProgress:    40,
    })

    doc.Save("handoff-from-xiaoming.md")
}
```

### 5.3 最佳实践

#### 实践一：何时生成 Handoff

**推荐时机**：

```
✅ 会话结束前（下班、开会前）
✅ 完成一个里程碑时
✅ 遇到复杂问题需要切换工具时
✅ 需要与团队协作时
✅ 代码提交前

❌ 每轮对话都生成（太频繁）
❌ 完全没有任何进展时
```

#### 实践二：如何写好决策记录

**好的决策记录**：

```go
handler.MarkDecision(handoff.Decision{
    Title:     "使用 PostgreSQL 而非 MySQL",
    Reasoning: "1. 更好的 JSON 支持（存储订单扩展字段）\n" +
               "2. 更强的 ACID 保证（金融数据一致性）\n" +
               "3. 团队已有运维经验",
    Status:    "decided",
    Alternatives: []string{
        "MySQL 8.0 - JSON 支持较弱",
        "MongoDB - 放弃 ACID",
    },
})
```

**差的决策记录**：

```go
handler.MarkDecision(handoff.Decision{
    Title:     "用 Postgres",
    Reasoning: "更好",
})
```

#### 实践三：如何处理敏感信息

**方法**：使用过滤器

```go
// 过滤敏感信息
filter := handoff.FilterByType{
    AllowedTypes: []handoff.EventType{
        handoff.EventTypeMessage,
        handoff.EventTypeToolCall,
        // 不包括 EventTypeCustom
    },
}

handler := handoff.NewHandler(&handoff.HandlerConfig{
    Collector: &handoff.CollectorConfig{
        Filters: []handoff.EventFilter{filter},
    },
})
```

#### 实践四：版本控制集成

**推荐做法**：

```bash
# 将 handoff 文件纳入版本控制
# .gitignore 中添加例外
!handoffs/
!handoffs/*.md

# 提交时包含 handoff
git add handoffs/handoff-*.md
git commit -m "feat: 完成订单模块设计

包含 handoff 文档，记录了架构决策""
```

---

## 6. 实际案例分析

### 6.1 个人开发跨会话

**用户**：独立开发者小明
**场景**：开发一个电商后台系统

**Day 1**：

```
时间: 14:00 - 17:30 (3.5 小时)
工具: Claude (Web)
任务: 设计数据库和核心实体
```

**小明的操作**：

```go
// 下午 5:30，准备下班
handler := handoff.NewHandler(&handoff.HandlerConfig{
    SessionID: "ecommerce-day1",
})

// ... 3.5 小时的开发工作 ...

// 下班前生成 Handoff
doc, _ := handler.Generate(ctx, &handoff.GenerateOptions{
    TaskTitle:       "电商后台 - 数据库设计",
    TaskStatus:      handoff.TaskStatusCompleted,
    TaskProgress:    100,
})
doc.Save("handoff-day1.md")
```

**生成的 handoff-day1.md** 包含：
- 5 个关键决策（为什么用 PostgreSQL、订单 ID 生成策略等）
- 完成的数据库 schema
- 已完成的 3 个里程碑

---

**Day 2**：

```
时间: 09:00 - 12:00 (3 小时)
工具: Cursor (IDE)
任务: 实现 Repository 层
```

**小明的操作**：

```go
// 上午 9:00，加载昨天的 Handoff
loader := handoff.NewLoader()
yesterday, _ := loader.Load("handoff-day1.md")

// 打印摘要
fmt.Println(loader.ExtractSummary(yesterday))
// 输出:
// 任务: 电商后台 - 数据库设计
// 进度: 100%
// 状态: completed
// 决策数: 5
// 下一步数: 2

// 创建新的 Handler，继续追踪
handler := handoff.NewHandler(&handoff.HandlerConfig{
    SessionID: "ecommerce-day2",
    Context: &handoff.ContextInfo{
        ParentGoal: "构建完整电商后台",
        Dependencies: []string{
            "基于 Day 1 的数据库设计: handoff-day1.md",
        },
    },
})

// ... 继续开发 ...
```

**效果**：
- 无需重新解释项目背景
- 直接查看昨天做的关键决策
- 了解今天应该实现什么

### 6.2 团队协作

**团队**：3 人后端团队
**项目**：支付系统重构

**角色分工**：
- 小红（高级工程师）：架构设计
- 小明（中级工程师）：核心实现
- 小李（初级工程师）：测试和文档

**协作流程**：

```
Week 1
├── 小红: 架构设计
│   └── handoff-arch.md
│       ├── 技术选型决策
│       ├── 接口规范
│       └── 已知限制
│
Week 2
├── 小明: 核心实现
│   └── handoff-impl.md
│       ├── 基于架构设计的实现
│       ├── 新增的实现细节决策
│       └── 发现的问题
│
Week 3
├── 小李: 测试 + 文档
│   └── handoff-test.md
│       ├── 测试覆盖情况
│       ├── 发现的问题
│       └── 使用文档
```

**Handoff 的价值**：

```
没有 Handoff 时：
小明: "小红，这个接口的 timeout 你当时设的是多少？"
小红: "我看看...应该是 30 秒？"
小明: "但代码里是 10 秒..."
小红: "哦对，后来改了，因为..."
（浪费 15 分钟回忆当时的决策）

有 Handoff 时：
小明: 查看 handoff-arch.md
      ├── 关键决策表格
      │   └── "API Timeout: 10s" (原因: 防止雪崩)
      └── 已知限制
          └── "超时时间可能需要根据压测调整"
（1 分钟找到答案）
```

### 6.3 工具切换

**场景**：从 Cursor 切换到 Claude 解决复杂问题

**Cursor 中**：

```
用户: 帮我实现一个高性能的订单查询接口
Cursor: 好的，我先设计一下...
[30 分钟开发]

用户: 等等，我需要更复杂的查询优化建议
       让我切换到 Claude
```

**导出 Handoff**：

```bash
# 在 Cursor 中运行（假设有 CLI）
> /handoff export --output=order-query-handoff.md
✓ 已生成 handoff: order-query-handoff.md
```

**生成的文档包含**：
- 已完成的：基础查询接口、索引设计
- 关键决策：使用 Elasticsearch 做全文检索
- 当前问题：复杂查询性能不达标
- 下一步：优化查询语句或考虑缓存

**切换到 Claude**：

```
用户: [上传 order-query-handoff.md]

Claude: 我已经了解了您的工作。根据 handoff 文档：
        - 您已经实现了基础查询接口
        - 使用了 Elasticsearch 做全文检索
        - 当前遇到的问题是复杂查询性能不达标

        我建议从以下几个方面优化：
        1. 添加查询结果缓存
        2. 优化 Elasticsearch 的 mapping
        3. 考虑使用预聚合

        您想先从哪个方面入手？
```

**效果**：
- 无需重复解释项目背景
- Claude 直接了解当前状态和问题
- 可以立即给出针对性建议

---

## 7. 与现有方案对比

### 7.1 对比表

| 特性 | Eino Handoff | Claude Handoff | Eino Checkpoint | 人工文档 |
|------|--------------|----------------|-----------------|----------|
| **框架集成** | ✅ 深度集成 | ❌ 无 | ✅ 深度集成 | ❌ 无 |
| **人类可读** | ✅ YAML+MD | ✅ Markdown | ❌ 二进制 | ✅ Markdown |
| **智能摘要** | ✅ 规则+LLM | ❌ 模板填充 | ❌ 无 | ❌ 纯人工 |
| **代码感知** | ✅ Git/FS | ❌ 无 | ❌ 无 | ⚠️ 部分 |
| **跨工具** | ✅ 通用格式 | ⚠️ 仅文本 | ❌ 同版本 | ✅ 通用 |
| **自动化** | ✅ 自动收集 | ❌ 手动 | ✅ 自动 | ❌ 手动 |
| **版本控制** | ✅ 友好 | ✅ 友好 | ❌ 不友好 | ✅ 友好 |
| **自定义** | ✅ 高 | ⚠️ 低 | ⚠️ 中 | ✅ 高 |

### 7.2 详细对比

#### vs Claude Handoff

**Claude Handoff** 是一个社区 Python 工具：

```python
# Claude Handoff 使用方式
# 1. 手动复制粘贴会话历史
# 2. 运行脚本生成模板
# 3. 手动填写各个部分
```

**Eino Handoff 的优势**：

```
1. 自动收集: 通过 callback 自动捕获事件，无需手动复制
2. 智能分析: 自动提取文件变更、决策点，不是模板填充
3. 代码感知: 自动检测 git 状态、文件修改
4. 框架集成: 与 Agent 生命周期绑定，无需额外操作
```

#### vs Eino Checkpoint

**Eino Checkpoint** 是二进制状态持久化：

```go
// Checkpoint 使用
checkpoint := compose.NewCheckPoint(...)
graph.Run(ctx, input, compose.WithCheckPoint(checkpoint))
// 保存的是内存状态
```

**Eino Handoff 的不同定位**：

```
Checkpoint: 程序恢复      →  机器快速恢复执行
Handoff:    人类交接      →  人类理解并继续工作

Checkpoint 解决: "程序在中断处继续"
Handoff 解决: "另一个人理解发生了什么并接手"

两者可以共存：
- 开发时使用 Handoff 交接
- 部署时使用 Checkpoint 恢复
```

### 7.3 选择建议

```
场景 1: 需要人类理解交接内容
  → 使用 Eino Handoff

场景 2: 程序自动化恢复
  → 使用 Eino Checkpoint

场景 3: 跨工具切换
  → 使用 Eino Handoff

场景 4: 简单脚本，无框架
  → 使用 Claude Handoff 或人工文档
```

---

## 8. 未来演进

### 8.1 近期规划 (Phase 1)

- [x] 核心功能实现
- [x] 基础测试覆盖
- [ ] 更多示例（Web 开发、数据处理、AI 应用）
- [ ] IDE 插件指南
- [ ] CI/CD 集成示例

### 8.2 中期规划 (Phase 2)

- [ ] Web 可视化：渲染为美观的 HTML 页面
- [ ] 差异对比：handoff diff 命令
- [ ] 导入工具：从其他格式导入
- [ ] 团队协作：共享 handoff 的协作空间

### 8.3 远期愿景 (Phase 3)

- [ ] AI 自动接手：Agent 自动读取 handoff 并继续
- [ ] 智能合并：自动合并多个 handoff
- [ ] 预测建议：基于历史 handoff 给出建议
- [ ] 生态系统：与其他 AI 开发工具集成

### 8.4 Schema 演进策略

```
版本兼容性策略：

1. 向后兼容：新版本解析旧文档
   - 缺少的字段使用默认值
   - 未知字段忽略

2. 版本声明：每个文档包含 handoff_version
   - "1.0" - 初始版本
   - "1.1" - 新增字段
   - "2.0" - 重大变更

3. 迁移工具：提供版本升级工具
   handoff migrate --from=1.0 --to=1.1 old.md > new.md
```

---

## 总结

Eino Handoff 试图解决 AI 辅助开发中最容易被忽视的问题：**知识的传递和延续**。

它不仅仅是一个工具，更是一种**工作方式**：
- 让每个 AI 会话都产生可沉淀的资产
- 让团队协作不再依赖口头交流
- 让工具切换不再丢失上下文

**核心理念**：
```
AI 会话应该像代码一样：
- 可版本化（handoff.md）
- 可审查（结构化格式）
- 可交接（人类可读）
- 可延续（完整上下文）
```

**开始使用的最小步骤**：
```go
handler := handoff.NewHandler(nil)
agent.Run(ctx, input, adk.WithCallbacks(handler))
doc, _ := handler.Generate(ctx, nil)
doc.Save("handoff.md")
```

---

**附录：术语表**

| 术语 | 定义 |
|------|------|
| Handoff | 工作交接文档，记录会话上下文 |
| Collector | 事件收集器，通过 callback 收集事件 |
| Summarizer | 摘要生成器，提取关键信息 |
| CodeTracker | 代码追踪器，检测文件变更 |
| Formatter | 格式化器，输出 YAML+Markdown |
| Loader | 加载器，解析 handoff 文档 |
| Wrapper | 包装器，快速集成到现有 Agent |

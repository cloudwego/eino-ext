Version migration notes for Eino framework, covering v0.4 through v0.8.

## v0.4 -- Compose Optimization

**Released:** 2025-07-25

### Breaking Changes
- **Removed `GetState` method** from Graph. State management must use other mechanisms.
- **AllPredecessor mode defaults to eager execution** for improved performance.

### Notable Additions (v0.4.1 - v0.4.8)
- JSONSchema support for tool parameters (`schema.ToJSONSchema()`)
- `ToJSONSchema()` handles OpenAPIV3-to-JSONSchema conversion
- React Agent `WithTools` convenience function
- Reasoning content (ReasoningContent) support

### Migration Steps
1. Replace any `GetState` calls with state pre-handlers or other state access patterns.
2. Test graphs using AllPredecessor trigger mode -- behavior may change with eager execution.

## v0.5 -- ADK Implementation

**Released:** 2025-09-10

### Major Feature: Agent Development Kit (ADK)

The ADK introduced a complete agent development framework:

- **ChatModelAgent** -- ReAct-style agent with tool calling and streaming
- **Runner** -- Agent lifecycle management, checkpointing, event streams
- **Multi-agent patterns** -- Supervisor, Sequential, Plan-Execute-Replan
- **Session management** -- Event storage, session values, history rewriting
- **Interrupt/Resume** -- Agent execution pause and checkpoint-based recovery
- **Agent as Tool** -- Wrap agents as callable tools for other agents

### Notable Additions (v0.5.1 - v0.5.15)
- **DeepAgent** preset for complex task orchestration
- **Agent middleware** support
- **Global callbacks** for built-in agents
- **Multimodal support** (UserInputMultiContent, AssistantGenMultiContent)

### Migration Steps
1. For new agent use cases, prefer `adk.NewChatModelAgent` + `adk.NewRunner` over manual compose graph patterns.
2. Use `runner.Query(ctx, "text")` for simple single-turn, `runner.Run(ctx, history)` for multi-turn.

## v0.6 -- JSONSchema (Removed kin-openapi)

**Released:** 2025-11-14

### Breaking Changes
- **Removed `kin-openapi` dependency** and all OpenAPI 3.0 type definitions.
- Functions like `schema.NewParamsOneOfByOpenAPIV3` no longer exist.

### Migration Steps
1. Replace OpenAPI 3.0 types with standard JSONSchema types.
2. Use `schema.ToJSONSchema()` for tool parameter schemas.
3. Upgrade all eino-ext modules that reference removed types:
   ```bash
   go get github.com/cloudwego/eino-ext/components/model/openai@latest
   ```
4. If schema conversion is complex, use the conversion utilities from the migration guide.

## v0.7 -- Interrupt/Resume Refactor

**Released:** 2025-11-20

### Breaking Changes: Architecture-Level Interrupt/Resume Rewrite

Major refactoring (+7527/-1692 lines) of the Human-in-the-Loop mechanism:

**New APIs:**
- `compose.GetInterruptState[T]` -- Type-safe retrieval of interrupt state
- `compose.GetResumeContext` -- Check if current component is a resume target
- `adk.Interrupt(ctx, payload)` -- Trigger interrupt from any tool or agent

**New resume strategies:**
1. **Implicit "Resume All"** -- Single continue button resumes all interrupt points
2. **Explicit "Targeted Resume"** -- Resume specific interrupt points independently (recommended)

### Notable Additions (v0.7.1 - v0.7.36)
- **Tool interrupt API** -- Trigger interrupts during tool execution
- **Nested agent interrupt/resume** -- Arbitrary nesting depth
- **Skill middleware** -- Encapsulate reusable capabilities as Skills
- **ChatModel retry** -- Automatic retry with configurable `ModelRetryConfig`
- **Multimodal tools** -- Enhanced tool interface for multimodal I/O
- **Nested graph state access** -- Sub-graphs can access parent graph state

### Migration Steps
1. Replace manual interrupt state management with `GetInterruptState[T]`.
2. Use `CompositeInterrupt` for combining multiple interrupt signals.
3. Register types with gob for checkpoint serialization if using custom types.

## v0.8 -- ADK Middlewares

**Released:** 2026 (latest)

### Breaking Changes

#### API Changes
- `ShellBackend` renamed to `Shell`; `StreamingShellBackend` renamed to `StreamingShell`
- Shell interfaces no longer embed `Backend` -- implement them separately
- `Backend.Read` returns `*FileContent` instead of `string`

#### Behavioral Changes
- `ReadRequest.Offset`: 0-based -> 1-based
- `FileInfo.Path`: no longer guaranteed to be absolute
- `WriteRequest`: file-exists now overwrites instead of returning error
- `GrepRequest.Pattern`: literal string matching -> regex (ripgrep syntax)
- `EditRequest.FilePath`: no longer required to be absolute
- `AgentEvent` delivery via Middleware instead of eino callback

### Major Feature: ChatModelAgentMiddleware

New extensible middleware interface for ChatModelAgent:

```go
agent, _ := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
    Model:    model,
    Handlers: []adk.ChatModelAgentMiddleware{mw1, mw2},
})
```

Methods: `BeforeAgent`, `BeforeModelRewriteState`, `AfterModelRewriteState`, `WrapInvokableToolCall`, `WrapStreamableToolCall`, `WrapModel`.

### Built-in Middlewares
- **Summarization** -- Auto-summarize conversation history when tokens exceed threshold
- **ToolReduction** -- Truncate/clear tool outputs to save context
- **Filesystem** -- Enhanced file system tools with regex grep
- **Skill** -- Dynamic skill loading with fork/isolate context modes
- **PlanTask** -- Task planning and tracking
- **ToolSearch** -- Semantic search across large tool sets
- **PatchToolCalls** -- Fix dangling tool calls in message history

### Agent Callbacks
Agent-level callback support for observability:

```go
agent.Run(ctx, input, adk.WithCallbacks(handler))
```

### Language Setting
```go
adk.SetLanguage(adk.LanguageEnglish)  // or adk.LanguageChinese
```

### Migration Steps
1. Fix compile errors: rename `ShellBackend` -> `Shell`, update `Backend.Read` return type.
2. Update `ReadRequest.Offset` from 0-based to 1-based.
3. Escape regex special characters in `GrepRequest.Pattern`.
4. Check code relying on `WriteRequest` error-on-exists behavior.
5. Migrate ChatModel/Tool Decorators to `ChatModelAgentMiddleware`.
6. Upgrade backend implementations:
   ```bash
   go get github.com/cloudwego/eino-ext/adk/backend/local@latest
   ```
7. Run full test suite to verify.

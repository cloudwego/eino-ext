# ACP Bridge

将 [EINO ADK](https://github.com/cloudwego/eino) agent 桥接到 [Agent Client Protocol (ACP)](https://agentclientprotocol.com) 的工具库。提供两个核心函数：

- **`AgentEventToSessionUpdate`** — 将 eino `AgentEvent` 转换为 ACP `SessionUpdate` 通知，用于将 agent 输出流式推送给 ACP 客户端。
- **`NewClientToolsMiddleware`** — 将 ACP 客户端能力（文件读写、终端执行）桥接到 eino 的文件系统中间件，使 agent 能够在客户端上读写文件和执行命令。

## 安装

```bash
go get github.com/cloudwego/eino-ext/acp
```

## API 说明

### AgentEventToSessionUpdate

将 eino `AgentEvent` 转换为一组 ACP `SessionUpdate` 通知。支持流式和非流式消息输出。

```go
func AgentEventToSessionUpdate(
    event *adk.AgentEvent,
    opt *EventConverterOption,
) iter.Seq2[acpproto.SessionUpdate, error]
```

支持的事件类型映射：

| eino 事件 | ACP SessionUpdate |
|---|---|
| Assistant 文本消息 | `AgentMessageChunk` |
| 推理/思考内容 | `AgentThoughtChunk` |
| User 消息 | `UserMessageChunk` |
| 工具调用 | `ToolCall` |
| 工具结果 | `ToolCallUpdate` |
| Interrupt 中断 | `AgentMessageChunk`（`_meta["eino:interrupted"]` 标记，可自定义） |

#### 流式推送示例

在 ACP `Prompt` 方法中，遍历 agent 事件并逐条推送给客户端：

```go
iter := runner.Query(ctx, query)
for {
    event, ok := iter.Next()
    if !ok {
        break
    }
    for su, err := range einoacp.AgentEventToSessionUpdate(event, nil) {
        if err != nil {
            return acp.PromptResponse{}, err
        }
        conn.SessionUpdate(ctx, acp.SessionNotification{
            SessionID: sessionID,
            Update:    su,
        })
    }
}
return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, nil
```

#### 自定义 Interrupt 转换

eino 的 Interrupt（中断）机制在 ACP 中没有直接对应的概念。默认行为是将中断数据序列化为 `AgentMessageChunk`，并在 `_meta` 中标记中断信息。可以通过 `EventConverterOption` 自定义转换逻辑：

```go
opt := &einoacp.EventConverterOption{
    InterruptConverter: func(info *adk.InterruptInfo) iter.Seq2[acpproto.SessionUpdate, error] {
        return func(yield func(acpproto.SessionUpdate, error) bool) {
            yield(acpproto.NewSessionUpdateAgentMessageChunk(acpproto.ContentChunk{
                Content: acpproto.NewContentBlockText(acpproto.TextContent{
                    Text: fmt.Sprintf("需要确认: %v", info.Data),
                }),
            }), nil)
        }
    },
}

for su, err := range einoacp.AgentEventToSessionUpdate(event, opt) {
    // ...
}
```

默认转换的 `_meta` 结构：

```json
{
    "eino:interrupted": true,
    "eino:interruptContexts": [
        {
            "id": "agent:root;tool:bash",
            "isRootCause": true,
            "address": [{"type": "agent", "id": "root"}, {"type": "tool", "id": "bash"}],
            "info": "需要用户确认",
            "parentId": "agent:root"
        }
    ]
}
```

### NewClientToolsMiddleware

创建一个 `ChatModelAgentMiddleware`，将 ACP 客户端能力桥接到 eino 的文件系统工具。根据客户端声明的能力自动启用对应工具，未支持的工具会被禁用。

```go
func NewClientToolsMiddleware(
    ctx context.Context,
    sessionID acpproto.SessionID,
    capabilities *acpproto.ClientCapabilities,
    conn *acpconn.AgentConnection,
) (adk.ChatModelAgentMiddleware, error)
```

能力与工具的对应关系：

| 客户端能力 | 启用的工具 | 说明 |
|---|---|---|
| `fs.readTextFile` | `read_file` | 通过 ACP 连接读取客户端文件 |
| `fs.writeTextFile` | `write_file` | 通过 ACP 连接写入客户端文件 |
| `terminal` | Shell 命令执行 | 通过 ACP 连接在客户端创建终端并执行命令 |

#### 使用示例

在创建 session 时，根据客户端能力注入中间件：

```go
if clientCapabilities != nil {
    middleware, err := einoacp.NewClientToolsMiddleware(
        ctx, sessionID, clientCapabilities, conn,
    )
    if err != nil {
        return err
    }
    agentConfig.Handlers = append(agentConfig.Handlers, middleware)
}
```

## 完整示例

参见 [example/main.go](examples/main.go)，展示了一个完整的 ACP Server 实现：

1. 每个 session 创建独立的 eino `ChatModelAgent`
2. 通过 `NewClientToolsMiddleware` 桥接客户端文件系统和终端能力
3. 通过 `AgentEventToSessionUpdate` 将 agent 事件流式推送给 ACP 客户端

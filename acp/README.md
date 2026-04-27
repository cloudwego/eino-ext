# ACP Bridge

Utilities for bridging [EINO ADK](https://github.com/cloudwego/eino) agents to the [Agent Client Protocol (ACP)](https://agentclientprotocol.com). Provides two core functions:

- **`AgentEventToSessionUpdate`** — Converts eino `AgentEvent` into ACP `SessionUpdate` notifications for streaming agent output to ACP clients.
- **`NewClientToolsMiddleware`** — Bridges ACP client-side capabilities (filesystem, terminal) to eino's filesystem middleware, so the agent can read/write files and run commands on the client.

## Installation

```bash
go get github.com/cloudwego/eino-ext/acp
```

## API Reference

### AgentEventToSessionUpdate

Converts an eino `AgentEvent` into a sequence of ACP `SessionUpdate` notifications.

```go
func AgentEventToSessionUpdate(
    event *adk.AgentEvent,
    opt *EventConverterOption,
) iter.Seq2[acpproto.SessionUpdate, error]
```

Handles:
- **Assistant messages** → `AgentMessageChunk`
- **Reasoning content** → `AgentThoughtChunk`
- **User messages** → `UserMessageChunk`
- **Tool calls** → `ToolCall`
- **Tool results** → `ToolCallUpdate`
- **Interrupts** → `AgentMessageChunk` with `_meta["eino:interrupted"]` (customizable)

#### Streaming Events to Client

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

#### Custom Interrupt Converter

```go
opt := &einoacp.EventConverterOption{
    InterruptConverter: func(info *adk.InterruptInfo) iter.Seq2[acpproto.SessionUpdate, error] {
        return func(yield func(acpproto.SessionUpdate, error) bool) {
            // Custom interrupt handling logic
            yield(acpproto.NewSessionUpdateAgentMessageChunk(acpproto.ContentChunk{
                Content: acpproto.NewContentBlockText(acpproto.TextContent{
                    Text: fmt.Sprintf("Action required: %v", info.Data),
                }),
            }), nil)
        }
    },
}

for su, err := range einoacp.AgentEventToSessionUpdate(event, opt) {
    // ...
}
```

### NewClientToolsMiddleware

Creates a `ChatModelAgentMiddleware` that bridges ACP client-side capabilities to eino's filesystem tools.

```go
func NewClientToolsMiddleware(
    ctx context.Context,
    sessionID acpproto.SessionID,
    capabilities *acpproto.ClientCapabilities,
    conn *acpconn.AgentConnection,
) (adk.ChatModelAgentMiddleware, error)
```

Tools are enabled based on client-advertised capabilities:

| Client Capability | Enabled Tool |
|---|---|
| `fs.readTextFile` | `read_file` |
| `fs.writeTextFile` | `write_file` |
| `terminal` | Shell command execution |

```go
if clientCapabilities != nil {
    middleware, err := einoacp.NewClientToolsMiddleware(
        ctx, sessionID, clientCapabilities, conn,
    )
    if err != nil {
        return err
    }
    // Add to agent config
    agentConfig.Handlers = append(agentConfig.Handlers, middleware)
}
```

## Examples

See [example/main.go](examples/main.go) for a complete ACP server implementation that:

1. Creates an eino `ChatModelAgent` per session
2. Bridges client filesystem/terminal capabilities via `NewClientToolsMiddleware`
3. Streams `AgentEvent`s back as ACP `SessionUpdate` notifications

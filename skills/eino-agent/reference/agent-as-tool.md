AgentAsTool wraps an Agent as a Tool, enabling one agent to call another via function calling with isolated message history.

## NewAgentTool

```go
import "github.com/cloudwego/eino/adk"

agentTool := adk.NewAgentTool(ctx, subAgent, options...)
```

The wrapped agent:
- Receives a fresh task description (not the parent's full history)
- Shares SessionValues with the parent agent
- By default does NOT emit internal AgentEvents to the parent's iterator

To emit internal events, set `EmitInternalEvents: true` in the parent's ToolsConfig.

## Basic Usage

```go
import (
    "github.com/cloudwego/eino/adk"
    "github.com/cloudwego/eino/components/tool"
    "github.com/cloudwego/eino/compose"
)

// Create a sub-agent
researchAgent, _ := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
    Name:        "Researcher",
    Description: "Searches and summarizes information on any topic",
    Instruction: "Research the given topic thoroughly and provide a summary.",
    Model:       cm,
    ToolsConfig: adk.ToolsConfig{
        ToolsNodeConfig: compose.ToolsNodeConfig{
            Tools: []tool.BaseTool{webSearchTool},
        },
    },
})

// Wrap as tool
researchTool := adk.NewAgentTool(ctx, researchAgent)

// Use in parent agent
parentAgent, _ := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
    Name:        "Coordinator",
    Description: "Coordinates research and writing tasks",
    Instruction: "Use the Researcher tool to gather information, then write a report.",
    Model:       cm,
    ToolsConfig: adk.ToolsConfig{
        ToolsNodeConfig: compose.ToolsNodeConfig{
            Tools: []tool.BaseTool{researchTool},
        },
        EmitInternalEvents: true,  // See sub-agent's events
    },
})
```

## When to Use AgentAsTool

- The sub-agent needs only a clear task description, not the parent's full conversation history
- You want the parent to maintain control flow (call sub-agent, get result, continue reasoning)
- You need isolated context between parent and sub-agent (each has its own message history)
- SessionValues are shared between parent and sub-agent for cross-cutting concerns

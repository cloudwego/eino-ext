Common errors, FAQ, and troubleshooting guide for the Eino framework.

## kin-openapi Version Conflict

**Error:** `cannot use openapi3.TypeObject (untyped string constant "object") as *openapi3.Types value`

**Cause:** `github.com/getkin/kin-openapi` version exceeds v0.118.0.

**Fix:** Eino v0.6.0+ removed the kin-openapi dependency entirely. Upgrade to eino v0.6.0+:
```bash
go get github.com/cloudwego/eino@latest
```

If stuck on older versions, pin: `go get github.com/getkin/kin-openapi@v0.118.0`

## StreamToolCallChecker Issues

**Symptom:** Agent streaming does not enter ToolsNode, or streaming appears as non-streaming.

**Cause:** Different models output tool calls differently during streaming. OpenAI outputs tool calls directly; Claude outputs text first, then tool calls.

**Fix:** First, update to the latest eino version. If the default checker does not work for your model, provide a custom `StreamToolCallChecker`:

```go
// Custom checker that reads all chunks (loses streaming effect for non-tool responses)
toolCallChecker := func(ctx context.Context, sr *schema.StreamReader[*schema.Message]) (bool, error) {
    defer sr.Close()
    for {
        msg, err := sr.Recv()
        if err != nil {
            if errors.Is(err, io.EOF) {
                break
            }
            return false, err
        }
        if len(msg.ToolCalls) > 0 {
            return true, nil
        }
    }
    return false, nil
}
```

Tip: Add a prompt like "If you need to call a tool, output the tool call directly without additional text" to improve streaming detection.

## Import Path Confusion (eino vs eino-ext)

**Rule of thumb:**
- `github.com/cloudwego/eino` -- Interfaces, schema types, compose engine, ADK
- `github.com/cloudwego/eino-ext` -- Concrete implementations (OpenAI, Ark, Milvus, etc.)

**Common mistakes:**
- Importing `eino-ext` for interfaces (wrong -- use `eino`)
- Importing `eino` for ChatModel implementations (wrong -- use `eino-ext/components/model/openai`)
- After upgrading eino to v0.6.x, eino-ext modules may error with `undefined: schema.NewParamsOneOfByOpenAPIV3` -- upgrade the eino-ext module too

## v0.6.x Upgrade: JSONSchema Migration

**Error:** `undefined: schema.NewParamsOneOfByOpenAPIV3`

**Fix:** eino v0.6.0 removed all OpenAPI 3.0 types. Upgrade affected eino-ext modules:
```bash
go get github.com/cloudwego/eino-ext/components/model/openai@latest
```

Replace OpenAPI 3.0 schema definitions with standard JSONSchema using `schema.ToJSONSchema()`.

## sonic Loader Error (Go 1.24+)

**Error:** `github.com/bytedance/sonic/loader: invalid reference to runtime.lastmoduledatap`

**Fix:** Upgrade sonic to v1.13.2+:
```bash
go get github.com/bytedance/sonic@latest
```

## Tool Input Unmarshal Failure

**Error:** `failed to invoke tool call {tool_call_id}: unmarshal input fail`

**Cause:** Usually the model produced truncated or invalid JSON in the tool call arguments.

**Fix:** Check if the model output was truncated (long argument strings). Consider implementing a JSON fix middleware. See: [github.com/cloudwego/eino-examples/tree/main/components/tool/middlewares/jsonfix](https://github.com/cloudwego/eino-examples/tree/main/components/tool/middlewares/jsonfix)

## MCP Tool JSON Unmarshal Failure

**Error:** `failed to call mcp tool: failed to marshal request: json: error calling MarshalJSON for type json.RawMessage: unexpected end of JSON input`

**Cause:** Same as above -- the model produced invalid JSON in tool call arguments.

**Fix:** Same JSON fix approach. Add a prompt to encourage the model to output valid JSON.

## Context Timeout / Cancelled

**Error:** `context deadline exceeded` or `context canceled`

**Diagnosis:**
1. `context canceled` -- Your application code cancelled the context. Check your `context.WithCancel` usage.
2. `context deadline exceeded` -- Check the error's `node path: [node name]`:
   - If the node is NOT a ChatModel/external call: your upstream set a timeout on the context (e.g., FaaS platform).
   - If the node IS a ChatModel: check model-side timeout config (e.g., Ark SDK defaults to 10min, DeepSeek SDK defaults to 5min).

## Structured Output from Models

Three approaches:
1. Use model-specific config (e.g., OpenAI's `ResponseFormat`)
2. Use tool calling to get structured output
3. Prompt engineering

Parse structured output: `schema.NewMessageJSONParser[MyStruct]()`

## Accessing Reasoning Content

Models that support reasoning/thinking (e.g., DeepSeek, Claude) store it in `Message.ReasoningContent`.

## Batch Processing

Eino does not have built-in batch nodes. Two approaches:
1. Dynamically build a graph per request (low overhead)
2. Implement a custom batch processing Lambda node

See: [github.com/cloudwego/eino-examples/tree/main/compose/batch](https://github.com/cloudwego/eino-examples/tree/main/compose/batch)

## Accessing Parent Graph State from Sub-Graph

If sub-graph and parent graph have different state types, use `compose.ProcessState[ParentStateType]()`. If they share the same type, create a type alias: `type NewParentState StateType`.

## Visualizing Graph Topology

Use `GraphCompileCallback` to export topology during `graph.Compile`. Mermaid export example: [github.com/cloudwego/eino-examples/tree/main/devops/visualize](https://github.com/cloudwego/eino-examples/tree/main/devops/visualize)

## Gemini "missing thought_signature" Error

Gemini does not use the OpenAI-compatible protocol. Use the Gemini-specific ChatModel:
```bash
go get github.com/cloudwego/eino-ext/components/model/gemini@latest
```

## Debugging ChatModel API Errors

**Error:** `[NodeRunError] failed to create chat completion: error, status code: 400`

**Approach:** Print the actual HTTP request for debugging. See: [github.com/cloudwego/eino-examples/tree/main/components/model/httptransport](https://github.com/cloudwego/eino-examples/tree/main/components/model/httptransport)

Common causes: missing fields, wrong field values, incorrect BaseURL.

## Multimodal Input Not Working

If the model does not receive multimodal data, upgrade the eino-ext model package:
```bash
go get github.com/cloudwego/eino-ext/components/model/openai@latest
```

Use `UserInputMultiContent` for user-side multimodal input and `AssistantGenMultiContent` for model-side multimodal output.

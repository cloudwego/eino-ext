# Ark Chat Completions

[中文文档](README_zh.md)

An Eino `ToolCallingChatModel` built on the official
[`ark-runtime-go`](https://github.com/volcengine/ark-runtime-go) SDK recommended
by the [Ark installation guide](https://docs.volcengine.com/docs/ark/install-and-upgrade-sdk?lang=en).
It provides Chat Completions generation, streaming, function calling, multimodal
input, and synchronous batch inference through Eino messages and model options.
Authentication, HTTP requests, and retries use the official SDK.

See [go.mod](go.mod) for Go requirements and dependency versions, and the
[Chat API reference](https://docs.volcengine.com/docs/ark/chat-api?lang=en)
for model-specific parameters and supported capabilities.

## Installation

```sh
go get github.com/cloudwego/eino-ext/components/model/ark/v2
```

Configure `ARK_API_KEY` and `ARK_MODEL` in your application environment.
Use an API key and a model or endpoint ID available to your Ark account and region.

## Quick start

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    ark "github.com/cloudwego/eino-ext/components/model/ark/v2"
    "github.com/cloudwego/eino/schema"
)

func main() {
    ctx := context.Background()
    cm, err := ark.NewChatModel(ctx, &ark.ChatModelConfig{
        APIKey: os.Getenv("ARK_API_KEY"),
        Model:  os.Getenv("ARK_MODEL"),
    })
    if err != nil {
        log.Fatal(err)
    }
    messages := []*schema.Message{{Role: schema.User, Content: "Hello"}}
    reply, err := cm.Generate(ctx, messages)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(reply.Content)
}
```

`ChatModelConfig` configures the service connection and generation parameters:

| Configuration | Purpose |
| --- | --- |
| `APIKey` | API key authentication |
| `AccessKey`, `SecretKey` | AK/SK authentication; `APIKey` takes precedence when provided |
| `Model` | Model or endpoint ID |
| `BaseURL`, `Region` | Service endpoint and region |
| `Timeout`, `HTTPClient`, `RetryTimes` | Request timeout, custom HTTP client, and retries; `HTTPClient` takes precedence over `Timeout` |
| `MaxTokens`, `MaxCompletionTokens` | Output token limits; select the parameter supported by the model |
| `Temperature`, `TopP`, `Stop` | Sampling and stop sequences |
| `FrequencyPenalty`, `PresencePenalty`, `RepetitionPenalty`, `LogitBias` | Token selection controls |
| `Thinking`, `ReasoningEffort`, `ResponseFormat` | Thinking mode, reasoning effort, and output format using official SDK types |
| `CustomHeader`, `ServiceTier`, `User` | Custom headers, service tier, and end-user identifier |

Eino options such as `model.WithModel`, `model.WithTemperature`,
`model.WithMaxTokens`, and `model.WithTopP` set values for an individual call.
Ark-specific options include `WithThinking`, `WithReasoningEffort`,
`WithMaxCompletionTokens`, `WithParallelToolCalls`, and `WithCustomHeader`.
`WithCustomHeader` replaces the configured custom header map for that call.

## Streaming

Using the model and messages created above, add `errors` and `io` to the imports:

```go
streamCtx, cancel := context.WithCancel(ctx)
defer cancel()

reader, err := cm.Stream(streamCtx, messages)
if err != nil {
    log.Fatal(err)
}
defer reader.Close()

for {
    chunk, err := reader.Recv()
    if errors.Is(err, io.EOF) {
        break
    }
    if err != nil {
        log.Fatal(err)
    }
    fmt.Print(chunk.Content)
}
```

Streaming messages contain incremental content, reasoning, and tool calls.
Use `schema.ConcatMessages` to assemble collected chunks into one message.
The stream requests final token usage, which may arrive in a chunk without text.
Close the reader after use. When stopping early, also cancel the request context
so an HTTP read waiting for the next chunk can finish promptly.

## Function calling

Bind Eino tool definitions with `WithTools`:

```go
bound, err := cm.WithTools([]*schema.ToolInfo{
    {
        Name: "get_weather",
        Desc: "Get the weather for a city",
        ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
            "city": {Type: schema.String, Required: true},
        }),
    },
})
if err != nil {
    log.Fatal(err)
}

reply, err := bound.Generate(ctx, []*schema.Message{
    {Role: schema.User, Content: "What is the weather in Beijing?"},
})
if err != nil {
    log.Fatal(err)
}
for _, call := range reply.ToolCalls {
    fmt.Printf("%s: %s\n", call.Function.Name, call.Function.Arguments)
}
```

The model returns function requests in `reply.ToolCalls`; your application executes
them. Append the assistant message and a `schema.Tool` result for each call to the
conversation, then call the bound model again. Each result's `ToolCallID` must match
the corresponding tool call ID. Tool results can contain text or multimodal parts.

Use `model.WithTools` for per-call definitions, and `model.WithToolChoice` to allow,
forbid, or require a tool call. `ToolStrict` sets the model-wide default for strict
function schemas. Use `ark.WithToolStrict(tool, true)` or
`ark.WithToolStrict(tool, false)` to override that default for an individual
`*schema.ToolInfo`; the helper returns a copy. If neither the model default nor a
tool override is set, `strict` is omitted. Ensure each schema meets the model's
requirements when enabling strict mode.
`ParallelToolCalls` or `WithParallelToolCalls` controls whether a response may
contain multiple tool calls when supported by the model.

## Multimodal input and FileID

Use `UserInputMultiContent` in `schema.User` or `schema.Tool` messages:

| Eino part type | Content field | Additional settings |
| --- | --- | --- |
| `schema.ChatMessagePartTypeText` | `Text` | Text content |
| `schema.ChatMessagePartTypeImageURL` | `Image` | `Detail` controls image detail |
| `schema.ChatMessagePartTypeAudioURL` | `Audio` | `MIMEType` identifies the audio format |
| `schema.ChatMessagePartTypeVideoURL` | `Video` | `SetInputVideoFPS` sets frame sampling |
| `schema.ChatMessagePartTypeFileURL` | `File` | `Name` provides the filename |

For each media part, set one source in `MessagePartCommon`: `URL`, `Base64Data`,
or `FileID`. Image and video base64 require `MIMEType` and are sent as data URLs.
Audio uses raw base64 and a MIME-derived format. File base64 is sent as
`file_data` and requires `MessageInputFile.Name`.

For example, send a document uploaded through the Ark Files API:

```go
fileID := os.Getenv("ARK_FILE_ID")
messages := []*schema.Message{
    {
        Role: schema.User,
        UserInputMultiContent: []schema.MessageInputPart{
            {Type: schema.ChatMessagePartTypeText, Text: "Summarize this document."},
            {
                Type: schema.ChatMessagePartTypeFileURL,
                File: &schema.MessageInputFile{
                    MessagePartCommon: schema.MessagePartCommon{FileID: &fileID},
                },
            },
        },
    },
}
```

File upload and processing use the official SDK's Files API. Before inference,
ensure the file is active, belongs to the API key's project, and has a type
supported by the selected model. The adapter passes the FileID to Chat Completions.

A tool can return the same document content using the call ID from the assistant:

```go
toolResult := &schema.Message{
    Role:                  schema.Tool,
    ToolCallID:            toolCallID,
    UserInputMultiContent: messages[0].UserInputMultiContent,
}
```

Here `toolCallID` is the ID of the tool call whose execution produced the document.

## Official SDK parameters

`Thinking`, `ReasoningEffort`, and `ResponseFormat` use types from
`github.com/volcengine/ark-runtime-go/arkruntime/model/chat` and the SDK's
`NewOpt*` constructors. For example:

```go
thinking := &chat.Thinking{
    Type: chat.NewOptThinkingMode(chat.ThinkingModeEnabled),
}
reply, err := cm.Generate(ctx, messages, ark.WithThinking(thinking))
if err != nil {
    log.Fatal(err)
}
fmt.Println(reply.Content)
```

## Request and response customization

Configure the modifiers in `ChatModelConfig`, or select them per call:

| Option | Modified object | Applies to |
| --- | --- | --- |
| `WithRequestPayloadModifier` | Serialized request JSON, with the input Eino messages available as context | `Generate`, `Stream`, synchronous batch generation |
| `WithResponseMessageModifier` | Converted `*schema.Message`, with the original response JSON available as context | `Generate`, including synchronous batch generation |
| `WithResponseChunkMessageModifier` | Converted message chunk, with the original chunk JSON available as context | `Stream` |

To customize the request payload, import `encoding/json` for `json.RawMessage`
and `github.com/bytedance/sonic` for JSON encoding and decoding:

```go
modifier := ark.WithRequestPayloadModifier(func(ctx context.Context, messages []*schema.Message, rawBody []byte) ([]byte, error) {
    var payload map[string]json.RawMessage
    if err := sonic.Unmarshal(rawBody, &payload); err != nil {
        return nil, err
    }
    payload["stream_options"] = json.RawMessage(`{"include_usage":true,"chunk_include_usage":true}`)
    return sonic.Marshal(payload)
})
reader, err := cm.Stream(ctx, messages, modifier)
if err != nil {
    log.Fatal(err)
}
defer reader.Close()
```

The modifier runs once after request conversion and serialization, before the SDK
sends or retries the request. It can add fields the generated SDK types do not
expose. The result must be a JSON object retaining a string `model` and `messages`.
`Generate` and `Stream` control the stream flag, and streaming always requests
final usage. Eino returns one completion, so `n` must be omitted or equal to 1.

To customize the Eino message, return the final `*schema.Message`:

```go
reply, err := cm.Generate(ctx, messages, ark.WithResponseMessageModifier(
    func(ctx context.Context, message *schema.Message, rawBody []byte) (*schema.Message, error) {
        if message.Extra == nil {
            message.Extra = make(map[string]any)
        }
        message.Extra["application"] = "my-assistant"
        return message, nil
    },
))
if err != nil {
    log.Fatal(err)
}
fmt.Println(reply.Content)
```

Response modifiers run before Eino callbacks receive the message, so callbacks
observe the modified output. `rawBody` contains the original response JSON; it is
`nil` for synchronous batch generation because the SDK's batch client does not
expose the raw response body. A Generate modifier must return a non-nil message.

The streaming modifier receives `end == false` for each chunk. Its message can be
nil when a chunk has no standard message projection; the raw JSON is still
available. At EOF, it is called once with `end == true` and nil message/rawBody,
and may return a final message. Returning nil skips a chunk.

A call-specific modifier replaces the corresponding configured modifier. Passing
nil disables it for that call. Returning an error aborts the request or stream.
Modifiers must support concurrent calls when the model is shared.

## Synchronous batch inference

Set `ChatModelConfig.BatchChat` when constructing a model:

```go
batchModel, err := ark.NewChatModel(ctx, &ark.ChatModelConfig{
    APIKey: os.Getenv("ARK_API_KEY"),
    Model:  os.Getenv("ARK_MODEL"),
    BatchChat: &ark.BatchChatConfig{
        EnableBatchChat:            true,
        BatchChatAsyncRetryTimeout: 30 * time.Minute,
    },
})
if err != nil {
    log.Fatal(err)
}
reply, err := batchModel.Generate(ctx, messages)
if err != nil {
    log.Fatal(err)
}
fmt.Println(reply.Content)
```

Import `time` for this example. `Generate` then uses the synchronous batch Chat
Completions endpoint. `BatchChatAsyncRetryTimeout` bounds the complete batch
request and retry lifecycle; `BatchMaxParallel` configures the SDK connection pool.
Batch inference uses its own HTTP connection pool, separate from `HTTPClient`,
and supports non-streaming calls.

## Returned messages

`schema.Message` exposes `Content`, `ReasoningContent`, `ToolCalls`, and
`ResponseMeta` with finish reason, token usage, and log probabilities.
Additional Ark response data is available through these helpers:

| Helper | Result |
| --- | --- |
| `GetArkRequestID` | Completion ID |
| `GetModelName` | Actual model name |
| `GetServiceTier` | Service tier |
| `GetEncryptedContent` | Encrypted reasoning content |
| `GetModerationHitType` | Moderation classification |
| `GetServiceStatus` | Service status, including model fallback information |
| `GetTokenUsageDetails` | Full SDK usage, including modality-specific counts |

Keep the returned assistant message in conversation history to pass its reasoning,
encrypted reasoning, and tool calls back to the API. The adapter integrates with
Eino model callbacks for request, response, error, and usage reporting.

See the [runnable example](examples/generate/main.go) for generation, streaming,
and an optional document FileID.

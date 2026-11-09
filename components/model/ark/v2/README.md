# Ark Chat Completions and Responses

[中文文档](README_zh.md)

An Eino `ToolCallingChatModel` built on the official
[`ark-runtime-go`](https://github.com/volcengine/ark-runtime-go) SDK recommended
by the [Ark installation guide](https://docs.volcengine.com/docs/ark/install-and-upgrade-sdk?lang=en).
It provides Chat Completions generation, streaming, function calling, multimodal
input through Eino messages and model options.
`NewResponsesAPIChatModel` adds the Responses API, including context reuse,
reasoning passback, structured output, and built-in Web Search.
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
forbid, or require a tool call. Pass tool names to `model.WithToolChoice` to limit
that call to a subset of the available tools; unknown names return an error.
Both automatic and forced selection support multiple allowed names.
Forced selection sends `required`; only one explicitly supplied name sends a
named function choice. Callbacks receive the same selected tool definitions.
Request construction treats model configuration and call options as read-only,
so the same options can be reused across turns or concurrent calls. Keep shared
tool schemas and option values immutable while calls are in progress.
`ToolStrict` sets the model-wide default for strict
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

Video FPS helpers take the outer part and use its `Extra`:

```go
videoURL := os.Getenv("ARK_VIDEO_URL")
part := schema.MessageInputPart{
    Type: schema.ChatMessagePartTypeVideoURL,
    Video: &schema.MessageInputVideo{
        MessagePartCommon: schema.MessagePartCommon{URL: &videoURL},
    },
}
ark.SetInputVideoFPS(&part, 1)
```

`GetInputVideoFPS(&part)` reads the configured value. `GetOutputVideoFPS` takes
`*schema.MessageOutputPart` for assistant history.
Nil or non-video parts are ignored. The deprecated `MessagePartCommon.Extra`
is not read for FPS.

For each media part, set one source in `MessagePartCommon`: `URL`, `Base64Data`,
or `FileID`. All base64 inputs require `MIMEType`. Images, videos, and files are
sent as data URLs; audio uses raw base64 and a MIME-derived format. File base64
uses `file_data` (for example, `data:application/pdf;base64,...`) and also requires
`MessageInputFile.Name`.

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
| `WithRequestPayloadModifier` | Serialized request JSON, with the input Eino messages available as context | `Generate`, `Stream` |
| `WithResponseMessageModifier` | Converted `*schema.Message`, with the original response JSON available as context | `Generate` |
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
observe the modified output. `rawBody` contains the original response JSON.
A Generate modifier must return a non-nil message.
Output callback configuration reflects the payload after request modification;
the start callback observes the configuration before modification.

The streaming modifier receives `end == false` for each chunk. Its message can be
nil when a chunk has no standard message projection; the raw JSON is still
available. After the `[DONE]` event, it is called once with `end == true` and nil
message/rawBody, and may return a final message. A premature EOF returns
`io.ErrUnexpectedEOF` without this final call. Returning nil skips a chunk.
If no chunk contains completion choice 0, the stream returns `ErrEmptyResponse`
without successful finalization. A valid choice with empty text is accepted.

A call-specific modifier replaces the corresponding configured modifier. Passing
nil disables it for that call. Returning an error aborts the request or stream.
Modifiers must support concurrent calls when the model is shared.

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

Ark metadata in `Extra` uses scalar values so messages can round-trip through
`encoding/json`, Sonic, and gob. Common token counts are stored only in
`ResponseMeta.Usage`, and reasoning text in `ReasoningContent`. Both usage helpers
combine the current common counts with SDK-specific details in `Extra`, so changes
made by modifiers or stream concatenation are reflected in their results.
SDK-specific counts and service-status fields use flat keys, such as
`ark-responses-usage-details.input_tokens_details.audio_tokens` and
`ark-service-status.model_fallback.fallback_triggered`. Complex provider
attributes and tool details are JSON strings. Usage helpers reconstruct SDK types
after restoration. Usage helpers read common counts from `ResponseMeta.Usage` and
SDK-specific fields from the current `Extra` format. An optional count is set
when its key exists, including an explicit zero. Stream concatenation keeps the
last provided value for each SDK-specific field; an omitted field leaves it
unchanged. Responses history is generated from the canonical output parts and
ToolCalls described below.

Keep the returned assistant message in conversation history to pass its reasoning,
encrypted reasoning, and tool calls back to the API. The adapter integrates with
Eino model callbacks for request, response, error, and usage reporting.

## Runnable examples

Run these commands from `components/model/ark/v2` in the repository checkout:

```sh
export ARK_API_KEY="your-api-key"
export ARK_MODEL="your-model-or-endpoint-id"
go run ./examples/generate
```

Choose a model that supports the example's features. Different examples may need
different model or endpoint IDs. The shared [environment helper](examples/internal/exampleutil/config.go)
validates these variables, applies an overall context timeout, and accepts the
optional `ARK_BASE_URL` override, including the `/api/v3` suffix.

| Example | Demonstrates |
| --- | --- |
| [generate](examples/generate/main.go) | Text generation, request ID, and token usage |
| [stream](examples/stream/main.go) | Thinking/reasoning output, final usage, and `schema.ConcatMessages` |
| [tools](examples/tools/main.go) | Restrict allowed tools, assemble streamed arguments, execute a local addition, and return the result for a final answer |
| [tool_options](examples/tool_options/main.go) | Reuse per-call tool options, strict defaults and overrides, forced subsets, and an unnamed forced choice |
| [multimodal](examples/multimodal/main.go) | Image/audio/video/file input using a URL, local-file base64, or FileID |
| [tool_result](examples/tool_result/main.go) | Return a document FileID in a multimodal tool result with the matching `ToolCallID` |
| [structured_output](examples/structured_output/main.go) | Strict JSON schema output and decoding into a Go struct |
| [modifiers](examples/modifiers/main.go) | Request JSON edits, raw response access, and successful stream finalization |

Run the other examples with `go run ./examples/stream`, `go run ./examples/tools`,
`go run ./examples/structured_output`, or `go run ./examples/modifiers`.

For multimodal input, select a kind and source. The base64 source reads the local
file and encodes it; `-mime` overrides MIME type detection from its extension:

```sh
go run ./examples/multimodal -kind image -source base64 -value ../examples/generate_with_image/eino.png -thinking disabled
go run ./examples/multimodal -kind audio -source base64 -value /path/to/audio.mp3 -mime audio/mpeg
go run ./examples/multimodal -kind video -source url -value "$ARK_VIDEO_URL"
go run ./examples/multimodal -kind file -source file_id -value "$ARK_FILE_ID"
go run ./examples/tool_result -file-id "$ARK_FILE_ID"
```

Supply your own local audio file, reachable video URL, or active FileID. FileID
examples use files uploaded and processed separately through the Ark Files API.
`-prompt` changes the multimodal question. On thinking-capable models, use
`-thinking disabled` for a direct answer or `-thinking enabled` to request
reasoning; omitting it retains the model's default. Audio input requires an
audio-capable model. `-help` lists the available flags.
These programs make real inference calls when pointed at Ark.


## Responses API

Use `NewResponsesAPIChatModel` for `/responses`. The independent constructor
keeps the same Eino `Generate`, `Stream`, `WithTools`, and callback interfaces:

```go
cm, err := ark.NewResponsesAPIChatModel(ctx, &ark.ResponsesAPIConfig{
    APIKey: os.Getenv("ARK_API_KEY"),
    Model: os.Getenv("ARK_MODEL"),
})
if err != nil {
    return err
}
reply, err := cm.Generate(ctx, []*schema.Message{schema.UserMessage("Hello")})
```

`ResponsesAPIConfig` shares connection settings, custom headers, parallel function
calls, and request/response/chunk modifiers with `ChatModelConfig`.
Its `Thinking` and `ReasoningEffort` fields use `responses.Thinking` and
`responses.ReasoningEffort`; override them with `WithResponsesThinking` and
`WithResponsesReasoningEffort`. Chat Completions uses its own SDK types and
`WithThinking` / `WithReasoningEffort` options.
Use `MaxOutputTokens` (or `model.WithMaxTokens`) for its
output limit. `Stop` sequences apply to Chat Completions. Context and
image-generation APIs remain in the legacy module.

- `ResponseFormat` uses `arkruntime/model/responses.TextFormat`. For JSON schema,
  set `type: json_schema`, `name`, `strict`, and the JSON schema in `schema`.
- `EnableToolWebSearch` uses `responses.WebSearchTool`; use
  `WithEnableToolWebSearch(nil)` to disable it for a call. The adapter supplies
  `ark-beta-web-search: true` unless explicitly overridden in custom headers.
  `MaxToolCalls` and
  `WithMaxToolCalls` bound built-in tool rounds as supported by the service.
- Function tools support `ToolStrict`, `WithToolStrict`, call-level tool options,
  and allowed-name filtering. Eino tool calls retain `Type: "function"` and
  `call_id` as `ToolCall.ID`; return results using that ID in `ToolCallID`.
  Explicit allowed names restrict the call to those function tools, excluding
  built-in search. Without names, forced choice can use any configured tool.
- Image/audio/video/file input and multimodal tool results use the same Eino
  parts and URL → Base64Data → FileID precedence as Chat Completions. Responses
  audio base64 is encoded as a MIME data URL in `audio_url`.
- Two independent conversion paths map Responses output → Eino message, and Eino
  message → SDK `responses.InputItem`. It does not reconstruct output items as
  an intermediate request representation or replay raw output JSON.
  For this adapter's own output, `AssistantGenMultiContent` and `ToolCalls` are
  authoritative; `Content` and `ReasoningContent` are display-only compatibility views.
  Edit text/reasoning parts or function values in `ToolCalls` to change history.
  Item metadata is attached to the first data part. `ark.ResponsesOutputItem`
  is needed only when there is no mapped data part, such as an empty message,
  a function call represented in ToolCalls, or an opaque built-in output.
  Item metadata uses `ark-responses-item-type`, `ark-responses-item-id`,
  `ark-responses-item-status`, `ark-responses-item-role`, and `ark-responses-partial`;
  content uses its own `ark-responses-content-type` and `ark-responses-annotations`.
  Output projection reads SDK types directly. Fields not exposed by the SDK,
  including `message.phase` and `action.queries`, are not extracted from raw JSON.
  Built-in or unknown variants without a standard Eino mapping retain the SDK
  value as atomic JSON in `ark-responses-item-data` / `ark-responses-content-data`.
  SDK annotation codecs and unknown-variant fallbacks preserve their own values
  and number precision through JSON/gob persistence.
  These metadata fields are not automatically sent
  as request fields, and mapped text, signatures and function values are not
  duplicated in Extra.
  `ark-responses-item-index` records item ownership and order across parts/tools;
  section/content indices preserve the content array within an item. A reasoning
  summary array is joined with newlines into one display part without segment
  indices; input conversion emits at most one summary_text for that part.
  These persistent indices are independent of stream merge indices. The first
  reasoning part initially carries its item's encrypted `Signature`; input
  conversion accepts it on any reasoning part of that item and rejects
  conflicting nonempty signatures.
  Message, reasoning and function-call history maps directly to SDK input types.
  Output-only items are omitted from subsequent input. Server-side MCP and
  Tool Search configuration and history mapping are outside this adapter's
  supported workflows, even where the SDK declares corresponding input types.
  These SDK output values remain in Message parts for inspection.
  Unknown content without a supported input
  mapping returns an error. History conversion stays within SDK input types;
  the adapter does not replace the entire input to implement unsupported fields.
  SDK-documented extensions remain in use for tool strictness and explicit
  payload modifiers; SDK annotation codecs preserve unknown annotation variants.
  Generate leaves part `StreamingMeta` nil. Streams emit complete parts once at
  the terminal event, with indices identifying their final positions so Concat
  keeps adjacent parts separate. Text, reasoning and function arguments continue
  through existing delta fields; these events do not emit partial output parts.
  A previous response ID supplies cached history without resending it.
  Reasoning parts and their signatures are included automatically.
- System and developer messages can appear between conversation turns. Put
  text in `Content`, or input parts in `UserInputMultiContent`; the adapter
  preserves their position in the message list. Use `ark.Developer` for the
  developer role, for example `&schema.Message{Role: ark.Developer, Content: "Follow project conventions."}`.
- Assistant messages from other models or applications use only standard Eino
  fields. `AssistantGenMultiContent` takes precedence over legacy `MultiContent`
  and `Content`; text, reasoning, image, audio and video parts retain their array
  order. Reasoning parts preserve `Text` and `Signature`; without reasoning parts,
  `ReasoningContent` supplies a reasoning item, including alongside media parts.
  Tool calls retain their array order, IDs, function names and arguments.
  Provider-specific `Extra` is ignored. Streaming merge indices do not define
  ordering between the separate parts and tool-call arrays.
- For this adapter's output, native item/content indices determine original
  output order across parts and tools. Deleting a
  `ToolCall` removes its function item; deleting content parts removes those
  segments. Before deleting the data part that carries item metadata, transfer
  its item-level Extra entries to a retained part; unresolved item references are
  rejected. New parts without native metadata are appended as new input items.
  Invalidate a cached
  message before reusing edited history.
- `GetResponseID` exposes the response ID; `GetArkRequestID` exposes the HTTP
  request ID when returned. `GetResponsesTokenUsageDetails` returns full SDK
  usage. Inspect output parts and their Extra for annotations and built-in tool
  information beyond Eino's common fields.
- Terminal stream events fill text and tool-call fields not already emitted as
  deltas. An empty completed output returns `ErrEmptyResponse`, as in Generate.
  Reasoning display text also receives a missing suffix when one delta channel
  matches a terminal prefix. Mixed channels or a different flattened segment
  order keep the streamed display; complete reasoning remains in output parts.
  When stopping a stream early, cancel its request context as well as closing
  the reader: Eino v0.9.22 does not propagate reader closure to a blocked SDK read.

### Context reuse

For Responses, forced selection uses `required` unless exactly one function name
is explicitly provided. This also applies when only one tool is configured.

The first request filters its tools by the allowed names. Reuse the same options
for cached continuations with either allowed or forced selection, including
multiple names; the server inherits the already filtered set. Cached calls omit
both `tools` and `tool_choice`: the current call's choice cannot force or forbid
inherited tools again. Start a new cache chain to change the set or apply a new
choice. An explicit cache ID does not require local function definitions.

Request construction treats model configuration and call options as read-only.
Omitting cached tool definitions leaves the model's bindings intact, so a later
full-history request can send them again. Callbacks receive the local function
metadata derived for that call; it does not verify the remote tools of an
arbitrary cached response ID.

Set `SessionCache: &ark.SessionCacheConfig{EnableCache: true, TTL: 3600}` to store
responses and automatically reuse the newest unexpired cached message in later
calls. Send the full conversation; the adapter trims it after that message and
sets `previous_response_id`. Read expiry with `GetCacheExpiration`, and call
`InvalidateMessageCaches(messages)` after editing cached history.

Cached continuations inherit their tool definitions from the first request.
The adapter omits `tools`, including its extra-body encoding, and `tool_choice`
whenever it sends `previous_response_id`. To change tool settings, start a new cache
chain; for automatic session caching, invalidate the message caches and resend
the full conversation.

`WithCache(&ark.CacheOption{HeadPreviousResponseID: &id})` explicitly continues a
stored response or prefix. A valid cached message takes precedence over this
head ID. `CreatePrefixCache(ctx, prefix, ttl)` creates a reusable prefix where
supported. Zero TTL leaves expiration to the service. Cache availability must
be enabled for the account and model; a permission error does not prove an
adapter defect.

The [message parts example](examples/responses/message_parts/main.go) edits and
persists canonical output parts before continuing the conversation.

The [tool options example](examples/responses/tool_options/main.go) combines
strict overrides and function-name subsets. The
[forced cached tools example](examples/responses/cached_tools/main.go) reuses the
same Forced multi-name options for addition and multiplication; the
[allowed cached tools example](examples/responses/cached_tool_options/main.go)
reuses unchanged Allowed options through a normal follow-up. Both retain full
history, print cached token usage, and inherit the first request's tool set.
The forced example invalidates local cache references before applying Forbidden
in a new chain for its final answer.

### Streaming and modifiers

Text/reasoning deltas and indexed function arguments can be concatenated with
`schema.ConcatMessages`. Parallel function calls are tracked by output index.
The terminal event supplies response metadata and token usage exactly once.
`response.incomplete` preserves partial output and records its reason;
`response.failed` and `error` return errors. EOF or `[DONE]` without a terminal
response is `io.ErrUnexpectedEOF`.

Request modifiers receive a JSON object with `model` and `input`. Unknown fields,
strict tool settings, and exact JSON numbers survive SDK serialization. `Stream`
and `Generate` retain control of `stream`. Response modifiers receive original
response JSON. Chunk modifiers receive each original event, including events
without a message projection; successful finalization (`end == true`) occurs
once after `response.completed` or `response.incomplete`.

### Runnable examples and verification

[Responses code examples](examples/responses/README.md) are split into one
program per feature. Credentials and inputs come from environment variables;
feature configuration and API calls are shown directly in each `main.go`.

```sh
go run ./examples/responses/generate
go run ./examples/responses/stream
go run ./examples/responses/tools
go run ./examples/responses/structured_output
go run ./examples/responses/reasoning
go run ./examples/responses/modifiers
go run ./examples/responses/file
```

Set `ARK_API_KEY`, `ARK_MODEL`, and optional `ARK_BASE_URL`. Use `ARK_PROMPT` for
an optional question, `ARK_FILE_ID` for an existing document, or `ARK_FILE_PATH`
for a local PDF. The file example defaults to the bundled marker PDF. Built-in
search, cache, and media support depend on the account and model.
The opt-in live tests also upload a temporary PDF and delete it after testing:

```sh
GOWORK=off go test -race ./...
ARK_RESPONSES_LIVE=1 GOWORK=off go test -run TestResponsesLive -v
# Also enable the session test for accounts with the cache service activated:
ARK_RESPONSES_LIVE=1 ARK_RESPONSES_LIVE_CACHE=1 GOWORK=off go test -run TestResponsesLive/session -v
```

The cache and search cases are separate opt-ins (`ARK_RESPONSES_LIVE_CACHE=1`
and `ARK_RESPONSES_LIVE_WEBSEARCH=1`). Multimodal tool-result reading can be tested
with `ARK_RESPONSES_LIVE_MULTIMODAL_TOOLS=1`; it requires model support beyond
accepting the request. The supplied `doubao-seed-2-1-pro-260628` model did not
reliably read a PDF returned by a tool in live validation. Ordinary PDF user input
passed for both base64 and FileID. The adapter retains the documented native
multimodal tool-result request shape; local wire tests cover each media source.

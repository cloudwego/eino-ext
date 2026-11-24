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
| `FrequencyPenalty`, `PresencePenalty`, `LogitBias` | Token selection controls |
| `Thinking`, `ReasoningEffort`, `ResponseFormat` | Thinking mode, reasoning effort, and output format using official SDK types |
| `CustomHeader`, `ServiceTier` | Custom headers and service tier |

Eino options such as `model.WithModel`, `model.WithTemperature`,
`model.WithMaxTokens`, and `model.WithTopP` set values for an individual call.
Ark-specific options include `WithThinking`, `WithReasoningEffort`,
`WithMaxCompletionTokens`, `WithParallelToolCalls`, and `WithCustomHeader`.
`WithCustomHeader` replaces the configured custom header map for that call.

Chat Completions uses Ark's [implicit caching](https://docs.volcengine.com/docs/ark/context-cache?lang=en),
which is automatic for supported models and requires no cache option here.
`Generate` and `Stream` send the full message history to `/chat/completions`;
they do not call Context or Responses endpoints to manage a cache. Server-reported
cache hits appear in `reply.ResponseMeta.Usage.PromptTokenDetails.CachedTokens`.
Explicit session and prefix caching below apply to `ResponsesAPIModel`.

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
A connection closed before `[DONE]` returns `io.ErrUnexpectedEOF`. A completed stream
without choice 0 returns `ErrEmptyResponse`.
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
made by stream concatenation are reflected in their results.
SDK-specific counts and service-status fields use flat keys, such as
`ark-v2-responses-usage-details.input_tokens_details.audio_tokens` and
`ark-v2-service-status.model_fallback.fallback_triggered`. Complex provider
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

Run the other examples with `go run ./examples/stream`, `go run ./examples/tools`,
or `go run ./examples/structured_output`.

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

`ResponsesAPIConfig` shares connection settings and custom headers with `ChatModelConfig`.
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
- Function tools support call-level tool options and allowed-name filtering. Eino tool calls retain `Type: "function"` and
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
  is needed only for items without a standard Eino field, such as an empty
  message or an opaque built-in output. A function call's item metadata belongs
  to its `ToolCalls[i].Extra`, alongside the function values; it creates no
  `AssistantGenMultiContent` part.
  The adapter stores message, part, and tool-call metadata under `ark-v2-` keys.
  Session cache reuse and invalidation operate only on v2 cache metadata.
  Item metadata uses `ark-v2-responses-item-type`, `ark-v2-responses-item-id`,
  `ark-v2-responses-item-status`, `ark-v2-responses-item-role`, and `ark-v2-responses-partial`;
  content uses its own `ark-v2-responses-content-type` and `ark-v2-responses-annotations`.
  Output projection reads SDK types directly. Fields not exposed by the SDK,
  including `message.phase` and `action.queries`, are not extracted from raw JSON.
  Built-in or unknown variants without a standard Eino mapping retain the SDK
  value as atomic JSON in `ark-v2-responses-item-data` / `ark-v2-responses-content-data`.
  SDK annotation codecs and unknown-variant fallbacks preserve their own values
  and number precision through JSON/gob persistence.
  These metadata fields are not automatically sent
  as request fields, and mapped text, signatures and function values are not
  duplicated in Extra.
  `ark-v2-responses-item-index` records item ownership and order across parts/tools;
  each reasoning item produces one reasoning part. Its `Reasoning.Text` joins
  nonempty summaries with newlines, and `Reasoning.Signature` holds its encrypted
  content. `GetRawReasoningContent(&part)` returns the ordered raw text segments
  stored in the part's Extra. History sends the joined summary as one entry
  and each raw segment as a separate `reasoning_text`. Message `output_text`
  retains its text and annotations on a separate text part.
  Message, reasoning and function-call history maps directly to SDK input types.
  Output-only items are omitted from subsequent input. Server-side MCP and
  Tool Search configuration and history mapping are outside this adapter's
  supported workflows, even where the SDK declares corresponding input types.
  These SDK output values remain in Message parts for inspection.
  Unknown content without a supported input
  mapping returns an error. History conversion stays within SDK input types;
  the adapter does not replace the entire input to implement unsupported fields.
  SDK annotation codecs preserve unknown annotation variants.
  Generate leaves part `StreamingMeta` nil. Streams assign one merge index per reasoning item and one per message content part. Each part receives the next
  zero-based index when first emitted; this index is reused, rather than derived
  arithmetically from `output_index`. Function calls have separate zero-based
  `ToolCall.Index` values assigned on first occurrence and reused for all chunks.
  The native `output_index` remains in `ToolCall.Extra` for history ordering;
  function calls allocate no part index. Metadata/signatures are completed
  at item done or the terminal response; complete text is not replayed. Because
  Eino Concat merges adjacent chunks, later parts/items may wait for the current
  item's final metadata. Within a reasoning item, new summary text streams in nondecreasing
  `summary_index` order. Returning to an earlier index with additional text
  fails the stream. Reasoning snapshots supply only missing reasoning suffixes.
  Raw reasoning segments accumulate independently by `content_index`.
  Legacy text and function arguments remain incremental; summary display follows
  the emitted summary part chunks.
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
- For this adapter's output, native item indices order the output items, including
  tool calls. Within each message item, history replay sends content parts in their
  current order in `AssistantGenMultiContent`. Deleting a
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
- Streamed message text comes only from `response.output_text.delta`. A stream
  without text deltas has empty message text. Terminal events still complete tool calls. An empty completed output returns `ErrEmptyResponse`, as in Generate.
  `ReasoningContent` displays only reasoning summaries, with nonempty segments
  separated by newlines. Streams derive this view from summary part chunks, so
  added/delta/done snapshots do not duplicate it. Raw reasoning content and
  signatures remain in output parts and subsequent SDK input; no summary means
  an empty `ReasoningContent`.
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
sets `previous_response_id`. Read the cached response ID and Unix expiration time with `GetResponseCacheRef`, and call
`InvalidateMessageCaches(messages)` after editing cached history.

When switching models, `ark.ClearReasoning(history)` removes reasoning summaries,
raw text, and encrypted content from a copy of the history. Each message, its
top-level Extra map, and its output part slice are copied. Other nested values
are shared. Chat Completions can use the copy directly. For Responses, invalidate
the cache references on the copy to replay the full history and start a new cache
chain when session caching is enabled.

```go
clean := ark.ClearReasoning(history)
if err := ark.InvalidateMessageCaches(clean); err != nil {
	return nil, err
}
return next.Generate(ctx, clean)
```

Cached continuations inherit their tool definitions from the first request.
The adapter omits `tools` and `tool_choice`
whenever it sends `previous_response_id`. To change tool settings, start a new cache
chain; for automatic session caching, invalidate the message caches and resend
the full conversation.

`WithCache(ark.CacheOption{HeadPreviousResponseID: &id})` explicitly continues a
stored response or prefix. A valid cached message takes precedence over this
head ID. `CreatePrefixCache(ctx, prefix, ttl)` creates a reusable prefix where
supported. Zero TTL leaves expiration to the service. Cache availability must
be enabled for the account and model; a permission error does not prove an
adapter defect.

The [message parts example](examples/responses/message_parts/main.go) edits and
persists canonical output parts before continuing the conversation.

The [tool options example](examples/responses/tool_options/main.go) demonstrates
allowed and forced function-name subsets. The
[forced cached tools example](examples/responses/cached_tools/main.go) reuses the
same Forced multi-name options for addition and multiplication; the
[allowed cached tools example](examples/responses/cached_tool_options/main.go)
reuses unchanged Allowed options through a normal follow-up. Both retain full
history, print cached token usage, and inherit the first request's tool set.
The forced example invalidates local cache references before applying Forbidden
in a new chain for its final answer.

### Streaming

Text/reasoning deltas and indexed function arguments can be concatenated with
`schema.ConcatMessages`. Parallel function calls are tracked by output index.
The terminal event supplies response metadata and token usage exactly once.
`response.incomplete` preserves partial output and records its reason;
`response.failed` and `error` return errors. EOF or `[DONE]` without a terminal
response is `io.ErrUnexpectedEOF`.

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
go run ./examples/responses/file
```

Set `ARK_API_KEY`, `ARK_MODEL`, and optional `ARK_BASE_URL`. Use `ARK_PROMPT` for
an optional question, `ARK_FILE_ID` for an existing document, or `ARK_FILE_PATH`
for a local PDF; the file example requires one of these document inputs. Built-in
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

## Source layout

| Files | Responsibility |
| --- | --- |
| `chat_model.go` | Chat Completions configuration, call options, Generate, and Stream |
| `chat_request.go`, `chat_content.go` | Chat request construction, tools, and content conversion |
| `chat_response.go` | Chat response projection, usage, and metadata |
| `responses_api.go`, `responses_request.go` | Responses configuration, call options, API calls, and request construction |
| `responses_content.go`, `responses_history.go` | Standard message conversion and native Responses history reconstruction |
| `responses_projection.go`, `responses_stream.go`, `responses_cache.go` | Responses output projection, stream assembly, and cache management |
| `client.go`, `request_common.go`, `option.go`, `message_extra.go` | Shared transport, request helpers, call options, and message metadata |

Tests use the same API prefixes and group related scenarios by function. Shared
history and metadata tests exercise both implementations. Both APIs remain in package `ark`.

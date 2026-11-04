# Ark Chat Completions 与 Responses 模型

[English](README.md)

本包基于[方舟官方接入文档](https://docs.volcengine.com/docs/ark/install-and-upgrade-sdk?lang=zh)
推荐的 [`ark-runtime-go`](https://github.com/volcengine/ark-runtime-go) SDK，
实现 Eino `ToolCallingChatModel`，通过 Eino 消息和模型选项提供 Chat Completions
生成、流式输出、函数调用、多模态输入；`NewResponsesAPIChatModel` 另提供
Responses API、缓存续接、推理回传和内置搜索。鉴权、HTTP 请求和重试使用官方 SDK。

Go 版本要求和依赖版本以 [go.mod](go.mod) 为准。
模型参数和能力要求参见 [Chat API 文档](https://docs.volcengine.com/docs/ark/chat-api?lang=zh)。

## 安装

```sh
go get github.com/cloudwego/eino-ext/components/model/ark/v2
```

在应用环境中配置 `ARK_API_KEY` 和 `ARK_MODEL`，使用当前方舟账号及地区可访问的
API key、模型 ID 或推理接入点 ID。

## 快速开始

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
    messages := []*schema.Message{{Role: schema.User, Content: "你好"}}
    reply, err := cm.Generate(ctx, messages)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(reply.Content)
}
```

通过 `ChatModelConfig` 配置服务连接和生成参数：

| 配置 | 用途 |
| --- | --- |
| `APIKey` | API key 鉴权 |
| `AccessKey`、`SecretKey` | AK/SK 鉴权；同时提供 `APIKey` 时优先使用 API key |
| `Model` | 模型 ID 或推理接入点 ID |
| `BaseURL`、`Region` | 服务地址和地区 |
| `Timeout`、`HTTPClient`、`RetryTimes` | 请求超时、自定义 HTTP 客户端和重试次数；提供 `HTTPClient` 时其超时配置优先于 `Timeout` |
| `MaxTokens`、`MaxCompletionTokens` | 输出 token 限制，按模型要求选择参数 |
| `Temperature`、`TopP`、`Stop` | 采样参数和停止词 |
| `FrequencyPenalty`、`PresencePenalty`、`RepetitionPenalty`、`LogitBias` | token 选择控制 |
| `Thinking`、`ReasoningEffort`、`ResponseFormat` | 使用官方 SDK 类型配置思考模式、推理强度和输出格式 |
| `CustomHeader`、`ServiceTier`、`User` | 自定义请求头、服务等级和终端用户标识 |

单次调用可以使用 Eino 的 `model.WithModel`、`model.WithTemperature`、
`model.WithMaxTokens`、`model.WithTopP` 等选项。
本包还提供 `WithThinking`、`WithReasoningEffort`、`WithMaxCompletionTokens`、
`WithParallelToolCalls` 和 `WithCustomHeader`。
`WithCustomHeader` 会替换该次调用的自定义请求头配置。

## 流式输出

使用上面创建的模型和消息，在 import 中加入 `errors` 和 `io`：

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

流式消息按增量返回正文、思考内容和工具调用。收集消息后，可以使用
`schema.ConcatMessages` 拼接。流式请求会请求最终 token 用量，该信息可能在没有
正文的尾包中返回。使用结束后关闭 reader；提前停止消费时，同时取消请求 context，
及时结束正在等待下一个数据块的 HTTP 读取。

## 函数调用

使用 `WithTools` 绑定 Eino 工具定义：

```go
bound, err := cm.WithTools([]*schema.ToolInfo{
    {
        Name: "get_weather",
        Desc: "查询城市天气",
        ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
            "city": {Type: schema.String, Required: true},
        }),
    },
})
if err != nil {
    log.Fatal(err)
}

reply, err := bound.Generate(ctx, []*schema.Message{
    {Role: schema.User, Content: "北京今天天气如何？"},
})
if err != nil {
    log.Fatal(err)
}
for _, call := range reply.ToolCalls {
    fmt.Printf("%s: %s\n", call.Function.Name, call.Function.Arguments)
}
```

模型通过 `reply.ToolCalls` 返回函数调用请求，由应用执行工具。把 assistant 消息和
每个调用对应的 `schema.Tool` 结果追加到对话后，再调用绑定工具的模型。
结果消息的 `ToolCallID` 必须对应原工具调用 ID，工具结果可包含文本或多模态内容。

使用 `model.WithTools` 设置单次调用的工具定义，使用 `model.WithToolChoice` 控制
允许、禁止或强制工具调用。向 `model.WithToolChoice` 传入工具名可将本次调用限制在
指定的工具子集内，未知名称会返回错误；自动和强制选择均支持多个候选工具名。
`ToolStrict` 设置模型级的严格 schema 默认值；对单个
`*schema.ToolInfo`，使用 `ark.WithToolStrict(tool, true)` 或
`ark.WithToolStrict(tool, false)` 覆盖默认值，该函数会返回工具定义的副本。
模型默认值和工具覆盖值都未设置时，不发送 `strict` 字段。启用严格模式时，schema
需满足目标模型的要求。对于支持并行工具调用的模型，使用 `ParallelToolCalls` 或
`WithParallelToolCalls` 控制单次响应是否允许返回多个工具调用。

## 多模态输入与 FileID

在 `schema.User` 或 `schema.Tool` 消息中使用 `UserInputMultiContent`：

| Eino part 类型 | 内容字段 | 附加配置 |
| --- | --- | --- |
| `schema.ChatMessagePartTypeText` | `Text` | 文本内容 |
| `schema.ChatMessagePartTypeImageURL` | `Image` | `Detail` 控制图片细节 |
| `schema.ChatMessagePartTypeAudioURL` | `Audio` | `MIMEType` 标识音频格式 |
| `schema.ChatMessagePartTypeVideoURL` | `Video` | `SetInputVideoFPS` 设置抽帧频率 |
| `schema.ChatMessagePartTypeFileURL` | `File` | `Name` 提供文件名 |

每个媒体 part 在 `MessagePartCommon` 中选择一个来源：`URL`、`Base64Data` 或
`FileID`。所有 base64 输入均需提供 `MIMEType`。图片、视频和文件发送时转换为
data URL；音频使用原始 base64 并根据 MIME 类型设置格式。文件 base64 通过
`file_data` 发送，例如 `data:application/pdf;base64,...`，还必须设置 `MessageInputFile.Name`。

例如，传入已通过方舟 Files API 上传的文档：

```go
fileID := os.Getenv("ARK_FILE_ID")
messages := []*schema.Message{
    {
        Role: schema.User,
        UserInputMultiContent: []schema.MessageInputPart{
            {Type: schema.ChatMessagePartTypeText, Text: "总结这份文档。"},
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

文件上传和处理使用官方 SDK 的 Files API。发起推理前，确保文件处于 active 状态，
与 API key 属于同一项目，且文件类型受目标模型支持。本包将 FileID 传入 Chat Completions。

工具也可以使用 assistant 返回的调用 ID 回传该文档内容：

```go
toolResult := &schema.Message{
    Role:                  schema.Tool,
    ToolCallID:            toolCallID,
    UserInputMultiContent: messages[0].UserInputMultiContent,
}
```

其中 `toolCallID` 是执行后产生该文档结果的工具调用 ID。

## 官方 SDK 参数

`Thinking`、`ReasoningEffort` 和 `ResponseFormat` 使用
`github.com/volcengine/ark-runtime-go/arkruntime/model/chat` 中的类型，
通过 SDK 的 `NewOpt*` 构造器设置可选值。例如：

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

## 请求与响应定制

可以在 `ChatModelConfig` 中配置 modifier，也可以按单次调用指定：

| 选项 | 修改对象 | 适用范围 |
| --- | --- | --- |
| `WithRequestPayloadModifier` | 序列化后的请求 JSON，同时可读取输入的 Eino 消息 | `Generate`、`Stream` |
| `WithResponseMessageModifier` | 转换后的 `*schema.Message`，同时可读取原始响应 JSON | `Generate` |
| `WithResponseChunkMessageModifier` | 转换后的消息分片，同时可读取原始分片 JSON | `Stream` |

定制请求 Payload 时，导入 `encoding/json` 使用 `json.RawMessage`，
通过 `github.com/bytedance/sonic` 进行 JSON 编解码：

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

modifier 在请求转换、序列化后执行一次，随后由 SDK 发送或重试请求，可用于添加生成式
SDK 类型尚未暴露的字段。返回值必须是 JSON 对象，并保留字符串类型的 `model` 字段及 `messages` 字段。`Generate` 和 `Stream` 负责设置流式标志，流式请求会请求最终 usage。
Eino 每次返回一个 completion，因此 `n` 应不设置或设为 1。

定制响应消息时，返回处理后的 `*schema.Message`：

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

响应 modifier 在 Eino 回调收到消息之前执行，回调和调用方看到的是处理后的消息。
`rawBody` 是原始响应 JSON。`Generate` 的 modifier 必须返回非 nil 的消息。

流式 modifier 处理每个分片时 `end == false`。当分片没有标准 Eino 消息投影时，
消息可能为 nil，但仍可读取原始 JSON。收到 `[DONE]` 事件后会额外调用一次，
此时 `end == true`，消息和 rawBody 均为 nil，可以返回一条结束消息。
提前遇到 EOF 会返回 `io.ErrUnexpectedEOF`，不触发结束调用。返回 nil 表示跳过该分片。

调用级 modifier 会替换初始化配置中的同名 modifier，传入 nil 可为本次调用禁用它。
返回错误会中止请求或流。模型并发使用时，modifier 也应支持并发调用。

## 返回信息

`schema.Message` 通过 `Content`、`ReasoningContent`、`ToolCalls` 和 `ResponseMeta`
提供正文、思考内容、工具调用、结束原因、token 用量和 log probabilities。
本包还提供以下辅助函数读取 Ark 返回信息：

| 函数 | 返回信息 |
| --- | --- |
| `GetArkRequestID` | Completion ID |
| `GetModelName` | 实际模型名称 |
| `GetReasoningContent` | 当前消息的思考内容，兼容旧 Extra 元数据 |
| `GetServiceTier` | 服务等级 |
| `GetEncryptedContent` | 加密思考内容 |
| `GetModerationHitType` | 内容审核分类 |
| `GetServiceStatus` | 服务状态，包括模型回退信息 |
| `GetTokenUsageDetails` | SDK 完整用量，包括各模态 token 明细 |

`Extra` 中的方舟元数据使用原子值保存，支持 `encoding/json`、Sonic 和 gob 编解码往返。
通用 token 计数仅保存在 `ResponseMeta.Usage`，思考文本保存在 `ReasoningContent`。
两个 usage getter 将当前通用统计与 `Extra` 中的 SDK 专有明细合并返回，
因此 modifier 或流拼接对通用统计的修改也会反映在 getter 结果中。
SDK 专有计数和服务状态平铺为独立 key，例如 `ark-responses-usage-details.input_tokens_details.audio_tokens`
和 `ark-service-status.model_fallback.fallback_triggered`。结构可变的 Responses output
和工具明细保存为 JSON 字符串，辅助函数在恢复后还原 SDK 类型。旧的元数据注册类型继续支持读取。
Responses 推理回传使用保存的完整 output，不再重复保存一份 reasoning。

多轮对话中保留返回的 assistant 消息，即可向 API 回传其中的思考内容、加密思考内容
和工具调用。本包集成 Eino 模型回调，提供请求、响应、错误和用量信息。

## 可运行示例

在仓库的 `components/model/ark/v2` 目录下运行：

```sh
export ARK_API_KEY="your-api-key"
export ARK_MODEL="your-model-or-endpoint-id"
go run ./examples/generate
```

请选择支持相应能力的模型，不同示例可能需要使用不同的模型或接入点 ID。
共用的[环境配置辅助函数](examples/internal/exampleutil/config.go) 会检查环境变量、
为完整流程设置 context 超时，并支持可选的 `ARK_BASE_URL`，其中应包含 `/api/v3` 后缀。

| 示例 | 演示内容 |
| --- | --- |
| [generate](examples/generate/main.go) | 文本生成、请求 ID 和 token 用量 |
| [stream](examples/stream/main.go) | 思考过程输出、最终用量及 `schema.ConcatMessages` 拼接 |
| [tools](examples/tools/main.go) | 限制候选工具、拼接流式参数、执行本地加法、回传结果并生成最终回答 |
| [multimodal](examples/multimodal/main.go) | 图片、音频、视频和文件，支持 URL、本地文件 base64 或 FileID |
| [tool_result](examples/tool_result/main.go) | 在多模态工具结果中回传文档 FileID，并匹配 `ToolCallID` |
| [structured_output](examples/structured_output/main.go) | 严格 JSON schema 输出并解析为 Go 结构体 |
| [modifiers](examples/modifiers/main.go) | 修改请求 JSON、读取原始响应以及流正常完成时的处理 |

其余示例可通过 `go run ./examples/stream`、`go run ./examples/tools`、
`go run ./examples/structured_output` 或 `go run ./examples/modifiers` 运行。

多模态示例通过参数选择类型和来源。`base64` 来源会读取本地文件并编码，
`-mime` 可以覆盖根据文件扩展名推断的 MIME 类型：

```sh
go run ./examples/multimodal -kind image -source base64 -value ../examples/generate_with_image/eino.png -thinking disabled
go run ./examples/multimodal -kind audio -source base64 -value /path/to/audio.mp3 -mime audio/mpeg
go run ./examples/multimodal -kind video -source url -value "$ARK_VIDEO_URL"
go run ./examples/multimodal -kind file -source file_id -value "$ARK_FILE_ID"
go run ./examples/tool_result -file-id "$ARK_FILE_ID"
```

请提供自己的本地音频文件、可访问的视频 URL 或已处理完成的 FileID。
FileID 示例需要先通过 Ark Files API 上传文件并等待处理完成。
`-prompt` 可修改多模态问题。支持思考的模型可用 `-thinking disabled` 直接回答，
或用 `-thinking enabled` 请求思考；不传此参数时保留模型默认配置。
音频输入需要选择支持音频的模型，`-help` 可查看所有参数。
连接到 Ark 时，这些程序会发起真实推理请求。


## Responses API

使用 `NewResponsesAPIChatModel` 调用 `/responses`，仍然实现 Eino 的
`Generate`、`Stream`、`WithTools` 和 callback 接口：

```go
cm, err := ark.NewResponsesAPIChatModel(ctx, &ark.ResponsesAPIConfig{
    APIKey: os.Getenv("ARK_API_KEY"),
    Model: os.Getenv("ARK_MODEL"),
})
if err != nil {
    return err
}
reply, err := cm.Generate(ctx, []*schema.Message{schema.UserMessage("你好")})
```

`ResponsesAPIConfig` 与 `ChatModelConfig` 共享连接配置、思考和推理强度、请求头、
并行工具调用和请求/响应/流式 modifier。输出上限使用 `MaxOutputTokens`，调用时可用
`model.WithMaxTokens` 覆盖。停止序列仅用于 Chat Completions。
Context 和图片生成 API 仍使用旧模块。

- `ResponseFormat` 使用 `arkruntime/model/responses.TextFormat`。JSON schema 的
  `type` 为 `json_schema`，`name`、`strict` 和 `schema` 直接放在 format 中。
- `EnableToolWebSearch` 使用 `responses.WebSearchTool`；单次调用可通过
  `WithEnableToolWebSearch(nil)` 关闭。适配器默认发送 `ark-beta-web-search: true`，
  可用自定义请求头覆盖。`MaxToolCalls` / `WithMaxToolCalls` 设置内置工具轮次上限。
- 函数工具支持 `ToolStrict`、`WithToolStrict`、调用级工具配置和候选名称筛选。
  Eino 工具调用的 `Type` 为 `function`，`ToolCall.ID` 保存 API 的 `call_id`；
  回传结果时将该值放入 `ToolCallID`。
- 图片、音频、视频、文件及多模态工具结果沿用相同 Eino 消息结构与
  URL → Base64Data → FileID 优先级。Responses 音频 base64 使用 MIME data URL，
  放入 `audio_url`。
- `EnableReasoningContentPassback` 保留并回传推理 item，包括加密内容。
  已有 previous response ID 时无需重复回传。需要账号与模型支持。
- `GetResponseID` 读取响应 ID；`GetArkRequestID` 读取服务返回的 HTTP 请求 ID。
  `GetResponsesTokenUsageDetails` 读取完整 SDK 用量，`GetResponsesOutput` 读取最终
  输出 item，保留注释、内置工具结果等通用消息字段之外的信息。

### 缓存与上下文续接

通过 `SessionCache: &ark.SessionCacheConfig{EnableCache: true, TTL: 3600}` 存储响应。
后续传入完整历史时，适配器寻找最近的未过期缓存消息，仅发送其后的增量，
并设置 `previous_response_id`。`GetCacheExpiration` 读取过期时间；编辑缓存历史后，
使用 `InvalidateMessageCaches(messages)` 使相关消息的缓存失效。

`WithCache(&ark.CacheOption{HeadPreviousResponseID: &id})` 可以显式续接已存储的响应或
前缀；历史中有效的缓存消息优先于此 ID。`CreatePrefixCache(ctx, prefix, ttl)` 用于
服务支持的前缀缓存。TTL 为零时采用服务默认过期时间。缓存服务需要为账号和模型
开通；服务权限错误不能视为适配器错误。

### 流式事件与 modifier

文本、推理增量和带索引的工具参数可用 `schema.ConcatMessages` 拼接。并行工具调用
按输出索引区分，终止事件只发送一次响应元数据和 token 用量。
`response.incomplete` 保留部分输出并记录结束原因；`response.failed` 和 `error`
返回错误。未收到终止响应就遇到 EOF 或 `[DONE]`，返回 `io.ErrUnexpectedEOF`。

请求 modifier 收到包含 `model` 和 `input` 的 JSON 对象，保留 SDK 未识别的字段、
工具 strict 设置和 JSON 数字精度。`Generate` / `Stream` 控制 `stream` 字段。
响应 modifier 收到原始响应 JSON；流式 modifier 收到每个原始事件，包括没有通用消息
投影的事件。`response.completed` / `response.incomplete` 后调用一次 `end == true`。

### 运行示例与验证

[Responses 代码示例](examples/responses/README.md) 按功能拆成独立程序，
从环境变量读取凭证和输入；每个 `main.go` 直接展示功能配置、消息结构和 API 调用。
[示例运行记录](examples/responses/VALIDATION.md) 保存独立运行结果与验证边界。

```sh
go run ./examples/responses/generate
go run ./examples/responses/stream
go run ./examples/responses/tools
go run ./examples/responses/structured_output
go run ./examples/responses/reasoning
go run ./examples/responses/modifiers
go run ./examples/responses/file
```

设置 `ARK_API_KEY`、`ARK_MODEL` 和可选的 `ARK_BASE_URL`。可用 `ARK_PROMPT` 修改问题，
用 `ARK_FILE_ID` 指定已有文档，或用 `ARK_FILE_PATH` 指定本地 PDF；文件示例默认使用
仓库中的标记 PDF。内置搜索、缓存与各类媒体输入需要账号和模型支持。
显式启用的实测还会上传临时 PDF，并在测试后删除：

```sh
GOWORK=off go test -race ./...
ARK_RESPONSES_LIVE=1 GOWORK=off go test -run TestResponsesLive -v
# 已开通缓存服务的账号可额外启用会话缓存测试：
ARK_RESPONSES_LIVE=1 ARK_RESPONSES_LIVE_CACHE=1 GOWORK=off go test -run TestResponsesLive/session -v
```

缓存和搜索测试分别通过 `ARK_RESPONSES_LIVE_CACHE=1` 与
`ARK_RESPONSES_LIVE_WEBSEARCH=1` 启用。多模态工具结果阅读测试使用
`ARK_RESPONSES_LIVE_MULTIMODAL_TOOLS=1`，需要模型真正理解工具结果中的媒体。
实测 `doubao-seed-2-1-pro-260628` 未能可靠读出工具返回 PDF 的内容；普通用户输入的
PDF base64 / FileID 均已通过。适配器保留官方文档的原生多模态工具结果结构，
本地 HTTP 测试覆盖各类媒体及来源的序列化。

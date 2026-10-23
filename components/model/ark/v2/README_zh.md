# Ark Chat Completions 模型

[English](README.md)

本包基于[方舟官方接入文档](https://docs.volcengine.com/docs/ark/install-and-upgrade-sdk?lang=zh)
推荐的 [`ark-runtime-go`](https://github.com/volcengine/ark-runtime-go) SDK，
实现 Eino `ToolCallingChatModel`，通过 Eino 消息和模型选项提供 Chat Completions
生成、流式输出、函数调用、多模态输入和同步批量推理。鉴权、HTTP 请求和重试使用官方 SDK。

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
`FileID`。图片和视频的 base64 需要提供 `MIMEType`，发送时转换为 data URL；音频使用
原始 base64 并根据 MIME 类型设置格式；文件 base64 通过 `file_data` 发送，必须设置
`MessageInputFile.Name`。

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
| `WithRequestPayloadModifier` | 序列化后的请求 JSON，同时可读取输入的 Eino 消息 | `Generate`、`Stream`、同步批量生成 |
| `WithResponseMessageModifier` | 转换后的 `*schema.Message`，同时可读取原始响应 JSON | `Generate`，包括同步批量生成 |
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
`rawBody` 是原始响应 JSON；同步批量生成时，由于 SDK 的批量客户端不暴露原始响应体，
该参数为 `nil`。`Generate` 的 modifier 必须返回非 nil 的消息。

流式 modifier 处理每个分片时 `end == false`。当分片没有标准 Eino 消息投影时，
消息可能为 nil，但仍可读取原始 JSON。收到 `[DONE]` 事件后会额外调用一次，
此时 `end == true`，消息和 rawBody 均为 nil，可以返回一条结束消息。
提前遇到 EOF 会返回 `io.ErrUnexpectedEOF`，不触发结束调用。返回 nil 表示跳过该分片。

调用级 modifier 会替换初始化配置中的同名 modifier，传入 nil 可为本次调用禁用它。
返回错误会中止请求或流。模型并发使用时，modifier 也应支持并发调用。

## 同步批量推理

创建模型时设置 `ChatModelConfig.BatchChat`：

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

该示例需导入 `time`。配置后，`Generate` 使用同步批量 Chat Completions 接口。
`BatchChatAsyncRetryTimeout` 限制整个批量请求及重试过程的时间，
`BatchMaxParallel` 配置 SDK 的连接池并发上限。批量推理使用独立于 `HTTPClient` 的
HTTP 连接池，支持非流式调用。

## 返回信息

`schema.Message` 通过 `Content`、`ReasoningContent`、`ToolCalls` 和 `ResponseMeta`
提供正文、思考内容、工具调用、结束原因、token 用量和 log probabilities。
本包还提供以下辅助函数读取 Ark 返回信息：

| 函数 | 返回信息 |
| --- | --- |
| `GetArkRequestID` | Completion ID |
| `GetModelName` | 实际模型名称 |
| `GetServiceTier` | 服务等级 |
| `GetEncryptedContent` | 加密思考内容 |
| `GetModerationHitType` | 内容审核分类 |
| `GetServiceStatus` | 服务状态，包括模型回退信息 |
| `GetTokenUsageDetails` | SDK 完整用量，包括各模态 token 明细 |

多轮对话中保留返回的 assistant 消息，即可向 API 回传其中的思考内容、加密思考内容
和工具调用。本包集成 Eino 模型回调，提供请求、响应、错误和用量信息。

[可运行示例](examples/generate/main.go) 包含普通生成、流式输出和可选的文档 FileID。

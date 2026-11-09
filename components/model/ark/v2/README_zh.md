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

视频 FPS helper 接收外层 part，并读写它的 `Extra`：

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

`GetInputVideoFPS(&part)` 读取配置值。`GetOutputVideoFPS` 接收
`*schema.MessageOutputPart`，用于助手视频历史。
nil 或非视频 part 会忽略；FPS 不再从废弃的 `MessagePartCommon.Extra` 读取。

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
和 `ark-service-status.model_fallback.fallback_triggered`。复杂的 provider 属性和工具明细
保存为 JSON 字符串，用量辅助函数在恢复后还原 SDK 类型。
usage getter 只读取 `ResponseMeta.Usage` 和当前 `Extra` 格式。
可选计数通过 key 是否存在区分未设置和显式零值；流拼接保留每个 SDK 专有字段
最后一次提供的值，后续缺失字段不会清空此前的值。
Responses 历史根据下文所述的输出 parts 和 ToolCalls 构造。

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

`ResponsesAPIConfig` 与 `ChatModelConfig` 共享连接配置、请求头、并行工具调用和
请求/响应/流式 modifier。其 `Thinking`、`ReasoningEffort` 分别使用
`responses.Thinking`、`responses.ReasoningEffort`，单次调用通过
`WithResponsesThinking`、`WithResponsesReasoningEffort` 覆盖。
Chat Completions 使用自己的 SDK 类型及 `WithThinking` / `WithReasoningEffort`。
输出上限使用 `MaxOutputTokens`，调用时可用
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
- 转换链为 Responses output → Eino Message，以及 Eino Message → SDK
  `responses.InputItem`；不反向还原 output 作为请求中间表示，也不直接回放 raw output。
  本适配器自身的输出以 `AssistantGenMultiContent` 和 `ToolCalls` 为准；`Content`、
  `ReasoningContent` 仅为展示用兼容视图。修改文本/推理 part 或函数值即可编辑历史。
  item 元数据合入首个 data part；仅没有映射 data part 时使用独立的
  `ark.ResponsesOutputItem`，例如空消息、由 ToolCalls 承载的函数调用和内置工具输出。
  item 使用 `ark-responses-item-type`、`ark-responses-item-id`、`ark-responses-item-status`、
  `ark-responses-item-role`、`ark-responses-partial`；content 使用独立的
  `ark-responses-content-type` 和 `ark-responses-annotations`。
  输出直接读取 SDK 类型，不从 raw JSON 补取 SDK 未声明的 `message.phase`、
  `action.queries` 等字段。没有标准 Eino 映射的内置工具或未知变体，仅将 SDK 值
  以原子 JSON 保存在 `ark-responses-item-data` / `ark-responses-content-data`。
  SDK annotations codec 和未知变体兼容机制保留自身值，经 JSON/gob 持久化保留数字精度。
  这些输出元数据不会自动成为请求字段；文本、签名和函数值只保存在公开字段。
  `ark-responses-item-index` 保存 parts/tools 的 item 归属和顺序，section/content-index
  保存 item 内的 content 数组，独立于流式合包索引。reasoning.summary 按换行符合成
  一个展示 part，不保留分段索引；输入转换至多生成一个 summary_text。
  初始投影时，首个 reasoning part 保存该 item 的加密 `Signature`；输入转换读取
  同一 item 任意 reasoning part 的签名，非空签名冲突则报错。
  消息、推理和函数调用历史直接映射到 SDK 输入类型。输出专有的内置工具 item 留在
  Message 中供读取，不作为未定义输入发送；没有支持的输入映射的未知内容 part 会报错。
  历史转换限于 SDK 输入类型，不通过整体替换 input 补充 SDK 未支持的字段。
  工具 strict 和显式 payload modifier 继续使用 SDK 已声明的扩展能力；annotations
  使用 SDK codec 自带的未知变体保留机制。
  Generate 的 part `StreamingMeta` 为 nil。流式完整 parts 在终止事件一次性发出，
  索引表示最终位置，防止 Concat 合并相邻 parts；正文、推理和函数参数保持增量输出，
  这些增量事件不输出局部 output parts。
  previous response ID 提供缓存历史，无需重发历史内容。
  reasoning parts 及其签名自动回传。
- system 和 developer 消息可位于对话中间；文本放入 `Content`，输入内容块放入
  `UserInputMultiContent`。适配器保留它们在消息列表中的位置。
  developer 角色使用 `ark.Developer`，例如
  `&schema.Message{Role: ark.Developer, Content: "遵循项目规范。"}`。
- 其他模型或应用构造的助手消息只依赖 Eino 标准字段。`AssistantGenMultiContent` 优先于
  旧 `MultiContent` 和 `Content`；文本、推理、图片、音频和视频 parts 保持数组顺序。
  reasoning parts 保留 `Text` 和 `Signature`；没有 reasoning part 时，用 `ReasoningContent`
  生成推理 item，与媒体 parts 共存时也会保留。工具调用保持数组顺序、ID、函数名和参数。
  此路径不读取模型私有 `Extra`；流式合包索引不代表 parts 与工具调用两个数组之间的顺序。
- 本适配器自身的输出通过原生 item/content 索引确定 parts 和 tools 的原始顺序；删除 `ToolCall` 会删除
  对应函数 item，删除内容 part 会删除对应分段。删除承载 item 元数据的 data part 前，
  应先将其 item 级 Extra 转移到保留的 part；缺少 item 元数据的引用会报错。
  新增且没有原生元数据的 part 会追加为新的输入 item。编辑缓存历史后，应先使其缓存失效。
- `GetResponseID` 读取响应 ID；`GetArkRequestID` 读取服务返回的 HTTP 请求 ID。
  `GetResponsesTokenUsageDetails` 读取完整 SDK 用量；输出 parts 和 Extra 提供注释、
  内置工具结果等通用字段之外的信息。
- 流式终止事件会补齐 delta 尚未发出的正文与工具调用；completed 的空输出与
  Generate 一样返回 `ErrEmptyResponse`。单一推理渠道与终态文本前缀匹配时，也会补齐
  展示字段缺少的后缀；混合渠道或分段压平后顺序不同时保留已输出的展示文本，完整推理
  仍保存在输出 parts 中。提前停止读取时，同时取消请求 context 并
  关闭 reader；Eino v0.9.22 的 reader 关闭不会中断 SDK 中阻塞的读取。

[message parts 示例](examples/responses/message_parts/main.go) 演示编辑输出 parts、
经 JSON 持久化后继续对话，聚合兼容字段的修改不会覆盖 parts。

### 缓存与上下文续接

Responses 的强制选择仅在显式提供一个函数名称时按名称调用，其余情况使用
`required`；即使只配置了一个工具，也不会自动推断函数名称。

首轮会按候选名称筛选工具。后续缓存请求可复用相同 options，允许或强制选择均支持
多个候选名称，服务端直接继承已经筛选的集合。缓存请求中的候选名称不会再次筛选
这个集合。续接会省略 tools 和 tool_choice，本轮选项不能重新强制或禁止继承的工具；
需要更换集合或应用新的工具策略时应建立新的缓存链。显式缓存 ID 无需重复本地函数定义。
非缓存调用显式指定允许的函数名时，
内置搜索也会排除；没有名称限制时，强制调用可选择任一已配置工具。

请求构造只读模型配置和本次 options。省略缓存工具定义不会清空模型绑定，
后续完整历史请求仍可重新发送工具。回调提供本次推导的本地函数资料，
不代表已核验任意缓存响应 ID 在远端保存的工具集合。

通过 `SessionCache: &ark.SessionCacheConfig{EnableCache: true, TTL: 3600}` 存储响应。
后续传入完整历史时，适配器寻找最近的未过期缓存消息，仅发送其后的增量，
并设置 `previous_response_id`。`GetCacheExpiration` 读取过期时间；编辑缓存历史后，
使用 `InvalidateMessageCaches(messages)` 使相关消息的缓存失效。

缓存续接继承首轮工具定义。发送 `previous_response_id` 时，适配器会省略 `tools`
（包括额外请求体中的工具编码）和 `tool_choice`。需要更换工具设置时应建立新的缓存链；
自动会话缓存可先使消息缓存失效，再发送完整对话。

[工具配置示例](examples/responses/tool_options/main.go) 展示 strict 覆盖值与候选名称
筛选。[强制缓存工具示例](examples/responses/cached_tools/main.go) 用相同的 Forced
多名称 options 完成加法和乘法；[允许缓存工具示例](examples/responses/cached_tool_options/main.go)
保持 Allowed options 不变，直到普通下一轮。两者都保留完整历史、打印缓存 token 数，
续接继承首轮工具集合。强制调用示例在最终使用 Forbidden 前使本地缓存引用失效，
通过完整历史建立新链，让新的策略生效。

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

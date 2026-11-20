# OrcaRouter

[Eino](https://github.com/cloudwego/eino) 的 [OrcaRouter](https://www.orcarouter.ai) 实现，实现了 `ChatModel` 接口。这使得与 Eino 的 LLM 功能无缝集成，以增强自然语言处理和生成能力。

OrcaRouter 是一个 AI 网关，在同一个 base URL 上提供 OpenAI 兼容（`/v1/chat/completions`）、Anthropic 兼容（`/v1/messages`）以及 Embedding 端点，并使用命名空间模型 ID，例如 `anthropic/claude-sonnet-5` 和 `anthropic/claude-haiku-4.5`。它还在同一端点上运行网关级别的零信任 AI 代理安全防护——以默认拒绝的方式对每个 prompt/response 进行筛查并管理每个工具调用，无需修改任何应用代码。

## 功能

- 实现 `github.com/cloudwego/eino/components/model.Model`
- 轻松与 Eino 的模型系统集成
- 可配置的模型参数
- 支持聊天补全
- 支持流式响应
- 支持工具调用
- 透传 OrcaRouter 提供的 Anthropic 模型思考内容（`reasoning_content`）

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/model/orcarouter@latest
```

## 快速开始

以下是如何使用 OrcaRouter 模型的快速示例：

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/components/model/orcarouter"
)

func main() {
	ctx := context.Background()

	chatModel, err := orcarouter.NewChatModel(ctx, &orcarouter.Config{
		APIKey: os.Getenv("ORCAROUTER_API_KEY"),
		Model:  os.Getenv("ORCAROUTER_MODEL"), // 例如 anthropic/claude-sonnet-5
	})
	if err != nil {
		log.Fatalf("NewChatModel failed, err=%v", err)
	}

	resp, err := chatModel.Generate(ctx, []*schema.Message{
		{
			Role:    schema.User,
			Content: "as a machine, how do you answer user's question?",
		},
	})
	if err != nil {
		log.Fatalf("Generate failed, err=%v", err)
	}
	fmt.Printf("output: \n%v", resp)
}
```

## 配置

模型可以使用 `orcarouter.Config` 结构体进行配置：

```go
type Config struct {
    APIKey string
    // Timeout 指定等待 API 响应的最大时长。
    // 如果设置了 HTTPClient，则不会使用 Timeout。
    // 可选。默认：无超时
    Timeout time.Duration `json:"timeout"`

    // HTTPClient 指定用于发送 HTTP 请求的客户端。
    // 如果设置了 HTTPClient，则不会使用 Timeout。
    // 可选。默认 &http.Client{Timeout: Timeout}
    HTTPClient *http.Client `json:"http_client"`

    // BaseURL 指定 OrcaRouter 端点 URL。
    // 可选。默认：https://api.orcarouter.ai/v1
    BaseURL string `json:"base_url"`

    // Model 指定要使用的模型 ID。
    // OrcaRouter 在同一个 OpenAI 兼容端点上提供命名空间模型 ID
    // （例如 anthropic/claude-sonnet-5、anthropic/claude-haiku-4.5）。
    // 可选。
    Model string `json:"model,omitempty"`

    // MaxTokens 表示聊天补全中可生成的最大 token 数。
    // 可选。默认：模型的最大值
    MaxTokens *int `json:"max_tokens,omitempty"`

    // MaxCompletionTokens 表示模型输出的 token 总数上限，
    // 包括最终输出和思考过程中生成的任何 token。
    MaxCompletionTokens *int `json:"max_completion_tokens,omitempty"`

    // Temperature 指定要使用的采样温度。
    // 通常建议只修改此项或 TopP，不要同时修改。
    // 范围：0.0 到 2.0。较高的值使输出更随机。
    // 可选。默认：1.0
    Temperature *float32 `json:"temperature,omitempty"`

    // TopP 通过核采样控制多样性。
    // 通常建议只修改此项或 Temperature，不要同时修改。
    // 范围：0.0 到 1.0。较低的值使输出更集中。
    // 可选。默认：1.0
    TopP *float32 `json:"top_p,omitempty"`

    // Stop 是 API 停止生成更多 token 的停止序列。
    // 可选。示例：[]string{"\n", "User:"}
    Stop []string `json:"stop,omitempty"`

    // PresencePenalty 根据 token 的存在来惩罚重复。
    // 范围：-2.0 到 2.0。正值增加新话题的可能性。
    // 可选。默认：0
    PresencePenalty *float32 `json:"presence_penalty,omitempty"`

    // FrequencyPenalty 根据 token 的频率来惩罚重复。
    // 范围：-2.0 到 2.0。负值降低重复的可能性。
    // 可选。默认：0
    FrequencyPenalty *float32 `json:"frequency_penalty,omitempty"`

    // Seed 启用确定性采样以获得一致的输出。
    // 可选。为可复现的结果设置。
    Seed *int `json:"seed,omitempty"`

    // User 代表最终用户的唯一标识符。
    // 可选。
    User *string `json:"user,omitempty"`

    // LogitBias 修改指定 token 出现在补全中的可能性。
    // 可选。将 token ID 映射到 -100 到 100 之间的偏差值。
    LogitBias map[string]int `json:"logit_bias,omitempty"`

    // LogProbs 指定是否返回输出 token 的对数概率。
    LogProbs bool `json:"log_probs"`

    // TopLogProbs 是一个 0 到 20 之间的整数，指定在每个 token 位置返回的最可能的
    // token 数量，每个 token 都带有相关的对数概率。
    // 如果使用此参数，则必须将 logprobs 设置为 true。
    TopLogProbs int `json:"top_logprobs"`

    // ResponseFormat 指定模型响应的格式。
    // 可选。用于结构化输出。
    ResponseFormat *openai.ChatCompletionResponseFormat `json:"response_format,omitempty"`

    // ExtraFields 将覆盖任何具有相同 key 的现有字段。
    // 可选。对于尚未正式支持的实验性功能很有用。
    ExtraFields map[string]any `json:"extra_fields,omitempty"`
}
```

## 示例

更多用法请参阅以下示例：

- [基础聊天补全](./examples/chat/)

## 更多详情

- [Eino 文档](https://github.com/cloudwego/eino)
- [OrcaRouter](https://www.orcarouter.ai)

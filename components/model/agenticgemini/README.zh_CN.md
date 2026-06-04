# Google Gemini

这是一个用于 [Eino](https://github.com/cloudwego/eino) 的 Google Gemini 实现，实现了 `model.AgentModel` 接口。它能够与 Eino 的 LLM 能力无缝集成，提供增强的自然语言处理和生成功能。

## 特性

- 实现 `github.com/cloudwego/eino/components/model.AgentModel` 接口
- 与 Eino 模型系统轻松集成
- 可配置的模型参数
- 支持对话补全
- 支持流式响应
- 支持自定义响应解析
- 灵活的模型配置

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/model/agenticgemini@latest
```

## 快速开始

以下是使用 Gemini agentic 模型的快速示例：

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"google.golang.org/genai"

	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/components/model/agenticgemini"
)

func main() {
	apiKey := os.Getenv("GEMINI_API_KEY")
	modelName := os.Getenv("GEMINI_MODEL")

	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: apiKey,
	})
	if err != nil {
		log.Fatalf("NewClient of gemini failed, err=%v", err)
	}

	cm, err := agenticgemini.New(ctx, &agenticgemini.Config{
		Client: client,
		Model:  modelName,
		ThinkingConfig: &genai.ThinkingConfig{
			IncludeThoughts: true,
			ThinkingBudget:  nil,
		},
	})
	if err != nil {
		log.Fatalf("NewChatModel of gemini failed, err=%v", err)
	}

	resp, err := cm.Generate(ctx, []*schema.AgenticMessage{schema.UserAgenticMessage("What's the capital of France")})
	if err != nil {
		log.Fatalf("Generate error: %v", err)
	}

	fmt.Printf("\n%s\n\n\n", resp.String())

	resp, err = cm.Generate(ctx, []*schema.AgenticMessage{
		schema.UserAgenticMessage("What's the capital of France"),
		resp,
		schema.UserAgenticMessage("What's the capital of England"),
	})
	if err != nil {
		log.Fatalf("Generate error: %v", err)
	}

	fmt.Printf("\n%s\n\n\n", resp.String())
}


```

## 配置

可以使用 `agenticgemini.Config` 结构体来配置模型：

```go
// Config 包含 Gemini agentic 模型的配置选项
type Config struct {
    // Client 是 Gemini API 客户端实例
    // 必需，用于调用 Gemini API
    Client *genai.Client

    // Model 指定使用的 Gemini 模型
    // 示例："gemini-pro"、"gemini-pro-vision"、"gemini-1.5-flash"
    Model string
    
    // MaxTokens 限制响应中的最大 token 数量
    // 可选。示例：maxTokens := 100
    MaxTokens *int
    
    // Temperature 控制响应的随机性
    // 范围：[0.0, 1.0]，0.0 更加专注，1.0 更具创造性
    // 可选。示例：temperature := float32(0.7)
    Temperature *float32
    
    // TopP 通过核采样控制多样性
    // 范围：[0.0, 1.0]，1.0 表示禁用核采样
    // 可选。示例：topP := float32(0.95)
    TopP *float32
    
    // TopK 通过限制采样的前 K 个 token 来控制多样性
    // 可选。示例：topK := int32(40)
    TopK *int32
    
    // ResponseJSONSchema 定义 JSON 响应的结构
    // 可选。当需要 JSON 格式的结构化输出时使用
    ResponseJSONSchema *jsonschema.Schema
    
    // SafetySettings 配置不同危害类别的内容过滤
    // 控制模型对潜在有害内容的过滤行为
    // 可选。
    SafetySettings []*genai.SafetySetting
    
    ThinkingConfig *genai.ThinkingConfig

    // ImageConfig 是图片生成配置
    // 注意：如果模型不支持该配置选项，将会返回错误
    // 可选。
    ImageConfig *genai.ImageConfig
    
    // ResponseModalities 指定模型可以返回的模态类型
    // 可选。
    ResponseModalities []genai.Modality
    
    MediaResolution genai.MediaResolution
    
    // CacheExpiration 配置前缀缓存资源的过期策略。
    // 可选。
    CacheExpiration *CacheExpiration
}
```


## 扩展字段说明

Eino agentic schema 中的若干字段被声明为 `any` 类型，以便每个模型实现都能附加各自特定的数据。当你消费本包产生的
响应时，必须先将这些 `any` 字段类型断言为本包中定义的具体类型，才能读取其内容。本节记录了每一个此类字段及其
承载的确切类型。

### ResponseMeta

每个返回的 `*schema.AgenticMessage` 都带有 `ResponseMeta *schema.AgenticResponseMeta`。本包会填充强类型的
`GeminiExtension` 字段（无需断言）；通用的 `Extension any` 字段未被使用。

```go
type AgenticResponseMeta struct {
    // TokenUsage 填充了 prompt / completion / total 的 token 计数。
    TokenUsage *TokenUsage

    // GeminiExtension 由本包填充（强类型，无需断言）。
    GeminiExtension *gemini.ResponseMetaExtension

    // OpenAIExtension / ClaudeExtension / Extension 本包未使用。
}
```

`GeminiExtension` 的类型是 `*github.com/cloudwego/eino/schema/gemini.ResponseMetaExtension`。本包会填充
结束原因，以及在使用 grounding（Google 搜索）时填充 grounding 元数据：

```go
type ResponseMetaExtension struct {
    ID            string             // 响应 ID（由流式分片拼接而来）
    FinishReason  string             // 例如 "STOP"、"MAX_TOKENS"、"SAFETY"
    GroundingMeta *GroundingMetadata // 仅在使用 grounding/搜索时非 nil
}
```

```go
ext := msg.ResponseMeta.GeminiExtension // 强类型，无需断言
```

### ServerToolCall 与 ServerToolResult

当模型使用内置的代码执行（code execution）服务端工具时，生成的内容块会携带 `*schema.ServerToolCall` 与
`*schema.ServerToolResult`。两者都将其载荷包装在 `any` 字段中，本包始终用自身的具体类型填充它们。`Name` 字段
为 `agenticgemini.ServerToolNameCodeExecution`。

```go
type ServerToolCall struct {
    Name      string // "CodeExecution"（agenticgemini.ServerToolNameCodeExecution）
    CallID    string
    Arguments any    // 具体类型：*agenticgemini.ServerToolCallArguments
}

type ServerToolResult struct {
    Name    string
    CallID  string
    Content any    // 具体类型：*agenticgemini.ServerToolCallResult
}
```

#### `ServerToolCall.Arguments`（`any`）

断言为 `*agenticgemini.ServerToolCallArguments`。它承载模型生成的可执行代码：

```go
type ServerToolCallArguments struct {
    ExecutableCode *ExecutableCode // Code 字符串 + Language（例如 agenticgemini.LanguagePython）
}
```

```go
// 具体类型始终为 *agenticgemini.ServerToolCallArguments。
args, ok := msg.ContentBlocks[i].ServerToolCall.Arguments.(*agenticgemini.ServerToolCallArguments)
```

#### `ServerToolResult.Content`（`any`）

断言为 `*agenticgemini.ServerToolCallResult`。它承载代码执行的结果与输出：

```go
type ServerToolCallResult struct {
    CodeExecutionResult *CodeExecutionResult // Outcome（例如 agenticgemini.OutcomeOK）+ Output 字符串
}
```

```go
// 具体类型始终为 *agenticgemini.ServerToolCallResult。
result, ok := msg.ContentBlocks[i].ServerToolResult.Content.(*agenticgemini.ServerToolCallResult)
```

## 更多详情

- [Eino 文档](https://github.com/cloudwego/eino)
- [Gemini API 文档](https://ai.google.dev/api/generate-content?hl=zh-cn#v1beta.GenerateContentResponse)

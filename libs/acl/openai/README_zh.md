# OpenAI ACL（API 客户端库）

[English](README.md) | 简体中文

## 简介

这是一个用于 Go 的低级 OpenAI API 客户端库。它提供对 OpenAI 聊天完成和嵌入 API 的直接访问，同时支持 OpenAI 和 Azure OpenAI 服务。此库由更高级别的组件包内部使用。

**注意**：对于大多数 Eino 用例，您应该使用更高级别的组件包，如 `components/model/openai` 和 `components/embedding/openai`，它们提供了与 Eino 组件系统集成的更简单接口。

## 特性

- 直接的 OpenAI API 客户端实现
- 同时支持 OpenAI 和 Azure OpenAI 服务
- 支持流式聊天完成
- 嵌入生成
- 工具/函数调用支持
- JSON schema 响应格式化
- 可配置的 HTTP 客户端
- 线程安全操作

## 安装

```bash
go get github.com/cloudwego/eino-ext/libs/acl/openai
```

## 聊天完成

### 基本用法

```go
package main

import (
	"context"
	"log"

	"github.com/cloudwego/eino-ext/libs/acl/openai"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()

	client, err := openai.NewChatModel(ctx, &openai.Config{
		APIKey: "your-api-key",
	})
	if err != nil {
		log.Fatal(err)
	}

	resp, err := client.Generate(ctx, []*schema.Message{
		{Role: schema.User, Content: "你好，你好吗？"},
	})
	if err != nil {
		log.Fatal(err)
	}

	log.Println(resp.Message.Content)
}
```

### 使用 Azure OpenAI

```go
client, err := openai.NewChatModel(ctx, &openai.Config{
	APIKey:     "your-azure-api-key",
	ByAzure:    true,
	BaseURL:    "https://your-resource.openai.azure.com",
	APIVersion: "2024-02-01",
})
```

### 流式处理

```go
stream, err := client.Stream(ctx, []*schema.Message{
	{Role: schema.User, Content: "给我讲个故事"},
})
if err != nil {
	log.Fatal(err)
}
defer stream.Close()

for {
	chunk, err := stream.Recv()
	if err == io.EOF {
		break
	}
	if err != nil {
		log.Fatal(err)
	}
	fmt.Print(chunk.Message.Content)
}
```

## 嵌入

### 基本用法

```go
package main

import (
	"context"
	"log"

	"github.com/cloudwego/eino-ext/libs/acl/openai"
)

func main() {
	ctx := context.Background()

	client, err := openai.NewEmbeddingClient(ctx, &openai.EmbeddingConfig{
		APIKey: "your-api-key",
		Model:  "text-embedding-3-small",
	})
	if err != nil {
		log.Fatal(err)
	}

	embeddings, err := client.EmbedStrings(ctx, []string{
		"你好世界",
		"你好吗？",
	})
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("生成了 %d 个嵌入向量", len(embeddings))
}
```

## 配置

### 聊天模型配置

```go
type Config struct {
    // 认证（必填）
    APIKey string
    
    // HTTP 客户端（选填）
    HTTPClient *http.Client
    
    // Azure OpenAI（选填）
    ByAzure              bool
    BaseURL              string
    APIVersion           string
    AzureModelMapperFunc func(model string) string
    
    // OpenAI Base URL 覆盖（选填）
    BaseURL string
    
    // 聊天完成参数
    Model             string
    Temperature       *float32
    TopP              *float32
    N                 *int
    Stop              []string
    MaxTokens         *int
    PresencePenalty   *float32
    ResponseFormat    *ChatCompletionResponseFormat
    Seed              *int
    FrequencyPenalty  *float32
    LogitBias         map[string]int
    LogProbs          *bool
    TopLogProbs       *int
    User              *string
    Modalities        []Modality
    Audio             *Audio
    StreamOptions     *StreamOptions
}
```

### 嵌入配置

```go
type EmbeddingConfig struct {
    // 认证（必填）
    APIKey string
    
    // HTTP 客户端（选填）
    HTTPClient *http.Client
    
    // Azure OpenAI（选填）
    ByAzure    bool
    BaseURL    string
    APIVersion string
    
    // 嵌入参数
    Model          string
    EncodingFormat *EmbeddingEncodingFormat
    Dimensions     *int
    User           *string
}
```

## 高级功能

### 工具/函数调用

```go
tools := []*schema.ToolInfo{
	{
		Name:        "get_weather",
		Description: "获取某个位置的当前天气",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"location": {
				Type:        "string",
				Description: "城市名称",
			},
		}),
	},
}

resp, err := client.Generate(ctx, messages, 
	model.WithTools(tools),
)
```

### JSON Schema 响应

```go
jsonSchema := &openai.ChatCompletionResponseFormat{
	Type: openai.ChatCompletionResponseFormatTypeJSONSchema,
	JSONSchema: &openai.ChatCompletionResponseFormatJSONSchema{
		Name:   "weather_response",
		Schema: mySchema,
		Strict: true,
	},
}

resp, err := client.Generate(ctx, messages,
	model.WithGenOption(openai.WithResponseFormat(jsonSchema)),
)
```

### 调用选项

可用的调用选项：

```go
// 温度控制
openai.WithTemperature(0.7)

// 最大 token 数
openai.WithMaxTokens(1000)

// Top P 采样
openai.WithTopP(0.9)

// 频率惩罚
openai.WithFrequencyPenalty(0.5)

// 存在惩罚
openai.WithPresencePenalty(0.5)

// 停止序列
openai.WithStop([]string{"\n", "END"})

// 响应格式
openai.WithResponseFormat(format)

// 流选项
openai.WithStreamOptions(&openai.StreamOptions{
	IncludeUsage: true,
})
```

## 用例

此库通常用于：

1. **内部实现**：作为更高级别组件包的底层客户端
2. **直接集成**：当您需要对 OpenAI API 调用进行细粒度控制时
3. **自定义解决方案**：构建自定义 LLM 集成时

对于大多数 Eino 集成，请改用更高级别的组件包：
- [components/model/openai](../../components/model/openai) 用于聊天模型
- [components/embedding/openai](../../components/embedding/openai) 用于嵌入

## 许可证

本项目采用 Apache License 2.0 许可证 - 详见 LICENSE 文件。

# Google Gemini

一个针对 [Eino](https://github.com/cloudwego/eino) 的 Google Gemini 实现，实现了 `ToolCallingChatModel` 接口。这使得能够与 Eino 的 LLM 功能无缝集成，以增强自然语言处理和生成能力。

## 特性

- 实现了 `github.com/cloudwego/eino/components/model.Model`
- 轻松与 Eino 的模型系统集成
- 可配置的模型参数
- 支持聊天补全
- 支持流式响应
- 支持自定义响应解析
- 灵活的模型配置
- 支持对生成的响应进行缓存
- 自动处理重复的工具调用 ID

## 重要说明

### 工具调用 ID 处理

Gemini 的 API 不在其响应中提供工具调用 ID。为了确保与 Eino 框架的兼容性并实现正确的工具执行跟踪，此实现会自动为每个工具调用生成唯一的 UUID（v4）。

**ID 生成：**
- 每个工具调用都会收到一个新生成的 UUID
- UUID 在所有响应和会话中全局唯一
- 格式：标准 UUID v4（例如，`550e8400-e29b-41d4-a716-446655440000`）

**示例：**
```go
// 如果 Gemini 为不同城市返回多次 "get_weather" 调用：
// 工具调用 1：ID = "550e8400-e29b-41d4-a716-446655440000", Args = {"city": "Paris"}
// 工具调用 2：ID = "6ba7b810-9dad-11d1-80b4-00c04fd430c8", Args = {"city": "London"}
// 工具调用 3：ID = "7c9e6679-7425-40de-944b-e07fc1f90ae7", Args = {"city": "Tokyo"}
```

**优势：**
- **会话范围内的唯一性**：UUID 可防止多次模型调用之间的 ID 冲突
- **标准格式**：与行业标准工具跟踪系统兼容
- **简化实现**：无需在调用之间维护状态

这确保每个工具调用都有一个全局唯一的标识符，这对于具有多次模型交互的复杂 Agent 工作流中的工具执行跟踪和响应处理至关重要。

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/model/gemini@latest
```

## 快速开始

以下是如何使用 Gemini 模型的快速示例：

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"google.golang.org/genai"

	"github.com/cloudwego/eino-ext/components/model/gemini"
	"github.com/cloudwego/eino/schema"
)

func main() {
	apiKey := os.Getenv("GEMINI_API_KEY")
	baseURL := os.Getenv("GEMINI_BASE_URL")

	ctx := context.Background()
	clientConfig := &genai.ClientConfig{
		APIKey: apiKey,
	}
	// Optional: route request to custom Gemini-compatible endpoint
	if baseURL != "" {
		clientConfig.HTTPOptions = genai.HTTPOptions{
			BaseURL: baseURL,
		}
	}
	client, err := genai.NewClient(ctx, clientConfig)
	if err != nil {
		log.Fatalf("NewClient of gemini failed, err=%v", err)
	}

	cm, err := gemini.NewChatModel(ctx, &gemini.Config{
		Client: client,
		Model:  "gemini-1.5-flash",
		ThinkingConfig: &genai.ThinkingConfig{
			IncludeThoughts: true,
			ThinkingBudget:  nil,
		},
	})
	if err != nil {
		log.Fatalf("NewChatModel of gemini failed, err=%v", err)
	}

	// If you are using a model that supports image understanding (e.g., gemini-1.5-flash-image-preview),
	// you can provide both image and text input like this:
	/*
		image, err := os.ReadFile("./path/to/your/image.jpg")
		if err != nil {
			log.Fatalf("os.ReadFile failed, err=%v\n", err)
		}

		imageStr := base64.StdEncoding.EncodeToString(image)

		resp, err := cm.Generate(ctx, []*schema.Message{
			{
				Role: schema.User,
				UserInputMultiContent: []schema.MessageInputPart{
					{
						Type: schema.ChatMessagePartTypeText,
						Text: "What do you see in this image?",
					},
					{
						Type: schema.ChatMessagePartTypeImageURL,
						Image: &schema.MessageInputImage{
							MessagePartCommon: schema.MessagePartCommon{
								Base64Data: &imageStr,
								MIMEType:   "image/jpeg",
							},
							Detail: schema.ImageURLDetailAuto,
						},
					},
				},
			},
		})
	*/

	resp, err := cm.Generate(ctx, []*schema.Message{
		{
			Role:    schema.User,
			Content: "What is the capital of France?",
		},
	})
	if err != nil {
		log.Fatalf("Generate error: %v", err)
	}

	fmt.Printf("Assistant: %s\n", resp.Content)
	if len(resp.ReasoningContent) > 0 {
		fmt.Printf("ReasoningContent: %s\n", resp.ReasoningContent)
	}
}
```

如果需要把请求路由到自定义的 Gemini 兼容端点，请设置 `GEMINI_BASE_URL`

## 配置

可以使用 `gemini.Config` 结构体配置模型：

```go
type Config struct {
	// Client is the Gemini API client instance
	// Required for making API calls to Gemini
	Client *genai.Client

	// Model specifies which Gemini model to use
	// Examples: "gemini-pro", "gemini-pro-vision", "gemini-1.5-flash"
	Model string

	// MaxTokens limits the maximum number of tokens in the response
	// Optional. Example: maxTokens := 100
	MaxTokens *int

	// Temperature controls randomness in responses
	// Range: [0.0, 1.0], where 0.0 is more focused and 1.0 is more creative
	// Optional. Example: temperature := float32(0.7)
	Temperature *float32

	// TopP controls diversity via nucleus sampling
	// Range: [0.0, 1.0], where 1.0 disables nucleus sampling
	// Optional. Example: topP := float32(0.95)
	TopP *float32

	// TopK controls diversity by limiting the top K tokens to sample from
	// Optional. Example: topK := int32(40)
	TopK *int32

	// ResponseSchema defines the structure for JSON responses
	// Optional. Used when you want structured output in JSON format
	ResponseSchema *openapi3.Schema

	// EnableCodeExecution allows the model to execute code
	// Warning: Be cautious with code execution in production
	// Optional. Default: false
	EnableCodeExecution bool

	// SafetySettings configures content filtering for different harm categories
	// Controls the model's filtering behavior for potentially harmful content
	// Optional.
	SafetySettings []*genai.SafetySetting

	ThinkingConfig *genai.ThinkingConfig

	// ResponseModalities specifies the modalities the model can return.
	// Optional.
	ResponseModalities []
	
	MediaResolution genai.MediaResolution

	// Cache controls prefix cache settings for the model.
	// Optional. used to CreatePrefixCache for reused inputs.
	Cache *CacheConfig
}

// CacheConfig controls prefix cache settings for the model.
type CacheConfig struct {
	// TTL specifies how long cached resources remain valid (now + TTL).
	TTL time.Duration `json:"ttl,omitempty"`
	// ExpireTime sets the absolute expiration timestamp for cached resources.
	ExpireTime time.Time `json:"expireTime,omitempty"`
}
```

## 缓存

该组件支持两种缓存策略以提高延迟并减少 API 调用：

- 显式缓存（前缀缓存）：从系统指令、工具和消息中构建可重用的上下文。使用 `CreatePrefixCache` 创建缓存，并在后续请求中使用 `gemini.WithCachedContentName(...)` 传递其名称。通过 `CacheConfig`（`TTL`、`ExpireTime`）配置 TTL 和绝对到期时间。当使用缓存内容时，请求会省略系统指令和工具，并依赖于缓存的前缀。
- 隐式缓存：由 Gemini 自身管理。服务可能会自动重用先前的请求或响应。到期和重用由 Gemini 控制，无法配置。

下面的示例展示了如何创建前缀缓存并在后续调用中重用它。
```go
toolInfoList := []*schema.ToolInfo{
	{
		Name:        "tool_a",
		Desc:        "desc",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{}),
	},
}
cacheInfo, _ := cm.CreatePrefixCache(ctx, []*schema.Message{
		{
			Role: schema.System,
			Content: `aaa`,
		},
		{
			Role: schema.User,
			Content: `bbb`,
		},
	}, model.WithTools(toolInfoList))


msg, err := cm.Generate(ctx, []*schema.Message{
		{
			Role:    schema.User,
			Content: "give a very short summary about this transcript",
		},
	}, gemini.WithCachedContentName(cacheInfo.Name))
```


## 示例

查看以下示例了解更多用法：

- [基础生成](./examples/generate/)
- [图像输入](./examples/generate_with_image/)
- [前缀缓存](./examples/generate_with_prefix_cache/)
- [图像生成](./examples/image_generate/)
- [意图识别与工具调用](./examples/intent_tool/)
- [ReAct 模式](./examples/react/)
- [流式响应](./examples/stream/)



## 更多信息

- [Eino Documentation](https://github.com/cloudwego/eino)
- [Gemini API Documentation](https://ai.google.dev/api/generate-content?hl=zh-cn#v1beta.GenerateContentResponse)

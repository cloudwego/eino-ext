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

	"github.com/cloudwego/eino-ext/components/agentic/gemini"
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

	cm, err := agenticgemini.NewAgenticModel(ctx, &agenticgemini.Config{
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

// EnableCodeExecution 允许模型使用服务端工具 CodeExecution
// 可选。
EnableCodeExecution *genai.ToolCodeExecution
// EnableGoogleSearch 允许模型使用服务端工具 GoogleSearch
// 可选。
EnableGoogleSearch *genai.GoogleSearch
// EnableGoogleSearchRetrieval 允许模型使用服务端工具 GoogleSearchRetrieval
// 可选。
EnableGoogleSearchRetrieval *genai.GoogleSearchRetrieval
// EnableComputerUse 允许模型使用服务端工具 ComputerUse
// 可选。
EnableComputerUse *genai.ComputerUse
// EnableURLContext 允许模型使用服务端工具 URLContext
// 可选。
EnableURLContext *genai.URLContext
// EnableFileSearch 允许模型使用服务端工具 FileSearch
// 可选。
EnableFileSearch *genai.FileSearch
// EnableGoogleMaps 允许模型使用服务端工具 GoogleMaps
// 可选。
EnableGoogleMaps *genai.GoogleMaps

// SafetySettings 配置不同危害类别的内容过滤
// 控制模型对潜在有害内容的过滤行为
// 可选。
SafetySettings []*genai.SafetySetting

ThinkingConfig *genai.ThinkingConfig

// ResponseModalities 指定模型可以返回的模态类型
// 可选。
ResponseModalities []ResponseModality

MediaResolution genai.MediaResolution

// Cache 控制模型的前缀缓存设置
// 可选。用于为重复使用的输入创建前缀缓存
Cache *CacheConfig
}
```


## 更多详情

- [Eino 文档](https://github.com/cloudwego/eino)
- [Gemini API 文档](https://ai.google.dev/api/generate-content?hl=zh-cn#v1beta.GenerateContentResponse)

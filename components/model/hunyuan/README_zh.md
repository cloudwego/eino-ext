# Hunyuan 模型

腾讯云Hunyuan大模型的[Eino](https://github.com/cloudwego/eino)实现，实现了`ToolCallingChatModel`接口。这使得能够与Eino的LLM能力无缝集成，提供增强的自然语言处理和生成功能。

## 功能特性

- 实现 `github.com/cloudwego/eino/components/model.Model` 接口
- 与Eino模型系统轻松集成
- 可配置的模型参数
- 支持聊天补全
- 支持流式响应
- 自定义响应解析支持
- 灵活的模型配置
- 支持多模态内容（图像、视频）
- 工具调用（Function Calling）支持
- 推理内容支持
- 完整的错误处理
- 回调机制支持

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/model/hunyuan@latest
```

## 快速开始

以下是使用Hunyuan模型的快速示例：

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/cloudwego/eino-ext/components/model/hunyuan"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()
	
	// 从环境变量获取认证信息
	secretId := os.Getenv("HUNYUAN_SECRET_ID")
	secretKey := os.Getenv("HUNYUAN_SECRET_KEY")
	
	if secretId == "" || secretKey == "" {
		log.Fatal("需要设置 HUNYUAN_SECRET_ID 和 HUNYUAN_SECRET_KEY 环境变量")
	}

	// 创建Hunyuan聊天模型
	cm, err := hunyuan.NewChatModel(ctx, &hunyuan.ChatModelConfig{
		SecretId:  secretId,
		SecretKey: secretKey,
		Model:     "hunyuan-lite", // 选项：hunyuan-lite, hunyuan-pro, hunyuan-turbo
		Timeout:   30 * time.Second,
	})
	if err != nil {
		log.Fatal(err)
	}

	// 构建对话消息
	messages := []*schema.Message{
		schema.SystemMessage("你是一个有用的AI助手。请用中文回答用户问题。"),
		schema.UserMessage("请介绍腾讯云Hunyuan大模型的功能特性和优势。"),
	}

	// 调用模型生成响应
	resp, err := cm.Generate(ctx, messages)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("助手：%s\n", resp.Content)
	
	if resp.ReasoningContent != "" {
		fmt.Printf("\n推理过程：%s\n", resp.ReasoningContent)
	}
	
	if resp.ResponseMeta != nil && resp.ResponseMeta.Usage != nil {
		usage := resp.ResponseMeta.Usage
		fmt.Printf("\nToken使用情况：\n")
		fmt.Printf("  提示词Token：%d\n", usage.PromptTokens)
		fmt.Printf("  补全Token：%d\n", usage.CompletionTokens)
		fmt.Printf("  总Token：%d\n", usage.TotalTokens)
	}
}
```

## 配置

模型可以通过 `hunyuan.ChatModelConfig` 结构体进行配置：

```go
type ChatModelConfig struct {
	// SecretId 是您的腾讯云API Secret ID
	// 必需
	SecretId string

	// SecretKey 是您的腾讯云API Secret Key
	// 必需
	SecretKey string

	// Model 指定要使用的模型ID
	// 必需。选项：hunyuan-lite, hunyuan-pro, hunyuan-turbo
	Model string

	// Region 指定服务区域
	// 可选。默认：ap-guangzhou
	Region string

	// Timeout 指定等待API响应的最大时长
	// 可选。默认：30秒
	Timeout time.Duration

	// Temperature 控制输出的随机性
	// 范围：[0.0, 2.0]。值越高输出越随机
	// 可选。默认：1.0
	Temperature float32

	// TopP 通过核心采样控制多样性
	// 范围：[0.0, 1.0]。值越低输出越集中
	// 可选。默认：1.0
	TopP float32

	// Stop 指定API停止生成更多token的序列
	// 可选
	Stop []string

	// PresencePenalty 通过基于存在的惩罚防止重复
	// 范围：[-2.0, 2.0]。正值增加新主题的可能性
	// 可选。默认：0
	PresencePenalty float32

	// FrequencyPenalty 通过基于频率的惩罚防止重复
	// 范围：[-2.0, 2.0]。正值减少重复的可能性
	// 可选。默认：0
	FrequencyPenalty float32
}
```

## 示例

### 基础文本生成

参考：[examples/generate/generate.go](./examples/generate/generate.go)

```bash
cd examples/generate
go run generate.go
```

### 流式响应

参考：[examples/stream/stream.go](./examples/stream/stream.go)

```bash
cd examples/stream
go run stream.go
```

### 工具调用

参考：[examples/tool_call/tool_call.go](./examples/tool_call/tool_call.go)

```bash
cd examples/tool_call
go run tool_call.go
```

### 多模态处理

参考：[examples/multimodal/multimodal.go](./examples/multimodal/multimodal.go)

```bash
cd examples/multimodal
go run multimodal.go
```

## 工具调用

### 定义工具

```go
tools := []*schema.ToolInfo{
	{
		Name: "get_weather",
		Desc: "查询城市天气信息",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"city": {
				Type: schema.String,
				Desc: "城市名称",
			},
		}),
	},
}

cmWithTools, err := cm.WithTools(tools)
```

### 处理工具调用

```go
resp, err := cmWithTools.Generate(ctx, messages)
if len(resp.ToolCalls) > 0 {
	for _, toolCall := range resp.ToolCalls {
		switch toolCall.Function.Name {
		case "get_weather":
			// 执行工具调用
			result, _ := getWeather(city)
			
			// 将结果添加到对话中
			messages = append(messages, &schema.Message{
				Role:       schema.Tool,
				ToolCallID: toolCall.ID,
				Content:    result,
			})
		}
	}
	
	// 继续对话
	finalResp, err := cmWithTools.Generate(ctx, messages)
}
```

## 多模态内容处理

### 图像处理

```go
messages := []*schema.Message{
	{
		Role: schema.User,
		UserInputMultiContent: []schema.MessageInputPart{
			{
				Type: schema.ChatMessagePartTypeText,
				Text: "请描述这张图片的内容：",
			},
			{
				Type: schema.ChatMessagePartTypeImageURL,
				Image: &schema.MessageInputImage{
					MessagePartCommon: schema.MessagePartCommon{
						MIMEType: "image/jpeg",
						Base64Data: toPtr("base64-image-data"),
					},
				},
			},
		},
	},
}
```

### 视频处理

```go
messages := []*schema.Message{
	{
		Role: schema.User,
		UserInputMultiContent: []schema.MessageInputPart{
			{
				Type: schema.ChatMessagePartTypeVideoURL,
				Video: &schema.MessageInputVideo{
					MessagePartCommon: schema.MessagePartCommon{
						MIMEType: "video/mp4",
						Base64Data: toPtr("base64-video-data"),
					},
				},
			},
			{
				Type: schema.ChatMessagePartTypeText,
				Text: "请分析这个视频的主要内容。",
			},
		},
	},
}
```

## 环境变量

| 变量名 | 描述 | 必需 |
|--------|------|------|
| HUNYUAN_SECRET_ID | 腾讯云API Secret ID | 是 |
| HUNYUAN_SECRET_KEY | 腾讯云API Secret Key | 是 |

## 测试

运行单元测试：

```bash
go test -v ./...
```

## 相关链接

- [腾讯云Hunyuan文档](https://cloud.tencent.com/document/product/1729)
- [CloudWeGo Eino框架](https://github.com/cloudwego/eino)
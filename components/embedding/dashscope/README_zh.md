# DashScope Embedding for Eino

[English](./README.md) | 简体中文

## 简介

这是一个为 [Eino](https://github.com/cloudwego/eino) 实现的 DashScope Embedding 组件，实现了 `Embedder` 接口，可无缝集成到 Eino 的 embedding 系统中，提供文本向量化能力。它通过 OpenAI 兼容的端点使用阿里云百炼平台的文本嵌入 API。

## 特性

- 实现 `github.com/cloudwego/eino/components/embedding.Embedder` 接口
- 易于集成到 Eino 的工作流
- 支持多个百炼嵌入模型（text-embedding-v1、text-embedding-v2、text-embedding-v3）
- 为 text-embedding-v3 模型提供可配置的输出维度
- Eino 内置回调支持
- 可配置的超时和 HTTP 客户端设置

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/embedding/dashscope
```

## 快速开始

```go
package main

import (
	"context"
	"log"
	"time"

	"github.com/cloudwego/eino-ext/components/embedding/dashscope"
)

func main() {
	ctx := context.Background()

	embedder, err := dashscope.NewEmbedder(ctx, &dashscope.EmbeddingConfig{
		APIKey:  "your-dashscope-api-key",
		Model:   "text-embedding-v3",
		Timeout: 30 * time.Second,
	})
	if err != nil {
		log.Fatalf("NewEmbedder of dashscope error: %v", err)
		return
	}

	vectors, err := embedder.EmbedStrings(ctx, []string{"hello", "how are you"})
	if err != nil {
		log.Fatalf("EmbedStrings of DashScope failed, err=%v", err)
	}

	log.Printf("vectors : %v", vectors)
}
```

## 配置说明

embedder 可以通过 `EmbeddingConfig` 结构体进行配置：

```go
type EmbeddingConfig struct {
    // APIKey 是你的百炼 API 密钥
    // 必需
    APIKey string `json:"api_key"`

    // Timeout 指定 HTTP 请求超时时间
    // 如果设置了 HTTPClient，则不会使用 Timeout
    // 可选。默认：无超时
    Timeout time.Duration `json:"timeout"`

    // HTTPClient 指定用于发送 HTTP 请求的客户端
    // 如果设置了 HTTPClient，则不会使用 Timeout
    // 可选。默认 &http.Client{Timeout: Timeout}
    HTTPClient *http.Client `json:"http_client"`

    // Model 指定使用哪个嵌入模型
    // 可用模型：text-embedding-v1、text-embedding-v2、text-embedding-v3
    // 不支持异步嵌入模型
    // 必需
    Model string `json:"model"`

    // Dimensions 指定输出向量维度
    // 仅适用于 text-embedding-v3 模型
    // 只能在三个值之间选择：1024、768 和 512
    // 可选。默认：1024
    Dimensions *int `json:"dimensions,omitempty"`
}
```

## 可用模型

百炼支持以下文本嵌入模型：

- `text-embedding-v1`：基础文本嵌入模型
- `text-embedding-v2`：增强文本嵌入模型  
- `text-embedding-v3`：最新文本嵌入模型，支持可配置维度（512、768 或 1024）

注意：不支持异步嵌入模型。

## 示例

查看以下示例了解更多用法：

- [文本嵌入](./examples/embedding/)

## API 参考

有关百炼文本嵌入 API 的更多详情，请参考：
- [百炼文本嵌入 API 文档](https://help.aliyun.com/zh/model-studio/developer-reference/text-embedding-synchronous-api)

## 许可证

本组件采用 Apache License 2.0 许可证。详情请参阅 LICENSE 文件。

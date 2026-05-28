# Gemini Embedding for Eino

[English](./README.md) | 简体中文

## 简介

这是一个为 [Eino](https://github.com/cloudwego/eino) 实现的 Gemini Embedding 组件，实现了 `Embedder` 接口，可无缝集成到 Eino 的 embedding 系统中，使用 Google 的 Gemini 嵌入模型提供文本向量化能力。

## 特性

- 实现 `github.com/cloudwego/eino/components/embedding.Embedder` 接口
- 易于集成到 Eino 的工作流
- 支持多个 Gemini 嵌入模型
- 可配置的任务类型和输出维度
- Eino 内置回调支持
- 支持自动截断和自定义 MIME 类型

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/embedding/gemini
```

## 快速开始

```go
package main

import (
	"context"
	"log"

	"github.com/cloudwego/eino-ext/components/embedding/gemini"
	"google.golang.org/genai"
)

func main() {
	ctx := context.Background()

	cli, err := genai.NewClient(ctx, &genai.ClientConfig{})
	if err != nil {
		log.Fatal("create genai client error: ", err)
	}

	embedder, err := gemini.NewEmbedder(ctx, &gemini.EmbeddingConfig{
		Client:   cli,
		Model:    "gemini-embedding-001",
		TaskType: "RETRIEVAL_QUERY",
	})
	if err != nil {
		log.Printf("new embedder error: %v\n", err)
		return
	}

	embedding, err := embedder.EmbedStrings(ctx, []string{"hello world", "你好世界"})
	if err != nil {
		log.Printf("embedding error: %v\n", err)
		return
	}

	log.Printf("embedding: %v\n", embedding)
}
```

## 配置说明

embedder 可以通过 `EmbeddingConfig` 结构体进行配置：

```go
type EmbeddingConfig struct {
    // Client 是 Gemini API 客户端实例
    // 用于对 Gemini 进行 API 调用
    // 必需
    Client *genai.Client

    // Model 指定使用哪个 Gemini 嵌入模型
    // 示例："gemini-embedding-001"、"text-embedding-004"
    // 必需
    Model string

    // TaskType 指定嵌入将用于哪种类型的任务
    // 可选
    TaskType string

    // Title 为文本指定标题
    // 仅当 TaskType 为 RETRIEVAL_DOCUMENT 时适用
    // 可选
    Title string

    // OutputDimensionality 指定输出嵌入的降维维度
    // 如果设置，输出嵌入中的多余值将从末尾截断
    // 仅 2024 年以来的较新模型支持
    // 如果使用早期模型（models/embedding-001），则无法设置此值
    // 可选
    OutputDimensionality *int32

    // MIMEType 指定输入的 MIME 类型（仅限 Vertex API）
    // 可选
    MIMEType string

    // AutoTruncate 决定是否静默截断超过最大序列长度的输入
    // 如果设置为 false，过大的输入将导致 INVALID_ARGUMENT 错误
    // 仅限 Vertex API
    // 可选。默认：false
    AutoTruncate bool
}
```

## 任务类型

`TaskType` 参数允许您指定嵌入的用途：

- `RETRIEVAL_QUERY`：当嵌入将用于搜索/检索查询时使用
- `RETRIEVAL_DOCUMENT`：当嵌入将用于语料库中的文档时使用
- `SEMANTIC_SIMILARITY`：用于语义相似性任务
- `CLASSIFICATION`：用于分类任务
- `CLUSTERING`：用于聚类任务

## 可用模型

Gemini 支持多个嵌入模型：

- `gemini-embedding-001`：早期版本的嵌入模型
- `text-embedding-004`：较新的嵌入模型，支持输出维度配置

注意：`OutputDimensionality` 参数仅由较新的模型（2024 年及以后）支持。

## 认证

要使用 Gemini API，您需要设置认证：

1. 使用您的 API 密钥设置 `GOOGLE_API_KEY` 或 `GEMINI_API_KEY` 环境变量
2. 使用 `genai.NewClient(ctx, &genai.ClientConfig{})` 创建 Gemini 客户端
3. 将客户端传递给 embedder 配置

有关 Gemini API 认证的更多详情，请参阅 [Google AI 文档](https://ai.google.dev/docs)。

## 示例

查看 [examples](./examples/) 目录获取完整的使用示例。

## 许可证

本组件采用 Apache License 2.0 许可证。详情请参阅 LICENSE 文件。

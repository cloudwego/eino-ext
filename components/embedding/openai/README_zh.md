# OpenAI Embedding for Eino

[English](./README.md) | 简体中文

## 简介

这是一个为 [Eino](https://github.com/cloudwego/eino) 实现的 OpenAI Embedding 组件，实现了 `Embedder` 接口，可无缝集成到 Eino 的 embedding 系统中，提供文本向量化能力。支持 OpenAI API 和 Azure OpenAI 服务。

## 特性

- 实现 `github.com/cloudwego/eino/components/embedding.Embedder` 接口
- 易于集成到 Eino 的工作流
- 支持 OpenAI 和 Azure OpenAI 服务
- 支持多个 OpenAI 嵌入模型
- 可配置的编码格式（浮点数或 base64）
- 为 text-embedding-3 模型提供可配置的输出维度
- Eino 内置回调支持
- 可配置的超时和 HTTP 客户端设置

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/embedding/openai
```

## 快速开始

### 使用 OpenAI API

```go
package main

import (
	"context"
	"log"
	"time"

	"github.com/cloudwego/eino-ext/components/embedding/openai"
)

func main() {
	ctx := context.Background()

	embedder, err := openai.NewEmbedder(ctx, &openai.EmbeddingConfig{
		APIKey:  "your-openai-api-key",
		Model:   "text-embedding-3-small",
		Timeout: 30 * time.Second,
	})
	if err != nil {
		log.Fatalf("NewEmbedder of openai error: %v", err)
		return
	}

	vectors, err := embedder.EmbedStrings(ctx, []string{"hello", "how are you"})
	if err != nil {
		log.Fatalf("EmbedStrings of OpenAI failed, err=%v", err)
	}

	log.Printf("vectors : %v", vectors)
}
```

### 使用 Azure OpenAI 服务

```go
package main

import (
	"context"
	"log"
	"time"

	"github.com/cloudwego/eino-ext/components/embedding/openai"
)

func main() {
	ctx := context.Background()

	embedder, err := openai.NewEmbedder(ctx, &openai.EmbeddingConfig{
		APIKey:     "your-azure-api-key",
		ByAzure:    true,
		BaseURL:    "https://{YOUR_RESOURCE_NAME}.openai.azure.com",
		APIVersion: "2024-02-01",
		Model:      "text-embedding-3-small",
		Timeout:    30 * time.Second,
	})
	if err != nil {
		log.Fatalf("NewEmbedder of openai error: %v", err)
		return
	}

	vectors, err := embedder.EmbedStrings(ctx, []string{"hello", "how are you"})
	if err != nil {
		log.Fatalf("EmbedStrings of OpenAI failed, err=%v", err)
	}

	log.Printf("vectors : %v", vectors)
}
```

## 配置说明

embedder 可以通过 `EmbeddingConfig` 结构体进行配置：

```go
type EmbeddingConfig struct {
    // Timeout 指定 API 响应的最大等待时间
    // 如果设置了 HTTPClient，则不会使用 Timeout
    // 可选。默认：无超时
    Timeout time.Duration `json:"timeout"`

    // HTTPClient 指定用于发送 HTTP 请求的客户端
    // 如果设置了 HTTPClient，则不会使用 Timeout
    // 可选。默认 &http.Client{Timeout: Timeout}
    HTTPClient *http.Client `json:"http_client"`

    // APIKey 是您的认证密钥
    // 根据服务使用 OpenAI API 密钥或 Azure API 密钥
    // 必需
    APIKey string `json:"api_key"`

    // ByAzure 指示是否使用 Azure OpenAI 服务
    // 可选。默认：false
    ByAzure bool `json:"by_azure"`

    // BaseURL 是 Azure OpenAI 端点 URL（仅用于 Azure）
    // 格式：https://{YOUR_RESOURCE_NAME}.openai.azure.com
    // 对于 Azure 是必需的
    BaseURL string `json:"base_url"`

    // APIVersion 指定 Azure OpenAI API 版本（仅用于 Azure）
    // 对于 Azure 是必需的
    APIVersion string `json:"api_version"`

    // Model 指定用于嵌入生成的模型 ID
    // 必需
    Model string `json:"model"`

    // EncodingFormat 指定嵌入输出的格式
    // 可选。默认：EmbeddingEncodingFormatFloat
    EncodingFormat *EmbeddingEncodingFormat `json:"encoding_format,omitempty"`

    // Dimensions 指定结果输出嵌入应具有的维度数
    // 仅在 text-embedding-3 及更高版本的模型中支持
    // 可选
    Dimensions *int `json:"dimensions,omitempty"`

    // User 是代表您的最终用户的唯一标识符
    // 可选。帮助 OpenAI 监控和检测滥用
    User *string `json:"user,omitempty"`
}
```

### 编码格式

embedder 支持两种编码格式：

- `EmbeddingEncodingFormatFloat`：以浮点数组形式返回嵌入（默认）
- `EmbeddingEncodingFormatBase64`：以 base64 编码字符串形式返回嵌入

## 可用模型

OpenAI 支持多个嵌入模型：

- `text-embedding-3-small`：最新的小型嵌入模型，性能改进
- `text-embedding-3-large`：最新的大型嵌入模型，质量最高
- `text-embedding-ada-002`：上一代嵌入模型

`text-embedding-3-small` 和 `text-embedding-3-large` 模型支持 `Dimensions` 参数，用于配置输出维度。

## Azure OpenAI 服务

要使用 Azure OpenAI 服务，您需要：

1. 将 `ByAzure` 设置为 `true`
2. 在 `BaseURL` 中提供您的 Azure 资源 URL（格式：`https://{YOUR_RESOURCE_NAME}.openai.azure.com`）
3. 在 `APIVersion` 中指定 API 版本
4. 在 `APIKey` 中使用您的 Azure API 密钥

有关 Azure OpenAI 服务的更多详情，请参阅 [Azure 文档](https://learn.microsoft.com/zh-cn/azure/ai-services/openai/)。

## 示例

查看以下示例了解更多用法：

- [文本嵌入](./examples/embedding/)

## API 参考

有关 OpenAI 嵌入 API 的更多详情，请参考：
- [OpenAI Embeddings API 文档](https://platform.openai.com/docs/api-reference/embeddings/create)

## 许可证

本组件采用 Apache License 2.0 许可证。详情请参阅 LICENSE 文件。

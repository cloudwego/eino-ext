# Ark Embedding for Eino

[English](./README.md) | 简体中文

## 简介

这是一个为 [Eino](https://github.com/cloudwego/eino) 实现的 Ark Embedding 组件，实现了 `Embedder` 接口，可无缝集成到 Eino 的 embedding 系统中，提供文本向量化能力。支持火山引擎 Ark 提供的文本嵌入和多模态嵌入 API。

## 特性

- 实现 `github.com/cloudwego/eino/components/embedding.Embedder` 接口
- 易于集成到 Eino 的工作流
- 支持文本和多模态嵌入 API
- 支持多种认证方式（API Key 或 AccessKey/SecretKey）
- Eino 内置回调支持
- 可配置的重试机制和超时设置
- 多模态嵌入 API 支持并发请求

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/embedding/ark
```

## 快速开始

### 文本嵌入 API

```go
package main

import (
	"context"
	"log"

	"github.com/cloudwego/eino-ext/components/embedding/ark"
)

func main() {
	ctx := context.Background()

	embedder, err := ark.NewEmbedder(ctx, &ark.EmbeddingConfig{
		APIKey: "your-api-key",
		Model:  "your-model-endpoint-id",
	})
	if err != nil {
		log.Fatalf("NewEmbedder of ark error: %v", err)
		return
	}

	vectors, err := embedder.EmbedStrings(ctx, []string{"hello", "how are you"})
	if err != nil {
		log.Fatalf("EmbedStrings of Ark failed, err=%v", err)
	}

	log.Printf("vectors : %v", vectors)
}
```

### 多模态嵌入 API

```go
package main

import (
	"context"
	"log"

	"github.com/cloudwego/eino-ext/components/embedding/ark"
)

func main() {
	ctx := context.Background()

	apiType := ark.APITypeMultiModal
	embedder, err := ark.NewEmbedder(ctx, &ark.EmbeddingConfig{
		APIKey:  "your-api-key",
		Model:   "your-model-endpoint-id",
		APIType: &apiType,
	})
	if err != nil {
		log.Fatalf("NewEmbedder of ark error: %v", err)
		return
	}

	vectors, err := embedder.EmbedStrings(ctx, []string{"hello", "how are you"})
	if err != nil {
		log.Fatalf("EmbedStrings of Ark failed, err=%v", err)
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
    // 可选。默认：10 分钟
    Timeout *time.Duration `json:"timeout"`

    // HTTPClient 指定用于发送 HTTP 请求的客户端
    // 如果设置了 HTTPClient，则不会使用 Timeout
    // 可选。默认 &http.Client{Timeout: Timeout}
    HTTPClient *http.Client `json:"http_client"`

    // RetryTimes 指定失败的 API 调用的重试次数
    // 可选。默认：2
    RetryTimes *int `json:"retry_times"`

    // BaseURL 指定 Ark 服务的基础 URL
    // 可选。默认："https://ark.cn-beijing.volces.com/api/v3"
    BaseURL string `json:"base_url"`

    // Region 指定 Ark 服务所在的区域
    // 可选。默认："cn-beijing"
    Region string `json:"region"`

    // APIKey 是用于认证的 API 密钥
    // 如果同时提供了 APIKey 和 AccessKey/SecretKey，APIKey 优先
    // 认证详情请参考：https://www.volcengine.com/docs/82379/1298459
    APIKey string `json:"api_key"`

    // AccessKey 和 SecretKey 用于认证
    // APIKey 或 AccessKey/SecretKey 对其中一种是必需的
    AccessKey string `json:"access_key"`
    SecretKey string `json:"secret_key"`

    // Model 指定 Ark 平台上的端点 ID
    // 必需
    Model string `json:"model"`

    // APIType 指定使用哪个 API：文本或多模态
    // 可选。默认：APITypeText
    APIType *APIType `json:"api_type,omitempty"`

    // MaxConcurrentRequests 指定多模态嵌入 API 调用的最大并发数
    // 仅当 APIType 为 APITypeMultiModal 时适用
    // 可选。默认：5
    MaxConcurrentRequests *int `json:"max_concurrent_requests"`
}
```

### API 类型

- `APITypeText`：使用 `/embeddings` 文本嵌入 API
  - API 参考：https://www.volcengine.com/docs/82379/1521766
  - BaseURL：https://ark.cn-beijing.volces.com/api/v3

- `APITypeMultiModal`：使用 `/embeddings/multimodal` 多模态嵌入 API
  - API 参考：https://www.volcengine.com/docs/82379/1523520
  - BaseURL：https://ark.cn-beijing.volces.com/api/v3

## 认证

embedder 支持两种认证方式：

1. **API Key**：在配置中提供 `APIKey`
2. **Access Key / Secret Key**：在配置中同时提供 `AccessKey` 和 `SecretKey`

关于认证的更多详情，请参阅[火山引擎文档](https://www.volcengine.com/docs/82379/1298459)。

## 示例

查看以下示例了解更多用法：

- [文本嵌入](./examples/embedding/)

## 许可证

本组件采用 Apache License 2.0 许可证。详情请参阅 LICENSE 文件。

# QianFan Embedding for Eino

[English](./README.md) | 简体中文

## 简介

这是一个为 [Eino](https://github.com/cloudwego/eino) 实现的千帆（百度千帆）Embedding 组件，实现了 `Embedder` 接口，可无缝集成到 Eino 的 embedding 系统中，使用百度千帆平台提供文本向量化能力。

## 特性

- 实现 `github.com/cloudwego/eino/components/embedding.Embedder` 接口
- 易于集成到 Eino 的工作流
- 支持百度千帆嵌入模型
- Eino 内置回调支持
- 可配置的重试机制（重试次数、超时、退避因子）
- Token 使用量跟踪

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/embedding/qianfan
```

## 快速开始

```go
package main

import (
	"context"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/embedding/qianfan"
)

func main() {
	ctx := context.Background()

	qcfg := qianfan.GetQianfanSingletonConfig()
	qcfg.AccessKey = os.Getenv("QIANFAN_ACCESS_KEY")
	qcfg.SecretKey = os.Getenv("QIANFAN_SECRET_KEY")

	embedder, err := qianfan.NewEmbedder(ctx, &qianfan.EmbeddingConfig{
		Model: "Embedding-V1",
	})
	if err != nil {
		log.Fatalf("NewEmbedder of qianfan error: %v", err)
		return
	}

	vectors, err := embedder.EmbedStrings(ctx, []string{"hello world", "bye world"})
	if err != nil {
		log.Fatalf("EmbedStrings of QianFan failed, err=%v", err)
	}

	log.Printf("vectors : %v", vectors)
}
```

## 配置说明

### 认证配置

千帆使用单例模式进行认证配置。您需要在创建 embedder 之前配置认证：

```go
qcfg := qianfan.GetQianfanSingletonConfig()
qcfg.AccessKey = "your-access-key"
qcfg.SecretKey = "your-secret-key"
```

您也可以通过环境变量设置认证：
- `QIANFAN_ACCESS_KEY`：您的 IAM Access Key
- `QIANFAN_SECRET_KEY`：您的 IAM Secret Key

### Embedder 配置

embedder 可以通过 `EmbeddingConfig` 结构体进行配置：

```go
type EmbeddingConfig struct {
    // Model 指定使用哪个千帆嵌入模型
    // 必需
    Model string

    // LLMRetryCount 指定失败的 API 调用的重试次数
    // 可选
    LLMRetryCount *int

    // LLMRetryTimeout 指定重试尝试的超时时间（以秒为单位）
    // 可选
    LLMRetryTimeout *float32

    // LLMRetryBackoffFactor 指定重试尝试的退避乘数
    // 可选
    LLMRetryBackoffFactor *float32
}
```

### 重试配置示例

```go
retryCount := 3
retryTimeout := float32(10.0)
backoffFactor := float32(2.0)

embedder, err := qianfan.NewEmbedder(ctx, &qianfan.EmbeddingConfig{
    Model:                 "Embedding-V1",
    LLMRetryCount:         &retryCount,
    LLMRetryTimeout:       &retryTimeout,
    LLMRetryBackoffFactor: &backoffFactor,
})
```

## 可用模型

千帆支持多种嵌入模型。常见模型包括：

- `Embedding-V1`：百度标准嵌入模型

有关可用模型的完整列表及其规格，请参阅[百度千帆文档](https://cloud.baidu.com/doc/WENXINWORKSHOP/s/Nlks5zkzu)。

## 认证

要使用千帆 API，您需要获取 IAM 凭证：

1. 注册百度云账号
2. 在 IAM 控制台创建凭证以获取 AccessKey 和 SecretKey
3. 通过代码或环境变量设置凭证

有关认证的更多详情，请参阅[百度千帆认证指南](https://cloud.baidu.com/doc/WENXINWORKSHOP/s/Ilkkrb0i5)。

## Token 使用量

embedder 会自动跟踪 token 使用量，并通过回调返回：

- `PromptTokens`：输入中的 token 数量
- `CompletionTokens`：输出中的 token 数量（如果适用）
- `TotalTokens`：使用的 token 总数

## 示例

查看 [examples](./examples/) 目录获取完整的使用示例。

## 许可证

本组件采用 Apache License 2.0 许可证。详情请参阅 LICENSE 文件。

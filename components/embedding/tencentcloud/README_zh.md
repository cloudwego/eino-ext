# 腾讯云混元 Embedding

[English](README.md) | 简体中文

这是一个为 [Eino](https://github.com/cloudwego/eino) 实现的腾讯云混元 Embedding 组件，实现了 `Embedder` 接口。它可以无缝集成到 Eino 的 embedding 系统中，提供文本向量化能力。

## 特性

- 实现了 `github.com/cloudwego/eino/components/embedding.Embedder` 接口
- 易于集成到 Eino 的 rag 工作流中
- 内置 token 使用量追踪
- 自动处理大规模文本数组的批处理
- 内置回调支持

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/embedding/tencentcloud
```

## 快速开始

```go
package main

import (
    "context"
    "fmt"
    "os"
    
    "github.com/cloudwego/eino-ext/components/embedding/tencentcloud"
)

func main() {
    ctx := context.Background()
    
    // 创建 embedder 配置
    cfg := &tencentcloud.EmbeddingConfig{
        SecretID:  os.Getenv("TENCENTCLOUD_SECRET_ID"),
        SecretKey: os.Getenv("TENCENTCLOUD_SECRET_KEY"),
        Region:    "ap-guangzhou",
    }

    // 创建 embedder
    embedder, err := tencentcloud.NewEmbedder(ctx, cfg)
    if err != nil {
        panic(err)
    }

    // 获取文本的向量表示
    embeddings, err := embedder.EmbedStrings(ctx, []string{"hello world", "bye world"})
    if err != nil {
        panic(err)
    }

    fmt.Printf("Embeddings: %v\n", embeddings)
}
```

## 配置说明

embedder 可以通过 `EmbeddingConfig` 结构体进行配置：

```go
type EmbeddingConfig struct {
    SecretID  string // 腾讯云 Secret ID
    SecretKey string // 腾讯云 Secret Key
    Region    string // 腾讯云地域（如 "ap-guangzhou"）
}
```

## 功能详情

### 自动批处理

embedder 会自动处理大规模文本数组的批处理。根据腾讯云 API 的限制，每个请求最多可以处理 200 个文本。embedder 会自动将较大的数组分割成适当的批次进行处理。

### Token 使用量追踪

embedder 通过 Eino 的回调系统追踪 token 使用量。token 使用信息包括：
- 输入 token 数量
- 总 token 数量

### 回调支持

embedder 完全支持 Eino 的回调系统，支持：
- 错误追踪
- 开始/结束事件监控
- Token 使用统计

## 更多信息

- [腾讯云混元 API 文档](https://cloud.tencent.com/document/product/1729/102832)
- [Eino 文档](https://github.com/cloudwego/eino) 
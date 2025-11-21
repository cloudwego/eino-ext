# Milvus 新版存储

[English](README.md) | [简体中文](README_zh.md)

基于 Milvus 2.6+ 的向量存储实现，为 [Eino](https://github.com/cloudwego/eino) 提供了符合 `Indexer` 接口的存储方案。该组件可无缝集成
Eino 的向量存储和检索系统，增强语义搜索能力。
与旧版本相比，此版本基于新的 Milvus Client V2 API，具有更好的性能和类型安全性。

## 快速开始

### 安装

它需要 milvus/client/v2 版本 2.6+

```bash
go get github.com/milvus-io/milvus/client/v2@v2.6.1
go get github.com/cloudwego/eino-ext/components/indexer/milvus_new@latest
```

### 创建 Milvus 存储

```go
package main

import (
	"context"
	"log"
	"os"
	
	"github.com/cloudwego/eino-ext/components/embedding/ark"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
	
	"github.com/cloudwego/eino-ext/components/indexer/milvus_new"
)

func main() {
	// Get the environment variables
	addr := os.Getenv("MILVUS_ADDR")
	username := os.Getenv("MILVUS_USERNAME")
	password := os.Getenv("MILVUS_PASSWORD")
	arkApiKey := os.Getenv("ARK_API_KEY")
	arkModel := os.Getenv("ARK_MODEL")
	
	// Create a client
	ctx := context.Background()
	cli, err := milvusclient.New(ctx, &milvusclient.ClientConfig{
		Address:  addr,
		Username: username,
		Password: password,
	})
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
		return
	}
	defer cli.Close()
	
	// Create an embedding model
	emb, err := ark.NewEmbedder(ctx, &ark.EmbeddingConfig{
		APIKey: arkApiKey,
		Model:  arkModel,
	})
	if err != nil {
		log.Fatalf("Failed to create embedding: %v", err)
		return
	}
	
	// Create an indexer
	indexer, err := milvus_new.NewIndexer(ctx, &milvus_new.IndexerConfig{
		Client:    cli,
		Embedding: emb,
	})
	if err != nil {
		log.Fatalf("Failed to create indexer: %v", err)
		return
	}
	log.Printf("Indexer created success")
	
	// Store documents
	docs := []*schema.Document{
		{
			ID:      "milvus-1",
			Content: "milvus is an open-source vector database",
			MetaData: map[string]any{
				"h1": "milvus",
				"h2": "open-source",
				"h3": "vector database",
			},
		},
		{
			ID:      "milvus-2",
			Content: "milvus is a distributed vector database",
		},
	}
	ids, err := indexer.Store(ctx, docs)
	if err != nil {
		log.Fatalf("Failed to store: %v", err)
		return
	}
	log.Printf("Store success, ids: %v", ids)
}
```

## 配置

```go
type IndexerConfig struct {
    // Client 是要调用的 milvus 客户端
    // 它使用新的 milvus/client/v2/milvusclient
    // 必需
    Client *milvusclient.Client

    // 默认集合配置
    // Collection 是 milvus 数据库中的集合名称
    // 可选，默认值为 "eino_collection"
    Collection string
    // Description 是集合的描述
    // 可选，默认值为 "the collection for eino"
    Description string
    // PartitionNum 是集合分区数量
    // 可选，默认值为 0（禁用）
    // 如果分区数量大于 1，表示使用分区，并且必须在 Fields 中有一个分区键
    PartitionNum int64
    // PartitionName 是集合分区名称
    // 可选，默认值为空
    PartitionName string
    // Fields 是集合字段
    // 可选，默认值为默认字段
    Fields       []*entity.Field
    // SharedNum 是创建集合所需的 milvus 参数
    // 可选，默认值为 1
    SharedNum int32
    // ConsistencyLevel 是 milvus 集合一致性策略
    // 可选，默认级别为 ClBounded（有界一致性级别，默认容忍度为 5 秒）
    ConsistencyLevel ConsistencyLevel
    // EnableDynamicSchema 表示集合是否启用动态模式
    // 可选，默认值为 false
    // 启用动态模式可能会影响 milvus 性能
    EnableDynamicSchema bool

    // DocumentConverter 是将 schema.Document 转换为行数据的函数
    // 可选，默认值为 defaultDocumentConverter
    DocumentConverter func(ctx context.Context, docs []*schema.Document, vectors [][]float64) ([]column.Column, error)

    // 向量列的索引配置
    // MetricType 是向量的度量类型
    // 可选，默认类型为 HAMMING
    MetricType MetricType

    // Embedding 是从 schema.Document 的内容中嵌入值所需的向量化方法
    // 必需
    Embedding embedding.Embedder
}
```

## 选项

### WithPartition

指定存储的分区名称

```go
ids, err := indexer.Store(ctx, docs, 
    milvus_new.WithPartition("partition_2024"))
```

## 默认数据模型

| 字段       | 数据类型           | 字段类型         | 索引类型                       | 描述     | 备注          |
|----------|----------------|--------------|----------------------------|--------|-------------|
| id       | string         | varchar      |                            | 文章唯一标识 | 最大长度: 255   |
| content  | string         | varchar      |                            | 文章内容   | 最大长度: 1024  |
| vector   | []float32      | float array  | HAMMING(default) / JACCARD | 文章内容向量 | 默认维度: 768   |
| metadata | map[string]any | json         |                            | 文章元数据  |             |

## 与旧版本的主要区别

1. **客户端 API**：使用新的 `milvus/client/v2/milvusclient` 而不是 `milvus-sdk-go/v2/client`
2. **数据格式**：使用基于列的数据格式而不是基于行的格式，以获得更好的性能
3. **类型安全**：更好的类型安全性，使用具体类型而不是接口
4. **配置**：某些配置参数已更改以匹配新 API

## 如何确定 dim 参数

转换关系为 `dim = embedding model output`

在新版本中，我们直接使用浮点向量而不是将 float64 转换为字节，因此维度直接由嵌入模型输出决定。
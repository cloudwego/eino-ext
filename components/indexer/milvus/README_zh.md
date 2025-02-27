# Milvus 存储

[English](README.md) | [简体中文](README_zh.md)

基于 Milvus 2.x 的向量存储实现，为 [Eino](https://github.com/cloudwego/eino) 提供了符合 `Indexer` 接口的存储方案。该组件可无缝集成 Eino 的向量存储和检索系统，增强语义搜索能力。

## 快速开始

### 安装

```bash
go get github.com/eino-project/eino/indexer/milvus@latest
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
	"github.com/milvus-io/milvus-sdk-go/v2/client"
	
	"github.com/cloudwego/eino-ext/components/retriever/milvus"
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
	cli, err := client.NewClient(ctx, client.Config{
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
	indexer, err := milvus.NewIndexer(ctx, &milvus.IndexerConfig{
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

## Configuration

```go
type IndexerConfig struct {
    // Client 需要调用的 Milvus 客户端
    // 必填
    Client client.Client

    // 集合默认配置
    // Collection 指定 Milvus 中的集合名称
    // 可选，默认值 "eino_collection"
    Collection string
    // PartitionNum 集合分区数量
    // 可选，默认值 1（禁用分区）
    // 当分区数大于 1 时，表示启用分区且分区键为 collection id
    PartitionNum int64
    // Description 集合描述信息
    // 可选，默认值 "the collection for eino"
    Description string
    // Dim 向量维度
    // 可选，默认值 10,240 * 8
    // 注意：维度必须是 8 的倍数
    Dim int64
    // SharedNum Milvus 创建集合的必要参数
    // 可选，默认值 1
    SharedNum int32
    // ConsistencyLevel 集合一致性级别
    // 可选，默认级别 ClBounded（有限一致性，默认 5 秒容忍时间）
    ConsistencyLevel ConsistencyLevel
    // EnableDynamicSchema 是否启用动态模式
    // 可选，默认值 false
    // 启用动态模式可能影响 Milvus 性能
    EnableDynamicSchema bool

    // 向量列索引配置
    // MetricType 向量相似度计算方式
    // 可选，默认 HAMMING
    // 可选值：HAMMING 和 JACCARD
    MetricType MetricType

    // Embedding 用于将文档内容转换为向量的嵌入模型
    // 必填
    Embedding embedding.Embedder
}
```

## 数据模型

| 字段       | 数据类型           | 字段类型         | 索引类型                       | 描述     | 备注          |
|----------|----------------|--------------|----------------------------|--------|-------------|
| id       | string         | varchar      |                            | 文章唯一标识 | 最大长度: 255   |
| content  | string         | varchar      |                            | 文章内容   | 最大长度: 1024  |
| vector   | []byte         | binary array | HAMMING(default) / JACCARD | 文章内容向量 | 默认维度: 81920 |
| metadata | map[string]any | json         |                            | 文章元数据  |             |


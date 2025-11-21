# Milvus 新版搜索

[English](README.md) | [简体中文](README_zh.md)

基于 Milvus 2.6+ 的向量搜索实现，为 [Eino](https://github.com/cloudwego/eino) 提供了符合 `Retriever` 接口的存储方案。该组件可无缝集成
Eino 的向量存储和检索系统，增强语义搜索能力。
与旧版本相比，此版本基于新的 Milvus Client V2 API，具有更好的性能和类型安全性。

## 快速开始

### 安装

它需要 milvus/client/v2 版本 2.6+

```bash
go get github.com/milvus-io/milvus/client/v2@v2.6.1
go get github.com/cloudwego/eino-ext/components/retriever/milvus_new@latest
```

### 创建 Milvus 搜索

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/embedding/ark"
	"github.com/milvus-io/milvus/client/v2/milvusclient"

	"github.com/cloudwego/eino-ext/components/retriever/milvus_new"
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

	// Create a retriever
	retriever, err := milvus_new.NewRetriever(ctx, &milvus_new.RetrieverConfig{
		Client:      cli,
		Collection:  "",
		Partition:   "",
		VectorField: "",
		OutputFields: []string{
			"id",
			"content",
			"metadata",
		},
		DocumentConverter: nil,
		MetricType:        "",
		TopK:              0,
		ScoreThreshold:    5,
		Embedding:         emb,
	})
	if err != nil {
		log.Fatalf("Failed to create retriever: %v", err)
		return
	}

	// Retrieve documents
	documents, err := retriever.Retrieve(ctx, "milvus")
	if err != nil {
		log.Fatalf("Failed to retrieve: %v", err)
		return
	}
	
	// Print the documents
	for i, doc := range documents {
		fmt.Printf("Document %d:\n", i)
		fmt.Printf("title: %s\n", doc.ID)
		fmt.Printf("content: %s\n", doc.Content)
		fmt.Printf("metadata: %v\n", doc.MetaData)
	}
}
```

## 配置

```go
type RetrieverConfig struct {
	// Client 是要调用的 milvus 客户端
	// 它使用新的 milvus/client/v2/milvusclient
	// 必需
	Client *milvusclient.Client

	// 默认搜索配置
	// Collection 是 milvus 数据库中的集合名称
	// 可选，默认值为 "eino_collection"
	Collection string
	// Partition 是集合的分区名称
	// 可选，默认值为空
	Partition string
	// VectorField 是集合中的向量字段名称
	// 可选，默认值为 "vector"
	VectorField string
	// OutputFields 是要返回的字段
	// 可选，默认值为除向量外的所有字段
	OutputFields []string
	// DocumentConverter 是将搜索结果转换为 schema.Document 的函数
	// 可选，默认值为 defaultDocumentConverter
	DocumentConverter func(ctx context.Context, columns []column.Column, scores []float32) ([]*schema.Document, error)
	// VectorConverter 是将向量转换为二进制向量字节的函数
	// 已弃用：此字段不再用于浮点向量。浮点向量直接处理。
	VectorConverter func(ctx context.Context, vectors [][]float64) ([][]byte, error)
	// MetricType 是向量的度量类型
	// 可选，默认值为浮点向量的 "COSINE"
	MetricType MetricType
	// TopK 是要返回的前 k 个结果
	// 可选，默认值为 5
	TopK int
	// ScoreThreshold 是搜索结果的阈值
	// 可选，默认值为 0
	ScoreThreshold float64

	// Embedding 是从 schema.Document 的内容中嵌入需要嵌入的值的方法
	// 必需的
	Embedding embedding.Embedder
}
```

## 选项

### WithFilter

为搜索设置过滤表达式

```go
docs, err := retriever.Retrieve(ctx, "query", 
    milvus_new.WithFilter("year > 2020"))
```

### WithPartition

指定搜索的分区名称

```go
docs, err := retriever.Retrieve(ctx, "query", 
    milvus_new.WithPartition("partition_2024"))
```

## 与旧版本的主要区别

1. **客户端 API**：使用新的 `milvus/client/v2/milvusclient` 而不是 `milvus-sdk-go/v2/client`
2. **数据格式**：使用基于列的数据格式而不是 SearchResult，以获得更好的性能
3. **类型安全**：更好的类型安全性，使用具体类型而不是接口
4. **配置**：某些配置参数已更改以匹配新 API
5. **选项**：使用新的选项模式 `milvus_new.WithFilter()` 和 `milvus_new.WithPartition()` 而不是在 Retrieve 方法中使用参数
# Milvus 搜索

[English](README.md) | [简体中文](README_zh.md)

基于 Milvus 2.x 的向量搜索实现，为 [Eino](https://github.com/cloudwego/eino) 提供了符合 `Retriever` 接口的存储方案。该组件可无缝集成
Eino 的向量存储和检索系统，增强语义搜索能力。

> **注意**: 此包支持 **Milvus 2.4.x**。如需使用 Milvus 2.5+ 的新功能（BM25、服务端函数、混合检索），请使用 [`milvus2`](../milvus2) 包。

## 快速开始

### 安装

它需要 milvus-sdk-go 客户端版本 2.4.x

```bash
go get github.com/eino-project/eino/retriever/milvus@latest
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
	
	// Create a retriever
	retriever, err := milvus.NewRetriever(ctx, &milvus.RetrieverConfig{
		Client:      cli,
		Collection:  "",
		Partition:   nil,
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
		Sp:                nil,
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
    // 必需
    Client client.Client

    // Retriever 通用配置
    // Collection 是 milvus 数据库中的集合名称
    // 可选，默认值为 "eino_collection"
    Collection string
    // Partition 是集合的分区名称
    // 可选，默认值为空
    Partition []string
    // VectorField 是集合中的向量字段名称
    // 可选，默认值为 "vector"
    VectorField string
    // OutputFields 是要返回的字段
    // 可选，默认值为空
    OutputFields []string
    // DocumentConverter 是将搜索结果转换为 s.Document 的函数
    // 可选，默认值为 defaultDocumentConverter
    DocumentConverter func(ctx context.Context, doc client.SearchResult) ([]*s.Document, error)
    // MetricType 是向量的度量类型
    // 可选，默认值为 "HAMMING"
    MetricType entity.MetricType
    // TopK 是要返回的前 k 个结果
    // 可选，默认值为 5
    TopK int
    // ScoreThreshold 是搜索结果的阈值
    // 可选，默认值为 0
    ScoreThreshold float64
    // SearchParams
    // 可选，默认值为 entity.IndexAUTOINDEXSearchParam，级别为 1
    Sp entity.SearchParam

    // Embedding 是从 s.Document 的内容中嵌入需要嵌入的值的方法
    // 必需的
    Embedding embedding.Embedder
}
```
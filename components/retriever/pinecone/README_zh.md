# Pinecone 搜索

[English](README.md) | [简体中文](README_zh.md)

基于 Pinecone 的向量搜索实现，为 [Eino](https://github.com/cloudwego/eino) 提供了符合 `Retriever` 接口的存储方案。该组件可无缝集成
Eino 的向量存储和检索系统，增强语义搜索能力。

## 快速开始

### 安装

它需要 pinecone-io/go-pinecone/v3 客户端版本 3.x.x

```bash
go get github.com/eino-project/eino/retriever/pinecone@latest
```

### 创建 Pinecone 搜索

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino-ext/components/retriever/pinecone"
	"github.com/cloudwego/eino/components/embedding"
	pc "github.com/pinecone-io/go-pinecone/v3/pinecone"
)

func main() {
	// Load configuration from environment variables
	apiKey := os.Getenv("PINECONE_APIKEY")
	if apiKey == "" {
		log.Fatal("PINECONE_APIKEY environment variable is required")
	}

	// Initialize Pinecone client
	client, err := pc.NewClient(pc.NewClientParams{
		ApiKey: apiKey,
	})
	if err != nil {
		log.Fatalf("Failed to create Pinecone client: %v", err)
	}

	// Create Pinecone retriever config
	config := pinecone.RetrieverConfig{
		Client:    client,
		Embedding: &mockEmbedding{},
	}

	ctx := context.Background()
	retriever, err := pinecone.NewRetriever(ctx, &config)
	if err != nil {
		log.Fatalf("Failed to create Pinecone retriever: %v", err)
	}
	log.Println("Retriever created successfully")

	// Retrieve documents
	documents, err := retriever.Retrieve(ctx, "pinecone")
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
	// Pinecone 客户端实例，用于所有 API 操作。
	// 必填，需提前初始化。
	Client *pc.Client

	// Pinecone 索引名称。
	// 可选，默认值为 "eino-index"。
	IndexName string

	// Pinecone 命名空间，用于多租户或数据隔离场景。
	// 可选，默认值为 ""（默认命名空间）。
	Namespace string

	// 相似度度量方式（如 cosine、dotproduct、euclidean）。
	// 可选，默认值为 pc.IndexMetricCosine。
	MetricType pc.IndexMetric

	// 文档字段名，用于将 Pinecone 向量与应用文档字段映射。
	// 可选，默认值为 ""，如需映射特定字段可设置。
	Field string

	// 向量转换函数，将 embedding 生成的 float64 向量转换为 Pinecone 所需的 float32。
	// 可选，若为 nil 则使用默认转换。
	VectorConverter func(ctx context.Context, vector []float64) ([]float32, error)

	// 结果转换函数，将 Pinecone 检索结果转换为 schema.Document 对象。
	// 可选，若为 nil 则使用默认转换。
	DocumentConverter func(ctx context.Context, vector *pc.Vector, field string) (*schema.Document, error)

	// 返回每次查询的 TopK 结果数。
	// 可选，默认值为 10。
	TopK int

	// 相似度分数阈值，低于该分数的结果将被过滤。
	// 可选，默认值为 0。
	ScoreThreshold float64

	// 向量化模型或服务，用于将查询转换为向量表示。
	// 语义检索必填。
	Embedding embedding.Embedder
}
```
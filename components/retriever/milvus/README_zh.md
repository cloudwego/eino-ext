# Milvus 搜索

[English](README.md) | [简体中文](README_zh.md)

基于 Milvus 2.x 的向量搜索实现，为 [Eino](https://github.com/cloudwego/eino) 提供了符合 `Retriever` 接口的检索方案。该组件可无缝集成 Eino 的向量存储和检索系统，增强语义搜索能力。

## 快速开始

### 安装

需要 milvus-sdk-go 版本 2.4.x：

```bash
go get github.com/milvus-io/milvus-sdk-go/v2@2.4.2
go get github.com/cloudwego/eino-ext/components/retriever/milvus@latest
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
	// Client 是用于数据库操作的 Milvus 客户端
	// 必需，需要 milvus-sdk-go 客户端版本 2.4.x
	Client client.Client

	// Collection 指定 Milvus 数据库中的集合名称
	// 可选，默认值为 "eino_collection"
	Collection string

	// Partition 指定要搜索的分区
	// 可选，默认值为空切片（搜索所有分区）
	Partition []string

	// VectorField 指定集合中向量字段的名称
	// 可选，默认值为 "vector"
	VectorField string

	// OutputFields 指定搜索结果中要包含的字段
	// 可选，默认值为空切片（仅返回 ID 和距离）
	OutputFields []string

	// DocumentConverter 将 Milvus 搜索结果转换为 schema.Document 实例
	// 可选，默认值为与内置 schema 兼容的转换器
	DocumentConverter func(ctx context.Context, doc client.SearchResult) ([]*schema.Document, error)

	// VectorConverter 将嵌入向量转换为 Milvus entity.Vector 格式
	// 可选，默认值为 BinaryVector 转换器
	VectorConverter func(ctx context.Context, vectors [][]float64) ([]entity.Vector, error)

	// TopK 指定要返回的最大结果数
	// 可选，默认值为 5
	TopK int

	// ScoreThreshold 过滤低于此相似度分数的结果
	// 可选，默认值为 0（不过滤）
	ScoreThreshold float64

	// SearchMode 定义不同索引类型的搜索策略和参数
	// 使用 search_mode.SearchModeHNSW、SearchModeIvfFlat、SearchModeAuto 或 SearchModeFlat
	// 可选，默认值为使用 COSINE 度量的 AUTOINDEX
	// 设置 SearchMode 后，MetricType 和 Sp 字段将被忽略
	SearchMode SearchMode

	// Deprecated: MetricType 已弃用；请在 SearchMode 中设置度量类型
	MetricType entity.MetricType

	// Deprecated: Sp 已弃用；请使用 SearchMode 配置搜索参数
	Sp entity.SearchParam

	// Embedding 提供用于向量化查询字符串的嵌入模型
	// 必需
	Embedding embedding.Embedder
}
```

## SearchMode

通过 `SearchMode` 接口灵活配置搜索参数，搜索模式需要与索引类型匹配：

| 搜索模式 | 索引类型 | 关键参数 |
|----------|----------|----------|
| `SearchModeAuto` | AUTOINDEX | `level`（1-5，速度 vs 精度） |
| `SearchModeHNSW` | HNSW | `ef`（搜索宽度，越高越精确） |
| `SearchModeIvfFlat` | IVF_FLAT | `nprobe`（搜索的聚类数） |
| `SearchModeFlat` | FLAT | 无（暴力搜索） |

> **重要提示**：SearchMode 的度量类型（Metric Type）必须与索引的度量类型匹配。

**使用示例：**

```go
import "github.com/cloudwego/eino-ext/components/retriever/milvus/search_mode"

// AUTOINDEX 搜索模式（推荐用于大多数场景）
autoMode := search_mode.SearchModeAuto(&search_mode.AutoConfig{
    Level:  1,             // 1=最快, 5=最精确
    Metric: entity.COSINE, // 必须与索引的度量类型匹配
})
retriever, err := milvus.NewRetriever(ctx, &milvus.RetrieverConfig{
    Client:     cli,
    Collection: "my_collection",
    SearchMode: autoMode,
    Embedding:  emb,
})

// HNSW 搜索模式 - 用于 HNSW 索引的集合
hnswMode, _ := search_mode.SearchModeHNSW(&search_mode.HNSWConfig{
    Ef:     64,        // 越高越精确，但更慢
    Metric: entity.L2, // 必须与索引的度量类型匹配
})
retriever, err := milvus.NewRetriever(ctx, &milvus.RetrieverConfig{
    Client:     cli,
    Collection: "hnsw_collection",
    SearchMode: hnswMode,
    Embedding:  emb,
})

// IVF_FLAT 搜索模式
ivfMode, _ := search_mode.SearchModeIvfFlat(&search_mode.IvfFlatConfig{
    Nprobe: 16,            // 搜索的聚类数
    Metric: entity.COSINE,
})
```
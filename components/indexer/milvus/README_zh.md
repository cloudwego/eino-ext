# Milvus 存储

[English](README.md) | [简体中文](README_zh.md)

基于 Milvus 2.x 的向量存储实现，为 [Eino](https://github.com/cloudwego/eino) 提供了符合 `Indexer` 接口的存储方案。该组件可无缝集成
Eino 的向量存储和检索系统，增强语义搜索能力。

## 快速开始

### 安装

它需要 milvus-sdk-go 客户端版本 2.4.x

```bash
go get github.com/milvus-io/milvus-sdk-go/v2@2.4.2
go get github.com/cloudwego/eino-ext/components/indexer/milvus@latest
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
	
	"github.com/cloudwego/eino-ext/components/indexer/milvus"
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
	// Client 是要调用的 milvus 客户端
	// 需要 milvus-sdk-go 客户端版本 2.4.x
	// 必需
	Client client.Client

	// 默认集合配置
	// Collection 是 milvus 数据库中的集合名称
	// 可选，默认值为 "eino_collection"
	Collection string
	// Description 是集合的描述
	// 可选，默认值为 "the collection for eino"
	Description string
	// PartitionNum 是集合分区数量
	// 可选，默认值为 1（禁用）
	// 如果分区数量大于 1，表示使用分区，并且必须在 Fields 中有一个分区键
	PartitionNum int64
	// PartitionName 是 milvus 数据库中的分区名称
	// 可选，默认值为 ""
	// 如果 PartitionNum 大于 1，表示使用分区键模式，不支持手动指定分区名称
	// 优先使用 WithPartition
	PartitionName string
	// Fields 是集合字段
	// 可选，默认值为默认字段
	Fields []*entity.Field
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
	DocumentConverter func(ctx context.Context, docs []*schema.Document, vectors [][]float64) ([]interface{}, error)

	// 向量列的索引配置
	// MetricType 是向量相似度计算的度量类型（Metric Type）
	// 可选，默认值为 COSINE
	// 常用值: L2（欧几里得）、IP（内积）、COSINE（余弦相似度）
	MetricType MetricType

	// IndexBuilder 定义向量字段的索引类型和参数
	// 可选；如果为 nil，将使用 AUTOINDEX（推荐用于大多数场景）
	// 使用 NewHNSWIndexBuilder、NewIvfFlatIndexBuilder、NewFlatIndexBuilder 或
	// NewAutoIndexBuilder 创建 IndexBuilder
	IndexBuilder IndexBuilder

	// VectorFieldName 是要创建索引的向量字段名称
	// 可选，默认值为 "vector"
	VectorFieldName string

	// Embedding 是从 schema.Document 的内容中嵌入值所需的向量化方法
	// 必需
	Embedding embedding.Embedder
}
```

## IndexBuilder

通过 `IndexBuilder` 接口灵活选择索引类型，根据您的使用场景选择合适的索引：

| 索引类型 | 适用场景 | 权衡 |
|----------|----------|------|
| `AUTOINDEX` | 大多数场景 | Milvus 自动优化（推荐） |
| `HNSW` | 高召回率需求 | 内存占用较高，搜索速度优秀 |
| `IVF_FLAT` | 大规模数据集 | 速度与精度的良好平衡 |
| `FLAT` | 小数据集，需要 100% 召回率 | 大数据集时较慢 |

**使用示例：**

```go
// AUTOINDEX（推荐用于大多数场景）
indexer, err := milvus.NewIndexer(ctx, &milvus.IndexerConfig{
    Client:     cli,
    Collection: "my_collection",
    MetricType: milvus.COSINE, // 或 milvus.L2, milvus.IP
    Embedding:  emb,
    // 未指定 IndexBuilder 时默认使用 AUTOINDEX
})

// HNSW - 高性能图索引
hnswBuilder, _ := milvus.NewHNSWIndexBuilder(16, 200) // M=16, efConstruction=200
indexer, err := milvus.NewIndexer(ctx, &milvus.IndexerConfig{
    Client:       cli,
    Collection:   "hnsw_collection",
    MetricType:   milvus.L2,
    IndexBuilder: hnswBuilder,
    Embedding:    emb,
})

// IVF_FLAT - 倒排文件索引，用于大规模搜索
ivfBuilder, _ := milvus.NewIvfFlatIndexBuilder(1024) // nlist=1024
indexer, err := milvus.NewIndexer(ctx, &milvus.IndexerConfig{
    Client:       cli,
    Collection:   "ivf_collection",
    MetricType:   milvus.COSINE,
    IndexBuilder: ivfBuilder,
    Embedding:    emb,
})
```

## 默认数据模型

| 字段       | 数据类型           | 字段类型         | 索引类型                       | 描述     | 备注          |
|----------|----------------|--------------|----------------------------|--------|-------------|
| id       | string         | varchar      |                            | 文章唯一标识 | 最大长度: 255   |
| content  | string         | varchar      |                            | 文章内容   | 最大长度: 1024  |
| vector   | []byte         | binary array | HAMMING(default) / JACCARD | 文章内容向量 | 默认维度: 81920 |
| metadata | map[string]any | json         |                            | 文章元数据  |             |

## 如何确定 dim 参数

转换关系为 `dim = embedding model output * 4 * 8`

首先，我们在将 `[]float64` 转换为 `DefaultDocumentConvert` 中的 `[]byte`，这导致向量维度的四倍扩展

```go
// vector2Bytes converts vector to bytes
func vector2Bytes(vector []float64) []byte {
float32Arr := make([]float32, len(vector))
for i, v := range vector {
float32Arr[i] = float32(v)
}

bytes := make([]byte, len(float32Arr)*4)

for i, v := range float32Arr {
binary.LittleEndian.PutUint32(bytes[i*4:], math.Float32bits(v))
}

return bytes
}
```

其次，我们可以参考 [Milvus 官方文档](https://milvus.io/api-reference/go/v2.4.x/Collection/Vectors.md)
在这里，我们的向量又经过了一次8倍的扩展

因此，我们可以得到以 milvus 向量列的 dim 与嵌入模型的输出纬度之间的转换关系, dim = embedding model output * 4 * 8
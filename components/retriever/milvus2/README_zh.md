# Milvus 2.x Retriever

[English](./README.md) | 中文

本包为 EINO 框架提供 Milvus 2.x (V2 SDK) 检索器实现，支持多种搜索模式的向量相似度搜索。

> **注意**: 本包需要 **Milvus 2.5+** 以支持服务器端函数（如 BM25）。

## 功能特性

- **Milvus V2 SDK**: 使用最新的 `milvus-io/milvus/client/v2` SDK
- **多种搜索模式**: 支持近似搜索、范围搜索、混合搜索、迭代器搜索和标量搜索
- **稠密 + 稀疏混合搜索**: 结合稠密向量和稀疏向量，使用 RRF 重排序
- **自定义结果转换**: 可配置的结果到文档转换

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/retriever/milvus2
```

## 快速开始

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/embedding/ark"
	"github.com/milvus-io/milvus/client/v2/milvusclient"

	milvus2 "github.com/cloudwego/eino-ext/components/retriever/milvus2"
	"github.com/cloudwego/eino-ext/components/retriever/milvus2/search_mode"
)

func main() {
	// 获取环境变量
	addr := os.Getenv("MILVUS_ADDR")
	username := os.Getenv("MILVUS_USERNAME")
	password := os.Getenv("MILVUS_PASSWORD")
	arkApiKey := os.Getenv("ARK_API_KEY")
	arkModel := os.Getenv("ARK_MODEL")

	ctx := context.Background()

	// 创建 embedding 模型
	emb, err := ark.NewEmbedder(ctx, &ark.EmbeddingConfig{
		APIKey: arkApiKey,
		Model:  arkModel,
	})
	if err != nil {
		log.Fatalf("Failed to create embedding: %v", err)
		return
	}

	// 创建 retriever
	retriever, err := milvus2.NewRetriever(ctx, &milvus2.RetrieverConfig{
		ClientConfig: &milvusclient.ClientConfig{
			Address:  addr,
			Username: username,
			Password: password,
		},
		Collection: "my_collection",
		TopK:       10,
		SearchMode: search_mode.NewApproximate(milvus2.COSINE),
		Embedding:  emb,
	})
	if err != nil {
		log.Fatalf("Failed to create retriever: %v", err)
		return
	}
	log.Printf("Retriever created successfully")

	// 检索文档
	documents, err := retriever.Retrieve(ctx, "search query")
	if err != nil {
		log.Fatalf("Failed to retrieve: %v", err)
		return
	}

	// 打印文档
	for i, doc := range documents {
		fmt.Printf("Document %d:\n", i)
		fmt.Printf("  ID: %s\n", doc.ID)
		fmt.Printf("  Content: %s\n", doc.Content)
		fmt.Printf("  Score: %v\n", doc.Score())
	}
}
```

## 配置选项

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `Client` | `*milvusclient.Client` | - | 预配置的 Milvus 客户端（可选） |
| `ClientConfig` | `*milvusclient.ClientConfig` | - | 客户端配置（Client 为空时必需） |
| `Collection` | `string` | `"eino_collection"` | 集合名称 |
| `TopK` | `int` | `5` | 返回结果数量 |
| `VectorField` | `string` | `"vector"` | 稠密向量字段名 |
| `SparseVectorField` | `string` | `"sparse_vector"` | 稀疏向量字段名 |
| `OutputFields` | `[]string` | 所有字段 | 结果中返回的字段 |
| `SearchMode` | `SearchMode` | - | 搜索策略（必需） |
| `Embedding` | `embedding.Embedder` | - | 用于查询向量化的 Embedder（必需） |
| `DocumentConverter` | `func` | 默认转换器 | 自定义结果到文档转换 |
| `ConsistencyLevel` | `ConsistencyLevel` | `ConsistencyLevelDefault` | 一致性级别 (`ConsistencyLevelDefault` 使用 collection 的级别；不应用按请求覆盖) |
| `Partitions` | `[]string` | - | 要搜索的分区 |

## 搜索模式

从 `github.com/cloudwego/eino-ext/components/retriever/milvus2/search_mode` 导入搜索模式。

### 近似搜索 (Approximate)

标准的近似最近邻 (ANN) 搜索。

```go
mode := search_mode.NewApproximate(milvus2.COSINE)
```

### 范围搜索 (Range)

在指定距离范围内搜索 (向量在 `Radius` 内)。

```go
// L2: 距离 <= Radius
// IP/Cosine: 分数 >= Radius
mode := search_mode.NewRange(milvus2.L2, 0.5).
    WithRangeFilter(0.1) // 可选: 环形搜索的内边界
```

### 稀疏搜索 (BM25)

使用 BM25 进行纯稀疏向量搜索。需要 Milvus 2.5+ 支持稀疏向量字段并启用 Functions。

```go
// 纯稀疏搜索 (BM25) 需要指定 OutputFields 以获取内容
// MetricType: BM25 (默认) 或 IP
mode := search_mode.NewSparse(milvus2.BM25)

// 在配置中，使用 "*" 或特定字段以确保返回内容:
// OutputFields: []string{"*"}
```

### 混合搜索 (Hybrid - 稠密 + 稀疏)

结合稠密向量和稀疏向量的多向量搜索，支持结果重排序。需要一个同时包含稠密和稀疏向量字段的集合（参见 indexer sparse 示例）。

```go
import (
    "github.com/milvus-io/milvus/client/v2/milvusclient"
    milvus2 "github.com/cloudwego/eino-ext/components/retriever/milvus2"
    "github.com/cloudwego/eino-ext/components/retriever/milvus2/search_mode"
)

// 定义稠密 + 稀疏子请求的混合搜索
hybridMode := search_mode.NewHybrid(
    milvusclient.NewRRFReranker().WithK(60), // RRF 重排序器
    &search_mode.SubRequest{
        VectorField: "vector",             // 稠密向量字段
        VectorType:  milvus2.DenseVector,  // 默认值，可省略
        TopK:        10,
        MetricType:  milvus2.L2,
    },
    // 稀疏子请求 (Sparse SubRequest)
    &search_mode.SubRequest{
        VectorField: "sparse_vector",       // 稀疏向量字段
        VectorType:  milvus2.SparseVector,  // 指定稀疏类型
        TopK:        10,
        MetricType:  milvus2.BM25,          // 使用 BM25 或 IP
    },
)

// 创建 retriever (稀疏向量生成由 Milvus Function 服务器端处理)
retriever, err := milvus2.NewRetriever(ctx, &milvus2.RetrieverConfig{
    ClientConfig:      &milvusclient.ClientConfig{Address: "localhost:19530"},
    Collection:        "hybrid_collection",
    VectorField:       "vector",             // 默认稠密字段
    SparseVectorField: "sparse_vector",      // 默认稀疏字段
    TopK:              5,
    SearchMode:        hybridMode,
    Embedding:         denseEmbedder,        // 稠密向量的标准 Embedder
})
```

### 迭代器搜索 (Iterator)

基于批次的遍历，适用于大结果集。

> [!WARNING]
> `Iterator` 模式的 `Retrieve` 方法会获取 **所有** 结果，直到达到总限制 (`TopK`) 或集合末尾。对于极大数据集，这可能会消耗大量内存。

```go
// 100 是批次大小 (每次网络调用的条目数)
mode := search_mode.NewIterator(milvus2.COSINE, 100).
    WithSearchParams(map[string]string{"nprobe": "10"})

// 使用 RetrieverConfig.TopK 设置总限制 (IteratorLimit)。
```

### 标量搜索 (Scalar)

仅基于元数据过滤，不使用向量相似度（将过滤表达式作为查询）。

```go
mode := search_mode.NewScalar()

// 使用过滤表达式查询
docs, err := retriever.Retrieve(ctx, `category == "electronics" AND year >= 2023`)
```

### 稠密向量度量 (Dense)
| 度量类型 | 描述 |
|----------|------|
| `L2` | 欧几里得距离 |
| `IP` | 内积 |
| `COSINE` | 余弦相似度 |

### 稀疏向量度量 (Sparse)
| 度量类型 | 描述 |
|----------|------|
| `BM25` | Okapi BM25 (BM25 搜索必需) |
| `IP` | 内积 (适用于预计算的稀疏向量) |

### 二进制向量度量 (Binary)
| 度量类型 | 描述 |
|----------|------|
| `HAMMING` | 汉明距离 |
| `JACCARD` | 杰卡德距离 |
| `TANIMOTO` | Tanimoto 距离 |
| `SUBSTRUCTURE` | 子结构搜索 |
| `SUPERSTRUCTURE` | 超结构搜索 |

> **重要提示**: SearchMode 中的度量类型必须与创建集合时使用的索引度量类型一致。

## 示例

查看 [examples](./examples) 目录获取完整的示例代码：

- [approximate](./examples/approximate) - 基础 ANN 搜索
- [range](./examples/range) - 范围搜索示例
- [hybrid](./examples/hybrid) - 混合多向量搜索 (稠密 + BM25)
- [hybrid_chinese](./examples/hybrid_chinese) - 中文混合搜索示例
- [iterator](./examples/iterator) - 批次迭代器搜索
- [scalar](./examples/scalar) - 标量/元数据过滤
- [grouping](./examples/grouping) - 分组搜索结果
- [filtered](./examples/filtered) - 带过滤的向量搜索
- [sparse](./examples/sparse) - 纯稀疏搜索示例 (BM25)

## 许可证

Apache License 2.0

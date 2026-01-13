# Milvus 2.x Indexer

[English](./README.md) | 中文

本包为 EINO 框架提供 Milvus 2.x (V2 SDK) 索引器实现，支持文档存储和向量索引。

> **注意**: 本包需要 **Milvus 2.5+** 以支持服务器端函数（如 BM25）。

## 功能特性

- **Milvus V2 SDK**: 使用最新的 `milvus-io/milvus/client/v2` SDK
- **灵活的索引类型**: 支持多种索引构建器，包括 Auto, HNSW, IVF 系列, SCANN, DiskANN, GPU 索引以及 RaBitQ (Milvus 2.6+)
- **混合搜索就绪**: 原生支持稀疏向量 (BM25/SPLADE) 与稠密向量的混合存储
- **服务端向量生成**: 使用 Milvus Functions (BM25) 自动生成稀疏向量
- **自动化管理**: 自动处理集合 Schema 创建、索引构建和加载
- **字段分析**: 可配置的文本分析器（支持中文 Jieba、英文、Standard 等）
- **自定义文档转换**: Eino 文档到 Milvus 列的灵活映射

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/indexer/milvus2
```

## 快速开始

```go
package main

import (
	"context"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/embedding/ark"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus/client/v2/milvusclient"

	milvus2 "github.com/cloudwego/eino-ext/components/indexer/milvus2"
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

	// 创建索引器
	indexer, err := milvus2.NewIndexer(ctx, &milvus2.IndexerConfig{
		ClientConfig: &milvusclient.ClientConfig{
			Address:  addr,
			Username: username,
			Password: password,
		},
		Collection:   "my_collection",
		Dimension:    1024, // 与 embedding 模型维度匹配
		MetricType:   milvus2.COSINE,
		IndexBuilder: milvus2.NewHNSWIndexBuilder().WithM(16).WithEfConstruction(200),
		Embedding:    emb,
	})
	if err != nil {
		log.Fatalf("Failed to create indexer: %v", err)
		return
	}
	log.Printf("Indexer created successfully")

	// 存储文档
	docs := []*schema.Document{
		{
			ID:      "doc1",
			Content: "Milvus is an open-source vector database",
			MetaData: map[string]any{
				"category": "database",
				"year":     2021,
			},
		},
		{
			ID:      "doc2",
			Content: "EINO is a framework for building AI applications",
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

## 配置选项

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `Client` | `*milvusclient.Client` | - | 预配置的 Milvus 客户端（可选） |
| `ClientConfig` | `*milvusclient.ClientConfig` | - | 客户端配置（Client 为空时必需） |
| `Collection` | `string` | `"eino_collection"` | 集合名称 |
| `Dimension` | `int64` | - | 向量维度（创建新集合时必需） |
| `VectorField` | `string` | `"vector"` | 向量字段名称 |
| `MetricType` | `MetricType` | `L2` | 相似度度量类型（L2, IP, COSINE 等） |
| `IndexBuilder` | `IndexBuilder` | AutoIndex | 索引类型构建器 |
| `Embedding` | `embedding.Embedder` | - | 用于向量化的 Embedder（可选）。如果为空，文档必须包含向量。 |
| `ConsistencyLevel` | `ConsistencyLevel` | `Default` | 一致性级别 (Default 使用 Milvus 默认: Bounded) |
| `PartitionName` | `string` | - | 插入数据的默认分区 |
| `EnableDynamicSchema` | `bool` | `false` | 启用动态字段支持 |
| `Sparse` | `*SparseIndexerConfig` | - | 稀疏向量索引配置（可选） |
| `Functions` | `[]*entity.Function` | - | Schema 函数定义（如 BM25），用于服务器端处理 |
| `FieldParams` | `map[string]map[string]string` | - | 字段参数配置（如 enable_analyzer） |

## 索引构建器

| 构建器 | 描述 | 关键参数 |
|--------|------|----------|
| `NewAutoIndexBuilder()` | Milvus 自动选择最优索引 | - |
| `NewHNSWIndexBuilder()` | 基于图的高性能索引 | `M`, `EfConstruction` |
| `NewIVFFlatIndexBuilder()` | 基于聚类的搜索 | `NList` |
| `NewIVFPQIndexBuilder()` | 乘积量化，内存高效 | `NList`, `M`, `NBits` |
| `NewIVFSQ8IndexBuilder()` | 标量量化 | `NList` |
| `NewIVFRabitQIndexBuilder()` | IVF + RaBitQ 二进制量化 (Milvus 2.6+) | `NList` |
| `NewFlatIndexBuilder()` | 暴力精确搜索 | - |
| `NewDiskANNIndexBuilder()` | 面向大数据集的磁盘索引 | - |
| `NewSCANNIndexBuilder()` | 高召回率的快速搜索 | `NList`, `WithReorder` |

#### 稀疏索引构建器

| 构建器 | 描述 | 关键参数 |
|--------|------|----------|
| `NewSparseInvertedIndexBuilder()` | 稀疏向量倒排索引 | `DropRatioBuild` |
| `NewSparseWANDIndexBuilder()` | 稀疏向量 WAND 算法 | `DropRatioBuild` |

### 示例：HNSW 索引

```go
indexBuilder := milvus2.NewHNSWIndexBuilder().
	WithM(16).              // 每个节点的最大连接数 (4-64)
	WithEfConstruction(200) // 索引构建时的搜索宽度 (8-512)
```

### 示例：IVF_FLAT 索引

```go
indexBuilder := milvus2.NewIVFFlatIndexBuilder().
	WithNList(256) // 聚类单元数量 (1-65536)
```

### 示例：IVF_PQ 索引（内存高效）

```go
indexBuilder := milvus2.NewIVFPQIndexBuilder().
	WithNList(256). // 聚类单元数量
	WithM(16).      // 子量化器数量
	WithNBits(8)    // 每个子量化器的位数 (1-16)
```

### 示例：SCANN 索引（高召回率快速搜索）

```go
indexBuilder := milvus2.NewSCANNIndexBuilder().
	WithNList(256).           // 聚类单元数量
	WithRawDataEnabled(true)  // 启用原始数据进行重排序
```

### 示例：DiskANN 索引（大数据集）

```go
indexBuilder := milvus2.NewDiskANNIndexBuilder() // 基于磁盘，无额外参数
```

## 度量类型 (Metric Type)

| 度量类型 | 描述 |
|----------|------|
| `L2` | 欧几里得距离 |
| `IP` | 内积 |
| `COSINE` | 余弦相似度 |
| `HAMMING` | 汉明距离（二进制） |
| `JACCARD` | 杰卡德距离（二进制） |
| `BM25` | Okapi BM25 (稀疏) |

## 示例

查看 [examples](./examples) 目录获取完整的示例代码：

- [demo](./examples/demo) - 使用 HNSW 索引的基础集合设置
- [hnsw](./examples/hnsw) - HNSW 索引示例
- [ivf_flat](./examples/ivf_flat) - IVF_FLAT 索引示例
- [rabitq](./examples/rabitq) - IVF_RABITQ 索引示例 (Milvus 2.6+)
- [auto](./examples/auto) - AutoIndex 示例
- [diskann](./examples/diskann) - DISKANN 索引示例
- [hybrid](./examples/hybrid) - 混合搜索设置 (稠密 + BM25 稀疏) (Milvus 2.5+)
- [hybrid_chinese](./examples/hybrid_chinese) - 中文混合搜索示例 (Milvus 2.5+)
- [byov](./examples/byov) - 自带向量示例

### 稀疏向量支持

使用 Milvus 服务器端函数（如 BM25）从文本内容自动生成稀疏向量：

```go
// 创建带有函数的 indexer
indexer, err := milvus2.NewIndexer(ctx, &milvus2.IndexerConfig{
    // ... 基础配置 ...
    Collection:        "hybrid_collection",
    
    // 启用稀疏向量支持
    Sparse: &milvus2.SparseIndexerConfig{
        VectorField: "sparse_vector",
        MetricType:  milvus2.BM25,
        Method:      milvus2.SparseMethodAuto, // 使用 BM25 自动生成
    },
    
    // 当 Method=Auto 时会自动检测 BM25 配置
    // 如果需要显式添加函数:
    // Functions: []*entity.Function{bm25Function},
    
    // BM25 需要在内容字段上启用分析器
    // 分析器选项 (内置):
    // - {"type": "standard"} - 通用, 分词 + 小写
    // - {"type": "english"}  - 英文, 支持停用词
    // - {"type": "chinese"}  - 中文, Jieba 分词
    // - 自定义: {"tokenizer": "...", "filter": [...]}
    // 参考: https://milvus.io/docs/analyzer-overview.md
    FieldParams: map[string]map[string]string{
        "content": {
            "enable_analyzer": "true",
            "analyzer_params": `{"type": "standard"}`, // 中文使用 {"type": "chinese"}
        },
    },
})
```

### 自带向量 (Bring Your Own Vectors)

如果您的文档已经包含向量，可以不配置 Embedder 使用 Indexer。

```go
// 创建不带 embedding 的 indexer
indexer, err := milvus2.NewIndexer(ctx, &milvus2.IndexerConfig{
    ClientConfig: &milvusclient.ClientConfig{
        Address: "localhost:19530",
    },
    Collection:   "my_collection",
    Dimension:    128,
    // Embedding: nil, // 留空
})

// 存储带有预计算向量的文档
docs := []*schema.Document{
    {
        ID:      "doc1",
        Content: "Document with existing vector",
    },
}

// 附加稠密向量到文档
// 向量维度必须与集合维度匹配
vector := []float64{0.1, 0.2, ...} 
docs[0].WithDenseVector(vector)

// 附加稀疏向量（可选，如果配置了 Sparse）
// 稀疏向量是 index -> weight 的映射
sparseVector := map[int]float64{
    10: 0.5,
    25: 0.8,
}
docs[0].WithSparseVector(sparseVector)

ids, err := indexer.Store(ctx, docs)
```

对于 BYOV 模式下的稀疏向量，请确保您的配置使用 `Method: milvus2.SparseMethodPrecomputed`：

```go
Sparse: &milvus2.SparseIndexerConfig{
    VectorField: "sparse_vector",
    MetricType:  milvus2.IP, // 或 BM25（如适用）
    Method:      milvus2.SparseMethodPrecomputed,
},
```

## 许可证

Apache License 2.0

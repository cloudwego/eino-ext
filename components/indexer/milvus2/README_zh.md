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

		Vector: &milvus2.VectorConfig{
			Dimension:  1024, // 与 embedding 模型维度匹配
			MetricType: milvus2.COSINE,
			IndexBuilder: milvus2.NewHNSWIndexBuilder().WithM(16).WithEfConstruction(200),
		},
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
| `Vector` | `*VectorConfig` | - | 稠密向量配置 (维度, MetricType, 字段名) |
| `Sparse` | `*SparseVectorConfig` | - | 稀疏向量配置 (MetricType, 字段名) |
| `IndexBuilder` | `IndexBuilder` | `AutoIndexBuilder` | 索引类型构建器 |
| `Embedding` | `embedding.Embedder` | - | 用于向量化的 Embedder（可选）。如果为空，文档必须包含向量 (BYOV)。 |
| `ConsistencyLevel` | `ConsistencyLevel` | `ConsistencyLevelDefault` | 一致性级别 (`ConsistencyLevelDefault` 使用 Milvus 默认: Bounded; 如果未显式设置，则保持集合级别设置) |
| `PartitionName` | `string` | - | 插入数据的默认分区 |
| `EnableDynamicSchema` | `bool` | `false` | 启用动态字段支持 |
| `Functions` | `[]*entity.Function` | - | Schema 函数定义（如 BM25），用于服务器端处理 |
| `FieldParams` | `map[string]map[string]string` | - | 字段参数配置（如 enable_analyzer） |

### 稠密向量配置 (`VectorConfig`)

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `Dimension` | `int64` | - | 向量维度 (必需) |
| `MetricType` | `MetricType` | `L2` | 相似度度量类型 (L2, IP, COSINE 等) |
| `VectorField` | `string` | `"vector"` | 稠密向量字段名 |

### 稀疏向量配置 (`SparseVectorConfig`)

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `VectorField` | `string` | `"sparse_vector"` | 稀疏向量字段名 |
| `MetricType` | `MetricType` | `BM25` | 相似度度量类型 |
| `Method` | `SparseMethod` | `SparseMethodAuto` | 生成方法 (`SparseMethodAuto` 或 `SparseMethodPrecomputed`) |
| `IndexBuilder` | `SparseIndexBuilder` | `SparseInvertedIndex` | 索引构建器 (`NewSparseInvertedIndexBuilder` 或 `NewSparseWANDIndexBuilder`) |

> **注意**: 仅当 `MetricType` 为 `BM25` 时，`Method` 默认为 `Auto`。`Auto` 意味着使用 Milvus 服务器端函数（远程函数）。对于其他度量类型（如 `IP`），默认为 `Precomputed`。

## 索引构建器

### 稠密索引构建器 (Dense)

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
| `NewSCANNIndexBuilder()` | 高召回率的快速搜索 | `NList`, `WithRawDataEnabled` |
| `NewBinFlatIndexBuilder()` | 二进制向量的暴力搜索 | - |
| `NewBinIVFFlatIndexBuilder()` | 二进制向量的聚类搜索 | `NList` |
| `NewGPUBruteForceIndexBuilder()` | GPU 加速暴力搜索 | - |
| `NewGPUIVFFlatIndexBuilder()` | GPU 加速 IVF_FLAT | - |
| `NewGPUIVFPQIndexBuilder()` | GPU 加速 IVF_PQ | - |
| `NewGPUCagraIndexBuilder()` | GPU 加速图索引 (CAGRA) | `IntermediateGraphDegree`, `GraphDegree` |

### 稀疏索引构建器 (Sparse)

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

### 示例：Sparse Inverted Index (稀疏倒排索引)

```go
indexBuilder := milvus2.NewSparseInvertedIndexBuilder().
	WithDropRatioBuild(0.2) // 构建时忽略小值的比例 (0.0-1.0)
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
| `BM25` | Okapi BM25 (`SparseMethodAuto` 必需) |
| `IP` | 内积 (适用于预计算的稀疏向量) |

### 二进制向量度量 (Binary)
| 度量类型 | 描述 |
|----------|------|
| `HAMMING` | 汉明距离 |
| `JACCARD` | 杰卡德距离 |
| `TANIMOTO` | Tanimoto 距离 |
| `SUBSTRUCTURE` | 子结构搜索 |
| `SUPERSTRUCTURE` | 超结构搜索 |

## 稀疏向量支持

索引器支持两种稀疏向量模式：**自动生成 (Auto-Generation)** 和 **预计算 (Precomputed)**。

### 1. 自动生成 (BM25)

使用 Milvus 服务器端函数从内容字段自动生成稀疏向量。

- **要求**: Milvus 2.5+
- **配置**: 设置 `MetricType: milvus2.BM25`。

```go
indexer, err := milvus2.NewIndexer(ctx, &milvus2.IndexerConfig{
    // ... 基础配置 ...
    Collection:        "hybrid_collection",
    
    Sparse: &milvus2.SparseVectorConfig{
        VectorField: "sparse_vector",
        MetricType:  milvus2.BM25, 
        // BM25 时 Method 默认为 SparseMethodAuto
    },
    
    // BM25 的分析器配置
    FieldParams: map[string]map[string]string{
        "content": {
            "enable_analyzer": "true",
            "analyzer_params": `{"type": "standard"}`, // 中文可使用 {"type": "chinese"}
        },
    },
})
```

### 2. 预计算 (SPLADE, BGE-M3 等)

允许存储由外部模型（如 SPLADE, BGE-M3）或自定义逻辑生成的稀疏向量。

- **配置**: 设置 `MetricType`（通常为 `IP`）和 `Method: milvus2.SparseMethodPrecomputed`。
- **用法**: 通过 `doc.WithSparseVector()` 传入稀疏向量。

```go
indexer, err := milvus2.NewIndexer(ctx, &milvus2.IndexerConfig{
    Collection: "sparse_collection",
    
    Sparse: &milvus2.SparseVectorConfig{
        VectorField: "sparse_vector",
        MetricType:  milvus2.IP,
        Method:      milvus2.SparseMethodPrecomputed,
    },
})

// 存储包含稀疏向量的文档
doc := &schema.Document{ID: "1", Content: "..."}
doc.WithSparseVector(map[int]float64{
    1024: 0.5,
    2048: 0.3,
})
indexer.Store(ctx, []*schema.Document{doc})
```

## 自带向量 (Bring Your Own Vectors)

如果您的文档已经包含向量，可以不配置 Embedder 使用 Indexer。

```go
// 创建不带 embedding 的 indexer
indexer, err := milvus2.NewIndexer(ctx, &milvus2.IndexerConfig{
    ClientConfig: &milvusclient.ClientConfig{
        Address: "localhost:19530",
    },
    Collection:   "my_collection",
    Vector: &milvus2.VectorConfig{
        Dimension:  128,
        MetricType: milvus2.L2,
    },
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

对于 BYOV 模式下的稀疏向量，请参考上文 **预计算 (Precomputed)** 部分进行配置。

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
- [sparse](./examples/sparse) - 纯稀疏索引示例 (BM25)
- [byov](./examples/byov) - 自带向量示例

## 许可证

Apache License 2.0

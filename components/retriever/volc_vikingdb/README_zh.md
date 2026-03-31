# VikingDB Retriever

[English](README.md) | 中文

[Eino](https://github.com/cloudwego/eino) 的 VikingDB 检索器实现，实现了 `Retriever` 接口。该组件集成火山引擎 VikingDB 服务，基于语义相似度检索文档。

## 功能特性

- 实现了 `github.com/cloudwego/eino/components/retriever.Retriever`
- 易于集成 Eino 检索系统
- 支持火山引擎 VikingDB 索引
- 多种 embedding 模式：
  - 内置 VikingDB embedding（Embedding V2）
  - 使用自定义 embedder 的自定义 embedding
  - 平台向量化的多模态搜索
- 稠密和稀疏向量搜索
- 可配置的 top-k 结果
- 分数阈值过滤
- 基于 DSL 的过滤支持
- 分区（子索引）支持

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/retriever/volc_vikingdb@latest
```

## 快速开始

### 使用内置 VikingDB Embedding

```go
import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/eino-ext/components/retriever/volc_vikingdb"
)

func main() {
	ctx := context.Background()

	// 使用内置 embedding 创建 VikingDB 检索器
	retriever, _ := volc_vikingdb.NewRetriever(ctx, &volc_vikingdb.RetrieverConfig{
		Host:       "api-vikingdb.volces.com",
		Region:     "cn-beijing",
		AK:         "your-access-key",
		SK:         "your-secret-key",
		Scheme:     "https",
		Collection: "your_collection_name",
		Index:      "your_index_name",
		EmbeddingConfig: volc_vikingdb.EmbeddingConfig{
			UseBuiltin:  true,
			ModelName:   "bge-large-zh",
			UseSparse:   false,
			DenseWeight: 0.5,  // 用于混合搜索
		},
		TopK: ptrOf(10),
	})

	// 检索文档
	docs, err := retriever.Retrieve(ctx, "search query")
	if err != nil {
		fmt.Printf("retrieve error: %v\n", err)
		return
	}

	for _, doc := range docs {
		fmt.Printf("ID: %s, Content: %s, Score: %f\n", 
			doc.ID, doc.Content, doc.Score())
	}
}

func ptrOf[T any](v T) *T {
	return &v
}
```

### 使用自定义 Embedding

```go
import (
	"context"
	"os"

	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/eino-ext/components/embedding/ark"
	"github.com/cloudwego/eino-ext/components/retriever/volc_vikingdb"
)

func main() {
	ctx := context.Background()

	// 创建自定义 embedding 组件
	emb, _ := ark.NewEmbedder(ctx, &ark.EmbeddingConfig{
		APIKey: os.Getenv("ARK_API_KEY"),
		Region: os.Getenv("ARK_REGION"),
		Model:  os.Getenv("ARK_MODEL"),
	})

	// 使用自定义 embedding 创建 VikingDB 检索器
	retriever, _ := volc_vikingdb.NewRetriever(ctx, &volc_vikingdb.RetrieverConfig{
		Host:       "api-vikingdb.volces.com",
		Region:     "cn-beijing",
		AK:         "your-access-key",
		SK:         "your-secret-key",
		Scheme:     "https",
		Collection: "your_collection_name",
		Index:      "your_index_name",
		EmbeddingConfig: volc_vikingdb.EmbeddingConfig{
			UseBuiltin: false,
			Embedding:  emb,
		},
	})

	docs, _ := retriever.Retrieve(ctx, "search query")
	for _, doc := range docs {
		fmt.Printf("ID: %s, Content: %s\n", doc.ID, doc.Content)
	}
}
```

### 使用多模态搜索

对于具有平台向量化的数据集：

```go
retriever, _ := volc_vikingdb.NewRetriever(ctx, &volc_vikingdb.RetrieverConfig{
	Host:           "api-vikingdb.volces.com",
	Region:         "cn-beijing",
	AK:             "your-access-key",
	SK:             "your-secret-key",
	Collection:     "your_collection_name",
	Index:          "your_index_name",
	WithMultiModal: true,  // 启用多模态搜索
})

// 直接使用文本查询进行搜索
docs, _ := retriever.Retrieve(ctx, "search query")
```

## 配置

```go
type RetrieverConfig struct {
    // VikingDB 连接设置
    Host              string  // 必填：VikingDB 服务主机
    Region            string  // 必填：服务区域（例如 "cn-beijing"）
    AK                string  // 必填：访问密钥
    SK                string  // 必填：密钥
    Scheme            string  // 选填："http" 或 "https"（默认："https"）
    ConnectionTimeout int64   // 选填：连接超时时间（秒）

    // 集合和索引设置
    Collection string  // 必填：集合名称
    Index      string  // 必填：索引名称

    // 多模态支持
    // 如果数据集在平台上进行向量化，设置为 true
    // 为 true 时，无需配置 EmbeddingConfig
    WithMultiModal bool

    // Embedding 配置
    EmbeddingConfig EmbeddingConfig

    // 选填：分区（子索引）字段（默认："default"）
    Partition string

    // 选填：返回的结果数量（默认：100）
    TopK *int

    // 选填：结果的最小分数阈值
    ScoreThreshold *float64

    // 选填：DSL 过滤表达式
    // 参见：https://www.volcengine.com/docs/84313/1254609
    FilterDSL map[string]any
}

type EmbeddingConfig struct {
    // UseBuiltin 决定是否使用 VikingDB 内置 embedding（Embedding V2）
    // 为 true 时，配置 ModelName 和 UseSparse
    // 为 false 时，配置 Embedding
    // 参见：https://www.volcengine.com/docs/84313/1254568
    UseBuiltin bool

    // ModelName 指定内置模型名称
    ModelName string

    // UseSparse 决定是否返回稀疏向量
    // 用于带稀疏向量的混合索引搜索
    UseSparse bool

    // DenseWeight 控制混合搜索中稠密向量的权重
    // 范围：[0.2, 1.0]，默认：0.5
    // 仅对混合索引有效
    DenseWeight float64

    // Embedding 在 UseBuiltin 为 false 时使用
    Embedding embedding.Embedder
}
```

## 高级特性

### 分数阈值过滤

通过最小分数过滤结果：

```go
threshold := 0.8

retriever, _ := volc_vikingdb.NewRetriever(ctx, &volc_vikingdb.RetrieverConfig{
	// ... 其他配置
	ScoreThreshold: &threshold,
})

docs, _ := retriever.Retrieve(ctx, "search query")
```

### DSL 过滤

使用 DSL 表达式进行高级过滤：

```go
import "github.com/cloudwego/eino-ext/components/retriever/volc_vikingdb"

filterDSL := map[string]any{
	"op": "and",
	"conditions": []map[string]any{
		{
			"op":    "range",
			"field": "price",
			"gte":   100,
			"lt":    500,
		},
		{
			"op":    "term",
			"field": "category",
			"value": "electronics",
		},
	},
}

retriever, _ := volc_vikingdb.NewRetriever(ctx, &volc_vikingdb.RetrieverConfig{
	// ... 其他配置
	FilterDSL: filterDSL,
})

docs, _ := retriever.Retrieve(ctx, "search query")
```

### 分区支持

在特定分区（子索引）中搜索：

```go
retriever, _ := volc_vikingdb.NewRetriever(ctx, &volc_vikingdb.RetrieverConfig{
	// ... 其他配置
	Partition: "partition_2023",
})
```

### 使用稀疏向量的混合搜索

对于支持稀疏向量的索引：

```go
retriever, _ := volc_vikingdb.NewRetriever(ctx, &volc_vikingdb.RetrieverConfig{
	// ... 其他配置
	EmbeddingConfig: volc_vikingdb.EmbeddingConfig{
		UseBuiltin:  true,
		ModelName:   "model-with-sparse-support",
		UseSparse:   true,
		DenseWeight: 0.7,  // 稠密向量 70% 权重
	},
})
```

## 运行时选项

在检索时覆盖配置：

```go
import "github.com/cloudwego/eino/components/retriever"

docs, _ := retriever.Retrieve(ctx, "search query",
	retriever.WithTopK(20),
	retriever.WithScoreThreshold(0.9),
)
```

## 更多详情

- [Eino 文档](https://www.cloudwego.io/zh/docs/eino/)
- [VikingDB 文档](https://www.volcengine.com/docs/84313/1254568)
- [VikingDB DSL 过滤](https://www.volcengine.com/docs/84313/1254609)
- [VikingDB Go SDK](https://github.com/volcengine/volc-sdk-golang)
## 示例

查看以下示例了解更多用法：

- [嵌入检索器](./examples/embed_retriever/)
- [基础检索器](./examples/retriever/)


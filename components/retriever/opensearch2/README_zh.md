# OpenSearch 2 Retriever

[English](README.md) | 简体中文

[Eino](https://github.com/cloudwego/eino) 的 OpenSearch 2 检索器实现，实现了 `Retriever` 接口。这使得 OpenSearch 可以无缝集成到 Eino 的向量检索系统中，增强语义搜索能力。

## 功能特性

- 实现 `github.com/cloudwego/eino/components/retriever.Retriever`
- 易于集成到 Eino 的检索系统
- 可配置的 OpenSearch 参数
- 支持向量相似度搜索和关键词搜索
- 多种搜索模式：
  - KNN (近似最近邻)
  - Exact Match (精确匹配/关键词)
  - Raw String (原生 JSON 请求体)
  - Dense Vector Similarity (脚本评分，稠密向量)
  - Neural Sparse (稀疏向量)
- 支持自定义结果解析

## 搜索模式兼容性

| 搜索模式 | 最低 OpenSearch 版本 | 说明 |
|-------------|----------------------------|-------|
| `ExactMatch` | 1.0+ | 标准查询 DSL |
| `RawString` | 1.0+ | 标准查询 DSL |
| `DenseVectorSimilarity` | 1.0+ | 使用 `script_score` 和 painless 向量函数 |
| `Approximate` (KNN) | 1.0+ | 自 1.0 起支持基础 KNN。高效过滤 (Post-filtering) 需要 2.4+ (Lucene HNSW) 或 2.9+ (Faiss)。 |
| `Approximate` (Hybrid) | 2.10+ | 生成 `bool` 查询。需要 2.10+ `normalization-processor` 支持高级分数归一化 (Convex Combination)。基础 `bool` 查询在早期版本 (1.0+) 也可工作。 |
| `Approximate` (RRF) | 2.19+ | 需要 `score-ranker-processor` (2.19+) 和 `neural-search` 插件。 |
| `NeuralSparse` (Query Text) | 2.11+ | 需要 `neural-search` 插件和已部署的模型。 |
| `NeuralSparse` (TokenWeights) | 2.11+ | 需要 `neural-search` 插件。 |

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/retriever/opensearch2@latest
```

## 快速开始

以下是一个如何使用该检索器的简单示例：

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	
	"github.com/cloudwego/eino/schema"
	opensearch "github.com/opensearch-project/opensearch-go/v2"

	"github.com/cloudwego/eino-ext/components/embedding/ark"
	"github.com/cloudwego/eino-ext/components/retriever/opensearch2"
	"github.com/cloudwego/eino-ext/components/retriever/opensearch2/search_mode"
)

func main() {
	ctx := context.Background()

	client, err := opensearch.NewClient(opensearch.Config{
		Addresses: []string{"http://localhost:9200"},
	})
	if err != nil {
		log.Fatal(err)
	}

	// 使用 Volcengine ARK 创建 embedding 组件
	emb, _ := ark.NewEmbedder(ctx, &ark.EmbeddingConfig{
		APIKey: os.Getenv("ARK_API_KEY"),
		Region: os.Getenv("ARK_REGION"),
		Model:  os.Getenv("ARK_MODEL"),
	})

	// 创建检索器组件
	retriever, _ := opensearch2.NewRetriever(ctx, &opensearch2.RetrieverConfig{
		Client: client,
		Index:  "your_index_name",
		TopK:   5,
		// 选择搜索模式
		SearchMode: search_mode.Approximate(&search_mode.ApproximateConfig{
			VectorField: "content_vector",
			K:           5,
		}),
		ResultParser: func(ctx context.Context, hit map[string]interface{}) (*schema.Document, error) {
			// 解析 hit map 为 Document
			id, _ := hit["_id"].(string)
			source := hit["_source"].(map[string]interface{})
			content, _ := source["content"].(string)
			return &schema.Document{ID: id, Content: content}, nil
		},
		Embedding: emb,
	})

	docs, err := retriever.Retrieve(ctx, "search query")
	if err != nil {
		fmt.Printf("retrieve error: %v\n", err)
		return
	}
	for _, doc := range docs {
		fmt.Printf("ID: %s, Content: %s\n", doc.ID, doc.Content)
	}
}
```

## 配置说明

可以通过 `RetrieverConfig` 结构体配置检索器：

```go
type RetrieverConfig struct {
    Client *opensearch.Client // 必填：OpenSearch 客户端实例
    Index  string             // 必填：从中检索文档的索引名称
    TopK   int                // 必填：返回的结果数量

    // 必填：搜索模式配置
    // search_mode 包中提供了预置实现：
    // - search_mode.Approximate(&ApproximateConfig{...})
    // - search_mode.ExactMatch(field)
    // - search_mode.RawStringRequest()
    // - search_mode.DenseVectorSimilarity(type, vectorField)
    // - search_mode.NeuralSparse(vectorField, &NeuralSparseConfig{...})
    SearchMode SearchMode

    // 选填：将 OpenSearch hits (map[string]interface{}) 解析为 Document 的函数
    // 如果未提供，将使用默认解析器。
    ResultParser func(ctx context.Context, hit map[string]interface{}) (doc *schema.Document, err error)

    // 选填：仅当需要查询向量化时必填
    Embedding embedding.Embedder
}
```

## 完整示例

- [近似搜索示例](./examples/approximate)
- [稠密向量相似度示例](./examples/dense_vector_similarity)
- [KNN 搜索示例](./examples/knn)
- [Neural Sparse 搜索示例](./examples/neural_sparse)

## 更多详情

- [Eino 文档](https://www.cloudwego.io/zh/docs/eino/)
- [OpenSearch Go 客户端文档](https://github.com/opensearch-project/opensearch-go)

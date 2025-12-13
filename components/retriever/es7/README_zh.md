# ES7 Retriever

[English](README.md)

[Eino](https://github.com/cloudwego/eino) 的 Elasticsearch 7.x 检索器实现，实现了 `Retriever` 接口。该组件可以与 Eino 的检索系统无缝集成，提供增强的语义搜索能力。

## 功能特性

- 实现了 `github.com/cloudwego/eino/components/retriever.Retriever`
- 易于集成 Eino 检索系统
- 可配置 Elasticsearch 参数
- 多种搜索模式：
  - 精确匹配（文本搜索）
  - 稠密向量相似度（语义搜索）
  - 原始字符串（自定义查询）
- 支持默认结果解析器及自定义
- 支持过滤器以进行精细查询

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/retriever/es7@latest
```

## 快速开始

以下是一个使用检索器的简单示例：

```go
import (
	"context"
	"fmt"
	"os"

	"github.com/cloudwego/eino/schema"
	"github.com/elastic/go-elasticsearch/v7"

	"github.com/cloudwego/eino-ext/components/retriever/es7"
	"github.com/cloudwego/eino-ext/components/retriever/es7/search_mode"
)

func main() {
	ctx := context.Background()

	// 连接到 Elasticsearch
	username := os.Getenv("ES_USERNAME")
	password := os.Getenv("ES_PASSWORD")

	client, _ := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{"http://localhost:9200"},
		Username:  username,
		Password:  password,
	})

	// 创建用于向量搜索的 embedding 组件
	emb := createYourEmbedding()

	// 创建带有稠密向量相似度搜索的检索器
	retriever, _ := es7.NewRetriever(ctx, &es7.RetrieverConfig{
		Client:     client,
		Index:      "my_index",
		TopK:       10,
		SearchMode: search_mode.DenseVectorSimilarity(search_mode.DenseVectorSimilarityTypeCosineSimilarity, "content_vector"),
		Embedding:  emb,
	})

	// 检索文档
	docs, _ := retriever.Retrieve(ctx, "search query")

	for _, doc := range docs {
		fmt.Printf("ID: %s, Content: %s, Score: %v\n", doc.ID, doc.Content, doc.MetaData["score"])
	}
}
```

## 搜索模式

### 精确匹配 (Exact Match)

使用 Elasticsearch match 查询进行简单的文本搜索：

```go
searchMode := search_mode.ExactMatch("content")
```

### 稠密向量相似度 (Dense Vector Similarity)

使用带有稠密向量的 script_score 进行语义搜索：

```go
// 余弦相似度
searchMode := search_mode.DenseVectorSimilarity(
    search_mode.DenseVectorSimilarityTypeCosineSimilarity,
    "content_vector",
)

// 其他相似度类型：
// - DenseVectorSimilarityTypeDotProduct
// - DenseVectorSimilarityTypeL1Norm
// - DenseVectorSimilarityTypeL2Norm
```

### 原始字符串请求 (Raw String Request)

直接传递自定义 JSON 查询：

```go
searchMode := search_mode.RawStringRequest()

// 然后使用 JSON 查询字符串作为搜索查询
query := `{"query": {"bool": {"must": [{"match": {"content": "search term"}}]}}}`
docs, _ := retriever.Retrieve(ctx, query)
```

## 配置

```go
type RetrieverConfig struct {
    Client         *elasticsearch.Client  // 必填：Elasticsearch 客户端
    Index          string                 // 必填：索引名称
    TopK           int                    // 选填：结果数量（默认：10）
    ScoreThreshold *float64               // 选填：最低分数阈值
    SearchMode     SearchMode             // 必填：搜索策略
    ResultParser   func(ctx context.Context, hit map[string]interface{}) (*schema.Document, error) // 选填：自定义解析器
    Embedding      embedding.Embedder     // 向量搜索模式必填
}
```

## 使用过滤器

使用 `WithFilters` 选项添加查询过滤器：

```go
filters := []interface{}{
    map[string]interface{}{
        "term": map[string]interface{}{
            "category": "news",
        },
    },
}

docs, _ := retriever.Retrieve(ctx, "query", es7.WithFilters(filters))
```

## 完整示例

- [索引器示例](../../indexer/es7/examples/indexer)
- [稠密向量相似度示例](./examples/dense_vector_similarity)
- [精确匹配示例](./examples/exact_match)
- [原始字符串请求示例](./examples/raw_string)

## 更多详情

- [Eino 文档](https://www.cloudwego.io/zh/docs/eino/)
- [Elasticsearch Go 客户端文档](https://github.com/elastic/go-elasticsearch)
- [Elasticsearch 7.10 Query DSL](https://www.elastic.co/guide/en/elasticsearch/reference/7.10/query-dsl.html)

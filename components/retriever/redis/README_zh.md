# Redis Retriever

[English](README.md) | 中文

[Eino](https://github.com/cloudwego/eino) 的 Redis 检索器实现，实现了 `Retriever` 接口。该组件使用 Redis 向量搜索功能（FT.SEARCH）基于语义相似度检索文档。

## 功能特性

- 实现了 `github.com/cloudwego/eino/components/retriever.Retriever`
- 易于集成 Eino 检索系统
- 两种搜索模式：
  - KNN 向量搜索返回 top-k 结果
  - 基于距离阈值的向量范围搜索
- 支持自定义过滤器以精细化查询
- 可配置返回字段
- 支持自定义文档解析器
- 基于距离的排序

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/retriever/redis@latest
```

## 前置要求

**重要**：要使用 Redis 向量搜索，您的 Redis 客户端必须配置：
1. **Protocol 2**：默认是 3，必须设置为 2 才能使用 FT.SEARCH
2. **UnstableResp3**：默认是 false，必须设置为 true

```go
client := redis.NewClient(&redis.Options{
    Addr:     "localhost:6379",
    Protocol: 2,              // FT.SEARCH 必需
})
client.Options().UnstableResp3 = true  // 向量搜索必需
```

## 快速开始

### KNN 向量搜索

```go
import (
	"context"
	"fmt"
	"os"

	"github.com/redis/go-redis/v9"
	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/components/embedding/ark"
	redisRetriever "github.com/cloudwego/eino-ext/components/retriever/redis"
)

func main() {
	ctx := context.Background()

	// 1. 创建正确配置的 Redis 客户端
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
		Protocol: 2,  // FT.SEARCH 必需
	})
	client.Options().UnstableResp3 = true  // 必需

	// 2. 创建 embedding 组件
	emb, _ := ark.NewEmbedder(ctx, &ark.EmbeddingConfig{
		APIKey: os.Getenv("ARK_API_KEY"),
		Region: os.Getenv("ARK_REGION"),
		Model:  os.Getenv("ARK_MODEL"),
	})

	// 3. 使用 KNN 搜索创建 Redis 检索器
	retriever, _ := redisRetriever.NewRetriever(ctx, &redisRetriever.RetrieverConfig{
		Client:      client,
		Index:       "my_index",
		VectorField: "content_vector",
		TopK:        5,
		Embedding:   emb,
	})

	// 4. 检索文档
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

### 向量范围搜索

使用距离阈值进行基于范围的搜索：

```go
threshold := 0.8

retriever, _ := redisRetriever.NewRetriever(ctx, &redisRetriever.RetrieverConfig{
	Client:            client,
	Index:             "my_index",
	VectorField:       "content_vector",
	DistanceThreshold: &threshold,  // 启用范围搜索
	Embedding:         emb,
})

docs, _ := retriever.Retrieve(ctx, "search query")
```

## 配置

```go
type RetrieverConfig struct {
    // 必填：Redis 客户端实例（必须设置 Protocol=2 和 UnstableResp3=true）
    Client *redis.Client

    // 必填：向量搜索的索引名称
    Index string

    // 选填：搜索查询中的向量字段名称（默认："vector_content"）
    // 应该与 redis indexer 的 EmbedKey 匹配
    VectorField string

    // 选填：范围搜索的距离阈值
    // 如果设置：使用向量范围搜索
    // 如果为 nil：使用 KNN 向量搜索（默认）
    DistanceThreshold *float64

    // 选填：查询方言（默认：2）
    // 参见：https://redis.io/docs/latest/develop/interact/search-and-query/advanced-concepts/dialects/
    Dialect int

    // 选填：从文档返回的字段（默认：["content", "vector_content"]）
    ReturnFields []string

    // 选填：自定义文档转换器
    // 默认：defaultResultParser
    DocumentConverter func(ctx context.Context, doc redis.Document) (*schema.Document, error)

    // 选填：返回的结果数量（默认：5）
    TopK int

    // 必填：查询向量化的 embedding 方法
    Embedding embedding.Embedder
}
```

## 搜索模式

### KNN 向量搜索

返回最相似的 top-k 文档：

```go
retriever, _ := redisRetriever.NewRetriever(ctx, &redisRetriever.RetrieverConfig{
    Client:      client,
    Index:       "my_index",
    VectorField: "content_vector",
    TopK:        10,  // 返回 top 10 结果
    Embedding:   emb,
})
```

查询格式：`(*)=>[KNN 10 @content_vector $vector AS __vector_distance]`

### 向量范围搜索

返回距离阈值内的所有文档：

```go
threshold := 0.5

retriever, _ := redisRetriever.NewRetriever(ctx, &redisRetriever.RetrieverConfig{
    Client:            client,
    Index:             "my_index",
    VectorField:       "content_vector",
    DistanceThreshold: &threshold,
    Embedding:         emb,
})
```

查询格式：`@content_vector:[VECTOR_RANGE $distance_threshold $vector]=>{$yield_distance_as: __vector_distance}`

## 使用过滤器

使用过滤器缩小搜索结果范围：

```go
docs, _ := retriever.Retrieve(ctx, "search query", 
    redisRetriever.WithFilterQuery("@category:{technology}"))
```

## 自定义返回字段

指定要返回的字段：

```go
retriever, _ := redisRetriever.NewRetriever(ctx, &redisRetriever.RetrieverConfig{
    Client:       client,
    Index:        "my_index",
    VectorField:  "content_vector",
    ReturnFields: []string{"content", "content_vector", "title", "category"},
    Embedding:    emb,
})
```

## 自定义文档解析器

自定义搜索结果的解析方式：

```go
retriever, _ := redisRetriever.NewRetriever(ctx, &redisRetriever.RetrieverConfig{
    Client:      client,
    Index:       "my_index",
    VectorField: "content_vector",
    DocumentConverter: func(ctx context.Context, doc redis.Document) (*schema.Document, error) {
        return &schema.Document{
            ID:      doc.ID,
            Content: doc.Fields["content"],
            MetaData: map[string]any{
                "title":    doc.Fields["title"],
                "category": doc.Fields["category"],
            },
        }, nil
    },
    Embedding: emb,
})
```

## 更多详情

- [Eino 文档](https://www.cloudwego.io/zh/docs/eino/)
- [Redis Go 客户端文档](https://github.com/redis/go-redis)
- [Redis 向量搜索](https://redis.io/docs/latest/develop/interact/search-and-query/advanced-concepts/vectors/)
- [KNN 向量搜索](https://redis.io/docs/latest/develop/interact/search-and-query/advanced-concepts/vectors/#knn-vector-search)
- [向量范围查询](https://redis.io/docs/latest/develop/interact/search-and-query/advanced-concepts/vectors/#vector-range-queries)
## 示例

查看以下示例了解更多用法：

- [自定义检索器](./examples/customized_retriever/)
- [默认检索器](./examples/default_retriever/)


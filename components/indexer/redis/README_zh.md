# Redis Indexer

[English](README.md) | 中文

[Eino](https://github.com/cloudwego/eino) 的 Redis 索引器实现，实现了 `Indexer` 接口。该组件使用 Redis Hashes 存储带有向量嵌入的文档，支持向量相似度搜索功能。

## 功能特性

- 实现了 `github.com/cloudwego/eino/components/indexer.Indexer`
- 易于集成 Eino 索引系统
- 使用 Redis Hashes 进行数据存储
- 支持配置 embedding 的文档向量化
- 批量 embedding 操作以提升性能
- 支持自定义字段映射
- 灵活的文档到哈希的转换

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/indexer/redis@latest
```

## 快速开始

以下是一个使用索引器的简单示例：

```go
import (
	"context"
	"fmt"
	"os"

	"github.com/cloudwego/eino/schema"
	"github.com/redis/go-redis/v9"

	"github.com/cloudwego/eino-ext/components/embedding/ark"
	"github.com/cloudwego/eino-ext/components/indexer/redis"
)

func main() {
	ctx := context.Background()

	// 1. 创建 Redis 客户端
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})

	// 2. 创建 embedding 组件
	emb, _ := ark.NewEmbedder(ctx, &ark.EmbeddingConfig{
		APIKey: os.Getenv("ARK_API_KEY"),
		Region: os.Getenv("ARK_REGION"),
		Model:  os.Getenv("ARK_MODEL"),
	})

	// 3. 准备文档
	docs := []*schema.Document{
		{
			ID:      "1",
			Content: "Eiffel Tower: Located in Paris, France.",
			MetaData: map[string]any{
				"location": "France",
			},
		},
		{
			ID:      "2",
			Content: "The Great Wall: Located in China.",
			MetaData: map[string]any{
				"location": "China",
			},
		},
	}

	// 4. 创建 Redis 索引器
	indexer, _ := redis.NewIndexer(ctx, &redis.IndexerConfig{
		Client:    client,
		KeyPrefix: "doc:",
		BatchSize: 10,
		Embedding: emb,
	})

	// 5. 索引文档
	ids, err := indexer.Store(ctx, docs)
	if err != nil {
		fmt.Printf("index error: %v\n", err)
		return
	}
	fmt.Println("indexed ids:", ids)
}
```

## 配置

可以使用 `IndexerConfig` 结构体配置索引器：

```go
type IndexerConfig struct {
    // 必填: Redis 客户端实例
    Client *redis.Client

    // 选填：每个 key 的前缀，hset key 将是 KeyPrefix+Hashes.Key
    // 如果不设置，请确保 DocumentToHashes 返回的每个 key 都包含相同的前缀
    KeyPrefix string

    // 选填：自定义 redis hash 的 key、field 和 value
    // 默认值：defaultDocumentToFields（使用 doc.ID 作为 key，content 作为 field，并向量化 content）
    DocumentToHashes func(ctx context.Context, doc *schema.Document) (*Hashes, error)

    // 选填：批量 embedding 的最大文本数量（默认：10）
    BatchSize int

    // 必填：用于文本向量化的 embedding 方法
    Embedding embedding.Embedder
}

// Hashes 定义了 Redis hash 存储的结构
type Hashes struct {
    Key         string                   // Redis hash key
    Field2Value map[string]FieldValue    // 字段到值的映射
}

// FieldValue 定义了字段应如何存储和向量化
type FieldValue struct {
    Value     any                             // 要存储的原始值
    EmbedKey  string                          // 如果设置，Value 将被向量化并存储在此 key 下
    Stringify func(val any) (string, error)  // 选填：用于 embedding 的自定义字符串转换
}
```

## 自定义文档映射

您可以自定义文档如何映射到 Redis hashes：

```go
indexer, _ := redis.NewIndexer(ctx, &redis.IndexerConfig{
    Client:    client,
    KeyPrefix: "doc:",
    DocumentToHashes: func(ctx context.Context, doc *schema.Document) (*redis.Hashes, error) {
        return &redis.Hashes{
            Key: doc.ID,
            Field2Value: map[string]redis.FieldValue{
                "content": {
                    Value:    doc.Content,
                    EmbedKey: "content_vector",  // 向量化 content 并保存到 "content_vector"
                },
                "title": {
                    Value:    doc.MetaData["title"],
                    EmbedKey: "title_vector",    // 向量化 title 并保存到 "title_vector"
                },
                "category": {
                    Value: doc.MetaData["category"],  // 存储 category 但不向量化
                },
            },
        }, nil
    },
    BatchSize: 10,
    Embedding: emb,
})
```

## 工作原理

1. **文档处理**：索引器使用 `DocumentToHashes` 将文档转换为 Redis hash 结构
2. **批量 Embedding**：标记为需要向量化的文本（设置了 `EmbedKey`）会被批量处理并一起 embed
3. **Pipeline 执行**：使用 Redis pipeline 进行高效的批量插入
4. **存储格式**：每个文档作为一个 Redis hash 存储，模式为 `KeyPrefix+key`

## 默认行为

默认情况下（未提供 `DocumentToHashes` 时）：
- 使用 `doc.ID` 作为 hash key
- 将 `doc.Content` 存储在 "content" 字段
- 向量化 content 并存储在 "content_vector" 字段
- 原样包含所有 `doc.MetaData` 字段

## 更多详情

- [Eino 文档](https://www.cloudwego.io/zh/docs/eino/)
- [Redis Go 客户端文档](https://github.com/redis/go-redis)
- [Redis 向量搜索](https://redis.io/docs/latest/develop/interact/search-and-query/advanced-concepts/vectors/)
## 示例

查看以下示例了解更多用法：

- [自定义索引器](./examples/customized_indexer/)
- [默认索引器](./examples/default_indexer/)


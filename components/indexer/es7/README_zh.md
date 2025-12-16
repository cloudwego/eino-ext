# ES7 Indexer

[English](README.md)

[Eino](https://github.com/cloudwego/eino) 的 Elasticsearch 7.x 索引器实现，实现了 `Indexer` 接口。该组件可以与 Eino 的文档索引系统无缝集成，提供强大的向量存储和检索能力。

## 功能特性

- 实现了 `github.com/cloudwego/eino/components/indexer.Indexer`
- 易于集成 Eino 索引系统
- 可配置 Elasticsearch 参数
- 支持向量相似度搜索
- 支持批量索引操作
- 支持自定义字段映射
- 灵活的文档向量化支持

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/indexer/es7@latest
```

## 快速开始

以下是一个使用索引器的简单示例：

```go
import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
	"github.com/elastic/go-elasticsearch/v7"

	"github.com/cloudwego/eino-ext/components/indexer/es7"
)

const (
	indexName          = "eino_example"
	fieldContent       = "content"
	fieldContentVector = "content_vector"
	fieldExtraLocation = "location"
	docExtraLocation   = "location"
)

func main() {
	ctx := context.Background()

	// ES 支持多种连接方式
	username := os.Getenv("ES_USERNAME")
	password := os.Getenv("ES_PASSWORD")

	client, _ := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{"http://localhost:9200"},
		Username:  username,
		Password:  password,
	})

	// 创建 embedding 组件
	emb := createYourEmbedding()

	// 加载文档
	docs := loadYourDocs()

	// 创建 ES 索引器组件
	indexer, _ := es7.NewIndexer(ctx, &es7.IndexerConfig{
		Client:    client,
		Index:     indexName,
		BatchSize: 10,
		DocumentToFields: func(ctx context.Context, doc *schema.Document) (field2Value map[string]es7.FieldValue, err error) {
			return map[string]es7.FieldValue{
				fieldContent: {
					Value:    doc.Content,
					EmbedKey: fieldContentVector, // 对文档内容进行向量化并保存到 "content_vector" 字段
				},
				fieldExtraLocation: {
					Value: doc.MetaData[docExtraLocation],
				},
			}, nil
		},
		Embedding: emb, // 替换为真实的 embedding 组件
	})

	ids, _ := indexer.Store(ctx, docs)

	fmt.Println(ids)
    // 与 Eino 系统一起使用
    // ... 配置并与 Eino 一起使用
}
```

## 配置

可以使用 `IndexerConfig` 结构体配置索引器：

```go
type IndexerConfig struct {
    Client *elasticsearch.Client // 必填：Elasticsearch 客户端实例
    Index  string                // 必填：存储文档的索引名称
    BatchSize int                // 选填：最大文本嵌入批次大小（默认：5）

    // 必填：将 Document 字段映射到 Elasticsearch 字段的函数
    DocumentToFields func(ctx context.Context, doc *schema.Document) (map[string]FieldValue, error)

    // 选填：仅在需要向量化时必填
    Embedding embedding.Embedder
}

// FieldValue 定义了字段应如何存储和向量化
type FieldValue struct {
    Value     any    // 要存储的原始值
    EmbedKey  string // 如果设置，Value 将被向量化并保存
    Stringify func(val any) (string, error) // 选填：自定义字符串转换
}
```

## 更多详情

- [Eino 文档](https://www.cloudwego.io/zh/docs/eino/)
- [Elasticsearch Go 客户端文档](https://github.com/elastic/go-elasticsearch)

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

	"github.com/cloudwego/eino-ext/components/embedding/ark"
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

	username := os.Getenv("ES_USERNAME")
	password := os.Getenv("ES_PASSWORD")

	// 1. 创建 ES 客户端
	client, _ := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{"http://localhost:9200"},
		Username:  username,
		Password:  password,
	})

	// 2. 定义 Index Spec（选填：如果索引不存在，将自动创建）
	indexSpec := &es7.IndexSpec{
		Settings: map[string]any{
			"number_of_shards":   1,
			"number_of_replicas": 0,
		},
		Mappings: map[string]any{
			"properties": map[string]any{
				fieldContentVector: map[string]any{
					"type": "dense_vector",
					"dims": 1536,
				},
			},
		},
	}

	// 3. 使用 Volcengine ARK 创建 embedding 组件
	emb, _ := ark.NewEmbedder(ctx, &ark.EmbeddingConfig{
		APIKey: os.Getenv("ARK_API_KEY"),
		Region: os.Getenv("ARK_REGION"),
		Model:  os.Getenv("ARK_MODEL"),
	})

	// 4. 准备文档
	// 文档通常包含 ID 和 Content。也可以添加额外的元数据用于过滤等用途。
	docs := []*schema.Document{
		{
			ID:      "1",
			Content: "Eiffel Tower: Located in Paris, France.",
			MetaData: map[string]any{
				docExtraLocation: "France",
			},
		},
		{
			ID:      "2",
			Content: "The Great Wall: Located in China.",
			MetaData: map[string]any{
				docExtraLocation: "China",
			},
		},
	}

	// 5. 创建 ES 索引器组件
	indexer, _ := es7.NewIndexer(ctx, &es7.IndexerConfig{
		Client:    client,
		Index:     indexName,
		IndexSpec: indexSpec, // 添加此项以启用自动索引创建
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
		Embedding: emb,
	})

	// 6. 索引文档
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
    Client *elasticsearch.Client // 必填: ES 客户端实例
    Index  string                // 必填: 存储文档的索引名称
    IndexSpec *IndexSpec         // 选填: 用于自动创建索引的设置和映射。
                                 // 如果提供，索引器将在初始化（NewIndexer）时检查索引是否存在。
                                 // 如果不存在，将使用提供的 Spec 创建索引；如果已存在，则不执行任何操作。
    BatchSize int                // 选填: 用于 embedding 的最大文本数量 (默认: 5)

    // 必填：将 Document 字段映射到 Elasticsearch 字段的函数
    DocumentToFields func(ctx context.Context, doc *schema.Document) (map[string]FieldValue, error)

    // 选填：仅在需要向量化时必填
    Embedding embedding.Embedder
}

// IndexSpec 定义了索引的设置和映射
type IndexSpec struct {
    Settings map[string]any `json:"settings,omitempty"`
    Mappings map[string]any `json:"mappings,omitempty"`
    Aliases  map[string]any `json:"aliases,omitempty"`
}

// FieldValue 定义了字段应如何存储和向量化
type FieldValue struct {
    Value     any    // 要存储的原始值
    EmbedKey  string // 如果设置，Value 将被向量化并保存
    Stringify func(val any) (string, error) // 选填：自定义字符串转换
}
```

## 完整示例

- [索引器示例](./examples/indexer)

## 更多详情

- [Eino 文档](https://www.cloudwego.io/zh/docs/eino/)
- [Elasticsearch Go 客户端文档](https://github.com/elastic/go-elasticsearch)

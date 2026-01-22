# OpenSearch 2 Indexer

[English](README.md) | 简体中文

[Eino](https://github.com/cloudwego/eino) 的 OpenSearch 2 索引器实现，实现了 `Indexer` 接口。这使得 OpenSearch 可以无缝集成到 Eino 的向量存储和检索系统中，增强语义搜索能力。

## 功能特性

- 实现 `github.com/cloudwego/eino/components/indexer.Indexer`
- 易于集成到 Eino 的索引系统
- 可配置的 OpenSearch 参数
- 支持向量相似度搜索
- 支持批量索引操作
- 支持自定义字段映射
- 灵活的文档向量化支持

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/indexer/opensearch2@latest
```

## 快速开始

以下是一个如何使用该索引器的简单示例，更多细节可参考 components/indexer/opensearch2/examples/indexer/main.go：

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
	"github.com/cloudwego/eino-ext/components/indexer/opensearch2"
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
	username := os.Getenv("OPENSEARCH_USERNAME")
	password := os.Getenv("OPENSEARCH_PASSWORD")

	// 1. 创建 OpenSearch 客户端
	client, err := opensearch.NewClient(opensearch.Config{
		Addresses: []string{"http://localhost:9200"},
		Username:  username,
		Password:  password,
	})
	if err != nil {
		log.Fatal(err)
	}

	// 2. 定义 Index Spec（选填：如果索引不存在，将自动创建）
	indexSpec := &opensearch2.IndexSpec{
		Settings: map[string]any{
			"number_of_shards": 1,
		},
		Mappings: map[string]any{
			"properties": map[string]any{
				fieldContentVector: map[string]any{
					"type":      "knn_vector",
					"dimension": 1536,
					"method": map[string]any{
						"name":       "hnsw",
						"engine":     "nmslib",
						"space_type": "l2",
					},
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

	// 4. 创建 opensearch indexer 组件
	indexer, _ := opensearch2.NewIndexer(ctx, &opensearch2.IndexerConfig{
		Client:    client,
		Index:     indexName,
		IndexSpec: indexSpec, // 添加此项以启用自动索引创建
		BatchSize: 10,
		DocumentToFields: func(ctx context.Context, doc *schema.Document) (map[string]opensearch2.FieldValue, error) {
			return map[string]opensearch2.FieldValue{
				fieldContent: {
					Value:    doc.Content,
					EmbedKey: fieldContentVector, // 向量化文档内容并保存到 "content_vector" 字段
				},
				fieldExtraLocation: {
					Value: doc.MetaData[docExtraLocation],
				},
			}, nil
		},
		Embedding: emb,
	})

	// 5. 准备文档
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

	// 6. 索引文档
	ids, err := indexer.Store(ctx, docs)
	if err != nil {
		fmt.Printf("index error: %v\n", err)
		return
	}
	fmt.Println("indexed ids:", ids)
}
```

## 配置说明

可以通过 `IndexerConfig` 结构体配置索引器：

```go
type IndexerConfig struct {
    Client *opensearch.Client // 必填：OpenSearch 客户端实例
    Index  string             // 必填：用于存储文档的索引名称
    IndexSpec *IndexSpec       // 选填：用于自动创建索引的设置和映射。
                               // 如果提供，索引器将在初始化（NewIndexer）时检查索引是否存在。
                               // 如果不存在，将使用提供的 Spec 创建索引；如果已存在，则不执行任何操作。
    BatchSize int             // 选填：最大文本嵌入批次大小（默认：5）

    // 必填：将 Document 字段映射到 OpenSearch 字段的函数
    DocumentToFields func(ctx context.Context, doc *schema.Document) (map[string]FieldValue, error)

    // 选填：仅当需要向量化时必填
    Embedding embedding.Embedder
}

// IndexSpec 定义了索引的设置和映射
type IndexSpec struct {
    Settings map[string]any `json:"settings,omitempty"`
    Mappings map[string]any `json:"mappings,omitempty"`
    Aliases  map[string]any `json:"aliases,omitempty"`
}

// FieldValue 定义字段应如何存储和向量化
type FieldValue struct {
    Value     any    // 存储的原始值
    EmbedKey  string // 如果设置，Value 将被向量化并保存及其向量值
    Stringify func(val any) (string, error) // 选填：自定义字符串转换函数
}
```

## 完整示例

- [索引器示例](./examples/indexer)

## 更多详情

- [Eino 文档](https://www.cloudwego.io/zh/docs/eino/)
- [OpenSearch Go 客户端文档](https://github.com/opensearch-project/opensearch-go)

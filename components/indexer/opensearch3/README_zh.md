# OpenSearch 3 Indexer

[English](README.md) | 简体中文

[Eino](https://github.com/cloudwego/eino) 的 OpenSearch 3 索引器实现，实现了 `Indexer` 接口。这使得 OpenSearch 可以无缝集成到 Eino 的向量存储和检索系统中，增强语义搜索能力。

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
go get github.com/cloudwego/eino-ext/components/indexer/opensearch3@latest
```

## 快速开始

以下是一个如何使用该索引器的简单示例，更多细节可参考 components/indexer/opensearch3/examples/indexer/main.go：

```go
package main

import (
	"context"
	"fmt"
	"log"
	
	"github.com/cloudwego/eino/schema"
	opensearch "github.com/opensearch-project/opensearch-go/v4"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"

	"github.com/cloudwego/eino-ext/components/indexer/opensearch3"
)

func main() {
	ctx := context.Background()

	client, err := opensearchapi.NewClient(opensearchapi.Config{
		Client: opensearch.Config{
			Addresses: []string{"http://localhost:9200"},
			// ... auth config
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	// 创建 embedding 组件
	emb := createYourEmbedding()

	// 创建 opensearch indexer 组件
	indexer, _ := opensearch3.NewIndexer(ctx, &opensearch3.IndexerConfig{
		Client:    client,
		Index:     "your_index_name",
		BatchSize: 10,
		DocumentToFields: func(ctx context.Context, doc *schema.Document) (map[string]opensearch3.FieldValue, error) {
			return map[string]opensearch3.FieldValue{
				"content": {
					Value:    doc.Content,
					EmbedKey: "content_vector",
				},
			}, nil
		},
		Embedding: emb,
	})

	docs := []*schema.Document{
		{ID: "1", Content: "example content"},
	}

	ids, _ := indexer.Store(ctx, docs)
	fmt.Println(ids)
}
```

## 配置说明

可以通过 `IndexerConfig` 结构体配置索引器：

```go
type IndexerConfig struct {
    Client *opensearchapi.Client // 必填：OpenSearch 客户端实例
    Index  string             // 必填：用于存储文档的索引名称
    BatchSize int             // 选填：最大文本嵌入批次大小（默认：5）

    // 必填：将 Document 字段映射到 OpenSearch 字段的函数
    DocumentToFields func(ctx context.Context, doc *schema.Document) (map[string]FieldValue, error)

    // 选填：仅当需要向量化时必填
    Embedding embedding.Embedder
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

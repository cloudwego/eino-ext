# ES8 Indexer

[English](README.md)

为 [Eino](https://github.com/cloudwego/eino) 实现的 Elasticsearch 8.x 索引器，实现了 `Indexer` 接口。这使得可以与 Eino 的向量存储和检索系统无缝集成，从而增强语义搜索能力。

## 功能特性

- 实现 `github.com/cloudwego/eino/components/indexer.Indexer`
- 易于集成 Eino 的索引系统
- 可配置 Elasticsearch 参数
- 支持向量相似度搜索
- 批量索引操作
- 自定义字段映射支持
- 灵活的文档向量化

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/indexer/es8@latest
```

## 快速开始

这里是使用索引器的快速示例，更多细节请阅读 components/indexer/es8/examples/indexer/add_documents.go：

```go
import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
	"github.com/elastic/go-elasticsearch/v8"

	"github.com/cloudwego/eino-ext/components/embedding/ark"
	"github.com/cloudwego/eino-ext/components/indexer/es8"
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

	// 准备 CA 证书（ES8 默认启用 TLS，需要提供自定义证书的 CA）
	httpCACertPath := os.Getenv("ES_HTTP_CA_CERT_PATH")
	var cert []byte
	if httpCACertPath != "" {
		var err error
		cert, err = os.ReadFile(httpCACertPath)
		if err != nil {
			log.Fatalf("read file failed, err=%v", err)
		}
	}

	// 1. 创建 ES 客户端
	client, _ := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{"https://localhost:9200"},
		Username:  username,
		Password:  password,
		CACert:    cert,
	})

	// 2. 定义 Index Spec（选填：如果索引不存在，将自动创建）
	indexSpec := &es8.IndexSpec{
		Settings: map[string]any{
			"number_of_shards":   1,
			"number_of_replicas": 0,
		},
		Mappings: map[string]any{
			"properties": map[string]any{
				fieldContentVector: map[string]any{
					"type":            "dense_vector",
					"dims":            1536,
					"index":           true,
					"similarity":      "l2_norm",
				},
			},
		},
	}

	// 3. 创建 embedding 组件
	// 使用火山引擎 Ark，替换环境变量为真实配置
	emb, _ := ark.NewEmbedder(ctx, &ark.EmbeddingConfig{
		APIKey: os.Getenv("ARK_API_KEY"),
		Region: os.Getenv("ARK_REGION"),
		Model:  os.Getenv("ARK_MODEL"),
	})

	// 4. 准备文档
	// 文档通常包含 ID 和 Content。也可以添加额外的元数据用于过滤等用途.
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
	indexer, _ := es8.NewIndexer(ctx, &es8.IndexerConfig{
		Client:    client,
		Index:     indexName,
		IndexSpec: indexSpec, // 添加此项以启用自动索引创建
		BatchSize: 10,
		// DocumentToFields 指定如何将文档字段映射到 ES 字段
		DocumentToFields: func(ctx context.Context, doc *schema.Document) (field2Value map[string]es8.FieldValue, err error) {
			return map[string]es8.FieldValue{
				fieldContent: {
					Value:    doc.Content,
					EmbedKey: fieldContentVector, // 向量化文档内容并保存到 "content_vector" 字段
				},
				fieldExtraLocation: {
					// 额外的元数据字段
					Value: doc.MetaData[docExtraLocation],
				},
			}, nil
		},
		// 提供 embedding 组件用于向量化
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
    Client *elasticsearch.Client // 必填: Elasticsearch 客户端实例
    Index  string                // 必填: 存储文档的索引名称
    IndexSpec *IndexSpec         // 选填: 用于自动创建索引的设置和映射。
                                 // 如果提供，索引器将在初始化（NewIndexer）时检查索引是否存在。
                                 // 如果不存在，将使用提供的 Spec 创建索引；如果已存在，则不执行任何操作。
    BatchSize int                // 选填: 用于 embedding 的最大文本数量 (默认: 5)

    // 必填: 将 Document 字段映射到 Elasticsearch 字段的函数
    DocumentToFields func(ctx context.Context, doc *schema.Document) (map[string]FieldValue, error)

    // 选填: 仅在需要向量化时必填
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
    Stringify func(val any) (string, error) // 选填: 自定义字符串转换
}
```

## 完整示例

- [索引器示例](./examples/indexer)
- [稀疏向量索引器示例](./examples/indexer_with_sparse_vector)

## 更多详情

- [Eino 文档](https://www.cloudwego.io/zh/docs/eino/)
- [Elasticsearch Go Client 文档](https://github.com/elastic/go-elasticsearch)

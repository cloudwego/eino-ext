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
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
	"github.com/elastic/go-elasticsearch/v8"

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

	// es 支持多种连接方式
	username := os.Getenv("ES_USERNAME")
	password := os.Getenv("ES_PASSWORD")
	httpCACertPath := os.Getenv("ES_HTTP_CA_CERT_PATH")

	cert, err := os.ReadFile(httpCACertPath)
	if err != nil {
		log.Fatalf("read file failed, err=%v", err)
	}

	client, _ := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{"https://localhost:9200"},
		Username:  username,
		Password:  password,
		CACert:    cert,
	})

	// 创建 embedding 组件
	// emb := createYourEmbedding()

	// 加载文档
	// docs := loadYourDocs()

	// 创建 es 索引器组件
	indexer, _ := es8.NewIndexer(ctx, &es8.IndexerConfig{
		Client:    client,
		Index:     indexName,
		BatchSize: 10,
		DocumentToFields: func(ctx context.Context, doc *schema.Document) (field2Value map[string]es8.FieldValue, err error) {
			return map[string]es8.FieldValue{
				fieldContent: {
					Value:    doc.Content,
					EmbedKey: fieldContentVector, // 对文档内容进行向量化并保存到 "content_vector" 字段
				},
				fieldExtraLocation: {
					Value: doc.MetaData[docExtraLocation],
				},
			}, nil
		},
		// Embedding: emb, // 替换为真实的 embedding 组件
	})

	// ids, _ := indexer.Store(ctx, docs)

	// fmt.Println(ids)
    // 与 Eino 系统一起使用
    // ... 在 Eino 中配置和使用
}
```

## 配置

可以使用 `IndexerConfig` 结构体配置索引器：

```go
type IndexerConfig struct {
    Client *elasticsearch.Client // 必填: Elasticsearch 客户端实例
    Index  string                // 必填: 存储文档的索引名称
    BatchSize int                // 选填: 用于 embedding 的最大文本数量 (默认: 5)

    // 必填: 将 Document 字段映射到 Elasticsearch 字段的函数
    DocumentToFields func(ctx context.Context, doc *schema.Document) (map[string]FieldValue, error)

    // 选填: 仅在需要向量化时必填
    Embedding embedding.Embedder
}

// FieldValue 定义了字段应如何存储和向量化
type FieldValue struct {
    Value     any    // 要存储的原始值
    EmbedKey  string // 如果设置，Value 将被向量化并保存
    Stringify func(val any) (string, error) // 选填: 自定义字符串转换
}
```

## 更多详情

- [Eino 文档](https://www.cloudwego.io/zh/docs/eino/)
- [Elasticsearch Go Client 文档](https://github.com/elastic/go-elasticsearch)

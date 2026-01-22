# ES8 Retriever

[English](README.md)

为 [Eino](https://github.com/cloudwego/eino) 实现的 Elasticsearch 8.x 检索器，实现了 `Retriever` 接口。这使得可以与 Eino 的向量检索系统无缝集成，从而增强语义搜索能力。

## 功能特性

- 实现 `github.com/cloudwego/eino/components/retriever.Retriever`
- 易于集成 Eino 的检索系统
- 可配置 Elasticsearch 参数
- 支持向量相似度搜索
- 多种搜索模式（包括近似搜索）
- 自定义结果解析支持
- 灵活的文档过滤

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/retriever/es8@latest
```

## 快速开始

这里是使用近似搜索模式的快速示例，更多细节请阅读 components/retriever/es8/examples/approximate/approximate.go：

```go
package main

import (
	"context"
	"encoding/json"
	"os"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

	"github.com/cloudwego/eino-ext/components/embedding/ark"
	"github.com/cloudwego/eino-ext/components/retriever/es8"
	"github.com/cloudwego/eino-ext/components/retriever/es8/search_mode"
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
	// es supports multiple ways to connect
	username := os.Getenv("ES_USERNAME")
	password := os.Getenv("ES_PASSWORD")

	// 1. 创建 ES 客户端
	httpCACertPath := os.Getenv("ES_HTTP_CA_CERT_PATH")
	if httpCACertPath != "" {
		cert, err := os.ReadFile(httpCACertPath)
		if err != nil {
			log.Fatalf("read file failed, err=%v", err)
		}
	}

	client, _ := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{"https://localhost:9200"},
		Username:  username,
		Password:  password,
		CACert:    cert,
	})

	// 2. 创建 embedding 组件
	// 使用火山引擎 Ark，替换环境变量为真实配置
	emb, _ := ark.NewEmbedder(ctx, &ark.EmbeddingConfig{
		APIKey: os.Getenv("ARK_API_KEY"),
		Region: os.Getenv("ARK_REGION"),
		Model:  os.Getenv("ARK_MODEL"),
	})

	// 3. 创建 ES 检索器组件
	retriever, _ := es8.NewRetriever(ctx, &es8.RetrieverConfig{
		Client: client,
		Index:  indexName,
		TopK:   5, // 检索前 5 个结果
		// 使用近似搜索模式 (向量搜索)
		SearchMode: search_mode.SearchModeApproximate(&search_mode.ApproximateConfig{
			QueryFieldName:  fieldContent,
			VectorFieldName: fieldContentVector,
			Hybrid:          true, // 启用混合搜索 (向量 + 关键词)
			// RRF 仅在特定许可证下可用
			// 参见: https://www.elastic.co/subscriptions
			RRF:             false,
			RRFRankConstant: nil,
			RRFWindowSize:   nil,
		}),
		// ResultParser 从 ES 结果中提取文档字段
		ResultParser: func(ctx context.Context, hit types.Hit) (doc *schema.Document, err error) {
			doc = &schema.Document{
				ID:       *hit.Id_,
				Content:  "",
				MetaData: map[string]any{},
			}

			var src map[string]any
			if err = json.Unmarshal(hit.Source_, &src); err != nil {
				return nil, err
			}

			for field, val := range src {
				switch field {
				case fieldContent:
					doc.Content = val.(string)
				case fieldContentVector:
					var v []float64
					for _, item := range val.([]interface{}) {
						v = append(v, item.(float64))
					}
					doc.WithDenseVector(v)
				case fieldExtraLocation:
					doc.MetaData[docExtraLocation] = val.(string)
				}
			}

			if hit.Score_ != nil {
				doc.WithScore(float64(*hit.Score_))
			}

			return doc, nil
		},
		// Embedding: emb, // 你的 embedding 组件
	})

	// 不带过滤器的搜索
	docs, _ := retriever.Retrieve(ctx, "tourist attraction")

	// 带过滤器的搜索
	caseInsensitive := true
	docs, _ = retriever.Retrieve(ctx, "tourist attraction",
		es8.WithFilters([]types.Query{{
			Term: map[string]types.TermQuery{
				fieldExtraLocation: {
					CaseInsensitive: &caseInsensitive,
					Value:           "China",
				},
			},
		}}),
	)
}
```

## 配置

可以使用 `RetrieverConfig` 结构体配置检索器：

```go
type RetrieverConfig struct {
    Client *elasticsearch.Client // 必填: Elasticsearch 客户端实例
    Index  string               // 必填: 检索文档的索引名称
    TopK   int                  // 必填: 返回的结果数量

    // 必填: 搜索模式配置
    SearchMode search_mode.SearchMode

    // 选填: 将 Elasticsearch hits 解析为 Document 的函数
    // 如果未提供，将使用默认解析器：
    // 1. 从 source 中提取 "content" 字段作为 Document.Content
    // 2. 将其他 source 字段作为 Document.MetaData
    ResultParser func(ctx context.Context, hit types.Hit) (*schema.Document, error)

    // 选填: 仅在需要查询向量化时必填
    Embedding embedding.Embedder
}
```

## 完整示例

- [近似搜索示例](./examples/approximate)
- [稠密向量相似度示例](./examples/dense_vector_similarity)
- [精确匹配示例](./examples/exact_match)
- [原始字符串请求示例](./examples/raw_string)
- [稀疏向量查询示例](./examples/sparse_vector_query)

## 更多详情

- [Eino 文档](https://www.cloudwego.io/zh/docs/eino/)
- [Elasticsearch Go Client 文档](https://github.com/elastic/go-elasticsearch)

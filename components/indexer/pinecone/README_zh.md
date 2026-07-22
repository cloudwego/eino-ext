# Pinecone 存储

[English](README.md) | [简体中文](README_zh.md)

基于 Pinecone 的向量存储实现，为 [Eino](https://github.com/cloudwego/eino) 提供了符合 `Indexer` 接口的存储方案。该组件可无缝集成到 Eino 的向量存储和检索系统中，增强语义搜索能力。

## 快速开始

### 安装

需要 go-pinecone v3.x 客户端：

```bash
go get github.com/pinecone-io/go-pinecone/v3@latest
go get github.com/cloudwego/eino-ext/components/indexer/pinecone@latest
```

### 创建 Pinecone 存储

```go
package main

import (
	"context"
	"log"
	"os"

	pc "github.com/pinecone-io/go-pinecone/v3/pinecone"
	"github.com/cloudwego/eino-ext/components/indexer/pinecone"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
)

func main() {
	// 从环境变量加载配置
	apiKey := os.Getenv("PINECONE_APIKEY")
	if apiKey == "" {
		log.Fatal("PINECONE_APIKEY 环境变量必填")
	}

	// 初始化 Pinecone 客户端
	client, err := pc.NewClient(pc.NewClientParams{
		ApiKey: apiKey,
	})
	if err != nil {
		log.Fatalf("创建 Pinecone 客户端失败: %v", err)
	}

	// 创建 Pinecone 存储配置
	config := pinecone.IndexerConfig{
		Client:    client,
		Dimension: 2560, // 按照你的 embedding 维度设置
		Embedding: &mockEmbedding{},
	}

	// 创建 Indexer
	ctx := context.Background()
	indexer, err := pinecone.NewIndexer(ctx, &config)
	if err != nil {
		log.Fatalf("创建 Pinecone Indexer 失败: %v", err)
	}
	log.Println("Indexer 创建成功")

	// 存储文档
	docs := []*schema.Document{
		{
			ID:      "pinecone-1",
			Content: "pinecone 是一个向量数据库",
			MetaData: map[string]any{
				"tag1": "pinecone",
				"tag2": "vector",
				"tag3": "database",
			},
		},
		{
			ID:      "pinecone-2",
			Content: "Pinecone 是为 AI 应用构建的高性能向量数据库。",
		},
	}

	ids, err := indexer.Store(ctx, docs)
	if err != nil {
		log.Fatalf("存储文档失败: %v", err)
		return
	}
	log.Printf("已存储文档 ids: %v", ids)
}

// mockEmbedding 是 embedding 实现的占位符
// 请替换为你自己的 embedding 模型

// type mockEmbedding struct{}
// func (m *mockEmbedding) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
// 	// 实现 embedding 逻辑
// }
```

## 配置说明

`IndexerConfig` 支持如下配置项：

| 字段               | 类型                        | 说明                                   | 默认值         |
|--------------------|-----------------------------|----------------------------------------|----------------|
| Client             | *pinecone.Client            | Pinecone 客户端实例（必填）            | -              |
| IndexName          | string                      | Pinecone 索引名称                      | "eino-index"   |
| Cloud              | pinecone.Cloud              | 云服务商（如 "aws"）                   | "aws"          |
| Region             | string                      | 区域（如 "us-east-1"）                 | "us-east-1"    |
| Metric             | pinecone.IndexMetric        | 距离度量："cosine"、"euclidean"、"dotproduct" | "cosine" |
| Dimension          | int32                       | 向量维度                               | 2560           |
| VectorType         | string                      | 向量类型（如 "float32"）               | "float32"      |
| Namespace          | string                      | Pinecone 命名空间                      | (默认)         |
| Field              | string                      | 存储内容文本的字段                     | (默认)         |
| Tags               | *pinecone.IndexTags         | 元数据标签                             | (可选)         |
| DeletionProtection | pinecone.DeletionProtection | 删除保护                               | (可选)         |
| DocumentConverter  | func                       | 自定义文档转换器                       | (可选)         |
| BatchSize          | int                         | 批量 upsert 的大小                     | 100            |
| MaxConcurrency     | int                         | 并发 upsert 的最大协程数               | 10             |
| Embedding          | embedding.Embedder          | embedding 模型实例                     | （必填）       |

## 许可证

Apache 2.0。详见 [LICENSE](../../LICENSE)。

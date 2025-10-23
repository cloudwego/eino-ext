# Milvus 检索器

[English](README.md) | 简体中文

## 快速开始

### 安装

```bash
go get github.com/milvus-io/milvus/client/v2
go get github.com/cloudwego/eino-ext/components/retriever/milvus
```

### 示例

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/cloudwego/eino-ext/components/retriever/milvus"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

func main() {
	// 创建 Milvus 客户端
	client, err := milvusclient.New(context.Background(), &milvusclient.ClientConfig{
		Address: "localhost:19530",
	})
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close(context.Background())

	// 创建嵌入模型（示例使用假设的嵌入服务）
	embeddingModel := embedding.NewOpenAIEmbedding(&embedding.OpenAIConfig{
		APIKey: "your-api-key",
		Model:  "text-embedding-ada-002",
	})

	// 创建 Milvus 检索器
	retriever, err := milvus.NewRetriever(&milvus.RetrieverConfig{
		Client:     client,
		Collection: "my_documents",
		TopK:       10,
		Embedding:  embeddingModel,
	})
	if err != nil {
		log.Fatal(err)
	}

	// 执行语义搜索
	docs, err := retriever.Retrieve(context.Background(), "什么是机器学习？")
	if err != nil {
		log.Fatal(err)
	}

	// 处理结果
	for i, doc := range docs {
		fmt.Printf("文档 %d: %s\n", i+1, doc.PageContent)
		fmt.Printf("相似度分数: %v\n", doc.MetaData["score"])
		fmt.Println("---")
	}
}
```

## 配置说明

### RetrieverConfig

| 参数 | 类型 | 必填/可选 | 默认值 | 描述 |
|------|------|-----------|--------|------|
| `Client` | `*milvusclient.Client` | **必填** | - | 用于数据库操作的 Milvus 客户端实例 |
| `Collection` | `string` | 可选 | `"eino_collection"` | 要搜索的 Milvus 集合名称 |
| `TopK` | `int` | 可选 | `5` | 要检索的文档最大数量 |
| `Embedding` | `embedding.Embedder` | 可选 | `nil` | 用于将文本查询转换为向量的嵌入器 |
| `DocumentConverter` | `DocumentConverter` | 可选 | 默认转换器 | 将 Milvus 搜索结果转换为 schema.Document 对象 |
| `VectorConverter` | `VectorConverter` | 可选 | 默认转换器 | 将 float64 向量转换为 Milvus entity.Vector 格式 |

### 搜索选项

检索器支持多种搜索选项，可以传递给 `Retrieve` 方法：

#### WithLimit

```go
// 为特定搜索覆盖 TopK 值
docs, err := retriever.Retrieve(ctx, "查询", milvus.WithLimit(20))
```

#### WithHybridSearchOption

```go
// 使用混合搜索处理更复杂的场景
hybridSearch := milvus.NewHybridSearchOption("vector_field", 10).
	WithFilter("category == 'technology'").
	WithOffset(5)

docs, err := retriever.Retrieve(ctx, "查询", milvus.WithHybridSearchOption(hybridSearch))
```

### 混合搜索配置

`HybridSearch` 类型提供高级搜索功能：

| 方法 | 描述 | 必填/可选 |
|------|------|----------|
| `WithANNSField(field)` | 设置向量字段名称 | 可选 |
| `WithFilter(expr)` | 添加布尔过滤表达式 | 可选 |
| `WithGroupByField(field)` | 按字段分组结果 | 可选 |
| `WithGroupSize(size)` | 设置分组大小 | 可选 |
| `WithStrictGroupSize(strict)` | 强制严格分组大小 | 可选 |
| `WithSearchParam(key, value)` | 添加搜索参数 | 可选 |
| `WithAnnParam(param)` | 设置 ANN 参数 | 可选 |
| `WithOffset(offset)` | 跳过结果 | 可选 |
| `WithIgnoreGrowing(ignore)` | 忽略增长段 | 可选 |
| `WithTemplateParam(key, val)` | 添加模板参数 | 可选 |

## 向量维度计算

使用 Milvus 时，需要确保向量维度与集合模式匹配。维度取决于您的嵌入模型。

有关向量维度和集合设置的更多信息，请参考 [Milvus 官方文档](https://milvus.io/docs)。
# Dify 检索器

[English](README.md) | 简体中文

这是一个为 [Eino](https://github.com/cloudwego/eino) 实现的 Dify 检索器，实现了 `Retriever` 接口。它能够与 Eino 的检索系统无缝集成，从 Dify 知识库中检索相关文档。

## 特性

- 实现了 `github.com/cloudwego/eino/components/retriever.Retriever` 接口
- 易于与 Eino 的检索系统集成
- 支持可配置的检索参数
- 支持重排序功能

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/retriever/dify
```

## 快速开始

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/retriever/dify"
)



func main() {
	APIKey := os.Getenv("DIFY_DATASET_API_KEY")
	Endpoint := os.Getenv("DIFY_ENDPOINT")
	DatasetID := os.Getenv("DIFY_DATASET_ID")
	// 创建基本的 Dify Retriever
	ctx := context.Background()

	// 创建基本的 Dify Retriever
	ret, err := dify.NewRetriever(ctx, &dify.RetrieverConfig{
		APIKey:    APIKey,
		Endpoint:  Endpoint,
		DatasetID: DatasetID,
	})
	if err != nil {
		log.Fatalf("Failed to create retriever: %v", err)
	}

	// 执行检索
	docs, err := ret.Retrieve(ctx, "test query")
	if err != nil {
		log.Fatalf("Failed to retrieve: %v", err)
	}

	// 处理检索结果
	for _, doc := range docs {
		fmt.Printf("doc id: %s\n", doc.ID)
		fmt.Printf("doc content: %s\n", doc.Content)
		fmt.Printf("score: %v\n\n", doc.Score())
	}
}
```

## 配置

检索器可以使用 `RetrieverConfig` 结构体进行配置：

```go
type RetrieverConfig struct {
    APIKey string   // APIKey 是 Dify API 的认证密钥
    Endpoint string // Endpoint 是 Dify API 的服务地址, 默认为: https://api.dify.ai/v1
    DatasetID string    // DatasetID 是知识库的唯一标识
    RetrievalModel *RetrievalModel  // RetrievalModel 检索参数 选填，如不填，按照默认方式召回
    Timeout time.Duration   // Timeout 定义了 HTTP 连接超时时间
}

type RetrievalModel struct {
    SearchMethod          SearchMethod   // 搜索方法
    RerankingEnable      *bool           // 启用重排序
    RerankingMode        *string         // 重排序模式
    RerankingModel       *RerankingModel // 重排序模型设置
    Weights              *float64        // 搜索权重
    TopK                 *int            // 要检索的文档数量
    ScoreThresholdEnabled *bool          // 启用分数阈值
    ScoreThreshold       *float64        // 最小分数阈值
}
```

## 文档元数据

检索器会为检索到的文档添加以下元数据：

- `orig_doc_id`：Dify 中的原始文档 ID
- `orig_doc_name`：Dify 中的原始文档名称
- `keywords`：从文档中提取的关键词

你可以使用以下辅助函数访问这些元数据：

```go
docID := dify.GetOrgDocID(doc)
docName := dify.GetOrgDocName(doc)
keywords := dify.GetKeywords(doc)
```

## 更多详情

- [Dify 文档](https://github.com/langgenius/dify)
- [Eino 文档](https://github.com/cloudwego/eino)
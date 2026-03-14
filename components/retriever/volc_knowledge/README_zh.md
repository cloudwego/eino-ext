# Eino 火山引擎知识库检索器

[English](README.md) | 简体中文

## 简介

这是为 [Eino](https://github.com/cloudwego/eino) 实现的火山引擎知识库检索器组件，实现了 `Retriever` 接口，与火山引擎知识库服务集成，根据查询文本检索相关文档。

## 特性

- 实现 `github.com/cloudwego/eino/components/retriever.Retriever` 接口
- 与火山引擎知识库服务集成
- 支持密集检索并可配置权重
- 查询预处理（改写、指令生成）
- 后处理选项（重排序、块扩散、分组）
- 文档过滤功能
- 元数据提取（文档 ID、文档名称、块 ID、附件、表格）
- Token 使用量跟踪
- 易于集成到 Eino 工作流

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/retriever/volc_knowledge
```

## 快速开始

```go
package main

import (
	"context"
	"log"

	knowledge "github.com/cloudwego/eino-ext/components/retriever/volc_knowledge"
)

func main() {
	ctx := context.Background()

	retriever, err := knowledge.NewRetriever(ctx, &knowledge.Config{
		AK:         "your-access-key",
		SK:         "your-secret-key",
		AccountID:  "your-account-id",
		Name:       "your-knowledge-base-name",
		Project:    "default",
		Limit:      10,
	})
	if err != nil {
		log.Fatalf("knowledge.NewRetriever failed, err=%v", err)
	}

	docs, err := retriever.Retrieve(ctx, "什么是机器学习？")
	if err != nil {
		log.Fatalf("retriever.Retrieve failed, err=%v", err)
	}

	for i, doc := range docs {
		log.Printf("文档 %d:", i+1)
		log.Printf("  ID: %s", doc.ID)
		log.Printf("  内容: %s", doc.Content)
		log.Printf("  文档 ID: %s", knowledge.GetDocID(doc))
		log.Printf("  文档名称: %s", knowledge.GetDocName(doc))
	}
}
```

## 配置说明

检索器可以通过 `Config` 结构体进行配置：

```go
type Config struct {
    // 认证信息（必填）
    AK        string // 访问密钥
    SK        string // 私密密钥
    AccountID string // 火山引擎账号 ID
    
    // 连接设置（选填）
    Timeout time.Duration // 请求超时（默认：无超时）
    BaseURL string        // 基础 URL（默认："api-knowledgebase.mlp.cn-beijing.volces.com"）
    
    // 知识库标识（必填：Name+Project 或 ResourceID 二选一）
    Name       string // 知识库名称
    Project    string // 项目标识（默认："default"）
    ResourceID string // 资源标识（替代 Name+Project）
    
    // 检索设置（选填）
    Limit       int32          // 返回的最大文档数（1-200，默认：10）
    DocFilter   map[string]any // 文档过滤器
    DenseWeight float64        // 密集检索权重（默认：0.5）
    
    // 预处理（选填）
    NeedInstruction  bool              // 包含指令
    Rewrite          bool              // 启用查询改写
    ReturnTokenUsage bool              // 返回 token 使用信息
    Messages         []*schema.Message // 用于改写的对话历史
    
    // 后处理（选填）
    RerankSwitch        bool   // 启用重排序
    RetrieveCount       int32  // 用于重排序的文档数（≥Limit，默认：25）
    ChunkDiffusionCount int32  // 块扩散数量
    ChunkGroup          bool   // 分组块
    RerankModel         string // 重排序模型名称
    RerankOnlyChunk     bool   // 仅使用块内容重排序
    GetAttachmentLink   bool   // 包含附件链接
}
```

## 高级用法

### 使用重排序

```go
retriever, err := knowledge.NewRetriever(ctx, &knowledge.Config{
    AK:            "your-access-key",
    SK:            "your-secret-key",
    AccountID:     "your-account-id",
    Name:          "your-knowledge-base-name",
    Limit:         10,
    RerankSwitch:  true,
    RetrieveCount: 50,
    RerankModel:   "bge-reranker-v2-m3",
})
```

### 使用查询改写

```go
retriever, err := knowledge.NewRetriever(ctx, &knowledge.Config{
    AK:        "your-access-key",
    SK:        "your-secret-key",
    AccountID: "your-account-id",
    Name:      "your-knowledge-base-name",
    Rewrite:   true,
    Messages: []*schema.Message{
        {Role: schema.User, Content: "告诉我关于人工智能的信息"},
        {Role: schema.Assistant, Content: "人工智能是..."},
        {Role: schema.User, Content: "它如何工作？"},
    },
})
```

### 使用文档过滤

```go
retriever, err := knowledge.NewRetriever(ctx, &knowledge.Config{
    AK:        "your-access-key",
    SK:        "your-secret-key",
    AccountID: "your-account-id",
    Name:      "your-knowledge-base-name",
    DocFilter: map[string]any{
        "doc_type": "pdf",
        "year":     2024,
    },
})
```

## 元数据辅助函数

该包提供了辅助函数来提取元数据：

```go
docID := knowledge.GetDocID(doc)           // 获取文档 ID
docName := knowledge.GetDocName(doc)       // 获取文档名称
chunkID := knowledge.GetChunkID(doc)       // 获取块 ID
attachments := knowledge.GetAttachments(doc) // 获取附件
tables := knowledge.GetTableChunks(doc)    // 获取表格块
```

## 在链中使用

```go
import (
    "github.com/cloudwego/eino/compose"
    knowledge "github.com/cloudwego/eino-ext/components/retriever/volc_knowledge"
)

retriever, _ := knowledge.NewRetriever(ctx, &knowledge.Config{
    AK:        "your-access-key",
    SK:        "your-secret-key",
    AccountID: "your-account-id",
    Name:      "your-knowledge-base-name",
})

chain := compose.NewChain[string, []*schema.Document]()
chain.AppendRetriever(retriever)

run, _ := chain.Compile(ctx)
docs, _ := run.Invoke(ctx, "查询文本")
```

## 更多详情

- [火山引擎知识库 API 文档](https://www.volcengine.com/docs/84313/1350012)
- [Eino 文档](https://www.cloudwego.io/zh/docs/eino/)

## 许可证

本项目采用 Apache License 2.0 许可证 - 详见 LICENSE 文件。

# Milvus Indexer

Chinese | [English](README.md)

## 组件简介

Milvus Indexer 是基于 CloudWeGo Eino 框架的 Milvus 向量数据库索引组件，支持将文本文档嵌入为向量并存储到
Milvus，便于后续向量检索、语义搜索等场景。

## 版本要求

- Milvus 客户端库：github.com/milvus-io/milvus/client/v2 v2.6.x

## 安装

在 go.mod 中添加依赖：

```shell
go get -u github.com/milvus-io/milvus/client/v2
go get -u github.com/cloudwego/eino-ext/components/indexer/milvus
```

## 快速开始

参考 `examples/main.go`，创建 Milvus 客户端和 Indexer：

```go
package main

import (
	"context"
	
	"github.com/cloudwego/eino-ext/components/indexer/milvus"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

func main() {
	ctx := context.Background()
	client, _ := milvusclient.New(ctx, &milvusclient.ClientConfig{
		// your Milvus server config
	})
	defer client.Close()
	indexer, _ := milvus.NewIndexer(ctx, &milvus.IndexerConfig{
		Client:   client,
		Dim:      yourVectorDim,
		Embedder: yourEmbedder,
	})
	// 存储文档
	ids, err := indexer.Store(ctx, docs)
}
```

## 配置说明

`IndexerConfig` 主要参数：

- `Client`：Milvus 客户端实例，必填
- `Collection`：集合名，默认自动创建
- `Partition`：分区名，可选
- `Schema`：集合 Schema，默认为动态 Schema
- `Dim`：向量维度，必填
- `Embedder`：嵌入器，实现 embedding.Embedder 接口
- `DocumentConverter`：文档转换器，默认为内置实现
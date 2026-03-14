# Qdrant Indexer

一个为 [Eino](https://github.com/cloudwego/eino) 实现的 [Qdrant](https://qdrant.tech/) indexer 组件。

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/indexer/qdrant@latest
```

## 快速开始

```go
import (
  "context"
  "github.com/cloudwego/eino/schema"
  qdrant "github.com/qdrant/go-client/qdrant"
  "github.com/cloudwego/eino-ext/components/indexer/qdrant"
)

func main() {
  ctx := context.Background()

  // 创建 Qdrant 客户端
  client, _ := qdrant.NewClient(&qdrant.Config{
    Host: "localhost",
    Port: 6333,
  })

  // 创建 indexer
  indexer, _ := qdrant.NewIndexer(ctx, &qdrant.Config{
    Client:     client,
    Collection: "my_collection",
    VectorDim:  384,
    Distance:   qdrant.Distance_Cosine,
    Embedding:  yourEmbedding, // 你的 embedding 组件
  })

  // 存储文档
  docs := []*schema.Document{
    {ID: "1", Content: "Hello world", MetaData: map[string]interface{}{"type": "text"}},
  }
  ids, _ := indexer.Store(ctx, docs)
}
```

## 配置

```go
type Config struct {
    Client     *qdrant.Client        // 必需：Qdrant 客户端
    Collection string                // 必需：集合名称
    VectorDim  int                   // 必需：向量维度
    Distance   qdrant.Distance       // 必需：距离度量
    BatchSize  int                   // 可选：批处理大小（默认：10）
    Embedding  embedding.Embedder    // 必需：Embedding 组件
}
```

**距离度量**：`Distance_Cosine`、`Distance_Dot`、`Distance_Euclid`、`Distance_Manhattan`

## 示例

查看 `examples/default_indexer.go` 以获取完整的工作示例。

## 文档

- [Eino](https://github.com/cloudwego/eino)
- [Qdrant Go Client](https://github.com/qdrant/go-client)
- [Qdrant 文档](https://qdrant.tech/documentation/)

# Qdrant Retriever

一个为 [Eino](https://github.com/cloudwego/eino) 实现的 [Qdrant](https://qdrant.tech/) retriever 组件，提供向量相似性搜索功能。

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/retriever/qdrant@latest
```

## 快速开始

```go
import (
 "context"
 "github.com/cloudwego/eino/components/embedding"
 qdrant "github.com/qdrant/go-client/qdrant"
 "github.com/cloudwego/eino-ext/components/retriever/qdrant"
)

func main() {
 ctx := context.Background()

 // 创建 Qdrant 客户端
 client, _ := qdrant.NewClient(&qdrant.Config{
  Host: "localhost",
  Port: 6334,
 })

 // 创建 retriever
 retriever, _ := qdrant.NewRetriever(ctx, &qdrant.Config{
  Client:     client,
  Collection: "my_collection",
  Embedding:  &myEmbedding{},
  TopK:       5,
 })

 // 搜索
 docs, _ := retriever.Retrieve(ctx, "tourist attraction")
}
```

## 配置

```go
type Config struct {
    Client            *qdrant.Client      // Qdrant 客户端
    Collection        string              // 集合名称
    Embedding         embedding.Embedder  // 查询嵌入组件
    ScoreThreshold    *float64            // 可选的分数阈值
    TopK              int                 // 结果数量
    ReturnFields      []string            // 限制返回的 payload 字段（默认：["metadata", "content"]）
    DocumentConverter  func(ctx context.Context, point *qdrant.ScoredPoint) (*schema.Document, error) // 自定义文档转换器
}
```

## 高级用法

### 过滤

```go
import "github.com/cloudwego/eino-ext/components/retriever/qdrant/options"

docs, _ := retriever.Retrieve(ctx, "query",
    options.WithFilter(&qdrant.Filter{
        Must: []*qdrant.Condition{
            qdrant.NewMatch("metadata.location", "Paris")
        },
    }),
)
```

### 分数阈值

```go
scoreThreshold := 0.7
retriever, _ := qdrant.NewRetriever(ctx, &qdrant.Config{
    // ... 其他配置
    ScoreThreshold: &scoreThreshold,
})
```

### 返回字段

默认情况下，retriever 从 payload 中获取 `"metadata"` 和 `"content"`。你可以自定义返回哪些字段：

```go
retriever, _ := qdrant.NewRetriever(ctx, &qdrant.Config{
    // ... 其他配置
    ReturnFields: []string{"content", "category", "author"},
})
```

这会在协议层限制返回的 payload 字段 — Qdrant 只会传输请求的字段。

### 自定义文档转换器

如需完全控制 Qdrant points 到 Eino documents 的转换，可以提供自定义 `DocumentConverter`：

```go
retriever, _ := qdrant.NewRetriever(ctx, &qdrant.Config{
    // ... 其他配置
    ReturnFields: []string{"text"},
    DocumentConverter: func(ctx context.Context, point *qdrant.ScoredPoint) (*schema.Document, error) {
        return &schema.Document{
            ID:      point.Id.GetUuid(),
            Content: point.Payload["text"].GetStringValue(),
            MetaData: map[string]any{
                "score": point.Score,
            },
        }, nil
    },
})
```

## 文档映射

文档自动映射到 Qdrant points：

- `doc.ID` → Point ID
- `doc.Content` → Payload `"content"`
- `doc.MetaData` → Payload `"metadata"`
- Embeddings → Point vectors

## 参考资料

- [Eino 文档](https://www.cloudwego.io/zh/docs/eino/)
- [Qdrant 文档](https://qdrant.tech/documentation/)
## 示例

查看以下示例了解更多用法：

- [默认检索器](./examples/default_retriever/)


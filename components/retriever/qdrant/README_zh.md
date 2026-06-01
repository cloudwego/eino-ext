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
    Client         *qdrant.Client      // Qdrant 客户端
    Collection     string              // 集合名称
    Embedding      embedding.Embedder  // 查询嵌入组件
    ScoreThreshold *float64            // 可选的分数阈值
    TopK           int                 // 结果数量
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


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
  qindexer "github.com/cloudwego/eino-ext/components/indexer/qdrant"
)

func main() {
  ctx := context.Background()

  // 创建 Qdrant 客户端
  client, _ := qdrant.NewClient(&qdrant.Config{
    Host: "localhost",
    Port: 6333,
  })

  // 创建 indexer
  indexer, _ := qindexer.NewIndexer(ctx, &qindexer.Config{
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
    VectorDim  int                   // 未设置 Vectors 时必需：向量维度
    Distance   qdrant.Distance       // 未设置 Vectors 时必需：距离度量
    BatchSize  int                   // 可选：批处理大小（默认：10）
    Embedding  embedding.Embedder    // 未设置 Vectors 时必需：Embedding 组件
    Vectors    []VectorConfig        // 可选：多个命名向量空间
}

type VectorConfig struct {
    Name      string                 // 必需：向量空间在 Qdrant 中的名字
    Dimension int                    // 必需：该向量空间的维度
    Distance  qdrant.Distance        // 必需：该向量空间的距离度量
    Embedding embedding.Embedder     // 必需：该向量空间的 embedder
}
```

**距离度量**：`Distance_Cosine`、`Distance_Dot`、`Distance_Euclid`、`Distance_Manhattan`

## 命名向量

默认情况下集合只有一个匿名向量，更换 embedding 模型就必须全量重建索引。改用
`Vectors` 可以声明多个命名向量空间，每个 embedding 模型一个，各自带维度与
embedder：

```go
indexer, _ := qindexer.NewIndexer(ctx, &qindexer.Config{
    Client:     client,
    Collection: "docs",
    Vectors: []qindexer.VectorConfig{
        {Name: "bge-m3", Dimension: 1024, Distance: qdrant.Distance_Cosine, Embedding: bgeM3},
        {Name: "text-embedding-3-small", Dimension: 1536, Distance: qdrant.Distance_Cosine, Embedding: openAI},
    },
})
```

每次 `Store` 会把同一条文档在每个空间各算一次向量，并在**同一次 upsert** 中全部
写入，因此多个模型索引的是同一批 point，切回已经索引过的模型时不再需要重建索引。
查询时用 retriever 的 `VectorName` 指定与哪个空间比较。

`Vectors` 与 `VectorDim`、`Distance`、`Embedding` 互斥，维度、距离与 embedder
都在每个空间里单独配置。其中 `Distance` 为必填——它的零值是
`Distance_UnknownDistance`，`NewIndexer` 会直接拒绝，而不是留给服务端在建集合时报错。

### 为什么必须在同一个请求里写全所有空间

Qdrant 的 upsert 是整点替换。官方
[points 文档](https://qdrant.tech/documentation/manage-data/points/)的原话是
"the entire point is replaced, and any unspecified vectors are set to null"，
即未在请求中出现的向量会被置为 null。所以让两个 indexer 分别写同一个文档 ID 的
不同空间会互相清空，只能一次写全——这也意味着本 indexer 写入的 point 会丢掉它
没有配置的任何向量空间，包括在它之外创建的空间。

### 命名向量必须随集合一起创建

如果集合已存在，`NewIndexer` 会逐个校验配置的空间是否存在、维度与距离是否一致，
不一致则返回错误。它无法自行追加空间：本组件使用的 go-client（v1.15.x）中
`UpdateCollection.vectors_config` 是 `VectorsConfigDiff`，其 `VectorParamsDiff`
没有 size 与 distance，因此 gRPC API 无法新增带维度的命名向量。Qdrant 自 v1.18
起已在 [Update Vector Schema](https://qdrant.tech/documentation/manage-data/collections/#update-vector-schema)
一节提供该能力（`PUT /collections/{collection_name}/vectors/{vector_name}`），但 Go 客户端
尚无对应实现。请重建集合，在创建时一次配置好所有命名向量。

`Vectors` 留空时行为与本组件的历史版本完全一致，兼容旧集合。


## 示例

查看 `examples/default_indexer.go` 以获取完整的工作示例。

## 文档

- [Eino](https://github.com/cloudwego/eino)
- [Qdrant Go Client](https://github.com/qdrant/go-client)
- [Qdrant 文档](https://qdrant.tech/documentation/)

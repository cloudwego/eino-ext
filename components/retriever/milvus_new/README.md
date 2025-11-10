# Milvus New Retriever

## 概述

这是使用新版 Milvus Client 的 Retriever 实现，完全基于：
- `github.com/milvus-io/milvus/client/v2/milvusclient`
- `github.com/milvus-io/milvus/client/v2/entity`
- `github.com/milvus-io/milvus/client/v2/column`

**推荐新项目使用此版本**，旧版 milvus-sdk-go 已弃用。与新版 milvus_new indexer 完美兼容。

## 特性

✅ 使用最新的 Milvus Client V2 API (兼容 Milvus 2.6+)
✅ 支持列式数据查询（column-based query）
✅ 完整的 Collection 和 Partition 管理
✅ 灵活的过滤条件支持
✅ 内置 Embedding 支持
✅ 支持自定义数据转换

## 安装

```bash
go get github.com/milvus-io/milvus/client/v2@latest
go get github.com/cloudwego/eino-ext/components/retriever/milvus_new
```

## 快速开始

### 基本用法

```go
package main

import (
    "context"

    "github.com/cloudwego/eino-ext/components/retriever/milvus_new"
    "github.com/milvus-io/milvus/client/v2/milvusclient"
)

func main() {
    ctx := context.Background()

    // 1. 创建 Milvus Client
    client, err := milvusclient.New(ctx, &milvusclient.ClientConfig{
        Address: "localhost:19530",
    })
    if err != nil {
        panic(err)
    }
    defer client.Close(ctx)

    // 2. 配置 Retriever
    config := &milvus_new.RetrieverConfig{
        Client:         client,
        Collection:     "my_collection",
        VectorField:    "vector",
        Embedding:      yourEmbedding, // 你的 embedding 实现
        MetricType:     milvus_new.COSINE,
        TopK:           10,
        ScoreThreshold: 0.7,
    }

    // 3. 创建 Retriever
    retriever, err := milvus_new.NewRetriever(ctx, config)
    if err != nil {
        panic(err)
    }

    // 4. 检索文档
    docs, err := retriever.Retrieve(ctx, "search query")
    if err != nil {
        panic(err)
    }

    for _, doc := range docs {
        println("ID:", doc.ID, "Content:", doc.Content)
    }
}
```

### 使用过滤条件

```go
import "github.com/cloudwego/eino/components/retriever"

// 使用 milvus 表达式过滤
docs, err := retriever.Retrieve(ctx, "search query",
    milvus_new.WithFilter(`author == "John Doe" and year > 2020`),
    retriever.WithTopK(20),
)
```

### 使用 Partition

```go
// 在配置中指定默认分区
config := &milvus_new.RetrieverConfig{
    Client:     client,
    Collection: "my_collection",
    Partition:  "partition_2024",
    // ... 其他配置
}

// 或在检索时指定
docs, err := retriever.Retrieve(ctx, "search query",
    milvus_new.WithPartition("partition_2024"),
)
```

### 自定义数据转换

```go
import (
    "github.com/milvus-io/milvus/client/v2/column"
)

customConverter := func(ctx context.Context, columns []column.Column) ([]*schema.Document, error) {
    if len(columns) == 0 {
        return nil, nil
    }

    numDocs := columns[0].Len()
    result := make([]*schema.Document, numDocs)
    for i := range result {
        result[i] = &schema.Document{
            MetaData: make(map[string]any),
        }
    }

    // 处理每个列
    for _, col := range columns {
        switch col.Name() {
        case "id":
            for i := 0; i < col.Len(); i++ {
                val, _ := col.Get(i)
                if str, ok := val.(string); ok {
                    result[i].ID = str
                }
            }
        case "title":
            for i := 0; i < col.Len(); i++ {
                val, _ := col.Get(i)
                result[i].MetaData["title"] = val
            }
        case "content":
            for i := 0; i < col.Len(); i++ {
                val, _ := col.Get(i)
                if str, ok := val.(string); ok {
                    result[i].Content = str
                }
            }
        // ... 处理其他字段
        }
    }

    return result, nil
}

config := &milvus_new.RetrieverConfig{
    Client:            client,
    DocumentConverter: customConverter,
    // ... 其他配置
}
```

## 配置选项

### RetrieverConfig

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `Client` | `milvusclient.Client` | ✅ | - | Milvus 客户端实例 |
| `Embedding` | `embedding.Embedder` | ✅ | - | Embedding 实现 |
| `Collection` | `string` | ❌ | `"eino_collection"` | Collection 名称 |
| `Partition` | `string` | ❌ | `""` | Partition 名称 |
| `VectorField` | `string` | ❌ | `"vector"` | 向量字段名 |
| `OutputFields` | `[]string` | ❌ | `["id", "content", "metadata"]` | 返回的字段列表 |
| `MetricType` | `MetricType` | ❌ | `HAMMING` | 向量度量类型 |
| `TopK` | `int` | ❌ | `5` | 返回结果数量 |
| `ScoreThreshold` | `float64` | ❌ | `0` | 分数阈值 |
| `DocumentConverter` | `func` | ❌ | 默认转换器 | 自定义文档转换函数 |
| `VectorConverter` | `func` | ❌ | 默认转换器 | 自定义向量转换函数 |

### MetricType

- `L2` - 欧氏距离
- `IP` - 内积
- `COSINE` - 余弦相似度
- `HAMMING` - 汉明距离（默认）
- `JACCARD` - Jaccard 距离
- `TANIMOTO` - Tanimoto 距离

## 检索选项

### 通用选项（从 eino/components/retriever 继承）

```go
import "github.com/cloudwego/eino/components/retriever"

docs, err := retriever.Retrieve(ctx, "query",
    retriever.WithTopK(20),                    // 返回 Top 20
    retriever.WithScoreThreshold(0.8),         // 分数阈值 0.8
    retriever.WithEmbedding(customEmbedding),  // 自定义 Embedding
)
```

### Milvus 特定选项

```go
docs, err := retriever.Retrieve(ctx, "query",
    milvus_new.WithFilter(`year > 2020`),     // 过滤条件
    milvus_new.WithPartition("partition_01"), // 指定分区
    milvus_new.WithSearchOptFn(func(opt *milvusclient.SearchOption) *milvusclient.SearchOption {
        // 自定义搜索选项
        return opt.WithConsistencyLevel(entity.ClStrong)
    }),
)
```

## 与旧版的区别

### 旧版 (milvus_sdk_go v2.4.x)

```go
import "github.com/milvus-io/milvus-sdk-go/v2/client"
import "github.com/milvus-io/milvus-sdk-go/v2/entity"

// 返回 client.SearchResult
results, err := client.Search(ctx, collName, partitions, expr, outputFields,
    vectors, vectorField, metricType, topK, sp)

// 需要手动处理 SearchResult
for _, result := range results {
    for i := 0; i < result.IDs.Len(); i++ {
        id, _ := result.IDs.GetAsString(i)
        // ...
    }
}
```

### 新版 (milvus/client/v2)

```go
import "github.com/milvus-io/milvus/client/v2/milvusclient"
import "github.com/milvus-io/milvus/client/v2/column"

// 使用 Option 模式，更清晰
searchOpt := milvusclient.NewSearchOption(collection, topK, vectors).
    WithANNSField(vectorField).
    WithOutputFields(outputFields...).
    WithFilter(expr)

results, err := client.Search(ctx, searchOpt)

// 返回列式数据 []column.Column，更高效
for _, col := range results[0].Fields {
    for i := 0; i < col.Len(); i++ {
        val, _ := col.Get(i)
        // ...
    }
}
```

### 主要变化

1. **列式数据格式**: 新版使用列式数据，性能更好
2. **Option 模式**: 所有操作使用统一的 Option 模式，更清晰
3. **更好的类型安全**: 编译时类型检查
4. **简化的 API**: 更简洁易用

## 最佳实践

### 1. 选择合适的度量类型

```go
// 对于归一化向量
config.MetricType = milvus_new.COSINE  // 或 IP

// 对于未归一化向量
config.MetricType = milvus_new.L2

// 对于二进制向量
config.MetricType = milvus_new.HAMMING
```

### 2. 合理设置 TopK

```go
// 根据业务需求设置
config.TopK = 10  // 一般场景

// 或在检索时动态调整
docs, err := retriever.Retrieve(ctx, query,
    retriever.WithTopK(50),  // 需要更多结果时
)
```

### 3. 使用过滤条件优化查询

```go
// 使用 Milvus 表达式语法
docs, err := retriever.Retrieve(ctx, query,
    milvus_new.WithFilter(`category in ["tech", "science"] and year >= 2020`),
)
```

### 4. 指定必要的输出字段

```go
// 只返回需要的字段，减少网络传输
config.OutputFields = []string{"id", "content", "title"}
```

### 5. 错误处理

```go
docs, err := retriever.Retrieve(ctx, query)
if err != nil {
    if strings.Contains(err.Error(), "collection not found") {
        // Collection 不存在
    } else if strings.Contains(err.Error(), "embedding has error") {
        // Embedding 错误
    }
    // 其他错误处理
}
```

## 故障排查

### Collection 不存在

```
Error: [NewRetriever] collection not found
```

**解决方案**: 确保 Collection 已创建并且名称正确

### Collection 未加载

```
Error: [NewRetriever] failed to load collection
```

**解决方案**: 使用 `client.LoadCollection()` 手动加载 Collection

### 向量字段不存在

```
Error: [NewRetriever] collection schema not match: vector field not found
```

**解决方案**: 检查 `VectorField` 配置是否与 Collection Schema 中的字段名一致

### 检索结果为空

```
返回空结果
```

**可能原因**:
1. 查询向量与数据库中的向量相似度太低
2. ScoreThreshold 设置过高
3. Filter 过滤条件过于严格
4. Collection 中没有数据

**解决方案**:
- 降低 ScoreThreshold
- 检查过滤条件
- 确认数据已正确插入

## 性能优化

### 1. 批量检索

```go
queries := []string{"query1", "query2", "query3"}
for _, query := range queries {
    docs, err := retriever.Retrieve(ctx, query)
    // 处理结果
}
```

### 2. 并发检索

```go
import "sync"

var wg sync.WaitGroup
results := make([][]*schema.Document, len(queries))

for i, query := range queries {
    wg.Add(1)
    go func(idx int, q string) {
        defer wg.Done()
        docs, err := retriever.Retrieve(ctx, q)
        if err == nil {
            results[idx] = docs
        }
    }(i, query)
}
wg.Wait()
```

### 3. 使用连接池

```go
// Milvus Client 已内置连接池管理
// 只需创建一个 client 实例并复用
client, err := milvusclient.New(ctx, &milvusclient.ClientConfig{
    Address: "localhost:19530",
})
// 复用这个 client 创建多个 retriever
```

## 与 milvus_new Indexer 配合使用

```go
// 1. 创建 Indexer 并存储文档
indexerConfig := &milvus_new.IndexerConfig{
    Client:     client,
    Collection: "my_collection",
    Embedding:  embedder,
}
indexer, err := milvus_new.NewIndexer(ctx, indexerConfig)
ids, err := indexer.Store(ctx, docs)

// 2. 创建 Retriever 并检索
retrieverConfig := &milvus_new.RetrieverConfig{
    Client:     client,
    Collection: "my_collection",
    Embedding:  embedder,
    TopK:       10,
}
retriever, err := milvus_new.NewRetriever(ctx, retrieverConfig)
results, err := retriever.Retrieve(ctx, "search query")
```

## 兼容性说明

- **Milvus 版本**: 2.6.0+
- **Go 版本**: 1.19+
- **Milvus Client**: github.com/milvus-io/milvus/client/v2 v2.6.0+

## 相关资源

- [Milvus V2 Client 文档](https://milvus.io/docs)
- [Milvus 表达式语法](https://milvus.io/docs/boolean.md)
- [Milvus GitHub](https://github.com/milvus-io/milvus)
- [Eino 框架](https://github.com/cloudwego/eino)

## 迁移指南

从旧版 `milvus` retriever 迁移到 `milvus_new`:

1. 更新导入路径
   ```go
   // 旧版
   import "github.com/cloudwego/eino-ext/components/retriever/milvus"

   // 新版
   import "github.com/cloudwego/eino-ext/components/retriever/milvus_new"
   ```

2. 更新客户端创建代码
   ```go
   // 旧版
   client, err := client.NewGrpcClient(ctx, "localhost:19530")

   // 新版
   client, err := milvusclient.New(ctx, &milvusclient.ClientConfig{
       Address: "localhost:19530",
   })
   ```

3. 更新配置
   ```go
   // 旧版
   config := &milvus.RetrieverConfig{
       Client: client,
       // ...
   }

   // 新版
   config := &milvus_new.RetrieverConfig{
       Client: client,
       // ...
   }
   ```

4. 如果有自定义 DocumentConverter，更新为接收 `[]column.Column`
   ```go
   // 新版 DocumentConverter 签名
   func(ctx context.Context, columns []column.Column) ([]*schema.Document, error)
   ```
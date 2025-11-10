# Milvus New Indexer

## 概述

这是使用新版 Milvus Client 的 Indexer 实现，完全基于：
- `github.com/milvus-io/milvus/client/v2/milvusclient`
- `github.com/milvus-io/milvus/client/v2/entity`
- `github.com/milvus-io/milvus/client/v2/column`
- `github.com/milvus-io/milvus/client/v2/index`

**推荐新项目使用此版本**，旧版 milvus-sdk-go 已弃用。

## 特性

✅ 使用最新的 Milvus Client V2 API
✅ 支持列式数据插入（column-based insert）
✅ 支持自动索引创建
✅ 完整的 Collection 和 Partition 管理
✅ 灵活的 Schema 配置
✅ 内置 Embedding 支持

## 安装

```bash
go get github.com/milvus-io/milvus/client/v2@latest
go get github.com/cloudwego/eino-ext/components/indexer/milvus_new
```

## 快速开始

### 基本用法

```go
package main

import (
    "context"

    "github.com/cloudwego/eino-ext/components/indexer/milvus_new"
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

    // 2. 配置 Indexer
    config := &milvus_new.IndexerConfig{
        Client:              client,
        Collection:          "my_collection",
        Description:         "My document collection",
        Embedding:           yourEmbedding, // 你的 embedding 实现
        ConsistencyLevel:    milvus_new.ConsistencyLevelBounded,
        EnableDynamicSchema: false,
        MetricType:          milvus_new.COSINE,
        SharedNum:           1,
    }

    // 3. 创建 Indexer
    indexer, err := milvus_new.NewIndexer(ctx, config)
    if err != nil {
        panic(err)
    }

    // 4. 存储文档
    docs := []*schema.Document{
        {
            ID:      "doc1",
            Content: "This is a sample document",
            MetaData: map[string]interface{}{
                "author": "John Doe",
            },
        },
    }

    ids, err := indexer.Store(ctx, docs)
    if err != nil {
        panic(err)
    }

    println("Stored documents with IDs:", ids)
}
```

### 自定义 Schema

```go
import (
    "github.com/milvus-io/milvus/client/v2/entity"
)

// 定义自定义字段
fields := []*entity.Field{
    {
        Name:        "id",
        Description: "Document ID",
        DataType:    entity.FieldTypeVarChar,
        PrimaryKey:  true,
        TypeParams: map[string]interface{}{
            "max_length": int64(255),
        },
    },
    {
        Name:        "title",
        Description: "Document title",
        DataType:    entity.FieldTypeVarChar,
        TypeParams: map[string]interface{}{
            "max_length": int64(512),
        },
    },
    {
        Name:        "content",
        Description: "Document content",
        DataType:    entity.FieldTypeVarChar,
        TypeParams: map[string]interface{}{
            "max_length": int64(2048),
        },
    },
    {
        Name:        "embedding",
        Description: "Document vector embedding",
        DataType:    entity.FieldTypeFloatVector,
        TypeParams: map[string]interface{}{
            "dim": int64(768), // 向量维度
        },
    },
    {
        Name:        "metadata",
        Description: "Document metadata",
        DataType:    entity.FieldTypeJSON,
    },
}

config := &milvus_new.IndexerConfig{
    Client: client,
    Fields: fields,
    // ... 其他配置
}
```

### 使用 Partition

```go
config := &milvus_new.IndexerConfig{
    Client:        client,
    Collection:    "my_collection",
    PartitionName: "partition_2024",
    // ... 其他配置
}

// 或在存储时指定
ids, err := indexer.Store(ctx, docs, milvus_new.WithPartition("partition_2024"))
```

### 自定义数据转换

```go
import (
    "github.com/milvus-io/milvus/client/v2/column"
)

customConverter := func(ctx context.Context, docs []*schema.Document, vectors [][]float64) ([]column.Column, error) {
    // 提取字段数据
    ids := make([]string, len(docs))
    titles := make([]string, len(docs))
    contents := make([]string, len(docs))
    embeddings := make([][]float32, len(docs))
    metadata := make([][]byte, len(docs))

    for i, doc := range docs {
        ids[i] = doc.ID
        // 从 doc.MetaData 提取 title
        if title, ok := doc.MetaData["title"].(string); ok {
            titles[i] = title
        }
        contents[i] = doc.Content

        // 转换 vector 为 float32
        embeddings[i] = make([]float32, len(vectors[i]))
        for j, v := range vectors[i] {
            embeddings[i][j] = float32(v)
        }

        // 序列化 metadata
        metadataBytes, _ := json.Marshal(doc.MetaData)
        metadata[i] = metadataBytes
    }

    // 创建列
    return []column.Column{
        column.NewColumnVarChar("id", ids),
        column.NewColumnVarChar("title", titles),
        column.NewColumnVarChar("content", contents),
        column.NewColumnFloatVector("embedding", 768, embeddings),
        column.NewColumnJSONBytes("metadata", metadata),
    }, nil
}

config := &milvus_new.IndexerConfig{
    Client:            client,
    DocumentConverter: customConverter,
    // ... 其他配置
}
```

## 配置选项

### IndexerConfig

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `Client` | `milvusclient.Client` | ✅ | - | Milvus 客户端实例 |
| `Embedding` | `embedding.Embedder` | ✅ | - | Embedding 实现 |
| `Collection` | `string` | ❌ | `"eino_collection"` | Collection 名称 |
| `Description` | `string` | ❌ | `"the collection for eino"` | Collection 描述 |
| `Fields` | `[]*entity.Field` | ❌ | 默认 Schema | 自定义字段定义 |
| `SharedNum` | `int32` | ❌ | `1` | Shard 数量 |
| `ConsistencyLevel` | `ConsistencyLevel` | ❌ | `ConsistencyLevelBounded` | 一致性级别 |
| `EnableDynamicSchema` | `bool` | ❌ | `false` | 是否启用动态 Schema |
| `PartitionNum` | `int64` | ❌ | `0` | Partition 数量（用于 partition key 模式） |
| `PartitionName` | `string` | ❌ | `""` | 指定 Partition 名称 |
| `MetricType` | `MetricType` | ❌ | `HAMMING` | 向量度量类型 |
| `DocumentConverter` | `func` | ❌ | 默认转换器 | 自定义文档转换函数 |

### ConsistencyLevel

- `ConsistencyLevelStrong` - 强一致性
- `ConsistencyLevelSession` - 会话一致性
- `ConsistencyLevelBounded` - 有界一致性（默认）
- `ConsistencyLevelEventually` - 最终一致性
- `ConsistencyLevelCustomized` - 自定义一致性

### MetricType

- `L2` - 欧氏距离
- `IP` - 内积
- `COSINE` - 余弦相似度
- `HAMMING` - 汉明距离（默认）
- `JACCARD` - Jaccard 距离
- `TANIMOTO` - Tanimoto 距离

## 默认 Schema

默认情况下，Indexer 使用以下 Schema：

| 字段名 | 类型 | 说明 | 主键 |
|--------|------|------|------|
| `id` | `VarChar(255)` | 文档唯一 ID | ✅ |
| `content` | `VarChar(1024)` | 文档内容 | ❌ |
| `vector` | `BinaryVector(81920)` | 文档向量（binary） | ❌ |
| `metadata` | `JSON` | 文档元数据 | ❌ |

## 与旧版的区别

### 旧版 (milvus_sdk_go v2.4.x)

```go
import "github.com/milvus-io/milvus-sdk-go/v2/client"
import "github.com/milvus-io/milvus-sdk-go/v2/entity"

// 行式插入
rows := []interface{}{row1, row2, ...}
client.InsertRows(ctx, collName, partition, rows)
```

### 新版 (milvus/client/v2)

```go
import "github.com/milvus-io/milvus/client/v2/milvusclient"
import "github.com/milvus-io/milvus/client/v2/entity"
import "github.com/milvus-io/milvus/client/v2/column"

// 列式插入（更高效）
columns := []column.Column{col1, col2, ...}
client.Insert(ctx, option, columns...)
```

### 主要变化

1. **列式数据格式**: 新版使用列式存储，性能更好
2. **类型系统**: Field 定义使用 struct 而非 builder 模式
3. **Option 模式**: 所有操作使用统一的 Option 模式
4. **更好的类型安全**: 编译时类型检查

## 最佳实践

### 1. 选择合适的向量类型

```go
// FloatVector - 标准浮点向量（推荐用于大多数场景）
{
    Name: "embedding",
    DataType: entity.FieldTypeFloatVector,
    TypeParams: map[string]interface{}{"dim": int64(768)},
}

// BinaryVector - 二进制向量（内存占用小）
{
    Name: "embedding",
    DataType: entity.FieldTypeBinaryVector,
    TypeParams: map[string]interface{}{"dim": int64(81920)},
}
```

### 2. 合理设置一致性级别

- **实时性要求高**: 使用 `ConsistencyLevelStrong`
- **平衡场景**: 使用 `ConsistencyLevelBounded`（默认）
- **性能优先**: 使用 `ConsistencyLevelEventually`

### 3. 批量插入优化

```go
// 批量处理文档，每次 100-1000 条
batchSize := 500
for i := 0; i < len(allDocs); i += batchSize {
    end := i + batchSize
    if end > len(allDocs) {
        end = len(allDocs)
    }
    batch := allDocs[i:end]
    ids, err := indexer.Store(ctx, batch)
    // 处理结果...
}
```

### 4. 错误处理

```go
ids, err := indexer.Store(ctx, docs)
if err != nil {
    // 检查是否是客户端错误
    if strings.Contains(err.Error(), "client not ready") {
        // 重连逻辑
    }
    // 其他错误处理
}
```

## 故障排查

### Collection 创建失败

```
Error: collection already exists
```

**解决方案**: 检查 Collection 是否已存在，或使用不同的名称

### Schema 不匹配

```
Error: collection schema not match
```

**解决方案**: 确保 Fields 配置与现有 Collection 的 Schema 一致

### 向量维度错误

```
Error: invalid dimension
```

**解决方案**: 检查 Field TypeParams 中的 dim 与实际 embedding 维度是否一致

## 相关资源

- [Milvus V2 Client 文档](https://milvus.io/docs)
- [Milvus GitHub](https://github.com/milvus-io/milvus)
- [Eino 框架](https://github.com/cloudwego/eino)

## 迁移指南

从旧版 `milvus` 包迁移到 `milvus_new`:

1. 更新导入路径
2. 修改客户端创建代码
3. 如果有自定义 Schema，更新 Field 定义格式
4. 如果有自定义 DocumentConverter，改为返回 `[]column.Column`

详见旧版文档中的适配器说明。
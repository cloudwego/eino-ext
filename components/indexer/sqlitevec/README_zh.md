# SQLiteVec Indexer

SQLiteVec Indexer 使用 `modernc.org/sqlite/vec` 注册的 `sqlite-vec` 扩展，将 Eino 文档和 dense embedding 写入 SQLite。

该组件适合嵌入式、本地、小规模 RAG、示例和单元测试场景。大规模生产向量检索建议优先使用 Milvus、Qdrant、Elasticsearch、Redis、OpenSearch 等服务型后端。

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/indexer/sqlitevec@latest
```

当前组件要求 Go 1.25.10+，原因是 `modernc.org/sqlite/vec` 所在的 `modernc.org/sqlite` 版本要求 Go 1.25 或更高版本。

## 用法

```go
db, err := sql.Open("sqlite", "rag.db")
if err != nil {
    return err
}
defer db.Close()

idx, err := sqlitevec.NewIndexer(ctx, &sqlitevec.Config{
    DB:        db,
    VectorDim: 1536,
    Embedding: embedder,
})
if err != nil {
    return err
}

ids, err := idx.Store(ctx, []*schema.Document{
    {
        ID:      "doc-1",
        Content: "Eino is an LLM application framework.",
        MetaData: map[string]any{
            "source": "manual",
        },
    },
})
```

## 配置

```go
type Config struct {
    DB                *sql.DB
    DocumentTable     string
    VectorTable       string
    VectorDim         int
    BatchSize         int
    Embedding         embedding.Embedder
    DisableAutoCreate bool
}
```

默认值：

- `DocumentTable`: `eino_sqlitevec_documents`
- `VectorTable`: `eino_sqlitevec_vectors`
- `BatchSize`: `10`
- `DisableAutoCreate`: `false`

表名必须匹配 `[A-Za-z_][A-Za-z0-9_]*`。

## 表结构

Indexer 使用普通 SQLite 表保存文档字段，使用 `vec0` 虚拟表保存向量。向量表的 `rowid` 与文档表 integer 主键对齐。

配套 Retriever 必须使用相同的 `DB`、表名、`VectorDim` 和 embedding 模型。

## 注意事项

- 文档 ID 不能为空。
- 重复写入同一文档 ID 会更新 content、metadata 和 vector。
- metadata 使用 JSON 存储。
- 该组件会 blank import `modernc.org/sqlite` 和 `modernc.org/sqlite/vec`。

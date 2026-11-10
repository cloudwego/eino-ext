# SQLiteVec Retriever

SQLiteVec Retriever 使用 `modernc.org/sqlite/vec` 注册的 `sqlite-vec` 扩展，在 SQLite 中按向量相似度检索 Eino 文档。

该组件适合嵌入式、本地、小规模 RAG、示例和单元测试场景。大规模生产向量检索建议优先使用 Milvus、Qdrant、Elasticsearch、Redis、OpenSearch 等服务型后端。

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/retriever/sqlitevec@latest
```

当前组件要求 Go 1.25.10+，原因是 `modernc.org/sqlite/vec` 所在的 `modernc.org/sqlite` 版本要求 Go 1.25 或更高版本。

## 用法

```go
db, err := sql.Open("sqlite", "rag.db")
if err != nil {
    return err
}
defer db.Close()

ret, err := sqlitevec.NewRetriever(ctx, &sqlitevec.Config{
    DB:        db,
    VectorDim: 1536,
    Embedding: embedder,
    TopK:      5,
})
if err != nil {
    return err
}

docs, err := ret.Retrieve(ctx, "what is Eino?", retriever.WithTopK(3))
```

## 配置

```go
type Config struct {
    DB            *sql.DB
    DocumentTable string
    VectorTable   string
    VectorDim     int
    TopK          int
    Embedding     embedding.Embedder
}
```

默认值：

- `DocumentTable`: `eino_sqlitevec_documents`
- `VectorTable`: `eino_sqlitevec_vectors`
- `TopK`: `5`

表名必须匹配 `[A-Za-z_][A-Za-z0-9_]*`。

## Options

- `retriever.WithTopK(k)` 覆盖默认结果数量。
- `retriever.WithEmbedding(emb)` 覆盖配置中的 embedder。
- `retriever.WithScoreThreshold(score)` 按 Eino similarity score 过滤。
- `sqlitevec.WithMaxDistance(distance)` 按 sqlite-vec 原始 distance 过滤。

SQLiteVec distance 越小越相似。Eino score 计算方式：

```text
score = 1 / (1 + distance)
```

原始 distance 会写入 `doc.MetaData["sqlitevec_distance"]`。

## 注意事项

- 配套 Indexer 和 Retriever 必须使用相同的 `DB`、表名、`VectorDim` 和 embedding 模型。
- Retriever 的 SQL join 依赖配套 Indexer 创建的固定表结构：文档表需要名为 `id` 的 integer 主键列，并且向量表的 `rowid` 需要与该 `id` 对齐，因为检索时使用 `JOIN ... ON d.id = v.rowid`。
- 如果使用自建表或自行导入数据，需要保证上述 schema 和 rowid 对齐约束。
- 首版有意不支持 raw SQL filter。
- 该组件会 blank import `modernc.org/sqlite` 和 `modernc.org/sqlite/vec`。

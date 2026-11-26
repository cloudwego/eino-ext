# SQLiteVec Indexer

SQLiteVec Indexer stores Eino documents and dense embeddings in SQLite using the `sqlite-vec` extension registered by `modernc.org/sqlite/vec`.

This component is intended for embedded, local, test, and small-scale RAG scenarios. For large production vector search workloads, prefer service-backed stores such as Milvus, Qdrant, Elasticsearch, Redis, or OpenSearch.

## Install

```bash
go get github.com/cloudwego/eino-ext/components/indexer/sqlitevec@latest
```

This component currently requires Go 1.25.10+ because `modernc.org/sqlite/vec` is available in `modernc.org/sqlite` versions that require Go 1.25 or newer.

## Usage

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

## Config

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

Defaults:

- `DocumentTable`: `eino_sqlitevec_documents`
- `VectorTable`: `eino_sqlitevec_vectors`
- `BatchSize`: `10`
- `DisableAutoCreate`: `false`

Table names must match `[A-Za-z_][A-Za-z0-9_]*`.

## Schema

The indexer stores document fields in a normal SQLite table and vectors in a `vec0` virtual table. The vector table `rowid` is aligned with the document table integer primary key.

The paired retriever must use the same `DB`, table names, `VectorDim`, and embedding model.

## Notes

- Document IDs must be non-empty.
- Re-storing an existing document ID updates content, metadata, and vector.
- Metadata is stored as JSON.
- This component blank-imports `modernc.org/sqlite` and `modernc.org/sqlite/vec`.

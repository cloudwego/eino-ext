# SQLiteVec Retriever

SQLiteVec Retriever searches Eino documents stored in SQLite by vector similarity using the `sqlite-vec` extension registered by `modernc.org/sqlite/vec`.

This component is intended for embedded, local, test, and small-scale RAG scenarios. For large production vector search workloads, prefer service-backed stores such as Milvus, Qdrant, Elasticsearch, Redis, or OpenSearch.

## Install

```bash
go get github.com/cloudwego/eino-ext/components/retriever/sqlitevec@latest
```

This component currently requires Go 1.25.10+ because `modernc.org/sqlite/vec` is available in `modernc.org/sqlite` versions that require Go 1.25 or newer.

## Usage

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

## Config

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

Defaults:

- `DocumentTable`: `eino_sqlitevec_documents`
- `VectorTable`: `eino_sqlitevec_vectors`
- `TopK`: `5`

Table names must match `[A-Za-z_][A-Za-z0-9_]*`.

## Options

- `retriever.WithTopK(k)` overrides the default result count.
- `retriever.WithEmbedding(emb)` overrides the configured embedder.
- `retriever.WithScoreThreshold(score)` filters by Eino similarity score.
- `sqlitevec.WithMaxDistance(distance)` filters by raw sqlite-vec distance.

SQLiteVec distance is lower-is-better. Eino score is calculated as:

```text
score = 1 / (1 + distance)
```

The raw distance is stored in `doc.MetaData["sqlitevec_distance"]`.

## Notes

- The paired indexer and retriever must use the same `DB`, table names, `VectorDim`, and embedding model.
- The retriever expects the document table schema created by the paired indexer: the document table must have an integer primary key column named `id`, and the vector table `rowid` must be aligned to that `id` because retrieval joins documents and vectors with `d.id = v.rowid`.
- If you use pre-existing or custom tables instead of the paired indexer, make sure they preserve this schema and rowid alignment.
- Raw SQL filters are intentionally not supported in the first version.
- This component blank-imports `modernc.org/sqlite` and `modernc.org/sqlite/vec`.

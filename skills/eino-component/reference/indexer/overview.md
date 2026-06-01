# Indexer Overview

Indexers store documents (with optional vector embeddings) in a backend for later retrieval.

## Interface

```go
// github.com/cloudwego/eino/components/indexer
type Indexer interface {
    Store(ctx context.Context, docs []*schema.Document, opts ...Option) (ids []string, err error)
}
```

Common options:
- `indexer.WithEmbedding(emb embedding.Embedder)` -- embedder to generate vectors before storing
- `indexer.WithSubIndexes(indexes ...string)` -- write to logical sub-partitions

## Implementations

| Backend | Package | Key Config |
|---------|---------|------------|
| Redis | `indexer/redis` | Client, KeyPrefix, BatchSize, Embedding |
| Milvus 2.x | `indexer/milvus2` | Client, Collection, Embedding |
| Elasticsearch 8 | `indexer/es8` | Client, Index, Embedding |
| Qdrant | `indexer/qdrant` | Client, CollectionName, Embedding |

See `indexer/{backend}.md` for per-backend config and examples.

## Full Indexing Pipeline

Load, parse, split, then index:

```go
// 1. Load documents
loader, _ := file.NewFileLoader(ctx, &file.FileLoaderConfig{UseNameAsID: true})
docs, _ := loader.Load(ctx, document.Source{URI: "/path/to/file.txt"})

// 2. Split into chunks
splitter, _ := recursive.NewSplitter(ctx, &recursive.Config{
    ChunkSize: 1500, OverlapSize: 300,
})
chunks, _ := splitter.Transform(ctx, docs)

// 3. Index with embeddings
ids, _ := indexer.Store(ctx, chunks)
```

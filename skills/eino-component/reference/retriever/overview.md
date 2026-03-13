# Retriever Overview

Retrievers find the most relevant documents by vector similarity.

## Interface

```go
// github.com/cloudwego/eino/components/retriever
type Retriever interface {
    Retrieve(ctx context.Context, query string, opts ...Option) ([]*schema.Document, error)
}
```

Common options:
- `retriever.WithTopK(k int)` -- max number of results
- `retriever.WithScoreThreshold(t float64)` -- min relevance score filter
- `retriever.WithEmbedding(emb embedding.Embedder)` -- embedder for query vectorization

## Implementations

| Backend | Package | Key Config |
|---------|---------|------------|
| Redis | `retriever/redis` | Client, Index, VectorField, TopK |
| Milvus 2.x | `retriever/milvus2` | Client, Collection, SearchMode |
| Elasticsearch 8 | `retriever/es8` | Client, Index, SearchMode |
| Qdrant | `retriever/qdrant` | Client, CollectionName |

See `retriever/{backend}.md` for per-backend config and examples.

## RAG Retrieval Example

```go
// The retriever handles embedding internally when Embedding is configured
docs, err := retriever.Retrieve(ctx, "How does Eino handle streaming?",
    retriever.WithTopK(5),
)

for _, doc := range docs {
    fmt.Printf("ID: %s\nContent: %s\nScore: %v\n\n",
        doc.ID, doc.Content, doc.MetaData["score"])
}
```

The same Embedder model must be used for indexing and retrieval. Mismatched models will produce incorrect similarity scores.

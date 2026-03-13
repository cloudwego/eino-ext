# PGVector Retriever

pgvector Retriever for Eino framework - retrieve documents with vector similarity search from PostgreSQL using the pgvector extension.

## Features

- **Multiple distance functions** - cosine, L2 (Euclidean), and inner product
- **Score threshold filtering** - filter results by similarity score
- **Custom WHERE clauses** - filter results by metadata or other conditions
- **SQL injection protection** - validates table names and identifiers
- **Connection pooling** support via `pgxpool.Pool`
- **Eino callbacks** integration for observability

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/retriever/pgvector
```

## Prerequisites

1. **PostgreSQL** with pgvector extension installed
2. **Table with vectors** - documents should be stored with vector embeddings
3. **Vector index** (optional but recommended) for performance:

```sql
CREATE INDEX ON documents USING hnsw (embedding vector_cosine_ops);
-- or
CREATE INDEX ON documents USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);
```

## Usage

### Basic Example

```go
import (
    "context"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/cloudwego/eino-ext/components/retriever/pgvector"
    "github.com/cloudwego/eino/components/embedding/openai"
)

func main() {
    ctx := context.Background()

    // Create connection pool
    pool, err := pgxpool.New(ctx, "postgres://user:pass@localhost/dbname")
    if err != nil {
        panic(err)
    }
    defer pool.Close()

    // Create retriever
    retriever, err := pgvector.NewRetriever(ctx, &pgvector.RetrieverConfig{
        Conn:      pool,
        TableName: "documents",
        Embedding: openai.NewEmbedder(),
        TopK:      5,
    })
    if err != nil {
        panic(err)
    }

    // Retrieve similar documents
    docs, err := retriever.Retrieve(ctx, "search query")
    if err != nil {
        panic(err)
    }

    for _, doc := range docs {
        fmt.Printf("ID: %s, Score: %.2f, Content: %s\n",
            doc.ID, doc.Score, doc.Content)
    }
}
```

### Using with Distance Function

```go
retriever, err := pgvector.NewRetriever(ctx, &pgvector.RetrieverConfig{
    Conn:             pool,
    TableName:        "documents",
    Embedding:        openai.NewEmbedder(),
    TopK:             10,
    DistanceFunction: pgvector.DistanceL2,  // Use L2 distance
})
```

### Using with Score Threshold

```go
threshold := 0.8
retriever, err := pgvector.NewRetriever(ctx, &pgvector.RetrieverConfig{
    Conn:           pool,
    TableName:      "documents",
    Embedding:      openai.NewEmbedder(),
    TopK:           10,
    ScoreThreshold: &threshold,  // Only return docs with score >= 0.8
})
```

### Using with Custom WHERE Clause

```go
docs, err := retriever.Retrieve(ctx, "search query",
    pgvector.WithWhereClause("metadata->>'category' = 'tech'"),
)
```

### Using with Different Distance Function at Runtime

```go
docs, err := retriever.Retrieve(ctx, "search query",
    pgvector.WithDistanceFunction(pgvector.DistanceIP),
)
```

## Configuration

### RetrieverConfig

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Conn` | `PgxConn` | *required* | pgx connection or pool |
| `TableName` | `string` | `"documents"` | Table name for storing documents |
| `DistanceFunction` | `DistanceFunction` | `DistanceCosine` | Distance function for similarity |
| `TopK` | `int` | `5` | Maximum number of documents to retrieve |
| `Embedding` | `embedding.Embedder` | *required* | Embedding model for query vectorization |
| `ScoreThreshold` | `*float64` | `nil` | Minimum similarity score (0-1) |

### Distance Functions

| Function | Operator | Best For |
|----------|----------|----------|
| `DistanceCosine` | `<=>` | Text similarity, normalized vectors |
| `DistanceL2` | `<->` | General purpose, geometric distance |
| `DistanceIP` | `<#>` | Normalized vectors, negative inner product |

### Table Schema

The retriever expects a table with this schema:

```sql
CREATE TABLE table_name (
    id TEXT PRIMARY KEY,
    content TEXT NOT NULL,
    embedding vector(N),  -- N = vector dimension
    metadata JSONB
);
```

## Performance Tips

1. **Use connection pooling** - `pgxpool.Pool` for concurrent access
2. **Create vector indexes** - Use HNSW or IVFFlat indexes
3. **Tune index parameters** - Adjust `lists` for IVFFlat based on data size
4. **Use score thresholds** - Filter out low-similarity results early
5. **Use WHERE clauses** - Pre-filter by metadata before vector search

## Distance Function Selection

- **Cosine (`<=>`)**: Best for text similarity, measures angle between vectors
  - Range: 0 (identical) to 2 (opposite)
  - Score calculation: `1 - distance`

- **L2 (`<->`)**: Euclidean distance, measures straight-line distance
  - Range: 0 (identical) to ∞
  - Score calculation: `1 / (1 + distance)`

- **Inner Product (`<#>`)**: Negative inner product, fast for normalized vectors
  - Range: -∞ to ∞
  - Score calculation: `1 / (1 + distance)`

## Dependencies

- `github.com/cloudwego/eino` - Eino framework
- `github.com/jackc/pgx/v5` - PostgreSQL driver (v5.5.1+)
- `github.com/pgvector/pgvector-go` - pgvector Go library (v0.3.0+)

## Compatibility

- **PostgreSQL**: 12+
- **pgvector extension**: 0.5.0+
- **Go**: 1.23+

## Error Handling

The retriever returns detailed errors with context:

```go
[NewRetriever] embedding not provided for pgvector retriever
[NewRetriever] database connection not provided
[NewRetriever] invalid distance function: <cause>
[NewRetriever] invalid table name: <cause>
[Retrieve] failed to embed query: <cause>
[Retrieve] query failed: <cause>
```

## Testing

Run tests:

```bash
go test -v ./...
```

## License

Apache License 2.0

## See Also

- [pgvector Documentation](https://github.com/pgvector/pgvector)
- [PGVector Indexer](../indexer/pgvector) - Store documents with vectors
- [Eino Framework](https://github.com/cloudwego/eino)

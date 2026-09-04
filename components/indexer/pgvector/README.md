# PGVector Indexer

pgvector Indexer for Eino framework - store and retrieve documents with vector embeddings in PostgreSQL using the pgvector extension.

## Features

- **Type-safe vector operations** using `pgvector.Vector` from official `pgvector-go` library
- **Batch processing** for efficient embedding and storage
- **Automatic conflict resolution** with UPSERT semantics
- **SQL injection protection** with identifier validation
- **Connection pooling** support via `pgxpool.Pool`
- **Eino callbacks** integration for observability

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/indexer/pgvector
```

## Prerequisites

1. **PostgreSQL** with pgvector extension installed
2. **Create table** before using the indexer:

```sql
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE documents (
    id TEXT PRIMARY KEY,
    content TEXT NOT NULL,
    embedding vector(1536),  -- adjust dimension based on your model
    metadata JSONB
);

-- Optional: create index for vector similarity search
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
    "github.com/cloudwego/eino-ext/components/indexer/pgvector"
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

    // Create indexer
    indexer, err := pgvector.NewIndexer(ctx, &pgvector.IndexerConfig{
        Conn:      pool,
        TableName: "documents",
        Embedding: openai.NewEmbedder(), // or any embedding implementation
        BatchSize: 10,
    })
    if err != nil {
        panic(err)
    }

    // Store documents
    docs := []*schema.Document{
        {
            ID:      "doc1",
            Content: "Hello world",
            MetaData: map[string]any{
                "category": "greeting",
            },
        },
        // ... more documents
    }

    ids, err := indexer.Store(ctx, docs)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Stored %d documents\n", len(ids))
}
```

## Configuration

### IndexerConfig

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Conn` | `PgxConn` | *required* | pgx connection or pool |
| `TableName` | `string` | `"documents"` | Table name for storing documents |
| `Embedding` | `embedding.Embedder` | *required for Store* | Embedding model for vectorization |
| `BatchSize` | `int` | `10` | Batch size for embedding operations |

### Table Schema

The indexer expects a table with this schema:

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
2. **Adjust BatchSize** - Larger batches (10-100) improve throughput
3. **Create vector indexes** - Use HNSW or IVFFlat indexes for similarity search
4. **Tune index parameters** - Adjust `lists` for IVFFlat based on data size

## Dependencies

- `github.com/cloudwego/eino` - Eino framework
- `github.com/jackc/pgx/v5` - PostgreSQL driver (v5.5.1+)
- `github.com/pgvector/pgvector-go` - pgvector Go library (v0.3.0+)

## Compatibility

- **PostgreSQL**: 12+
- **pgvector extension**: 0.5.0+
- **Go**: 1.23+

## Error Handling

The indexer returns detailed errors with context:

```go
[NewIndexer] database connection not provided
[Indexer.Store] documents list is empty
[Indexer.Store] embedding failed: <cause>
[Indexer.Store] batch execution failed: <cause>
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
- [pgvector-go](https://github.com/pgvector/pgvector-go)
- [Eino Framework](https://github.com/cloudwego/eino)

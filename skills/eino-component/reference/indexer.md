# Indexer Reference

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

| Backend | Import Path | Key Config |
|---------|-------------|------------|
| Redis | `github.com/cloudwego/eino-ext/components/indexer/redis` | Client, KeyPrefix, BatchSize, Embedding |
| Milvus 2.x | `github.com/cloudwego/eino-ext/components/indexer/milvus2` | Client, Collection, Embedding |
| Elasticsearch 8 | `github.com/cloudwego/eino-ext/components/indexer/es8` | Client, Index, Embedding |
| Elasticsearch 9 | `github.com/cloudwego/eino-ext/components/indexer/es9` | Client, Index |
| Elasticsearch 7 | `github.com/cloudwego/eino-ext/components/indexer/es7` | Client, Index |
| Qdrant | `github.com/cloudwego/eino-ext/components/indexer/qdrant` | Client, CollectionName, Embedding |

## Redis Indexer Example

```go
import (
    "github.com/redis/go-redis/v9"
    redisIndexer "github.com/cloudwego/eino-ext/components/indexer/redis"
    embOpenai "github.com/cloudwego/eino-ext/components/embedding/openai"
)

// Create embedder
embedder, _ := embOpenai.NewEmbedder(ctx, &embOpenai.EmbeddingConfig{
    APIKey: "your-key",
    Model:  "text-embedding-3-small",
})

// Create Redis client
client := redis.NewClient(&redis.Options{
    Addr: "localhost:6379",
})

// Create indexer
indexer, err := redisIndexer.NewIndexer(ctx, &redisIndexer.IndexerConfig{
    Client:    client,
    KeyPrefix: "doc:",
    BatchSize: 10,
    Embedding: embedder,
})

// Store documents
docs := []*schema.Document{
    {
        ID:      "1",
        Content: "Eiffel Tower is located in Paris, France.",
        MetaData: map[string]any{"location": "France"},
    },
    {
        ID:      "2",
        Content: "The Great Wall is located in China.",
        MetaData: map[string]any{"location": "China"},
    },
}

ids, err := indexer.Store(ctx, docs)
// ids = ["1", "2"]
```

## Milvus 2.x Indexer Example

```go
import (
    "github.com/milvus-io/milvus/client/v2/milvusclient"
    milvusIndexer "github.com/cloudwego/eino-ext/components/indexer/milvus2"
)

milvusClient, _ := milvusclient.New(ctx, &milvusclient.ClientConfig{
    Address: "localhost:19530",
})

indexer, err := milvusIndexer.NewIndexer(ctx, &milvusIndexer.IndexerConfig{
    Client:     milvusClient,
    Collection: "my_collection",
    Embedding:  embedder,
})

ids, err := indexer.Store(ctx, docs)
```

## Elasticsearch 8 Indexer Example

```go
import (
    "github.com/elastic/go-elasticsearch/v8"
    esIndexer "github.com/cloudwego/eino-ext/components/indexer/es8"
)

esClient, _ := elasticsearch.NewTypedClient(elasticsearch.Config{
    Addresses: []string{"http://localhost:9200"},
})

indexer, err := esIndexer.NewIndexer(ctx, &esIndexer.IndexerConfig{
    Client:    esClient,
    Index:     "my_index",
    Embedding: embedder,
})

ids, err := indexer.Store(ctx, docs)
```

## Qdrant Indexer Example

```go
import (
    qdrantIndexer "github.com/cloudwego/eino-ext/components/indexer/qdrant"
)

indexer, err := qdrantIndexer.NewIndexer(ctx, &qdrantIndexer.IndexerConfig{
    Client:         qdrantClient,
    CollectionName: "my_collection",
    Embedding:      embedder,
})

ids, err := indexer.Store(ctx, docs)
```

## Full Indexing Pipeline

Load, parse, split, then index:

```go
import (
    "github.com/cloudwego/eino-ext/components/document/loader/file"
    "github.com/cloudwego/eino-ext/components/document/transformer/splitter/recursive"
    redisIndexer "github.com/cloudwego/eino-ext/components/indexer/redis"
)

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

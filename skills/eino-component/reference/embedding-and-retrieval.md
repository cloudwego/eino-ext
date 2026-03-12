# Embedding and Retrieval Reference

Embedders convert text to vectors; Retrievers find the most relevant documents by vector similarity.

## Embedder Interface

```go
// github.com/cloudwego/eino/components/embedding
type Embedder interface {
    EmbedStrings(ctx context.Context, texts []string, opts ...Option) ([][]float64, error)
}
```

Returns one vector per input text. Vector dimensions are fixed by the model (e.g., 1536 for ada-002).

## Retriever Interface

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

## Embedding Implementations

| Provider | Import Path | Key Config |
|----------|-------------|------------|
| OpenAI | `github.com/cloudwego/eino-ext/components/embedding/openai` | APIKey, Model (`text-embedding-3-small`) |
| Ark | `github.com/cloudwego/eino-ext/components/embedding/ark` | APIKey, Region, Model |
| Gemini | `github.com/cloudwego/eino-ext/components/embedding/gemini` | Client (*genai.Client), Model |
| DashScope | `github.com/cloudwego/eino-ext/components/embedding/dashscope` | APIKey, Model |
| Ollama | `github.com/cloudwego/eino-ext/components/embedding/ollama` | BaseURL, Model |
| Qianfan | `github.com/cloudwego/eino-ext/components/embedding/qianfan` | APIKey, SecretKey |
| TencentCloud | `github.com/cloudwego/eino-ext/components/embedding/tencentcloud` | SecretID, SecretKey |

### OpenAI Embedder Example

```go
import "github.com/cloudwego/eino-ext/components/embedding/openai"

embedder, err := openai.NewEmbedder(ctx, &openai.EmbeddingConfig{
    APIKey: "your-key",
    Model:  "text-embedding-3-small",
    // ByAzure: true,  // for Azure OpenAI
    // BaseURL: "https://{RESOURCE}.openai.azure.com",
})

vectors, err := embedder.EmbedStrings(ctx, []string{"hello world", "foo bar"})
// vectors[0] is the embedding for "hello world"
```

### Ark Embedder Example

```go
import "github.com/cloudwego/eino-ext/components/embedding/ark"

embedder, err := ark.NewEmbedder(ctx, &ark.EmbeddingConfig{
    APIKey: os.Getenv("ARK_API_KEY"),
    Region: os.Getenv("ARK_REGION"),
    Model:  os.Getenv("ARK_MODEL"),
})
```

### Ollama Embedder Example

```go
import "github.com/cloudwego/eino-ext/components/embedding/ollama"

embedder, err := ollama.NewEmbedder(ctx, &ollama.EmbeddingConfig{
    BaseURL: "http://localhost:11434",
    Model:   "nomic-embed-text",
})
```

## Retriever Implementations

| Backend | Import Path | Key Config |
|---------|-------------|------------|
| Redis | `github.com/cloudwego/eino-ext/components/retriever/redis` | Client, Index, VectorField, TopK |
| Milvus 2.x | `github.com/cloudwego/eino-ext/components/retriever/milvus2` | Client, Collection, SearchMode |
| Elasticsearch 8 | `github.com/cloudwego/eino-ext/components/retriever/es8` | Client, Index, SearchMode |
| Elasticsearch 9 | `github.com/cloudwego/eino-ext/components/retriever/es9` | Client, Index |
| Elasticsearch 7 | `github.com/cloudwego/eino-ext/components/retriever/es7` | Client, Index |
| Qdrant | `github.com/cloudwego/eino-ext/components/retriever/qdrant` | Client, CollectionName |
| Dify | `github.com/cloudwego/eino-ext/components/retriever/dify` | APIKey, DatasetID |

### Redis Retriever Example

```go
import (
    "github.com/redis/go-redis/v9"
    redisRetriever "github.com/cloudwego/eino-ext/components/retriever/redis"
)

// Redis client MUST use Protocol 2 for FT.SEARCH
client := redis.NewClient(&redis.Options{
    Addr:     "localhost:6379",
    Protocol: 2,
})
client.Options().UnstableResp3 = true

retriever, err := redisRetriever.NewRetriever(ctx, &redisRetriever.RetrieverConfig{
    Client:      client,
    Index:       "my_index",
    VectorField: "content_vector",
    TopK:        5,
    Embedding:   embedder,
})

docs, err := retriever.Retrieve(ctx, "what is eino?")
for _, doc := range docs {
    fmt.Println(doc.Content)
}
```

### Milvus 2.x Retriever Example

```go
import (
    "github.com/milvus-io/milvus/client/v2/milvusclient"
    milvus2 "github.com/cloudwego/eino-ext/components/retriever/milvus2"
    "github.com/cloudwego/eino-ext/components/retriever/milvus2/search_mode"
)

milvusClient, _ := milvusclient.New(ctx, &milvusclient.ClientConfig{
    Address:  "localhost:19530",
    Username: "user",
    Password: "pass",
})

retriever, err := milvus2.NewRetriever(ctx, &milvus2.RetrieverConfig{
    Client:         milvusClient,
    Collection:     "my_collection",
    Embedding:      embedder,
    SearchMode:     search_mode.NewApproximateSearchMode(10),
    DocumentParser: nil, // custom result-to-document parser
})
```

### Elasticsearch 8 Retriever Example

```go
import (
    "github.com/elastic/go-elasticsearch/v8"
    "github.com/cloudwego/eino-ext/components/retriever/es8"
    "github.com/cloudwego/eino-ext/components/retriever/es8/search_mode"
)

esClient, _ := elasticsearch.NewTypedClient(elasticsearch.Config{
    Addresses: []string{"http://localhost:9200"},
})

retriever, err := es8.NewRetriever(ctx, &es8.RetrieverConfig{
    Client:     esClient,
    Index:      "my_index",
    TopK:       5,
    Embedding:  embedder,
    SearchMode: search_mode.NewApproximateMode("content_vector"),
})
```

## RAG Retrieval Example

End-to-end: embed a query, search, and return documents.

```go
// The retriever handles embedding internally when Embedding is configured
docs, err := retriever.Retrieve(ctx, "How does Eino handle streaming?",
    retriever.WithTopK(5),
)
if err != nil {
    log.Fatal(err)
}

for _, doc := range docs {
    fmt.Printf("ID: %s\nContent: %s\nScore: %v\n\n",
        doc.ID, doc.Content, doc.MetaData["score"])
}
```

The same Embedder model must be used for indexing and retrieval. Mismatched models will produce incorrect similarity scores.

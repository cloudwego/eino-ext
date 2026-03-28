# Document Pipeline Reference

Loaders, parsers, and transformers form the document processing pipeline: load raw content, parse it, then split/transform for indexing.

## Interfaces

```go
// github.com/cloudwego/eino/components/document

type Loader interface {
    Load(ctx context.Context, src Source, opts ...LoaderOption) ([]*schema.Document, error)
}

type Transformer interface {
    Transform(ctx context.Context, src []*schema.Document, opts ...TransformerOption) ([]*schema.Document, error)
}

type Source struct {
    URI string  // file path or URL
}
```

Note: Parsers are typically used internally by loaders via configuration, not called directly.

## Loaders

| Loader | Import Path | Description |
|--------|-------------|-------------|
| File | `github.com/cloudwego/eino-ext/components/document/loader/file` | Load local files |
| S3 | `github.com/cloudwego/eino-ext/components/document/loader/s3` | Load from AWS S3 |
| URL | `github.com/cloudwego/eino-ext/components/document/loader/url` | Load from HTTP URLs |

### File Loader

```go
import "github.com/cloudwego/eino-ext/components/document/loader/file"

loader, err := file.NewFileLoader(ctx, &file.FileLoaderConfig{
    UseNameAsID: true,
})

docs, err := loader.Load(ctx, document.Source{URI: "/path/to/file.txt"})
```

### URL Loader

```go
import "github.com/cloudwego/eino-ext/components/document/loader/url"

loader, err := url.NewLoader(ctx, &url.LoaderConfig{})
docs, err := loader.Load(ctx, document.Source{URI: "https://example.com/page"})
```

### S3 Loader

```go
import "github.com/cloudwego/eino-ext/components/document/loader/s3"

loader, err := s3.NewLoader(ctx, &s3.LoaderConfig{
    Region: "us-east-1",
    Bucket: "my-bucket",
})
docs, err := loader.Load(ctx, document.Source{URI: "s3://my-bucket/file.pdf"})
```

## Parsers

Parsers convert raw file content into structured documents. They are typically configured on loaders.

| Parser | Import Path | Formats |
|--------|-------------|---------|
| HTML | `github.com/cloudwego/eino-ext/components/document/parser/html` | HTML to text, extracts metadata |
| PDF | `github.com/cloudwego/eino-ext/components/document/parser/pdf` | PDF text extraction |
| DOCX | `github.com/cloudwego/eino-ext/components/document/parser/docx` | Word documents |
| XLSX | `github.com/cloudwego/eino-ext/components/document/parser/xlsx` | Excel spreadsheets |

### HTML Parser

```go
import "github.com/cloudwego/eino-ext/components/document/parser/html"

parser, err := html.NewParser(ctx, &html.Config{
    Selector: "article",  // Optional CSS selector
})
```

### PDF Parser

```go
import "github.com/cloudwego/eino-ext/components/document/parser/pdf"

parser, err := pdf.NewParser(ctx, &pdf.Config{})
```

## Transformers

Transformers operate on document slices: split, filter, merge, or re-rank.

### Splitters

| Splitter | Import Path | Description |
|----------|-------------|-------------|
| Recursive | `github.com/cloudwego/eino-ext/components/document/transformer/splitter/recursive` | Split by chunk size with overlap |
| Markdown | `github.com/cloudwego/eino-ext/components/document/transformer/splitter/markdown` | Split by markdown headers |
| HTML | `github.com/cloudwego/eino-ext/components/document/transformer/splitter/html` | Split by HTML structure |
| Semantic | `github.com/cloudwego/eino-ext/components/document/transformer/splitter/semantic` | Split by semantic similarity |

#### Recursive Splitter

```go
import "github.com/cloudwego/eino-ext/components/document/transformer/splitter/recursive"

splitter, err := recursive.NewSplitter(ctx, &recursive.Config{
    ChunkSize:   1500,  // max characters per chunk
    OverlapSize: 300,   // overlap from previous chunk for context
})

chunks, err := splitter.Transform(ctx, docs)
```

#### Markdown Splitter

```go
import "github.com/cloudwego/eino-ext/components/document/transformer/splitter/markdown"

splitter, err := markdown.NewHeaderSplitter(ctx, &markdown.HeaderConfig{
    Headers: []markdown.HeaderLevel{
        {Level: 1, Name: "h1"},
        {Level: 2, Name: "h2"},
    },
})
chunks, err := splitter.Transform(ctx, docs)
```

### Reranker

| Reranker | Import Path | Description |
|----------|-------------|-------------|
| Score | `github.com/cloudwego/eino-ext/components/document/transformer/reranker/score` | Rerank by score metadata |

## Full Document Pipeline

Load, parse, split, and index documents end-to-end:

```go
import (
    "github.com/cloudwego/eino/components/document"
    "github.com/cloudwego/eino/schema"

    "github.com/cloudwego/eino-ext/components/document/loader/file"
    "github.com/cloudwego/eino-ext/components/document/transformer/splitter/recursive"
    embOpenai "github.com/cloudwego/eino-ext/components/embedding/openai"
    redisIndexer "github.com/cloudwego/eino-ext/components/indexer/redis"
)

ctx := context.Background()

// 1. Load
loader, _ := file.NewFileLoader(ctx, &file.FileLoaderConfig{
    UseNameAsID: true,
})
docs, _ := loader.Load(ctx, document.Source{URI: "/data/knowledge.txt"})

// 2. Split
splitter, _ := recursive.NewSplitter(ctx, &recursive.Config{
    ChunkSize:   1500,
    OverlapSize: 300,
})
chunks, _ := splitter.Transform(ctx, docs)

// 3. Create embedder
embedder, _ := embOpenai.NewEmbedder(ctx, &embOpenai.EmbeddingConfig{
    APIKey: os.Getenv("OPENAI_API_KEY"),
    Model:  "text-embedding-3-small",
})

// 4. Index
indexer, _ := redisIndexer.NewIndexer(ctx, &redisIndexer.IndexerConfig{
    Client:    redisClient,
    KeyPrefix: "doc:",
    BatchSize: 10,
    Embedding: embedder,
})
ids, _ := indexer.Store(ctx, chunks)
fmt.Printf("Indexed %d chunks\n", len(ids))
```

For the Compose-based pipeline approach (using Graph), see the eino-compose skill.

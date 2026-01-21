# ES8 Retriever

English | [中文](README_zh.md)

An Elasticsearch 8.x retriever implementation for [Eino](https://github.com/cloudwego/eino) that implements the `Retriever` interface. This enables seamless integration with Eino's vector retrieval system for enhanced semantic search capabilities.

## Features

- Implements `github.com/cloudwego/eino/components/retriever.Retriever`
- Easy integration with Eino's retrieval system
- Configurable Elasticsearch parameters
- Support for vector similarity search
- Multiple search modes including approximate search
- Custom result parsing support
- Flexible document filtering

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/retriever/es8@latest
```

## Quick Start

Here's a quick example of how to use the retriever with approximate search mode, you could read components/retriever/es8/examples/approximate/approximate.go for more details:

```go
import (
	"context"
	"encoding/json"
	"os"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

	"github.com/cloudwego/eino-ext/components/embedding/ark"
	"github.com/cloudwego/eino-ext/components/retriever/es8"
	"github.com/cloudwego/eino-ext/components/retriever/es8/search_mode"
)

const (
	indexName          = "eino_example"
	fieldContent       = "content"
	fieldContentVector = "content_vector"
	fieldExtraLocation = "location"
	docExtraLocation   = "location"
)

func main() {
	ctx := context.Background()
	// es supports multiple ways to connect
	username := os.Getenv("ES_USERNAME")
	password := os.Getenv("ES_PASSWORD")

	// 1. Create ES client
	httpCACertPath := os.Getenv("ES_HTTP_CA_CERT_PATH")
	if httpCACertPath != "" {
		cert, err := os.ReadFile(httpCACertPath)
		if err != nil {
			log.Fatalf("read file failed, err=%v", err)
		}
	}

	client, _ := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{"https://localhost:9200"},
		Username:  username,
		Password:  password,
		CACert:    cert,
	})

	// 2. Create embedding component using ARK
	// Replace "ARK_API_KEY", "ARK_REGION", "ARK_MODEL" with your actual config
	emb, _ := ark.NewEmbedder(ctx, &ark.EmbeddingConfig{
		APIKey: os.Getenv("ARK_API_KEY"),
		Region: os.Getenv("ARK_REGION"),
		Model:  os.Getenv("ARK_MODEL"),
	})

	// 3. Create ES retriever component
	retriever, _ := es8.NewRetriever(ctx, &es8.RetrieverConfig{
		Client: client,
		Index:  indexName,
		TopK:   5, // Retrieve top 5 results
		// Use approximate search mode (vector search)
		SearchMode: search_mode.SearchModeApproximate(&search_mode.ApproximateConfig{
			QueryFieldName:  fieldContent,
			VectorFieldName: fieldContentVector,
			Hybrid:          true, // Enable hybrid search (vector + keyword)
			// RRF only available with specific licenses
			// see: https://www.elastic.co/subscriptions
			RRF:             false,
			RRFRankConstant: nil,
			RRFWindowSize:   nil,
		}),
		// ResultParser extracts document fields from ES hit
		ResultParser: func(ctx context.Context, hit types.Hit) (doc *schema.Document, err error) {
			doc = &schema.Document{
				ID:       *hit.Id_,
				Content:  "",
				MetaData: map[string]any{},
			}

			var src map[string]any
			if err = json.Unmarshal(hit.Source_, &src); err != nil {
				return nil, err
			}

			for field, val := range src {
				switch field {
				case fieldContent:
					doc.Content = val.(string)
				case fieldContentVector:
					var v []float64
					for _, item := range val.([]interface{}) {
						v = append(v, item.(float64))
					}
					doc.WithDenseVector(v)
				case fieldExtraLocation:
					doc.MetaData[docExtraLocation] = val.(string)
				}
			}

			if hit.Score_ != nil {
				doc.WithScore(float64(*hit.Score_))
			}

			return doc, nil
		},
		Embedding: emb, // your embedding component
	})

	// search without filter
	docs, _ := retriever.Retrieve(ctx, "tourist attraction")

	// search with filter
	caseInsensitive := true
	docs, _ = retriever.Retrieve(ctx, "tourist attraction",
		es8.WithFilters([]types.Query{{
			Term: map[string]types.TermQuery{
				fieldExtraLocation: {
					CaseInsensitive: &caseInsensitive,
					Value:           "China",
				},
			},
		}}),
	)
}
```

## Configuration

The retriever can be configured using the `RetrieverConfig` struct:

```go
type RetrieverConfig struct {
    Client *elasticsearch.Client // Required: Elasticsearch client instance
    Index  string               // Required: Index name to retrieve documents from
    TopK   int                  // Required: Number of results to return

    // Required: Search mode configuration
    SearchMode search_mode.SearchMode

    // Optional: Function to parse Elasticsearch hits into Documents
    // If not provided, default parser will be used which:
    // 1. Extracts "content" field from source as Document.Content
    // 2. Used other source fields as Document.MetaData
    ResultParser func(ctx context.Context, hit types.Hit) (*schema.Document, error)

    // Optional: Required only if query vectorization is needed
    Embedding embedding.Embedder
}
```

## Full Examples

- [Approximate Search Example](./examples/approximate)
- [Dense Vector Similarity Example](./examples/dense_vector_similarity)
- [Exact Match Example](./examples/exact_match)
- [Raw String Request Example](./examples/raw_string)
- [Sparse Vector Query Example](./examples/sparse_vector_query)

## For More Details

- [Eino Documentation](https://www.cloudwego.io/zh/docs/eino/)
- [Elasticsearch Go Client Documentation](https://github.com/elastic/go-elasticsearch)

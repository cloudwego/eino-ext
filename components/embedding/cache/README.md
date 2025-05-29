# Cache Embedder for Eino

This module provides a cache embedder for Eino, which is designed to store and retrieve embeddings efficiently. The cache embedder can be used to speed up the embedding process by caching previously computed embeddings.

## Installation

```shell
go get github.com/cloudwego/eino-ext/components/embedding/cache
```

## Usage


```go
package main

import (
	"context"
	"crypto/md5"
	"log"

	"github.com/cloudwego/eino-ext/components/embedding/cache"
	cacheredis "github.com/cloudwego/eino-ext/components/embedding/cache/redis"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/redis/go-redis/v9"
)

func main() {
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	// the original embedder, you can replace it with any other embedder implementation
	// It's only a example, you need to bring a real embedder implementation here.
	var originalEmbedder embedding.Embedder
	// embedder, err := openai.NewEmbedder(ctx, &openai.EmbeddingConfig{
	// 	APIKey:     accessKey,
	// }
	// ...

	embedder, err := cache.NewEmbedder(originalEmbedder,
		cache.WithCacher(cacheredis.NewCacher(rdb)),            // using Redis as the cache
		cache.WithGenerator(cache.NewHashGenerator(md5.New())), // using md5 for generating unique keys
	)
	if err != nil {
		log.Fatal(err)
	}

	embeddings, err := embedder.EmbedStrings(context.Background(), []string{"hello", "how are you"})
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("embeddings: %v", embeddings)
}
```

## Features

- **Cache**: The cache embedder stores embeddings in a cache to avoid recomputing them for the same input.
- **Cacher**: The cache embedder supports different caching backends, such as Redis.
  - Currently, [Redis](./redis) is supported.
- **Generator**: The cache embedder uses a generator to create unique keys for caching embeddings.
  - Currently, a simple generator and a hash generator base on hash.Hash interface are supported.
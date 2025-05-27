package main

import (
	"context"
	"crypto/md5"
	"log"

	"github.com/cloudwego/eino-ext/components/embedding/cached"
	cachedredis "github.com/cloudwego/eino-ext/components/embedding/cached/redis"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/redis/go-redis/v9"
)

func main() {
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	// the original embedder
	var originalEmbedder embedding.Embedder
	// embedder, err := openai.NewEmbedder(ctx, &openai.EmbeddingConfig{
	// 	APIKey:     accessKey,
	// }
	// ...

	embedder := cached.NewEmbedder(originalEmbedder,
		cached.WithCacher(cachedredis.NewCacher(rdb)),            // using Redis as the cache
		cached.WithGenerator(cached.NewHashGenerator(md5.New())), // using md5 for generating unique keys
	)

	embeddings, err := embedder.EmbedStrings(context.Background(), []string{"hello", "how are you"})
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("embeddings: %v", embeddings)
}

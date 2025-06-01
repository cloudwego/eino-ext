/*
 * Copyright 2024 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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

	// the original embedder
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

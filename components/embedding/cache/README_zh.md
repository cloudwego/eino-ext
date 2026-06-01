# Eino 缓存嵌入器

此模块为 Eino 提供了一个缓存嵌入器，旨在高效地存储和检索嵌入。缓存嵌入器可以通过缓存先前计算的嵌入来加速嵌入过程。

## 安装

```shell
go get github.com/cloudwego/eino-ext/components/embedding/cache
```

## 使用方法

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

	// 原始嵌入器，你可以用任何其他嵌入器实现替换它
	// 这只是一个示例，你需要在这里提供一个真实的嵌入器实现
	var originalEmbedder embedding.Embedder
	// embedder, err := openai.NewEmbedder(ctx, &openai.EmbeddingConfig{
	// 	APIKey:     accessKey,
	// }
	// ...

	embedder, err := cache.NewEmbedder(originalEmbedder,
		cache.WithCacher(cacheredis.NewCacher(rdb)),            // 使用 Redis 作为缓存
		cache.WithGenerator(cache.NewHashGenerator(md5.New())), // 使用 md5 生成唯一键
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

## 功能特性

- **缓存**：缓存嵌入器将嵌入存储在缓存中，以避免对相同输入重新计算。
- **缓存器**：缓存嵌入器支持不同的缓存后端，例如 Redis。
  - 目前支持 [Redis](./redis)。
- **生成器**：缓存嵌入器使用生成器创建用于缓存嵌入的唯一键。
  - 目前支持基于 hash.Hash 接口的简单生成器和哈希生成器。

## 示例

查看 [examples](./examples/) 目录获取完整的使用示例。

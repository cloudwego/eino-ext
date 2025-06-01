# Redis Cacher for cache embedder

This directory contains the implementation of a Redis cacher for the cache embedder.

## Installation

```shell
go get github.com/cloudwego/eino-ext/components/embedding/cache/redis
```

## Usage

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	cacheredis "github.com/cloudwego/eino-ext/components/embedding/cache/redis"
)

func main() {
	ctx := context.Background()
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	cacher := cacheredis.NewCacher(rdb,
		cacheredis.WithPrefix("eino:embedding:"),
	)

	if err := cacher.Set(ctx, "example_key", []float64{1.0, 2.0, 3.0}, time.Second*10); err != nil {
		panic(err)
	}

	value, found, err := cacher.Get(ctx, "example_key")
	if err != nil {
		panic(err)
	}
	fmt.Println("value:", value, "found:", found)
}
```
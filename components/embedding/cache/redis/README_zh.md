# 缓存嵌入器的 Redis 缓存器

此目录包含用于缓存嵌入器的 Redis 缓存器实现。

## 安装

```shell
go get github.com/cloudwego/eino-ext/components/embedding/cache/redis
```

## 使用方法

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

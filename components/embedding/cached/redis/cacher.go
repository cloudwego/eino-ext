package redis

import (
	"context"
	"errors"
	"time"

	"github.com/cloudwego/eino-ext/components/embedding/cached"
	"github.com/redis/go-redis/v9"
)

type Cacher struct {
	rdb   redis.UniversalClient
	codec codec
}

var _ cached.Cacher = (*Cacher)(nil)

func NewCacher(rdb redis.UniversalClient) *Cacher {
	return &Cacher{
		rdb:   rdb,
		codec: defaultCodec,
	}
}

func (c Cacher) Set(ctx context.Context, key string, value []float64, expire time.Duration) error {
	data, err := c.codec.Marshal(value)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, key, data, expire).Err()
}

func (c Cacher) Get(ctx context.Context, key string) ([]float64, error) {
	data, err := c.rdb.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, cached.ErrNotFound
		}
		return nil, err
	}

	var value []float64
	return value, c.codec.Unmarshal(data, &value)
}

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

package redis

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/components/embedding/cache"
	"github.com/redis/go-redis/v9"
)

type Cacher struct {
	rdb    redis.UniversalClient
	prefix string
	codec  codec
}

type Option interface {
	apply(*Cacher)
}

type optionFunc func(*Cacher)

func (f optionFunc) apply(c *Cacher) {
	f(c)
}

func WithPrefix(prefix string) Option {
	return optionFunc(func(c *Cacher) {
		c.prefix = strings.TrimSuffix(prefix, ":") + ":"
	})
}

var _ cache.Cacher = (*Cacher)(nil)

func NewCacher(rdb redis.UniversalClient, opts ...Option) *Cacher {
	cacher := &Cacher{
		rdb:    rdb,
		prefix: "eino:",
		codec:  defaultCodec,
	}
	for _, opt := range opts {
		opt.apply(cacher)
	}
	return cacher
}

func (c *Cacher) Set(ctx context.Context, key string, value []float64, expire time.Duration) error {
	data, err := c.codec.Marshal(value)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, c.prefix+key, data, expire).Err()
}

func (c *Cacher) Get(ctx context.Context, key string) ([]float64, bool, error) {
	data, err := c.rdb.Get(ctx, c.prefix+key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, false, nil
		}
		return nil, false, err
	}

	var value []float64
	if err := c.codec.Unmarshal(data, &value); err != nil {
		return nil, false, err
	}
	return value, true, nil
}

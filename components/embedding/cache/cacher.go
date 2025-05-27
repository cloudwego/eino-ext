package cache

import (
	"context"
	"errors"
	"time"
)

var ErrNotFound = errors.New("the key is not found")

type Cacher interface {
	// Set stores the value in the cache with the given key.
	// If the key already exists, it will be overwritten.
	Set(ctx context.Context, key string, value []float64, expire time.Duration) error

	// Get retrieves the value from the cache with the given key.
	// If the key does not exist, it returns ErrNotFound.
	// If the value is not of type []float64, it returns an error.
	Get(ctx context.Context, key string) ([]float64, error)
}

type noCacher struct{}

var _ Cacher = (*noCacher)(nil)

func (n noCacher) Set(context.Context, string, []float64, time.Duration) error {
	return nil
}

func (n noCacher) Get(context.Context, string) ([]float64, error) {
	return nil, ErrNotFound
}

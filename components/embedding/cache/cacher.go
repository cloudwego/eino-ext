package cache

import (
	"context"
	"time"
)

type Cacher interface {
	// Set stores the value in the cache with the given key.
	// If the key already exists, it will be overwritten.
	Set(ctx context.Context, key string, value []float64, expire time.Duration) error

	// Get retrieves the value from the cache with the given key.
	// If the key does not exist, the bool return value is false，otherwise it returns true
	// If the value is not of type []float64, it returns an error.
	Get(ctx context.Context, key string) ([]float64, bool, error)
}

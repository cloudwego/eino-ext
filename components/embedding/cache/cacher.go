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

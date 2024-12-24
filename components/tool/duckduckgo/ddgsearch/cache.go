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

package ddgsearch

import (
	"sync"
	"time"
)

// cache implements a simple in-memory cache with expiration
type cache struct {
	mu     sync.RWMutex
	items  map[string]*cacheItem
	maxAge time.Duration
}

type cacheItem struct {
	value      interface{}
	expiration time.Time
}

func newCache(maxAge time.Duration) *cache {
	return &cache{
		items:  make(map[string]*cacheItem),
		maxAge: maxAge,
	}
}

func (c *cache) get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists {
		return nil, false
	}

	if time.Now().After(item.expiration) {
		delete(c.items, key)
		return nil, false
	}

	return item.value, true
}

func (c *cache) set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = &cacheItem{
		value:      value,
		expiration: time.Now().Add(c.maxAge),
	}
}

func (c *cache) delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}

func (c *cache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]*cacheItem)
}

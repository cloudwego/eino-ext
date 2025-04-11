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
	"runtime"
	"sync"
	"time"
)

// janitor is a background task that cleans up expired Cache items
type janitor struct {
	interval time.Duration
	stop     chan struct{}
}

// Run starts the janitor in a new goroutine
func (j *janitor) Run(c *cache) {
	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.deleteExpired()
		case <-j.stop:
			ticker.Stop()
			return
		}
	}
}

// stopJanitor stops the janitor
func stopJanitor(c *cache) {
	c.janitor.stop <- struct{}{}
}

// newJanitor creates a new janitor with the specified interval
func newJanitor(interval time.Duration) *janitor {
	return &janitor{
		interval: interval,
		stop:     make(chan struct{}),
	}
}

// cache implements a simple in-memory cache with expiration
type cache struct {
	mu      sync.RWMutex
	items   map[string]*cacheItem
	maxAge  time.Duration
	janitor *janitor
}

type cacheItem struct {
	value      interface{}
	expiration time.Time
}

func newCache(maxAge time.Duration) *cache {
	c := &cache{
		items:   make(map[string]*cacheItem),
		maxAge:  maxAge,
		janitor: newJanitor(maxAge),
	}

	go c.janitor.Run(c)
	runtime.SetFinalizer(c, stopJanitor)

	return c
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

func (c *cache) deleteExpired() {
	now := time.Now()
	expiredKeys := make([]string, 0)

	c.mu.RLock() // add read lock extract expired key
	for k, v := range c.items {
		if now.After(v.expiration) {
			expiredKeys = append(expiredKeys, k)
		}
	}
	c.mu.RUnlock()

	c.mu.Lock() // add write locks Delete expired keys
	defer c.mu.Unlock()
	for _, k := range expiredKeys {
		delete(c.items, k)
	}
}

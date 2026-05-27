/*
 * Copyright 2026 CloudWeGo Authors
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
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/eino/adk/middlewares/automemory"
	goredis "github.com/redis/go-redis/v9"
)

const defaultPrefix = "eino:automemory:"

var errLockTokenMismatch = errors.New("lock token mismatch")

type Config struct {
	// Client is a Redis client representing a pool of zero or more underlying connections.
	// It is safe for concurrent use by multiple goroutines.
	Client goredis.UniversalClient

	// Prefix namespaces all keys managed by the coordinator.
	// Default: "eino:automemory:".
	Prefix string
}

type Coordinator struct {
	client goredis.UniversalClient
	prefix string
}

var _ automemory.Coordinator = (*Coordinator)(nil)

func NewCoordinator(config *Config) (*Coordinator, error) {
	if config == nil {
		return nil, fmt.Errorf("redis coordinator config is required")
	}
	if config.Client == nil {
		return nil, fmt.Errorf("redis coordinator client is required")
	}

	return &Coordinator{
		client: config.Client,
		prefix: normalizePrefix(config.Prefix, defaultPrefix),
	}, nil
}

func (c *Coordinator) AcquireLock(ctx context.Context, sessionID string, ttl time.Duration) (func(context.Context) error, bool, error) {
	token := randToken()
	ok, err := c.client.SetNX(ctx, c.lockKey(sessionID), token, normalizeTTL(ttl)).Result()
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, nil
	}

	return func(unlockCtx context.Context) error {
		return compareAndDelete(unlockCtx, c.client, c.lockKey(sessionID), token)
	}, true, nil
}

func (c *Coordinator) PopPendingSnapshot(ctx context.Context, sessionID string) (*automemory.PendingSnapshot, error) {
	key := c.pendingKey(sessionID)

	for i := 0; i < 16; i++ {
		var payload string
		err := c.client.Watch(ctx, func(tx *goredis.Tx) error {
			value, err := tx.Get(ctx, key).Result()
			if errors.Is(err, goredis.Nil) {
				payload = ""
				return nil
			}
			if err != nil {
				return err
			}

			payload = value
			_, err = tx.TxPipelined(ctx, func(pipe goredis.Pipeliner) error {
				pipe.Del(ctx, key)
				return nil
			})
			return err
		}, key)
		if errors.Is(err, goredis.TxFailedErr) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if payload == "" {
			return nil, nil
		}

		var snapshot automemory.PendingSnapshot
		if err := json.Unmarshal([]byte(payload), &snapshot); err != nil {
			return nil, err
		}
		return &snapshot, nil
	}

	return nil, fmt.Errorf("pop pending snapshot: transaction conflict")
}

func (c *Coordinator) SetPendingSnapshot(ctx context.Context, sessionID string, snapshot *automemory.PendingSnapshot) error {
	key := c.pendingKey(sessionID)
	if snapshot == nil {
		return c.client.Del(ctx, key).Err()
	}

	payload, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}

	return c.client.Set(ctx, key, payload, 0).Err()
}

func (c *Coordinator) GetCursor(ctx context.Context, sessionID string) (int, bool, error) {
	value, err := c.client.Get(ctx, c.cursorKey(sessionID)).Result()
	if errors.Is(err, goredis.Nil) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}

	cursor, err := strconv.Atoi(value)
	if err != nil {
		return 0, false, err
	}
	return cursor, true, nil
}

func (c *Coordinator) SetCursor(ctx context.Context, sessionID string, cursor int) error {
	return c.client.Set(ctx, c.cursorKey(sessionID), strconv.Itoa(cursor), 0).Err()
}

func (c *Coordinator) lockKey(sessionID string) string {
	return c.prefix + "lock:" + sessionID
}

func (c *Coordinator) pendingKey(sessionID string) string {
	return c.prefix + "pending:" + sessionID
}

func (c *Coordinator) cursorKey(sessionID string) string {
	return c.prefix + "cursor:" + sessionID
}

func normalizePrefix(prefix, fallback string) string {
	if prefix == "" {
		prefix = fallback
	}
	return strings.TrimSuffix(prefix, ":") + ":"
}

func normalizeTTL(ttl time.Duration) time.Duration {
	if ttl > 0 {
		return ttl
	}
	return time.Millisecond
}

func compareAndDelete(ctx context.Context, client goredis.UniversalClient, key, expected string) error {
	for i := 0; i < 16; i++ {
		err := client.Watch(ctx, func(tx *goredis.Tx) error {
			value, err := tx.Get(ctx, key).Result()
			if errors.Is(err, goredis.Nil) {
				return nil
			}
			if err != nil {
				return err
			}
			if value != expected {
				return errLockTokenMismatch
			}

			_, err = tx.TxPipelined(ctx, func(pipe goredis.Pipeliner) error {
				pipe.Del(ctx, key)
				return nil
			})
			return err
		}, key)
		if errors.Is(err, goredis.TxFailedErr) {
			continue
		}
		return err
	}

	return fmt.Errorf("delete lock: transaction conflict")
}

func randToken() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

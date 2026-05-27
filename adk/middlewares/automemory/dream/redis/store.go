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
	"sort"
	"strings"
	"time"

	"github.com/cloudwego/eino/adk/middlewares/automemory/dream"
	goredis "github.com/redis/go-redis/v9"
)

const defaultPrefix = "eino:automemory:dream:"

var errLockTokenMismatch = errors.New("lock token mismatch")

type Config struct {
	// Client is a Redis client representing a pool of zero or more underlying connections.
	// It is safe for concurrent use by multiple goroutines.
	Client goredis.UniversalClient

	// Prefix namespaces all keys managed by the store.
	// Default: "eino:automemory:dream:".
	Prefix string
}

type Store struct {
	client goredis.UniversalClient
	prefix string
}

var _ dream.Store = (*Store)(nil)

func NewStore(config *Config) (*Store, error) {
	if config == nil {
		return nil, fmt.Errorf("redis dream store config is required")
	}
	if config.Client == nil {
		return nil, fmt.Errorf("redis dream store client is required")
	}

	return &Store{
		client: config.Client,
		prefix: normalizePrefix(config.Prefix, defaultPrefix),
	}, nil
}

func (s *Store) RecordSessionTouch(ctx context.Context, memoryDir, sessionID string, at time.Time) error {
	if err := s.client.ZAdd(ctx, s.touchKey(memoryDir), goredis.Z{
		Score:  float64(at.UnixNano()),
		Member: sessionID,
	}).Err(); err != nil {
		return err
	}

	state, err := s.GetScheduleState(ctx, memoryDir)
	if err != nil {
		return err
	}
	if state.NextCheckAt.IsZero() {
		state.NextCheckAt = at
		return s.SetScheduleState(ctx, memoryDir, state)
	}
	return nil
}

func (s *Store) ListSessionsTouchedSince(ctx context.Context, memoryDir string, since time.Time) ([]string, error) {
	values, err := s.client.ZRangeByScore(ctx, s.touchKey(memoryDir), &goredis.ZRangeBy{
		Min: fmt.Sprintf("(%d", since.UnixNano()),
		Max: "+inf",
	}).Result()
	if err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, nil
	}

	sessions := append([]string(nil), values...)
	sort.Strings(sessions)
	return sessions, nil
}

func (s *Store) GetScheduleState(ctx context.Context, memoryDir string) (*dream.ScheduleState, error) {
	value, err := s.client.Get(ctx, s.stateKey(memoryDir)).Result()
	if errors.Is(err, goredis.Nil) {
		return &dream.ScheduleState{}, nil
	}
	if err != nil {
		return nil, err
	}

	var state dream.ScheduleState
	if err := json.Unmarshal([]byte(value), &state); err != nil {
		return nil, err
	}
	return &state, nil
}

func (s *Store) SetScheduleState(ctx context.Context, memoryDir string, state *dream.ScheduleState) error {
	key := s.stateKey(memoryDir)
	if state == nil {
		return s.client.Del(ctx, key).Err()
	}

	payload, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, key, payload, 0).Err()
}

func (s *Store) AcquireRunLock(ctx context.Context, memoryDir string, ttl time.Duration) (func(context.Context) error, bool, error) {
	token := randToken()
	ok, err := s.client.SetNX(ctx, s.lockKey(memoryDir), token, normalizeTTL(ttl)).Result()
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, nil
	}

	return func(unlockCtx context.Context) error {
		return compareAndDelete(unlockCtx, s.client, s.lockKey(memoryDir), token)
	}, true, nil
}

func (s *Store) touchKey(memoryDir string) string {
	return s.prefix + "touch:" + memoryDir
}

func (s *Store) stateKey(memoryDir string) string {
	return s.prefix + "state:" + memoryDir
}

func (s *Store) lockKey(memoryDir string) string {
	return s.prefix + "lock:" + memoryDir
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

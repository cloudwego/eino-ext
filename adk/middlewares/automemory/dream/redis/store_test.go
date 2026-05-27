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
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/cloudwego/eino/adk/middlewares/automemory/dream"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestStoreRecordTouchAndListSessions(t *testing.T) {
	ctx := context.Background()
	mr := miniredis.RunT(t)
	store := newTestStore(t, mr)

	base := time.Unix(1710000000, 0)
	require.NoError(t, store.RecordSessionTouch(ctx, "/mem", "session-b", base.Add(2*time.Minute)))
	require.NoError(t, store.RecordSessionTouch(ctx, "/mem", "session-a", base.Add(3*time.Minute)))
	require.NoError(t, store.RecordSessionTouch(ctx, "/mem", "session-a", base.Add(4*time.Minute)))

	sessions, err := store.ListSessionsTouchedSince(ctx, "/mem", base.Add(time.Minute))
	require.NoError(t, err)
	require.Equal(t, []string{"session-a", "session-b"}, sessions)

	state, err := store.GetScheduleState(ctx, "/mem")
	require.NoError(t, err)
	require.Equal(t, base.Add(2*time.Minute), state.NextCheckAt)
}

func TestStoreScheduleStateLifecycle(t *testing.T) {
	ctx := context.Background()
	mr := miniredis.RunT(t)
	store := newTestStore(t, mr)

	state := &dream.ScheduleState{
		LastConsolidatedAt: time.Unix(1710000000, 0),
		NextCheckAt:        time.Unix(1710003600, 0),
	}
	require.NoError(t, store.SetScheduleState(ctx, "/mem", state))

	got, err := store.GetScheduleState(ctx, "/mem")
	require.NoError(t, err)
	require.Equal(t, *state, *got)

	require.NoError(t, store.SetScheduleState(ctx, "/mem", nil))

	got, err = store.GetScheduleState(ctx, "/mem")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.True(t, got.LastConsolidatedAt.IsZero())
	require.True(t, got.NextCheckAt.IsZero())
}

func TestStoreRunLockLifecycle(t *testing.T) {
	ctx := context.Background()
	mr := miniredis.RunT(t)
	store := newTestStore(t, mr)

	unlock, ok, err := store.AcquireRunLock(ctx, "/mem", time.Second)
	require.NoError(t, err)
	require.True(t, ok)

	unlock2, ok, err := store.AcquireRunLock(ctx, "/mem", time.Minute)
	require.NoError(t, err)
	require.False(t, ok)
	require.Nil(t, unlock2)

	mr.FastForward(2 * time.Second)

	unlock3, ok, err := store.AcquireRunLock(ctx, "/mem", time.Minute)
	require.NoError(t, err)
	require.True(t, ok)

	require.EqualError(t, unlock(ctx), errLockTokenMismatch.Error())
	require.NoError(t, unlock3(ctx))
}

func newTestStore(t *testing.T, mr *miniredis.Miniredis) *Store {
	t.Helper()

	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
	})

	store, err := NewStore(&Config{Client: client})
	require.NoError(t, err)
	return store
}

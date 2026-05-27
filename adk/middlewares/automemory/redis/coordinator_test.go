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
	"github.com/cloudwego/eino/adk/middlewares/automemory"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestCoordinatorLockLifecycle(t *testing.T) {
	ctx := context.Background()
	mr := miniredis.RunT(t)
	coord := newTestCoordinator(t, mr)

	unlock, ok, err := coord.AcquireLock(ctx, "session-a", time.Minute)
	require.NoError(t, err)
	require.True(t, ok)

	unlock2, ok, err := coord.AcquireLock(ctx, "session-a", time.Minute)
	require.NoError(t, err)
	require.False(t, ok)
	require.Nil(t, unlock2)

	require.NoError(t, unlock(ctx))

	unlock3, ok, err := coord.AcquireLock(ctx, "session-a", time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	require.NoError(t, unlock3(ctx))
}

func TestCoordinatorUnlockDoesNotDeleteReacquiredLock(t *testing.T) {
	ctx := context.Background()
	mr := miniredis.RunT(t)
	coord := newTestCoordinator(t, mr)

	unlock, ok, err := coord.AcquireLock(ctx, "session-a", time.Second)
	require.NoError(t, err)
	require.True(t, ok)

	mr.FastForward(2 * time.Second)

	unlock2, ok, err := coord.AcquireLock(ctx, "session-a", time.Minute)
	require.NoError(t, err)
	require.True(t, ok)

	require.EqualError(t, unlock(ctx), errLockTokenMismatch.Error())

	_, ok, err = coord.AcquireLock(ctx, "session-a", time.Minute)
	require.NoError(t, err)
	require.False(t, ok)

	require.NoError(t, unlock2(ctx))
}

func TestCoordinatorPendingSnapshotLifecycle(t *testing.T) {
	ctx := context.Background()
	mr := miniredis.RunT(t)
	coord := newTestCoordinator(t, mr)

	snapshot := &automemory.PendingSnapshot{
		Cursor:    7,
		Messages:  []byte(`[{"role":"user","content":"remember this"}]`),
		ToolInfos: []byte(`[{"name":"write_memory"}]`),
	}
	require.NoError(t, coord.SetPendingSnapshot(ctx, "session-a", snapshot))

	got, err := coord.PopPendingSnapshot(ctx, "session-a")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, snapshot.Cursor, got.Cursor)
	require.JSONEq(t, string(snapshot.Messages), string(got.Messages))
	require.JSONEq(t, string(snapshot.ToolInfos), string(got.ToolInfos))

	got, err = coord.PopPendingSnapshot(ctx, "session-a")
	require.NoError(t, err)
	require.Nil(t, got)
}

func TestCoordinatorCursorLifecycle(t *testing.T) {
	ctx := context.Background()
	mr := miniredis.RunT(t)
	coord := newTestCoordinator(t, mr)

	cursor, ok, err := coord.GetCursor(ctx, "session-a")
	require.NoError(t, err)
	require.False(t, ok)
	require.Zero(t, cursor)

	require.NoError(t, coord.SetCursor(ctx, "session-a", 11))

	cursor, ok, err = coord.GetCursor(ctx, "session-a")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, 11, cursor)
}

func newTestCoordinator(t *testing.T, mr *miniredis.Miniredis) *Coordinator {
	t.Helper()

	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
	})

	coord, err := NewCoordinator(&Config{Client: client})
	require.NoError(t, err)
	return coord
}

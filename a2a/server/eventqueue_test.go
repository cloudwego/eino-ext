/*
 * Copyright 2025 CloudWeGo Authors
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

package server

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloudwego/eino-ext/a2a/models"
)

func TestInMemoryEventQueue(t *testing.T) {
	eq := newInMemoryEventQueue().(*inMemoryEventQueue)
	ctx := context.Background()
	assert.NoError(t, eq.Reset(ctx, "1"))
	assert.NoError(t, eq.Push(ctx, "1", &models.SendMessageStreamingResponseUnion{Message: &models.Message{Role: "1"}}, nil))
	assert.NoError(t, eq.Push(ctx, "1", &models.SendMessageStreamingResponseUnion{Message: &models.Message{Role: "2"}}, nil))
	assert.NoError(t, eq.Push(ctx, "1", nil, fmt.Errorf("test error")))
	assert.NoError(t, eq.Push(ctx, "1", &models.SendMessageStreamingResponseUnion{Message: &models.Message{Role: "3"}}, nil))
	assert.NoError(t, eq.Close(ctx, "1"))

	e, taskErr, closed, err := eq.Pop(ctx, "1")
	assert.NoError(t, err)
	assert.False(t, closed)
	assert.Nil(t, taskErr)
	assert.Equal(t, models.Role("1"), e.Message.Role)
	e, taskErr, closed, err = eq.Pop(ctx, "1")
	assert.NoError(t, err)
	assert.False(t, closed)
	assert.Nil(t, taskErr)
	assert.Equal(t, models.Role("2"), e.Message.Role)
	e, taskErr, closed, err = eq.Pop(ctx, "1")
	assert.NoError(t, err)
	assert.False(t, closed)
	assert.ErrorContains(t, taskErr, "test error")
	assert.Nil(t, e)
	e, taskErr, closed, err = eq.Pop(ctx, "1")
	assert.NoError(t, err)
	assert.False(t, closed)
	assert.Nil(t, taskErr)
	assert.Equal(t, models.Role("3"), e.Message.Role)
	e, taskErr, closed, err = eq.Pop(ctx, "1")
	assert.NoError(t, err)
	assert.True(t, closed)
	assert.Nil(t, taskErr)
	assert.Nil(t, e)
	exists, _ := queueState(eq, "1")
	assert.False(t, exists, "closed and drained queue should be removed")
}

func msgEvent(id string) *models.SendMessageStreamingResponseUnion {
	return &models.SendMessageStreamingResponseUnion{Message: &models.Message{MessageID: id}}
}

// queueState reports whether taskID currently has a queue and whether its
// eviction timer is armed.
func queueState(eq *inMemoryEventQueue, taskID string) (exists, evictionArmed bool) {
	eq.mu.Lock()
	defer eq.mu.Unlock()
	q, ok := eq.queues[taskID]
	return ok, ok && q.evictTimer != nil
}

// drain pops from sub until it observes closed and returns the message IDs read.
// It does not stop the test, so it is safe to call from any goroutine.
func drain(t *testing.T, sub *inMemorySubscription) []string {
	t.Helper()
	var ids []string
	for {
		e, taskErr, closed, err := sub.Pop(context.Background())
		if closed || !assert.NoError(t, err) || !assert.NoError(t, taskErr) {
			return ids
		}
		ids = append(ids, e.Message.MessageID)
	}
}

func TestInMemoryEventQueueEviction(t *testing.T) {
	ctx := context.Background()
	const taskID = "task"

	t.Run("drained queue is removed immediately", func(t *testing.T) {
		eq := &inMemoryEventQueue{}
		require.NoError(t, eq.Reset(ctx, taskID))
		require.NoError(t, eq.Push(ctx, taskID, msgEvent("e1"), nil))
		require.NoError(t, eq.Close(ctx, taskID))
		eq.mu.Lock()
		q := eq.queues[taskID]
		eq.mu.Unlock()

		sub, err := eq.subscribe(taskID)
		require.NoError(t, err)
		assert.Equal(t, []string{"e1"}, drain(t, sub))
		sub.Release()

		exists, _ := queueState(eq, taskID)
		assert.False(t, exists)
		assert.Nil(t, q.evictTimer, "eviction timer must be stopped once drained")
	})

	t.Run("closed undrained queue is evicted after the grace period", func(t *testing.T) {
		eq := &inMemoryEventQueue{closedTTL: time.Millisecond}
		require.NoError(t, eq.Reset(ctx, taskID))
		require.NoError(t, eq.Push(ctx, taskID, msgEvent("e1"), nil))
		require.NoError(t, eq.Push(ctx, taskID, msgEvent("e2"), nil))
		require.NoError(t, eq.Close(ctx, taskID))

		e, _, _, err := eq.Pop(ctx, taskID) // partially consumed, then abandoned
		require.NoError(t, err)
		assert.Equal(t, "e1", e.Message.MessageID)

		assert.Eventually(t, func() bool {
			exists, _ := queueState(eq, taskID)
			return !exists
		}, 5*time.Second, time.Millisecond)
		_, _, _, err = eq.Pop(ctx, taskID)
		assert.Error(t, err)
	})

	t.Run("grace period starts only after the last subscriber leaves", func(t *testing.T) {
		eq := &inMemoryEventQueue{}
		require.NoError(t, eq.Reset(ctx, taskID))
		require.NoError(t, eq.Push(ctx, taskID, msgEvent("e1"), nil))
		require.NoError(t, eq.Push(ctx, taskID, msgEvent("e2"), nil))
		sub1, err := eq.subscribe(taskID)
		require.NoError(t, err)
		sub2, err := eq.subscribe(taskID)
		require.NoError(t, err)
		require.NoError(t, eq.Close(ctx, taskID))
		require.NoError(t, eq.Close(ctx, taskID), "Close must be idempotent")

		_, armed := queueState(eq, taskID)
		assert.False(t, armed, "no eviction while subscribers are active")
		sub1.Release()
		_, armed = queueState(eq, taskID)
		assert.False(t, armed, "no eviction while a subscriber is still active")
		sub2.Release()
		_, armed = queueState(eq, taskID)
		assert.True(t, armed, "grace period starts after the last subscriber leaves")

		sub3, err := eq.subscribe(taskID) // a resubscribing client cancels the grace period
		require.NoError(t, err)
		_, armed = queueState(eq, taskID)
		assert.False(t, armed)
		assert.Equal(t, []string{"e1", "e2"}, drain(t, sub3))
		sub3.Release()
		exists, _ := queueState(eq, taskID)
		assert.False(t, exists)
	})
}

func TestInMemoryEventQueueReset(t *testing.T) {
	ctx := context.Background()
	const taskID = "task"

	t.Run("subscriber stays bound to its generation", func(t *testing.T) {
		eq := &inMemoryEventQueue{}
		require.NoError(t, eq.Reset(ctx, taskID))
		require.NoError(t, eq.Push(ctx, taskID, msgEvent("old"), nil))
		oldSub, err := eq.subscribe(taskID)
		require.NoError(t, err)

		require.NoError(t, eq.Reset(ctx, taskID)) // retires the still open old queue
		require.NoError(t, eq.Push(ctx, taskID, msgEvent("new"), nil))

		assert.Equal(t, []string{"old"}, drain(t, oldSub), "old reader drains its own queue without blocking")
		oldSub.Release()
		exists, armed := queueState(eq, taskID)
		assert.True(t, exists, "draining the old queue must not remove the new one")
		assert.False(t, armed, "releasing the old queue must not arm eviction of the new one")

		require.NoError(t, eq.Close(ctx, taskID))
		newSub, err := eq.subscribe(taskID)
		require.NoError(t, err)
		assert.Equal(t, []string{"new"}, drain(t, newSub))
		newSub.Release()
	})

	t.Run("wakes a reader blocked on the old queue", func(t *testing.T) {
		eq := &inMemoryEventQueue{}
		require.NoError(t, eq.Reset(ctx, taskID))
		oldSub, err := eq.subscribe(taskID)
		require.NoError(t, err)
		done := make(chan []string, 1)
		go func() { done <- drain(t, oldSub) }()

		require.NoError(t, eq.Reset(ctx, taskID))
		select {
		case ids := <-done:
			assert.Empty(t, ids)
		case <-time.After(5 * time.Second):
			t.Fatal("reader of the retired queue is still blocked")
		}
		oldSub.Release()
	})

	t.Run("Push to a closed queue returns an error instead of panicking", func(t *testing.T) {
		eq := &inMemoryEventQueue{}
		require.NoError(t, eq.Reset(ctx, taskID))
		require.NoError(t, eq.Close(ctx, taskID))
		assert.ErrorContains(t, eq.Push(ctx, taskID, msgEvent("late"), nil), "closed")
	})
}

func TestInMemoryEventQueueConcurrentLifecycle(t *testing.T) {
	ctx := context.Background()
	// A tiny grace period makes eviction timers fire while readers are active.
	eq := &inMemoryEventQueue{closedTTL: time.Microsecond}
	const workers, rounds, events = 4, 50, 20

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		w := w
		wg.Add(2)
		// Each worker owns a task and checks that an active subscriber never loses events.
		go func() {
			defer wg.Done()
			taskID := fmt.Sprintf("owned-%d", w)
			for r := 0; r < rounds; r++ {
				assert.NoError(t, eq.Reset(ctx, taskID))
				sub, err := eq.subscribe(taskID)
				if !assert.NoError(t, err) {
					return
				}
				go func() {
					for i := 0; i < events; i++ {
						assert.NoError(t, eq.Push(ctx, taskID, msgEvent(fmt.Sprint(i)), nil))
					}
					assert.NoError(t, eq.Close(ctx, taskID))
				}()
				assert.Len(t, drain(t, sub), events)
				sub.Release()
			}
		}()
		// All workers also race Reset/Push/Close/Pop on one shared task; errors are
		// expected, panics and deadlocks are not.
		go func() {
			defer wg.Done()
			for r := 0; r < rounds; r++ {
				_ = eq.Reset(ctx, "shared")
				_ = eq.Push(ctx, "shared", msgEvent("x"), nil)
				_ = eq.Close(ctx, "shared")
				_, _, _, _ = eq.Pop(ctx, "shared")
			}
		}()
	}
	wg.Wait()

	assert.Eventually(t, func() bool {
		eq.mu.Lock()
		defer eq.mu.Unlock()
		return len(eq.queues) == 0
	}, 5*time.Second, time.Millisecond, "all queues must be drained or evicted")
}

func TestUnboundedChanReleasesConsumedEvents(t *testing.T) {
	ch := newUnboundedChan[*int]()
	a, b := new(int), new(int)
	ch.Send(a)
	ch.Send(b)
	backing := ch.buffer[:2]

	v, ok := ch.Receive()
	require.True(t, ok)
	assert.Same(t, a, v)
	assert.Nil(t, backing[0], "consumed slot must not keep the event reachable")

	v, ok = ch.Receive()
	require.True(t, ok)
	assert.Same(t, b, v)
	assert.Nil(t, backing[1])
	assert.Nil(t, ch.buffer, "drained buffer must release its backing array")
}

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
	_, ok := eq.chanMap.Load("1")
	assert.False(t, ok, "closed and drained queue should be removed")
}

func TestResubscribeStopsWaitingWhenRequestExpires(t *testing.T) {
	const taskID = "running-task"
	queue := newInMemoryEventQueue()
	require.NoError(t, queue.Reset(context.Background(), taskID))
	t.Cleanup(func() { _ = queue.Close(context.Background(), taskID) })
	server := &A2AServer{queue: queue}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- server.resubscribeTask(ctx, &models.TaskIDParams{ID: taskID}, &mockWriter{})
	}()

	select {
	case err := <-done:
		require.ErrorIs(t, err, context.DeadlineExceeded)
	case <-time.After(time.Second):
		t.Fatal("resubscribe is still waiting for events after the request expired")
	}

	want := &models.SendMessageStreamingResponseUnion{Message: &models.Message{MessageID: "next"}}
	require.NoError(t, queue.Push(context.Background(), taskID, want, nil))
	freshCtx, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	event, taskErr, closed, err := queue.Pop(freshCtx, taskID)
	require.NoError(t, err)
	require.NoError(t, taskErr)
	require.False(t, closed)
	require.Same(t, want, event)
}

func TestCanceledPopKeepsBufferedEvent(t *testing.T) {
	const taskID = "buffered-task"
	queue := newInMemoryEventQueue()
	require.NoError(t, queue.Reset(context.Background(), taskID))
	want := &models.SendMessageStreamingResponseUnion{Message: &models.Message{MessageID: "queued"}}
	require.NoError(t, queue.Push(context.Background(), taskID, want, nil))
	require.NoError(t, queue.Close(context.Background(), taskID))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	event, _, closed, err := queue.Pop(ctx, taskID)
	require.ErrorIs(t, err, context.Canceled)
	require.False(t, closed)
	require.Nil(t, event)

	event, taskErr, closed, err := queue.Pop(context.Background(), taskID)
	require.NoError(t, err)
	require.NoError(t, taskErr)
	require.False(t, closed)
	require.Same(t, want, event)
}

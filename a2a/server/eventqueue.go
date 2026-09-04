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

	"github.com/cloudwego/eino-ext/a2a/models"
)

type EventQueue interface {
	Push(ctx context.Context, taskID string, event *models.SendMessageStreamingResponseUnion, taskErr error) error
	Pop(ctx context.Context, taskID string) (event *models.SendMessageStreamingResponseUnion, taskErr error, closed bool, err error)
	Close(ctx context.Context, taskID string) error
	Reset(ctx context.Context, taskID string) error
}

func newInMemoryEventQueue() EventQueue {
	return &inMemoryEventQueue{}
}

type inMemoryEventQueue struct {
	chanMap sync.Map
}

type inMemoryEventQueuePair struct {
	taskErr error
	union   *models.SendMessageStreamingResponseUnion
}

func (i *inMemoryEventQueue) Push(ctx context.Context, taskID string, event *models.SendMessageStreamingResponseUnion, taskErr error) error {
	v, ok := i.chanMap.Load(taskID)
	if !ok {
		return fmt.Errorf("failed to push queue: cannot find the queue of task[%s]", taskID)
	}
	ch := v.(*unboundedChan[*inMemoryEventQueuePair])
	ch.Send(&inMemoryEventQueuePair{
		taskErr: taskErr,
		union:   event,
	})
	return nil
}

func (i *inMemoryEventQueue) Pop(ctx context.Context, taskID string) (event *models.SendMessageStreamingResponseUnion, taskErr error, closed bool, err error) {
	v, ok := i.chanMap.Load(taskID)
	if !ok {
		return nil, nil, false, fmt.Errorf("failed to pop from queue: cannot find the queue of task[%s]", taskID)
	}
	ch := v.(*unboundedChan[*inMemoryEventQueuePair])
	resp, success := ch.Receive()
	if success {
		return resp.union, resp.taskErr, false, nil
	}
	return nil, nil, true, nil
}

func (i *inMemoryEventQueue) Close(ctx context.Context, taskID string) error {
	v, ok := i.chanMap.Load(taskID)
	if !ok {
		return fmt.Errorf("failed to close queue: cannot find the queue of task[%s]", taskID)
	}
	v.(*unboundedChan[*inMemoryEventQueuePair]).Close()
	return nil
}

func (i *inMemoryEventQueue) Reset(ctx context.Context, taskID string) error {
	i.chanMap.Store(taskID, newUnboundedChan[*inMemoryEventQueuePair]())
	return nil
}

type unboundedChan[T any] struct {
	buffer   []T        // Internal buffer to store data
	mutex    sync.Mutex // Mutex to protect buffer access
	notEmpty *sync.Cond // Condition variable to wait for data
	closed   bool       // Indicates if the channel has been closed
}

func newUnboundedChan[T any]() *unboundedChan[T] {
	ch := &unboundedChan[T]{}
	ch.notEmpty = sync.NewCond(&ch.mutex)
	return ch
}

func (ch *unboundedChan[T]) Send(value T) {
	ch.mutex.Lock()
	defer ch.mutex.Unlock()

	if ch.closed {
		panic("send on closed channel")
	}

	ch.buffer = append(ch.buffer, value)
	ch.notEmpty.Signal() // Wake up one goroutine waiting to receive
}

func (ch *unboundedChan[T]) Receive() (T, bool) {
	ch.mutex.Lock()
	defer ch.mutex.Unlock()

	for len(ch.buffer) == 0 && !ch.closed {
		ch.notEmpty.Wait() // Wait until data is available
	}

	if len(ch.buffer) == 0 {
		// Channel is closed and empty
		var zero T
		return zero, false
	}

	val := ch.buffer[0]
	ch.buffer = ch.buffer[1:]
	return val, true
}

func (ch *unboundedChan[T]) Close() {
	ch.mutex.Lock()
	defer ch.mutex.Unlock()

	if !ch.closed {
		ch.closed = true
		ch.notEmpty.Broadcast() // Wake up all waiting goroutines
	}
}

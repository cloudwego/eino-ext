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
	"time"

	"github.com/cloudwego/eino-ext/a2a/models"
)

// defaultClosedQueueTTL is how long a closed but undrained in-memory queue is
// retained after its last subscriber leaves, so that a disconnected client can
// still resubscribe to the remaining events.
const defaultClosedQueueTTL = 5 * time.Minute

type EventQueue interface {
	Push(ctx context.Context, taskID string, event *models.SendMessageStreamingResponseUnion, taskErr error) error
	Pop(ctx context.Context, taskID string) (event *models.SendMessageStreamingResponseUnion, taskErr error, closed bool, err error)
	Close(ctx context.Context, taskID string) error
	Reset(ctx context.Context, taskID string) error
}

func newInMemoryEventQueue() EventQueue {
	return &inMemoryEventQueue{}
}

// inMemoryEventQueue is the default EventQueue. A task's queue is removed as
// soon as a reader drains it, or closedTTL after it has been closed and its
// last subscriber has left, whichever happens first.
type inMemoryEventQueue struct {
	mu     sync.Mutex
	queues map[string]*inMemoryQueue
	// closedTTL overrides defaultClosedQueueTTL when positive.
	closedTTL time.Duration
}

type inMemoryQueue struct {
	ch          *unboundedChan[*inMemoryEventQueuePair]
	closed      bool
	subscribers int
	evictTimer  *time.Timer
}

type inMemoryEventQueuePair struct {
	taskErr error
	union   *models.SendMessageStreamingResponseUnion
}

// inMemorySubscription is an active reader bound to one generation of a task's
// queue. The queue is never evicted while a subscription is held, even if the
// reader is blocked outside Pop, e.g. writing to a slow client.
type inMemorySubscription struct {
	eq     *inMemoryEventQueue
	taskID string
	q      *inMemoryQueue
}

func (i *inMemoryEventQueue) subscribe(taskID string) (*inMemorySubscription, error) {
	i.mu.Lock()
	defer i.mu.Unlock()

	q, ok := i.queues[taskID]
	if !ok {
		return nil, fmt.Errorf("failed to pop from queue: cannot find the queue of task[%s]", taskID)
	}
	q.subscribers++
	q.stopEvictTimer()
	return &inMemorySubscription{eq: i, taskID: taskID, q: q}, nil
}

func (s *inMemorySubscription) Pop(ctx context.Context) (event *models.SendMessageStreamingResponseUnion, taskErr error, closed bool, err error) {
	resp, ok := s.q.ch.Receive()
	if ok {
		return resp.union, resp.taskErr, false, nil
	}
	s.eq.mu.Lock()
	s.eq.removeLocked(s.taskID, s.q)
	s.eq.mu.Unlock()
	return nil, nil, true, nil
}

func (s *inMemorySubscription) Release() {
	s.eq.mu.Lock()
	defer s.eq.mu.Unlock()

	s.q.subscribers--
	s.eq.scheduleEvictionLocked(s.taskID, s.q)
}

func (i *inMemoryEventQueue) Push(ctx context.Context, taskID string, event *models.SendMessageStreamingResponseUnion, taskErr error) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	q, ok := i.queues[taskID]
	if !ok {
		return fmt.Errorf("failed to push queue: cannot find the queue of task[%s]", taskID)
	}
	if q.closed {
		return fmt.Errorf("failed to push queue: the queue of task[%s] is closed", taskID)
	}
	q.ch.Send(&inMemoryEventQueuePair{
		taskErr: taskErr,
		union:   event,
	})
	return nil
}

func (i *inMemoryEventQueue) Pop(ctx context.Context, taskID string) (event *models.SendMessageStreamingResponseUnion, taskErr error, closed bool, err error) {
	sub, err := i.subscribe(taskID)
	if err != nil {
		return nil, nil, false, err
	}
	defer sub.Release()
	return sub.Pop(ctx)
}

func (i *inMemoryEventQueue) Close(ctx context.Context, taskID string) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	q, ok := i.queues[taskID]
	if !ok {
		return fmt.Errorf("failed to close queue: cannot find the queue of task[%s]", taskID)
	}
	q.closed = true
	q.ch.Close()
	i.scheduleEvictionLocked(taskID, q)
	return nil
}

func (i *inMemoryEventQueue) Reset(ctx context.Context, taskID string) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if old, ok := i.queues[taskID]; ok {
		// Retire the previous generation: its subscribers drain what is left
		// and then observe closed, while new readers only see the new queue.
		old.closed = true
		old.ch.Close()
		old.stopEvictTimer()
	}
	if i.queues == nil {
		i.queues = make(map[string]*inMemoryQueue)
	}
	i.queues[taskID] = &inMemoryQueue{ch: newUnboundedChan[*inMemoryEventQueuePair]()}
	return nil
}

// scheduleEvictionLocked starts the eviction timer of q if q is still the
// current queue of taskID, is closed and has no subscriber.
func (i *inMemoryEventQueue) scheduleEvictionLocked(taskID string, q *inMemoryQueue) {
	if !q.closed || q.subscribers > 0 || q.evictTimer != nil || i.queues[taskID] != q {
		return
	}
	ttl := i.closedTTL
	if ttl <= 0 {
		ttl = defaultClosedQueueTTL
	}
	var t *time.Timer
	t = time.AfterFunc(ttl, func() {
		i.mu.Lock()
		defer i.mu.Unlock()
		if q.evictTimer == t { // otherwise the timer was stopped after it had fired
			i.removeLocked(taskID, q)
		}
	})
	q.evictTimer = t
}

func (i *inMemoryEventQueue) removeLocked(taskID string, q *inMemoryQueue) {
	q.stopEvictTimer()
	if i.queues[taskID] == q {
		delete(i.queues, taskID)
	}
}

func (q *inMemoryQueue) stopEvictTimer() {
	if q.evictTimer != nil {
		q.evictTimer.Stop()
		q.evictTimer = nil
	}
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

	var zero T
	if len(ch.buffer) == 0 {
		// Channel is closed and empty
		return zero, false
	}

	val := ch.buffer[0]
	ch.buffer[0] = zero // drop the reference held by the consumed slot
	ch.buffer = ch.buffer[1:]
	if len(ch.buffer) == 0 {
		ch.buffer = nil // release the backing array once drained
	}
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

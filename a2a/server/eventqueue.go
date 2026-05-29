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

type OffsetEventQueue interface {
	EventQueue
	PushWithOffset(ctx context.Context, taskID string, event *models.SendMessageStreamingResponseUnion, taskErr error) (int64, error)
	PopFromOffset(ctx context.Context, taskID string, offset int64) (event *models.SendMessageStreamingResponseUnion, taskErr error, currentOffset int64, closed bool, err error)
	CurrentOffset(ctx context.Context, taskID string) (int64, error)
}

func newInMemoryEventQueue() EventQueue {
	return &inMemoryEventQueue{}
}

func NewInMemoryOffsetEventQueue() OffsetEventQueue {
	return &inMemoryOffsetEventQueue{}
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

type inMemoryOffsetEventQueue struct {
	queueMap sync.Map
}

type offsetEvent struct {
	taskErr error
	union   *models.SendMessageStreamingResponseUnion
}

type inMemoryOffsetQueue struct {
	events     []*offsetEvent
	nextOffset int64
	readOffset int64
	closed     bool
	mutex      sync.Mutex
	notEmpty   *sync.Cond
}

func newInMemoryOffsetQueue() *inMemoryOffsetQueue {
	q := &inMemoryOffsetQueue{}
	q.notEmpty = sync.NewCond(&q.mutex)
	return q
}

func (i *inMemoryOffsetEventQueue) Push(ctx context.Context, taskID string, event *models.SendMessageStreamingResponseUnion, taskErr error) error {
	_, err := i.PushWithOffset(ctx, taskID, event, taskErr)
	return err
}

func (i *inMemoryOffsetEventQueue) PushWithOffset(ctx context.Context, taskID string, event *models.SendMessageStreamingResponseUnion, taskErr error) (int64, error) {
	v, ok := i.queueMap.Load(taskID)
	if !ok {
		return 0, fmt.Errorf("failed to push queue: cannot find the queue of task[%s]", taskID)
	}
	return v.(*inMemoryOffsetQueue).push(event, taskErr)
}

func (i *inMemoryOffsetEventQueue) Pop(ctx context.Context, taskID string) (event *models.SendMessageStreamingResponseUnion, taskErr error, closed bool, err error) {
	v, ok := i.queueMap.Load(taskID)
	if !ok {
		return nil, nil, false, fmt.Errorf("failed to pop from queue: cannot find the queue of task[%s]", taskID)
	}
	return v.(*inMemoryOffsetQueue).pop()
}

func (i *inMemoryOffsetEventQueue) PopFromOffset(ctx context.Context, taskID string, offset int64) (event *models.SendMessageStreamingResponseUnion, taskErr error, currentOffset int64, closed bool, err error) {
	v, ok := i.queueMap.Load(taskID)
	if !ok {
		return nil, nil, 0, false, fmt.Errorf("failed to pop from queue: cannot find the queue of task[%s]", taskID)
	}
	return v.(*inMemoryOffsetQueue).popFromOffset(offset)
}

func (i *inMemoryOffsetEventQueue) CurrentOffset(ctx context.Context, taskID string) (int64, error) {
	v, ok := i.queueMap.Load(taskID)
	if !ok {
		return 0, fmt.Errorf("failed to get offset: cannot find the queue of task[%s]", taskID)
	}
	return v.(*inMemoryOffsetQueue).currentOffset(), nil
}

func (i *inMemoryOffsetEventQueue) Close(ctx context.Context, taskID string) error {
	v, ok := i.queueMap.Load(taskID)
	if !ok {
		return fmt.Errorf("failed to close queue: cannot find the queue of task[%s]", taskID)
	}
	v.(*inMemoryOffsetQueue).close()
	return nil
}

func (i *inMemoryOffsetEventQueue) Reset(ctx context.Context, taskID string) error {
	i.queueMap.Store(taskID, newInMemoryOffsetQueue())
	return nil
}

func (q *inMemoryOffsetQueue) push(event *models.SendMessageStreamingResponseUnion, taskErr error) (int64, error) {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	if q.closed {
		return 0, fmt.Errorf("send on closed queue")
	}
	offset := q.nextOffset
	q.events = append(q.events, &offsetEvent{
		taskErr: taskErr,
		union:   event,
	})
	q.nextOffset++
	q.notEmpty.Signal()
	return offset, nil
}

func (q *inMemoryOffsetQueue) pop() (event *models.SendMessageStreamingResponseUnion, taskErr error, closed bool, err error) {
	event, taskErr, _, closed, err = q.popFromOffset(q.readOffset)
	if err != nil || closed {
		return event, taskErr, closed, err
	}
	q.readOffset++
	return event, taskErr, false, nil
}

func (q *inMemoryOffsetQueue) popFromOffset(offset int64) (event *models.SendMessageStreamingResponseUnion, taskErr error, currentOffset int64, closed bool, err error) {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	for {
		if offset < int64(len(q.events)) {
			ev := q.events[offset]
			return ev.union, ev.taskErr, offset, false, nil
		}
		if q.closed {
			return nil, nil, 0, true, nil
		}
		q.notEmpty.Wait()
	}
}

func (q *inMemoryOffsetQueue) currentOffset() int64 {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	return q.nextOffset
}

func (q *inMemoryOffsetQueue) close() {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	if !q.closed {
		q.closed = true
		q.notEmpty.Broadcast()
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

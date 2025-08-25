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

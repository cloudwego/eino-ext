package server

import (
	"context"
	"fmt"
	"sync"
)

type TaskLocker interface {
	Lock(ctx context.Context, id string) error
	Unlock(ctx context.Context, id string) error
}

func newInMemoryTaskLocker() TaskLocker {
	return &inMemoryTaskLocker{}
}

type inMemoryTaskLocker struct {
	mutexMap sync.Map
}

func (i *inMemoryTaskLocker) Lock(ctx context.Context, id string) error {
	mu, _ := i.mutexMap.LoadOrStore(id, &sync.Mutex{})
	mutex := mu.(*sync.Mutex)

	mutex.Lock()

	return nil
}

func (i *inMemoryTaskLocker) Unlock(ctx context.Context, id string) error {
	mu, ok := i.mutexMap.Load(id)
	if !ok {
		return fmt.Errorf("no lock found for task with id %s", id)
	}

	mutex := mu.(*sync.Mutex)
	mutex.Unlock()

	return nil
}

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

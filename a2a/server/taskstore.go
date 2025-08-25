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
	"sync"

	"github.com/cloudwego/eino-ext/a2a/models"
)

type TaskStore interface {
	Get(ctx context.Context, id string) (*models.Task, bool, error)
	Save(ctx context.Context, task *models.Task) error
}

func newInMemoryTaskStore() TaskStore {
	return &inMemoryTaskStore{}
}

type inMemoryTaskStore struct {
	taskMap sync.Map
}

func (i *inMemoryTaskStore) Get(ctx context.Context, id string) (*models.Task, bool, error) {
	t, ok := i.taskMap.Load(id)
	if !ok {
		return nil, false, nil
	}
	return t.(*models.Task), true, nil
}

func (i *inMemoryTaskStore) Save(ctx context.Context, task *models.Task) error {
	i.taskMap.Store(task.ID, task)
	return nil
}

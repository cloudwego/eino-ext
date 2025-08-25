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

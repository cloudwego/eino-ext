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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/cloudwego/eino-ext/a2a/models"
)

type PushNotifier interface {
	Set(ctx context.Context, config *models.TaskPushNotificationConfig) error
	Get(ctx context.Context, taskID string) (models.PushNotificationConfig, bool, error)
	Delete(ctx context.Context, taskID string) error
	SendNotification(ctx context.Context, event *models.SendMessageStreamingResponseUnion) error
}

func NewInMemoryPushNotifier() PushNotifier {
	return &inMemoryPushNotifier{}
}

type inMemoryPushNotifier struct {
	infoMap sync.Map
}

func (i *inMemoryPushNotifier) Set(ctx context.Context, config *models.TaskPushNotificationConfig) error {
	if config == nil {
		return nil
	}
	i.infoMap.Store(config.TaskID, config.PushNotificationConfig)
	return nil
}

func (i *inMemoryPushNotifier) Get(ctx context.Context, taskID string) (models.PushNotificationConfig, bool, error) {
	result, ok := i.infoMap.Load(taskID)
	if !ok {
		return models.PushNotificationConfig{}, false, nil
	}
	return result.(models.PushNotificationConfig), true, nil
}

func (i *inMemoryPushNotifier) Delete(ctx context.Context, taskID string) error {
	i.infoMap.Delete(taskID)
	return nil
}

func (i *inMemoryPushNotifier) SendNotification(ctx context.Context, event *models.SendMessageStreamingResponseUnion) error {
	if event == nil {
		return nil
	}
	config, existed, err := i.Get(ctx, event.GetTaskID())
	if err != nil {
		return err
	}
	if !existed {
		return nil
	}

	var body []byte
	body, err = json.Marshal(wrapUnion(event))
	if err != nil {
		return fmt.Errorf("failed to marshal event when send notification of task[%s]: %w", event.GetTaskID(), err)
	}
	req, err := http.NewRequest(http.MethodPost, config.URL, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create http request when send notification of task[%s]: %w", event.GetTaskID(), err)
	}
	req = req.WithContext(ctx)

	// todo: how to set auth

	_, err = http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send notification of task[%s]: %w", event.GetTaskID(), err)
	}
	return nil
}

func wrapUnion(u *models.SendMessageStreamingResponseUnion) any {
	if u == nil {
		return nil
	}
	if u.Message != nil {
		return struct {
			*models.Message
			Kind models.ResponseKind `json:"kind"`
		}{
			Message: u.Message,
			Kind:    models.ResponseKindMessage,
		}
	} else if u.Task != nil {
		return struct {
			*models.Task
			Kind models.ResponseKind `json:"kind"`
		}{
			Task: u.Task,
			Kind: models.ResponseKindTask,
		}
	} else if u.TaskStatusUpdateEvent != nil {
		return struct {
			*models.TaskStatusUpdateEvent
			Kind models.ResponseKind `json:"kind"`
		}{
			TaskStatusUpdateEvent: u.TaskStatusUpdateEvent,
			Kind:                  models.ResponseKindStatusUpdate,
		}
	} else if u.TaskArtifactUpdateEvent != nil {
		return struct {
			*models.TaskArtifactUpdateEvent
			Kind models.ResponseKind `json:"kind"`
		}{
			TaskArtifactUpdateEvent: u.TaskArtifactUpdateEvent,
			Kind:                    models.ResponseKindArtifactUpdate,
		}
	}
	return nil
}

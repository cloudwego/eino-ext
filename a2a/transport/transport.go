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

package transport

import (
	"context"

	"github.com/cloudwego/eino-ext/a2a/models"
)

type ClientTransport interface {
	AgentCard(ctx context.Context) (*models.AgentCard, error)
	SendMessage(ctx context.Context, params *models.MessageSendParams) (*models.SendMessageResponseUnion, error)
	SendMessageStreaming(ctx context.Context, params *models.MessageSendParams) (models.ResponseReader, error)
	GetTask(ctx context.Context, params *models.TaskQueryParams) (*models.Task, error)
	CancelTask(ctx context.Context, params *models.TaskIDParams) (*models.Task, error)
	ResubscribeTask(ctx context.Context, params *models.TaskIDParams) (models.ResponseReader, error)
	SetPushNotificationConfig(ctx context.Context, params *models.TaskPushNotificationConfig) (*models.TaskPushNotificationConfig, error)
	GetPushNotificationConfig(ctx context.Context, params *models.GetTaskPushNotificationConfigParams) (*models.TaskPushNotificationConfig, error)
	Close() error
}

type HandlerRegistrar interface {
	Register(context.Context, *models.ServerHandlers) error
}

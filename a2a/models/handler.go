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

package models

import "context"

type ResponseWriter interface {
	Write(ctx context.Context, f *SendMessageStreamingResponseUnion) error
	Close() error
}

type ResponseReader interface {
	Read() (*SendMessageStreamingResponseUnion, error)
	Close() error
}

type ServerHandlers struct {
	AgentCard                 func(ctx context.Context) *AgentCard
	SendMessage               func(ctx context.Context, params *MessageSendParams) (*SendMessageResponseUnion, error)
	SendMessageStreaming      func(ctx context.Context, params *MessageSendParams, writer ResponseWriter) error
	GetTask                   func(ctx context.Context, params *TaskQueryParams) (*Task, error)
	CancelTask                func(ctx context.Context, params *TaskIDParams) (*Task, error)
	ResubscribeTask           func(ctx context.Context, params *TaskIDParams, writer ResponseWriter) error
	SetPushNotificationConfig func(ctx context.Context, params *TaskPushNotificationConfig) (*TaskPushNotificationConfig, error)
	GetPushNotificationConfig func(ctx context.Context, params *GetTaskPushNotificationConfigParams) (*TaskPushNotificationConfig, error)
}

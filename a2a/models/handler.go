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

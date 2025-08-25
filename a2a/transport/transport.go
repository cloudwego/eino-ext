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

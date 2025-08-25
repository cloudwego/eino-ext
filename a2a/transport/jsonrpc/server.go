package jsonrpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/cloudwego/eino-ext/a2a/models"
	"github.com/cloudwego/eino-ext/a2a/transport"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/route"

	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc/core"
	jsonrpc_http "github.com/cloudwego/eino-ext/a2a/transport/jsonrpc/pkg/transport/http"
	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc/server"
	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc/streaming"
)

type ServerConfig struct {
	Router               route.IRoutes
	AgentCardPath        *string
	AgentCardMiddleWares []app.HandlerFunc
	HandlerPath          string
	HandlerMiddleWares   []app.HandlerFunc
}

func NewRegistrar(ctx context.Context, config *ServerConfig) (transport.HandlerRegistrar, error) {
	if config == nil {
		return nil, errors.New("config is required")
	}
	if config.Router == nil {
		return nil, errors.New("router is required")
	}
	path := ".well-known/agent-card.json"
	if config.AgentCardPath != nil {
		path = *config.AgentCardPath
	}
	return &registry{
		route:                config.Router,
		agentCardPath:        path,
		agentCardMiddleWares: config.AgentCardMiddleWares,
		handlerPath:          config.HandlerPath,
		handlerMiddleWares:   config.HandlerMiddleWares,
	}, nil
}

type registry struct {
	route                route.IRoutes
	agentCardPath        string
	agentCardMiddleWares []app.HandlerFunc
	handlerPath          string
	handlerMiddleWares   []app.HandlerFunc
}

func (r *registry) Register(ctx context.Context, handlers *models.ServerHandlers) error {
	a, h, err := getHertzHandlerFuncs(ctx, handlers)
	if err != nil {
		return err
	}
	r.route.GET(r.agentCardPath, append(r.agentCardMiddleWares, a)...)
	r.route.POST(r.handlerPath, h)
	return nil
}

func getHertzHandlerFuncs(_ context.Context, hs *models.ServerHandlers) (agentCard, handlers app.HandlerFunc, err error) {
	if hs == nil {
		return nil, nil, errors.New("A2AHandlers is nil")
	}
	agentCard = convAgentCardHandler(hs.AgentCard)
	h, err := convHandlers(hs)
	if err != nil {
		return nil, nil, err
	}
	return agentCard, h, nil
}

func convAgentCardHandler(f func(ctx context.Context) *models.AgentCard) app.HandlerFunc {
	if f == nil {
		return nil
	}
	return func(c context.Context, ctx *app.RequestContext) {
		ctx.JSON(http.StatusOK, f(c))
		return
	}
}

func convHandlers(hs *models.ServerHandlers) (app.HandlerFunc, error) {
	var opts []server.Option

	if hs.SendMessage != nil {
		opts = append(opts, server.WithPingPongHandler("message/send", func(ctx context.Context, _ core.Connection, req json.RawMessage) (interface{}, error) {
			input := &models.MessageSendParams{}
			if err := json.Unmarshal(req, input); err != nil {
				return nil, fmt.Errorf("failed to unmarshal input: %w", err)
			}
			u, err := hs.SendMessage(ctx, input)
			if err != nil {
				return nil, err
			}
			return wrapSendMessageResponseUnion(u), nil
		}))
	}
	if hs.SendMessageStreaming != nil {
		opts = append(opts, server.WithServerStreamingHandler("message/stream", func(ctx context.Context, _ core.Connection, req json.RawMessage, srv streaming.ServerStreamingServer) error {
			input := &models.MessageSendParams{}
			if err := json.Unmarshal(req, input); err != nil {
				return fmt.Errorf("failed to unmarshal input: %w", err)
			}
			return hs.SendMessageStreaming(ctx, input, &serverStreamingWrapper{srv})
		}))
	}
	if hs.ResubscribeTask != nil {
		opts = append(opts, server.WithServerStreamingHandler("tasks/resubscribe", func(ctx context.Context, conn core.Connection, req json.RawMessage, srv streaming.ServerStreamingServer) error {
			input := &models.TaskIDParams{}
			if err := json.Unmarshal(req, input); err != nil {
				return fmt.Errorf("failed to unmarshal input: %w", err)
			}
			return hs.ResubscribeTask(ctx, input, &serverStreamingWrapper{srv})
		}))
	}
	if hs.CancelTask != nil {
		opts = append(opts, server.WithPingPongHandler("tasks/cancel", func(ctx context.Context, conn core.Connection, req json.RawMessage) (interface{}, error) {
			input := &models.TaskIDParams{}
			if err := json.Unmarshal(req, input); err != nil {
				return nil, fmt.Errorf("failed to unmarshal input: %w", err)
			}
			return hs.CancelTask(ctx, input)
		}))
	}
	if hs.GetTask != nil {
		opts = append(opts, server.WithPingPongHandler("tasks/get", func(ctx context.Context, conn core.Connection, req json.RawMessage) (interface{}, error) {
			input := &models.TaskQueryParams{}
			if err := json.Unmarshal(req, input); err != nil {
				return nil, fmt.Errorf("failed to unmarshal input: %w", err)
			}
			return hs.GetTask(ctx, input)
		}))
	}
	if hs.GetPushNotificationConfig != nil {
		opts = append(opts, server.WithPingPongHandler("tasks/pushNotificationConfig/get", func(ctx context.Context, conn core.Connection, req json.RawMessage) (interface{}, error) {
			input := &models.GetTaskPushNotificationConfigParams{}
			if err := json.Unmarshal(req, input); err != nil {
				return nil, fmt.Errorf("failed to unmarshal input: %w", err)
			}
			return hs.GetPushNotificationConfig(ctx, input)
		}))
	}
	if hs.SetPushNotificationConfig != nil {
		opts = append(opts, server.WithPingPongHandler("tasks/pushNotificationConfig/set", func(ctx context.Context, conn core.Connection, req json.RawMessage) (interface{}, error) {
			input := &models.TaskPushNotificationConfig{}
			if err := json.Unmarshal(req, input); err != nil {
				return nil, fmt.Errorf("failed to unmarshal input: %w", err)
			}
			return hs.SetPushNotificationConfig(ctx, input)
		}))
	}

	h, err := server.NewServerTransportHandler(opts...)
	if err != nil {
		return nil, err
	}

	return jsonrpc_http.NewServerTransportBuilder("" /*unused*/, jsonrpc_http.WithServerTransportHandler(h)).POST, nil
}

type serverStreamingWrapper struct {
	s streaming.ServerStreamingServer
}

func (s *serverStreamingWrapper) Write(ctx context.Context, f *models.SendMessageStreamingResponseUnion) error {
	return s.s.Send(ctx, wrapSendMessageStreamingResponseUnion(f))
}

func (s *serverStreamingWrapper) Close() error {
	return nil
}

func wrapSendMessageResponseUnion(u *models.SendMessageResponseUnion) any {
	if u == nil {
		return nil
	}
	return wrapSendMessageStreamingResponseUnion(&models.SendMessageStreamingResponseUnion{
		Message: u.Message,
		Task:    u.Task,
	})
}

func wrapSendMessageStreamingResponseUnion(u *models.SendMessageStreamingResponseUnion) any {
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

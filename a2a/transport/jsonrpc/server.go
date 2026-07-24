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

package jsonrpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/route"

	"github.com/cloudwego/eino-ext/a2a/models"
	"github.com/cloudwego/eino-ext/a2a/transport"
	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc/core"
	jsonrpc_http "github.com/cloudwego/eino-ext/a2a/transport/jsonrpc/pkg/transport/http"
	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc/server"
	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc/streaming"
	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc/wire"
)

type ServerConfig struct {
	Router               route.IRoutes
	AgentCardPath        *string
	AgentCardMiddleWares []app.HandlerFunc
	HandlerPath          string
	HandlerMiddleWares   []app.HandlerFunc
	// ProtocolVersions lists the A2A wire versions this endpoint serves.
	// v0.3 and v1.0 use disjoint JSON-RPC method names, so both can be
	// registered on the same handler path and dispatched by method name.
	// Defaults to {ProtocolVersion03, ProtocolVersion10} when empty.
	ProtocolVersions []models.ProtocolVersion
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
	versions := config.ProtocolVersions
	if len(versions) == 0 {
		versions = []models.ProtocolVersion{models.ProtocolVersion03, models.ProtocolVersion10}
	}
	codecs := make([]wire.Codec, 0, len(versions))
	for _, v := range versions {
		c, err := wire.NewCodec(v)
		if err != nil {
			return nil, err
		}
		codecs = append(codecs, c)
	}
	return &registry{
		route:                config.Router,
		agentCardPath:        path,
		agentCardMiddleWares: config.AgentCardMiddleWares,
		handlerPath:          config.HandlerPath,
		handlerMiddleWares:   config.HandlerMiddleWares,
		codecs:               codecs,
	}, nil
}

type registry struct {
	route                route.IRoutes
	agentCardPath        string
	agentCardMiddleWares []app.HandlerFunc
	handlerPath          string
	handlerMiddleWares   []app.HandlerFunc
	codecs               []wire.Codec
}

func (r *registry) Register(ctx context.Context, handlers *models.ServerHandlers) error {
	a, h, err := getHertzHandlerFuncs(ctx, handlers, r.codecs)
	if err != nil {
		return err
	}
	r.route.GET(r.agentCardPath, append(r.agentCardMiddleWares, a)...)
	r.route.POST(r.handlerPath, h)
	return nil
}

func getHertzHandlerFuncs(_ context.Context, hs *models.ServerHandlers, codecs []wire.Codec) (agentCard, handlers app.HandlerFunc, err error) {
	if hs == nil {
		return nil, nil, errors.New("A2AHandlers is nil")
	}
	agentCard = convAgentCardHandler(hs.AgentCard)
	h, err := convHandlers(hs, codecs)
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

func convHandlers(hs *models.ServerHandlers, codecs []wire.Codec) (app.HandlerFunc, error) {
	var opts []server.Option
	// v0.3 and v1.0 method names are disjoint, so registering handlers for
	// every codec on one transport lets the endpoint serve all versions and
	// dispatch by method name. Each handler decodes the request and encodes the
	// response with its own codec.
	for _, codec := range codecs {
		opts = append(opts, handlerOptionsForCodec(hs, codec)...)
	}

	h, err := server.NewServerTransportHandler(opts...)
	if err != nil {
		return nil, err
	}

	return jsonrpc_http.NewServerTransportBuilder("" /*unused*/, jsonrpc_http.WithServerTransportHandler(h)).POST, nil
}

func handlerOptionsForCodec(hs *models.ServerHandlers, codec wire.Codec) []server.Option {
	m := codec.Methods()
	var opts []server.Option

	if hs.SendMessage != nil {
		opts = append(opts, server.WithPingPongHandler(m.Send, func(ctx context.Context, _ core.Connection, req json.RawMessage) (interface{}, error) {
			input, err := codec.DecodeMessageSendParams(req)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal input: %w", err)
			}
			u, err := hs.SendMessage(ctx, input)
			if err != nil {
				return nil, err
			}
			if u == nil {
				return nil, nil
			}
			return codec.EncodeStreamingUnion(&models.SendMessageStreamingResponseUnion{Message: u.Message, Task: u.Task})
		}))
	}
	if hs.SendMessageStreaming != nil {
		opts = append(opts, server.WithServerStreamingHandler(m.Stream, func(ctx context.Context, _ core.Connection, req json.RawMessage, srv streaming.ServerStreamingServer) error {
			input, err := codec.DecodeMessageSendParams(req)
			if err != nil {
				return fmt.Errorf("failed to unmarshal input: %w", err)
			}
			return hs.SendMessageStreaming(ctx, input, &serverStreamingWrapper{s: srv, codec: codec})
		}))
	}
	if hs.ResubscribeTask != nil {
		opts = append(opts, server.WithServerStreamingHandler(m.Resubscribe, func(ctx context.Context, conn core.Connection, req json.RawMessage, srv streaming.ServerStreamingServer) error {
			input, err := codec.DecodeTaskIDParams(req)
			if err != nil {
				return fmt.Errorf("failed to unmarshal input: %w", err)
			}
			return hs.ResubscribeTask(ctx, input, &serverStreamingWrapper{s: srv, codec: codec})
		}))
	}
	if hs.CancelTask != nil {
		opts = append(opts, server.WithPingPongHandler(m.Cancel, func(ctx context.Context, conn core.Connection, req json.RawMessage) (interface{}, error) {
			input, err := codec.DecodeTaskIDParams(req)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal input: %w", err)
			}
			t, err := hs.CancelTask(ctx, input)
			if err != nil {
				return nil, err
			}
			return codec.EncodeTask(t)
		}))
	}
	if hs.GetTask != nil {
		opts = append(opts, server.WithPingPongHandler(m.GetTask, func(ctx context.Context, conn core.Connection, req json.RawMessage) (interface{}, error) {
			input, err := codec.DecodeTaskQueryParams(req)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal input: %w", err)
			}
			t, err := hs.GetTask(ctx, input)
			if err != nil {
				return nil, err
			}
			return codec.EncodeTask(t)
		}))
	}
	if hs.GetPushNotificationConfig != nil {
		opts = append(opts, server.WithPingPongHandler(m.PushGet, func(ctx context.Context, conn core.Connection, req json.RawMessage) (interface{}, error) {
			input, err := codec.DecodeGetPushParams(req)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal input: %w", err)
			}
			cfg, err := hs.GetPushNotificationConfig(ctx, input)
			if err != nil {
				return nil, err
			}
			return codec.EncodeTaskPushNotificationConfig(cfg)
		}))
	}
	if hs.SetPushNotificationConfig != nil {
		opts = append(opts, server.WithPingPongHandler(m.PushSet, func(ctx context.Context, conn core.Connection, req json.RawMessage) (interface{}, error) {
			input, err := codec.DecodeTaskPushNotificationConfig(req)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal input: %w", err)
			}
			cfg, err := hs.SetPushNotificationConfig(ctx, input)
			if err != nil {
				return nil, err
			}
			return codec.EncodeTaskPushNotificationConfig(cfg)
		}))
	}
	return opts
}

type serverStreamingWrapper struct {
	s     streaming.ServerStreamingServer
	codec wire.Codec
}

func (s *serverStreamingWrapper) Write(ctx context.Context, f *models.SendMessageStreamingResponseUnion) error {
	frame, err := s.codec.EncodeStreamingUnion(f)
	if err != nil {
		return err
	}
	return s.s.Send(ctx, frame)
}

func (s *serverStreamingWrapper) Close() error {
	return nil
}

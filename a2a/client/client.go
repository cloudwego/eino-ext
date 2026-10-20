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

package client

import (
	"context"
	"errors"

	"github.com/cloudwego/eino-ext/a2a/models"
	"github.com/cloudwego/eino-ext/a2a/transport"
)

// A2AClient represents an A2A protocol client
type A2AClient struct {
	cli transport.ClientTransport
}

type Config struct {
	Transport transport.ClientTransport
}

// NewA2AClient creates a new A2A client
func NewA2AClient(ctx context.Context, config *Config) (*A2AClient, error) {
	if config == nil {
		return nil, errors.New("config is required")
	}

	return &A2AClient{cli: config.Transport}, nil
}

func (c *A2AClient) AgentCard(ctx context.Context) (*models.AgentCard, error) {
	return c.cli.AgentCard(ctx)
}

func (c *A2AClient) SendMessage(ctx context.Context, params *models.MessageSendParams) (*models.SendMessageResponseUnion, error) {
	if params != nil {
		(&params.Message).EnsureRequiredFields()
	}
	return c.cli.SendMessage(ctx, params)
}

func (c *A2AClient) SendMessageStreaming(ctx context.Context, params *models.MessageSendParams) (*ServerStreamingWrapper, error) {
	if params != nil {
		(&params.Message).EnsureRequiredFields()
	}
	stream, err := c.cli.SendMessageStreaming(ctx, params)
	if err != nil {
		return nil, err
	}
	return &ServerStreamingWrapper{stream}, nil
}

func (c *A2AClient) GetTask(ctx context.Context, params *models.TaskQueryParams) (*models.Task, error) {
	return c.cli.GetTask(ctx, params)
}

func (c *A2AClient) CancelTask(ctx context.Context, params *models.TaskIDParams) (*models.Task, error) {
	return c.cli.CancelTask(ctx, params)
}

func (c *A2AClient) ResubscribeTask(ctx context.Context, params *models.TaskIDParams) (*ServerStreamingWrapper, error) {
	stream, err := c.cli.ResubscribeTask(ctx, params)
	if err != nil {
		return nil, err
	}
	return &ServerStreamingWrapper{stream}, nil
}

func (c *A2AClient) SetPushNotificationConfig(ctx context.Context, params *models.TaskPushNotificationConfig) (*models.TaskPushNotificationConfig, error) {
	return c.cli.SetPushNotificationConfig(ctx, params)
}

func (c *A2AClient) GetPushNotificationConfig(ctx context.Context, params *models.GetTaskPushNotificationConfigParams) (*models.TaskPushNotificationConfig, error) {
	return c.cli.GetPushNotificationConfig(ctx, params)
}

// todo: list/delete notification config & agent/authenticatedExtendedCard

type ServerStreamingWrapper struct {
	s models.ResponseReader
}

func (s *ServerStreamingWrapper) Recv() (resp *models.SendMessageStreamingResponseUnion, err error) {
	defer func() {
		if err != nil {
			s.s.Close()
		}
	}()
	return s.s.Read()
}

func (s *ServerStreamingWrapper) Close() error {
	return s.s.Close()
}

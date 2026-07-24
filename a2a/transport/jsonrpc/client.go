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
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	std_http "net/http"
	"net/url"

	hertz_client "github.com/cloudwego/hertz/pkg/app/client"
	"github.com/cloudwego/hertz/pkg/protocol/consts"

	"github.com/cloudwego/eino-ext/a2a/models"
	"github.com/cloudwego/eino-ext/a2a/transport"

	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc/client"
	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc/core"
	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc/pkg/metadata"
	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc/pkg/transport/http"
	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc/wire"
)

type ClientConfig struct {
	BaseURL       string
	HandlerPath   string
	AgentCardPath *string
	// HertzClient is the Hertz HTTP client to use for requests.
	// If both HertzClient and HTTPClient are set, only HertzClient will be used.
	// If neither is set, a default Hertz client will be created.
	HertzClient *hertz_client.Client
	// HTTPClient is the standard net/http client to use for requests.
	// If both HertzClient and HTTPClient are set, only HertzClient will be used.
	// If neither is set, a default Hertz client will be created.
	HTTPClient                  *std_http.Client
	SSEBufferSize               *int
	JSONRPCIDGenerator          core.IDGenerator
	DisablePrevHeaderForwarding bool
	// ProtocolVersion selects the A2A wire version this client speaks.
	// Defaults to models.ProtocolVersion03 for backward compatibility.
	ProtocolVersion models.ProtocolVersion
}

func NewTransport(ctx context.Context, config *ClientConfig) (transport.ClientTransport, error) {
	if config == nil {
		return nil, errors.New("config is required")
	}
	agentCardPath := ".well-known/agent-card.json"
	if config.AgentCardPath != nil {
		agentCardPath = *config.AgentCardPath
	}
	transOpts := make([]http.ClientTransportBuilderOption, 0)
	if config.HertzClient != nil {
		transOpts = append(transOpts, http.WithHertzClient(config.HertzClient))
	}
	if config.HTTPClient != nil {
		transOpts = append(transOpts, http.WithHTTPClient(config.HTTPClient))
	}
	if config.SSEBufferSize != nil {
		transOpts = append(transOpts, http.WithSSEBufferSize(*config.SSEBufferSize))
	} else {
		transOpts = append(transOpts, http.WithSSEBufferSize(bufio.MaxScanTokenSize))
	}
	if config.DisablePrevHeaderForwarding {
		transOpts = append(transOpts, http.WithDisablePrevHeaderForwarding())
	}
	var err error
	var handlerURL string
	if len(config.HandlerPath) > 0 {
		handlerURL, err = url.JoinPath(config.BaseURL, config.HandlerPath)
		if err != nil {
			return nil, fmt.Errorf("failed to join handler url: %w", err)
		}
	} else {
		handlerURL = config.BaseURL
	}

	cliOpts := []client.Option{client.WithURL(handlerURL), client.WithTransportHandler(http.NewClientTransportHandler(transOpts...))}
	if config.JSONRPCIDGenerator != nil {
		cliOpts = append(cliOpts, client.WithJSONRPCIDGenerator(config.JSONRPCIDGenerator))
	}
	cli, err := client.NewClient(cliOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create jsonrpc client: %w", err)
	}
	conn, err := cli.NewConnection(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create jsonrpc client connection: %w", err)
	}

	hCli := config.HertzClient
	if hCli == nil && config.HTTPClient == nil {
		hCli, _ = hertz_client.NewClient(hertz_client.WithDialTimeout(consts.DefaultDialTimeout))
	}

	agentCardURL, err := url.JoinPath(config.BaseURL, agentCardPath)
	if err != nil {
		return nil, fmt.Errorf("failed to join agent card url: %w", err)
	}

	codec, err := wire.NewCodec(config.ProtocolVersion)
	if err != nil {
		return nil, err
	}

	return &Transport{
		agentCardURL: agentCardURL,
		conn:         conn,
		hCli:         hCli,
		cli:          config.HTTPClient,
		codec:        codec,
	}, nil
}

type Transport struct {
	agentCardURL string
	conn         core.Connection
	hCli         *hertz_client.Client
	cli          *std_http.Client
	codec        wire.Codec
}

// withVersionHeader tags the outgoing request with the A2A-Version header so a
// version-aware server can pick the matching decoder. v0.3 servers ignore the
// unknown header, and an absent header is interpreted as v0.3 anyway, so this
// is safe against legacy peers.
func (t *Transport) withVersionHeader(ctx context.Context) context.Context {
	return metadata.WithValue(ctx, models.HeaderA2AVersion, string(t.codec.Version()))
}

func (t *Transport) AgentCard(ctx context.Context) (*models.AgentCard, error) {
	var code int
	var body []byte
	var err error
	if t.hCli != nil {
		code, body, err = t.hCli.Get(ctx, nil, t.agentCardURL)
		if err != nil {
			return nil, fmt.Errorf("failed to get agent card: %w", err)
		}
	} else {
		resp, err := t.cli.Get(t.agentCardURL)
		if err != nil {
			return nil, fmt.Errorf("failed to get agent card: %w", err)
		}
		code = resp.StatusCode
		body, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read agent card: %w", err)
		}
		_ = resp.Body.Close()
	}
	if code != std_http.StatusOK && code != std_http.StatusAccepted {
		return nil, fmt.Errorf("failed to get agent card, code: %d, body: %s", code, string(body))
	}

	card := &models.AgentCard{}
	err = json.Unmarshal(body, card)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal agent card: %w, body: %s", err, string(body))
	}
	return card, nil
}

func (t *Transport) SendMessage(ctx context.Context, params *models.MessageSendParams) (*models.SendMessageResponseUnion, error) {
	ctx = t.withVersionHeader(ctx)
	reqBody, err := t.codec.EncodeMessageSendParams(params)
	if err != nil {
		return nil, fmt.Errorf("failed to encode send message params: %w", err)
	}
	var b json.RawMessage
	if err = t.conn.Call(ctx, t.codec.Methods().Send, reqBody, &b); err != nil {
		return nil, fmt.Errorf("failed to send message: %w", err)
	}
	su, err := t.codec.DecodeStreamingUnion(b)
	if err != nil {
		return nil, err
	}
	if su == nil {
		return nil, nil
	}
	// A non-streaming send only ever yields a Message or a Task. If the peer
	// returned a status/artifact update frame, surface it rather than silently
	// returning an empty union.
	if su.Message == nil && su.Task == nil {
		return nil, fmt.Errorf("unexpected send message response: not a message or task")
	}
	return &models.SendMessageResponseUnion{Message: su.Message, Task: su.Task}, nil
}

func (t *Transport) SendMessageStreaming(ctx context.Context, params *models.MessageSendParams) (models.ResponseReader, error) {
	ctx = t.withVersionHeader(ctx)
	reqBody, err := t.codec.EncodeMessageSendParams(params)
	if err != nil {
		return nil, fmt.Errorf("failed to encode send message params: %w", err)
	}
	stream, err := t.conn.AsyncCall(ctx, t.codec.Methods().Stream, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to send message: %w", err)
	}
	return &frameReader{a: stream, codec: t.codec}, nil
}

func (t *Transport) GetTask(ctx context.Context, params *models.TaskQueryParams) (*models.Task, error) {
	ctx = t.withVersionHeader(ctx)
	reqBody, err := t.codec.EncodeTaskQueryParams(params)
	if err != nil {
		return nil, err
	}
	var b json.RawMessage
	if err = t.conn.Call(ctx, t.codec.Methods().GetTask, reqBody, &b); err != nil {
		return nil, err
	}
	return t.codec.DecodeTask(b)
}

func (t *Transport) CancelTask(ctx context.Context, params *models.TaskIDParams) (*models.Task, error) {
	ctx = t.withVersionHeader(ctx)
	reqBody, err := t.codec.EncodeTaskIDParams(params)
	if err != nil {
		return nil, err
	}
	var b json.RawMessage
	if err = t.conn.Call(ctx, t.codec.Methods().Cancel, reqBody, &b); err != nil {
		return nil, err
	}
	return t.codec.DecodeTask(b)
}

func (t *Transport) ResubscribeTask(ctx context.Context, params *models.TaskIDParams) (models.ResponseReader, error) {
	ctx = t.withVersionHeader(ctx)
	reqBody, err := t.codec.EncodeTaskIDParams(params)
	if err != nil {
		return nil, err
	}
	ret, err := t.conn.AsyncCall(ctx, t.codec.Methods().Resubscribe, reqBody)
	if err != nil {
		return nil, err
	}
	return &frameReader{a: ret, codec: t.codec}, nil
}

func (t *Transport) SetPushNotificationConfig(ctx context.Context, params *models.TaskPushNotificationConfig) (*models.TaskPushNotificationConfig, error) {
	ctx = t.withVersionHeader(ctx)
	reqBody, err := t.codec.EncodeTaskPushNotificationConfig(params)
	if err != nil {
		return nil, err
	}
	var b json.RawMessage
	if err = t.conn.Call(ctx, t.codec.Methods().PushSet, reqBody, &b); err != nil {
		return nil, err
	}
	return t.codec.DecodeTaskPushNotificationConfig(b)
}

func (t *Transport) GetPushNotificationConfig(ctx context.Context, params *models.GetTaskPushNotificationConfigParams) (*models.TaskPushNotificationConfig, error) {
	ctx = t.withVersionHeader(ctx)
	reqBody, err := t.codec.EncodeGetPushParams(params)
	if err != nil {
		return nil, err
	}
	var b json.RawMessage
	if err = t.conn.Call(ctx, t.codec.Methods().PushGet, reqBody, &b); err != nil {
		return nil, err
	}
	return t.codec.DecodeTaskPushNotificationConfig(b)
}

func (t *Transport) Close() error {
	return nil
}

type frameReader struct {
	a     core.ClientAsync
	codec wire.Codec
}

func (f *frameReader) Read() (*models.SendMessageStreamingResponseUnion, error) {
	var b json.RawMessage
	err := f.a.Recv(context.Background(), &b)
	if err != nil {
		if err == io.EOF {
			return nil, io.EOF
		}
		return nil, fmt.Errorf("failed to read frame: %w", err)
	}
	return f.codec.DecodeStreamingUnion(b)
}

func (f *frameReader) Close() error {
	return f.a.Close()
}

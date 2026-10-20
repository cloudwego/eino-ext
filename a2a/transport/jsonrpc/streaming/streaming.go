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

package streaming

import (
	"context"

	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc/core"
	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc/pkg/tracer"
)

// Extension is a direct user-facing interface that extends the semantics of JSON-RPC2 Ping-Pong to gRPC-like Stream.
// The req/resp involved in each method call has the same ID.
//
// # ClientStreaming and BidiStreaming would be supported when there is a clear usage scenario in the future.
//
// ServerStreaming:
//
//	Client => req(id=1) => Server
//	Client <= resp(id=1) <= Server
//	Client <= resp(id=1) <= Server
//	            ...
type Extension interface {
	ServerStreaming(ctx context.Context, method string, req interface{}, opts ...core.CallOption) (ServerStreamingClient, error)
}

type ServerStreamingClient interface {
	Recv(ctx context.Context, obj interface{}) error
	Close() error
}

type serverStreamingClient struct {
	async  core.ClientAsync
	tracer tracer.Tracer
}

func (c *serverStreamingClient) Recv(ctx context.Context, obj interface{}) error {
	// todo: check err and do tracing
	return c.async.Recv(ctx, obj)
}

func (c *serverStreamingClient) Close() error {
	return c.async.Close()
}

type extension struct {
	conn     core.Connection
	ssCallEp ServerStreamingCallEndpoint
	tracer   tracer.Tracer
}

func (ext *extension) ServerStreaming(ctx context.Context, method string, req interface{}, opts ...core.CallOption) (ServerStreamingClient, error) {
	ctx = ext.tracer.Start(ctx)
	return ext.ssCallEp(ctx, method, req)
}

func (ext *extension) serverStreamingEndpoint(ctx context.Context, method string, req interface{}) (ServerStreamingClient, error) {
	async, err := ext.conn.AsyncCall(ctx, method, req)
	if err != nil {
		return nil, err
	}
	return &serverStreamingClient{
		async: async,
	}, nil
}

func NewExtension(conn core.Connection, opts ...Option) (Extension, error) {
	options := new(Options)
	for _, opt := range opts {
		opt(options)
	}
	ext := &extension{conn: conn}
	ext.ssCallEp = serverStreamingCallChain(options.ssCallMWs...)(ext.serverStreamingEndpoint)
	getter, ok := conn.(tracer.Getter)
	if ok {
		ext.tracer = getter.GetTracer()
	} else {
		ext.tracer = tracer.NewNoopTracer()
	}
	return ext, nil
}

type ServerStreamingServer interface {
	Send(ctx context.Context, obj interface{}) error
}

type serverStreamingServer struct {
	async core.ServerAsync
}

func (s *serverStreamingServer) Send(ctx context.Context, obj interface{}) error {
	return s.async.SendStreaming(ctx, obj)
}

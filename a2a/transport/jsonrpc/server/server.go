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
	"context"

	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc/core"
	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc/pkg/transport"
	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc/streaming"
)

type Server struct {
	builder transport.ServerTransportBuilder
	hdl     *jsonRPCHandler
}

func NewServer(opts ...Option) *Server {
	options := buildOptions(opts...)
	return &Server{
		builder: options.transBuilder,
		hdl:     newJsonRPCHandler(options),
	}
}

func (srv *Server) Run(ctx context.Context) error {
	trans, err := srv.builder.Build(ctx, srv.hdl)
	if err != nil {
		return err
	}
	return trans.ListenAndServe(ctx)
}

type jsonRPCHandler struct {
	options *Options
}

func NewServerTransportHandler(opts ...Option) (transport.ServerTransportHandler, error) {
	options := buildOptions(opts...)
	return newJsonRPCHandler(options), nil
}

func newJsonRPCHandler(options *Options) *jsonRPCHandler {
	return &jsonRPCHandler{options: options}
}

func (hdl *jsonRPCHandler) OnTransport(ctx context.Context, trans core.Transport) error {
	options := hdl.options
	var opts []core.Option
	for method, ppHdl := range options.ppHdls {
		opts = append(opts, core.WithHandler(method, ppHdl))
	}
	var ssOpts []streaming.Option
	for method, ssHdl := range options.ssHdls {
		ssOpts = append(ssOpts, streaming.WithServerStreamingHandler(method, ssHdl))
	}
	opts = append(opts, streaming.ConvertConnectionOption(ssOpts...)...)

	if cliTrans, ok := trans.ClientCapability(); ok {
		opts = append(opts, core.WithClientRounder(cliTrans))
	}
	if srvTrans, ok := trans.ServerCapability(); ok {
		opts = append(opts, core.WithServerRounder(srvTrans))
	}
	ctx, _, err := core.NewConnection(ctx, opts...)
	if err != nil {
		return err
	}
	return nil
}

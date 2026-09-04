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

package http

import (
	"context"
	"strconv"
	"sync"

	"github.com/cloudwego/hertz/pkg/app"
	hz_server "github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/protocol/sse"

	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc/pkg/metadata"
	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc/pkg/transport"
	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc/pkg/utils"

	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc/core"
)

type ServerTransportBuilderOptions struct {
	hz  *hz_server.Hertz
	hdl transport.ServerTransportHandler
}

type ServerTransportBuilderOption func(*ServerTransportBuilderOptions)

func WithHertzIns(hz *hz_server.Hertz) ServerTransportBuilderOption {
	return func(o *ServerTransportBuilderOptions) {
		o.hz = hz
	}
}

func WithServerTransportHandler(hdl transport.ServerTransportHandler) ServerTransportBuilderOption {
	return func(o *ServerTransportBuilderOptions) {
		o.hdl = hdl
	}
}

func NewServerTransportBuilder(path string, opts ...ServerTransportBuilderOption) *ServerTransportBuilder {
	options := ServerTransportBuilderOptions{}
	for _, opt := range opts {
		opt(&options)
	}
	return &ServerTransportBuilder{
		path:    path,
		hdl:     options.hdl,
		rounder: newServerRounder(),
		hzIns:   options.hz,
	}
}

type ServerTransportBuilder struct {
	path    string
	hdl     transport.ServerTransportHandler
	rounder *serverRounder
	once    sync.Once
	hzIns   *hz_server.Hertz
}

func (s *ServerTransportBuilder) Build(ctx context.Context, hdl transport.ServerTransportHandler) (transport.ServerTransport, error) {
	hz := s.hzIns
	if hz == nil {
		// sense the disconnection by default
		hz = hz_server.Default(hz_server.WithSenseClientDisconnection(true))
	}
	hz.POST(s.path, s.POST)
	s.hdl = hdl
	return &server{hz: hz}, nil
}

func (s *ServerTransportBuilder) POST(c context.Context, ctx *app.RequestContext) {
	msg, err := core.ReadMessage(ctx.Request.Body())
	if err != nil {
		ctx.JSON(400, core.NewFailureResponse(core.ID{}, core.ErrorInvalidRequest))
		return
	}
	s.once.Do(func() {
		s.hdl.OnTransport(c, &httpServerTransport{rounder: s.rounder})
	})
	ctx.VisitAllHeaders(func(key, val []byte) {
		c = metadata.WithValue(c, string(key), string(val))
	})
	// wait for handler return
	<-s.rounder.newRound(c, msg, ctx)
}

type server struct {
	hz *hz_server.Hertz
}

func (s *server) ListenAndServe(ctx context.Context) error {
	return s.hz.Run()
}

func (s *server) Shutdown(ctx context.Context) error {
	return s.hz.Shutdown(ctx)
}

type serverRounder struct {
	buf *utils.UnboundBuffer[roundMeta]
}

func newServerRounder() *serverRounder {
	return &serverRounder{buf: utils.NewUnboundBuffer[roundMeta]()}
}

type roundMeta struct {
	ctx    context.Context
	msg    core.Message
	writer *writer
}

func (s *serverRounder) newRound(ctx context.Context, msg core.Message, hzCtx *app.RequestContext) <-chan struct{} {
	w := newWriter(hzCtx)
	s.buf.Push(roundMeta{
		ctx:    ctx,
		msg:    msg,
		writer: w,
	})
	return w.finishCh
}

func (s *serverRounder) OnRound() (context.Context, core.Message, core.MessageWriter, error) {
	meta := <-s.buf.PopChan()
	s.buf.Load()
	return meta.ctx, meta.msg, meta.writer, nil
}

type writer struct {
	ctx       *app.RequestContext
	sseWriter *sse.Writer
	id        int
	finishCh  chan struct{}
	isClosed  bool
}

func newWriter(hzCtx *app.RequestContext) *writer {
	return &writer{
		ctx:      hzCtx,
		finishCh: make(chan struct{}),
	}
}

func (w *writer) WriteStreaming(ctx context.Context, msg core.Message) error {
	if w.sseWriter == nil {
		w.sseWriter = sse.NewWriter(w.ctx)
	}
	return w.writeSSE(msg)
}

func (w *writer) writeSSE(msg core.Message) error {
	buf, err := core.Marshal(msg)
	if err != nil {
		return err
	}
	w.id += 1
	if err = w.sseWriter.WriteEvent(strconv.Itoa(w.id), "message", buf); err != nil {
		return err
	}
	return nil
}

func (w *writer) Close() error {
	if w.isClosed {
		return nil
	}
	w.isClosed = true
	close(w.finishCh)
	return nil
}

func (w *writer) Write(ctx context.Context, msg core.Message) error {
	w.ctx.JSON(200, msg)
	w.Close()
	return nil
}

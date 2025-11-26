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
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/cloudwego/hertz/pkg/app/client"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/cloudwego/hertz/pkg/protocol/sse"

	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc/core"
	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc/pkg/conninfo"
	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc/pkg/metadata"
	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc/pkg/transport"
	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc/pkg/utils"
)

type ClientTransportBuilderOptions struct {
	hCli       *client.Client
	cli        *http.Client
	sseBufSize *int
}
type ClientTransportBuilderOption func(*ClientTransportBuilderOptions)

func WithHertzClient(cli *client.Client) ClientTransportBuilderOption {
	return func(o *ClientTransportBuilderOptions) {
		o.hCli = cli
	}
}

func WithHTTPClient(cli *http.Client) ClientTransportBuilderOption {
	return func(o *ClientTransportBuilderOptions) {
		o.cli = cli
	}
}

// WithSSEBufferSize specifies the maximum buffer size will be used in SSE Reader Processing.
// if size <= 0, then default maximum size (64 * 1024) would be used.
func WithSSEBufferSize(size int) ClientTransportBuilderOption {
	return func(o *ClientTransportBuilderOptions) {
		o.sseBufSize = &size
	}
}

type clientTransportHandler struct {
	hCli       *client.Client
	cli        *http.Client
	sseBufSize *int
}

func NewClientTransportHandler(opts ...ClientTransportBuilderOption) transport.ClientTransportHandler {
	o := &ClientTransportBuilderOptions{}
	for _, opt := range opts {
		opt(o)
	}
	h := &clientTransportHandler{
		hCli:       o.hCli,
		cli:        o.cli,
		sseBufSize: o.sseBufSize,
	}
	if h.hCli == nil && h.cli == nil {
		cli, _ := client.NewClient(client.WithDialTimeout(consts.DefaultDialTimeout))
		h.hCli = cli
	}
	return h
}

func (c *clientTransportHandler) NewTransport(ctx context.Context, peer conninfo.Peer) (core.Transport, error) {
	addr := peer.Address()
	var r rounder
	if c.hCli != nil {
		r = newHertzClientRounder(addr, c.hCli, c.sseBufSize)
	} else {
		r = newHTTPClientRounder(addr, c.cli, c.sseBufSize)
	}
	return &httpClientTransport{rounder: r}, nil
}

type rounder interface {
	Round(ctx context.Context, msg core.Message) (core.MessageReader, error)
}

type sseReaderInt interface {
	ForEach(ctx context.Context, f func(e *sse.Event) error) error
}

type httpClientRounder struct {
	cli        *http.Client
	addr       string
	sseBufSize *int
}

func newHTTPClientRounder(addr string, cli *http.Client, sseBufSize *int) *httpClientRounder {
	return &httpClientRounder{
		cli:        cli,
		addr:       addr,
		sseBufSize: sseBufSize,
	}
}

func (c *httpClientRounder) Round(ctx context.Context, msg core.Message) (core.MessageReader, error) {
	buf, err := core.EncodeMessage(msg)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.addr, bytes.NewBuffer(buf))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}
	req.Header.Set("Accept", "application/json,text/event-stream")
	md, ok := metadata.GetAllValues(ctx)
	if ok {
		for k, v := range md {
			req.Header.Set(k, v)
		}
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.cli.Do(req)
	if err != nil {
		return nil, err
	}

	status := resp.StatusCode
	if status != consts.StatusOK {
		// return specific error based on status code
		switch status {
		case consts.StatusNotFound:
			resp.Body.Close()
			return nil, fmt.Errorf("url path %s not found", c.addr)
		default:
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("unexpected status code %d, body: %s", status, string(body))
		}
	}
	ct := resp.Header.Get("content-type")
	switch {
	case strings.Contains(ct, "application/json"):
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return &pingPongReader{
			resp: body,
		}, nil
	case strings.Contains(ct, "text/event-stream"):
		// Don't close resp.Body here - it will be closed by sseReader
		r := newSSEReader(resp.Body)
		if c.sseBufSize != nil {
			r.SetMaxBufferSize(*c.sseBufSize)
		}
		sr := &sseReader{
			ctx:    ctx,
			reader: r,
			buf:    utils.NewUnboundBuffer[sseData](),
		}
		sr.run()
		return sr, nil
	default:
		resp.Body.Close()
		return nil, fmt.Errorf("non-expected content-type: %s, status-code: %d", ct, status)
	}
}

type hertzClientRounder struct {
	cli        *client.Client
	addr       string
	sseBufSize *int
}

func newHertzClientRounder(addr string, cli *client.Client, sseBufSize *int) *hertzClientRounder {
	return &hertzClientRounder{
		addr:       addr,
		cli:        cli,
		sseBufSize: sseBufSize,
	}
}

func (c *hertzClientRounder) Round(ctx context.Context, msg core.Message) (core.MessageReader, error) {
	buf, err := core.EncodeMessage(msg)
	if err != nil {
		return nil, err
	}
	req := &protocol.Request{}
	resp := &protocol.Response{}
	req.SetMethod(consts.MethodPost)
	req.SetRequestURI(c.addr)
	req.SetHeader("Accept", "application/json,text/event-stream")
	md, ok := metadata.GetAllValues(ctx)
	if ok {
		for k, v := range md {
			req.SetHeader(k, v)
		}
	}
	req.Header.SetContentTypeBytes([]byte("application/json"))
	req.SetBody(buf)
	if err = c.cli.Do(ctx, req, resp); err != nil {
		return nil, err
	}
	status := resp.StatusCode()
	if status != consts.StatusOK {
		// return specific error based on status code
		switch status {
		case consts.StatusNotFound:
			return nil, fmt.Errorf("url path %s not found", c.addr)
		default:
			return nil, fmt.Errorf("unexpected status code %d, body: %s", status, string(resp.Body()))
		}
	}
	ct := string(resp.Header.ContentType())
	switch {
	case strings.Contains(ct, "application/json"):
		return &pingPongReader{
			resp: resp.Body(),
		}, nil
	case strings.Contains(ct, "text/event-stream"):
		r, _ := sse.NewReader(resp)
		if c.sseBufSize != nil {
			r.SetMaxBufferSize(*c.sseBufSize)
		}
		sr := &sseReader{
			ctx:    ctx,
			reader: r,
			buf:    utils.NewUnboundBuffer[sseData](),
		}
		sr.run()
		return sr, nil
	default:
		return nil, fmt.Errorf("non-expected content-type: %s, status-code: %d", ct, status)
	}
}

type pingPongReader struct {
	resp     []byte
	isFinish bool
}

func (r *pingPongReader) Read(ctx context.Context) (core.Message, error) {
	if r.isFinish {
		return nil, io.EOF
	}
	// todo: think about batch
	msgs, _, err := core.DecodeMessages(r.resp)
	if err != nil {
		return nil, err
	}
	r.isFinish = true
	return msgs[0], nil
}

func (r *pingPongReader) Close() error {
	if r.isFinish {
		return nil
	}
	r.resp = nil
	r.isFinish = true
	return nil
}

type sseReader struct {
	ctx        context.Context
	reader     sseReaderInt
	buf        *utils.UnboundBuffer[sseData]
	err        error
	cancelFunc context.CancelFunc
}

type sseData struct {
	event *sse.Event
	err   error
}

func (s *sseReader) run() {
	ctx, cancel := context.WithCancel(s.ctx)
	s.cancelFunc = cancel
	go func() {
		err := s.reader.ForEach(ctx, func(e *sse.Event) error {
			s.buf.Push(sseData{
				event: e.Clone(),
			})
			return nil
		})
		if err == nil {
			err = io.EOF
		}
		s.buf.Push(sseData{
			err: err,
		})
	}()
}

func (s *sseReader) Read(ctx context.Context) (core.Message, error) {
	if s.err != nil {
		return nil, s.err
	}
	data := <-s.buf.PopChan()
	defer s.buf.Load()
	if data.err != nil {
		s.err = data.err
		s.cancelFunc()
		return nil, s.err
	}
	msgs, _, err := core.DecodeMessages(data.event.Data)
	if err != nil {
		return nil, err
	}
	return msgs[0], nil
}

func (s *sseReader) Close() error {
	if s.err != nil { // which means the lifecycle of sse.Reader has finished
		return nil
	}
	s.err = io.EOF
	s.cancelFunc()
	// Close the underlying reader to release the HTTP response body
	//return s.reader.Close()
	return nil
}

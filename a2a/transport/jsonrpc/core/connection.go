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

package core

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc/pkg/tracer"
)

func NewConnection(ctx context.Context, opts ...Option) (context.Context, Connection, error) {
	options := defaultOptions()
	for _, opt := range opts {
		opt(&options)
	}
	// verify
	if options.cliTrans == nil && options.srvTrans == nil {
		return ctx, nil, errors.New("ClientRounder and ServerRounder cannot both be nil")
	}

	conn := &connection{
		tracer:    options.tracer,
		outbounds: make(map[string]*rpcCall),
	}

	if options.cliTrans != nil {
		conn.cliTrans = options.cliTrans
		conn.callEp = callChain(options.callMWs...)(conn.callEndpoint)
	}

	if options.srvTrans != nil {
		conn.srvTrans = options.srvTrans
		conn.hdls = make(map[string]requestHandlerFunc)
		// PingPongHandler
		for method, hdl := range options.ppHdls {
			conn.hdls[method] = conn.getRequestHandlerFunc(handleChain(options.hdlMWs...)(hdl))
		}
		// RequestHandler
		for method, hdl := range options.reqHdls {
			// todo: provide requestHandler middleware
			conn.hdls[method] = requestHandlerFunc(hdl)
		}
		// NotificationHandler
		conn.notifHdls = make(map[string]NotificationHandleEndpoint)
		for method, hdl := range options.notifHdls {
			conn.notifHdls[method] = hdl
		}
		go conn.roundLoop()
	}

	return ctx, conn, nil
}

func (conn *connection) getRequestHandlerFunc(ep HandleEndpoint) requestHandlerFunc {
	return func(ctx context.Context, conn Connection, req *Request, async ServerAsync) (err error) {
		defer func() {
			if rawPanic := recover(); rawPanic != nil {
				err = ConvertError(rawPanic.(error))
				async.Finish(ctx, err)
			}
		}()
		resp, epErr := ep(ctx, conn, req.Params)
		if epErr != nil {
			err = ConvertError(epErr)
			async.Finish(ctx, err)
			return
		}
		if sErr := async.Send(ctx, resp); sErr != nil {
			return sErr
		}
		return async.Finish(ctx, nil)
	}
}

func ConvertError(rawErr error) *Error {
	// todo: using errors api
	if err, ok := rawErr.(*Error); ok {
		return err
	}
	return NewError(InternalErrorCode, rawErr.Error(), nil)
}

type connection struct {
	hdls      map[string]requestHandlerFunc
	notifHdls map[string]NotificationHandleEndpoint

	callEp CallEndpoint
	tracer tracer.Tracer

	mu        sync.Mutex
	cliTrans  ClientRounder
	srvTrans  ServerRounder
	outbounds map[string]*rpcCall
}

func (conn *connection) Call(ctx context.Context, method string, req, res interface{}, opts ...CallOption) error {
	ctx = conn.tracer.Start(ctx)
	defer func() {
		// todo: inject error
		conn.tracer.Finish(ctx)
	}()
	return conn.callEp(ctx, method, req, res)
}

func (conn *connection) callEndpoint(ctx context.Context, method string, req, resp interface{}) error {
	newCall, err := conn.asyncCall(ctx, method, req)
	if err != nil {
		return err
	}
	defer newCall.Close()
	if err = newCall.Recv(ctx, resp); err != nil {
		return err
	}
	return nil
}

func (conn *connection) AsyncCall(ctx context.Context, method string, req interface{}, opts ...CallOption) (ClientAsync, error) {
	return conn.asyncCall(ctx, method, req)
}

func (conn *connection) asyncCall(ctx context.Context, method string, req interface{}) (*rpcCall, error) {
	newId := allocateId()
	newCall := &rpcCall{
		id:     newId,
		ctx:    ctx,
		method: method,
		conn:   conn,
	}
	msg, err := NewRequest(method, newId, req)
	if err != nil {
		return nil, err
	}
	reader, err := conn.sendRequest(ctx, msg)
	if err != nil {
		return nil, err
	}
	newCall.reader = reader
	conn.mu.Lock()
	conn.outbounds[newId.String()] = newCall
	conn.mu.Unlock()
	return newCall, nil
}

func (conn *connection) Notify(ctx context.Context, method string, params interface{}, opts ...CallOption) error {
	notif, err := NewNotification(method, params)
	if err != nil {
		return err
	}
	return conn.sendNotification(ctx, notif)
}

func (conn *connection) handleRequest(ctx context.Context, req *Request, async ServerAsync) error {
	method := req.Method
	hdl, ok := conn.hdls[method]
	if !ok {
		return async.Finish(ctx, NewError(MethodNotFoundCode, fmt.Sprintf("Method %s not found", method), nil))
	}
	return hdl(ctx, conn, req, async)
}

func (conn *connection) handleNotification(ctx context.Context, notif *Notification) {
	method := notif.Method
	hdl, ok := conn.notifHdls[method]
	if !ok {
		// todo: log
		return
	}
	// ignore the error now
	hdl(ctx, conn, notif.Params)
}

func (conn *connection) GetTracer() tracer.Tracer {
	return conn.tracer
}

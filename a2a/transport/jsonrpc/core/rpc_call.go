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

	"github.com/google/uuid"
)

func (conn *connection) roundLoop() error {
	for {
		nCtx, msg, writer, err := conn.srvTrans.OnRound()
		if err != nil {
			return err
		}
		switch msg.Type() {
		case ObjectTypeRequest:
			req := msg.(*Request)
			// todo: implement timeout
			newCtx, cancel := context.WithCancel(nCtx)
			call := &rpcCall{
				id:     req.ID,
				ctx:    newCtx,
				cancel: cancel,
				req:    req,
				writer: writer,
				conn:   conn,
			}
			go func() {
				call.handleRequest()
			}()
		case ObjectTypeNotification:
			// there is no need to care about writer
			notif := msg.(*Notification)
			go func() {
				conn.handleNotification(nCtx, notif)
			}()
		}
	}
}

func (conn *connection) sendRequest(ctx context.Context, msg Message) (MessageReader, error) {
	conn.mu.Lock()
	defer conn.mu.Unlock()
	reader, err := conn.cliTrans.Round(ctx, msg)
	if err != nil {
		return nil, err
	}
	return reader, nil
}

func (conn *connection) sendNotification(ctx context.Context, msg Message) error {
	conn.mu.Lock()
	defer conn.mu.Unlock()
	// there is no need to care about reader
	if _, err := conn.cliTrans.Round(ctx, msg); err != nil {
		return err
	}
	return nil
}

func (conn *connection) deleteOutBound(id ID) {
	conn.mu.Lock()
	defer conn.mu.Unlock()
	delete(conn.outbounds, id.String())
}

func allocateId() ID {
	return NewIDFromString(uuid.New().String())
}

var _ ClientAsync = &rpcCall{}

type rpcCall struct {
	id     ID
	ctx    context.Context
	cancel context.CancelFunc
	method string
	req    *Request
	// for client-side
	reader MessageReader
	// for server-side
	writer MessageWriter
	conn   *connection
}

func (c *rpcCall) ID() ID {
	return c.id
}

func (c *rpcCall) SendStreaming(ctx context.Context, obj interface{}) error {
	msg, err := NewResponse(c.id, obj)
	if err != nil {
		return err
	}
	return c.writer.WriteStreaming(ctx, msg)
}

func (c *rpcCall) FinishStreaming(ctx context.Context, err error) error {
	if err == nil {
		return c.writer.Close()
	}
	jErr := err.(*Error)
	msg := NewFailureResponse(c.id, jErr)
	if wErr := c.writer.WriteStreaming(c.ctx, msg); wErr != nil {
		return wErr
	}
	return c.writer.Close()
}

func (c *rpcCall) Send(ctx context.Context, obj interface{}) error {
	msg, err := NewResponse(c.id, obj)
	if err != nil {
		return err
	}
	return c.writer.Write(ctx, msg)
}

func (c *rpcCall) Finish(ctx context.Context, err error) error {
	if err == nil {
		return c.writer.Close()
	}
	jErr := err.(*Error)
	msg := NewFailureResponse(c.id, jErr)
	if wErr := c.writer.Write(c.ctx, msg); wErr != nil {
		return wErr
	}
	return c.writer.Close()
}

func (c *rpcCall) Recv(ctx context.Context, obj interface{}) error {
	msg, err := c.reader.Read(ctx)
	if err != nil {
		return err
	}
	if msg.Type() != ObjectTypeResponse {
		err = errors.New("not a response")
		c.Close()
		return err
	}
	resp := msg.(*Response)
	if resp.Err != nil {
		c.Close()
		return resp.Err
	}
	params := resp.Result
	if err = Unmarshal(params, obj); err != nil {
		c.Close()
		return err
	}
	return nil
}

func (c *rpcCall) Close() error {
	c.conn.deleteOutBound(c.id)
	if c.writer != nil {
		return c.writer.Close()
	}
	if c.reader != nil {
		return c.reader.Close()
	}
	return nil
}

func (c *rpcCall) handleRequest() {
	var err error
	defer func() {
		if err != nil {
			c.Close()
		}
	}()
	if err = c.conn.handleRequest(c.ctx, c.req, c); err != nil {
		return
	}
}

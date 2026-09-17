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
)

const (
	version = "2.0"
)

// Connection is a direct user-facing interface that implements the semantics of JSON-RPC2
type Connection interface {
	Call(ctx context.Context, method string, req, res interface{}, opts ...CallOption) error
	AsyncCall(ctx context.Context, method string, req interface{}, opts ...CallOption) (ClientAsync, error)
	Notify(ctx context.Context, method string, params interface{}, opts ...CallOption) error
}

type ClientAsync interface {
	// Recv returns io.EOF when finished
	Recv(ctx context.Context, obj interface{}) error
	Close() error
}

type ServerAsync interface {
	SendStreaming(ctx context.Context, obj interface{}) error
	FinishStreaming(ctx context.Context, err error) error
	Send(ctx context.Context, obj interface{}) error
	Finish(ctx context.Context, err error) error
}

type requestHandlerFunc func(ctx context.Context, conn Connection, req *Request, async ServerAsync) error

type Transport interface {
	ClientCapability() (ClientRounder, bool)
	ServerCapability() (ServerRounder, bool)
}

// ClientRounder is used to send a jsonrpc Message(Request or Notification)
// to remote side and make use of MessageReader to receive jsonrpc Message(s) (Response or Notification).
//
// Usually, there is no need to return a substantive MessageReader when sending a jsonrpc Notification.
type ClientRounder interface {
	Round(ctx context.Context, msg Message) (MessageReader, error)
}

// MessageReader can express the semantics of either Ping-Pong reader or ServerStreaming reader.
//   - Ping-Pong reader:
//     Read -> Message, nil
//     Read -> nil, io.EOF
//   - ServerStreaming reader:
//     Read -> Message, nil
//     ...
//     Read -> nil, io.EOF
type MessageReader interface {
	// Read returns io.EOF and nil Message when finished
	Read(ctx context.Context) (Message, error)
	// Close terminates MessageReader directly.
	// Usually Read returns io.EOF or some other error, and the MessageReader's lifecycle ends naturally.
	Close() error
}

// ServerRounder is used to accept a jsonrpc Message(Request or Notification)
// from remote side and make use of MessageWriter to response jsonrpc Message(s) (Response or Notification).
//
// Usually, there is no need to return a substantive MessageWriter when receiving a jsonrpc Notification.
type ServerRounder interface {
	OnRound() (context.Context, Message, MessageWriter, error)
}

type MessageWriter interface {
	WriteStreaming(ctx context.Context, msg Message) error
	// Close is used to finish the series of WriteStreaming
	Close() error
	Write(ctx context.Context, msg Message) error
}

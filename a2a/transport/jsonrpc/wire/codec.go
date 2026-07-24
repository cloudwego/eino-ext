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

// Package wire isolates the A2A JSON-RPC wire format behind a version-aware
// Codec. The Go-facing model structs (package models) always stay in their
// v0.3 shape; a Codec is responsible for translating those structs to/from the
// concrete JSON that goes on the wire for a given ProtocolVersion.
//
// This lets a single eino-ext build speak both A2A v0.3 and v1.0: the transport
// picks a Codec per protocol version, and everything above the transport
// (handlers, client API, models) is unaware of the version.
package wire

import (
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino-ext/a2a/models"
)

// Methods holds the JSON-RPC method-name strings for one protocol version.
// v0.3 and v1.0 use disjoint names, which is what allows a server to register
// both versions on a single endpoint and dispatch purely by method name.
type Methods struct {
	Send        string
	Stream      string
	GetTask     string
	Cancel      string
	Resubscribe string
	PushSet     string
	PushGet     string
}

var methodsV03 = Methods{
	Send:        "message/send",
	Stream:      "message/stream",
	GetTask:     "tasks/get",
	Cancel:      "tasks/cancel",
	Resubscribe: "tasks/resubscribe",
	PushSet:     "tasks/pushNotificationConfig/set",
	PushGet:     "tasks/pushNotificationConfig/get",
}

// methodsV10 aligns JSON-RPC method names with the a2a.proto service, per A2A
// v1.0. TODO(pcap): confirm the exact push-notification-config method names
// against a reference v1.0 server.
var methodsV10 = Methods{
	Send:        "SendMessage",
	Stream:      "SendStreamingMessage",
	GetTask:     "GetTask",
	Cancel:      "CancelTask",
	Resubscribe: "SubscribeToTask",
	PushSet:     "CreateTaskPushNotificationConfig",
	PushGet:     "GetTaskPushNotificationConfig",
}

// Codec encodes A2A request/response payloads to wire JSON and decodes them
// back into the shared models types. All methods deal in json.RawMessage so the
// result can be handed straight to the generic JSON-RPC core, which emits a
// RawMessage verbatim.
type Codec interface {
	Version() models.ProtocolVersion
	Methods() Methods

	// Request params: the client encodes, the server decodes.
	EncodeMessageSendParams(*models.MessageSendParams) (json.RawMessage, error)
	DecodeMessageSendParams([]byte) (*models.MessageSendParams, error)
	EncodeTaskQueryParams(*models.TaskQueryParams) (json.RawMessage, error)
	DecodeTaskQueryParams([]byte) (*models.TaskQueryParams, error)
	EncodeTaskIDParams(*models.TaskIDParams) (json.RawMessage, error)
	DecodeTaskIDParams([]byte) (*models.TaskIDParams, error)
	EncodeTaskPushNotificationConfig(*models.TaskPushNotificationConfig) (json.RawMessage, error)
	DecodeTaskPushNotificationConfig([]byte) (*models.TaskPushNotificationConfig, error)
	EncodeGetPushParams(*models.GetTaskPushNotificationConfigParams) (json.RawMessage, error)
	DecodeGetPushParams([]byte) (*models.GetTaskPushNotificationConfigParams, error)

	// Task result: the server encodes, the client decodes.
	EncodeTask(*models.Task) (json.RawMessage, error)
	DecodeTask([]byte) (*models.Task, error)

	// Send / streaming response union: the server encodes each frame, the
	// client decodes each frame.
	EncodeStreamingUnion(*models.SendMessageStreamingResponseUnion) (json.RawMessage, error)
	DecodeStreamingUnion([]byte) (*models.SendMessageStreamingResponseUnion, error)
}

// NewCodec returns the Codec for the given protocol version. An empty version
// defaults to v0.3, mirroring the server-side interpretation of an absent
// A2A-Version header.
func NewCodec(v models.ProtocolVersion) (Codec, error) {
	switch v {
	case "", models.ProtocolVersion03:
		return v03Codec{}, nil
	case models.ProtocolVersion10:
		return v10Codec{}, nil
	default:
		return nil, fmt.Errorf("unsupported A2A protocol version: %q", v)
	}
}

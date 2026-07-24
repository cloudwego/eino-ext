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

package wire

import (
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino-ext/a2a/models"
)

// v03Codec implements the legacy A2A v0.3 JSON-RPC binding. Because the models
// structs are already shaped for v0.3, params/results are plain json.Marshal,
// and the response union is discriminated by an injected "kind" field — exactly
// the behavior that previously lived inline in transport/jsonrpc/{client,server}.go.
type v03Codec struct{}

func (v03Codec) Version() models.ProtocolVersion { return models.ProtocolVersion03 }
func (v03Codec) Methods() Methods                { return methodsV03 }

func (v03Codec) EncodeMessageSendParams(p *models.MessageSendParams) (json.RawMessage, error) {
	return json.Marshal(p)
}

func (v03Codec) DecodeMessageSendParams(b []byte) (*models.MessageSendParams, error) {
	out := &models.MessageSendParams{}
	return out, json.Unmarshal(b, out)
}

func (v03Codec) EncodeTaskQueryParams(p *models.TaskQueryParams) (json.RawMessage, error) {
	return json.Marshal(p)
}

func (v03Codec) DecodeTaskQueryParams(b []byte) (*models.TaskQueryParams, error) {
	out := &models.TaskQueryParams{}
	return out, json.Unmarshal(b, out)
}

func (v03Codec) EncodeTaskIDParams(p *models.TaskIDParams) (json.RawMessage, error) {
	return json.Marshal(p)
}

func (v03Codec) DecodeTaskIDParams(b []byte) (*models.TaskIDParams, error) {
	out := &models.TaskIDParams{}
	return out, json.Unmarshal(b, out)
}

func (v03Codec) EncodeTaskPushNotificationConfig(p *models.TaskPushNotificationConfig) (json.RawMessage, error) {
	return json.Marshal(p)
}

func (v03Codec) DecodeTaskPushNotificationConfig(b []byte) (*models.TaskPushNotificationConfig, error) {
	out := &models.TaskPushNotificationConfig{}
	return out, json.Unmarshal(b, out)
}

func (v03Codec) EncodeGetPushParams(p *models.GetTaskPushNotificationConfigParams) (json.RawMessage, error) {
	return json.Marshal(p)
}

func (v03Codec) DecodeGetPushParams(b []byte) (*models.GetTaskPushNotificationConfigParams, error) {
	out := &models.GetTaskPushNotificationConfigParams{}
	return out, json.Unmarshal(b, out)
}

func (v03Codec) EncodeTask(t *models.Task) (json.RawMessage, error) {
	return json.Marshal(t)
}

func (v03Codec) DecodeTask(b []byte) (*models.Task, error) {
	out := &models.Task{}
	return out, json.Unmarshal(b, out)
}

func (v03Codec) EncodeStreamingUnion(u *models.SendMessageStreamingResponseUnion) (json.RawMessage, error) {
	if u == nil {
		return json.Marshal(nil)
	}
	switch {
	case u.Message != nil:
		return json.Marshal(struct {
			*models.Message
			Kind models.ResponseKind `json:"kind"`
		}{Message: u.Message, Kind: models.ResponseKindMessage})
	case u.Task != nil:
		return json.Marshal(struct {
			*models.Task
			Kind models.ResponseKind `json:"kind"`
		}{Task: u.Task, Kind: models.ResponseKindTask})
	case u.TaskStatusUpdateEvent != nil:
		return json.Marshal(struct {
			*models.TaskStatusUpdateEvent
			Kind models.ResponseKind `json:"kind"`
		}{TaskStatusUpdateEvent: u.TaskStatusUpdateEvent, Kind: models.ResponseKindStatusUpdate})
	case u.TaskArtifactUpdateEvent != nil:
		return json.Marshal(struct {
			*models.TaskArtifactUpdateEvent
			Kind models.ResponseKind `json:"kind"`
		}{TaskArtifactUpdateEvent: u.TaskArtifactUpdateEvent, Kind: models.ResponseKindArtifactUpdate})
	default:
		return json.Marshal(nil)
	}
}

func (v03Codec) DecodeStreamingUnion(b []byte) (*models.SendMessageStreamingResponseUnion, error) {
	kind := struct {
		Kind models.ResponseKind `json:"kind"`
	}{}
	if err := json.Unmarshal(b, &kind); err != nil {
		return nil, fmt.Errorf("failed to extract response's kind: %w", err)
	}
	switch kind.Kind {
	case models.ResponseKindMessage:
		m := &models.Message{}
		if err := json.Unmarshal(b, m); err != nil {
			return nil, fmt.Errorf("failed to extract response's message: %w", err)
		}
		return &models.SendMessageStreamingResponseUnion{Message: m}, nil
	case models.ResponseKindTask:
		t := &models.Task{}
		if err := json.Unmarshal(b, t); err != nil {
			return nil, fmt.Errorf("failed to extract response's task: %w", err)
		}
		return &models.SendMessageStreamingResponseUnion{Task: t}, nil
	case models.ResponseKindArtifactUpdate:
		a := &models.TaskArtifactUpdateEvent{}
		if err := json.Unmarshal(b, a); err != nil {
			return nil, fmt.Errorf("failed to extract response's artifact update: %w", err)
		}
		return &models.SendMessageStreamingResponseUnion{TaskArtifactUpdateEvent: a}, nil
	case models.ResponseKindStatusUpdate:
		s := &models.TaskStatusUpdateEvent{}
		if err := json.Unmarshal(b, s); err != nil {
			return nil, fmt.Errorf("failed to extract response's status update: %w", err)
		}
		return &models.SendMessageStreamingResponseUnion{TaskStatusUpdateEvent: s}, nil
	default:
		return nil, fmt.Errorf("unsupported response's kind: %s", kind.Kind)
	}
}

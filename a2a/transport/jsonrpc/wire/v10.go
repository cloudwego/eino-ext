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

// v10Codec implements the A2A v1.0 JSON-RPC binding. It translates the v0.3-shaped
// models types to/from v1.0 DTOs so the rest of eino-ext stays version-agnostic.
type v10Codec struct{}

func (v10Codec) Version() models.ProtocolVersion { return models.ProtocolVersion10 }
func (v10Codec) Methods() Methods                { return methodsV10 }

// ---- request params ---------------------------------------------------------

// taskPushNotificationConfigV10 is the v1.0 flat wire shape. In v0.3 this was
// split into TaskPushNotificationConfig{TaskID, PushNotificationConfig{URL,...}}.
type taskPushNotificationConfigV10 struct {
	ID             string                    `json:"id,omitempty"`
	TaskID         string                    `json:"taskId,omitempty"`
	URL            string                    `json:"url"`
	Token          string                    `json:"token,omitempty"`
	Authentication *models.AuthenticationInfo `json:"authentication,omitempty"`
}

func encTaskPushNotificationConfig(p *models.TaskPushNotificationConfig) *taskPushNotificationConfigV10 {
	if p == nil {
		return nil
	}
	return &taskPushNotificationConfigV10{
		TaskID:         p.TaskID,
		URL:            p.PushNotificationConfig.URL,
		Token:          p.PushNotificationConfig.Token,
		Authentication: p.PushNotificationConfig.Authentication,
	}
}

func decTaskPushNotificationConfig(dto *taskPushNotificationConfigV10) *models.TaskPushNotificationConfig {
	if dto == nil {
		return nil
	}
	return &models.TaskPushNotificationConfig{
		TaskID: dto.TaskID,
		PushNotificationConfig: models.PushNotificationConfig{
			URL:            dto.URL,
			Token:          dto.Token,
			Authentication: dto.Authentication,
		},
	}
}

// sendMessageConfigurationV10 maps the v1.0 SendMessageConfiguration wire shape.
// v0.3 "blocking" becomes v1.0 "returnImmediately" (logical inverse).
// v0.3 "pushNotificationConfig" becomes v1.0 "taskPushNotificationConfig" (flat).
type sendMessageConfigurationV10 struct {
	AcceptedOutputModes        []string                        `json:"acceptedOutputModes,omitempty"`
	TaskPushNotificationConfig *taskPushNotificationConfigV10  `json:"taskPushNotificationConfig,omitempty"`
	HistoryLength              *int                            `json:"historyLength,omitempty"`
	ReturnImmediately          bool                            `json:"returnImmediately,omitempty"`
}

func encSendMessageConfiguration(c *models.MessageSendConfiguration) *sendMessageConfigurationV10 {
	if c == nil {
		return nil
	}
	dto := &sendMessageConfigurationV10{}
	if c.Blocking != nil {
		dto.ReturnImmediately = !*c.Blocking
	}
	if c.PushNotificationConfig != nil {
		dto.TaskPushNotificationConfig = &taskPushNotificationConfigV10{
			URL:            c.PushNotificationConfig.URL,
			Token:          c.PushNotificationConfig.Token,
			Authentication: c.PushNotificationConfig.Authentication,
		}
	}
	return dto
}

func decSendMessageConfiguration(dto *sendMessageConfigurationV10) *models.MessageSendConfiguration {
	if dto == nil {
		return nil
	}
	c := &models.MessageSendConfiguration{}
	blocking := !dto.ReturnImmediately
	c.Blocking = &blocking
	if dto.TaskPushNotificationConfig != nil {
		c.PushNotificationConfig = &models.PushNotificationConfig{
			URL:            dto.TaskPushNotificationConfig.URL,
			Token:          dto.TaskPushNotificationConfig.Token,
			Authentication: dto.TaskPushNotificationConfig.Authentication,
		}
	}
	return c
}

type messageSendParamsV10 struct {
	Message       *messageV10                `json:"message"`
	Configuration *sendMessageConfigurationV10 `json:"configuration,omitempty"`
	Metadata      map[string]any             `json:"metadata,omitempty"`
}

func (v10Codec) EncodeMessageSendParams(p *models.MessageSendParams) (json.RawMessage, error) {
	if p == nil {
		return json.Marshal(nil)
	}
	dto := &messageSendParamsV10{
		Message:       encMessage(&p.Message),
		Configuration: encSendMessageConfiguration(p.Configuration),
		Metadata:      p.Metadata,
	}
	return json.Marshal(dto)
}

func (v10Codec) DecodeMessageSendParams(b []byte) (*models.MessageSendParams, error) {
	dto := &messageSendParamsV10{}
	if err := json.Unmarshal(b, dto); err != nil {
		return nil, err
	}
	out := &models.MessageSendParams{
		Configuration: decSendMessageConfiguration(dto.Configuration),
		Metadata:      dto.Metadata,
	}
	if m := decMessage(dto.Message); m != nil {
		out.Message = *m
	}
	return out, nil
}

func (v10Codec) EncodeTaskQueryParams(p *models.TaskQueryParams) (json.RawMessage, error) {
	return json.Marshal(p)
}

func (v10Codec) DecodeTaskQueryParams(b []byte) (*models.TaskQueryParams, error) {
	out := &models.TaskQueryParams{}
	return out, json.Unmarshal(b, out)
}

func (v10Codec) EncodeTaskIDParams(p *models.TaskIDParams) (json.RawMessage, error) {
	return json.Marshal(p)
}

func (v10Codec) DecodeTaskIDParams(b []byte) (*models.TaskIDParams, error) {
	out := &models.TaskIDParams{}
	return out, json.Unmarshal(b, out)
}

func (v10Codec) EncodeTaskPushNotificationConfig(p *models.TaskPushNotificationConfig) (json.RawMessage, error) {
	return json.Marshal(encTaskPushNotificationConfig(p))
}

func (v10Codec) DecodeTaskPushNotificationConfig(b []byte) (*models.TaskPushNotificationConfig, error) {
	dto := &taskPushNotificationConfigV10{}
	if err := json.Unmarshal(b, dto); err != nil {
		return nil, err
	}
	return decTaskPushNotificationConfig(dto), nil
}

type getPushParamsV10 struct {
	TaskID string `json:"taskId,omitempty"`
	ID     string `json:"id,omitempty"`
}

func (v10Codec) EncodeGetPushParams(p *models.GetTaskPushNotificationConfigParams) (json.RawMessage, error) {
	if p == nil {
		return json.Marshal(nil)
	}
	return json.Marshal(&getPushParamsV10{
		TaskID: p.TaskID,
		ID:     p.PushNotificationConfigID,
	})
}

func (v10Codec) DecodeGetPushParams(b []byte) (*models.GetTaskPushNotificationConfigParams, error) {
	dto := &getPushParamsV10{}
	if err := json.Unmarshal(b, dto); err != nil {
		return nil, err
	}
	return &models.GetTaskPushNotificationConfigParams{
		TaskID:                   dto.TaskID,
		PushNotificationConfigID: dto.ID,
	}, nil
}

// ---- Task result ------------------------------------------------------------

func (v10Codec) EncodeTask(t *models.Task) (json.RawMessage, error) {
	return json.Marshal(encTask(t))
}

func (v10Codec) DecodeTask(b []byte) (*models.Task, error) {
	dto := &taskV10{}
	if err := json.Unmarshal(b, dto); err != nil {
		return nil, err
	}
	return decTask(dto), nil
}

// ---- streaming / send response union ----------------------------------------

// statusUpdateV10 / artifactUpdateV10 drop the v0.3 "final" flag and the "kind"
// discriminator; the wrapper below discriminates by member presence instead.
type statusUpdateV10 struct {
	TaskID    string         `json:"taskId"`
	ContextID string         `json:"contextId"`
	Status    taskStatusV10  `json:"status"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type artifactUpdateV10 struct {
	TaskID    string         `json:"taskId"`
	ContextID string         `json:"contextId"`
	Artifact  *artifactV10   `json:"artifact"`
	Append    bool           `json:"append,omitempty"`
	LastChunk bool           `json:"lastChunk,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type streamingUnionV10 struct {
	Message        *messageV10        `json:"message,omitempty"`
	Task           *taskV10           `json:"task,omitempty"`
	StatusUpdate   *statusUpdateV10   `json:"statusUpdate,omitempty"`
	ArtifactUpdate *artifactUpdateV10 `json:"artifactUpdate,omitempty"`
}

func (v10Codec) EncodeStreamingUnion(u *models.SendMessageStreamingResponseUnion) (json.RawMessage, error) {
	if u == nil {
		return json.Marshal(nil)
	}
	dto := streamingUnionV10{}
	switch {
	case u.Message != nil:
		dto.Message = encMessage(u.Message)
	case u.Task != nil:
		dto.Task = encTask(u.Task)
	case u.TaskStatusUpdateEvent != nil:
		e := u.TaskStatusUpdateEvent
		dto.StatusUpdate = &statusUpdateV10{
			TaskID:    e.TaskID,
			ContextID: e.ContextID,
			Status:    encStatus(e.Status),
			Metadata:  e.Metadata,
		}
	case u.TaskArtifactUpdateEvent != nil:
		e := u.TaskArtifactUpdateEvent
		art := e.Artifact
		dto.ArtifactUpdate = &artifactUpdateV10{
			TaskID:    e.TaskID,
			ContextID: e.ContextID,
			Artifact:  encArtifact(&art),
			Append:    e.Append,
			LastChunk: e.LastChunk,
			Metadata:  e.Metadata,
		}
	default:
		return json.Marshal(nil)
	}
	return json.Marshal(dto)
}

func (v10Codec) DecodeStreamingUnion(b []byte) (*models.SendMessageStreamingResponseUnion, error) {
	dto := &streamingUnionV10{}
	if err := json.Unmarshal(b, dto); err != nil {
		return nil, fmt.Errorf("failed to decode v1.0 streaming union: %w", err)
	}
	switch {
	case dto.Message != nil:
		return &models.SendMessageStreamingResponseUnion{Message: decMessage(dto.Message)}, nil
	case dto.Task != nil:
		return &models.SendMessageStreamingResponseUnion{Task: decTask(dto.Task)}, nil
	case dto.StatusUpdate != nil:
		s := dto.StatusUpdate
		status := decStatus(s.Status)
		return &models.SendMessageStreamingResponseUnion{
			TaskStatusUpdateEvent: &models.TaskStatusUpdateEvent{
				TaskID:    s.TaskID,
				ContextID: s.ContextID,
				Status:    status,
				// v1.0 dropped the explicit "final" flag; terminality is now
				// conveyed by the task state. Reconstruct Final from the state so
				// version-agnostic consumers (which key off Final to end the
				// stream) keep working. See isFinalState.
				Final:    isFinalState(status.State),
				Metadata: s.Metadata,
			},
		}, nil
	case dto.ArtifactUpdate != nil:
		a := dto.ArtifactUpdate
		return &models.SendMessageStreamingResponseUnion{
			TaskArtifactUpdateEvent: &models.TaskArtifactUpdateEvent{
				TaskID:    a.TaskID,
				ContextID: a.ContextID,
				Artifact:  decArtifact(a.Artifact),
				Append:    a.Append,
				LastChunk: a.LastChunk,
				Metadata:  a.Metadata,
			},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported v1.0 streaming union: no member present")
	}
}

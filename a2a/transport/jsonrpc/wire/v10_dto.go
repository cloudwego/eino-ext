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

	"github.com/cloudwego/eino-ext/a2a/models"
)

// This file holds the v1.0 wire DTOs and the pure translation functions between
// them and the shared models types. The DTOs exist only so we can attach v1.0
// JSON tags/shapes without polluting the stable models structs.
//
// TODO(pcap): every shape here is our best reading of A2A v1.0 and must be
// confirmed against a reference server (a2a-python). Known open questions are
// annotated inline.

// ---- Enums ------------------------------------------------------------------

// TODO(pcap): confirm the JSON-RPC binding actually serializes enums as
// SCREAMING_SNAKE_CASE (proto/ProtoJSON) rather than the lowercase strings the
// v0.3 binding uses. Also confirm CANCELLED vs CANCELED spelling.
var (
	taskStateToV10 = map[models.TaskState]string{
		models.TaskStateSubmitted:     "TASK_STATE_SUBMITTED",
		models.TaskStateWorking:       "TASK_STATE_WORKING",
		models.TaskStateInputRequired: "TASK_STATE_INPUT_REQUIRED",
		models.TaskStateCompleted:     "TASK_STATE_COMPLETED",
		models.TaskStateCanceled:      "TASK_STATE_CANCELED",
		models.TaskStateFailed:        "TASK_STATE_FAILED",
		models.TaskStateRejected:      "TASK_STATE_REJECTED",
		models.TaskStateAuthRequired:  "TASK_STATE_AUTH_REQUIRED",
		models.TaskStateUnknown:       "TASK_STATE_UNSPECIFIED",
	}
	taskStateFromV10 = reverseTaskState(taskStateToV10)

	roleToV10 = map[models.Role]string{
		models.RoleUser:  "ROLE_USER",
		models.RoleAgent: "ROLE_AGENT",
	}
	roleFromV10 = reverseRole(roleToV10)
)

func reverseTaskState(m map[models.TaskState]string) map[string]models.TaskState {
	out := make(map[string]models.TaskState, len(m))
	for k, v := range m {
		out[v] = k
	}
	return out
}

func reverseRole(m map[models.Role]string) map[string]models.Role {
	out := make(map[string]models.Role, len(m))
	for k, v := range m {
		out[v] = k
	}
	return out
}

func encTaskState(s models.TaskState) string {
	if v, ok := taskStateToV10[s]; ok {
		return v
	}
	return taskStateToV10[models.TaskStateUnknown]
}

func decTaskState(s string) models.TaskState {
	if v, ok := taskStateFromV10[s]; ok {
		return v
	}
	return models.TaskStateUnknown
}

// isFinalState reports whether a task state is terminal or paused, i.e. a state
// after which a v0.3 server would have set TaskStatusUpdateEvent.Final=true and
// closed the SSE stream. v1.0 removed the explicit "final" flag, so the codec
// derives it from the state to preserve the behavior of version-agnostic
// consumers that end their receive loop on Final.
func isFinalState(s models.TaskState) bool {
	switch s {
	case models.TaskStateCompleted,
		models.TaskStateCanceled,
		models.TaskStateFailed,
		models.TaskStateRejected:
		return true
	default:
		return false
	}
}

func encRole(r models.Role) string {
	if v, ok := roleToV10[r]; ok {
		return v
	}
	return ""
}

func decRole(s string) models.Role {
	if v, ok := roleFromV10[s]; ok {
		return v
	}
	return models.Role(s)
}

// ---- Part -------------------------------------------------------------------

// partV10 is the v1.0 unified Part. In v1.0 the content is discriminated by
// which top-level member is present (text / url / raw / data) — there is no
// "kind" field and no nested "file" object. filename and mediaType sit at the
// top level alongside the content member.
type partV10 struct {
	kind      models.PartKind
	Text      *string
	Raw       *string
	URL       *string
	Data      map[string]any
	Metadata  map[string]any
	Filename  string
	MediaType string
}

// partV10JSON is the raw wire shape; partV10 (un)marshals through it.
type partV10JSON struct {
	Text      *string        `json:"text,omitempty"`
	Raw       *string        `json:"raw,omitempty"`
	URL       *string        `json:"url,omitempty"`
	Data      map[string]any `json:"data,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	Filename  string         `json:"filename,omitempty"`
	MediaType string         `json:"mediaType,omitempty"`
}

func (p partV10) MarshalJSON() ([]byte, error) {
	obj := map[string]any{}
	switch p.kind {
	case models.PartKindText:
		if p.Text != nil {
			obj["text"] = *p.Text
		}
	case models.PartKindData:
		if p.Data != nil {
			obj["data"] = p.Data
		} else {
			obj["data"] = map[string]any{}
		}
	case models.PartKindFile:
		if p.URL != nil {
			obj["url"] = *p.URL
		} else if p.Raw != nil {
			obj["raw"] = *p.Raw
		}
	default:
		if p.Text != nil {
			obj["text"] = *p.Text
		}
		if p.Data != nil {
			obj["data"] = p.Data
		}
		if p.URL != nil {
			obj["url"] = *p.URL
		}
		if p.Raw != nil {
			obj["raw"] = *p.Raw
		}
	}
	if p.Filename != "" {
		obj["filename"] = p.Filename
	}
	if p.MediaType != "" {
		obj["mediaType"] = p.MediaType
	}
	if len(p.Metadata) > 0 {
		obj["metadata"] = p.Metadata
	}
	return json.Marshal(obj)
}

func (p *partV10) UnmarshalJSON(b []byte) error {
	raw := partV10JSON{}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	p.Text = raw.Text
	p.Raw = raw.Raw
	p.URL = raw.URL
	p.Data = raw.Data
	p.Metadata = raw.Metadata
	p.Filename = raw.Filename
	p.MediaType = raw.MediaType
	switch {
	case raw.URL != nil || raw.Raw != nil:
		p.kind = models.PartKindFile
	case raw.Data != nil:
		p.kind = models.PartKindData
	default:
		p.kind = models.PartKindText
	}
	return nil
}

func encPart(p models.Part) partV10 {
	out := partV10{kind: p.Kind, Metadata: p.Metadata}
	switch p.Kind {
	case models.PartKindText:
		out.Text = p.Text
	case models.PartKindData:
		out.Data = p.Data
	case models.PartKindFile:
		if p.File != nil {
			out.Filename = p.File.Name
			out.MediaType = p.File.MimeType
			out.URL = p.File.URI
			out.Raw = p.File.Bytes
		}
	default:
		out.Text = p.Text
		out.Data = p.Data
	}
	return out
}

func decPart(p partV10) models.Part {
	out := models.Part{Kind: p.kind, Metadata: p.Metadata}
	switch p.kind {
	case models.PartKindFile:
		out.File = &models.FileContent{
			Name:     p.Filename,
			MimeType: p.MediaType,
			URI:      p.URL,
			Bytes:    p.Raw,
		}
	case models.PartKindData:
		out.Data = p.Data
	default:
		out.Text = p.Text
	}
	return out
}

func encParts(ps []models.Part) []partV10 {
	if ps == nil {
		return nil
	}
	out := make([]partV10, len(ps))
	for i, p := range ps {
		out[i] = encPart(p)
	}
	return out
}

func decParts(ps []partV10) []models.Part {
	if ps == nil {
		return nil
	}
	out := make([]models.Part, len(ps))
	for i, p := range ps {
		out[i] = decPart(p)
	}
	return out
}

// ---- Message ----------------------------------------------------------------

type messageV10 struct {
	Role             string         `json:"role"`
	Parts            []partV10      `json:"parts"`
	Metadata         map[string]any `json:"metadata,omitempty"`
	ReferenceTaskIDs []string       `json:"referenceTaskIds,omitempty"`
	MessageID        string         `json:"messageId"`
	TaskID           *string        `json:"taskId,omitempty"`
	ContextID        *string        `json:"contextId,omitempty"`
}

func encMessage(m *models.Message) *messageV10 {
	if m == nil {
		return nil
	}
	return &messageV10{
		Role:             encRole(m.Role),
		Parts:            encParts(m.Parts),
		Metadata:         m.Metadata,
		ReferenceTaskIDs: m.ReferenceTaskIDs,
		MessageID:        m.MessageID,
		TaskID:           m.TaskID,
		ContextID:        m.ContextID,
	}
}

func decMessage(m *messageV10) *models.Message {
	if m == nil {
		return nil
	}
	return &models.Message{
		Role:             decRole(m.Role),
		Parts:            decParts(m.Parts),
		Metadata:         m.Metadata,
		ReferenceTaskIDs: m.ReferenceTaskIDs,
		MessageID:        m.MessageID,
		TaskID:           m.TaskID,
		ContextID:        m.ContextID,
	}
}

// ---- Artifact ---------------------------------------------------------------

type artifactV10 struct {
	ArtifactID  string         `json:"artifactId"`
	Name        string         `json:"name,omitempty"`
	Description string         `json:"description,omitempty"`
	Parts       []partV10      `json:"parts"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

func encArtifact(a *models.Artifact) *artifactV10 {
	if a == nil {
		return nil
	}
	return &artifactV10{
		ArtifactID:  a.ArtifactID,
		Name:        a.Name,
		Description: a.Description,
		Parts:       encParts(a.Parts),
		Metadata:    a.Metadata,
	}
}

func decArtifact(a *artifactV10) models.Artifact {
	if a == nil {
		return models.Artifact{}
	}
	return models.Artifact{
		ArtifactID:  a.ArtifactID,
		Name:        a.Name,
		Description: a.Description,
		Parts:       decParts(a.Parts),
		Metadata:    a.Metadata,
	}
}

// ---- Task / TaskStatus ------------------------------------------------------

type taskStatusV10 struct {
	State     string      `json:"state"`
	Message   *messageV10 `json:"message,omitempty"`
	Timestamp string      `json:"timestamp,omitempty"`
}

func encStatus(s models.TaskStatus) taskStatusV10 {
	return taskStatusV10{
		State:     encTaskState(s.State),
		Message:   encMessage(s.Message),
		Timestamp: s.Timestamp,
	}
}

func decStatus(s taskStatusV10) models.TaskStatus {
	return models.TaskStatus{
		State:     decTaskState(s.State),
		Message:   decMessage(s.Message),
		Timestamp: s.Timestamp,
	}
}

type taskV10 struct {
	ID        string         `json:"id"`
	ContextID string         `json:"contextId"`
	Status    taskStatusV10  `json:"status"`
	Artifacts []*artifactV10 `json:"artifacts,omitempty"`
	History   []*messageV10  `json:"history,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

func encTask(t *models.Task) *taskV10 {
	if t == nil {
		return nil
	}
	out := &taskV10{
		ID:        t.ID,
		ContextID: t.ContextID,
		Status:    encStatus(t.Status),
		Metadata:  t.Metadata,
	}
	if t.Artifacts != nil {
		out.Artifacts = make([]*artifactV10, len(t.Artifacts))
		for i, a := range t.Artifacts {
			out.Artifacts[i] = encArtifact(a)
		}
	}
	if t.History != nil {
		out.History = make([]*messageV10, len(t.History))
		for i, m := range t.History {
			out.History[i] = encMessage(m)
		}
	}
	return out
}

func decTask(t *taskV10) *models.Task {
	if t == nil {
		return nil
	}
	out := &models.Task{
		ID:        t.ID,
		ContextID: t.ContextID,
		Status:    decStatus(t.Status),
		Metadata:  t.Metadata,
	}
	if t.Artifacts != nil {
		out.Artifacts = make([]*models.Artifact, len(t.Artifacts))
		for i, a := range t.Artifacts {
			art := decArtifact(a)
			out.Artifacts[i] = &art
		}
	}
	if t.History != nil {
		out.History = make([]*models.Message, len(t.History))
		for i, m := range t.History {
			out.History[i] = decMessage(m)
		}
	}
	return out
}

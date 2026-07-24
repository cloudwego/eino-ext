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
	"strings"
	"testing"

	"github.com/cloudwego/eino-ext/a2a/models"
)

func strptr(s string) *string { return &s }

func allCodecs(t *testing.T) []Codec {
	t.Helper()
	c03, err := NewCodec(models.ProtocolVersion03)
	if err != nil {
		t.Fatalf("NewCodec v03: %v", err)
	}
	c10, err := NewCodec(models.ProtocolVersion10)
	if err != nil {
		t.Fatalf("NewCodec v10: %v", err)
	}
	return []Codec{c03, c10}
}

func TestNewCodec_defaultAndUnknown(t *testing.T) {
	c, err := NewCodec("")
	if err != nil || c.Version() != models.ProtocolVersion03 {
		t.Fatalf("empty version should default to v0.3, got %v err %v", c, err)
	}
	if _, err := NewCodec("9.9"); err == nil {
		t.Fatalf("unknown version should error")
	}
}

func TestMethods_disjoint(t *testing.T) {
	m03, m10 := methodsV03, methodsV10
	names := map[string]bool{}
	for _, n := range []string{m03.Send, m03.Stream, m03.GetTask, m03.Cancel, m03.Resubscribe, m03.PushSet, m03.PushGet,
		m10.Send, m10.Stream, m10.GetTask, m10.Cancel, m10.Resubscribe, m10.PushSet, m10.PushGet} {
		if names[n] {
			t.Fatalf("method name %q collides across versions", n)
		}
		names[n] = true
	}
}

func TestMessageSendParams_roundTrip(t *testing.T) {
	for _, c := range allCodecs(t) {
		in := &models.MessageSendParams{
			Message: models.Message{
				Role:      models.RoleUser,
				MessageID: "m-1",
				Parts: []models.Part{
					{Kind: models.PartKindText, Text: strptr("hello")},
					{Kind: models.PartKindData, Data: map[string]any{"k": "v"}},
					{Kind: models.PartKindFile, File: &models.FileContent{
						Name: "a.txt", MimeType: "text/plain", URI: strptr("https://x/a.txt"),
					}},
				},
			},
		}
		b, err := c.EncodeMessageSendParams(in)
		if err != nil {
			t.Fatalf("[%s] encode: %v", c.Version(), err)
		}
		out, err := c.DecodeMessageSendParams(b)
		if err != nil {
			t.Fatalf("[%s] decode: %v", c.Version(), err)
		}
		if out.Message.Role != models.RoleUser || out.Message.MessageID != "m-1" {
			t.Errorf("[%s] message header lost: %+v", c.Version(), out.Message)
		}
		if len(out.Message.Parts) != 3 {
			t.Fatalf("[%s] want 3 parts, got %d", c.Version(), len(out.Message.Parts))
		}
		if out.Message.Parts[0].Kind != models.PartKindText || out.Message.Parts[0].Text == nil || *out.Message.Parts[0].Text != "hello" {
			t.Errorf("[%s] text part lost: %+v", c.Version(), out.Message.Parts[0])
		}
		if out.Message.Parts[1].Kind != models.PartKindData || out.Message.Parts[1].Data["k"] != "v" {
			t.Errorf("[%s] data part lost: %+v", c.Version(), out.Message.Parts[1])
		}
		fp := out.Message.Parts[2]
		if fp.Kind != models.PartKindFile || fp.File == nil || fp.File.MimeType != "text/plain" || fp.File.URI == nil || *fp.File.URI != "https://x/a.txt" {
			t.Errorf("[%s] file part lost: %+v", c.Version(), fp.File)
		}
	}
}

func TestTask_roundTrip(t *testing.T) {
	for _, c := range allCodecs(t) {
		in := &models.Task{
			ID:        "t-1",
			ContextID: "c-1",
			Status: models.TaskStatus{
				State:     models.TaskStateInputRequired,
				Timestamp: "2023-10-27T10:00:00Z",
				Message:   &models.Message{Role: models.RoleAgent, MessageID: "am-1", Parts: []models.Part{{Kind: models.PartKindText, Text: strptr("more?")}}},
			},
			Artifacts: []*models.Artifact{{ArtifactID: "a-1", Parts: []models.Part{{Kind: models.PartKindText, Text: strptr("out")}}}},
		}
		b, err := c.EncodeTask(in)
		if err != nil {
			t.Fatalf("[%s] encode: %v", c.Version(), err)
		}
		out, err := c.DecodeTask(b)
		if err != nil {
			t.Fatalf("[%s] decode: %v", c.Version(), err)
		}
		if out.ID != "t-1" || out.ContextID != "c-1" {
			t.Errorf("[%s] task ids lost: %+v", c.Version(), out)
		}
		if out.Status.State != models.TaskStateInputRequired {
			t.Errorf("[%s] state lost: %q", c.Version(), out.Status.State)
		}
		if out.Status.Message == nil || out.Status.Message.Role != models.RoleAgent {
			t.Errorf("[%s] status message lost: %+v", c.Version(), out.Status.Message)
		}
		if len(out.Artifacts) != 1 || out.Artifacts[0].ArtifactID != "a-1" {
			t.Errorf("[%s] artifacts lost: %+v", c.Version(), out.Artifacts)
		}
	}
}

func TestStreamingUnion_roundTrip(t *testing.T) {
	for _, c := range allCodecs(t) {
		cases := map[string]*models.SendMessageStreamingResponseUnion{
			"message": {Message: &models.Message{Role: models.RoleAgent, MessageID: "m", Parts: []models.Part{{Kind: models.PartKindText, Text: strptr("hi")}}}},
			"task":    {Task: &models.Task{ID: "t", Status: models.TaskStatus{State: models.TaskStateWorking}}},
			"status":  {TaskStatusUpdateEvent: &models.TaskStatusUpdateEvent{TaskID: "t", ContextID: "c", Status: models.TaskStatus{State: models.TaskStateCompleted}}},
			"artifact": {TaskArtifactUpdateEvent: &models.TaskArtifactUpdateEvent{TaskID: "t", ContextID: "c",
				Artifact: models.Artifact{ArtifactID: "a", Parts: []models.Part{{Kind: models.PartKindText, Text: strptr("chunk")}}}, LastChunk: true}},
		}
		for name, in := range cases {
			b, err := c.EncodeStreamingUnion(in)
			if err != nil {
				t.Fatalf("[%s/%s] encode: %v", c.Version(), name, err)
			}
			out, err := c.DecodeStreamingUnion(b)
			if err != nil {
				t.Fatalf("[%s/%s] decode: %v", c.Version(), name, err)
			}
			switch name {
			case "message":
				if out.Message == nil || out.Message.MessageID != "m" {
					t.Errorf("[%s] message frame lost: %+v", c.Version(), out)
				}
			case "task":
				if out.Task == nil || out.Task.ID != "t" || out.Task.Status.State != models.TaskStateWorking {
					t.Errorf("[%s] task frame lost: %+v", c.Version(), out)
				}
			case "status":
				if out.TaskStatusUpdateEvent == nil || out.TaskStatusUpdateEvent.Status.State != models.TaskStateCompleted {
					t.Errorf("[%s] status frame lost: %+v", c.Version(), out)
				}
			case "artifact":
				e := out.TaskArtifactUpdateEvent
				if e == nil || e.Artifact.ArtifactID != "a" || !e.LastChunk {
					t.Errorf("[%s] artifact frame lost: %+v", c.Version(), out)
				}
			}
		}
	}
}

// TestV10WireShape asserts the concrete v1.0 JSON differs from v0.3 in the ways
// the migration requires: no "kind" discriminator, SCREAMING_SNAKE_CASE enums,
// mediaType instead of mimeType, and member-wrapped stream events.
func TestV10WireShape(t *testing.T) {
	c10, _ := NewCodec(models.ProtocolVersion10)

	// enum casing + no kind on the task result
	tb, _ := c10.EncodeTask(&models.Task{ID: "t", Status: models.TaskStatus{State: models.TaskStateCompleted}})
	s := string(tb)
	if !strings.Contains(s, "TASK_STATE_COMPLETED") {
		t.Errorf("v1.0 task should use SCREAMING_SNAKE_CASE state: %s", s)
	}

	// file mediaType rename + no kind on parts
	pb, _ := c10.EncodeMessageSendParams(&models.MessageSendParams{Message: models.Message{
		Role: models.RoleUser, MessageID: "m",
		Parts: []models.Part{{Kind: models.PartKindFile, File: &models.FileContent{MimeType: "image/png", Bytes: strptr("AAAA")}}},
	}})
	ps := string(pb)
	if !strings.Contains(ps, "mediaType") || strings.Contains(ps, "mimeType") {
		t.Errorf("v1.0 file part should rename mimeType->mediaType: %s", ps)
	}
	if strings.Contains(ps, "ROLE_USER") == false {
		t.Errorf("v1.0 message role should be ROLE_USER: %s", ps)
	}
	if strings.Contains(ps, `"kind"`) {
		t.Errorf("v1.0 part must not carry a kind discriminator: %s", ps)
	}

	// stream status event wrapped in a statusUpdate member, no "final"
	sb, _ := c10.EncodeStreamingUnion(&models.SendMessageStreamingResponseUnion{
		TaskStatusUpdateEvent: &models.TaskStatusUpdateEvent{TaskID: "t", Status: models.TaskStatus{State: models.TaskStateWorking}, Final: true},
	})
	var m map[string]json.RawMessage
	_ = json.Unmarshal(sb, &m)
	if _, ok := m["statusUpdate"]; !ok {
		t.Errorf("v1.0 status event should be wrapped in statusUpdate: %s", sb)
	}
	if strings.Contains(string(sb), `"final"`) {
		t.Errorf("v1.0 status event must not carry final: %s", sb)
	}
}

func TestV03WireShape_unchanged(t *testing.T) {
	c03, _ := NewCodec(models.ProtocolVersion03)
	// v0.3 keeps lowercase enums and the kind discriminator on stream frames.
	sb, _ := c03.EncodeStreamingUnion(&models.SendMessageStreamingResponseUnion{
		TaskStatusUpdateEvent: &models.TaskStatusUpdateEvent{TaskID: "t", Status: models.TaskStatus{State: models.TaskStateWorking}},
	})
	s := string(sb)
	if !strings.Contains(s, `"kind":"status-update"`) {
		t.Errorf("v0.3 stream frame should keep kind=status-update: %s", s)
	}
	if !strings.Contains(s, `"state":"working"`) {
		t.Errorf("v0.3 should keep lowercase state: %s", s)
	}
}

// TestV10Final_reconstructedFromState verifies the v1.0 codec, which has no
// "final" flag on the wire, rebuilds TaskStatusUpdateEvent.Final from the task
// state so version-agnostic consumers that end their loop on Final keep working.
// Per the Python reference, only the 4 terminal states are final; interrupted
// states (input_required, auth_required) are NOT final.
func TestV10Final_reconstructedFromState(t *testing.T) {
	c10, _ := NewCodec(models.ProtocolVersion10)
	cases := map[models.TaskState]bool{
		models.TaskStateWorking:       false,
		models.TaskStateSubmitted:     false,
		models.TaskStateCompleted:     true,
		models.TaskStateCanceled:      true,
		models.TaskStateFailed:        true,
		models.TaskStateRejected:      true,
		models.TaskStateInputRequired: false,
		models.TaskStateAuthRequired:  false,
	}
	for state, wantFinal := range cases {
		b, err := c10.EncodeStreamingUnion(&models.SendMessageStreamingResponseUnion{
			TaskStatusUpdateEvent: &models.TaskStatusUpdateEvent{TaskID: "t", Status: models.TaskStatus{State: state}},
		})
		if err != nil {
			t.Fatalf("encode %q: %v", state, err)
		}
		out, err := c10.DecodeStreamingUnion(b)
		if err != nil {
			t.Fatalf("decode %q: %v", state, err)
		}
		if got := out.TaskStatusUpdateEvent.Final; got != wantFinal {
			t.Errorf("state %q: Final = %v, want %v", state, got, wantFinal)
		}
	}
}

// TestV10DataPart_emptyMapKeepsKind guards the round-trip regression where an
// empty-but-present data map (as EnsureRequiredFields produces) was dropped by
// omitempty and the part silently reclassified as text.
func TestV10DataPart_emptyMapKeepsKind(t *testing.T) {
	c10, _ := NewCodec(models.ProtocolVersion10)
	in := &models.MessageSendParams{Message: models.Message{
		Role: models.RoleUser, MessageID: "m",
		Parts: []models.Part{{Kind: models.PartKindData, Data: map[string]any{}}},
	}}
	b, err := c10.EncodeMessageSendParams(in)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	out, err := c10.DecodeMessageSendParams(b)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out.Message.Parts) != 1 || out.Message.Parts[0].Kind != models.PartKindData {
		t.Errorf("empty data part should stay data, got %+v", out.Message.Parts)
	}
}

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

package eino

import (
	"context"
	"reflect"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/a2a/models"
	"github.com/cloudwego/eino-ext/a2a/server"
)

func TestConcatParts(t *testing.T) {
	a := []models.Part{{Kind: models.PartKindText, Text: strPtr("a")}}
	b := []models.Part{{Kind: models.PartKindText, Text: strPtr("b")}}
	got := concatParts(a, b)
	if len(got) != 2 || *got[0].Text != "a" || *got[1].Text != "b" {
		t.Errorf("concat: got %+v", got)
	}
}

func TestConcatMessages(t *testing.T) {
	t.Run("empty returns nil", func(t *testing.T) {
		if got := concatMessages(nil); got != nil {
			t.Errorf("nil: got %+v", got)
		}
		if got := concatMessages([]*models.Message{}); got != nil {
			t.Errorf("empty: got %+v", got)
		}
	})
	t.Run("nil entries skipped", func(t *testing.T) {
		got := concatMessages([]*models.Message{
			nil,
			{Role: models.RoleAgent, Parts: []models.Part{{Kind: models.PartKindText, Text: strPtr("a")}}},
			nil,
		})
		if got == nil || len(got.Parts) != 1 {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("merges parts and metadata, last non-empty IDs win", func(t *testing.T) {
		taskA := "ta"
		taskB := "tb"
		ctxA := "ca"
		got := concatMessages([]*models.Message{
			{
				Role:      models.RoleAgent,
				MessageID: "m1",
				TaskID:    &taskA,
				ContextID: &ctxA,
				Parts:     []models.Part{{Kind: models.PartKindText, Text: strPtr("hello ")}},
				Metadata:  map[string]any{"k1": "v1"},
			},
			{
				Role:             models.RoleAgent,
				MessageID:        "m2",
				TaskID:           &taskB,
				Parts:            []models.Part{{Kind: models.PartKindText, Text: strPtr("world")}},
				Metadata:         map[string]any{"k2": "v2"},
				ReferenceTaskIDs: []string{"r-1"},
			},
		})
		if got == nil {
			t.Fatal("got nil")
		}
		if got.MessageID != "m2" {
			t.Errorf("messageID: got %q, want last non-empty (m2)", got.MessageID)
		}
		if got.TaskID == nil || *got.TaskID != "tb" {
			t.Errorf("taskID: got %+v, want tb", got.TaskID)
		}
		if got.ContextID == nil || *got.ContextID != "ca" {
			t.Errorf("contextID: got %+v, want ca (first non-nil persists)", got.ContextID)
		}
		if !reflect.DeepEqual(got.ReferenceTaskIDs, []string{"r-1"}) {
			t.Errorf("referenceTaskIDs: got %+v", got.ReferenceTaskIDs)
		}
		if len(got.Parts) != 2 {
			t.Errorf("parts: got %+v", got.Parts)
		}
		if got.Metadata["k1"] != "v1" || got.Metadata["k2"] != "v2" {
			t.Errorf("metadata merged: got %+v", got.Metadata)
		}
	})
}

func TestConcatArtifacts(t *testing.T) {
	t.Run("empty returns nil", func(t *testing.T) {
		if got := concatArtifacts(nil); got != nil {
			t.Errorf("nil: got %+v", got)
		}
	})
	t.Run("single returns same pointer", func(t *testing.T) {
		a := &models.Artifact{ArtifactID: "id"}
		if got := concatArtifacts([]*models.Artifact{a}); got != a {
			t.Errorf("single: got %+v, want same pointer", got)
		}
	})
	t.Run("multi merges parts and metadata", func(t *testing.T) {
		got := concatArtifacts([]*models.Artifact{
			{
				ArtifactID:  "art-1",
				Name:        "first",
				Description: "d1",
				Parts:       []models.Part{{Kind: models.PartKindText, Text: strPtr("foo ")}},
				Metadata:    map[string]any{"k1": "v1"},
			},
			{
				ArtifactID: "art-1",
				Parts:      []models.Part{{Kind: models.PartKindText, Text: strPtr("bar")}},
				Metadata:   map[string]any{"k2": "v2"},
			},
		})
		if got == nil {
			t.Fatal("nil")
		}
		if got.ArtifactID != "art-1" || got.Name != "first" || got.Description != "d1" {
			t.Errorf("identity: got id=%q name=%q desc=%q", got.ArtifactID, got.Name, got.Description)
		}
		if len(got.Parts) != 2 {
			t.Errorf("parts: got %+v", got.Parts)
		}
		if got.Metadata["k1"] != "v1" || got.Metadata["k2"] != "v2" {
			t.Errorf("metadata merged: got %+v", got.Metadata)
		}
	})
}

func TestMessageVar2Status_streaming(t *testing.T) {
	chunks := []*schema.Message{
		{Role: schema.Assistant, Content: "hello "},
		{Role: schema.Assistant, Content: "streaming "},
		{Role: schema.Assistant, Content: "world"},
	}
	mv := &adk.MessageVariant{
		IsStreaming:   true,
		MessageStream: schema.StreamReaderFromArray(chunks),
		Role:          schema.Assistant,
	}

	d := &defaultEventConvertor{
		messageIDGen: func(ctx context.Context) (string, error) {
			return "fixed-msg-id", nil
		},
		outputMessageConv: messages2Parts,
	}

	var emitted []models.ResponseEvent
	writer := func(p models.ResponseEvent) error {
		emitted = append(emitted, p)
		return nil
	}

	if err := d.messageVar2Status(context.Background(), mv, true, writer); err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(emitted) != len(chunks) {
		t.Fatalf("count: got %d, want %d", len(emitted), len(chunks))
	}

	for i, ev := range emitted {
		if ev.Message == nil {
			t.Fatalf("event %d: nil Message", i)
		}
		if ev.Message.MessageID != "fixed-msg-id" {
			t.Errorf("event %d: messageID got %q", i, ev.Message.MessageID)
		}
		isChunk, final := getStreamChunkFinal(ev.Message.Metadata)
		if !isChunk {
			t.Errorf("event %d: expected isChunk=true, metadata=%v", i, ev.Message.Metadata)
		}
		if want := i == len(chunks)-1; final != want {
			t.Errorf("event %d: final=%v, want %v", i, final, want)
		}
	}
}

func TestMessageVar2Status_streamingDisabled(t *testing.T) {
	chunks := []*schema.Message{
		{Role: schema.Assistant, Content: "hello "},
		{Role: schema.Assistant, Content: "world"},
	}
	mv := &adk.MessageVariant{
		IsStreaming:   true,
		MessageStream: schema.StreamReaderFromArray(chunks),
	}
	d := &defaultEventConvertor{
		messageIDGen:      func(ctx context.Context) (string, error) { return "id", nil },
		outputMessageConv: messages2Parts,
	}
	var emitted []models.ResponseEvent
	writer := func(p models.ResponseEvent) error { emitted = append(emitted, p); return nil }

	if err := d.messageVar2Status(context.Background(), mv, false, writer); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(emitted) != 1 {
		t.Fatalf("count: got %d, want 1 (concatenated)", len(emitted))
	}
	if isChunk, _ := getStreamChunkFinal(emitted[0].Message.Metadata); isChunk {
		t.Errorf("non-streaming should not have chunk marker, got %+v", emitted[0].Message.Metadata)
	}
}

func TestMessageVar2Status_nonStreamingMessage(t *testing.T) {
	mv := &adk.MessageVariant{
		Message: &schema.Message{Role: schema.Assistant, Content: "complete"},
	}
	d := &defaultEventConvertor{
		messageIDGen:      func(ctx context.Context) (string, error) { return "id", nil },
		outputMessageConv: messages2Parts,
	}
	var emitted []models.ResponseEvent
	writer := func(p models.ResponseEvent) error { emitted = append(emitted, p); return nil }

	if err := d.messageVar2Status(context.Background(), mv, true, writer); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(emitted) != 1 {
		t.Fatalf("count: got %d, want 1", len(emitted))
	}
	if isChunk, _ := getStreamChunkFinal(emitted[0].Message.Metadata); isChunk {
		t.Errorf("complete message should not have chunk marker")
	}
}

func TestEinoResponseEventConsolidator_messageEvents(t *testing.T) {
	b := &a2aHandlersBuilder{}
	params := &server.InputParams{
		Task: &models.Task{
			Status:  models.TaskStatus{State: models.TaskStateSubmitted},
			History: []*models.Message{},
		},
		Input: &models.Message{Role: models.RoleUser, MessageID: "in-1"},
	}
	events := []models.ResponseEvent{
		{Message: &models.Message{MessageID: "m-1", Role: models.RoleAgent, Parts: []models.Part{{Kind: models.PartKindText, Text: strPtr("hi")}}}},
	}

	tc := b.einoResponseEventConsolidator(context.Background(), params, events, nil)
	if tc == nil {
		t.Fatal("nil TaskContent")
	}
	// Input + the one message event are both in history.
	if len(tc.History) != 2 {
		t.Fatalf("history len: got %d, want 2", len(tc.History))
	}
	if tc.History[0].MessageID != "in-1" || tc.History[1].MessageID != "m-1" {
		t.Errorf("history order: got %q, %q", tc.History[0].MessageID, tc.History[1].MessageID)
	}
}

func TestEinoResponseEventConsolidator_streamingMessageChunks(t *testing.T) {
	b := &a2aHandlersBuilder{}
	mkChunk := func(text string, final bool) models.ResponseEvent {
		md := map[string]any{}
		setStreamChunkFinal(md, final)
		return models.ResponseEvent{
			Message: &models.Message{
				MessageID: "stream-1",
				Role:      models.RoleAgent,
				Parts:     []models.Part{{Kind: models.PartKindText, Text: strPtr(text)}},
				Metadata:  md,
			},
		}
	}
	params := &server.InputParams{
		Task:  &models.Task{Status: models.TaskStatus{State: models.TaskStateSubmitted}},
		Input: &models.Message{Role: models.RoleUser, MessageID: "in-1"},
	}
	events := []models.ResponseEvent{
		mkChunk("hello ", false),
		mkChunk("world", true),
	}

	tc := b.einoResponseEventConsolidator(context.Background(), params, events, nil)
	if tc == nil {
		t.Fatal("nil")
	}
	// Input + 1 merged message in history (chunks should be concatenated, not stored separately).
	if len(tc.History) != 2 {
		t.Fatalf("history: got %d entries, want 2", len(tc.History))
	}
	merged := tc.History[1]
	if merged.MessageID != "stream-1" {
		t.Errorf("merged messageID: got %q", merged.MessageID)
	}
	if len(merged.Parts) != 2 {
		t.Errorf("merged parts: got %d, want 2", len(merged.Parts))
	}
	if _, ok := merged.Metadata[metadataKeyOfStreamChunkFinal]; ok {
		t.Errorf("internal chunk marker should be stripped from merged message, got metadata=%+v", merged.Metadata)
	}
}

func TestEinoResponseEventConsolidator_artifactChunks(t *testing.T) {
	b := &a2aHandlersBuilder{}
	mkChunk := func(text string, last bool) models.ResponseEvent {
		return models.ResponseEvent{
			TaskArtifactUpdateEventContent: &models.TaskArtifactUpdateEventContent{
				Artifact: models.Artifact{
					ArtifactID: "art-1",
					Parts:      []models.Part{{Kind: models.PartKindText, Text: strPtr(text)}},
				},
				LastChunk: last,
			},
		}
	}
	params := &server.InputParams{
		Task:  &models.Task{Status: models.TaskStatus{State: models.TaskStateSubmitted}},
		Input: &models.Message{Role: models.RoleUser, MessageID: "in-1"},
	}
	events := []models.ResponseEvent{
		mkChunk("foo ", false),
		mkChunk("bar", true),
	}
	tc := b.einoResponseEventConsolidator(context.Background(), params, events, nil)
	if tc == nil {
		t.Fatal("nil")
	}
	if len(tc.Artifacts) != 1 {
		t.Fatalf("artifacts: got %d", len(tc.Artifacts))
	}
	if tc.Artifacts[0].ArtifactID != "art-1" {
		t.Errorf("artifactID: got %q", tc.Artifacts[0].ArtifactID)
	}
	if len(tc.Artifacts[0].Parts) != 2 {
		t.Errorf("parts: got %d, want 2", len(tc.Artifacts[0].Parts))
	}
}

func TestEinoResponseEventConsolidator_statusUpdatePropagatesMetadata(t *testing.T) {
	b := &a2aHandlersBuilder{}
	statusMd := map[string]any{}
	setInterrupted(statusMd)
	params := &server.InputParams{
		Task:  &models.Task{Status: models.TaskStatus{State: models.TaskStateSubmitted}},
		Input: &models.Message{Role: models.RoleUser, MessageID: "in-1"},
	}
	events := []models.ResponseEvent{
		{TaskStatusUpdateEventContent: &models.TaskStatusUpdateEventContent{
			Status:   models.TaskStatus{State: models.TaskStateInputRequired},
			Metadata: statusMd,
		}},
	}
	tc := b.einoResponseEventConsolidator(context.Background(), params, events, nil)
	if tc == nil {
		t.Fatal("nil")
	}
	if tc.Status.State != models.TaskStateInputRequired {
		t.Errorf("status: got %v", tc.Status.State)
	}
	if !getInterrupted(tc.Metadata) {
		t.Errorf("expected interrupted flag propagated to TaskContent.Metadata, got %+v", tc.Metadata)
	}
}

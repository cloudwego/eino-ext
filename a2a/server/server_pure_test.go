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

package server

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cloudwego/eino-ext/a2a/models"
)

func TestLoadTaskContext(t *testing.T) {
	t.Run("nil task returns nil", func(t *testing.T) {
		if got := loadTaskContext(nil, &models.TaskContent{}); got != nil {
			t.Errorf("got %+v, want nil", got)
		}
	})
	t.Run("nil content returns same task", func(t *testing.T) {
		t0 := &models.Task{ID: "t-1"}
		if got := loadTaskContext(t0, nil); got != t0 {
			t.Errorf("got %+v, want pointer-equal", got)
		}
	})
	t.Run("merges identity from task and content from tc", func(t *testing.T) {
		t0 := &models.Task{
			ID:        "task-id",
			ContextID: "ctx-id",
			Status:    models.TaskStatus{State: models.TaskStateSubmitted}, // should be overwritten
			Artifacts: []*models.Artifact{{ArtifactID: "old"}},             // should be overwritten
			History:   []*models.Message{{MessageID: "old"}},               // should be overwritten
			Metadata:  map[string]any{"old": true},                         // should be overwritten
		}
		tc := &models.TaskContent{
			Status:    models.TaskStatus{State: models.TaskStateCompleted},
			Artifacts: []*models.Artifact{{ArtifactID: "new"}},
			History:   []*models.Message{{MessageID: "new"}},
			Metadata:  map[string]any{"new": true},
		}
		got := loadTaskContext(t0, tc)
		if got.ID != "task-id" || got.ContextID != "ctx-id" {
			t.Errorf("identity not preserved: got id=%q ctx=%q", got.ID, got.ContextID)
		}
		if got.Status.State != models.TaskStateCompleted {
			t.Errorf("status not from tc: %v", got.Status.State)
		}
		if len(got.Artifacts) != 1 || got.Artifacts[0].ArtifactID != "new" {
			t.Errorf("artifacts not from tc: %+v", got.Artifacts)
		}
		if len(got.History) != 1 || got.History[0].MessageID != "new" {
			t.Errorf("history not from tc: %+v", got.History)
		}
		if got.Metadata["new"] != true || got.Metadata["old"] == true {
			t.Errorf("metadata not from tc: %+v", got.Metadata)
		}
	})
}

func TestInitAgentCard(t *testing.T) {
	cfg := &Config{AgentCardConfig: AgentCardConfig{
		Name:               "agent",
		Description:        "desc",
		URL:                "https://x",
		Version:            "v1",
		DocumentationURL:   "https://x/docs",
		DefaultInputModes:  []string{"text"},
		DefaultOutputModes: []string{"text"},
	}}
	got := initAgentCard(cfg)
	if got.ProtocolVersion == "" {
		t.Errorf("ProtocolVersion should be set, got empty")
	}
	if got.Name != "agent" || got.Description != "desc" || got.URL != "https://x" || got.Version != "v1" || got.DocumentationURL != "https://x/docs" {
		t.Errorf("identity fields: got %+v", got)
	}
	if len(got.DefaultInputModes) != 1 || got.DefaultInputModes[0] != "text" {
		t.Errorf("input modes: %+v", got.DefaultInputModes)
	}
	if len(got.DefaultOutputModes) != 1 || got.DefaultOutputModes[0] != "text" {
		t.Errorf("output modes: %+v", got.DefaultOutputModes)
	}
}

func TestLocalResponseEventWriter(t *testing.T) {
	t.Run("appends in order", func(t *testing.T) {
		w := &localResponseEventWriter{}
		assert.NoError(t, w.Write(models.ResponseEvent{Message: &models.Message{MessageID: "1"}}))
		assert.NoError(t, w.Write(models.ResponseEvent{Message: &models.Message{MessageID: "2"}}))
		if len(w.events) != 2 {
			t.Fatalf("len: got %d", len(w.events))
		}
		if w.events[0].Message.MessageID != "1" || w.events[1].Message.MessageID != "2" {
			t.Errorf("order: got %+v", w.events)
		}
	})
	t.Run("concurrent writes don't lose events", func(t *testing.T) {
		w := &localResponseEventWriter{}
		const n = 200
		var wg sync.WaitGroup
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = w.Write(models.ResponseEvent{Message: &models.Message{}})
			}()
		}
		wg.Wait()
		if len(w.events) != n {
			t.Errorf("len: got %d, want %d", len(w.events), n)
		}
	})
}

func TestWrapUnion(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		if got := wrapUnion(nil); got != nil {
			t.Errorf("got %+v, want nil", got)
		}
	})
	t.Run("each branch sets Kind", func(t *testing.T) {
		cases := []struct {
			name string
			u    *models.SendMessageStreamingResponseUnion
			want models.ResponseKind
		}{
			{"message", &models.SendMessageStreamingResponseUnion{Message: &models.Message{}}, models.ResponseKindMessage},
			{"task", &models.SendMessageStreamingResponseUnion{Task: &models.Task{}}, models.ResponseKindTask},
			{"status update", &models.SendMessageStreamingResponseUnion{TaskStatusUpdateEvent: &models.TaskStatusUpdateEvent{}}, models.ResponseKindStatusUpdate},
			{"artifact update", &models.SendMessageStreamingResponseUnion{TaskArtifactUpdateEvent: &models.TaskArtifactUpdateEvent{}}, models.ResponseKindArtifactUpdate},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				got := wrapUnion(c.u)
				if got == nil {
					t.Fatal("got nil")
				}
				// Kind is the only common field across the four anonymous structs;
				// fish it out reflectively-light by JSON shape isn't needed - we just
				// trust that the wrapped type embeds the original union member.
				switch v := got.(type) {
				case struct {
					*models.Message
					Kind models.ResponseKind `json:"kind"`
				}:
					if v.Kind != c.want {
						t.Errorf("kind: got %v, want %v", v.Kind, c.want)
					}
				case struct {
					*models.Task
					Kind models.ResponseKind `json:"kind"`
				}:
					if v.Kind != c.want {
						t.Errorf("kind: got %v, want %v", v.Kind, c.want)
					}
				case struct {
					*models.TaskStatusUpdateEvent
					Kind models.ResponseKind `json:"kind"`
				}:
					if v.Kind != c.want {
						t.Errorf("kind: got %v, want %v", v.Kind, c.want)
					}
				case struct {
					*models.TaskArtifactUpdateEvent
					Kind models.ResponseKind `json:"kind"`
				}:
					if v.Kind != c.want {
						t.Errorf("kind: got %v, want %v", v.Kind, c.want)
					}
				default:
					t.Errorf("unexpected wrapper type %T", v)
				}
			})
		}
	})
	t.Run("empty union returns nil", func(t *testing.T) {
		if got := wrapUnion(&models.SendMessageStreamingResponseUnion{}); got != nil {
			t.Errorf("got %+v, want nil", got)
		}
	})
}

func TestInMemoryTaskStore(t *testing.T) {
	ctx := context.Background()
	store := newInMemoryTaskStore()

	t.Run("missing returns ok=false", func(t *testing.T) {
		got, ok, err := store.Get(ctx, "missing")
		if err != nil || ok || got != nil {
			t.Errorf("missing: got=%+v ok=%v err=%v", got, ok, err)
		}
	})
	t.Run("save then get", func(t *testing.T) {
		task := &models.Task{ID: "t-1", Status: models.TaskStatus{State: models.TaskStateCompleted}}
		assert.NoError(t, store.Save(ctx, task))
		got, ok, err := store.Get(ctx, "t-1")
		assert.NoError(t, err)
		if !ok || got != task {
			t.Errorf("got %+v ok=%v, want pointer-equal task and ok=true", got, ok)
		}
	})
	t.Run("save overwrites", func(t *testing.T) {
		assert.NoError(t, store.Save(ctx, &models.Task{ID: "t-1", Status: models.TaskStatus{State: models.TaskStateSubmitted}}))
		assert.NoError(t, store.Save(ctx, &models.Task{ID: "t-1", Status: models.TaskStatus{State: models.TaskStateCanceled}}))
		got, _, _ := store.Get(ctx, "t-1")
		if got.Status.State != models.TaskStateCanceled {
			t.Errorf("overwrite: got %v", got.Status.State)
		}
	})
}

func TestInMemoryTaskLocker(t *testing.T) {
	ctx := context.Background()
	t.Run("lock then unlock", func(t *testing.T) {
		l := newInMemoryTaskLocker()
		assert.NoError(t, l.Lock(ctx, "id"))
		assert.NoError(t, l.Unlock(ctx, "id"))
	})
	t.Run("unlock unknown id returns error", func(t *testing.T) {
		l := newInMemoryTaskLocker()
		err := l.Unlock(ctx, "never-locked")
		if err == nil {
			t.Errorf("expected error for unknown id")
		}
	})
	t.Run("Lock blocks while another holder has it", func(t *testing.T) {
		l := newInMemoryTaskLocker()
		assert.NoError(t, l.Lock(ctx, "id"))
		acquired := make(chan struct{})
		go func() {
			_ = l.Lock(ctx, "id")
			close(acquired)
		}()
		select {
		case <-acquired:
			t.Errorf("second Lock should block until first Unlock")
		default:
		}
		assert.NoError(t, l.Unlock(ctx, "id"))
		<-acquired // must finish now
		assert.NoError(t, l.Unlock(ctx, "id"))
	})
}

func TestInMemoryPushNotifier(t *testing.T) {
	ctx := context.Background()
	n := NewInMemoryPushNotifier()

	t.Run("get missing", func(t *testing.T) {
		_, ok, err := n.Get(ctx, "missing")
		if err != nil || ok {
			t.Errorf("missing: ok=%v err=%v", ok, err)
		}
	})
	t.Run("set then get", func(t *testing.T) {
		cfg := &models.TaskPushNotificationConfig{
			TaskID: "t-1",
			PushNotificationConfig: models.PushNotificationConfig{
				URL: "https://hook",
			},
		}
		assert.NoError(t, n.Set(ctx, cfg))
		got, ok, err := n.Get(ctx, "t-1")
		assert.NoError(t, err)
		if !ok || got.URL != "https://hook" {
			t.Errorf("got %+v ok=%v", got, ok)
		}
	})
	t.Run("nil set is no-op", func(t *testing.T) {
		assert.NoError(t, n.Set(ctx, nil))
	})
	t.Run("delete", func(t *testing.T) {
		assert.NoError(t, n.Set(ctx, &models.TaskPushNotificationConfig{TaskID: "t-2", PushNotificationConfig: models.PushNotificationConfig{URL: "x"}}))
		assert.NoError(t, n.Delete(ctx, "t-2"))
		_, ok, _ := n.Get(ctx, "t-2")
		if ok {
			t.Errorf("expected missing after delete")
		}
	})
	t.Run("send notification with nil event no-op", func(t *testing.T) {
		assert.NoError(t, n.SendNotification(ctx, nil))
	})
	t.Run("send notification without registered config no-op", func(t *testing.T) {
		// Not registered for this task — should silently return nil without dialing.
		err := n.SendNotification(ctx, &models.SendMessageStreamingResponseUnion{
			Task: &models.Task{ID: "no-config"},
		})
		assert.NoError(t, err)
	})
}

func TestUnboundedChan_FIFO(t *testing.T) {
	ch := newUnboundedChan[int]()
	ch.Send(1)
	ch.Send(2)
	ch.Send(3)
	for _, want := range []int{1, 2, 3} {
		got, ok := ch.Receive()
		if !ok || got != want {
			t.Errorf("got %d ok=%v, want %d ok=true", got, ok, want)
		}
	}
}

func TestUnboundedChan_CloseWithBufferedDrains(t *testing.T) {
	ch := newUnboundedChan[int]()
	ch.Send(1)
	ch.Send(2)
	ch.Close()
	got, ok := ch.Receive()
	if !ok || got != 1 {
		t.Errorf("first after close: got %d ok=%v", got, ok)
	}
	got, ok = ch.Receive()
	if !ok || got != 2 {
		t.Errorf("second after close: got %d ok=%v", got, ok)
	}
	_, ok = ch.Receive()
	if ok {
		t.Errorf("third after close should be ok=false")
	}
}

func TestUnboundedChan_CloseWakesPendingReceiver(t *testing.T) {
	ch := newUnboundedChan[int]()
	done := make(chan struct{})
	go func() {
		_, ok := ch.Receive()
		if ok {
			t.Errorf("Receive on closed empty chan should return ok=false")
		}
		close(done)
	}()
	ch.Close()
	<-done
}

func TestUnboundedChan_SendOnClosedPanics(t *testing.T) {
	ch := newUnboundedChan[int]()
	ch.Close()
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic on send-after-close")
		}
	}()
	ch.Send(1)
}

func TestUnboundedChan_DoubleClose(t *testing.T) {
	ch := newUnboundedChan[int]()
	ch.Close()
	ch.Close() // second Close must not panic
}

func TestBuildMessageHandlerByStream(t *testing.T) {
	ctx := context.Background()
	captured := []models.ResponseEvent{
		{Message: &models.Message{MessageID: "evt-1"}},
		{TaskStatusUpdateEventContent: &models.TaskStatusUpdateEventContent{Status: models.TaskStatus{State: models.TaskStateCompleted}}},
	}
	streamHandler := func(ctx context.Context, params *InputParams, w ResponseEventWriter) error {
		for _, e := range captured {
			if err := w.Write(e); err != nil {
				return err
			}
		}
		return nil
	}
	consolidator := func(ctx context.Context, params *InputParams, events []models.ResponseEvent, _ error) *models.TaskContent {
		// Just verify the consolidator sees exactly what the streaming handler wrote.
		if len(events) != len(captured) {
			t.Errorf("consolidator events len: got %d, want %d", len(events), len(captured))
		}
		return &models.TaskContent{Status: models.TaskStatus{State: models.TaskStateCompleted}}
	}
	mh := buildMessageHandlerByStream(streamHandler, consolidator)
	tc, err := mh(ctx, &InputParams{Task: &models.Task{ID: "t-1"}, Input: &models.Message{MessageID: "in-1"}})
	assert.NoError(t, err)
	if tc == nil || tc.Status.State != models.TaskStateCompleted {
		t.Errorf("tc: got %+v", tc)
	}
}

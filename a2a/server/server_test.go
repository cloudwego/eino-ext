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
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloudwego/eino-ext/a2a/models"
)

type failingUnlockTaskLocker struct{}

func (failingUnlockTaskLocker) Lock(context.Context, string) error {
	return nil
}

func (failingUnlockTaskLocker) Unlock(context.Context, string) error {
	return errors.New("unlock failed")
}

type capturedLog struct {
	format string
	args   []any
}

func TestSendMessageUnlockErrorUsesGeneratedTaskID(t *testing.T) {
	const generatedTaskID = "generated-task-id"

	for _, tc := range []struct {
		name     string
		blocking bool
	}{
		{name: "blocking", blocking: true},
		{name: "non-blocking", blocking: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			r := &mockHandlerRegistrar{}
			logs := make(chan capturedLog, 1)
			require.NoError(t, RegisterHandlers(ctx, r, &Config{
				MessageHandler: func(context.Context, *InputParams) (*models.TaskContent, error) {
					return &models.TaskContent{Status: models.TaskStatus{State: models.TaskStateCompleted}}, nil
				},
				TaskIDGenerator: func(context.Context) (string, error) {
					return generatedTaskID, nil
				},
				TaskLocker: failingUnlockTaskLocker{},
				Logger: func(_ context.Context, format string, args ...any) {
					logs <- capturedLog{format: format, args: args}
				},
			}))

			result, err := r.h.SendMessage(ctx, &models.MessageSendParams{
				Message: models.Message{Role: models.RoleUser},
				Configuration: &models.MessageSendConfiguration{
					Blocking: &tc.blocking,
				},
			})
			require.NoError(t, err)
			require.NotNil(t, result.Task)
			assert.Equal(t, generatedTaskID, result.Task.ID)

			select {
			case entry := <-logs:
				assert.Equal(t, "failed to release lock for task[%s]: %s", entry.format)
				require.Len(t, entry.args, 2)
				assert.Equal(t, generatedTaskID, entry.args[0])
				assert.Equal(t, "unlock failed", entry.args[1])
			case <-time.After(time.Second):
				t.Fatal("timed out waiting for unlock error log")
			}
		})
	}
}

func TestMessageHandler(t *testing.T) {
	ctx := context.Background()
	r := &mockHandlerRegistrar{}
	taskStore := newInMemoryTaskStore()
	taskLocker := newInMemoryTaskLocker()

	text := "hello world"
	inputMessage := &models.Message{
		Role:  models.RoleUser,
		Parts: []models.Part{{Kind: models.PartKindText, Text: &text}},
	}
	inputMetadata := map[string]any{"1": "2"}
	assert.NoError(t, RegisterHandlers(ctx, r, &Config{
		AgentCardConfig: AgentCardConfig{},
		MessageHandler: func(ctx context.Context, params *InputParams) (*models.TaskContent, error) {
			assert.Equal(t, models.TaskStateSubmitted, params.Task.Status.State)
			assert.Equal(t, inputMessage, params.Input)
			assert.Equal(t, inputMetadata, params.Metadata)
			return &models.TaskContent{
				Status:    models.TaskStatus{State: models.TaskStateCompleted},
				History:   []*models.Message{nil},
				Artifacts: []*models.Artifact{nil},
			}, nil
		},
		TaskStore:  taskStore,
		TaskLocker: taskLocker,
	}))
	result, err := r.h.SendMessage(ctx, &models.MessageSendParams{
		Message:  *inputMessage,
		Metadata: inputMetadata,
	})
	assert.NoError(t, err)
	assert.Equal(t, models.TaskStateCompleted, result.Task.Status.State)
	assert.Equal(t, 1, len(result.Task.History))
	assert.Equal(t, 1, len(result.Task.Artifacts))

	task, existed, err := taskStore.Get(ctx, result.Task.ID)
	assert.NoError(t, err)
	assert.True(t, existed)
	assert.Equal(t, models.TaskStateCompleted, task.Status.State)
	assert.Equal(t, 1, len(task.History))
	assert.Equal(t, 1, len(task.Artifacts))
}

func TestStreamingMessageHandler(t *testing.T) {
	ctx := context.Background()
	r := &mockHandlerRegistrar{}
	taskStore := newInMemoryTaskStore()
	taskLocker := newInMemoryTaskLocker()

	text := "hello world"
	inputMessage := &models.Message{
		Role:  models.RoleUser,
		Parts: []models.Part{{Kind: models.PartKindText, Text: &text}},
	}
	inputMetadata := map[string]any{"1": "2"}
	assert.NoError(t, RegisterHandlers(ctx, r, &Config{
		AgentCardConfig: AgentCardConfig{},
		MessageStreamingHandler: func(ctx context.Context, params *InputParams, writer ResponseEventWriter) error {
			assert.Equal(t, models.TaskStateSubmitted, params.Task.Status.State)
			assert.Equal(t, inputMessage, params.Input)
			assert.Equal(t, inputMetadata, params.Metadata)
			if err := writer.Write(models.ResponseEvent{
				TaskContent: &models.TaskContent{Status: models.TaskStatus{State: models.TaskStateWorking}},
			}); err != nil {
				return err
			}
			if err := writer.Write(models.ResponseEvent{
				Message: &models.Message{
					MessageID: "test message id",
				},
			}); err != nil {
				return err
			}
			if err := writer.Write(models.ResponseEvent{
				TaskStatusUpdateEventContent: &models.TaskStatusUpdateEventContent{
					Status: models.TaskStatus{State: models.TaskStateCompleted},
				},
			}); err != nil {
				return err
			}
			if err := writer.Write(models.ResponseEvent{
				TaskArtifactUpdateEventContent: &models.TaskArtifactUpdateEventContent{
					Artifact: models.Artifact{
						ArtifactID: "test artifact id",
					},
				},
			}); err != nil {
				return err
			}
			return nil
		},
		TaskEventsConsolidator: func(ctx context.Context, params *InputParams, events []models.ResponseEvent, handlerErr error) *models.TaskContent {
			ot := params.Task
			tc := &models.TaskContent{
				Status:    ot.Status,
				Artifacts: ot.Artifacts,
				History:   append(ot.History, params.Input),
				Metadata:  ot.Metadata,
			}
			for _, event := range events {
				if event.Message != nil {
					tc.History = append(tc.History, event.Message)
				} else if event.TaskContent != nil {
					tc.Status = event.TaskContent.Status
					tc.Artifacts = event.TaskContent.Artifacts
					tc.History = event.TaskContent.History
					tc.Metadata = event.TaskContent.Metadata
				} else if event.TaskStatusUpdateEventContent != nil {
					tc.Status = event.TaskStatusUpdateEventContent.Status
				} else if event.TaskArtifactUpdateEventContent != nil {
					tc.Artifacts = append(tc.Artifacts, &event.TaskArtifactUpdateEventContent.Artifact)
				}
			}
			return tc
		},
		TaskStore:  taskStore,
		TaskLocker: taskLocker,
	}))

	// build message handler by streaming
	result, err := r.h.SendMessage(ctx, &models.MessageSendParams{
		Message:  *inputMessage,
		Metadata: inputMetadata,
	})
	assert.NoError(t, err)
	assert.Equal(t, models.TaskStateCompleted, result.Task.Status.State)
	assert.Equal(t, 1, len(result.Task.History))
	assert.Equal(t, 1, len(result.Task.Artifacts))

	task, existed, err := taskStore.Get(ctx, result.Task.ID)
	assert.NoError(t, err)
	assert.True(t, existed)
	assert.Equal(t, models.TaskStateCompleted, task.Status.State)
	assert.Equal(t, 1, len(task.History))
	assert.Equal(t, 1, len(task.Artifacts))

	// streaming
	writer := &mockWriter{}
	err = r.h.SendMessageStreaming(ctx, &models.MessageSendParams{Message: *inputMessage, Metadata: inputMetadata}, writer)
	assert.NoError(t, err)
	assert.Equal(t, 4, len(writer.unions))
	task, existed, err = taskStore.Get(ctx, writer.unions[0].GetTaskID())
	assert.NoError(t, err)
	assert.True(t, existed)
	assert.Equal(t, models.TaskStateCompleted, task.Status.State)
	assert.Equal(t, 1, len(task.History))
	assert.Equal(t, 1, len(task.Artifacts))
}

func TestWrapSendMessageStreamingResponseUnion(t *testing.T) {
	taskID := "TaskID"
	contextID := "ContextID"
	union := wrapSendMessageStreamingResponseUnion(models.ResponseEvent{
		Message: &models.Message{},
	}, taskID, contextID)
	assert.Equal(t, taskID, *union.Message.TaskID)
	assert.Equal(t, contextID, *union.Message.ContextID)
	union = wrapSendMessageStreamingResponseUnion(models.ResponseEvent{
		TaskContent: &models.TaskContent{},
	}, taskID, contextID)
	assert.Equal(t, taskID, union.Task.ID)
	assert.Equal(t, contextID, union.Task.ContextID)
	union = wrapSendMessageStreamingResponseUnion(models.ResponseEvent{
		TaskStatusUpdateEventContent: &models.TaskStatusUpdateEventContent{},
	}, taskID, contextID)
	assert.Equal(t, taskID, union.TaskStatusUpdateEvent.TaskID)
	assert.Equal(t, contextID, union.TaskStatusUpdateEvent.ContextID)
	union = wrapSendMessageStreamingResponseUnion(models.ResponseEvent{
		TaskArtifactUpdateEventContent: &models.TaskArtifactUpdateEventContent{},
	}, taskID, contextID)
	assert.Equal(t, taskID, union.TaskArtifactUpdateEvent.TaskID)
	assert.Equal(t, contextID, union.TaskArtifactUpdateEvent.ContextID)
}

type mockHandlerRegistrar struct {
	h *models.ServerHandlers
}

func (m *mockHandlerRegistrar) Register(ctx context.Context, handlers *models.ServerHandlers) error {
	m.h = handlers
	return nil
}

type mockWriter struct {
	unions []*models.SendMessageStreamingResponseUnion
}

func (m *mockWriter) Write(ctx context.Context, f *models.SendMessageStreamingResponseUnion) error {
	m.unions = append(m.unions, f)
	return nil
}

func (m *mockWriter) Close() error {
	return nil
}

// The tests below cover how streaming requests retain and release task queues.

const streamTaskID = "stream-task"

func newStreamingTestHandlers(t *testing.T, cfg *Config) *models.ServerHandlers {
	t.Helper()
	cfg.TaskIDGenerator = func(context.Context) (string, error) { return streamTaskID, nil }
	cfg.TaskEventsConsolidator = func(context.Context, *InputParams, []models.ResponseEvent, error) *models.TaskContent {
		return &models.TaskContent{Status: models.TaskStatus{State: models.TaskStateCompleted}}
	}
	r := &mockHandlerRegistrar{}
	require.NoError(t, RegisterHandlers(context.Background(), r, cfg))
	return r.h
}

func emitMessages(ids ...string) MessageStreamingHandler {
	return func(_ context.Context, _ *InputParams, w ResponseEventWriter) error {
		for _, id := range ids {
			if err := w.Write(models.ResponseEvent{Message: &models.Message{MessageID: id}}); err != nil {
				return err
			}
		}
		return nil
	}
}

// streamParams requests a push notification, which only takes effect when a PushNotifier is configured.
func streamParams() *models.MessageSendParams {
	return &models.MessageSendParams{
		Message: models.Message{Role: models.RoleUser},
		Configuration: &models.MessageSendConfiguration{
			PushNotificationConfig: &models.PushNotificationConfig{URL: "https://example.invalid/hook"},
		},
	}
}

func messageIDs(unions []*models.SendMessageStreamingResponseUnion) []string {
	ids := make([]string, 0, len(unions))
	for _, u := range unions {
		ids = append(ids, u.Message.MessageID)
	}
	return ids
}

func waitFor(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out")
	}
}

func callRecovering(f func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return f()
}

// signalLocker closes unlocked on the first Unlock, right after running onUnlock.
type signalLocker struct {
	onUnlock func()
	unlocked chan struct{}
	once     sync.Once
}

func newSignalLocker(onUnlock func()) *signalLocker {
	return &signalLocker{onUnlock: onUnlock, unlocked: make(chan struct{})}
}

func (*signalLocker) Lock(context.Context, string) error { return nil }

func (l *signalLocker) Unlock(context.Context, string) error {
	l.once.Do(func() {
		if l.onUnlock != nil {
			l.onUnlock()
		}
		close(l.unlocked)
	})
	return nil
}

// blockingWriter blocks its first Write until release is closed.
type blockingWriter struct {
	mockWriter
	entered, release chan struct{}
	once             sync.Once
}

func newBlockingWriter() *blockingWriter {
	return &blockingWriter{entered: make(chan struct{}), release: make(chan struct{})}
}

func (w *blockingWriter) Write(ctx context.Context, f *models.SendMessageStreamingResponseUnion) error {
	w.once.Do(func() {
		close(w.entered)
		<-w.release
	})
	return w.mockWriter.Write(ctx, f)
}

type disconnectedWriter struct{}

func (disconnectedWriter) Write(context.Context, *models.SendMessageStreamingResponseUnion) error {
	return errors.New("client disconnected")
}

func (disconnectedWriter) Close() error { return nil }

type failingSetPushNotifier struct {
	err      error
	panicMsg string
}

func (n failingSetPushNotifier) Set(context.Context, *models.TaskPushNotificationConfig) error {
	if n.panicMsg != "" {
		panic(n.panicMsg)
	}
	return n.err
}

func (failingSetPushNotifier) Get(context.Context, string) (models.PushNotificationConfig, bool, error) {
	return models.PushNotificationConfig{}, false, nil
}

func (failingSetPushNotifier) Delete(context.Context, string) error { return nil }

func (failingSetPushNotifier) SendNotification(context.Context, *models.SendMessageStreamingResponseUnion) error {
	return nil
}

// recordingQueue is a custom EventQueue that wraps another one through the
// public interface and records what the server passes to it.
type recordingQueue struct {
	EventQueue
	resetErr error

	mu       sync.Mutex
	closes   int
	popCtxs  []context.Context
	popTasks []string
}

func (q *recordingQueue) Reset(ctx context.Context, taskID string) error {
	if q.resetErr != nil {
		return q.resetErr
	}
	return q.EventQueue.Reset(ctx, taskID)
}

func (q *recordingQueue) Pop(ctx context.Context, taskID string) (*models.SendMessageStreamingResponseUnion, error, bool, error) {
	q.mu.Lock()
	q.popCtxs = append(q.popCtxs, ctx)
	q.popTasks = append(q.popTasks, taskID)
	q.mu.Unlock()
	return q.EventQueue.Pop(ctx, taskID)
}

func (q *recordingQueue) Close(ctx context.Context, taskID string) error {
	q.mu.Lock()
	q.closes++
	q.mu.Unlock()
	return q.EventQueue.Close(ctx, taskID)
}

func TestStreamingRetainsQueueWhileReaderIsActive(t *testing.T) {
	ctx := context.Background()

	t.Run("SendMessageStreaming", func(t *testing.T) {
		eq := &inMemoryEventQueue{}
		locker := newSignalLocker(nil)
		h := newStreamingTestHandlers(t, &Config{TaskLocker: locker, Queue: eq, MessageStreamingHandler: emitMessages("e1", "e2")})

		w := newBlockingWriter()
		var err error
		done := make(chan struct{})
		go func() {
			defer close(done)
			err = h.SendMessageStreaming(ctx, streamParams(), w)
		}()
		waitFor(t, w.entered)
		waitFor(t, locker.unlocked) // the handler has finished and closed the queue
		exists, armed := queueState(eq, streamTaskID)
		assert.True(t, exists)
		assert.False(t, armed, "queue must not be evicted while its reader is blocked in Write")

		close(w.release)
		waitFor(t, done)
		require.NoError(t, err)
		assert.Equal(t, []string{"e1", "e2"}, messageIDs(w.unions))
		exists, _ = queueState(eq, streamTaskID)
		assert.False(t, exists, "drained queue must be removed")
	})

	t.Run("ResubscribeTask", func(t *testing.T) {
		eq := &inMemoryEventQueue{}
		locker := newSignalLocker(nil)
		h := newStreamingTestHandlers(t, &Config{TaskLocker: locker, Queue: eq, MessageStreamingHandler: emitMessages("e1", "e2", "e3")})

		err := h.SendMessageStreaming(ctx, streamParams(), disconnectedWriter{})
		require.ErrorContains(t, err, "client disconnected")
		waitFor(t, locker.unlocked)
		_, armed := queueState(eq, streamTaskID)
		require.True(t, armed, "abandoned queue must wait for the grace period")

		w := newBlockingWriter()
		done := make(chan struct{})
		go func() {
			defer close(done)
			err = h.ResubscribeTask(ctx, &models.TaskIDParams{ID: streamTaskID}, w)
		}()
		waitFor(t, w.entered)
		_, armed = queueState(eq, streamTaskID)
		assert.False(t, armed, "resubscribing must cancel the grace period")

		close(w.release)
		waitFor(t, done)
		require.NoError(t, err)
		assert.Equal(t, []string{"e2", "e3"}, messageIDs(w.unions))
		exists, _ := queueState(eq, streamTaskID)
		assert.False(t, exists, "drained queue must be removed")
	})
}

func TestStreamingClosesAbandonedQueue(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name     string
		handler  MessageStreamingHandler
		notifier PushNotifier
		writer   models.ResponseWriter
		wantErr  string
	}{
		{
			name:    "client disconnect",
			handler: emitMessages("e1", "e2"),
			writer:  disconnectedWriter{},
			wantErr: "client disconnected",
		},
		{
			name:    "handler error",
			handler: func(context.Context, *InputParams, ResponseEventWriter) error { return errors.New("handler failed") },
			wantErr: "handler failed",
		},
		{
			name:    "handler panic",
			handler: func(context.Context, *InputParams, ResponseEventWriter) error { panic("handler panicked") },
			wantErr: "handler panicked",
		},
		{
			name:     "push notifier Set error",
			notifier: failingSetPushNotifier{err: errors.New("set failed")},
			wantErr:  "set failed",
		},
		{
			name:     "push notifier Set panic",
			notifier: failingSetPushNotifier{panicMsg: "set panicked"},
			wantErr:  "set panicked",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			eq := &inMemoryEventQueue{}
			closedAtUnlock := false
			locker := newSignalLocker(func() {
				eq.mu.Lock()
				defer eq.mu.Unlock()
				q, ok := eq.queues[streamTaskID]
				closedAtUnlock = ok && q.closed
			})
			if tc.handler == nil {
				tc.handler = emitMessages("e1")
			}
			if tc.writer == nil {
				tc.writer = &mockWriter{}
			}
			h := newStreamingTestHandlers(t, &Config{TaskLocker: locker, Queue: eq, PushNotifier: tc.notifier, MessageStreamingHandler: tc.handler})

			err := callRecovering(func() error { return h.SendMessageStreaming(ctx, streamParams(), tc.writer) })
			assert.ErrorContains(t, err, tc.wantErr)
			waitFor(t, locker.unlocked)
			assert.True(t, closedAtUnlock, "queue must be closed before the task lock is released")
			exists, armed := queueState(eq, streamTaskID)
			assert.True(t, exists && armed, "closed but undrained queue must be evicted after the grace period")
		})
	}
}

func TestStreamingWithCustomEventQueue(t *testing.T) {
	type ctxKey struct{}
	ctx := context.WithValue(context.Background(), ctxKey{}, "v")

	t.Run("Pop receives the caller's ctx and task ID", func(t *testing.T) {
		q := &recordingQueue{EventQueue: newInMemoryEventQueue()}
		locker := newSignalLocker(nil)
		h := newStreamingTestHandlers(t, &Config{TaskLocker: locker, Queue: q, MessageStreamingHandler: emitMessages("e1", "e2", "e3")})

		err := h.SendMessageStreaming(ctx, streamParams(), disconnectedWriter{})
		require.ErrorContains(t, err, "client disconnected")
		waitFor(t, locker.unlocked)
		require.Len(t, q.popCtxs, 1)
		assert.Equal(t, "v", q.popCtxs[0].Value(ctxKey{}))
		assert.Equal(t, 1, q.closes, "only the handler goroutine closes a custom queue")

		w := &mockWriter{}
		require.NoError(t, h.ResubscribeTask(ctx, &models.TaskIDParams{ID: streamTaskID}, w))
		assert.Equal(t, []string{"e2", "e3"}, messageIDs(w.unions))
		require.Len(t, q.popCtxs, 4)
		for _, popCtx := range q.popCtxs[1:] {
			assert.Same(t, ctx, popCtx)
		}
		assert.Equal(t, []string{streamTaskID, streamTaskID, streamTaskID, streamTaskID}, q.popTasks)
	})

	t.Run("push notifier failure does not Close the custom queue", func(t *testing.T) {
		for _, notifier := range []failingSetPushNotifier{{err: errors.New("set failed")}, {panicMsg: "set panicked"}} {
			q := &recordingQueue{EventQueue: newInMemoryEventQueue()}
			locker := newSignalLocker(nil)
			h := newStreamingTestHandlers(t, &Config{TaskLocker: locker, Queue: q, PushNotifier: notifier, MessageStreamingHandler: emitMessages()})

			err := callRecovering(func() error { return h.SendMessageStreaming(ctx, streamParams(), &mockWriter{}) })
			assert.ErrorContains(t, err, "set")
			waitFor(t, locker.unlocked)
			assert.Zero(t, q.closes)
		}
	})

	t.Run("Reset failure keeps the existing push notification config", func(t *testing.T) {
		taskStore := newInMemoryTaskStore()
		notifier := NewInMemoryPushNotifier()
		taskID := "existing-task"
		require.NoError(t, taskStore.Save(ctx, &models.Task{ID: taskID, ContextID: "ctx-1"}))
		require.NoError(t, notifier.Set(ctx, &models.TaskPushNotificationConfig{
			TaskID:                 taskID,
			PushNotificationConfig: models.PushNotificationConfig{URL: "https://old.example/hook"},
		}))
		h := newStreamingTestHandlers(t, &Config{
			TaskStore:               taskStore,
			PushNotifier:            notifier,
			Queue:                   &recordingQueue{EventQueue: newInMemoryEventQueue(), resetErr: errors.New("reset failed")},
			MessageStreamingHandler: emitMessages(),
		})

		params := streamParams()
		params.Message.TaskID = &taskID
		require.ErrorContains(t, h.SendMessageStreaming(ctx, params, &mockWriter{}), "reset failed")
		got, exists, err := notifier.Get(ctx, taskID)
		require.NoError(t, err)
		require.True(t, exists)
		assert.Equal(t, "https://old.example/hook", got.URL)
	})
}

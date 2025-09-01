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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cloudwego/eino-ext/a2a/models"
)

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
		TaskEventsConsolidator: func(ctx context.Context, t *models.Task, events []models.ResponseEvent, handlerErr error) *models.TaskContent {
			tc := &models.TaskContent{
				Status:    t.Status,
				Artifacts: t.Artifacts,
				History:   t.History,
				Metadata:  t.Metadata,
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

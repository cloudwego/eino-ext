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

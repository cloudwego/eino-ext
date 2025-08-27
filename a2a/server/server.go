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
	"log"
	"runtime/debug"
	"sync"
	"time"

	"github.com/go-openapi/spec"
	"github.com/google/uuid"

	"github.com/cloudwego/eino-ext/a2a/models"
	"github.com/cloudwego/eino-ext/a2a/transport"
	"github.com/cloudwego/eino-ext/a2a/utils"
)

type InputParams struct {
	Task     *models.Task
	Input    *models.Message
	Metadata map[string]any
}

type MessageHandler func(ctx context.Context, params *InputParams) (*models.TaskContent, error)
type MessageStreamingHandler func(ctx context.Context, params *InputParams, writer ResponseEventWriter) error
type CancelTaskHandler func(ctx context.Context, params *InputParams) (*models.TaskContent, error)
type TaskEventsConsolidator func(ctx context.Context, t *models.Task, events []models.ResponseEvent, handleErr error) *models.TaskContent
type Logger func(ctx context.Context, format string, v ...any)

type ResponseEventWriter interface {
	Write(event models.ResponseEvent) error
}

type Config struct {
	AgentCardConfig

	// optional
	MessageHandler MessageHandler
	// optional
	MessageStreamingHandler MessageStreamingHandler

	// optional, uuid by default
	TaskIDGenerator    func(ctx context.Context) (string, error)
	ContextIDGenerator func(ctx context.Context) (string, error)
	// required
	CancelTaskHandler CancelTaskHandler
	// required
	TaskEventsConsolidator TaskEventsConsolidator
	// optional, log.Printf by default
	Logger Logger
	// optional, in-memory by default
	TaskStore TaskStore
	// optional, in-memory by default
	TaskLocker TaskLocker
	// optional, in-memory by default
	Queue EventQueue
	// optional, in-memory by default
	PushNotifier PushNotifier
}

type AgentCardConfig struct {
	Name             string
	Description      string
	URL              string
	Version          string
	DocumentationURL string

	Provider *models.AgentProvider

	SecuritySchemes    map[string]*spec.SecurityScheme
	Security           map[string][]string
	DefaultInputModes  []string
	DefaultOutputModes []string
	Skills             []models.AgentSkill
}

func RegisterHandlers(ctx context.Context, registrar transport.HandlerRegistrar, config *Config) error {
	if registrar == nil {
		return fmt.Errorf("HandlerRegistrar is required")
	}
	if config == nil {
		return fmt.Errorf("config is required")
	}
	ret := &models.ServerHandlers{}

	s := initA2AServer(config)

	if s.logger == nil {
		s.logger = func(_ context.Context, format string, v ...any) { log.Printf(format, v...) }
	}
	if s.taskIDGenerator == nil {
		s.taskIDGenerator = func(_ context.Context) (string, error) {
			return uuid.NewString(), nil
		}
	}
	if s.contextIDGenerator == nil {
		s.contextIDGenerator = func(_ context.Context) (string, error) {
			return uuid.NewString(), nil
		}
	}
	if s.taskStore == nil {
		s.taskStore = newInMemoryTaskStore()
	}
	if s.taskLocker == nil {
		s.taskLocker = newInMemoryTaskLocker()
	}
	if s.queue == nil {
		s.queue = newInMemoryEventQueue()
	}

	ret.AgentCard = func(ctx context.Context) *models.AgentCard {
		return s.agentCard
	}
	ret.GetTask = func(ctx context.Context, params *models.TaskQueryParams) (*models.Task, error) {
		return s.getTask(ctx, params)
	}

	if s.messageHandler == nil && s.messageStreamingHandler == nil {
		return errors.New("handler is required")
	}
	if s.messageHandler != nil {
		ret.SendMessage = s.sendMessage
	}
	if s.messageStreamingHandler != nil {
		if s.taskEventsConsolidator == nil {
			return errors.New("task modifier is required if message stream handler has been set")
		}
		s.agentCard.Capabilities.Streaming = true

		ret.SendMessageStreaming = s.sendMessageStreaming
		ret.ResubscribeTask = s.resubscribeTask
		if s.messageHandler == nil {
			s.messageHandler = buildMessageHandlerByStream(s.messageStreamingHandler, s.taskEventsConsolidator)
			ret.SendMessage = s.sendMessage
		}
	}
	if s.cancelTaskHandler == nil {
		ret.CancelTask = func(ctx context.Context, params *models.TaskIDParams) (*models.Task, error) {
			return nil, fmt.Errorf("task cancel haven't been implemented")
		}
	} else {
		ret.CancelTask = s.cancelTask
	}
	if s.pushNotifier != nil {
		s.agentCard.Capabilities.PushNotifications = true
		ret.GetPushNotificationConfig = s.getTasksPushNotificationConfig
		ret.SetPushNotificationConfig = s.setTasksPushNotificationConfig
	}

	return registrar.Register(ctx, ret)
}

func initA2AServer(config *Config) *A2AServer {
	return &A2AServer{
		agentCard:               initAgentCard(config),
		messageHandler:          config.MessageHandler,
		messageStreamingHandler: config.MessageStreamingHandler,
		cancelTaskHandler:       config.CancelTaskHandler,
		taskEventsConsolidator:  config.TaskEventsConsolidator,
		logger:                  config.Logger,
		taskIDGenerator:         config.TaskIDGenerator,
		contextIDGenerator:      config.ContextIDGenerator,
		taskStore:               config.TaskStore,
		taskLocker:              config.TaskLocker,
		queue:                   config.Queue,
		pushNotifier:            config.PushNotifier,
	}
}

func initAgentCard(config *Config) *models.AgentCard {
	return &models.AgentCard{
		ProtocolVersion:    "0.2.5",
		Name:               config.Name,
		Description:        config.Description,
		URL:                config.URL,
		Provider:           config.Provider,
		Version:            config.Version,
		DocumentationURL:   config.DocumentationURL,
		SecuritySchemes:    config.SecuritySchemes,
		Security:           config.Security,
		DefaultInputModes:  config.DefaultInputModes,
		DefaultOutputModes: config.DefaultOutputModes,
		Skills:             config.Skills,
	}
}

// A2AServer represents an A2A server instance
type A2AServer struct {
	agentCard     *models.AgentCard
	agentCartPath string

	messageHandler          MessageHandler
	messageStreamingHandler MessageStreamingHandler
	cancelTaskHandler       CancelTaskHandler

	taskEventsConsolidator TaskEventsConsolidator

	logger Logger

	taskIDGenerator    func(ctx context.Context) (string, error)
	contextIDGenerator func(ctx context.Context) (string, error)
	taskStore          TaskStore
	taskLocker         TaskLocker
	queue              EventQueue
	pushNotifier       PushNotifier
}

func (s *A2AServer) getTask(ctx context.Context, input *models.TaskQueryParams) (*models.Task, error) {
	err := s.taskLocker.Lock(ctx, input.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire lock for new task[%s]: %w", input.ID, err)
	}
	defer func() {
		unlockErr := s.taskLocker.Unlock(ctx, input.ID)
		if unlockErr != nil {
			s.logger(ctx, "failed to release lock for task[%s]: %s", input.ID, unlockErr.Error())
		}
	}()

	t, ok, err := s.taskStore.Get(ctx, input.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get task[%s]: %w", input.ID, err)
	}
	if !ok {
		return nil, fmt.Errorf("task[%s] not found", input.ID)
	}
	if input.HistoryLength != nil && len(t.History) > *input.HistoryLength {
		t.History = t.History[:*input.HistoryLength]
	}
	return t, nil
}

func (s *A2AServer) cancelTask(ctx context.Context, input *models.TaskIDParams) (*models.Task, error) {
	var err error
	err = s.taskLocker.Lock(ctx, input.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire lock for new task[%s]: %w", input.ID, err)
	}
	defer func() {
		unlockErr := s.taskLocker.Unlock(ctx, input.ID)
		if unlockErr != nil {
			s.logger(ctx, "failed to release lock for task[%s]: %s", input.ID, unlockErr.Error())
		}
	}()

	t, ok, err := s.taskStore.Get(ctx, input.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get task[%s]: %w", input.ID, err)
	}
	if !ok {
		return nil, fmt.Errorf("task[%s] not found", input.ID)
	}
	if t == nil {
		return nil, fmt.Errorf("task[%s] is nil", input.ID)
	}

	resp, err := s.cancelTaskHandler(ctx, &InputParams{
		Task:     t,
		Metadata: input.Metadata,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to cancel task[%s]: %w", input.ID, err)
	}

	t = loadTaskContext(t, resp)

	err = s.taskStore.Save(ctx, t)
	if err != nil {
		return nil, fmt.Errorf("failed to save canceled task: %w", err)
	}

	return t, nil
}

func (s *A2AServer) sendMessage(ctx context.Context, input *models.MessageSendParams) (*models.SendMessageResponseUnion, error) {
	var err error
	var t *models.Task
	if input.Message.TaskID != nil {
		err = s.taskLocker.Lock(ctx, *input.Message.TaskID)
		if err != nil {
			return nil, fmt.Errorf("failed to acquire lock for task[%s]: %s", *input.Message.TaskID, err)
		}
		defer func() {
			unLockErr := s.taskLocker.Unlock(ctx, *input.Message.TaskID)
			if unLockErr != nil {
				s.logger(ctx, "failed to release lock for task[%s]: %s", *input.Message.TaskID, unLockErr.Error())
			}
		}()

		var ok bool
		t, ok, err = s.taskStore.Get(ctx, *input.Message.TaskID)
		if err != nil {
			return nil, fmt.Errorf("failed to get task[%s]: %s", *input.Message.TaskID, err)
		}
		if !ok {
			return nil, fmt.Errorf("task[%s] not found", *input.Message.TaskID)
		}
		if t == nil {
			return nil, fmt.Errorf("task[%s] is nil", *input.Message.TaskID)
		}
	} else {
		t, err = s.initTask(ctx)
		if err != nil {
			return nil, err
		}

		err = s.taskLocker.Lock(ctx, t.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to acquire lock for new task[%s]: %s", t.ID, err)
		}
		defer func() {
			err = s.taskLocker.Unlock(ctx, t.ID)
			if err != nil {
				s.logger(ctx, "failed to release lock for task[%s]: %s", t.ID, err.Error())
			}
		}()
	}

	// register notification
	if input.Configuration != nil && input.Configuration.PushNotificationConfig != nil && s.pushNotifier != nil {
		err = s.pushNotifier.Set(ctx, &models.TaskPushNotificationConfig{
			TaskID:                 t.ID,
			PushNotificationConfig: *input.Configuration.PushNotificationConfig,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to set push notification config for task[%s]: %s", t.ID, err)
		}
	}

	resp, err := s.messageHandler(ctx, &InputParams{
		Task:     t,
		Input:    &input.Message,
		Metadata: input.Metadata,
	})
	if err != nil {
		return nil, err
	}
	resp.EnsureRequiredFields()

	t = loadTaskContext(t, resp)
	err = s.taskStore.Save(ctx, t)
	if err != nil {
		return nil, fmt.Errorf("failed to save task: %w", err)
	}

	frame := wrapSendMessageStreamingResponseUnion(models.ResponseEvent{TaskContent: resp}, t.ID, t.ContextID)
	if s.pushNotifier != nil {
		go func() {
			defer func() {
				e := recover()
				if e != nil {
					s.logger(ctx, "panic when send notification of task[%s]: %v", t.ID, utils.NewPanicErr(e, debug.Stack()))
				}
			}()
			err = s.pushNotifier.SendNotification(ctx, frame)
			if err != nil {
				s.logger(ctx, "failed to push notification to notifier: %s", err)
			}
		}()
	}

	return &models.SendMessageResponseUnion{
		Message: frame.Message,
		Task:    frame.Task,
	}, nil
}

func (s *A2AServer) sendMessageStreaming(ctx context.Context, input *models.MessageSendParams, writer models.ResponseWriter) error {
	var err error
	var t *models.Task

	needReleaseLock := false // need release lock after acquire and before start async execute
	defer func() {
		if needReleaseLock {
			var tid string
			if input.Message.TaskID != nil {
				tid = *input.Message.TaskID
			} else {
				tid = t.ID
			}
			unlockErr := s.taskLocker.Unlock(ctx, tid)
			if unlockErr != nil {
				s.logger(ctx, "failed to release lock for task[%s]: %s", tid, unlockErr.Error())
			}
		}
		writer.Close()
	}()

	if input.Message.TaskID != nil {
		err = s.taskLocker.Lock(ctx, *input.Message.TaskID)
		if err != nil {
			return fmt.Errorf("failed to acquire lock for task[%s]: %w", *input.Message.TaskID, err)
		}
		needReleaseLock = true

		var ok bool
		t, ok, err = s.taskStore.Get(ctx, *input.Message.TaskID)
		if err != nil {
			return fmt.Errorf("failed to get task[%s]: %w", *input.Message.TaskID, err)
		}
		if !ok {
			return fmt.Errorf("task[%s] not found", *input.Message.TaskID)
		}
		if t == nil {
			return fmt.Errorf("task[%s] is nil", *input.Message.TaskID)
		}
	} else {
		t, err = s.initTask(ctx)
		if err != nil {
			return err
		}

		err = s.taskLocker.Lock(ctx, t.ID)
		if err != nil {
			return fmt.Errorf("failed to acquire lock for new task[%s]: %s", t.ID, err)
		}
		needReleaseLock = true
	}

	err = s.queue.Reset(ctx, t.ID)
	if err != nil {
		return fmt.Errorf("failed to reset queue for new task[%s]: %w", t.ID, err)
	}

	// register notification
	if input.Configuration != nil && input.Configuration.PushNotificationConfig != nil && s.pushNotifier != nil {
		err = s.pushNotifier.Set(ctx, &models.TaskPushNotificationConfig{
			TaskID:                 t.ID,
			PushNotificationConfig: *input.Configuration.PushNotificationConfig,
		})
		if err != nil {
			return fmt.Errorf("failed to set push notification config for task[%s]: %w", t.ID, err)
		}
	}

	needReleaseLock = false // will release after execution

	go func() {
		defer func() {
			e := recover()
			if e != nil {
				panicErr := utils.NewPanicErr(e, debug.Stack())
				err = s.queue.Push(ctx, t.ID, nil, panicErr)
				if err != nil {
					s.logger(ctx, "failed to push panic event, task: %s, push error: %v, panic: %v", t.ID, err, panicErr)
				}
			}

			err = s.queue.Close(ctx, t.ID)
			if err != nil {
				s.logger(ctx, "failed to close queue for task[%s]: %s", t.ID, err.Error())
			}
			err = s.taskLocker.Unlock(ctx, t.ID)
			if err != nil {
				s.logger(ctx, "failed to release lock for task[%s]: %s", t.ID, err.Error())
			}
		}()
		sr := &streamResponseWriter{
			taskID:    t.ID,
			contextID: t.ContextID,
			ctx:       ctx,
			s:         s,
		}
		err = s.messageStreamingHandler(ctx, &InputParams{
			Task:     t,
			Input:    &input.Message,
			Metadata: input.Metadata,
		}, sr)
		if err != nil {
			pushErr := s.queue.Push(ctx, t.ID, nil, err)
			if err != nil {
				s.logger(ctx, "failed to push task[%s] error[%v] to queue: %v", t.ID, err, pushErr)
			}
		}

		tc := s.taskEventsConsolidator(ctx, t, sr.events, err)
		t = loadTaskContext(t, tc)

		err = s.taskStore.Save(ctx, t)
		if err != nil {
			pushErr := s.queue.Push(ctx, t.ID, nil, err)
			if err != nil {
				s.logger(ctx, "failed to save task: %v, and failed to push task[%s] to queue: %v", err, t.ID, pushErr)
			}
			return
		}
	}()

	for {
		resp, taskErr, closed, err := s.queue.Pop(ctx, t.ID)
		if err != nil {
			return fmt.Errorf("failed to pop task[%s]: %w", t.ID, err)
		}
		if closed {
			return nil
		}
		if taskErr != nil {
			return taskErr // agent execute error
		}
		err = writer.Write(ctx, resp)
		if err != nil {
			return fmt.Errorf("failed to send response of task[%s]: %w", t.ID, err)
		}
	}
}

func buildMessageHandlerByStream(sh MessageStreamingHandler, tm TaskEventsConsolidator) MessageHandler {
	return func(ctx context.Context, params *InputParams) (*models.TaskContent, error) {
		localWriter := &localResponseEventWriter{
			mu:     sync.Mutex{},
			events: make([]models.ResponseEvent, 0),
		}
		err := sh(ctx, params, localWriter)
		return tm(ctx, params.Task, localWriter.events, err), nil
	}
}

type localResponseEventWriter struct {
	mu     sync.Mutex
	events []models.ResponseEvent
}

func (w *localResponseEventWriter) Write(event models.ResponseEvent) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.events = append(w.events, event)
	return nil
}

type streamResponseWriter struct {
	taskID    string
	contextID string

	ctx    context.Context
	s      *A2AServer
	events []models.ResponseEvent
	mu     sync.Mutex
}

func (r *streamResponseWriter) Write(p models.ResponseEvent) (err error) {
	r.mu.Lock()
	defer func() {
		r.mu.Unlock()
		if err == nil {
			r.events = append(r.events, p)
		}
	}()
	(&p).EnsureRequiredFields()
	chunk := wrapSendMessageStreamingResponseUnion(p, r.taskID, r.contextID)

	err = r.s.queue.Push(r.ctx, r.taskID, chunk, nil)
	if err != nil {
		return fmt.Errorf("failed to push chunk to queue: %s", err)
	}

	if r.s.pushNotifier != nil {
		go func() {
			defer func() {
				e := recover()
				if e != nil {
					r.s.logger(r.ctx, "panic when send notification of task[%s]: %v", r.taskID, utils.NewPanicErr(e, debug.Stack()))
				}
			}()
			err = r.s.pushNotifier.SendNotification(r.ctx, chunk)
			if err != nil {
				r.s.logger(r.ctx, "failed to push notification to notifier: %s", err)
			}
		}()
	}

	return nil
}

func (s *A2AServer) resubscribeTask(ctx context.Context, input *models.TaskIDParams, writer models.ResponseWriter) error {
	// events in queue can only be read once, so if multi clients subscribe one task, the events clients received will be incompleted.
	defer writer.Close()
	for {
		resp, taskErr, closed, err := s.queue.Pop(ctx, input.ID)
		if err != nil {
			return fmt.Errorf("failed to pop task[%s]: %w", input.ID, err)
		}
		if closed {
			return nil
		}
		if taskErr != nil {
			return taskErr
		}
		err = writer.Write(ctx, resp)
		if err != nil {
			return fmt.Errorf("failed to send response: %w", err)
		}
	}
}

func (s *A2AServer) setTasksPushNotificationConfig(ctx context.Context, input *models.TaskPushNotificationConfig) (*models.TaskPushNotificationConfig, error) {
	return input, s.pushNotifier.Set(ctx, input)
}

func (s *A2AServer) getTasksPushNotificationConfig(ctx context.Context, input *models.GetTaskPushNotificationConfigParams) (*models.TaskPushNotificationConfig, error) {
	conf, existed, err := s.pushNotifier.Get(ctx, input.PushNotificationConfigID)
	if err != nil {
		return nil, fmt.Errorf("failed to get push notification config for task[%s]: %w", input.PushNotificationConfigID, err)
	}
	if !existed {
		return nil, fmt.Errorf("task[%s] not found", input.PushNotificationConfigID)
	}
	return &models.TaskPushNotificationConfig{
		TaskID:                 input.PushNotificationConfigID,
		PushNotificationConfig: conf,
	}, nil
}

func (s *A2AServer) initTask(ctx context.Context) (*models.Task, error) {
	taskID, err := s.taskIDGenerator(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to generate task id: %w", err)
	}
	contextID, err := s.contextIDGenerator(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to generate context id: %w", err)
	}
	return &models.Task{
		ID:        taskID,
		ContextID: contextID,
		Status: models.TaskStatus{
			State:     models.TaskStateSubmitted,
			Timestamp: time.Now().Format(time.RFC3339),
		},
		History: []*models.Message{},
	}, nil
}

func wrapSendMessageStreamingResponseUnion(sr models.ResponseEvent, taskID, contextID string) *models.SendMessageStreamingResponseUnion {
	if sr.Message != nil {
		return &models.SendMessageStreamingResponseUnion{
			Message: &models.Message{
				TaskID:           &taskID,
				ContextID:        &contextID,
				MessageID:        sr.Message.MessageID,
				Role:             sr.Message.Role,
				Parts:            sr.Message.Parts,
				ReferenceTaskIDs: sr.Message.ReferenceTaskIDs,
				Metadata:         sr.Message.Metadata,
			}}
	} else if sr.TaskContent != nil {
		return &models.SendMessageStreamingResponseUnion{
			Task: &models.Task{
				ID:        taskID,
				ContextID: contextID,
				Status:    sr.TaskContent.Status,
				Artifacts: sr.TaskContent.Artifacts,
				History:   sr.TaskContent.History,
				Metadata:  sr.TaskContent.Metadata,
			},
		}
	} else if sr.TaskStatusUpdateEventContent != nil {
		return &models.SendMessageStreamingResponseUnion{
			TaskStatusUpdateEvent: &models.TaskStatusUpdateEvent{
				TaskID:    taskID,
				ContextID: contextID,
				Status:    sr.TaskStatusUpdateEventContent.Status,
				Final:     sr.TaskStatusUpdateEventContent.Final,
				Metadata:  sr.TaskStatusUpdateEventContent.Metadata,
			},
		}
	} else if sr.TaskArtifactUpdateEventContent != nil {
		return &models.SendMessageStreamingResponseUnion{
			TaskArtifactUpdateEvent: &models.TaskArtifactUpdateEvent{
				TaskID:    taskID,
				ContextID: contextID,
				Artifact:  sr.TaskArtifactUpdateEventContent.Artifact,
				Append:    sr.TaskArtifactUpdateEventContent.Append,
				LastChunk: sr.TaskArtifactUpdateEventContent.LastChunk,
				Metadata:  sr.TaskArtifactUpdateEventContent.Metadata,
			},
		}
	}
	return nil
}

func loadTaskContext(t *models.Task, tc *models.TaskContent) *models.Task {
	if t == nil {
		return nil
	}
	if tc == nil {
		return t
	}
	return &models.Task{
		ID:        t.ID,
		ContextID: t.ContextID,
		Status:    tc.Status,
		Artifacts: tc.Artifacts,
		History:   tc.History,
		Metadata:  tc.Metadata,
	}
}

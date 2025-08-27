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
	"encoding/gob"
	"fmt"
	"time"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/compose"
	"github.com/go-openapi/spec"
	"github.com/google/uuid"

	"github.com/cloudwego/eino-ext/a2a/models"
	"github.com/cloudwego/eino-ext/a2a/server"
	"github.com/cloudwego/eino-ext/a2a/transport"
)

func init() {
	gob.RegisterName("_eino_a2a_adk_interrupt_info", &InterruptInfo{})
}

type ServerConfig struct {
	Registrar transport.HandlerRegistrar

	// convert user input to agent run option
	AgentRunOptionConvertor func(ctx context.Context, t *models.Task, input *models.Message, metadata map[string]any) ([]adk.AgentRunOption, error)
	// optional, should be set if your agent will interrupt and resume
	// save agent interrupt status for resuming
	CheckPointStore compose.CheckPointStore
	// optional, convert a2a history to adk input messages
	HistoryMessageConvertor func(ctx context.Context, messages []*models.Message) ([]adk.Message, error)
	// convert user input to agent run option when resume
	ResumeConvertor func(ctx context.Context, t *models.Task, input *models.Message, metadata map[string]any) ([]adk.AgentRunOption, error)

	// optional. customized convertor to convert adk.AgentEvent to a2a models.ResponseEvent
	EventConvertor func(ctx context.Context, stream *adk.AsyncIterator[*adk.AgentEvent], writer func(p models.ResponseEvent) error) (err error)

	// the following 4 fields are used by default event convertor, if you customized event convertor, it's not necessary to configure these.

	// when interrupts have happened convert interrupt info to text for sending to client.
	InterruptInfoConvertor func(ctx context.Context, info *adk.InterruptInfo) (string, error)
	// optional, convert adk output message to a2a parts
	OutputMessageConvertor func(ctx context.Context, messages []adk.Message) ([]models.Part, error)
	// optional, uuid by default
	MessageIDGenerator  func(ctx context.Context) (string, error)
	ArtifactIDGenerator func(ctx context.Context) (string, error)

	// a2a server config

	// agent card
	URL                string
	Version            string
	DocumentationURL   string
	Provider           *models.AgentProvider
	SecuritySchemes    map[string]*spec.SecurityScheme
	Security           map[string][]string
	DefaultInputModes  []string
	DefaultOutputModes []string
	Skills             []models.AgentSkill
	// optional, uuid by default
	TaskIDGenerator func(ctx context.Context) (string, error)
	// optional, log.Printf by default
	Logger server.Logger
	// optional, in-memory by default
	TaskStore server.TaskStore
	// optional, in-memory by default
	TaskLocker server.TaskLocker
	// optional, in-memory by default
	Queue server.EventQueue
	// optional, in-memory by default
	PushNotifier server.PushNotifier
}

func RegisterServerHandlers(ctx context.Context, a adk.Agent, cfg *ServerConfig) error {
	if cfg == nil {
		cfg = &ServerConfig{}
	}

	builder := &a2aHandlersBuilder{
		agent:            a,
		cp:               cfg.CheckPointStore,
		runOptionConv:    cfg.AgentRunOptionConvertor,
		resumeConv:       cfg.ResumeConvertor,
		inputMessageConv: cfg.HistoryMessageConvertor,
		eventConvertor:   cfg.EventConvertor,
	}
	if builder.eventConvertor == nil {
		d := &defaultEventConvertor{
			messageIDGen:      cfg.MessageIDGenerator,
			artifactIDGen:     cfg.ArtifactIDGenerator,
			interruptInfoConv: cfg.InterruptInfoConvertor,
			outputMessageConv: cfg.OutputMessageConvertor,
		}
		if d.messageIDGen == nil {
			d.messageIDGen = func(ctx context.Context) (string, error) {
				return uuid.New().String(), nil
			}
		}
		if d.artifactIDGen == nil {
			d.artifactIDGen = func(ctx context.Context) (string, error) {
				return uuid.New().String(), nil
			}
		}
		if d.interruptInfoConv == nil {
			d.interruptInfoConv = func(ctx context.Context, info *adk.InterruptInfo) (string, error) {
				return sonic.MarshalString(info)
			}
		}
		if d.outputMessageConv == nil {
			d.outputMessageConv = messages2Parts
		}
		builder.eventConvertor = d.handlerEventIter
	}

	if builder.inputMessageConv == nil {
		builder.inputMessageConv = func(_ context.Context, messages []*models.Message) ([]adk.Message, error) {
			return toADKMessages(messages), nil
		}
	}

	return server.RegisterHandlers(ctx, cfg.Registrar, &server.Config{
		AgentCardConfig: server.AgentCardConfig{
			Name:               a.Name(ctx),
			Description:        a.Description(ctx),
			URL:                cfg.URL,
			Version:            cfg.Version,
			DocumentationURL:   cfg.DocumentationURL,
			Provider:           cfg.Provider,
			SecuritySchemes:    cfg.SecuritySchemes,
			Security:           cfg.Security,
			DefaultInputModes:  cfg.DefaultInputModes,
			DefaultOutputModes: cfg.DefaultOutputModes,
			Skills:             cfg.Skills,
		},
		MessageStreamingHandler: builder.buildStreamHandler(),
		TaskIDGenerator:         cfg.TaskIDGenerator,
		CancelTaskHandler:       builder.buildTaskCanceler(),
		TaskEventsConsolidator:  builder.einoResponseEventConcatenator,
		Logger:                  cfg.Logger,
		TaskStore:               cfg.TaskStore,
		TaskLocker:              cfg.TaskLocker,
		Queue:                   cfg.Queue,
		PushNotifier:            cfg.PushNotifier,
	})
}

type a2aHandlersBuilder struct {
	agent                     adk.Agent
	cp                        compose.CheckPointStore
	runOptionConv, resumeConv func(ctx context.Context, t *models.Task, input *models.Message, metadata map[string]any) ([]adk.AgentRunOption, error)
	inputMessageConv          func(ctx context.Context, messages []*models.Message) ([]adk.Message, error)

	eventConvertor func(ctx context.Context, event *adk.AsyncIterator[*adk.AgentEvent], writer func(p models.ResponseEvent) error) (err error)
}

func (a *a2aHandlersBuilder) buildStreamHandler() server.MessageStreamingHandler {
	return func(ctx context.Context, params *server.InputParams, writer server.ResponseEventWriter) error {
		iter, err := a.genIter(ctx, params.Task, params.Input, params.Metadata)
		if err != nil {
			return err
		}

		err = a.eventConvertor(ctx, iter, writer.Write)
		if err != nil {
			return err
		}
		return nil
	}
}

func (a *a2aHandlersBuilder) genIter(
	ctx context.Context,
	t *models.Task,
	input *models.Message,
	metadata map[string]any,
) (iter *adk.AsyncIterator[*adk.AgentEvent], err error) {
	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           a.agent,
		CheckPointStore: a.cp,
		EnableStreaming: getEnableStreaming(metadata),
	})
	var opts []adk.AgentRunOption
	if a.runOptionConv != nil {
		opts, err = a.runOptionConv(ctx, t, input, metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to convert user input to agent run options: %w", err)
		}
	}
	if getInterrupted(t.Metadata) {
		if a.resumeConv != nil {
			var rOpts []adk.AgentRunOption
			rOpts, err = a.resumeConv(ctx, t, input, metadata)
			if err != nil {
				return nil, fmt.Errorf("failed to convert user input to resume agent run options: %w", err)
			}
			opts = append(opts, rOpts...)
		}
		iter, err = runner.Resume(ctx, t.ID, opts...)
		if err != nil {
			return nil, err
		}
	} else {
		in, err := a.inputMessageConv(ctx, append(t.History, input))
		if err != nil {
			return nil, fmt.Errorf("failed to convert a2a history to adk input: %w", err)
		}
		iter = runner.Run(ctx, in, append(opts, adk.WithCheckPointID(t.ID))...)
	}
	return iter, nil
}

func (a *a2aHandlersBuilder) buildTaskCanceler() server.CancelTaskHandler {
	return func(ctx context.Context, params *server.InputParams) (*models.TaskContent, error) {
		ret := &models.TaskContent{
			Status:    params.Task.Status,
			History:   params.Task.History,
			Artifacts: params.Task.Artifacts,
			Metadata:  params.Metadata,
		}
		return ret, nil // todo: how to cancel
	}
}

type defaultEventConvertor struct {
	messageIDGen, artifactIDGen func(ctx context.Context) (string, error)
	interruptInfoConv           func(ctx context.Context, info *adk.InterruptInfo) (string, error)
	outputMessageConv           func(ctx context.Context, messages []adk.Message) ([]models.Part, error)
}

func (d *defaultEventConvertor) handlerEventIter(
	ctx context.Context,
	iter *adk.AsyncIterator[*adk.AgentEvent],
	writer func(p models.ResponseEvent) error,
) error {
	for {
		ret, ok := iter.Next()
		if !ok {
			// send final status update
			err := writer(models.ResponseEvent{
				TaskStatusUpdateEventContent: &models.TaskStatusUpdateEventContent{
					Status: models.TaskStatus{
						State:     models.TaskStateCompleted,
						Timestamp: time.Now().Format(time.RFC3339),
					},
					Final: true,
				},
			})
			if err != nil {
				return err
			}
			break
		}

		if ret.Err != nil {
			return fmt.Errorf("failed to execute agent: %w", ret.Err)
		}

		interrupted, err := d.convAgentEvent(ctx, ret, writer)
		if err != nil {
			return err
		}
		if interrupted {
			break
		}
	}
	return nil
}

func (d *defaultEventConvertor) convAgentEvent(
	ctx context.Context,
	event *adk.AgentEvent,
	writer func(p models.ResponseEvent) error,
) (interrupted bool, err error) {
	if event == nil {
		return false, nil
	}
	// 1. tool output(success transfer to xxx agent) + transfer action -> status
	// 2. interrupt -> status update
	// 3. iter closed -> final statue update
	// 3. llm output -> artifact
	// 4. tool output -> artifact
	// todo: customized output and action

	// transfer
	if event.Action != nil && event.Action.TransferToAgent != nil {
		text := fmt.Sprintf("transfer from agent[%s] to agent[%s]", event.AgentName, event.Action.TransferToAgent.DestAgentName) // todo: how
		messageID, err := d.messageIDGen(ctx)
		if err != nil {
			return false, fmt.Errorf("failed to generate message ID: %w", err)
		}
		return false, writer(
			models.ResponseEvent{
				TaskStatusUpdateEventContent: &models.TaskStatusUpdateEventContent{
					Status: models.TaskStatus{
						State: models.TaskStateWorking,
						Message: &models.Message{
							Role:      models.RoleAgent,
							Parts:     []models.Part{{Kind: models.PartKindText, Text: &text}},
							MessageID: messageID,
						},
						Timestamp: time.Now().Format(time.RFC3339),
					},
				},
			},
		)
	}

	// interrupt
	if event.Action != nil && event.Action.Interrupted != nil {
		data, err := d.interruptInfoConv(ctx, event.Action.Interrupted)
		if err != nil {
			return false, fmt.Errorf("failed to marshal interrupted info: %w", err)
		}
		metadata := map[string]any{}
		setInterrupted(metadata)
		messageID, err := d.messageIDGen(ctx)
		if err != nil {
			return false, fmt.Errorf("failed to generate message ID: %w", err)
		}
		return true, writer(models.ResponseEvent{
			TaskStatusUpdateEventContent: &models.TaskStatusUpdateEventContent{
				Status: models.TaskStatus{
					State: models.TaskStateInputRequired,
					Message: &models.Message{
						MessageID: messageID,
						Role:      models.RoleAgent,
						Parts: []models.Part{
							{
								Kind: models.PartKindText,
								Text: &data,
							},
						},
					},
					Timestamp: time.Now().Format(time.RFC3339),
				},
				Metadata: metadata,
			},
		})
	}

	// llm&tool output
	if event.Output != nil && event.Output.MessageOutput != nil {
		return false, d.messageVar2Status(ctx, event.AgentName, event.Output.MessageOutput, writer)
	}

	// empty agent event
	return false, nil
}

func (d *defaultEventConvertor) messageVar2Status(ctx context.Context, agentName string, messageVar *adk.MessageVariant, writer func(p models.ResponseEvent) (err error)) error {
	artifactID, err := d.artifactIDGen(ctx)
	if err != nil {
		return fmt.Errorf("failed to generate message ID: %w", err)
	}
	m, err := messageVar.GetMessage()
	if err != nil {
		return fmt.Errorf("failed to get message: %w", err)
	}
	p, convErr := d.outputMessageConv(ctx, []adk.Message{m})
	if convErr != nil {
		return convErr
	}
	if p != nil {
		return writer(models.ResponseEvent{
			TaskArtifactUpdateEventContent: &models.TaskArtifactUpdateEventContent{
				Artifact: models.Artifact{
					ArtifactID: artifactID,
					Name:       agentName,
					Parts:      p,
				},
				LastChunk: true,
			},
		})
	}
	return nil
}

func (a *a2aHandlersBuilder) einoResponseEventConcatenator(ctx context.Context, t *models.Task, events []models.ResponseEvent, _ error) *models.TaskContent {
	tc := &models.TaskContent{
		Status:    t.Status,
		Artifacts: t.Artifacts,
		History:   t.History,
		Metadata:  t.Metadata,
	}
	if tc.Metadata == nil {
		tc.Metadata = make(map[string]any)
	}

	artifacts := make(map[string][]*models.Artifact)
	var lastMessage *models.Message
	for _, event := range events {
		if event.Message != nil {
			tc.History = append(tc.History, event.Message)

			lastMessage = event.Message
		} else if event.TaskContent != nil {
			if tc.Status.Message != nil {
				tc.History = append(tc.History, tc.Status.Message)
			}
			tc.History = append(t.History, event.TaskContent.History...)
			tc.Artifacts = append(t.Artifacts, event.TaskContent.Artifacts...)
			tc.Status = event.TaskContent.Status

			if tc.Status.Message != nil {
				lastMessage = tc.Status.Message
			}
		} else if event.TaskStatusUpdateEventContent != nil {
			// save last status message to history
			if tc.Status.Message != nil {
				tc.History = append(tc.History, tc.Status.Message)
			}

			// set new status
			tc.Status = event.TaskStatusUpdateEventContent.Status

			for k, v := range event.TaskStatusUpdateEventContent.Metadata {
				// save interrupt
				tc.Metadata[k] = v
			}

			if tc.Status.Message != nil {
				lastMessage = tc.Status.Message
			}
		} else if event.TaskArtifactUpdateEventContent != nil {
			if _, ok := artifacts[event.TaskArtifactUpdateEventContent.Artifact.ArtifactID]; !ok {
				artifacts[event.TaskArtifactUpdateEventContent.Artifact.ArtifactID] = []*models.Artifact{}
			}
			artifacts[event.TaskArtifactUpdateEventContent.Artifact.ArtifactID] = append(artifacts[event.TaskArtifactUpdateEventContent.Artifact.ArtifactID], &event.TaskArtifactUpdateEventContent.Artifact)

			if event.TaskArtifactUpdateEventContent.LastChunk {
				newA := concatArtifacts(artifacts[event.TaskArtifactUpdateEventContent.Artifact.ArtifactID])
				artifacts[event.TaskArtifactUpdateEventContent.Artifact.ArtifactID] = []*models.Artifact{}

				tc.Artifacts = append(tc.Artifacts, newA)

				if newA != nil {
					lastMessage = &models.Message{
						Role:      models.RoleAgent,
						Parts:     newA.Parts,
						Metadata:  newA.Metadata,
						MessageID: newA.ArtifactID,
					}
				}
			}
		}
	}

	if lastMessage != nil {
		tc.Status.Message = lastMessage
	}

	return tc
}

func concatMessages(messages []*models.Message) *models.Message {
	if len(messages) == 0 {
		return nil
	}
	ret := &models.Message{}
	for _, m := range messages {
		if m == nil {
			continue
		}
		if len(m.Role) > 0 {
			ret.Role = m.Role
		}
		if len(m.MessageID) > 0 {
			ret.MessageID = m.MessageID
		}
		if m.TaskID != nil {
			ret.TaskID = m.TaskID
		}
		if len(m.MessageID) > 0 {
			ret.MessageID = m.MessageID
		}
		if m.ContextID != nil {
			ret.ContextID = m.ContextID
		}
		if len(m.ReferenceTaskIDs) > 0 {
			ret.ReferenceTaskIDs = m.ReferenceTaskIDs
		}
		for k, v := range m.Metadata {
			ret.Metadata[k] = v
		}

		ret.Parts = concatParts(ret.Parts, m.Parts)
	}
	return ret
}

func concatArtifacts(artifacts []*models.Artifact) *models.Artifact {
	if len(artifacts) == 0 {
		return nil
	}
	if len(artifacts) == 1 {
		return artifacts[0]
	}
	ret := &models.Artifact{}
	for _, artifact := range artifacts {
		if artifact == nil {
			continue
		}
		if len(artifact.ArtifactID) > 0 {
			ret.ArtifactID = artifact.ArtifactID
		}
		if len(artifact.Name) > 0 {
			ret.Name = artifact.Name
		}
		if len(artifact.Description) > 0 {
			ret.Description = artifact.Description
		}
		for k, v := range artifact.Metadata {
			ret.Metadata[k] = v
		}

		ret.Parts = concatParts(ret.Parts, artifact.Parts)
	}
	return ret
}

func concatParts(old, new []models.Part) []models.Part {
	return append(old, new...) // todo: how to concat parts
}

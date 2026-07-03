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
	"errors"
	"fmt"
	"io"
	"runtime/debug"
	"sync"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/a2a/client"
	"github.com/cloudwego/eino-ext/a2a/models"
	"github.com/cloudwego/eino-ext/a2a/utils"
)

type AgentConfig struct {
	Client *client.A2AClient

	// optional, from AgentCard by default
	Name        *string
	Description *string
	Streaming   *bool // use streaming first if have not set this field and agent support

	// InputMessageConvertor allows users to convert adk messages to a2a message
	// Optional.
	InputMessageConvertor func(ctx context.Context, messages []*schema.Message) (models.Message, error)

	OutputConvertor func(ctx context.Context, receiver *ResponseUnionReceiver, sender *AgentEventSender)
	// todo: support notification?
}

func NewAgent(ctx context.Context, cfg AgentConfig) (adk.Agent, error) {
	if cfg.Client == nil {
		return nil, errors.New("Client is required")
	}
	var name, desc string
	var streaming bool
	if cfg.Name == nil || cfg.Description == nil || cfg.Streaming == nil {
		card, err := cfg.Client.AgentCard(ctx)
		if err != nil {
			return nil, err
		}
		name = card.Name
		desc = card.Description
		streaming = card.Capabilities.Streaming
	}
	if cfg.Name != nil {
		name = *cfg.Name
	}
	if cfg.Description != nil {
		desc = *cfg.Description
	}
	if cfg.Streaming != nil {
		streaming = *cfg.Streaming
	}

	a := &a2aAgent{
		name:                  name,
		description:           desc,
		streaming:             streaming,
		inputMessageConvertor: cfg.InputMessageConvertor,
		outputConvertor:       cfg.OutputConvertor,
		cli:                   cfg.Client,
	}
	if a.inputMessageConvertor == nil {
		a.inputMessageConvertor = func(ctx context.Context, messages []*schema.Message) (models.Message, error) {
			p, err := messages2Parts(ctx, messages)
			if err != nil {
				return models.Message{}, err
			}
			return models.Message{
				Role:  models.RoleUser,
				Parts: p,
			}, nil
		}
	}
	if a.outputConvertor == nil {
		a.outputConvertor = defaultOutputConvertor
	}
	return a, nil
}

type InterruptInfo struct {
	TaskID           string
	InterruptMessage adk.Message
}

type options struct {
	metadata       map[string]any
	resumeMessages []*schema.Message
}

func WithResumeMessages(msgs []*schema.Message) adk.AgentRunOption {
	return adk.WrapImplSpecificOptFn(func(o *options) {
		o.resumeMessages = msgs
	})
}

func WithMetadata(metadata map[string]any) adk.AgentRunOption {
	return adk.WrapImplSpecificOptFn(func(o *options) {
		o.metadata = metadata
	})
}

type a2aAgent struct {
	name        string
	description string
	streaming   bool

	inputMessageConvertor func(ctx context.Context, messages []*schema.Message) (models.Message, error)

	outputConvertor func(ctx context.Context, message *ResponseUnionReceiver, sender *AgentEventSender)

	cli *client.A2AClient
}

func (a *a2aAgent) Name(ctx context.Context) string {
	return a.name
}

func (a *a2aAgent) Description(ctx context.Context) string {
	return a.description
}

func (a *a2aAgent) Run(ctx context.Context, input *adk.AgentInput, opts ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	o := adk.GetImplSpecificOptions(&options{}, opts...)
	m, err := a.inputMessageConvertor(ctx, input.Messages)
	if err != nil {
		iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
		gen.Send(&adk.AgentEvent{Err: fmt.Errorf("failed to convert adk messages to a2a message: %w", err)})
		gen.Close()
		return iter
	}

	return a.run(ctx, m, input.EnableStreaming, o.metadata)
}

func (a *a2aAgent) Resume(ctx context.Context, info *adk.ResumeInfo, opts ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	if info == nil {
		// unreachable
		iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
		gen.Send(&adk.AgentEvent{Err: fmt.Errorf("empty resume info")})
		gen.Close()
		return iter
	}
	ii, ok := info.InterruptInfo.Data.(*InterruptInfo)
	if !ok {
		// unreachable
		iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
		gen.Send(&adk.AgentEvent{Err: fmt.Errorf("resume info's data type[%T] is unexpected", info.InterruptInfo.Data)})
		gen.Close()
		return iter
	}

	o := adk.GetImplSpecificOptions(&options{}, opts...)

	msg, err := a.inputMessageConvertor(ctx, o.resumeMessages)
	if err != nil {
		iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
		gen.Send(&adk.AgentEvent{Err: fmt.Errorf("failed to convert adk messages to a2a message: %w", err)})
		gen.Close()
		return iter
	}
	msg.TaskID = &ii.TaskID
	return a.run(ctx, msg, info.EnableStreaming, o.metadata)
}

func (a *a2aAgent) run(ctx context.Context, msg models.Message, streaming bool, metadata map[string]any) *adk.AsyncIterator[*adk.AgentEvent] {
	iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()

	if streaming {
		if metadata == nil {
			metadata = make(map[string]any)
		}
		setEnableStreaming(metadata)
	}

	var receiver *ResponseUnionReceiver
	if a.streaming {
		stream, err := a.cli.SendMessageStreaming(ctx, &models.MessageSendParams{
			Message:  msg,
			Metadata: metadata,
		})
		if err != nil {
			gen.Send(&adk.AgentEvent{
				AgentName: a.Name(ctx),
				Err:       err,
			})
			gen.Close()
			return iter
		}
		receiver = &ResponseUnionReceiver{stream, true}
	} else {
		result, err := a.cli.SendMessage(ctx, &models.MessageSendParams{
			Message:  msg,
			Metadata: metadata,
		})
		if err != nil {
			gen.Send(&adk.AgentEvent{
				AgentName: a.Name(ctx),
				Err:       err,
			})
			return iter
		}

		var union *models.SendMessageStreamingResponseUnion
		if result != nil {
			union = &models.SendMessageStreamingResponseUnion{
				Message: result.Message,
				Task:    result.Task,
			}
		}

		receiver = &ResponseUnionReceiver{&localResponseUnionReceiver{
			mu:    sync.Mutex{},
			union: union,
			final: false,
		}, false}
	}
	go func() {
		defer func() {
			e := recover()
			if e != nil {
				gen.Send(&adk.AgentEvent{Err: utils.NewPanicErr(e, debug.Stack())})
			}
			gen.Close()
		}()
		a.outputConvertor(ctx, receiver, &AgentEventSender{gen: gen})
	}()

	return iter
}

type AgentEventSender struct {
	gen *adk.AsyncGenerator[*adk.AgentEvent]
}

func (a *AgentEventSender) Send(event *adk.AgentEvent) {
	a.gen.Send(event)
}

type responseUnionReceiver interface {
	Recv() (resp *models.SendMessageStreamingResponseUnion, err error)
	Close() error
}

type localResponseUnionReceiver struct {
	mu    sync.Mutex
	union *models.SendMessageStreamingResponseUnion
	final bool
}

func (l *localResponseUnionReceiver) Recv() (resp *models.SendMessageStreamingResponseUnion, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.final {
		return resp, io.EOF
	}
	l.final = true
	return l.union, nil
}

func (l *localResponseUnionReceiver) Close() error {
	return nil
}

type ResponseUnionReceiver struct {
	responseUnionReceiver
	streaming bool
}

func (r *ResponseUnionReceiver) IsStreaming() bool {
	return r.streaming
}

func defaultOutputConvertor(ctx context.Context, stream *ResponseUnionReceiver, sender *AgentEventSender) {
	artifactMap := make(map[string] /*artifact id*/ *schema.StreamWriter[*schema.Message])
	messageMap := make(map[string] /*message id*/ *schema.StreamWriter[*schema.Message])
	defer func() {
		for _, sw := range artifactMap {
			sw.Close()
		}
		for _, sw := range messageMap {
			sw.Close()
		}
	}()

	for {
		event, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			sender.Send(&adk.AgentEvent{Err: err})
			break
		}
		if event.Message != nil {
			handleMessageEvent(event.Message, messageMap, sender)
		} else if event.Task != nil {
			if handleTaskEvent(event.Task, stream.IsStreaming(), messageMap, sender) {
				return
			}
		} else if event.TaskStatusUpdateEvent != nil {
			if handleTaskStatusUpdateEvent(event.TaskStatusUpdateEvent, stream.IsStreaming(), messageMap, sender) {
				return
			}
		} else if event.TaskArtifactUpdateEvent != nil {
			m := artifact2ADKMessage(&event.TaskArtifactUpdateEvent.Artifact)
			handleNewMessage(event.TaskArtifactUpdateEvent.Artifact.ArtifactID, artifactMap, m, event.TaskArtifactUpdateEvent.LastChunk, "", sender)
		}
	}
}

func handleMessageEvent(msg *models.Message, messageMap map[string]*schema.StreamWriter[*schema.Message], sender *AgentEventSender) {
	isChunk, final := getStreamChunkFinal(msg.Metadata)
	if isChunk {
		delete(msg.Metadata, metadataKeyOfStreamChunkFinal)
		if len(msg.Metadata) == 0 {
			msg.Metadata = nil
		}
		m := toADKMessage(msg)
		handleNewMessage(msg.MessageID, messageMap, m, final, schema.Assistant, sender)
	} else {
		m := toADKMessage(msg)
		sender.Send(&adk.AgentEvent{
			Output: &adk.AgentOutput{
				MessageOutput: &adk.MessageVariant{
					Message: m,
					Role:    schema.Assistant,
				},
			},
		})
	}
}

// handleTaskEvent returns true if the convertor should stop (interrupt).
func handleTaskEvent(task *models.Task, streaming bool, messageMap map[string]*schema.StreamWriter[*schema.Message], sender *AgentEventSender) bool {
	var m adk.Message
	if task.Status.Message != nil {
		m = toADKMessage(task.Status.Message)
	}

	if task.Status.State == models.TaskStateInputRequired {
		sender.Send(&adk.AgentEvent{
			Action: &adk.AgentAction{
				Interrupted: &adk.InterruptInfo{Data: &InterruptInfo{
					TaskID:           task.ID,
					InterruptMessage: m,
				}},
			},
		})
		return true
	}

	if m != nil {
		if streaming {
			id := task.Status.Message.MessageID
			if id == "" {
				id = task.ID
			}
			handleNewMessage(id, messageMap, m, isTerminalTaskState(task.Status.State), schema.Assistant, sender)
		} else {
			sender.Send(&adk.AgentEvent{
				Output: &adk.AgentOutput{
					MessageOutput: &adk.MessageVariant{
						Message: m,
						Role:    schema.Assistant,
					},
				},
			})
		}
	}
	return false
}

// handleTaskStatusUpdateEvent returns true if the convertor should stop (interrupt or final).
func handleTaskStatusUpdateEvent(event *models.TaskStatusUpdateEvent, streaming bool, messageMap map[string]*schema.StreamWriter[*schema.Message], sender *AgentEventSender) bool {
	var m adk.Message
	if event.Status.Message != nil {
		m = toADKMessage(event.Status.Message)
	}

	if event.Status.State == models.TaskStateInputRequired {
		sender.Send(&adk.AgentEvent{
			Action: &adk.AgentAction{
				Interrupted: &adk.InterruptInfo{Data: &InterruptInfo{
					TaskID:           event.TaskID,
					InterruptMessage: m,
				}},
			},
		})
		return true
	}

	if m != nil {
		if streaming {
			id := event.Status.Message.MessageID
			if id == "" {
				id = event.TaskID
			}
			handleNewMessage(id, messageMap, m, event.Final, schema.Assistant, sender)
		} else {
			sender.Send(&adk.AgentEvent{
				Output: &adk.AgentOutput{
					MessageOutput: &adk.MessageVariant{
						Message: m,
					},
				},
			})
		}
	}

	return event.Final
}

func handleNewMessage(id string, idMap map[string]*schema.StreamWriter[*schema.Message], msg *schema.Message, final bool, role schema.RoleType, sender *AgentEventSender) {
	// 1. check if the messageID has been recorded
	// 		if not,
	//			if Final == true, report directly.
	//			else record it.
	// 2. write new message to stream writer.
	// 3. if Final == true, close the stream writer and delete it from the map.
	sw, ok := idMap[id]
	if !ok {
		if final {
			sender.Send(&adk.AgentEvent{
				Output: &adk.AgentOutput{
					MessageOutput: &adk.MessageVariant{Message: msg, Role: role},
				},
			})
			return
		}
		var sr *schema.StreamReader[*schema.Message]
		sr, sw = schema.Pipe[*schema.Message](100) // todo: buffer size
		idMap[id] = sw
		sender.Send(&adk.AgentEvent{
			Output: &adk.AgentOutput{MessageOutput: &adk.MessageVariant{
				IsStreaming:   true,
				MessageStream: sr,
				Role:          role,
			}},
		})
	}
	closed := sw.Send(msg, nil) // todo: blocking?
	if closed || final {
		sw.Close()
		delete(idMap, id)
	}
}

func isTerminalTaskState(state models.TaskState) bool {
	switch state {
	case models.TaskStateCompleted, models.TaskStateFailed, models.TaskStateCanceled, models.TaskStateRejected:
		return true
	default:
		return false
	}
}

func convInputMessages(ctx context.Context, messages []adk.Message, inputMessageConvertor func(ctx context.Context, messages []*schema.Message) ([]models.Part, error)) (models.Message, error) {
	ret := models.Message{
		Role: models.RoleUser,
	}

	if inputMessageConvertor != nil {
		parts, err := inputMessageConvertor(ctx, messages)
		if err != nil {
			return ret, err
		}
		ret.Parts = parts
		return ret, nil
	}

	for _, m := range messages {
		ret.Parts = append(ret.Parts, message2Parts(m)...)
	}
	return ret, nil
}

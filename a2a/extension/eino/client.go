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
	"github.com/cloudwego/eino-ext/a2a/transport"
	"github.com/cloudwego/eino-ext/a2a/utils"
)

type AgentConfig struct {
	Transport transport.ClientTransport

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
	cli, err := client.NewA2AClient(ctx, &client.Config{
		Transport: cfg.Transport,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create a2a client: %w", err)
	}
	var name, desc string
	var streaming bool
	if cfg.Name == nil || cfg.Description == nil || cfg.Streaming == nil {
		card, err := cli.AgentCard(ctx)
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
		cli:                   cli,
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
		receiver = &ResponseUnionReceiver{stream}
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
		}}
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
}

func defaultOutputConvertor(ctx context.Context, stream *ResponseUnionReceiver, sender *AgentEventSender) {
	artifactMap := make(map[string] /*artifact id*/ *schema.StreamWriter[*schema.Message])
	defer func() {
		for _, sw := range artifactMap {
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
		}
		if event.Message != nil {
			m := toADKMessage(event.Message)
			sender.Send(&adk.AgentEvent{
				Output: &adk.AgentOutput{
					MessageOutput: &adk.MessageVariant{
						Message: m,
						Role:    schema.Assistant,
					},
				},
			})
		} else if event.Task != nil {
			var m adk.Message
			if event.Task.Status.Message != nil {
				m = toADKMessage(event.Task.Status.Message)
			} else {
				m = schema.AssistantMessage(string(event.Task.Status.State), nil)
			}
			// check interrupt
			if event.Task.Status.State == models.TaskStateInputRequired {
				sender.Send(&adk.AgentEvent{
					Action: &adk.AgentAction{
						Interrupted: &adk.InterruptInfo{Data: &InterruptInfo{
							TaskID:           event.Task.ID,
							InterruptMessage: m,
						}},
					},
				})
				return
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
		} else if event.TaskStatusUpdateEvent != nil {
			statusUpdateEvent := event.TaskStatusUpdateEvent
			var m adk.Message
			if statusUpdateEvent.Status.Message != nil {
				m = toADKMessage(statusUpdateEvent.Status.Message)
			} else {
				m = schema.AssistantMessage(string(statusUpdateEvent.Status.State), nil)
			}

			if statusUpdateEvent.Status.State == models.TaskStateInputRequired {
				// handle interrupted
				sender.Send(&adk.AgentEvent{
					Action: &adk.AgentAction{
						Interrupted: &adk.InterruptInfo{Data: &InterruptInfo{
							TaskID:           statusUpdateEvent.TaskID,
							InterruptMessage: m,
						}},
					},
				})
			} else {
				// handler common status update
				sender.Send(&adk.AgentEvent{
					Output: &adk.AgentOutput{
						MessageOutput: &adk.MessageVariant{
							Message: m,
						},
					},
				})
			}

			if statusUpdateEvent.Final {
				return
			}
		} else if event.TaskArtifactUpdateEvent != nil {
			m := artifact2ADKMessage(&event.TaskArtifactUpdateEvent.Artifact)
			handleNewMessage(event.TaskArtifactUpdateEvent.Artifact.ArtifactID, artifactMap, m, event.TaskArtifactUpdateEvent.LastChunk, sender)
		}
	}
}

func handleNewMessage(id string, idMap map[string]*schema.StreamWriter[*schema.Message], msg *schema.Message, final bool, sender *AgentEventSender) {
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
					MessageOutput: &adk.MessageVariant{Message: msg},
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
			}},
		})
	}
	closed := sw.Send(msg, nil) // todo: blocking?
	if closed || final {
		sw.Close()
		delete(idMap, id)
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

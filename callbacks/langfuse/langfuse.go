/*
 * Copyright 2024 CloudWeGo Authors
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

package langfuse

import (
	"context"
	"io"
	"log"
	"time"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino-ext/libs/acl/langfuse"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func NewLangfuseHandler(cli langfuse.Langfuse, opts ...Option) callbacks.Handler {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}
	return &langfuseHandler{
		cli: cli,

		name:      o.name,
		userID:    o.userID,
		sessionID: o.sessionID,
		release:   o.release,
		tags:      o.tags,
		public:    o.public,
	}
}

type langfuseHandler struct {
	cli langfuse.Langfuse

	name      string
	userID    string
	sessionID string
	release   string
	tags      []string
	public    bool
}

type langfuseStateKey struct{}
type langfuseState struct {
	traceID       string
	observationID string
}

func (l *langfuseHandler) OnStart(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
	if info == nil {
		return ctx
	}

	ctx, state := l.getOrInitState(ctx, getName(info))
	if state == nil {
		return ctx
	}
	if info.Component == components.ComponentOfChatModel {
		mcbi := model.ConvCallbackInput(input)
		generationID, err := l.cli.CreateGeneration(&langfuse.GenerationEventBody{
			BaseObservationEventBody: langfuse.BaseObservationEventBody{
				BaseEventBody: langfuse.BaseEventBody{
					Name:     getName(info),
					MetaData: mcbi.Extra,
				},
				TraceID:             state.traceID,
				ParentObservationID: state.observationID,
				StartTime:           time.Now(),
			},
			InMessages:      mcbi.Messages,
			Model:           mcbi.Config.Model,
			ModelParameters: mcbi.Config,
		})
		if err != nil {
			log.Printf("create generation error: %v, runinfo: %+v", err, info)
			return ctx
		}
		return context.WithValue(ctx, langfuseStateKey{}, &langfuseState{
			traceID:       state.traceID,
			observationID: generationID,
		})
	}

	in, err := sonic.MarshalString(input)
	if err != nil {
		log.Printf("marshal input error: %v, runinfo: %+v", err, info)
		return ctx
	}
	spanID, err := l.cli.CreateSpan(&langfuse.SpanEventBody{
		BaseObservationEventBody: langfuse.BaseObservationEventBody{
			BaseEventBody: langfuse.BaseEventBody{
				Name: getName(info),
			},
			Input:               in,
			TraceID:             state.traceID,
			ParentObservationID: state.observationID,
			StartTime:           time.Now(),
		},
	})
	if err != nil {
		log.Printf("create span error: %v", err)
		return ctx
	}
	return context.WithValue(ctx, langfuseStateKey{}, &langfuseState{
		traceID:       state.traceID,
		observationID: spanID,
	})
}

func (l *langfuseHandler) OnEnd(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
	if info == nil {
		return ctx
	}

	state, ok := ctx.Value(langfuseStateKey{}).(*langfuseState)
	if !ok {
		log.Printf("no state in context, runinfo: %+v", info)
		return ctx
	}

	if info.Component == components.ComponentOfChatModel {
		mcbo := model.ConvCallbackOutput(output)
		body := &langfuse.GenerationEventBody{
			BaseObservationEventBody: langfuse.BaseObservationEventBody{
				BaseEventBody: langfuse.BaseEventBody{
					ID: state.observationID,
				},
			},
			OutMessage:          mcbo.Message,
			EndTime:             time.Now(),
			CompletionStartTime: time.Now(),
		}
		if mcbo.TokenUsage != nil {
			body.Usage = &langfuse.Usage{
				PromptTokens:     mcbo.TokenUsage.PromptTokens,
				CompletionTokens: mcbo.TokenUsage.CompletionTokens,
				TotalTokens:      mcbo.TokenUsage.TotalTokens,
			}
		}

		err := l.cli.EndGeneration(body)
		if err != nil {
			log.Printf("end generation error: %v, runinfo: %+v", err, info)
		}
		return ctx
	}

	out, err := sonic.MarshalString(output)
	if err != nil {
		log.Printf("marshal output error: %v, runinfo: %+v", err, info)
		return ctx
	}
	err = l.cli.EndSpan(&langfuse.SpanEventBody{
		BaseObservationEventBody: langfuse.BaseObservationEventBody{
			BaseEventBody: langfuse.BaseEventBody{
				ID: state.observationID,
			},
			Output: out,
		},
		EndTime: time.Now(),
	})
	if err != nil {
		log.Printf("end span fail: %v, runinfo: %+v", err, info)
	}
	return ctx
}

func (l *langfuseHandler) OnError(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
	if info == nil {
		return ctx
	}

	state, ok := ctx.Value(langfuseStateKey{}).(*langfuseState)
	if !ok {
		log.Printf("no state in context, runinfo: %+v, execute error: %v", info, err)
		return ctx
	}

	if info.Component == components.ComponentOfChatModel {
		body := &langfuse.GenerationEventBody{
			BaseObservationEventBody: langfuse.BaseObservationEventBody{
				BaseEventBody: langfuse.BaseEventBody{
					ID: state.observationID,
				},
				Level: langfuse.LevelTypeERROR,
			},
			OutMessage:          &schema.Message{Role: schema.Assistant, Content: err.Error()},
			EndTime:             time.Now(),
			CompletionStartTime: time.Now(),
		}

		reportErr := l.cli.EndGeneration(body)
		if reportErr != nil {
			log.Printf("end generation fail: %v, runinfo: %+v, execute error: %v", reportErr, info, err)
		}
		return ctx
	}

	reportErr := l.cli.EndSpan(&langfuse.SpanEventBody{
		BaseObservationEventBody: langfuse.BaseObservationEventBody{
			BaseEventBody: langfuse.BaseEventBody{
				ID: state.observationID,
			},
			Output: err.Error(),
		},
		EndTime: time.Now(),
	})
	if reportErr != nil {
		log.Printf("end span fail: %v, runinfo: %+v, execute error: %v", reportErr, info, err)
	}
	return ctx
}

func (l *langfuseHandler) OnStartWithStreamInput(ctx context.Context, info *callbacks.RunInfo, input *schema.StreamReader[callbacks.CallbackInput]) context.Context {
	defer input.Close()
	if info == nil {
		return ctx
	}

	ctx, state := l.getOrInitState(ctx, getName(info))
	if state == nil {
		return ctx
	}
	var ins []callbacks.CallbackInput
	for {
		chunk, err := input.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("read stream input error: %v, runinfo: %+v", err, info)
			return ctx
		}
		ins = append(ins, chunk)
	}
	if info.Component == components.ComponentOfChatModel {
		modelConf, inMessage, extra, err := extractModelInput(convModelCallbackInput(ins))
		if err != nil {
			log.Printf("extract stream model input error: %v, runinfo: %+v", err, info)
			return ctx
		}
		generationID, err := l.cli.CreateGeneration(&langfuse.GenerationEventBody{
			BaseObservationEventBody: langfuse.BaseObservationEventBody{
				BaseEventBody: langfuse.BaseEventBody{
					Name:     getName(info),
					MetaData: extra,
				},
				TraceID:             state.traceID,
				ParentObservationID: state.observationID,
				StartTime:           time.Now(),
			},
			InMessages:      inMessage,
			Model:           modelConf.Model,
			ModelParameters: modelConf,
		})
		if err != nil {
			log.Printf("create generation error: %v, runinfo: %+v", err, info)
			return ctx
		}
		return context.WithValue(ctx, langfuseStateKey{}, &langfuseState{
			traceID:       state.traceID,
			observationID: generationID,
		})
	}

	in, err := sonic.MarshalString(ins)
	if err != nil {
		log.Printf("marshal input error: %v, runinfo: %+v", err, info)
		return ctx
	}
	spanID, err := l.cli.CreateSpan(&langfuse.SpanEventBody{
		BaseObservationEventBody: langfuse.BaseObservationEventBody{
			BaseEventBody: langfuse.BaseEventBody{
				Name: getName(info),
			},
			Input:               in,
			TraceID:             state.traceID,
			ParentObservationID: state.observationID,
			StartTime:           time.Now(),
		},
	})
	if err != nil {
		log.Printf("create span error: %v", err)
		return ctx
	}
	return context.WithValue(ctx, langfuseStateKey{}, &langfuseState{
		traceID:       state.traceID,
		observationID: spanID,
	})
}

func (l *langfuseHandler) OnEndWithStreamOutput(ctx context.Context, info *callbacks.RunInfo, output *schema.StreamReader[callbacks.CallbackOutput]) context.Context {
	if info == nil {
		return ctx
	}

	state, ok := ctx.Value(langfuseStateKey{}).(*langfuseState)
	if !ok {
		log.Printf("no state in context, runinfo: %+v", info)
		return ctx
	}

	startTime := time.Now()
	var outs []callbacks.CallbackOutput
	for {
		chunk, err := output.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("read stream output error: %v, runinfo: %+v", err, info)
			return ctx
		}
		outs = append(outs, chunk)
	}

	if info.Component == components.ComponentOfChatModel {
		usage, outMessage, extra, err := extractModelOutput(convModelCallbackOutput(outs))
		body := &langfuse.GenerationEventBody{
			BaseObservationEventBody: langfuse.BaseObservationEventBody{
				BaseEventBody: langfuse.BaseEventBody{
					ID:       state.observationID,
					MetaData: extra,
				},
			},
			OutMessage:          outMessage,
			EndTime:             time.Now(),
			CompletionStartTime: startTime,
		}
		if usage != nil {
			body.Usage = &langfuse.Usage{
				PromptTokens:     usage.PromptTokens,
				CompletionTokens: usage.CompletionTokens,
				TotalTokens:      usage.TotalTokens,
			}
		}

		err = l.cli.EndGeneration(body)
		if err != nil {
			log.Printf("end stream generation error: %v, runinfo: %+v", err, info)
		}
		return ctx
	}

	out, err := sonic.MarshalString(outs)
	if err != nil {
		log.Printf("marshal stream output error: %v, runinfo: %+v", err, info)
		return ctx
	}
	err = l.cli.EndSpan(&langfuse.SpanEventBody{
		BaseObservationEventBody: langfuse.BaseObservationEventBody{
			BaseEventBody: langfuse.BaseEventBody{
				ID: state.observationID,
			},
			Output: out,
		},
		EndTime: time.Now(),
	})
	if err != nil {
		log.Printf("end stream span fail: %v, runinfo: %+v", err, info)
	}
	return ctx
}

func (l *langfuseHandler) getOrInitState(ctx context.Context, curName string) (context.Context, *langfuseState) {
	state := ctx.Value(langfuseStateKey{})
	if state == nil {
		name := l.name
		if len(name) == 0 {
			name = curName
		}
		traceID, err := l.cli.CreateTrace(&langfuse.TraceEventBody{
			BaseEventBody: langfuse.BaseEventBody{
				Name: name,
			},
			TimeStamp: time.Now(),
			UserID:    l.userID,
			SessionID: l.sessionID,
			Release:   l.release,
			Tags:      l.tags,
			Public:    l.public,
		})
		if err != nil {
			log.Printf("create trace error: %v", err)
			return ctx, nil
		}
		s := &langfuseState{
			traceID: traceID,
		}
		ctx = context.WithValue(ctx, langfuseStateKey{}, s)
		return ctx, s
	}
	return ctx, state.(*langfuseState)
}

func getName(info *callbacks.RunInfo) string {
	if len(info.Name) != 0 {
		return info.Name
	}
	return info.Type + string(info.Component)
}

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
	"runtime/debug"
	"time"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino-ext/libs/acl/langfuse"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type Config struct {
	// Host is the Langfuse server URL (Required)
	// Example: "https://cloud.langfuse.com"
	Host string

	// PublicKey is the public key for authentication (Required)
	// Example: "pk-lf-..."
	PublicKey string

	// SecretKey is the secret key for authentication (Required)
	// Example: "sk-lf-..."
	SecretKey string

	// Threads is the number of concurrent workers for processing events (Optional)
	// Default: 1
	// Example: 5
	Threads int

	// Timeout is the HTTP request timeout (Optional)
	// Default: no timeout
	// Example: 30 * time.Second
	Timeout time.Duration

	// MaxTaskQueueSize is the maximum number of events to buffer (Optional)
	// Default: 100
	// Example: 1000
	MaxTaskQueueSize int

	// FlushAt is the number of events to batch before sending (Optional)
	// Default: 15
	// Example: 50
	FlushAt int

	// FlushInterval is how often to flush events automatically (Optional)
	// Default: 500 * time.MilliSecond
	// Example: 10 * time.Second
	FlushInterval time.Duration

	// SampleRate is the percentage of events to send (Optional)
	// Default: 1.0 (100%)
	// Example: 0.5 (50%)
	SampleRate float64

	// LogMessage is the message to log when events exceed the limit length(1 000 000)  (Optional)
	// Default: ""
	// Example: "langfuse event:"
	LogMessage string

	// MaskFunc is a function to mask sensitive data before sending (Optional)
	// Default: nil
	// Example: func(s string) string { return strings.ReplaceAll(s, "secret", "***") }
	MaskFunc func(string) string

	// MaxRetry is the maximum number of retry attempts for failed requests (Optional)
	// Default: 3
	// Example: 5
	MaxRetry uint64

	// Name is the trace name (Optional)
	// Default: ""
	// Example: "my-app-trace"
	Name string

	// UserID is the user identifier for the trace (Optional)
	// Default: ""
	// Example: "user-123"
	UserID string

	// SessionID is the session identifier for the trace (Optional)
	// Default: ""
	// Example: "session-456"
	SessionID string

	// Release is the version or release identifier (Optional)
	// Default: ""
	// Example: "v1.2.3"
	Release string

	// Tags are labels attached to the trace (Optional)
	// Default: nil
	// Example: []string{"production", "feature-x"}
	Tags []string

	// Public determines if the trace is publicly accessible (Optional)
	// Default: false
	// Example: true
	Public bool
}

func NewLangfuseHandler(cfg *Config) (handler *CallbackHandler, flusher func()) {
	var langfuseOpts []langfuse.Option
	if cfg.Threads > 0 {
		langfuseOpts = append(langfuseOpts, langfuse.WithThreads(cfg.Threads))
	}
	if cfg.Timeout > 0 {
		langfuseOpts = append(langfuseOpts, langfuse.WithTimeout(cfg.Timeout))
	}
	if cfg.MaxTaskQueueSize > 0 {
		langfuseOpts = append(langfuseOpts, langfuse.WithMaxTaskQueueSize(cfg.MaxTaskQueueSize))
	}
	if cfg.FlushAt > 0 {
		langfuseOpts = append(langfuseOpts, langfuse.WithFlushAt(cfg.FlushAt))
	}
	if cfg.FlushInterval > 0 {
		langfuseOpts = append(langfuseOpts, langfuse.WithFlushInterval(cfg.FlushInterval))
	}
	if cfg.SampleRate > 0 {
		langfuseOpts = append(langfuseOpts, langfuse.WithSampleRate(cfg.SampleRate))
	}
	if len(cfg.LogMessage) > 0 {
		langfuseOpts = append(langfuseOpts, langfuse.WithLogMessage(cfg.LogMessage))
	}
	if cfg.MaskFunc != nil {
		langfuseOpts = append(langfuseOpts, langfuse.WithMaskFunc(cfg.MaskFunc))
	}
	if cfg.MaxRetry > 0 {
		langfuseOpts = append(langfuseOpts, langfuse.WithMaxRetry(cfg.MaxRetry))
	}

	cli := langfuse.NewLangfuse(
		cfg.Host,
		cfg.PublicKey,
		cfg.SecretKey,
		langfuseOpts...,
	)

	return &CallbackHandler{
		cli: cli,

		name:      cfg.Name,
		userID:    cfg.UserID,
		sessionID: cfg.SessionID,
		release:   cfg.Release,
		tags:      cfg.Tags,
		public:    cfg.Public,
	}, cli.Flush
}

type CallbackHandler struct {
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

func (c *CallbackHandler) OnStart(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
	if info == nil {
		return ctx
	}

	ctx, state := c.getOrInitState(ctx, getName(info))
	if state == nil {
		return ctx
	}
	if info.Component == components.ComponentOfChatModel {
		mcbi := model.ConvCallbackInput(input)

		body := &langfuse.GenerationEventBody{
			BaseObservationEventBody: langfuse.BaseObservationEventBody{
				BaseEventBody: langfuse.BaseEventBody{
					Name:     getName(info),
					MetaData: mcbi.Extra,
				},
				TraceID:             state.traceID,
				ParentObservationID: state.observationID,
				StartTime:           time.Now(),
			},
			InMessages: mcbi.Messages,
		}
		if mcbi.Config != nil {
			body.Model = mcbi.Config.Model
			body.ModelParameters = mcbi.Config
		}
		generationID, err := c.cli.CreateGeneration(body)
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
	spanID, err := c.cli.CreateSpan(&langfuse.SpanEventBody{
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

func (c *CallbackHandler) OnEnd(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
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

		err := c.cli.EndGeneration(body)
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
	err = c.cli.EndSpan(&langfuse.SpanEventBody{
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

func (c *CallbackHandler) OnError(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
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

		reportErr := c.cli.EndGeneration(body)
		if reportErr != nil {
			log.Printf("end generation fail: %v, runinfo: %+v, execute error: %v", reportErr, info, err)
		}
		return ctx
	}

	reportErr := c.cli.EndSpan(&langfuse.SpanEventBody{
		BaseObservationEventBody: langfuse.BaseObservationEventBody{
			BaseEventBody: langfuse.BaseEventBody{
				ID: state.observationID,
			},
			Output: err.Error(),
			Level:  langfuse.LevelTypeERROR,
		},
		EndTime: time.Now(),
	})
	if reportErr != nil {
		log.Printf("end span fail: %v, runinfo: %+v, execute error: %v", reportErr, info, err)
	}
	return ctx
}

func (c *CallbackHandler) OnStartWithStreamInput(ctx context.Context, info *callbacks.RunInfo, input *schema.StreamReader[callbacks.CallbackInput]) context.Context {
	if info == nil {
		return ctx
	}

	ctx, state := c.getOrInitState(ctx, getName(info))
	if state == nil {
		return ctx
	}

	if info.Component == components.ComponentOfChatModel {
		generationID, err := c.cli.CreateGeneration(&langfuse.GenerationEventBody{
			BaseObservationEventBody: langfuse.BaseObservationEventBody{
				BaseEventBody: langfuse.BaseEventBody{
					Name: getName(info),
				},
				TraceID:             state.traceID,
				ParentObservationID: state.observationID,
				StartTime:           time.Now(),
			},
		})
		if err != nil {
			log.Printf("create generation error: %v, runinfo: %+v", err, info)
			return ctx
		}

		go func() {
			defer func() {
				e := recover()
				if e != nil {
					log.Printf("recover update langfuse generation panic: %v, runinfo: %+v, stack: %s", e, info, string(debug.Stack()))
				}
				input.Close()
			}()
			var ins []callbacks.CallbackInput
			for {
				chunk, err_ := input.Recv()
				if err_ == io.EOF {
					break
				}
				if err_ != nil {
					log.Printf("read stream input error: %v, runinfo: %+v", err_, info)
					return
				}
				ins = append(ins, chunk)
			}

			modelConf, inMessage, extra, err_ := extractModelInput(convModelCallbackInput(ins))
			if err_ != nil {
				log.Printf("extract stream model input error: %v, runinfo: %+v", err_, info)
				return
			}
			body := &langfuse.GenerationEventBody{
				BaseObservationEventBody: langfuse.BaseObservationEventBody{
					BaseEventBody: langfuse.BaseEventBody{
						ID:       generationID,
						MetaData: extra,
					},
				},
				InMessages: inMessage,
			}
			body.Model = modelConf.Model
			body.ModelParameters = modelConf
			err = c.cli.EndGeneration(body)
			if err != nil {
				log.Printf("update stream generation fail: %v, runinfo: %+v", err, info)
			}
		}()

		return context.WithValue(ctx, langfuseStateKey{}, &langfuseState{
			traceID:       state.traceID,
			observationID: generationID,
		})
	}

	spanID, err := c.cli.CreateSpan(&langfuse.SpanEventBody{
		BaseObservationEventBody: langfuse.BaseObservationEventBody{
			BaseEventBody: langfuse.BaseEventBody{
				Name: getName(info),
			},
			TraceID:             state.traceID,
			ParentObservationID: state.observationID,
			StartTime:           time.Now(),
		},
	})
	if err != nil {
		log.Printf("create span error: %v", err)
		return ctx
	}

	go func() {
		defer func() {
			e := recover()
			if e != nil {
				log.Printf("recover update langfuse span panic: %v, runinfo: %+v, stack: %s", e, info, string(debug.Stack()))
			}
			input.Close()
		}()
		var ins []callbacks.CallbackInput
		for {
			chunk, err_ := input.Recv()
			if err_ == io.EOF {
				break
			}
			if err_ != nil {
				log.Printf("read stream input error: %v, runinfo: %+v", err_, info)
				return
			}
			ins = append(ins, chunk)
		}

		in, err_ := sonic.MarshalString(ins)
		if err_ != nil {
			log.Printf("marshal input error: %v, runinfo: %+v", err_, info)
			return
		}
		err = c.cli.EndSpan(&langfuse.SpanEventBody{
			BaseObservationEventBody: langfuse.BaseObservationEventBody{
				BaseEventBody: langfuse.BaseEventBody{
					ID: spanID,
				},
				Input: in,
			},
		})
		if err != nil {
			log.Printf("update stream span error: %v", err)
		}
	}()

	return context.WithValue(ctx, langfuseStateKey{}, &langfuseState{
		traceID:       state.traceID,
		observationID: spanID,
	})
}

func (c *CallbackHandler) OnEndWithStreamOutput(ctx context.Context, info *callbacks.RunInfo, output *schema.StreamReader[callbacks.CallbackOutput]) context.Context {
	if info == nil {
		return ctx
	}

	state, ok := ctx.Value(langfuseStateKey{}).(*langfuseState)
	if !ok {
		log.Printf("no state in context, runinfo: %+v", info)
		return ctx
	}

	if info.Component == components.ComponentOfChatModel {
		go func() {
			defer func() {
				e := recover()
				if e != nil {
					log.Printf("recover update langfuse span panic: %v, runinfo: %+v, stack: %s", e, info, string(debug.Stack()))
				}
				output.Close()
			}()
			startTime := time.Now()
			var outs []callbacks.CallbackOutput
			for {
				chunk, err := output.Recv()
				if err == io.EOF {
					break
				}
				if err != nil {
					log.Printf("read stream output error: %v, runinfo: %+v", err, info)
				}
				outs = append(outs, chunk)
			}

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

			err = c.cli.EndGeneration(body)
			if err != nil {
				log.Printf("end stream generation error: %v, runinfo: %+v", err, info)
			}
		}()
		return ctx
	}

	go func() {
		defer func() {
			e := recover()
			if e != nil {
				log.Printf("recover update langfuse span panic: %v, runinfo: %+v, stack: %s", e, info, string(debug.Stack()))
			}
			output.Close()
		}()
		var outs []callbacks.CallbackOutput
		for {
			chunk, err := output.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Printf("read stream output error: %v, runinfo: %+v", err, info)
			}
			outs = append(outs, chunk)
		}

		out, err := sonic.MarshalString(outs)
		if err != nil {
			log.Printf("marshal stream output error: %v, runinfo: %+v", err, info)
		}
		err = c.cli.EndSpan(&langfuse.SpanEventBody{
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
	}()

	return ctx
}

func (c *CallbackHandler) getOrInitState(ctx context.Context, curName string) (context.Context, *langfuseState) {
	state := ctx.Value(langfuseStateKey{})
	if state != nil {
		return ctx, state.(*langfuseState)
	}

	traceOpts := ctx.Value(langfuseTraceOptionKey{})
	if traceOpts != nil {
		nState, err := initState(ctx, c.cli, traceOpts.(*traceOptions))
		if err != nil {
			log.Printf("init state fail: %v", err)
			return ctx, nil
		}
		return context.WithValue(ctx, langfuseStateKey{}, nState), nState
	}

	name := c.name
	if len(name) == 0 {
		name = curName
	}
	nState, err := initState(ctx, c.cli, &traceOptions{
		Name:      c.name,
		UserID:    c.userID,
		SessionID: c.sessionID,
		Release:   c.release,
		Tags:      c.tags,
		Public:    c.public,
	})
	if err != nil {
		log.Printf("init state fail: %v", err)
		return ctx, nil
	}
	return context.WithValue(ctx, langfuseStateKey{}, nState), nState
}

func getName(info *callbacks.RunInfo) string {
	if len(info.Name) != 0 {
		return info.Name
	}
	return info.Type + string(info.Component)
}

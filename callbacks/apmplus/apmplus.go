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

package apmplus

import (
	"context"
	"fmt"
	"io"
	"log"
	"runtime/debug"
	"time"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino-ext/libs/acl/opentelemetry"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const scopeName = "github.com/cloudwego/eino-ext/callbacks/apmplus"

type Config struct {
	// Host is the Apmplus URL (Required)
	// Example: "https://apmplus-cn-beijing.volces.com:4317"
	Host string

	// AppKey is the key for authentication (Required)
	// Example: "abc..."
	AppKey string

	// ServiceName is the name of service (Required)
	// Example: "my-app"
	ServiceName string

	// Release is the version or release identifier (Optional)
	// Default: ""
	// Example: "v1.2.3"
	Release string
}

func NewApmplusHandler(cfg *Config) (handler callbacks.Handler, shutdown func(ctx context.Context) error) {
	p := opentelemetry.NewOpenTelemetryProvider(
		opentelemetry.WithServiceName(cfg.ServiceName),
		opentelemetry.WithExportEndpoint(cfg.Host),
		opentelemetry.WithInsecure(),
		opentelemetry.WithHeaders(map[string]string{"X-ByteAPM-AppKey": cfg.AppKey}),
	)

	return &apmplusHandler{
		otelProvider: p,
		serviceName:  cfg.ServiceName,
		release:      cfg.Release,
		tracer:       otel.Tracer(scopeName),
	}, p.Shutdown
}

type apmplusHandler struct {
	otelProvider opentelemetry.OtelProvider
	serviceName  string
	release      string
	tracer       trace.Tracer
}

type apmplusStateKey struct{}
type apmplusState struct {
	span trace.Span
}

func (a *apmplusHandler) OnStart(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
	if info == nil {
		return ctx
	}

	spanName := getName(info)
	if len(spanName) == 0 {
		spanName = "unset"
	}
	ctx, span := a.tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient), trace.WithTimestamp(time.Now()))

	contentReady := false
	config, inMessage, _, err := extractModelInput(convModelCallbackInput([]callbacks.CallbackInput{input}))
	if err != nil {
		log.Printf("extract stream model input error: %v, runinfo: %+v", err, info)
	} else {
		for i, in := range inMessage {
			if in != nil && len(in.Content) > 0 {
				contentReady = true
				span.SetAttributes(attribute.String(fmt.Sprintf("llm.prompts.%d.role", i), string(in.Role)))
				span.SetAttributes(attribute.String(fmt.Sprintf("llm.prompts.%d.content", i), in.Content))
			}
		}

		if config != nil {
			span.SetAttributes(attribute.String("llm.request.model", config.Model))
			span.SetAttributes(attribute.Int("llm.request.max_token", config.MaxTokens))
			span.SetAttributes(attribute.Float64("llm.request.temperature", float64(config.Temperature)))
			span.SetAttributes(attribute.Float64("llm.request.top_p", float64(config.TopP)))
			span.SetAttributes(attribute.StringSlice("llm.request.stop", config.Stop))
		}
	}

	if !contentReady {
		in, err := sonic.MarshalString(input)
		if err == nil {
			span.SetAttributes(attribute.String("llm.prompts.0.role", string(schema.User)))
			span.SetAttributes(attribute.String("llm.prompts.0.content", in))
		}
	}

	span.SetAttributes(attribute.String("runinfo.name", info.Name))
	span.SetAttributes(attribute.String("runinfo.type", info.Type))
	span.SetAttributes(attribute.String("runinfo.component", string(info.Component)))

	return context.WithValue(ctx, apmplusStateKey{}, &apmplusState{
		span: span,
	})
}

func (a *apmplusHandler) OnEnd(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
	if info == nil {
		return ctx
	}

	state, ok := ctx.Value(apmplusStateKey{}).(*apmplusState)
	if !ok {
		log.Printf("no state in context, runinfo: %+v", info)
		return ctx
	}
	span := state.span
	defer span.End(trace.WithTimestamp(time.Now()))

	contentReady := false
	switch info.Component {
	case components.ComponentOfEmbedding:
		if ecbo := embedding.ConvCallbackOutput(output); ecbo != nil {
			if ecbo.Config != nil {
				span.SetAttributes(attribute.String("llm.response.model", ecbo.Config.Model))
			}
		}
	case components.ComponentOfChatModel:
		fallthrough
	default:
		usage, outMessages, _, config, err := extractModelOutput(convModelCallbackOutput([]callbacks.CallbackOutput{output}))
		if err == nil {
			for i, out := range outMessages {
				if out != nil && len(out.Content) > 0 {
					contentReady = true
					span.SetAttributes(attribute.String(fmt.Sprintf("llm.completions.%d.role", i), string(out.Role)))
					span.SetAttributes(attribute.String(fmt.Sprintf("llm.completions.%d.content", i), out.Content))
					if out.ResponseMeta != nil {
						span.SetAttributes(attribute.String(fmt.Sprintf("llm.completions.%d.finish_reason", i), out.ResponseMeta.FinishReason))
					}
				}
			}
			if !contentReady && outMessages != nil {
				outMessage, err := sonic.MarshalString(outMessages)
				if err == nil {
					contentReady = true
					span.SetAttributes(attribute.String("llm.completions.0.content", outMessage))
				}
			}

			if config != nil {
				span.SetAttributes(attribute.String("llm.response.model", config.Model))
			}

			if usage != nil {
				span.SetAttributes(attribute.Int("llm.usage.total_tokens", usage.TotalTokens))
				span.SetAttributes(attribute.Int("llm.usage.prompt_tokens", usage.PromptTokens))
				span.SetAttributes(attribute.Int("llm.usage.completion_tokens", usage.CompletionTokens))
			}
		}
	}

	if !contentReady {
		out, err := sonic.MarshalString(output)
		if err != nil {
			log.Printf("marshal output error: %v, runinfo: %+v", err, info)
		}
		span.SetAttributes(attribute.String("llm.completions.0.content", out))
	}
	span.SetAttributes(attribute.Bool("llm.is_streaming", false))

	return ctx
}

func (a *apmplusHandler) OnError(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
	if info == nil {
		return ctx
	}

	state, ok := ctx.Value(apmplusStateKey{}).(*apmplusState)
	if !ok {
		log.Printf("no state in context, runinfo: %+v", info)
		return ctx
	}
	span := state.span
	defer span.End()

	span.SetStatus(codes.Error, err.Error())
	span.RecordError(err)
	return ctx
}

func (a *apmplusHandler) OnStartWithStreamInput(ctx context.Context, info *callbacks.RunInfo, input *schema.StreamReader[callbacks.CallbackInput]) context.Context {
	if info == nil {
		return ctx
	}

	spanName := getName(info)
	if len(spanName) == 0 {
		spanName = "unset"
	}
	ctx, span := a.tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient), trace.WithTimestamp(time.Now()))

	span.SetAttributes(attribute.String("runinfo.name", info.Name))
	span.SetAttributes(attribute.String("runinfo.type", info.Type))
	span.SetAttributes(attribute.String("runinfo.component", string(info.Component)))

	go func() {
		defer func() {
			e := recover()
			if e != nil {
				log.Printf("recover update span panic: %v, runinfo: %+v, stack: %s", e, info, string(debug.Stack()))
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
		contentReady := false
		config, inMessage, _, err := extractModelInput(convModelCallbackInput(ins))
		if err != nil {
			log.Printf("extract stream model input error: %v, runinfo: %+v", err, info)
		} else {
			for i, in := range inMessage {
				if in != nil && len(in.Content) > 0 {
					contentReady = true
					span.SetAttributes(attribute.String(fmt.Sprintf("llm.prompts.%d.role", i), string(in.Role)))
					span.SetAttributes(attribute.String(fmt.Sprintf("llm.prompts.%d.content", i), in.Content))
				}
			}

			if config != nil {
				span.SetAttributes(attribute.String("llm.request.model", config.Model))
				span.SetAttributes(attribute.Int("llm.request.max_token", config.MaxTokens))
				span.SetAttributes(attribute.Float64("llm.request.temperature", float64(config.Temperature)))
				span.SetAttributes(attribute.Float64("llm.request.top_p", float64(config.TopP)))
				span.SetAttributes(attribute.StringSlice("llm.request.stop", config.Stop))
			}
		}
		if !contentReady {
			in, err := sonic.MarshalString(ins)
			if err != nil {
				log.Printf("marshal input error: %v, runinfo: %+v", err, info)
			}
			span.SetAttributes(attribute.String("llm.prompts.0.role", string(schema.User)))
			span.SetAttributes(attribute.String("llm.prompts.0.content", in))
		}
	}()
	return context.WithValue(ctx, apmplusStateKey{}, &apmplusState{
		span: span,
	})
}

func (a *apmplusHandler) OnEndWithStreamOutput(ctx context.Context, info *callbacks.RunInfo, output *schema.StreamReader[callbacks.CallbackOutput]) context.Context {
	if info == nil {
		return ctx
	}

	state, ok := ctx.Value(apmplusStateKey{}).(*apmplusState)
	if !ok {
		log.Printf("no state in context, runinfo: %+v", info)
		return ctx
	}
	span := state.span

	go func() {
		defer func() {
			e := recover()
			if e != nil {
				log.Printf("recover update span panic: %v, runinfo: %+v, stack: %s", e, info, string(debug.Stack()))
			}
			output.Close()
			span.End(trace.WithTimestamp(time.Now()))
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
		contentReady := false
		// both work for ChatModel or not
		usage, outMessages, _, config, err := extractModelOutput(convModelCallbackOutput(outs))
		if err == nil {
			for i, out := range outMessages {
				if out != nil && len(out.Content) > 0 {
					contentReady = true
					span.SetAttributes(attribute.String(fmt.Sprintf("llm.completions.%d.role", i), string(out.Role)))
					span.SetAttributes(attribute.String(fmt.Sprintf("llm.completions.%d.content", i), out.Content))
					if out.ResponseMeta != nil {
						span.SetAttributes(attribute.String(fmt.Sprintf("llm.completions.%d.finish_reason", i), out.ResponseMeta.FinishReason))
					}
				}
			}
			if !contentReady && outMessages != nil {
				outMessage, err := sonic.MarshalString(outMessages)
				if err == nil {
					contentReady = true
					span.SetAttributes(attribute.String("llm.completions.0.role", string(schema.Assistant)))
					span.SetAttributes(attribute.String("llm.completions.0.content", outMessage))
				}
			}

			if config != nil {
				span.SetAttributes(attribute.String("llm.response.model", config.Model))
			}

			if usage != nil {
				span.SetAttributes(attribute.Int("llm.usage.total_tokens", usage.TotalTokens))
				span.SetAttributes(attribute.Int("llm.usage.prompt_tokens", usage.PromptTokens))
				span.SetAttributes(attribute.Int("llm.usage.completion_tokens", usage.CompletionTokens))
			}
		}
		if !contentReady {
			out, err := sonic.MarshalString(outs)
			if err != nil {
				log.Printf("marshal stream output error: %v, runinfo: %+v", err, info)
			}
			span.SetAttributes(attribute.String("llm.completions.0.content", out))
		}
		span.SetAttributes(attribute.Bool("llm.is_streaming", true))
	}()

	return ctx
}

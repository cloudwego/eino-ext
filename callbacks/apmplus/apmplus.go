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
	"errors"
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
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	runtimemetrics "go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
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

func NewApmplusHandler(cfg *Config) (handler callbacks.Handler, shutdown func(ctx context.Context) error, err error) {
	p, err := opentelemetry.NewOpenTelemetryProvider(
		opentelemetry.WithServiceName(cfg.ServiceName),
		opentelemetry.WithExportEndpoint(cfg.Host),
		opentelemetry.WithInsecure(),
		opentelemetry.WithHeaders(map[string]string{"X-ByteAPM-AppKey": cfg.AppKey}),
		opentelemetry.WithResourceAttribute(attribute.String("apmplus.business_type", "llm")),
	)
	if p == nil || err != nil {
		return nil, nil, errors.New("init opentelemetry provider failed")
	}

	if p.TracerProvider == nil || p.MeterProvider == nil {
		return nil, p.Shutdown, errors.New("tracer provider or meter provider is nil")
	}

	err = runtimemetrics.Start(runtimemetrics.WithMeterProvider(p.MeterProvider))
	if err != nil {
		return nil, p.Shutdown, err
	}

	meter := p.MeterProvider.Meter(scopeName)

	tokenUsage, err := meter.Int64Counter(
		"llm.token.usage",
		metric.WithDescription("Number of tokens used in prompt and completions"),
		metric.WithUnit("token"),
	)
	if err != nil {
		return nil, p.Shutdown, err
	}

	chatCount, err := meter.Int64Counter(
		"llm.chat.count",
		metric.WithDescription("Number of chat"),
		metric.WithUnit("time"),
	)
	if err != nil {
		return nil, p.Shutdown, err
	}

	chatChoiceCounter, err := meter.Int64Counter(
		"llm.chat_completions.choices",
		metric.WithDescription("Number of choices returned by chat completions call"),
		metric.WithUnit("choice"),
	)
	if err != nil {
		return nil, p.Shutdown, err
	}

	chatDurationHistogram, err := meter.Float64Histogram(
		"llm.chat_completions.duration",
		metric.WithDescription("Duration of chat completion operation"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, p.Shutdown, err
	}

	chatExceptionCounter, err := meter.Int64Counter(
		"llm.chat_completions.exceptions",
		metric.WithDescription("Number of exceptions occurred during chat completions"),
		metric.WithUnit("time"),
	)
	if err != nil {
		return nil, p.Shutdown, err
	}

	streamingTimeToFirstToken, err := meter.Float64Histogram(
		"llm.chat_completions.streaming_time_to_first_token",
		metric.WithDescription("Time to first token in streaming chat completions"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, p.Shutdown, err
	}

	streamingTimeToGenerate, err := meter.Float64Histogram(
		"llm.chat_completions.streaming_time_to_generate",
		metric.WithDescription("Time between first token and completion in streaming chat completions"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, p.Shutdown, err
	}

	streamingTimePerOutputToken, err := meter.Float64Histogram(
		"llm.chat_completions.streaming_time_per_output_token",
		metric.WithDescription("Time per output token in streaming chat completions"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, p.Shutdown, err
	}

	return &apmplusHandler{
		otelProvider: p,
		serviceName:  cfg.ServiceName,
		release:      cfg.Release,
		tracer:       otel.Tracer(scopeName),
		meter:        meter,

		tokenUsage:                  tokenUsage,
		chatCount:                   chatCount,
		chatChoiceCounter:           chatChoiceCounter,
		chatDurationHistogram:       chatDurationHistogram,
		chatExceptionCounter:        chatExceptionCounter,
		streamingTimeToFirstToken:   streamingTimeToFirstToken,
		streamingTimeToGenerate:     streamingTimeToGenerate,
		streamingTimePerOutputToken: streamingTimePerOutputToken,
	}, p.Shutdown, nil
}

type apmplusHandler struct {
	otelProvider *opentelemetry.OtelProvider
	serviceName  string
	release      string
	tracer       trace.Tracer
	meter        metric.Meter

	tokenUsage                  metric.Int64Counter
	chatCount                   metric.Int64Counter
	chatChoiceCounter           metric.Int64Counter
	chatDurationHistogram       metric.Float64Histogram
	chatExceptionCounter        metric.Int64Counter
	streamingTimeToFirstToken   metric.Float64Histogram
	streamingTimeToGenerate     metric.Float64Histogram
	streamingTimePerOutputToken metric.Float64Histogram
}

type requestInfo struct {
	model string
}

type apmplusStateKey struct{}
type apmplusState struct {
	startTime   time.Time
	span        trace.Span
	requestInfo *requestInfo
}

type traceStreamInputAsyncKey struct{}
type streamInputAsyncVal chan struct{}

func (a *apmplusHandler) OnStart(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
	if info == nil {
		return ctx
	}

	spanName := getName(info)
	if len(spanName) == 0 {
		spanName = "unset"
	}
	startTime := time.Now()
	requestModel := ""
	ctx, span := a.tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient), trace.WithTimestamp(startTime))

	contentReady := false

	//TODO: covert input from other type of component
	//ref: https://github.com/cloudwego/eino-ext/pull/103#discussion_r1967017732
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
			requestModel = config.Model
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

	if info.Component == components.ComponentOfChatModel {
		a.chatCount.Add(ctx, 1, metric.WithAttributes(
			attribute.String("llm.request.model", requestModel),
		))
	}

	return context.WithValue(ctx, apmplusStateKey{}, &apmplusState{
		startTime:   startTime,
		span:        span,
		requestInfo: &requestInfo{model: requestModel},
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
	startTime := state.startTime
	endTime := time.Now()

	defer func() {
		if stopCh, ok := ctx.Value(traceStreamInputAsyncKey{}).(streamInputAsyncVal); ok {
			<-stopCh
		}
		span.End(trace.WithTimestamp(time.Now()))
	}()

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
			responseModel := ""
			responseFinishReason := ""

			for i, out := range outMessages {
				if out != nil && len(out.Content) > 0 {
					contentReady = true
					span.SetAttributes(attribute.String(fmt.Sprintf("llm.completions.%d.role", i), string(out.Role)))
					span.SetAttributes(attribute.String(fmt.Sprintf("llm.completions.%d.content", i), out.Content))
					if out.ResponseMeta != nil {
						span.SetAttributes(attribute.String(fmt.Sprintf("llm.completions.%d.finish_reason", i), out.ResponseMeta.FinishReason))
						responseFinishReason = out.ResponseMeta.FinishReason
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
				responseModel = config.Model
			}

			if usage != nil {
				span.SetAttributes(attribute.Int("llm.usage.total_tokens", usage.TotalTokens))
				span.SetAttributes(attribute.Int("llm.usage.prompt_tokens", usage.PromptTokens))
				span.SetAttributes(attribute.Int("llm.usage.completion_tokens", usage.CompletionTokens))
			}

			if info.Component == components.ComponentOfChatModel {
				if len(responseFinishReason) > 0 {
					a.chatChoiceCounter.Add(ctx, 1, metric.WithAttributes(
						attribute.String("llm.response.model", responseModel),
						attribute.String("llm.response.finish_reason", responseFinishReason),
						attribute.Bool("stream", false),
					))
				}
				if usage != nil {
					a.AddTokenUsage(ctx, usage, responseModel, false)
				}
				a.chatDurationHistogram.Record(ctx, float64(endTime.Sub(startTime).Milliseconds()), metric.WithAttributes(
					attribute.String("llm.response.model", responseModel),
					attribute.Bool("stream", false),
				))
			}
		}
	}

	if !contentReady {
		out, err := sonic.MarshalString(output)
		if err != nil {
			log.Printf("marshal output error: %v, runinfo: %+v", err, info)
		} else {
			span.SetAttributes(attribute.String("llm.completions.0.content", out))
		}
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
	requestInfo := state.requestInfo
	defer func() {
		if stopCh, ok := ctx.Value(traceStreamInputAsyncKey{}).(streamInputAsyncVal); ok {
			<-stopCh
		}
		span.End(trace.WithTimestamp(time.Now()))
	}()

	span.SetStatus(codes.Error, err.Error())
	span.RecordError(err)

	if requestInfo != nil && len(requestInfo.model) > 0 {
		a.chatExceptionCounter.Add(ctx, 1, metric.WithAttributes(
			attribute.String("llm.request.model", requestInfo.model),
		))
	}

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
	startTime := time.Now()
	requestInfo := &requestInfo{}
	ctx, span := a.tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient), trace.WithTimestamp(startTime))

	span.SetAttributes(attribute.String("runinfo.name", info.Name))
	span.SetAttributes(attribute.String("runinfo.type", info.Type))
	span.SetAttributes(attribute.String("runinfo.component", string(info.Component)))

	stopCh := make(streamInputAsyncVal)
	ctx = context.WithValue(ctx, traceStreamInputAsyncKey{}, stopCh)

	go func() {
		defer func() {
			e := recover()
			if e != nil {
				log.Printf("recover update span panic: %v, runinfo: %+v, stack: %s", e, info, string(debug.Stack()))
			}
			input.Close()
			close(stopCh)
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
				requestInfo.model = config.Model
				if info.Component == components.ComponentOfChatModel {
					a.chatCount.Add(ctx, 1, metric.WithAttributes(
						attribute.String("llm.request.model", requestInfo.model),
					))
				}
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
			} else {
				span.SetAttributes(attribute.String("llm.prompts.0.role", string(schema.User)))
				span.SetAttributes(attribute.String("llm.prompts.0.content", in))
			}
		}
	}()
	return context.WithValue(ctx, apmplusStateKey{}, &apmplusState{
		span:        span,
		startTime:   startTime,
		requestInfo: requestInfo,
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
	startTime := state.startTime

	go func() {
		responseModel := ""
		responseFinishReason := ""

		defer func() {
			e := recover()
			if e != nil {
				log.Printf("recover update span panic: %v, runinfo: %+v, stack: %s", e, info, string(debug.Stack()))
			}
			output.Close()
			if stopCh, ok := ctx.Value(traceStreamInputAsyncKey{}).(streamInputAsyncVal); ok {
				<-stopCh
			}
			span.End(trace.WithTimestamp(time.Now()))
		}()
		var outs []callbacks.CallbackOutput
		timeOfFirstToken := time.Now()
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
		endTime := time.Now()
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
						responseFinishReason = out.ResponseMeta.FinishReason
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
				responseModel = config.Model
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
			} else {
				span.SetAttributes(attribute.String("llm.completions.0.content", out))
			}
		}
		span.SetAttributes(attribute.Bool("llm.is_streaming", true))

		if info.Component == components.ComponentOfChatModel {
			if len(responseFinishReason) > 0 {
				a.chatChoiceCounter.Add(ctx, 1, metric.WithAttributes(
					attribute.String("llm.response.model", responseModel),
					attribute.String("llm.response.finish_reason", responseFinishReason),
					attribute.Bool("stream", true),
				))
			}
			if usage != nil {
				a.AddTokenUsage(ctx, usage, responseModel, true)
				a.streamingTimePerOutputToken.Record(ctx, float64(endTime.Sub(timeOfFirstToken).Milliseconds())/float64(usage.CompletionTokens), metric.WithAttributes(
					attribute.String("llm.response.model", responseModel),
					attribute.Bool("stream", true),
				))
			}
			a.chatDurationHistogram.Record(ctx, float64(endTime.Sub(startTime).Milliseconds()), metric.WithAttributes(
				attribute.String("llm.response.model", responseModel),
				attribute.Bool("stream", true),
			))

			a.streamingTimeToFirstToken.Record(ctx, float64(timeOfFirstToken.Sub(startTime).Milliseconds()), metric.WithAttributes(
				attribute.String("llm.response.model", responseModel),
				attribute.Bool("stream", true),
			))
			a.streamingTimeToGenerate.Record(ctx, float64(endTime.Sub(timeOfFirstToken).Milliseconds()), metric.WithAttributes(
				attribute.String("llm.response.model", responseModel),
				attribute.Bool("stream", true),
			))
		}

	}()

	return ctx
}

func (a *apmplusHandler) AddTokenUsage(ctx context.Context, usage *model.TokenUsage, responseModel string, isStream bool) {
	if usage != nil {
		a.tokenUsage.Add(ctx, int64(usage.TotalTokens), metric.WithAttributes(
			attribute.String("llm.request.model", responseModel),
			attribute.String("llm.usage.token_type", "total"),
			attribute.Bool("stream", isStream),
		))
		a.tokenUsage.Add(ctx, int64(usage.CompletionTokens), metric.WithAttributes(
			attribute.String("llm.request.model", responseModel),
			attribute.String("llm.usage.token_type", "completion"),
			attribute.Bool("stream", isStream),
		))
		a.tokenUsage.Add(ctx, int64(usage.PromptTokens), metric.WithAttributes(
			attribute.String("llm.request.model", responseModel),
			attribute.String("llm.usage.token_type", "prompt"),
			attribute.Bool("stream", isStream),
		))
	}
}

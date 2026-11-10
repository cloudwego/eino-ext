/*
 * Copyright 2026 CloudWeGo Authors
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

package otel

import (
	"context"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// GenAI semantic convention attribute keys.
const (
	attrOperationName = "gen_ai.operation.name"
	attrSystem        = "gen_ai.system"
	attrRequestModel  = "gen_ai.request.model"
	attrUsageInput    = "gen_ai.usage.input_tokens"
	attrUsageOutput   = "gen_ai.usage.output_tokens"
)

type spanStateKey struct{}

type spanState struct {
	span trace.Span
}

// Handler reports component invocations as OpenTelemetry spans following the
// GenAI semantic conventions. It implements callbacks.Handler and
// callbacks.TimingChecker (via the builder) for all five timings.
type Handler struct {
	tracer trace.Tracer
}

// NewHandler builds a callbacks.Handler that emits OpenTelemetry spans for
// every component invocation observed through the Eino callback system.
//
// tp is the TracerProvider to obtain tracers from. Use the provider already
// configured with your OTLP exporter, or otel.GetTracerProvider() when the
// SDK has been set as the global provider.
func NewHandler(tp trace.TracerProvider, opts ...Option) callbacks.Handler {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	h := &Handler{tracer: tp.Tracer(o.tracerName)}

	return callbacks.NewHandlerBuilder().
		OnStartFn(h.onStart).
		OnEndFn(h.onEnd).
		OnErrorFn(h.onError).
		OnStartWithStreamInputFn(h.onStartWithStreamInput).
		OnEndWithStreamOutputFn(h.onEndWithStreamOutput).
		Build()
}

func (h *Handler) onStart(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
	if info == nil {
		return ctx
	}

	ctx, span := h.tracer.Start(ctx, spanName(info), trace.WithAttributes(startAttrs(info, input)...))
	return context.WithValue(ctx, spanStateKey{}, &spanState{span: span})
}

func (h *Handler) onEnd(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
	if info == nil {
		return ctx
	}
	st, _ := ctx.Value(spanStateKey{}).(*spanState)
	if st == nil {
		return ctx
	}

	st.span.SetAttributes(endAttrs(info, output)...)
	st.span.End()
	return ctx
}

func (h *Handler) onError(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
	if info == nil {
		return ctx
	}
	st, _ := ctx.Value(spanStateKey{}).(*spanState)
	if st == nil {
		return ctx
	}

	st.span.RecordError(err)
	st.span.SetStatus(codes.Error, err.Error())
	st.span.End()
	return ctx
}

func (h *Handler) onStartWithStreamInput(ctx context.Context, info *callbacks.RunInfo, input *schema.StreamReader[callbacks.CallbackInput]) context.Context {
	if info == nil {
		if input != nil {
			input.Close()
		}
		return ctx
	}

	ctx, span := h.tracer.Start(ctx, spanName(info), trace.WithAttributes(startAttrs(info, nil)...))
	drain(span, input)
	return ctx
}

func (h *Handler) onEndWithStreamOutput(ctx context.Context, info *callbacks.RunInfo, output *schema.StreamReader[callbacks.CallbackOutput]) context.Context {
	if info == nil {
		if output != nil {
			output.Close()
		}
		return ctx
	}

	ctx, span := h.tracer.Start(ctx, spanName(info), trace.WithAttributes(startAttrs(info, nil)...))
	drain(span, output)
	return ctx
}

// drain consumes the handler's copy of a stream in the background and ends the
// span once the stream is exhausted or closed. This keeps the span alive for
// the duration of the stream without leaking the reader copy.
func drain[T any](span trace.Span, reader *schema.StreamReader[T]) {
	if reader == nil {
		span.End()
		return
	}

	go func() {
		defer span.End()
		defer reader.Close()
		for {
			if _, err := reader.Recv(); err != nil {
				return
			}
		}
	}()
}

func spanName(info *callbacks.RunInfo) string {
	if info.Name != "" {
		return info.Name
	}
	return string(info.Component)
}

// startAttrs builds the span attributes available at invocation start.
func startAttrs(info *callbacks.RunInfo, input callbacks.CallbackInput) []attribute.KeyValue {
	attrs := make([]attribute.KeyValue, 0, 3)

	if op, ok := operationName(info.Component); ok {
		attrs = append(attrs, attribute.String(attrOperationName, op))
	}
	if info.Type != "" {
		attrs = append(attrs, attribute.String(attrSystem, strings.ToLower(info.Type)))
	}
	if m := requestModel(info.Component, input); m != "" {
		attrs = append(attrs, attribute.String(attrRequestModel, m))
	}

	return attrs
}

// endAttrs builds the span attributes available at invocation end (usage).
func endAttrs(info *callbacks.RunInfo, output callbacks.CallbackOutput) []attribute.KeyValue {
	switch info.Component {
	case components.ComponentOfChatModel, components.ComponentOfAgenticModel:
		if mo := model.ConvCallbackOutput(output); mo != nil && mo.TokenUsage != nil {
			return []attribute.KeyValue{
				attribute.Int(attrUsageInput, mo.TokenUsage.PromptTokens),
				attribute.Int(attrUsageOutput, mo.TokenUsage.CompletionTokens),
			}
		}
	case components.ComponentOfEmbedding:
		if eo := embedding.ConvCallbackOutput(output); eo != nil && eo.TokenUsage != nil {
			return []attribute.KeyValue{
				attribute.Int(attrUsageInput, eo.TokenUsage.PromptTokens),
			}
		}
	}
	return nil
}

// requestModel extracts the model name from a component's callback input when
// that component exposes it.
func requestModel(c components.Component, input callbacks.CallbackInput) string {
	switch c {
	case components.ComponentOfChatModel, components.ComponentOfAgenticModel:
		if mi := model.ConvCallbackInput(input); mi != nil && mi.Config != nil {
			return mi.Config.Model
		}
	case components.ComponentOfEmbedding:
		if ei := embedding.ConvCallbackInput(input); ei != nil && ei.Config != nil {
			return ei.Config.Model
		}
	}
	return ""
}

// operationName maps an Eino component kind to a GenAI operation name.
// The second return value reports whether the component is a GenAI operation.
func operationName(c components.Component) (string, bool) {
	switch c {
	case components.ComponentOfChatModel, components.ComponentOfAgenticModel:
		return "chat", true
	case components.ComponentOfEmbedding, components.ComponentOfIndexer:
		return "embeddings", true
	case components.ComponentOfRetriever:
		return "retrieve", true
	case components.ComponentOfTool, compose.ComponentOfToolsNode, compose.ComponentOfAgenticToolsNode:
		return "execute_tool", true
	case adk.ComponentOfAgent, adk.ComponentOfAgenticAgent,
		compose.ComponentOfGraph, compose.ComponentOfWorkflow, compose.ComponentOfChain:
		return "invoke_agent", true
	default:
		return "", false
	}
}

package opentelemetry

import (
	"context"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type options struct {
	scopeName string
	tp        trace.TracerProvider
}

type Option func(*options)

func newOptions(opts ...Option) *options {
	o := &options{
		scopeName: "github.com/cloudwego/eino-ext/callbacks/opentelemetry",
		tp:        otel.GetTracerProvider(),
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

type Handler struct {
	tracer trace.Tracer
}

var _ callbacks.Handler = (*Handler)(nil)

func NewOpenTelemetryHandler(opts ...Option) *Handler {
	o := newOptions(opts...)

	return &Handler{
		tracer: o.tp.Tracer(o.scopeName),
	}
}

type spanKey struct{}

func (h *Handler) OnStart(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
	if info == nil {
		return ctx
	}

	ctx, span := h.tracer.Start(ctx, getName(info))

	if info.Component == components.ComponentOfChatModel {
		mcbi := model.ConvCallbackInput(input)
		span.SetAttributes(
			attribute.String("gen_ai.request.model", mcbi.Config.Model),
			attribute.Int("gen_ai.request.max_tokens", mcbi.Config.MaxTokens),
			attribute.Float64("gen_ai.request.temperature", float64(mcbi.Config.Temperature)),
			attribute.Float64("gen_ai.request.top_p", float64(mcbi.Config.TopP)),
			attribute.StringSlice("gen_ai.response.finish_reasons", mcbi.Config.Stop),
		)
	}

	if in, err := sonic.MarshalString(input); err == nil {
		span.SetAttributes(attribute.String("eino.input.messages", in))
	}

	return context.WithValue(ctx, spanKey{}, span)
}

func (h *Handler) OnEnd(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
	span, ok := ctx.Value(spanKey{}).(trace.Span)
	if !ok {
		return ctx
	}
	if !span.IsRecording() {
		return ctx
	}
	defer span.End()

	if info == nil {
		return ctx
	}

	if info.Component == components.ComponentOfChatModel {
		mcbo := model.ConvCallbackOutput(output)
		span.SetAttributes(
			attribute.Int("gen_ai.usage.input_tokens", mcbo.TokenUsage.PromptTokens),
			attribute.Int("gen_ai.usage.output_tokens", mcbo.TokenUsage.CompletionTokens),
		)
	}
	if out, err := sonic.MarshalString(output); err == nil {
		span.SetAttributes(attribute.String("eino.output.messages", out))
	}

	return ctx
}

func (h *Handler) OnError(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
	span, ok := ctx.Value(spanKey{}).(trace.Span)
	if !ok {
		return ctx
	}
	if !span.IsRecording() {
		return ctx
	}
	defer span.End()

	if info == nil {
		return ctx
	}

	span.SetStatus(codes.Error, err.Error())
	span.RecordError(err)

	return ctx
}

func (h *Handler) OnStartWithStreamInput(ctx context.Context, info *callbacks.RunInfo, input *schema.StreamReader[callbacks.CallbackInput]) context.Context {
	if info == nil {
		return ctx
	}

	// todo: implement

	return ctx
}

func (h *Handler) OnEndWithStreamOutput(ctx context.Context, info *callbacks.RunInfo, output *schema.StreamReader[callbacks.CallbackOutput]) context.Context {
	// TODO implement me
	panic("implement me")
}

func getName(info *callbacks.RunInfo) string {
	if len(info.Name) != 0 {
		return info.Name
	}
	return info.Type + string(info.Component)
}

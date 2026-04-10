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

package tls

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"strings"
	"time"

	sem_ai "github.com/cloudwego/eino-ext/callbacks/tls/semconv"
	"github.com/cloudwego/eino-ext/libs/acl/opentelemetry"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/schema"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	otelsemconv "go.opentelemetry.io/otel/semconv/v1.27.0"
	"go.opentelemetry.io/otel/trace"
)

const scopeName = "github.com/cloudwego/eino-ext/callbacks/tls"

type TLSConfig struct {
	// TLSEndpoint is the TLS OTLP endpoint.
	// Example: "tls-cn-beijing.volces.com:4317"
	TLSEndpoint string

	// AppName is the application name shown in TLS.
	AppName string

	// TLSOTLPHeader contains OTLP export headers.
	TLSOTLPHeader map[string]string

	// TLSOTLPHeadersStr is the comma-separated OTLP headers string.
	TLSOTLPHeadersStr string

	// Release is the version or release identifier.
	Release string
}

func NewTLSCallbackHandler(config ...*TLSConfig) (handler callbacks.Handler, shutdown func(ctx context.Context) error, err error) {
	cfg, err := resolveTLSConfig(config...)
	if err != nil {
		return nil, nil, err
	}

	return buildTLSCallbackHandler(cfg, newOptions())
}

func NewTLSCallbackHandlerWithOptions(cfg *TLSConfig, opts ...Option) (handler callbacks.Handler, shutdown func(ctx context.Context) error, err error) {
	if cfg == nil {
		cfg, err = LoadTLSConfigFromEnv()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to load TLS config from environment: %w", err)
		}
	} else if err = ValidateTLSConfig(cfg); err != nil {
		return nil, nil, fmt.Errorf("failed to validate TLS config: %w", err)
	}

	return buildTLSCallbackHandler(cfg, newOptions(opts...))
}

// NewTLSCallbackHandlerFromEnv keeps the original API shape for callers already passing a prepared config.
func NewTLSCallbackHandlerFromEnv(cfg *TLSConfig) (handler callbacks.Handler, shutdown func(ctx context.Context) error, err error) {
	return NewTLSCallbackHandlerWithOptions(cfg)
}

// TLSCallbackHandler implements callbacks.Handler.
type TLSCallbackHandler struct {
	OtelProvider *opentelemetry.OtelProvider
	AppName      string
	Release      string
	Tracer       trace.Tracer
	dataParser   CallbackDataParser
}

// TLSHandler extends TLSCallbackHandler with manual span controls.
type TLSHandler struct {
	*TLSCallbackHandler
}

func NewTLSHandler(config ...*TLSConfig) (*TLSHandler, func(ctx context.Context) error, error) {
	cfg, err := resolveTLSConfig(config...)
	if err != nil {
		return nil, nil, err
	}

	return NewTLSHandlerWithOptions(cfg)
}

func NewTLSHandlerWithOptions(cfg *TLSConfig, opts ...Option) (*TLSHandler, func(ctx context.Context) error, error) {
	handler, shutdown, err := NewTLSCallbackHandlerWithOptions(cfg, opts...)
	if err != nil {
		return nil, shutdown, err
	}

	tlsHandler, ok := handler.(*TLSCallbackHandler)
	if !ok {
		return nil, shutdown, errors.New("failed to cast handler to *TLSCallbackHandler")
	}

	return &TLSHandler{TLSCallbackHandler: tlsHandler}, shutdown, nil
}

// NewTLSHandlerFromEnv keeps the original API shape for callers already passing a prepared config.
func NewTLSHandlerFromEnv(cfg *TLSConfig) (*TLSHandler, func(ctx context.Context) error, error) {
	return NewTLSHandlerWithOptions(cfg)
}

type RequestInfo struct {
	Model string
}

type TLSStateKey struct{}

type TLSState struct {
	StartTime   time.Time
	Span        trace.Span
	RequestInfo *RequestInfo
	IsRootNode  bool
}

type TLSStreamInputAsyncKey struct{}

type TLSStreamInputAsyncVal chan struct{}

func buildTLSCallbackHandler(cfg *TLSConfig, opts *options) (callbacks.Handler, func(ctx context.Context) error, error) {
	otelOpts := []opentelemetry.Option{
		opentelemetry.WithServiceName(cfg.AppName),
		opentelemetry.WithExportEndpoint(cfg.TLSEndpoint),
		opentelemetry.WithTLSInsecure(),
		opentelemetry.WithHeaders(cfg.TLSOTLPHeader),
		opentelemetry.WithResourceAttribute(attribute.String("tls.business_type", "gen_ai")),
		opentelemetry.WithEnableTracing(true),
		opentelemetry.WithEnableMetrics(false),
	}
	if cfg.Release != "" {
		otelOpts = append(otelOpts, opentelemetry.WithResourceAttribute(otelsemconv.ServiceVersionKey.String(cfg.Release)))
	}

	p, err := opentelemetry.NewOpenTelemetryProvider(otelOpts...)
	if err != nil {
		return nil, nil, fmt.Errorf("init opentelemetry provider failed: %w", err)
	}
	if p == nil {
		return nil, nil, errors.New("init opentelemetry provider failed")
	}
	if p.TracerProvider == nil {
		return nil, p.Shutdown, errors.New("tracer provider is nil")
	}

	parser := newDefaultDataParserWithConcatFuncs(opts.concatFuncs, opts.enableAggrOutput)
	if opts.parser != nil {
		parser = opts.parser
	}

	return &TLSCallbackHandler{
		OtelProvider: p,
		AppName:      cfg.AppName,
		Release:      cfg.Release,
		Tracer:       p.TracerProvider.Tracer(scopeName),
		dataParser:   parser,
	}, p.Shutdown, nil
}

func resolveTLSConfig(config ...*TLSConfig) (*TLSConfig, error) {
	if len(config) > 0 && config[0] != nil {
		if err := ValidateTLSConfig(config[0]); err != nil {
			return nil, fmt.Errorf("failed to validate TLS config: %w", err)
		}
		return config[0], nil
	}

	cfg, err := LoadTLSConfigFromEnv()
	if err != nil {
		return nil, fmt.Errorf("failed to load TLS config from environment: %w", err)
	}

	return cfg, nil
}

func setCommonSpanAttributes(ctx context.Context, span trace.Span, info *callbacks.RunInfo) bool {
	isRootNode := ctx.Value(TLSStateKey{}) == nil
	spanKind := _eino_run_component_to_span_kind(info.Component)
	if isRootNode && spanKind == sem_ai.GEN_AI_SPAN_KIND_TASK {
		span.SetAttributes(attribute.String(sem_ai.GEN_AI_SPAN_KIND, sem_ai.GEN_AI_SPAN_KIND_CHAIN))
	} else {
		span.SetAttributes(attribute.String(sem_ai.GEN_AI_SPAN_KIND, spanKind))
	}

	span.SetAttributes(
		attribute.String(sem_ai.GEN_AI_SYSTEM, "eino"),
		attribute.String(sem_ai.GEN_AI_FRAMEWORK, "eino"),
		attribute.String(sem_ai.TASK_NAME, info.Name),
		attribute.String("runinfo.name", info.Name),
		attribute.String("runinfo.type", info.Type),
		attribute.String("runinfo.component", string(info.Component)),
	)
	setSessionAttributes(ctx, span)

	return isRootNode
}

func setSessionAttributes(ctx context.Context, span trace.Span) {
	session, ok := ctx.Value(tlsSessionOptionKey{}).(*sessionOptions)
	if !ok || session == nil {
		return
	}

	if session.SessionID != "" {
		span.SetAttributes(attribute.String(sem_ai.GEN_AI_SESSION_ID, session.SessionID))
	}
	if session.UserID != "" {
		span.SetAttributes(attribute.String(sem_ai.GEN_AI_USER_ID, session.UserID))
	}
}

func waitStreamInput(ctx context.Context) {
	if stopCh, ok := ctx.Value(TLSStreamInputAsyncKey{}).(TLSStreamInputAsyncVal); ok {
		<-stopCh
	}
}

func endSpan(ctx context.Context, span trace.Span, status codes.Code, description string) {
	waitStreamInput(ctx)
	span.SetStatus(status, description)
	span.End(trace.WithTimestamp(time.Now()))
}

func (h *TLSCallbackHandler) OnStart(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
	if h == nil || info == nil {
		return ctx
	}

	info = completeRunInfo(info)
	ctx = injectToolIDNameMapToCtx(ctx, info, input)

	startTime := time.Now()
	ctx, span := h.Tracer.Start(ctx, getSpanName(info), trace.WithSpanKind(trace.SpanKindClient), trace.WithTimestamp(startTime))
	isRootNode := setCommonSpanAttributes(ctx, span, info)

	if h.dataParser != nil {
		tags, err := h.dataParser.ParseInput(ctx, info, input)
		if err != nil {
			log.Printf("ParseInput failed, info: %+v, err: %+v", info, err)
		} else {
			setSpanAttributesFromTags(span, tags)
		}
	}

	return context.WithValue(ctx, TLSStateKey{}, &TLSState{
		StartTime:   startTime,
		Span:        span,
		RequestInfo: &RequestInfo{},
		IsRootNode:  isRootNode,
	})
}

func (h *TLSCallbackHandler) OnEnd(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
	if h == nil || info == nil {
		return ctx
	}

	state, ok := ctx.Value(TLSStateKey{}).(*TLSState)
	if !ok || state == nil || state.Span == nil {
		log.Printf("no state in context, runinfo: %+v", info)
		return ctx
	}

	defer endSpan(ctx, state.Span, codes.Ok, "")

	if h.dataParser != nil {
		tags, err := h.dataParser.ParseOutput(ctx, completeRunInfo(info), output)
		if err != nil {
			log.Printf("ParseOutput failed, info: %+v, err: %+v", info, err)
		} else {
			setSpanAttributesFromTags(state.Span, tags)
		}
	}

	return ctx
}

func (h *TLSCallbackHandler) OnError(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
	if h == nil || info == nil {
		return ctx
	}

	state, ok := ctx.Value(TLSStateKey{}).(*TLSState)
	if !ok || state == nil || state.Span == nil {
		log.Printf("no state in context, runinfo: %+v", info)
		return ctx
	}

	if err != nil {
		state.Span.RecordError(err)
		defer endSpan(ctx, state.Span, codes.Error, err.Error())
	} else {
		defer endSpan(ctx, state.Span, codes.Error, "")
	}

	return ctx
}

func (h *TLSCallbackHandler) OnStartWithStreamInput(ctx context.Context, info *callbacks.RunInfo, input *schema.StreamReader[callbacks.CallbackInput]) context.Context {
	if h == nil {
		input.Close()
		return ctx
	}
	if info == nil {
		input.Close()
		return ctx
	}

	info = completeRunInfo(info)
	startTime := time.Now()
	ctx, span := h.Tracer.Start(ctx, getSpanName(info), trace.WithSpanKind(trace.SpanKindClient), trace.WithTimestamp(startTime))
	isRootNode := setCommonSpanAttributes(ctx, span, info)

	stopCh := make(TLSStreamInputAsyncVal)
	ctx = context.WithValue(ctx, TLSStreamInputAsyncKey{}, stopCh)

	if h.dataParser != nil {
		go func() {
			defer func() {
				if e := recover(); e != nil {
					log.Printf("recover update span panic: %v, runinfo: %+v, stack: %s", e, info, string(debug.Stack()))
				}
				close(stopCh)
			}()

			tags, err := h.dataParser.ParseStreamInput(ctx, info, input)
			if err != nil {
				log.Printf("ParseStreamInput failed, info: %+v, err: %+v", info, err)
				return
			}
			setSpanAttributesFromTags(span, tags)
		}()
	} else {
		input.Close()
		close(stopCh)
	}

	return context.WithValue(ctx, TLSStateKey{}, &TLSState{
		StartTime:   startTime,
		Span:        span,
		RequestInfo: &RequestInfo{},
		IsRootNode:  isRootNode,
	})
}

func (h *TLSCallbackHandler) OnEndWithStreamOutput(ctx context.Context, info *callbacks.RunInfo, output *schema.StreamReader[callbacks.CallbackOutput]) context.Context {
	if h == nil {
		output.Close()
		return ctx
	}
	if info == nil {
		output.Close()
		return ctx
	}

	state, ok := ctx.Value(TLSStateKey{}).(*TLSState)
	if !ok || state == nil || state.Span == nil {
		log.Printf("no state in context, runinfo: %+v", info)
		output.Close()
		return ctx
	}

	info = completeRunInfo(info)
	if h.dataParser != nil {
		go func() {
			defer func() {
				if e := recover(); e != nil {
					log.Printf("recover update span panic: %v, runinfo: %+v, stack: %s", e, info, string(debug.Stack()))
				}
				endSpan(ctx, state.Span, codes.Ok, "")
			}()

			tags, err := h.dataParser.ParseStreamOutput(ctx, info, output)
			if err != nil {
				log.Printf("ParseStreamOutput failed, info: %+v, err: %+v", info, err)
				return
			}
			setSpanAttributesFromTags(state.Span, tags)
		}()
	} else {
		output.Close()
		endSpan(ctx, state.Span, codes.Ok, "")
	}

	return ctx
}

func (h *TLSHandler) StartSpan(ctx context.Context, spanName string, tracingTags map[string]any) context.Context {
	if h == nil {
		return ctx
	}

	startTime := time.Now()
	if spanName == "" {
		spanName = DefaultSpanName
	}

	ctx, span := h.Tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient), trace.WithTimestamp(startTime))
	setSessionAttributes(ctx, span)
	setSpanAttributesFromTags(span, tracingTags)

	return context.WithValue(ctx, TLSStateKey{}, &TLSState{
		StartTime:   startTime,
		Span:        span,
		RequestInfo: nil,
		IsRootNode:  true,
	})
}

func (h *TLSHandler) FinishSpan(ctx context.Context, tracingTags map[string]any) context.Context {
	state, ok := ctx.Value(TLSStateKey{}).(*TLSState)
	if !ok || state == nil || state.Span == nil {
		log.Printf("no state in context")
		return ctx
	}

	setSpanAttributesFromTags(state.Span, tracingTags)
	state.Span.SetStatus(codes.Ok, "")
	state.Span.End(trace.WithTimestamp(time.Now()))
	return ctx
}

// SetSession stores session attributes in ctx.
func (h *TLSHandler) SetSession(ctx context.Context, sessionID, userID string) context.Context {
	return SetSession(ctx, WithSessionID(sessionID), WithUserID(userID))
}

func ValidateTLSConfig(cfg *TLSConfig) error {
	if cfg == nil {
		return errors.New("config cannot be nil")
	}
	if cfg.TLSEndpoint == "" || cfg.AppName == "" {
		return errors.New("TLSEndpoint and AppName are required")
	}

	if cfg.TLSOTLPHeadersStr != "" {
		cfg.TLSOTLPHeader = parseOTLPHeaders(cfg.TLSOTLPHeadersStr)
	}
	if len(cfg.TLSOTLPHeader) == 0 {
		return errors.New("TLSOTLPHeader is required, either directly set or via TLSOTLPHeadersStr")
	}

	return nil
}

func LoadTLSConfigFromEnv() (*TLSConfig, error) {
	cfg := &TLSConfig{
		TLSEndpoint:       os.Getenv("TLS_ENDPOINT"),
		AppName:           os.Getenv("TLS_APP_NAME"),
		Release:           getEnvOrDefault("TLS_AGENT_VERSION", DefaultVersion),
		TLSOTLPHeadersStr: os.Getenv("TLS_EXPORTER_OTLP_HEADERS"),
	}

	return cfg, ValidateTLSConfig(cfg)
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getSpanName(info *callbacks.RunInfo) string {
	spanName := getName(info)
	if spanName == "" {
		return DefaultSpanName
	}

	return spanName
}

func parseOTLPHeaders(headersStr string) map[string]string {
	headers := make(map[string]string)
	if headersStr == "" {
		return headers
	}

	pairs := strings.Split(headersStr, ",")
	for _, pair := range pairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			headers[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}

	return headers
}

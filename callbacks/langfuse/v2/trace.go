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

package langfuse

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"strings"
	"sync"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

const (
	ObservationTypeEvent      ObservationType = "event"
	ObservationTypeSpan       ObservationType = "span"
	ObservationTypeGeneration ObservationType = "generation"
	ObservationTypeAgent      ObservationType = "agent"
	ObservationTypeTool       ObservationType = "tool"
	ObservationTypeChain      ObservationType = "chain"
	ObservationTypeRetriever  ObservationType = "retriever"
	ObservationTypeEvaluator  ObservationType = "evaluator"
	ObservationTypeEmbedding  ObservationType = "embedding"
	ObservationTypeGuardrail  ObservationType = "guardrail"
)

type desiredTraceIDKey struct{}
type traceRunKey struct{}

type ObservationType string

type traceOptions struct {
	ID              string
	Name            string
	UserID          string
	Input           string
	SessionID       string
	Release         string
	Tags            []string
	Public          bool
	Metadata        map[string]any
	Environment     string
	Version         string
	ObservationType ObservationType
}

type TraceOption func(*traceOptions)

type traceRun struct {
	mu              sync.Mutex
	span            trace.Span
	attributeWriter *spanAttributeWriter
	attributes      []attribute.KeyValue
	active          int
	autoEnd         bool
	endRequested    bool
	terminalOutput  string
	ended           bool
	failed          bool
	lastOutput      string
	done            chan struct{}
	onEnd           func()
}

type traceRunRegistry struct {
	runs sync.Map
}

type contextIDGenerator struct{}

// StartTrace creates the application root observation. It is ended explicitly
// by EndTrace, when ctx is done and all active Eino child callbacks have ended,
// or when the handler shuts down.
func (c *CallbackHandler) StartTrace(ctx context.Context, opts ...TraceOption) context.Context {
	options := c.traceDefaults
	options.Tags = append([]string(nil), c.traceDefaults.Tags...)
	options.ObservationType = ObservationTypeSpan
	for _, opt := range opts {
		opt(&options)
	}

	options.Name = defaultString(options.Name, "eino-trace")
	traceID := requestedTraceID(ctx, options.ID)
	rootCtx := trace.ContextWithSpanContext(ctx, trace.SpanContext{})
	if traceID.IsValid() {
		rootCtx = context.WithValue(rootCtx, desiredTraceIDKey{}, traceID)
	}
	traceAttrs := c.traceAttributes(&options)
	rootAttrs := append([]attribute.KeyValue{
		attribute.String("langfuse.observation.type", string(options.ObservationType)),
	}, traceAttrs...)
	rootAttrs = appendStringAttribute(rootAttrs, "langfuse.observation.input", c.prepare(options.Input))
	rootAttrs, remaining := fitAttributes(rootAttrs, c.maxSpanAttributeBytes, c.losses)
	rootCtx, span := c.tracer.Start(rootCtx, options.Name,
		trace.WithAttributes(rootAttrs...))
	run := &traceRun{
		span:            span,
		attributeWriter: newSpanAttributeWriter(span, c.prepare, remaining, c.losses),
		attributes:      traceAttrs,
		done:            make(chan struct{}),
	}
	run.onEnd = func() {
		c.activeTraceRun.delete(run)
	}
	c.activeTraceRun.add(run)
	run.watchContext(ctx)
	return context.WithValue(rootCtx, traceRunKey{}, run)
}

// EndTrace records the final root-observation output and requests the trace to
// end. The root waits for active Eino child callbacks before it is exported.
// Calling EndTrace more than once is safe.
func (c *CallbackHandler) EndTrace(ctx context.Context, output string) {
	run, _ := ctx.Value(traceRunKey{}).(*traceRun)
	if run != nil {
		run.requestEnd(output)
	}
}

func WithID(id string) TraceOption {
	return func(options *traceOptions) { options.ID = id }
}

func WithName(name string) TraceOption {
	return func(options *traceOptions) { options.Name = name }
}

func WithUserID(userID string) TraceOption {
	return func(options *traceOptions) { options.UserID = userID }
}

func WithInput(input string) TraceOption {
	return func(options *traceOptions) { options.Input = input }
}

func WithSessionID(sessionID string) TraceOption {
	return func(options *traceOptions) { options.SessionID = sessionID }
}

func WithRelease(release string) TraceOption {
	return func(options *traceOptions) { options.Release = release }
}

func WithTags(tags ...string) TraceOption {
	return func(options *traceOptions) { options.Tags = append(options.Tags, tags...) }
}

func WithPublic(public bool) TraceOption {
	return func(options *traceOptions) { options.Public = public }
}

func WithMetadata(metadata map[string]string) TraceOption {
	return func(options *traceOptions) {
		if options.Metadata == nil {
			options.Metadata = make(map[string]any, len(metadata))
		}
		for key, value := range metadata {
			options.Metadata[key] = value
		}
	}
}

func WithMetadataValues(metadata map[string]any) TraceOption {
	return func(options *traceOptions) {
		if options.Metadata == nil {
			options.Metadata = make(map[string]any, len(metadata))
		}
		for key, value := range metadata {
			options.Metadata[key] = value
		}
	}
}

func WithEnvironment(environment string) TraceOption {
	return func(options *traceOptions) { options.Environment = environment }
}

func WithVersion(version string) TraceOption {
	return func(options *traceOptions) { options.Version = version }
}

func WithObservationType(observationType ObservationType) TraceOption {
	return func(options *traceOptions) { options.ObservationType = observationType }
}

func (c *CallbackHandler) traceAttributes(options *traceOptions) []attribute.KeyValue {
	var attrs []attribute.KeyValue
	attrs = appendStringAttribute(attrs, "langfuse.trace.name", options.Name)
	attrs = appendStringAttribute(attrs, "langfuse.user.id", options.UserID)
	attrs = appendStringAttribute(attrs, "langfuse.session.id", options.SessionID)
	attrs = appendStringAttribute(attrs, "langfuse.release", options.Release)
	attrs = appendStringAttribute(attrs, "langfuse.environment", options.Environment)
	attrs = appendStringAttribute(attrs, "langfuse.version", options.Version)
	if len(options.Tags) > 0 {
		attrs = append(attrs, attribute.StringSlice("langfuse.trace.tags", options.Tags))
	}
	if options.Public {
		attrs = append(attrs, attribute.Bool("langfuse.trace.public", true))
	}
	for key, value := range options.Metadata {
		encoded, err := json.Marshal(value)
		if err != nil {
			c.losses.Add(lossMetadataSerialization, 1)
			continue
		}
		if text, ok := value.(string); ok {
			encoded = []byte(text)
		}
		attrs = append(attrs, attribute.String("langfuse.trace.metadata."+metadataKey(key), c.prepare(string(encoded))))
	}
	return attrs
}

func (r *traceRun) childStarted() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.ended || r.endRequested {
		return false
	}
	r.active++
	return true
}

func (r *traceRun) childEnded(output string) {
	r.mu.Lock()
	if r.ended {
		r.mu.Unlock()
		return
	}
	if output != "" {
		r.lastOutput = output
	}
	if r.active > 0 {
		r.active--
	}
	shouldEnd := r.active == 0 && (r.endRequested || r.autoEnd)
	r.mu.Unlock()
	if shouldEnd {
		r.end("")
	}
}

func (r *traceRun) watchContext(ctx context.Context) {
	if ctx.Done() == nil {
		return
	}
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				recoveredPanic("trace context watcher", recovered)
			}
		}()
		select {
		case <-ctx.Done():
			r.contextEnded(ctx)
		case <-r.done:
		}
	}()
}

func (r *traceRun) contextEnded(ctx context.Context) {
	err := ctx.Err()
	cause := context.Cause(ctx)
	if cause == nil {
		cause = err
	}
	if causeText, interrupted := interruptionCause(cause); interrupted {
		message := "trace context: interrupted"
		r.recordInterruption(message, causeText)
		r.requestEnd(interruptedOutput(causeText))
		return
	}
	if errors.Is(err, context.Canceled) {
		if errors.Is(cause, context.Canceled) {
			r.requestEnd("")
			return
		}
		causeText := cause.Error()
		message := "trace context: cancelled"
		message += ": " + causeText
		r.recordCancellation(message, causeText)
		r.requestEnd(cancelledOutput(causeText))
		return
	}
	message := "trace context: " + cause.Error()
	r.recordError(message, cause)
	r.requestEnd(errorOutput(cause.Error()))
}

func (r *traceRun) requestEnd(output string) {
	r.mu.Lock()
	if r.ended {
		r.mu.Unlock()
		return
	}
	r.endRequested = true
	r.terminalOutput = output
	shouldEnd := r.active == 0
	r.mu.Unlock()
	if shouldEnd {
		r.end("")
	}
}

func (r *traceRun) recordError(message string, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.ended {
		return
	}
	r.failed = true
	r.span.RecordError(err)
	r.span.SetStatus(codes.Error, message)
	r.attributeWriter.setString("langfuse.observation.level", "ERROR")
	r.attributeWriter.setString("langfuse.observation.status_message", message)
}

func (r *traceRun) recordCancellation(message, cause string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.ended {
		return
	}
	if !r.failed {
		r.attributeWriter.setString("langfuse.observation.status_message", message)
	}
	r.attributeWriter.setString("langfuse.observation.metadata.eino_callback_cancelled", "true")
	r.attributeWriter.setString("langfuse.observation.metadata.eino_cancellation_cause", cause)
}

func (r *traceRun) recordInterruption(message, cause string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.ended {
		return
	}
	if !r.failed {
		r.attributeWriter.setString("langfuse.observation.status_message", message)
	}
	r.attributeWriter.setString("langfuse.observation.metadata.eino_callback_interrupted", "true")
	r.attributeWriter.setString("langfuse.observation.metadata.eino_interrupt_cause", cause)
}

func (r *traceRun) end(output string) {
	r.mu.Lock()
	if r.ended {
		done := r.done
		r.mu.Unlock()
		<-done
		return
	}
	r.ended = true
	if output == "" {
		output = r.terminalOutput
		if output == "" {
			output = r.lastOutput
		}
	}
	span := r.span
	onEnd := r.onEnd
	r.mu.Unlock()

	if output != "" {
		r.attributeWriter.setString("langfuse.observation.output", output)
	}
	span.End()
	if onEnd != nil {
		onEnd()
	}
	close(r.done)
}

func (r *traceRunRegistry) add(run *traceRun) {
	r.runs.Store(run, struct{}{})
}

func (r *traceRunRegistry) delete(run *traceRun) {
	r.runs.Delete(run)
}

func (r *traceRunRegistry) endAll() {
	r.runs.Range(func(key, _ any) bool {
		key.(*traceRun).end("")
		return true
	})
}

func (contextIDGenerator) NewIDs(ctx context.Context) (trace.TraceID, trace.SpanID) {
	traceID, _ := ctx.Value(desiredTraceIDKey{}).(trace.TraceID)
	if !traceID.IsValid() {
		traceID = newTraceID()
	}
	return traceID, newSpanID()
}

func (contextIDGenerator) NewSpanID(context.Context, trace.TraceID) trace.SpanID {
	return newSpanID()
}

func requestedTraceID(ctx context.Context, configured string) trace.TraceID {
	if configured != "" {
		if traceID, err := trace.TraceIDFromHex(configured); err == nil {
			return traceID
		}
	}
	return trace.SpanContextFromContext(ctx).TraceID()
}

func newTraceID() trace.TraceID {
	for {
		var id trace.TraceID
		_, _ = rand.Read(id[:])
		if id.IsValid() {
			return id
		}
	}
}

func newSpanID() trace.SpanID {
	for {
		var id trace.SpanID
		_, _ = rand.Read(id[:])
		if id.IsValid() {
			return id
		}
	}
}

func appendStringAttribute(attrs []attribute.KeyValue, key, value string) []attribute.KeyValue {
	if value == "" {
		return attrs
	}
	return append(attrs, attribute.String(key, value))
}

func metadataKey(key string) string {
	key = strings.TrimSpace(strings.ReplaceAll(key, ".", "_"))
	switch key {
	case "", "__proto__", "constructor", "prototype":
		return "_" + key
	default:
		return key
	}
}

var _ sdktrace.IDGenerator = contextIDGenerator{}

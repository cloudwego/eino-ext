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
	"encoding/json"
	"errors"
	"io"
	"log"
	"reflect"
	"strconv"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	internalReActGraphName = "ReAct"
	internalInitLambdaName = "Init"
)

type (
	activeAgentNameKey struct{}
	spanStateKey       struct{}
)

type spanState struct {
	span       trace.Span
	attributes *spanAttributeWriter
	run        *traceRun
	ctx        context.Context
	inputDone  chan struct{}
	endOnce    sync.Once
	outcomeMu  sync.Mutex
	failed     bool
}

type asyncGroup struct {
	mu      sync.Mutex
	active  int
	closing bool
	idle    chan struct{}
}

type spanAttributeWriter struct {
	mu        sync.Mutex
	span      trace.Span
	prepare   func(string) string
	remaining int
	losses    *lossReporter
}

func (c *CallbackHandler) OnStart(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
	if info == nil {
		return ctx
	}
	ctx, state := c.startSpan(ctx, info)
	c.setInput(state, info, []callbacks.CallbackInput{input})
	close(state.inputDone)
	ctx = context.WithValue(ctx, spanStateKey{}, state)
	return c.markActiveAgent(ctx, info)
}

// Needed implements callbacks.TimingChecker. When explicitly enabled, it
// removes Eino Agent implementation details while preserving named business
// components and their standard context-based parent relationships.
func (c *CallbackHandler) Needed(ctx context.Context, info *callbacks.RunInfo, _ callbacks.CallbackTiming) bool {
	if !c.collapseAgentInternalSpans || info == nil {
		return true
	}
	activeAgentName, _ := ctx.Value(activeAgentNameKey{}).(string)
	if activeAgentName == "" {
		return true
	}

	switch info.Component {
	case compose.ComponentOfChain:
		return info.Name != activeAgentName
	case compose.ComponentOfGraph:
		return info.Name != internalReActGraphName
	case compose.ComponentOfLambda:
		return info.Name != "" &&
			info.Name != string(compose.ComponentOfLambda) &&
			info.Name != internalInitLambdaName
	default:
		return true
	}
}

func (c *CallbackHandler) OnEnd(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
	state := spanStateFromContext(ctx)
	if info == nil || state == nil {
		return ctx
	}
	if c.endAgentOutput(state, info, output) {
		return ctx
	}
	serialized := c.setOutput(state, info, []callbacks.CallbackOutput{output}, time.Time{})
	state.end(serialized)
	return ctx
}

func (c *CallbackHandler) OnError(ctx context.Context, info *callbacks.RunInfo, callbackErr error) context.Context {
	state := spanStateFromContext(ctx)
	if info == nil || state == nil {
		return ctx
	}
	if callbackErr == nil {
		callbackErr = errors.New("eino callback failed without an error")
	}
	c.async.run(func() {
		<-state.inputDone
		status, cause := state.recordError("callback", callbackErr)
		switch status {
		case "cancelled":
			output := state.attributes.setString("langfuse.observation.output", cancelledOutput(cause))
			state.end(output)
			return
		case "interrupted":
			output := state.attributes.setString("langfuse.observation.output", interruptedOutput(cause))
			state.end(output)
			return
		}
		output := state.attributes.setString("langfuse.observation.output", errorOutput(callbackErr.Error()))
		state.end(output)
	})
	return ctx
}

func (c *CallbackHandler) OnStartWithStreamInput(ctx context.Context, info *callbacks.RunInfo, input *schema.StreamReader[callbacks.CallbackInput]) context.Context {
	if info == nil {
		closeStream(input)
		return ctx
	}
	ctx, state := c.startSpan(ctx, info)
	c.async.run(func() {
		defer close(state.inputDone)
		defer closeStream(input)
		inputs, err := receiveAll(input)
		if err != nil {
			state.recordError("stream input", err)
			return
		}
		c.setInput(state, info, inputs)
	})
	ctx = context.WithValue(ctx, spanStateKey{}, state)
	return c.markActiveAgent(ctx, info)
}

func (c *CallbackHandler) OnEndWithStreamOutput(ctx context.Context, info *callbacks.RunInfo, output *schema.StreamReader[callbacks.CallbackOutput]) context.Context {
	state := spanStateFromContext(ctx)
	if info == nil || state == nil {
		closeStream(output)
		return ctx
	}
	c.async.run(func() {
		defer closeStream(output)
		outputs, completionStart, err := receiveAllWithFirst(output)
		if err != nil {
			state.recordError("stream output", err)
		}
		<-state.inputDone
		serialized := c.setOutput(state, info, outputs, completionStart)
		state.end(serialized)
	})
	return ctx
}

// Flush waits for callback-owned stream collectors and flushes completed spans.
func (c *CallbackHandler) Flush(ctx context.Context) error {
	if err := c.async.wait(ctx, false); err != nil {
		log.Printf("langfuse callback flush interrupted while waiting for active callbacks; telemetry remains pending and may be discarded if the process exits: %v", err)
		return err
	}
	err := c.provider.ForceFlush(ctx)
	if err != nil {
		log.Printf("langfuse callback flush failed; telemetry may not have been exported: %v", err)
	}
	return err
}

// Shutdown completes active root observations, drains stream collectors, and
// shuts down the callback-owned provider.
func (c *CallbackHandler) Shutdown(ctx context.Context) error {
	defer c.losses.Stop()
	if err := c.async.wait(ctx, true); err != nil {
		log.Printf("langfuse callback shutdown interrupted while waiting for active callbacks; telemetry may be discarded if the process exits: %v", err)
		return err
	}
	c.activeTraceRun.endAll()
	if c.ownProvider {
		return c.provider.Shutdown(ctx)
	}
	err := c.provider.ForceFlush(ctx)
	if err != nil {
		log.Printf("langfuse callback shutdown flush failed for caller-owned tracer provider; telemetry may not have been exported: %v", err)
	}
	return err
}

func (c *CallbackHandler) markActiveAgent(ctx context.Context, info *callbacks.RunInfo) context.Context {
	if !c.collapseAgentInternalSpans || info.Name == "" {
		return ctx
	}
	switch info.Component {
	case adk.ComponentOfAgent, adk.ComponentOfAgenticAgent:
		return context.WithValue(ctx, activeAgentNameKey{}, info.Name)
	default:
		return ctx
	}
}

func (c *CallbackHandler) startSpan(ctx context.Context, info *callbacks.RunInfo) (context.Context, *spanState) {
	run, _ := ctx.Value(traceRunKey{}).(*traceRun)
	var attrs []attribute.KeyValue
	if run != nil && run.childStarted() {
		attrs = append(attrs, run.attributes...)
	} else {
		run = nil
	}
	attrs = append(attrs,
		attribute.String("langfuse.observation.type", string(observationType(info))),
		attribute.String("langfuse.observation.metadata.eino_component", string(info.Component)),
		attribute.String("langfuse.observation.metadata.eino_type", info.Type),
	)
	attrs, remaining := fitAttributes(attrs, c.maxSpanAttributeBytes, c.losses)
	ctx, span := c.tracer.Start(ctx, callbackName(info), trace.WithAttributes(attrs...))
	return ctx, &spanState{
		span:       span,
		attributes: newSpanAttributeWriter(span, c.prepare, remaining, c.losses),
		run:        run,
		ctx:        ctx,
		inputDone:  make(chan struct{}),
	}
}

func (c *CallbackHandler) setInput(state *spanState, info *callbacks.RunInfo, inputs []callbacks.CallbackInput) string {
	if isGeneration(info) {
		config, messages, extra, err := extractModelInput(inputs)
		if err != nil {
			state.recordError("model input", err)
			return ""
		}
		serialized := c.setJSONAttribute(state, "langfuse.observation.input", messages)
		setModelConfig(state.attributes.setString, config)
		setMetadata(state.attributes.setString, extra, c.losses)
		return serialized
	}
	return c.setJSONAttribute(state, "langfuse.observation.input", collapseValues(inputs))
}

func (c *CallbackHandler) setOutput(state *spanState, info *callbacks.RunInfo, outputs []callbacks.CallbackOutput, completionStart time.Time) string {
	if isGeneration(info) {
		output, extra, err := extractModelOutput(outputs)
		if err != nil {
			state.recordError("model output", err)
			return ""
		}
		serialized := c.setJSONAttribute(state, "langfuse.observation.output", output.Message)
		setModelConfig(state.attributes.setString, output.Config)
		setMetadata(state.attributes.setString, extra, c.losses)
		setUsage(state.attributes.setInt, modelUsage(output))
		if !completionStart.IsZero() {
			state.attributes.setString("langfuse.observation.completion_start_time", completionStart.UTC().Format(time.RFC3339Nano))
		}
		return serialized
	}
	return c.setJSONAttribute(state, "langfuse.observation.output", collapseValues(outputs))
}

func (c *CallbackHandler) setJSONAttribute(state *spanState, key string, value any) string {
	serialized, err := json.Marshal(value)
	if err != nil {
		c.losses.Add(lossPayloadSerialization, 1)
		state.recordError("serialize "+key, err)
		return ""
	}
	return state.attributes.setString(key, string(serialized))
}

func (c *CallbackHandler) endAgentOutput(state *spanState, info *callbacks.RunInfo, output callbacks.CallbackOutput) bool {
	if info.Component == adk.ComponentOfAgent {
		converted := adk.ConvAgentCallbackOutput(output)
		if converted == nil || converted.Events == nil {
			return false
		}
		c.async.run(func() {
			<-state.inputDone
			state.end(c.collectAgentMessages(state, converted.Events))
		})
		return true
	}
	if info.Component == adk.ComponentOfAgenticAgent {
		converted := adk.ConvTypedCallbackOutput[*schema.AgenticMessage](output)
		if converted == nil || converted.Events == nil {
			return false
		}
		c.async.run(func() {
			<-state.inputDone
			state.end(c.collectAgenticMessages(state, converted.Events))
		})
		return true
	}
	return false
}

func (c *CallbackHandler) collectAgentMessages(state *spanState, events *adk.AsyncIterator[*adk.AgentEvent]) string {
	var messages []*schema.Message
	var customized []any
	for {
		event, ok := events.Next()
		if !ok {
			break
		}
		if event == nil {
			continue
		}
		if event.Err != nil {
			state.recordError("agent output", event.Err)
		}
		if event.Output == nil {
			continue
		}
		if event.Output.MessageOutput != nil {
			message, err := event.Output.MessageOutput.GetMessage()
			if err != nil {
				state.recordError("agent message", err)
			} else if message != nil {
				messages = append(messages, message)
			}
		}
		if event.Output.CustomizedOutput != nil {
			customized = append(customized, event.Output.CustomizedOutput)
		}
	}
	return c.setAgentOutput(state, messages, customized)
}

func (c *CallbackHandler) collectAgenticMessages(state *spanState, events *adk.AsyncIterator[*adk.TypedAgentEvent[*schema.AgenticMessage]]) string {
	var messages []*schema.AgenticMessage
	var customized []any
	for {
		event, ok := events.Next()
		if !ok {
			break
		}
		if event == nil {
			continue
		}
		if event.Err != nil {
			state.recordError("agent output", event.Err)
		}
		if event.Output == nil {
			continue
		}
		if event.Output.MessageOutput != nil {
			message, err := event.Output.MessageOutput.GetMessage()
			if err != nil {
				state.recordError("agent message", err)
			} else if message != nil {
				messages = append(messages, message)
			}
		}
		if event.Output.CustomizedOutput != nil {
			customized = append(customized, event.Output.CustomizedOutput)
		}
	}
	return c.setAgentOutput(state, messages, customized)
}

func (c *CallbackHandler) setAgentOutput(state *spanState, messages, customized any) string {
	output := map[string]any{}
	if values := nonEmptySlice(messages); values != nil {
		output["messages"] = values
	}
	if values := nonEmptySlice(customized); values != nil {
		output["customized"] = values
	}
	if len(output) == 0 {
		return ""
	}
	return c.setJSONAttribute(state, "langfuse.observation.output", output)
}

func (s *spanState) recordError(operation string, err error) (string, string) {
	if err == nil {
		return "", ""
	}
	if cause, cancelled := cancellationCause(s.ctx, err); cancelled {
		causeText := cause.Error()
		message := operation + ": cancelled"
		if !errors.Is(cause, context.Canceled) {
			message += ": " + causeText
		}
		s.outcomeMu.Lock()
		failed := s.failed
		s.outcomeMu.Unlock()
		if !failed {
			s.attributes.setString("langfuse.observation.status_message", message)
		}
		s.attributes.setString("langfuse.observation.metadata.eino_callback_cancelled", "true")
		s.attributes.setString("langfuse.observation.metadata.eino_cancellation_cause", causeText)
		if s.run != nil {
			s.run.recordCancellation(message, causeText)
		}
		return "cancelled", causeText
	}
	if causeText, interrupted := interruptionCause(err); interrupted {
		message := operation + ": interrupted"
		s.outcomeMu.Lock()
		failed := s.failed
		s.outcomeMu.Unlock()
		if !failed {
			s.attributes.setString("langfuse.observation.status_message", message)
		}
		s.attributes.setString("langfuse.observation.metadata.eino_callback_interrupted", "true")
		s.attributes.setString("langfuse.observation.metadata.eino_interrupt_cause", causeText)
		if s.run != nil {
			s.run.recordInterruption(message, causeText)
		}
		return "interrupted", causeText
	}
	s.outcomeMu.Lock()
	s.failed = true
	s.outcomeMu.Unlock()
	message := operation + ": " + err.Error()
	s.span.RecordError(err)
	s.span.SetStatus(codes.Error, message)
	s.attributes.setString("langfuse.observation.level", "ERROR")
	s.attributes.setString("langfuse.observation.status_message", message)
	s.attributes.setString("langfuse.observation.metadata.eino_callback_error", message)
	if s.run != nil {
		s.run.recordError(message, err)
	}
	return "", ""
}

func (s *spanState) end(output string) {
	s.endOnce.Do(func() {
		s.span.End()
		if s.run != nil {
			s.run.childEnded(output)
		}
	})
}

func newAsyncGroup() asyncGroup {
	idle := make(chan struct{})
	close(idle)
	return asyncGroup{idle: idle}
}

func (g *asyncGroup) run(fn func()) bool {
	g.mu.Lock()
	if g.closing {
		g.mu.Unlock()
		return false
	}
	if g.active == 0 {
		g.idle = make(chan struct{})
	}
	g.active++
	g.mu.Unlock()
	go func() {
		defer g.done()
		defer func() {
			if recovered := recover(); recovered != nil {
				recoveredPanic("async callback", recovered)
			}
		}()
		fn()
	}()
	return true
}

func (g *asyncGroup) done() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.active--
	if g.active == 0 {
		close(g.idle)
	}
}

func (g *asyncGroup) wait(ctx context.Context, closeGroup bool) error {
	g.mu.Lock()
	if closeGroup {
		g.closing = true
	}
	idle := g.idle
	g.mu.Unlock()
	select {
	case <-idle:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func spanStateFromContext(ctx context.Context) *spanState {
	state, _ := ctx.Value(spanStateKey{}).(*spanState)
	return state
}

func observationType(info *callbacks.RunInfo) ObservationType {
	switch info.Component {
	case adk.ComponentOfAgent, adk.ComponentOfAgenticAgent:
		return ObservationTypeAgent
	case components.ComponentOfChatModel, components.ComponentOfAgenticModel:
		return ObservationTypeGeneration
	case components.ComponentOfEmbedding:
		return ObservationTypeEmbedding
	case components.ComponentOfRetriever:
		return ObservationTypeRetriever
	case components.ComponentOfTool:
		return ObservationTypeTool
	case components.ComponentOfPrompt, components.ComponentOfAgenticPrompt:
		return ObservationTypeChain
	default:
		return ObservationTypeSpan
	}
}

func isGeneration(info *callbacks.RunInfo) bool {
	return info.Component == components.ComponentOfChatModel || info.Component == components.ComponentOfAgenticModel
}

func callbackName(info *callbacks.RunInfo) string {
	if info.Name != "" {
		return info.Name
	}
	if info.Type != "" {
		return info.Type + string(info.Component)
	}
	if info.Component != "" {
		return string(info.Component)
	}
	return "eino-operation"
}

func newValuePreparer(mask func(string) string, maxLength int, losses *lossReporter) func(string) string {
	return func(value string) string {
		wasJSON := json.Valid([]byte(value))
		if mask != nil {
			value = mask(value)
		}
		if maxLength < 0 || len(value) <= maxLength {
			return value
		}
		losses.Add(lossValueLimit, 1)
		if wasJSON || json.Valid([]byte(value)) {
			return truncateJSON(value, maxLength)
		}
		return truncateText(value, maxLength)
	}
}

func newSpanAttributeWriter(span trace.Span, prepare func(string) string, remaining int, losses *lossReporter) *spanAttributeWriter {
	return &spanAttributeWriter{span: span, prepare: prepare, remaining: remaining, losses: losses}
}

func (w *spanAttributeWriter) setString(key, value string) string {
	if value == "" {
		return ""
	}
	prepared := w.prepare(value)
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.remaining >= 0 {
		available := w.remaining - len(key)
		if available <= 0 {
			w.losses.Add(lossSpanAttributeBudget, 1)
			return ""
		}
		if len(prepared) > available {
			w.losses.Add(lossSpanAttributeBudget, 1)
			if json.Valid([]byte(prepared)) {
				prepared = truncateJSON(prepared, available)
			} else {
				prepared = truncateText(prepared, available)
			}
		}
		w.remaining -= len(key) + len(prepared)
	}
	if prepared != "" {
		w.span.SetAttributes(attribute.String(key, prepared))
	}
	return prepared
}

func (w *spanAttributeWriter) setInt(key string, value int) {
	if value <= 0 {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.remaining >= 0 {
		size := len(key) + len(strconv.Itoa(value))
		if size > w.remaining {
			w.losses.Add(lossSpanAttributeBudget, 1)
			return
		}
		w.remaining -= size
	}
	w.span.SetAttributes(attribute.Int(key, value))
}

func fitAttributes(attrs []attribute.KeyValue, limit int, losses *lossReporter) ([]attribute.KeyValue, int) {
	if limit < 0 {
		return attrs, -1
	}
	remaining := limit
	result := make([]attribute.KeyValue, 0, len(attrs))
	for _, attr := range attrs {
		size := len(attr.Key) + attributeValueSize(attr.Value)
		if size > remaining {
			if attr.Value.Type() != attribute.STRING || remaining <= len(attr.Key) {
				losses.Add(lossSpanAttributeBudget, 1)
				continue
			}
			losses.Add(lossSpanAttributeBudget, 1)
			value := attr.Value.AsString()
			available := remaining - len(attr.Key)
			if json.Valid([]byte(value)) {
				value = truncateJSON(value, available)
			} else {
				value = truncateText(value, available)
			}
			attr = attribute.String(string(attr.Key), value)
			size = len(attr.Key) + len(value)
		}
		result = append(result, attr)
		remaining -= size
	}
	return result, remaining
}

func attributeValueSize(value attribute.Value) int {
	switch value.Type() {
	case attribute.STRING:
		return len(value.AsString())
	case attribute.STRINGSLICE:
		size := 0
		for _, item := range value.AsStringSlice() {
			size += len(item)
		}
		return size
	default:
		return len(value.Emit())
	}
}

func truncateJSON(value string, maxLength int) string {
	if maxLength < 2 {
		return ""
	}
	originalLength := len(value)
	previewLength := maxLength
	for previewLength >= 0 {
		preview := truncateText(value, previewLength)
		encoded, err := json.Marshal(map[string]any{
			"truncated":      true,
			"original_bytes": originalLength,
			"preview":        preview,
		})
		if err == nil && len(encoded) <= maxLength {
			return string(encoded)
		}
		if previewLength == 0 {
			break
		}
		previewLength /= 2
	}
	return "{}"
}

func truncateText(value string, maxLength int) string {
	if maxLength <= 0 {
		return ""
	}
	if len(value) <= maxLength {
		return value
	}
	const marker = "...[truncated]..."
	if maxLength <= len(marker) {
		return utf8Prefix(value, maxLength)
	}
	available := maxLength - len(marker)
	headLength := available * 7 / 10
	tailLength := available - headLength
	return utf8Prefix(value, headLength) + marker + utf8Suffix(value, tailLength)
}

func utf8Prefix(value string, maxLength int) string {
	if len(value) <= maxLength {
		return value
	}
	value = value[:maxLength]
	for !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value
}

func utf8Suffix(value string, maxLength int) string {
	if len(value) <= maxLength {
		return value
	}
	value = value[len(value)-maxLength:]
	for !utf8.ValidString(value) {
		value = value[1:]
	}
	return value
}

func errorOutput(message string) string {
	encoded, _ := json.Marshal(map[string]string{"error": message})
	return string(encoded)
}

func cancelledOutput(cause string) string {
	encoded, _ := json.Marshal(map[string]string{"status": "cancelled", "cause": cause})
	return string(encoded)
}

func interruptedOutput(cause string) string {
	encoded, _ := json.Marshal(map[string]string{"status": "interrupted", "cause": cause})
	return string(encoded)
}

func interruptionCause(err error) (string, bool) {
	if err == nil {
		return "", false
	}
	if _, interrupted := compose.IsInterruptRerunError(err); interrupted {
		return err.Error(), true
	}
	if _, interrupted := compose.ExtractInterruptInfo(err); interrupted {
		return err.Error(), true
	}
	var adkInterrupt *adk.InterruptError
	if errors.As(err, &adkInterrupt) {
		return err.Error(), true
	}
	return "", false
}

func cancellationCause(ctx context.Context, err error) (error, bool) {
	if errors.Is(err, context.Canceled) {
		if ctx != nil {
			if cause := context.Cause(ctx); cause != nil {
				return cause, true
			}
		}
		return context.Canceled, true
	}
	if ctx == nil || !errors.Is(ctx.Err(), context.Canceled) {
		return nil, false
	}
	cause := context.Cause(ctx)
	if cause != nil && errors.Is(err, cause) {
		return cause, true
	}
	return nil, false
}

func collapseValues[T any](values []T) any {
	if len(values) == 1 {
		return values[0]
	}
	return values
}

func nonEmptySlice(value any) any {
	reflected := reflect.ValueOf(value)
	if reflected.Kind() != reflect.Slice || reflected.Len() == 0 {
		return nil
	}
	if reflected.Len() == 1 {
		return reflected.Index(0).Interface()
	}
	return value
}

func receiveAll[T any](reader *schema.StreamReader[T]) ([]T, error) {
	values, _, err := receiveAllWithFirst(reader)
	return values, err
}

func receiveAllWithFirst[T any](reader *schema.StreamReader[T]) ([]T, time.Time, error) {
	if reader == nil {
		return nil, time.Time{}, errors.New("stream is nil")
	}
	var values []T
	var first time.Time
	for {
		value, err := reader.Recv()
		if errors.Is(err, io.EOF) {
			return values, first, nil
		}
		if err != nil {
			return values, first, err
		}
		if first.IsZero() {
			first = time.Now()
		}
		values = append(values, value)
	}
}

func closeStream[T any](reader *schema.StreamReader[T]) {
	if reader != nil {
		reader.Close()
	}
}

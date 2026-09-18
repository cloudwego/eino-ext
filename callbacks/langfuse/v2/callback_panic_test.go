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
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type panicJSONOutput struct{}

func (panicJSONOutput) MarshalJSON() ([]byte, error) { panic("test serialization panic") }

type panicEndSpan struct{ trace.Span }

func (panicEndSpan) End(...trace.SpanEndOption) { panic("test span end panic") }

func TestCallbackPanicFinalizesTerminalCollectors(t *testing.T) {
	for _, kind := range []string{"stream", "agent", "agentic", "error", "diagnostic"} {
		t.Run(kind, func(t *testing.T) {
			h, exporter := newScopeHandler(t, true)
			ctx := h.StartTrace(context.Background(), WithName("root"))
			info := &callbacks.RunInfo{Name: "child", Component: compose.ComponentOfChain}
			if kind == "agent" {
				info.Component = adk.ComponentOfAgent
			}
			if kind == "agentic" {
				info.Component = adk.ComponentOfAgenticAgent
			}
			childCtx := h.OnStart(ctx, info, nil)
			state := spanStateFromContext(childCtx)
			switch kind {
			case "agent":
				events, sender := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
				sender.Send(&adk.AgentEvent{Output: &adk.AgentOutput{CustomizedOutput: panicJSONOutput{}}})
				sender.Close()
				h.OnEnd(childCtx, info, &adk.AgentCallbackOutput{Events: events})
			case "agentic":
				events, sender := adk.NewAsyncIteratorPair[*adk.TypedAgentEvent[*schema.AgenticMessage]]()
				sender.Send(&adk.TypedAgentEvent[*schema.AgenticMessage]{Output: &adk.TypedAgentOutput[*schema.AgenticMessage]{CustomizedOutput: panicJSONOutput{}}})
				sender.Close()
				h.OnEnd(childCtx, info, &adk.TypedAgentCallbackOutput[*schema.AgenticMessage]{Events: events})
			case "error":
				original := state.attributes.prepare
				state.attributes.prepare = func(value string) string {
					if strings.HasPrefix(value, "{") {
						panic("test error output panic")
					}
					return original(value)
				}
				h.OnError(childCtx, info, errors.New("business failure"))
			default:
				if kind == "diagnostic" {
					state.attributes.prepare = func(string) string { panic("test diagnostic panic") }
				}
				r, w := schema.Pipe[callbacks.CallbackOutput](1)
				w.Send(panicJSONOutput{}, nil)
				w.Close()
				h.OnEndWithStreamOutput(childCtx, info, r)
			}
			h.EndTrace(ctx, "application result")
			flushScope(t, h)
			assertPanicRootEnded(t, ctx)
			if len(exporter.GetSpans()) != 2 {
				t.Fatalf("exported spans=%d", len(exporter.GetSpans()))
			}
			if kind == "diagnostic" {
				if spanByName(t, exporter.GetSpans(), "child").Status.Code != codes.Error {
					t.Fatal("missing OTel error status")
				}
			} else {
				assertScopeError(t, exporter, "child", true)
			}
			assertScopeError(t, exporter, "root", kind == "error")
			root := spanByName(t, exporter.GetSpans(), "root")
			rootAttrs := attributesByKey(root.Attributes)
			assertStringAttribute(t, rootAttrs, "langfuse.observation.output", "application result")
			if rootAttrs["langfuse.observation.metadata.eino_callback_panic"].Value.AsString() == "" {
				t.Fatal("missing root panic diagnostic")
			}
			child := spanByName(t, exporter.GetSpans(), "child")
			if kind != "diagnostic" && attributesByKey(child.Attributes)["langfuse.observation.metadata.eino_callback_panic"].Value.AsString() == "" {
				t.Fatal("missing child diagnostic")
			}
			if len(child.Events) == 0 {
				t.Fatal("missing panic exception")
			}
			state.end("must not replace final output")
			if len(exporter.GetSpans()) != 2 {
				t.Fatal("span ended more than once")
			}
		})
	}
}

func TestCallbackPanicInputWaitsForFinalOutput(t *testing.T) {
	h, exporter := newScopeHandler(t, true)
	ctx := h.StartTrace(context.Background(), WithName("root"))
	info := &callbacks.RunInfo{Name: "child", Component: compose.ComponentOfChain}
	input, w := schema.Pipe[callbacks.CallbackInput](1)
	w.Send(panicJSONOutput{}, nil)
	w.Close()
	childCtx := h.OnStartWithStreamInput(ctx, info, input)
	state := spanStateFromContext(childCtx)
	select {
	case <-state.inputDone:
	case <-time.After(time.Second):
		t.Fatal("input completion blocked")
	}
	if !state.span.IsRecording() {
		t.Fatal("input failure prematurely ended the span")
	}
	if len(exporter.GetSpans()) != 0 {
		t.Fatal("input failure exported incomplete output")
	}
	h.OnEnd(childCtx, info, "done")
	h.EndTrace(ctx, "")
	flushScope(t, h)
	assertPanicRootEnded(t, ctx)
	assertScopeError(t, exporter, "child", true)
	assertScopeError(t, exporter, "root", false)
	for _, name := range []string{"child", "root"} {
		span := spanByName(t, exporter.GetSpans(), name)
		assertStringAttribute(t, attributesByKey(span.Attributes), "langfuse.observation.output", `"done"`)
	}
}

func TestCallbackPanicPreservesAlreadyRecordedOutput(t *testing.T) {
	h, exporter := newScopeHandler(t, true)
	ctx := h.StartTrace(context.Background(), WithName("root"))
	info := &callbacks.RunInfo{Name: "agent", Component: adk.ComponentOfAgent}
	childCtx := h.OnStart(ctx, info, nil)
	state := spanStateFromContext(childCtx)
	original := state.attributes.prepare
	state.attributes.prepare = func(value string) string {
		if strings.HasPrefix(value, "[") {
			panic("test retry metadata panic")
		}
		return original(value)
	}
	events, sender := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	sender.Send(&adk.AgentEvent{Err: &adk.WillRetryError{ErrStr: "retry", RetryAttempt: 1}})
	sender.Send(&adk.AgentEvent{Output: &adk.AgentOutput{MessageOutput: &adk.MessageVariant{Message: schema.AssistantMessage("done", nil)}}})
	sender.Close()
	h.OnEnd(childCtx, info, &adk.AgentCallbackOutput{Events: events})
	h.EndTrace(ctx, "")
	flushScope(t, h)
	assertPanicRootEnded(t, ctx)
	assertScopeError(t, exporter, "agent", true)
	assertScopeError(t, exporter, "root", false)
	for _, name := range []string{"agent", "root"} {
		span := spanByName(t, exporter.GetSpans(), name)
		assertStringAttribute(t, attributesByKey(span.Attributes), "langfuse.observation.output", `{"messages":{"role":"assistant","content":"done"}}`)
	}
}

func TestCallbackPanicSpanEndStillReleasesRoot(t *testing.T) {
	h, exporter := newScopeHandler(t, true)
	ctx := h.StartTrace(context.Background(), WithName("root"))
	info := &callbacks.RunInfo{Name: "child", Component: compose.ComponentOfChain}
	childCtx := h.OnStart(ctx, info, nil)
	state := spanStateFromContext(childCtx)
	actualSpan := state.span
	state.span = panicEndSpan{Span: actualSpan}
	r, w := schema.Pipe[callbacks.CallbackOutput](1)
	w.Send("done", nil)
	w.Close()
	h.OnEndWithStreamOutput(childCtx, info, r)
	h.EndTrace(ctx, "result")
	flushScope(t, h)
	assertPanicRootEnded(t, ctx)
	spanByName(t, exporter.GetSpans(), "root")
	actualSpan.End()
}

func TestCallbackPanicClosesOwnedStream(t *testing.T) {
	h, _ := newScopeHandler(t, true)
	ctx := h.StartTrace(context.Background(), WithName("root"))
	info := &callbacks.RunInfo{Name: "child", Component: compose.ComponentOfChain}
	childCtx := h.OnStart(ctx, info, nil)
	r, w := schema.Pipe[callbacks.CallbackOutput](1)
	defer w.Close()
	w.Send("first", nil)
	converted := schema.StreamReaderWithConvert(r, func(v callbacks.CallbackOutput) (callbacks.CallbackOutput, error) { panic("stream conversion panic") })
	h.OnEndWithStreamOutput(childCtx, info, converted)
	h.EndTrace(ctx, "")
	flushScope(t, h)
	if !w.Send("after panic", nil) {
		t.Fatal("callback-owned stream was not closed")
	}
	assertPanicRootEnded(t, ctx)
}

func TestCallbackPanicRootEndReleasesWaitersAndRegistry(t *testing.T) {
	h, _ := newScopeHandler(t, true)
	ctx := h.StartTrace(context.Background(), WithName("root"))
	run := ctx.Value(traceRunKey{}).(*traceRun)
	actualSpan := run.span
	run.span = panicEndSpan{Span: actualSpan}
	info := &callbacks.RunInfo{Name: "child", Component: compose.ComponentOfChain}
	childCtx := h.OnStart(ctx, info, nil)
	r, w := schema.Pipe[callbacks.CallbackOutput](1)
	w.Send("done", nil)
	w.Close()
	h.EndTrace(ctx, "result")
	h.OnEndWithStreamOutput(childCtx, info, r)
	flushScope(t, h)
	assertPanicRootEnded(t, ctx)
	if _, ok := h.activeTraceRun.runs.Load(run); ok {
		t.Fatal("root remains in the registry")
	}
	actualSpan.End()
}

func assertPanicRootEnded(t *testing.T, ctx context.Context) {
	t.Helper()
	run := ctx.Value(traceRunKey{}).(*traceRun)
	run.mu.Lock()
	active, ended := run.active, run.ended
	run.mu.Unlock()
	if active != 0 || !ended {
		t.Fatalf("active=%d ended=%v", active, ended)
	}
	select {
	case <-run.done:
	default:
		t.Fatal("root completion channel not closed")
	}
}

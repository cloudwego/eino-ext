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
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// scopeModel emits the same model callbacks as a streaming provider. The ADK
// retry/failover wrappers and agent callbacks are real, including both the raw
// failed generation and the WillRetryError received by the agent collector.
type scopeModel struct {
	name     string
	failures int32
	calls    atomic.Int32
}

func (m *scopeModel) IsCallbacksEnabled() bool                                         { return true }
func (m *scopeModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) { return m, nil }
func (m *scopeModel) Generate(context.Context, []*schema.Message, ...model.Option) (*schema.Message, error) {
	return nil, errors.New("test requires streaming")
}
func (m *scopeModel) Stream(ctx context.Context, input []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	ctx = callbacks.EnsureRunInfo(ctx, m.name, components.ComponentOfChatModel)
	ctx = callbacks.OnStart(ctx, &model.CallbackInput{Messages: input})
	r, w := schema.Pipe[*model.CallbackOutput](2)
	w.Send(&model.CallbackOutput{Message: schema.AssistantMessage("partial", nil)}, nil)
	if m.calls.Add(1) <= m.failures {
		w.Send(nil, errors.New("provider stream failed"))
	} else {
		w.Send(&model.CallbackOutput{Message: &schema.Message{Role: schema.Assistant, Content: " done", ResponseMeta: &schema.ResponseMeta{FinishReason: "stop"}}}, nil)
	}
	w.Close()
	_, r = callbacks.OnEndWithStreamOutput(ctx, r)
	return schema.StreamReaderWithConvert(r, func(out *model.CallbackOutput) (*schema.Message, error) {
		// ADK assigns message IDs to its stream. Keep the provider's callback
		// snapshot separate so the test does not share mutable message structs.
		msg := *out.Message
		return &msg, nil
	}), nil
}

func TestAgentErrorScopeADKRecovery(t *testing.T) {
	for _, collapse := range []bool{false, true} {
		for _, mode := range []string{"retry", "failover", "exhausted"} {
			t.Run(fmt.Sprintf("%s/collapse=%v", mode, collapse), func(t *testing.T) {
				h, exporter := newScopeHandler(t, collapse)
				ctx := h.StartTrace(context.Background(), WithName("root"))
				primary := &scopeModel{name: "primary", failures: 1}
				backup := &scopeModel{name: "backup"}
				cfg := &adk.ChatModelAgentConfig{Name: "writer", Model: primary, ModelRetryConfig: &adk.ModelRetryConfig{
					MaxRetries:  1,
					BackoffFunc: func(context.Context, int) time.Duration { return time.Nanosecond },
					ShouldRetry: func(_ context.Context, r *adk.RetryContext) *adk.RetryDecision {
						return &adk.RetryDecision{Retry: r.Err != nil}
					},
				}}
				if mode == "exhausted" {
					primary.failures = 2
				}
				if mode == "failover" {
					cfg.ModelRetryConfig.MaxRetries = 0
					cfg.ModelFailoverConfig = &adk.ModelFailoverConfig[*schema.Message]{
						MaxRetries:     1,
						ShouldFailover: func(_ context.Context, _ *schema.Message, err error) bool { return err != nil },
						GetFailoverModel: func(context.Context, *adk.FailoverContext[*schema.Message]) (model.BaseChatModel, []*schema.Message, error) {
							return backup, nil, nil
						},
					}
				}
				agent, err := adk.NewChatModelAgent(ctx, cfg)
				if err != nil {
					t.Fatal(err)
				}
				iter := adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent, EnableStreaming: true}).Run(ctx, []*schema.Message{schema.UserMessage("write")}, adk.WithCallbacks(h))
				var terminal error
				var final string
				for {
					event, ok := iter.Next()
					if !ok {
						break
					}
					if event.Err != nil {
						terminal = event.Err
					}
					if event.Output != nil && event.Output.MessageOutput != nil {
						msg, err := event.Output.MessageOutput.GetMessage()
						if err == nil && msg != nil {
							final = msg.Content
						}
					}
				}
				h.EndTrace(ctx, "")
				flushScope(t, h)
				wantFailure := mode == "exhausted"
				if (terminal != nil) != wantFailure {
					t.Fatalf("terminal=%v, want failure=%v", terminal, wantFailure)
				}
				if !wantFailure && final != "partial done" {
					t.Fatalf("final=%q", final)
				}
				assertScopeError(t, exporter, "root", wantFailure)
				assertScopeError(t, exporter, "writer", wantFailure)
				failedGenerations := 0
				for _, span := range exporter.GetSpans() {
					attrs := attributesByKey(span.Attributes)
					if attrs["langfuse.observation.type"].Value.AsString() == "generation" && span.Status.Code == codes.Error {
						failedGenerations++
					}
				}
				wantFailedGenerations := 1
				if wantFailure {
					wantFailedGenerations = 2
				}
				if failedGenerations != wantFailedGenerations {
					t.Fatalf("failed generations=%d, want %d", failedGenerations, wantFailedGenerations)
				}
				if primary.calls.Load()+backup.calls.Load() != 2 {
					t.Fatal("expected two real model attempts")
				}
			})
		}
	}
}

func TestAgentRetryDiagnosticsForRejectedResponses(t *testing.T) {
	for _, exhausted := range []bool{false, true} {
		for _, reason := range []any{"empty content", errors.New("invalid finish reason"), map[string]any{"code": "invalid_output"}, make(chan int)} {
			t.Run(fmt.Sprintf("exhausted=%v/reason=%T", exhausted, reason), func(t *testing.T) {
				h, exporter := newScopeHandler(t, true)
				ctx := h.StartTrace(context.Background(), WithName("root"))
				m := &scopeModel{name: "provider"}
				agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{Name: "writer", Model: m, ModelRetryConfig: &adk.ModelRetryConfig{
					MaxRetries: 1, BackoffFunc: func(context.Context, int) time.Duration { return time.Nanosecond },
					ShouldRetry: func(_ context.Context, r *adk.RetryContext) *adk.RetryDecision {
						if exhausted || m.calls.Load() == 1 {
							return &adk.RetryDecision{Retry: true, RejectReason: reason}
						}
						return nil
					},
				}})
				if err != nil {
					t.Fatal(err)
				}
				it := adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent, EnableStreaming: true}).Run(ctx, []*schema.Message{schema.UserMessage("test")}, adk.WithCallbacks(h))
				var terminal error
				for {
					event, ok := it.Next()
					if !ok {
						break
					}
					if event.Err != nil {
						terminal = event.Err
					}
					if event.Output != nil && event.Output.MessageOutput != nil {
						_, _ = event.Output.MessageOutput.GetMessage()
					}
				}
				h.EndTrace(ctx, "")
				flushScope(t, h)
				if (terminal != nil) != exhausted || m.calls.Load() != 2 {
					t.Fatalf("terminal=%v calls=%d", terminal, m.calls.Load())
				}
				assertScopeError(t, exporter, "writer", exhausted)
				assertScopeError(t, exporter, "root", exhausted)
				attrs := attributesByKey(spanByName(t, exporter.GetSpans(), "writer").Attributes)
				var events []agentRetryEvent
				if err := json.Unmarshal([]byte(attrs["langfuse.observation.metadata.eino_retry_events"].Value.AsString()), &events); err != nil {
					t.Fatal(err)
				}
				wantCount := 1
				if exhausted {
					wantCount = 2
				}
				if len(events) != wantCount || attrs["langfuse.observation.metadata.eino_retry_event_count"].Value.AsInt64() != int64(wantCount) {
					t.Fatalf("retry events=%+v", events)
				}
				wantReason := reason
				if cause, ok := reason.(error); ok {
					wantReason = cause.Error()
				}
				encodedReason, err := json.Marshal(wantReason)
				if err != nil {
					encodedReason, _ = json.Marshal(fmt.Sprint(wantReason))
				}
				for index, event := range events {
					gotReason, _ := json.Marshal(event.RejectReason)
					if string(gotReason) != string(encodedReason) || event.Error == "" || event.Operation != "agent message" || event.Attempt != index {
						t.Fatalf("event=%+v reason=%s want=%s", event, gotReason, encodedReason)
					}
				}
				for _, span := range exporter.GetSpans() {
					if attributesByKey(span.Attributes)["langfuse.observation.type"].Value.AsString() == "generation" && span.Status.Code == codes.Error {
						t.Fatal("provider succeeded; rejection belongs to the retry policy")
					}
				}
			})
		}
	}
}

func TestAgentErrorScopeEventForms(t *testing.T) {
	for _, typed := range []bool{false, true} {
		for _, terminal := range []error{nil, errors.New("budget limit"), adk.ErrExceedMaxIterations} {
			t.Run(fmt.Sprintf("typed=%v/terminal=%v", typed, terminal), func(t *testing.T) {
				h, exporter := newScopeHandler(t, false)
				ctx := h.StartTrace(context.Background(), WithName("root"))
				component := adk.ComponentOfAgent
				if typed {
					component = adk.ComponentOfAgenticAgent
				}
				info := &callbacks.RunInfo{Name: "agent", Component: component}
				aCtx := h.OnStart(ctx, info, nil)
				retryErr := fmt.Errorf("wrapped: %w", &adk.WillRetryError{ErrStr: "retry", RetryAttempt: 1})
				if typed {
					events, sender := adk.NewAsyncIteratorPair[*adk.TypedAgentEvent[*schema.AgenticMessage]]()
					h.OnEnd(aCtx, info, &adk.TypedAgentCallbackOutput[*schema.AgenticMessage]{Events: events})
					sender.Send(&adk.TypedAgentEvent[*schema.AgenticMessage]{Err: retryErr})
					r, w := schema.Pipe[*schema.AgenticMessage](1)
					w.Send(nil, retryErr)
					w.Close()
					sender.Send(&adk.TypedAgentEvent[*schema.AgenticMessage]{Output: &adk.TypedAgentOutput[*schema.AgenticMessage]{MessageOutput: &adk.TypedMessageVariant[*schema.AgenticMessage]{IsStreaming: true, MessageStream: r}}})
					if terminal != nil {
						sender.Send(&adk.TypedAgentEvent[*schema.AgenticMessage]{Err: terminal})
					}
					sender.Close()
				} else {
					events, sender := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
					h.OnEnd(aCtx, info, &adk.AgentCallbackOutput{Events: events})
					sender.Send(&adk.AgentEvent{Err: retryErr})
					r, w := schema.Pipe[*schema.Message](1)
					w.Send(nil, retryErr)
					w.Close()
					sender.Send(&adk.AgentEvent{Output: &adk.AgentOutput{MessageOutput: &adk.MessageVariant{IsStreaming: true, MessageStream: r}}})
					if terminal != nil {
						sender.Send(&adk.AgentEvent{Err: terminal})
					}
					sender.Close()
				}
				h.EndTrace(ctx, "")
				flushScope(t, h)
				assertScopeError(t, exporter, "root", terminal != nil)
				assertScopeError(t, exporter, "agent", terminal != nil)
				attrs := attributesByKey(spanByName(t, exporter.GetSpans(), "agent").Attributes)
				var retries []agentRetryEvent
				if err := json.Unmarshal([]byte(attrs["langfuse.observation.metadata.eino_retry_events"].Value.AsString()), &retries); err != nil {
					t.Fatal(err)
				}
				if len(retries) != 2 || retries[0].Operation != "agent output" || retries[1].Operation != "agent message" || retries[0].Attempt != 1 || retries[1].Attempt != 1 {
					t.Fatalf("retry events=%+v", retries)
				}
			})
		}
	}
}

func TestAgentErrorScopeNestedAndStandalone(t *testing.T) {
	for _, collapse := range []bool{false, true} {
		t.Run(fmt.Sprint(collapse), func(t *testing.T) {
			h, exporter := newScopeHandler(t, collapse)
			ctx := h.StartTrace(context.Background(), WithName("root"))
			outer := &callbacks.RunInfo{Name: "outer", Component: adk.ComponentOfAgent}
			aCtx := h.OnStart(ctx, outer, nil)
			child := &callbacks.RunInfo{Name: "child", Component: adk.ComponentOfAgent}
			childCtx := h.OnStart(aCtx, child, nil)
			h.OnError(childCtx, child, errors.New("child failed, handled by outer"))
			tool := &callbacks.RunInfo{Name: "tool", Component: components.ComponentOfTool}
			toolCtx := h.OnStart(aCtx, tool, nil)
			h.OnError(toolCtx, tool, errors.New("tool failed, handled by outer"))
			// A new root nested in an agent context is independent of that agent.
			independent := h.StartTrace(aCtx, WithName("independent-root"))
			modelInfo := &callbacks.RunInfo{Name: "standalone-model", Component: components.ComponentOfChatModel}
			modelCtx := h.OnStart(independent, modelInfo, &model.CallbackInput{})
			h.OnError(modelCtx, modelInfo, errors.New("standalone failure"))
			h.EndTrace(independent, "")
			h.OnEnd(aCtx, outer, "recovered")
			h.EndTrace(ctx, "recovered")
			flushScope(t, h)
			for _, name := range []string{"root", "outer"} {
				assertScopeError(t, exporter, name, false)
			}
			for _, name := range []string{"child", "tool", "independent-root", "standalone-model"} {
				assertScopeError(t, exporter, name, true)
			}
		})
	}
}

func TestAgentErrorScopeLateChildAndCancellation(t *testing.T) {
	for _, terminal := range []bool{false, true} {
		t.Run(fmt.Sprint(terminal), func(t *testing.T) {
			h, exporter := newScopeHandler(t, false)
			base, cancel := context.WithCancelCause(context.Background())
			defer cancel(nil)
			ctx := h.StartTrace(base, WithName("root"))
			info := &callbacks.RunInfo{Name: "agent", Component: adk.ComponentOfAgent}
			aCtx := h.OnStart(ctx, info, nil)
			modelInfo := &callbacks.RunInfo{Name: "late-model", Component: components.ComponentOfChatModel}
			modelCtx := h.OnStart(aCtx, modelInfo, &model.CallbackInput{})
			r, w := schema.Pipe[callbacks.CallbackOutput](1)
			h.OnEndWithStreamOutput(modelCtx, modelInfo, r)
			if terminal {
				h.OnError(aCtx, info, errors.New("terminal failure"))
			} else {
				h.OnEnd(aCtx, info, "done")
			}
			// Complete the agent collector before cancellation, while the model
			// collector remains blocked. A late child must never change its result.
			state := spanStateFromContext(aCtx)
			deadline := time.After(time.Second)
			for state.span.IsRecording() {
				select {
				case <-deadline:
					t.Fatal("agent callback did not finish")
				case <-time.After(time.Millisecond):
				}
			}
			cancel(errors.New("generation stopped by user"))
			h.EndTrace(ctx, "done")
			w.Send(nil, errors.New("late stream error"))
			w.Close()
			flushScope(t, h)
			assertScopeError(t, exporter, "root", terminal)
			assertScopeError(t, exporter, "late-model", true)
		})
	}
}

func TestAgentErrorScopeInterruptedToolKeepsRootStatus(t *testing.T) {
	h, exporter := newScopeHandler(t, true)
	ctx := h.StartTrace(context.Background(), WithName("root"))
	info := &callbacks.RunInfo{Name: "agent", Component: adk.ComponentOfAgent}
	aCtx := h.OnStart(ctx, info, nil)
	tool := &callbacks.RunInfo{Name: "chapter", Component: components.ComponentOfTool}
	toolCtx := h.OnStart(aCtx, tool, nil)
	h.OnError(toolCtx, tool, compose.Interrupt(toolCtx, "awaiting confirmation"))
	h.OnEnd(aCtx, info, nil)
	h.EndTrace(ctx, "")
	flushScope(t, h)
	assertScopeError(t, exporter, "root", false)
	attrs := attributesByKey(spanByName(t, exporter.GetSpans(), "root").Attributes)
	assertStringAttribute(t, attrs, "langfuse.observation.metadata.eino_callback_interrupted", "true")
}

func newScopeHandler(t *testing.T, collapse bool) (*CallbackHandler, *tracetest.InMemoryExporter) {
	t.Helper()
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	h, err := NewHandler(context.Background(), &Config{TracerProvider: provider, CollapseAgentInternalSpans: collapse})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { h.Shutdown(context.Background()); provider.Shutdown(context.Background()) })
	return h, exporter
}

func flushScope(t *testing.T, h *CallbackHandler) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := h.Flush(ctx); err != nil {
		t.Fatal(err)
	}
}

func assertScopeError(t *testing.T, exporter *tracetest.InMemoryExporter, name string, want bool) {
	t.Helper()
	span := spanByName(t, exporter.GetSpans(), name)
	attrs := attributesByKey(span.Attributes)
	if got := span.Status.Code == codes.Error; got != want {
		t.Fatalf("%s OTel status=%v, want error=%v", name, span.Status, want)
	}
	if got := attrs["langfuse.observation.level"].Value.AsString() == "ERROR"; got != want {
		t.Fatalf("%s Langfuse level=%v, want error=%v", name, attrs["langfuse.observation.level"], want)
	}
}

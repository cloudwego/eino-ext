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
	"fmt"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func TestAgentRootOutputPriority(t *testing.T) {
	for _, component := range []components.Component{adk.ComponentOfAgent, adk.ComponentOfAgenticAgent} {
		for _, tc := range []struct {
			name, agentOutput, explicit, want string
			childFirst, cancel, noAgent       bool
		}{
			{name: "late child", agentOutput: "done", want: `"done"`},
			{name: "early child", agentOutput: "done", childFirst: true, want: `"done"`},
			{name: "explicit", agentOutput: "done", explicit: "application result", want: "application result"},
			{name: "explicit survives cancellation", agentOutput: "done", explicit: "application result", cancel: true, want: "application result"},
			{name: "context end", agentOutput: "done", cancel: true, want: `"done"`},
			{name: "empty agent output"},
			{name: "standalone fallback", noAgent: true, want: `{"role":"assistant","content":"failed partial"}`},
		} {
			t.Run(fmt.Sprintf("%s/%s", component, tc.name), func(t *testing.T) {
				h, exporter := newScopeHandler(t, true)
				base, cancel := context.WithCancel(context.Background())
				defer cancel()
				ctx := h.StartTrace(base, WithName("root"))
				info := &callbacks.RunInfo{Name: "agent", Component: component}
				agentCtx := ctx
				if !tc.noAgent {
					agentCtx = h.OnStart(ctx, info, nil)
				}
				mi := &callbacks.RunInfo{Name: "provider", Component: components.ComponentOfChatModel}
				mc := h.OnStart(agentCtx, mi, &model.CallbackInput{})
				finishChild := func() {
					r, w := schema.Pipe[callbacks.CallbackOutput](2)
					w.Send(&model.CallbackOutput{Message: schema.AssistantMessage("failed partial", nil)}, nil)
					w.Send(nil, errors.New("attempt failed"))
					w.Close()
					h.OnEndWithStreamOutput(mc, mi, r)
					flushScope(t, h)
				}
				if tc.childFirst {
					finishChild()
				}
				if !tc.noAgent {
					if tc.agentOutput == "" {
						events, sender := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
						sender.Close()
						// Exercise a genuinely empty agent result, not a JSON empty string.
						if component == adk.ComponentOfAgent {
							h.OnEnd(agentCtx, info, &adk.AgentCallbackOutput{Events: events})
						} else {
							typedEvents, typedSender := adk.NewAsyncIteratorPair[*adk.TypedAgentEvent[*schema.AgenticMessage]]()
							typedSender.Close()
							h.OnEnd(agentCtx, info, &adk.TypedAgentCallbackOutput[*schema.AgenticMessage]{Events: typedEvents})
						}
					} else {
						h.OnEnd(agentCtx, info, tc.agentOutput)
					}
					flushScope(t, h)
				}
				if !tc.cancel || tc.explicit != "" {
					h.EndTrace(ctx, tc.explicit)
				}
				if tc.cancel {
					cancel()
					// Deterministically run the same completion path as the context watcher.
					ctx.Value(traceRunKey{}).(*traceRun).contextEnded(ctx)
				}
				if !tc.childFirst {
					finishChild()
				}
				flushScope(t, h)
				root := spanByName(t, exporter.GetSpans(), "root")
				got := attributesByKey(root.Attributes)["langfuse.observation.output"].Value.AsString()
				if got != tc.want {
					t.Fatalf("root output=%q, want=%q", got, tc.want)
				}
				assertScopeError(t, exporter, "root", tc.noAgent)
				assertScopeError(t, exporter, "provider", true)
			})
		}
	}
}

func TestAgentRootOutputIgnoresNestedAgent(t *testing.T) {
	h, exporter := newScopeHandler(t, true)
	ctx := h.StartTrace(context.Background(), WithName("root"))
	outer := &callbacks.RunInfo{Name: "outer", Component: adk.ComponentOfAgent}
	outerCtx := h.OnStart(ctx, outer, nil)
	inner := &callbacks.RunInfo{Name: "inner", Component: adk.ComponentOfAgent}
	innerCtx := h.OnStart(outerCtx, inner, nil)
	h.OnEnd(outerCtx, outer, "done")
	h.EndTrace(ctx, "")
	h.OnError(innerCtx, inner, errors.New("handled child failure"))
	flushScope(t, h)
	root := spanByName(t, exporter.GetSpans(), "root")
	assertStringAttribute(t, attributesByKey(root.Attributes), "langfuse.observation.output", `"done"`)
	assertScopeError(t, exporter, "root", false)
	assertScopeError(t, exporter, "inner", true)
}

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

package tls

import (
	"context"
	"io"
	"testing"
	"time"

	sem_ai "github.com/cloudwego/eino-ext/callbacks/tls/semconv"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// lifecycleTestModel is deliberately small but exercises Eino's real
// Graph.Stream callback ordering rather than calling the TLS handler directly.
type lifecycleTestModel struct{}

func (lifecycleTestModel) BindTools([]*schema.ToolInfo) error { return nil }

func (lifecycleTestModel) Generate(context.Context, []*schema.Message, ...model.Option) (*schema.Message, error) {
	return &schema.Message{Role: schema.Assistant, Content: "stream output"}, nil
}

func (lifecycleTestModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	reader, writer := schema.Pipe[*schema.Message](1)
	go func() {
		defer writer.Close()
		writer.Send(&schema.Message{Role: schema.Assistant, Content: "stream output"}, nil)
	}()
	return reader, nil
}

// blockingStreamParser lets the graph's output parser finish while keeping the
// child model's output parser pending.  This is the ordering that previously
// allowed the root agent.turn span to end before model usage was available.
type blockingStreamParser struct {
	release          <-chan struct{}
	modelParserStart chan<- struct{}
	graphParserDone  chan<- struct{}
}

func (p *blockingStreamParser) ParseInput(context.Context, *callbacks.RunInfo, callbacks.CallbackInput) (map[string]any, error) {
	return nil, nil
}

func (p *blockingStreamParser) ParseOutput(context.Context, *callbacks.RunInfo, callbacks.CallbackOutput) (map[string]any, error) {
	return nil, nil
}

func (p *blockingStreamParser) ParseStreamInput(context.Context, *callbacks.RunInfo, *schema.StreamReader[callbacks.CallbackInput]) (map[string]any, error) {
	return nil, nil
}

func (p *blockingStreamParser) ParseStreamOutput(_ context.Context, info *callbacks.RunInfo, output *schema.StreamReader[callbacks.CallbackOutput]) (map[string]any, error) {
	defer output.Close()
	if info != nil && info.Component == components.ComponentOfChatModel {
		close(p.modelParserStart)
		<-p.release
	}

	for {
		_, err := output.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}

	if info != nil && info.Component == compose.ComponentOfGraph {
		close(p.graphParserDone)
		return nil, nil
	}
	if info == nil || info.Component != components.ComponentOfChatModel {
		return nil, nil
	}
	return map[string]any{
		sem_ai.GEN_AI_REQUEST_MODEL:       "lifecycle-test-model",
		sem_ai.GEN_AI_RESPONSE_MODEL:      "lifecycle-test-model",
		sem_ai.GEN_AI_USAGE_INPUT_TOKENS:  7,
		sem_ai.GEN_AI_USAGE_OUTPUT_TOKENS: 5,
		sem_ai.GEN_AI_USAGE_TOTAL_TOKENS:  12,
		sem_ai.GEN_AI_INPUT:               "stream input",
		sem_ai.GEN_AI_OUTPUT:              "stream output",
	}, nil
}

func TestGraphStreamWaitsForChildOutputAggregationBeforeEndingRoot(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })

	release := make(chan struct{})
	modelParserStart := make(chan struct{})
	graphParserDone := make(chan struct{})
	handler := &TLSCallbackHandler{
		Tracer:             provider.Tracer(scopeName),
		TLSExporterEnabled: true,
		dataParser: &blockingStreamParser{
			release:          release,
			modelParserStart: modelParserStart,
			graphParserDone:  graphParserDone,
		},
	}

	graph := compose.NewGraph[[]*schema.Message, *schema.Message]()
	if err := graph.AddChatModelNode("model", lifecycleTestModel{}); err != nil {
		t.Fatalf("add model: %v", err)
	}
	if err := graph.AddEdge(compose.START, "model"); err != nil {
		t.Fatalf("add start edge: %v", err)
	}
	if err := graph.AddEdge("model", compose.END); err != nil {
		t.Fatalf("add end edge: %v", err)
	}
	runner, err := graph.Compile(context.Background(), compose.WithGraphName("stream-lifecycle"))
	if err != nil {
		t.Fatalf("compile graph: %v", err)
	}

	output, err := runner.Stream(context.Background(), []*schema.Message{schema.UserMessage("stream input")}, compose.WithCallbacks(handler))
	if err != nil {
		t.Fatalf("start graph stream: %v", err)
	}
	for {
		_, err := output.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read graph stream: %v", err)
		}
	}

	select {
	case <-modelParserStart:
	case <-time.After(time.Second):
		t.Fatal("model stream parser did not start")
	}
	select {
	case <-graphParserDone:
	case <-time.After(time.Second):
		t.Fatal("graph stream parser did not finish")
	}
	for _, ended := range recorder.Ended() {
		if ended.Name() == tlsRootSpanName {
			t.Fatal("root span ended before its child stream output was aggregated")
		}
	}

	close(release)
	root := waitForEndedRootSpan(t, recorder)
	if got, ok := spanIntAttribute(root, sem_ai.GEN_AI_USAGE_TOTAL_TOKENS); !ok || got != 12 {
		t.Fatalf("root total tokens = %d (present=%v), want 12", got, ok)
	}
	if got, ok := spanStringAttribute(root, sem_ai.GEN_AI_OUTPUT); !ok || got != "stream output" {
		t.Fatalf("root output = %q (present=%v), want stream output", got, ok)
	}
}

func TestTLSRootStateWaitsForEveryStreamOutput(t *testing.T) {
	root := &tlsRootState{}
	finishFirst := root.beginStreamOutput()
	finishSecond := root.beginStreamOutput()
	waitReturned := make(chan struct{})
	go func() {
		root.waitForStreamOutputs(context.Background(), time.Second)
		close(waitReturned)
	}()

	finishFirst()
	select {
	case <-waitReturned:
		t.Fatal("root wait returned while a second stream output was pending")
	case <-time.After(20 * time.Millisecond):
	}

	finishSecond()
	select {
	case <-waitReturned:
	case <-time.After(time.Second):
		t.Fatal("root wait did not return after all stream outputs completed")
	}
}

func TestTLSRootStateStreamOutputWaitRespectsCancellationAndTimeout(t *testing.T) {
	canceledRoot := &tlsRootState{}
	finishCanceled := canceledRoot.beginStreamOutput()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	canceledRoot.waitForStreamOutputs(ctx, time.Second)
	if elapsed := time.Since(started); elapsed > 100*time.Millisecond {
		t.Fatalf("canceled stream output wait took %s", elapsed)
	}
	finishCanceled()

	timedRoot := &tlsRootState{}
	finishTimed := timedRoot.beginStreamOutput()
	started = time.Now()
	timedRoot.waitForStreamOutputs(context.Background(), 10*time.Millisecond)
	if elapsed := time.Since(started); elapsed > 200*time.Millisecond {
		t.Fatalf("timed stream output wait took %s", elapsed)
	}
	finishTimed()
}

func waitForEndedRootSpan(t *testing.T, recorder *tracetest.SpanRecorder) sdktrace.ReadOnlySpan {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		for _, ended := range recorder.Ended() {
			if ended.Name() == tlsRootSpanName {
				return ended
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("root span was not ended")
		}
		time.Sleep(time.Millisecond)
	}
}

func spanIntAttribute(span sdktrace.ReadOnlySpan, key string) (int, bool) {
	for _, attr := range span.Attributes() {
		if string(attr.Key) == key {
			return int(attr.Value.AsInt64()), true
		}
	}
	return 0, false
}

func spanStringAttribute(span sdktrace.ReadOnlySpan, key string) (string, bool) {
	for _, attr := range span.Attributes() {
		if string(attr.Key) == key {
			return attr.Value.AsString(), true
		}
	}
	return "", false
}

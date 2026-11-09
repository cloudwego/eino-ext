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
	"bytes"
	"compress/gzip"
	"context"
	"encoding/hex"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/compose"
	collectortrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"
)

func TestHandlerExportsNestedGraphParentIDs(t *testing.T) {
	var mu sync.Mutex
	var exported []*tracepb.Span
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		defer request.Body.Close()
		reader, err := gzip.NewReader(request.Body)
		if err != nil {
			return nil, err
		}
		defer reader.Close()
		body, err := io.ReadAll(reader)
		if err != nil {
			return nil, err
		}
		var batch collectortrace.ExportTraceServiceRequest
		if err := proto.Unmarshal(body, &batch); err != nil {
			return nil, err
		}
		mu.Lock()
		for _, resource := range batch.ResourceSpans {
			for _, scope := range resource.ScopeSpans {
				exported = append(exported, scope.Spans...)
			}
		}
		mu.Unlock()
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/x-protobuf"}},
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    request,
		}, nil
	})}
	handler, err := NewHandler(context.Background(), &Config{
		Host:        "https://langfuse.example",
		PublicKey:   "pk-test",
		SecretKey:   "sk-test",
		HTTPClient:  client,
		RetryConfig: &RetryConfig{Enabled: false},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := handler.Shutdown(shutdownCtx); err != nil {
			t.Error(err)
		}
	})

	identity := func(_ context.Context, input string) (string, error) {
		return input, nil
	}
	inner := compose.NewGraph[string, string]()
	outer := compose.NewGraph[string, string]()
	for _, err := range []error{
		inner.AddLambdaNode("child", compose.InvokableLambda(identity), compose.WithNodeName("child")),
		inner.AddEdge(compose.START, "child"),
		inner.AddEdge("child", compose.END),
		outer.AddGraphNode("subgraph", inner, compose.WithNodeName("subgraph")),
		outer.AddLambdaNode("sibling", compose.InvokableLambda(identity), compose.WithNodeName("sibling")),
		outer.AddEdge(compose.START, "subgraph"),
		outer.AddEdge("subgraph", "sibling"),
		outer.AddEdge("sibling", compose.END),
	} {
		if err != nil {
			t.Fatal(err)
		}
	}
	runner, err := outer.Compile(context.Background(), compose.WithGraphName("graph"))
	if err != nil {
		t.Fatal(err)
	}
	const traceID = "0123456789abcdef0123456789abcdef"
	ctx := handler.StartTrace(context.Background(), WithName("root"), WithID(traceID))
	output, err := runner.Invoke(ctx, "input", compose.WithCallbacks(handler))
	if err != nil {
		t.Fatal(err)
	}
	if output != "input" {
		t.Fatalf("graph output = %q, want input", output)
	}
	handler.EndTrace(ctx, output)
	flushCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := handler.Flush(flushCtx); err != nil {
		t.Fatal(err)
	}

	// Assert the wire hierarchy independently of batch boundaries and export order.
	parents := map[string]string{
		"root":     "",
		"graph":    "root",
		"subgraph": "graph",
		"child":    "subgraph",
		"sibling":  "graph",
	}
	mu.Lock()
	defer mu.Unlock()
	if len(exported) != len(parents) {
		t.Fatalf("exported %d spans, want %d", len(exported), len(parents))
	}
	byName := make(map[string]*tracepb.Span, len(exported))
	byID := make(map[string]string, len(exported))
	var zeroSpanID [8]byte
	for _, span := range exported {
		if _, expected := parents[span.Name]; !expected {
			t.Fatalf("unexpected span %q", span.Name)
		}
		if _, duplicate := byName[span.Name]; duplicate {
			t.Fatalf("duplicate span name %q", span.Name)
		}
		if got := hex.EncodeToString(span.TraceId); got != traceID {
			t.Fatalf("%s trace ID = %s, want %s", span.Name, got, traceID)
		}
		if len(span.SpanId) != len(zeroSpanID) || bytes.Equal(span.SpanId, zeroSpanID[:]) {
			t.Fatalf("%s has invalid span ID %x", span.Name, span.SpanId)
		}
		if previous, duplicate := byID[string(span.SpanId)]; duplicate {
			t.Fatalf("%s and %s share span ID %x", previous, span.Name, span.SpanId)
		}
		byName[span.Name] = span
		byID[string(span.SpanId)] = span.Name
	}
	for name, parentName := range parents {
		span := byName[name]
		if parentName == "" {
			if len(span.ParentSpanId) != 0 {
				t.Errorf("root unexpectedly has parent %x", span.ParentSpanId)
			}
			continue
		}
		parent := byName[parentName]
		if !bytes.Equal(span.ParentSpanId, parent.SpanId) {
			t.Errorf("%s parent = %x, want %x (%s)", name, span.ParentSpanId, parent.SpanId, parentName)
		}
	}
}

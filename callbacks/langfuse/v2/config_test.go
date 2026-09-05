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
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestHTTPExporterUsesRetryConfig(t *testing.T) {
	var attempts atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		response.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	exporter, err := newHTTPExporter(context.Background(), &Config{
		Host:      server.URL,
		PublicKey: "public",
		SecretKey: "secret",
		Timeout:   time.Second,
		RetryConfig: &RetryConfig{
			Enabled:         true,
			InitialInterval: time.Millisecond,
			MaxInterval:     time.Millisecond,
			MaxElapsedTime:  20 * time.Millisecond,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	_, span := provider.Tracer("retry-config-test").Start(context.Background(), "test")
	span.End()
	if err := exporter.ExportSpans(context.Background(), recorder.Ended()); err == nil {
		t.Fatal("export unexpectedly succeeded")
	}
	if got := attempts.Load(); got < 2 {
		t.Fatalf("export attempts = %d, want at least 2", got)
	}

	shutdownContext, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := exporter.Shutdown(shutdownContext); err != nil {
		t.Fatal(err)
	}
}

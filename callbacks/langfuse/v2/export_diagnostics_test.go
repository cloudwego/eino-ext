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
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"strings"
	"testing"
	"time"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type diagnosticTestRoundTripper struct{}

func (diagnosticTestRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	trace := httptrace.ContextClientTrace(request.Context())
	trace.GotConn(httptrace.GotConnInfo{Reused: true})
	trace.WroteRequest(httptrace.WroteRequestInfo{})
	time.Sleep(time.Millisecond)
	trace.GotFirstResponseByte()
	_, _ = io.Copy(io.Discard, request.Body)
	return &http.Response{
		StatusCode: http.StatusServiceUnavailable,
		Body:       io.NopCloser(strings.NewReader("unavailable")),
		Header:     make(http.Header),
		Request:    request,
	}, nil
}

func TestDiagnosticRoundTripperMeasuresCompressedBodyAndAttempt(t *testing.T) {
	protobuf := bytes.Repeat([]byte("trace payload"), 100)
	compressed := gzipBytes(t, protobuf)
	request, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://example.com/v1/traces", bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Encoding", "gzip")
	diagnostics := &exportDiagnostics{}
	request = request.WithContext(context.WithValue(request.Context(), exportDiagnosticsContextKey{}, diagnostics))

	transport := &diagnosticRoundTripper{base: diagnosticTestRoundTripper{}}
	response, err := transport.RoundTrip(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()

	diagnostics.mu.Lock()
	defer diagnostics.mu.Unlock()
	if diagnostics.protobufBytes != int64(len(protobuf)) {
		t.Fatalf("protobuf bytes = %d, want %d", diagnostics.protobufBytes, len(protobuf))
	}
	if diagnostics.gzipBytes != int64(len(compressed)) {
		t.Fatalf("gzip bytes = %d, want %d", diagnostics.gzipBytes, len(compressed))
	}
	if len(diagnostics.attempts) != 1 {
		t.Fatalf("attempts = %d, want 1", len(diagnostics.attempts))
	}
	attempt := diagnostics.attempts[0]
	if attempt.statusCode != http.StatusServiceUnavailable || !attempt.reused {
		t.Fatalf("attempt = %#v", attempt)
	}
	if attempt.total <= 0 || attempt.write <= 0 || attempt.waitHeaders <= 0 {
		t.Fatalf("attempt timings = %#v", attempt)
	}
}

func TestExportDiagnosticsStringOmitsTraceIDs(t *testing.T) {
	diagnostics := &exportDiagnostics{
		protobufBytes: 1024,
		gzipBytes:     256,
		attempts: []exportAttemptDiagnostics{{
			total:       20 * time.Second,
			waitHeaders: 20 * time.Second,
			reused:      true,
		}},
	}
	report := diagnostics.String()
	for _, expected := range []string{
		"protobuf_bytes=1024",
		"gzip_bytes=256",
		"attempts=1",
		"wait_headers=20s",
	} {
		if !strings.Contains(report, expected) {
			t.Fatalf("diagnostics %q does not contain %q", report, expected)
		}
	}
	if strings.Contains(report, "trace_id") {
		t.Fatalf("diagnostics unexpectedly contains a trace ID field: %q", report)
	}
}

func TestHTTPExporterAddsDiagnosticsToFinalFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	exporter, err := newHTTPExporter(context.Background(), &Config{
		Host:              server.URL,
		PublicKey:         "public",
		SecretKey:         "secret",
		Timeout:           time.Second,
		ExportDiagnostics: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	reported := make(chan error, 1)
	processor, err := newReportingBatchSpanProcessor(exporter, batchProcessorConfig{
		maxExportBatchSize: 1,
		exportDiagnostics:  true,
		handleExportError: func(err error) {
			reported <- err
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(processor))
	_, span := provider.Tracer("diagnostic-export-test").Start(context.Background(), "test")
	span.End()

	select {
	case err := <-reported:
		for _, expected := range []string{"protobuf_bytes=", "gzip_bytes=", "attempts=1", "trace_id"} {
			contains := strings.Contains(err.Error(), expected)
			if expected == "trace_id" && contains {
				t.Fatalf("export failure unexpectedly contains trace IDs: %v", err)
			}
			if expected != "trace_id" && !contains {
				t.Fatalf("export failure %q does not contain %q", err, expected)
			}
		}
	case <-time.After(time.Second):
		t.Fatal("export failure was not reported")
	}
	shutdownContext, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = provider.Shutdown(shutdownContext)
}

func gzipBytes(t *testing.T, input []byte) []byte {
	t.Helper()
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write(input); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return compressed.Bytes()
}

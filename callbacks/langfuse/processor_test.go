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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type blockingSpanExporter struct {
	started  chan struct{}
	release  chan struct{}
	start    sync.Once
	exported atomic.Int64
}

func newBlockingSpanExporter() *blockingSpanExporter {
	return &blockingSpanExporter{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
}

func (e *blockingSpanExporter) ExportSpans(_ context.Context, spans []sdktrace.ReadOnlySpan) error {
	e.start.Do(func() { close(e.started) })
	<-e.release
	e.exported.Add(int64(len(spans)))
	return nil
}

func (e *blockingSpanExporter) Shutdown(context.Context) error { return nil }

func TestReportingBatchSpanProcessorAggregatesQueueFullDrops(t *testing.T) {
	exporter := newBlockingSpanExporter()
	reports := make(chan map[string]uint64, 1)
	losses := newLossReporter(20*time.Millisecond, func(counts map[string]uint64, _ time.Duration) {
		reports <- counts
	})
	defer losses.Stop()
	processor, err := newReportingBatchSpanProcessor(exporter, batchProcessorConfig{
		maxQueueSize:       1,
		maxExportBatchSize: 1,
		batchTimeout:       time.Hour,
		losses:             losses,
	})
	if err != nil {
		t.Fatal(err)
	}
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(processor))
	tracer := provider.Tracer("queue-drop-test")

	endSpan := func() {
		_, span := tracer.Start(context.Background(), "test")
		span.End()
	}
	endSpan()
	select {
	case <-exporter.started:
	case <-time.After(time.Second):
		t.Fatal("first export did not start")
	}

	endSpan() // Occupies the only queue slot while the first export is blocked.
	for range 4 {
		endSpan()
	}

	select {
	case counts := <-reports:
		dropped := counts[lossQueueFull]
		if dropped != 4 {
			t.Fatalf("reported drops = %d, want 4", dropped)
		}
	case <-time.After(time.Second):
		t.Fatal("queue-full drops were not reported")
	}

	close(exporter.release)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := provider.Shutdown(shutdownCtx); err != nil {
		t.Fatal(err)
	}
	if exported := exporter.exported.Load(); exported != 2 {
		t.Fatalf("exported spans = %d, want 2", exported)
	}
}

type failingSpanExporter struct {
	err         error
	shutdownErr error
}

func (e failingSpanExporter) ExportSpans(context.Context, []sdktrace.ReadOnlySpan) error {
	return e.err
}

func (e failingSpanExporter) Shutdown(context.Context) error { return e.shutdownErr }

type panickingSpanExporter struct{}

func (panickingSpanExporter) ExportSpans(context.Context, []sdktrace.ReadOnlySpan) error {
	panic("export boom")
}

func (panickingSpanExporter) Shutdown(context.Context) error { return nil }

func TestReportingBatchSpanProcessorLogsFailedExportBatchSize(t *testing.T) {
	reported := make(chan error, 1)
	losses := newLossReporter(-1, func(map[string]uint64, time.Duration) {})
	defer losses.Stop()
	processor, err := newReportingBatchSpanProcessor(failingSpanExporter{err: errors.New("rejected")}, batchProcessorConfig{
		maxQueueSize:       2,
		maxExportBatchSize: 1,
		losses:             losses,
		handleExportError: func(err error) {
			reported <- err
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(processor))
	_, span := provider.Tracer("export-error-test").Start(context.Background(), "test")
	span.End()

	select {
	case err := <-reported:
		if !strings.Contains(err.Error(), "batch of 1 spans") || !strings.Contains(err.Error(), "rejected") {
			t.Fatalf("unexpected export error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("export failure was not reported")
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = provider.Shutdown(shutdownCtx)
}

func TestReportingBatchSpanProcessorLogsExporterShutdownFailure(t *testing.T) {
	reported := make(chan error, 1)
	shutdownErr := errors.New("shutdown rejected")
	losses := newLossReporter(time.Hour, func(map[string]uint64, time.Duration) {})
	defer losses.Stop()
	processor, err := newReportingBatchSpanProcessor(failingSpanExporter{shutdownErr: shutdownErr}, batchProcessorConfig{
		losses: losses,
		handleExportError: func(err error) {
			reported <- err
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := processor.Shutdown(shutdownCtx); !errors.Is(err, shutdownErr) {
		t.Fatalf("shutdown error = %v", err)
	}
	select {
	case err := <-reported:
		if !strings.Contains(err.Error(), "exporter shutdown failed") {
			t.Fatalf("unexpected shutdown log: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("exporter shutdown failure was not reported")
	}
}

func TestReportingBatchSpanProcessorRecoversExporterPanic(t *testing.T) {
	losses := newLossReporter(time.Hour, func(map[string]uint64, time.Duration) {})
	defer losses.Stop()
	processor, err := newReportingBatchSpanProcessor(panickingSpanExporter{}, batchProcessorConfig{
		maxQueueSize:       1,
		maxExportBatchSize: 1,
		losses:             losses,
	})
	if err != nil {
		t.Fatal(err)
	}
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(processor))
	_, span := provider.Tracer("panic-test").Start(context.Background(), "test")
	span.End()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err = provider.Shutdown(shutdownCtx)
	if err == nil || !strings.Contains(err.Error(), "batch span processor panic: export boom") {
		t.Fatalf("shutdown error = %v, want recovered processor panic", err)
	}
}

func TestReportingBatchSpanProcessorRejectsBatchLargerThanQueue(t *testing.T) {
	_, err := newReportingBatchSpanProcessor(newBlockingSpanExporter(), batchProcessorConfig{
		maxQueueSize:       1,
		maxExportBatchSize: 2,
	})
	if err == nil {
		t.Fatal("expected invalid batch and queue sizes to fail")
	}
}

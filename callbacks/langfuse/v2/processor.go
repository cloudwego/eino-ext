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
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

const (
	defaultMaxQueueSize       = 2048
	defaultMaxExportBatchSize = 512
	defaultBatchTimeout       = 5 * time.Second
	defaultShutdownTimeout    = 30 * time.Second
)

type batchProcessorConfig struct {
	maxQueueSize       int
	maxExportBatchSize int
	batchTimeout       time.Duration
	losses             *lossReporter
	handleExportError  func(error)
	exportDiagnostics  bool
}

type batchQueueItem struct {
	span    sdktrace.ReadOnlySpan
	ctx     context.Context
	flushed chan error
}

type reportingBatchSpanProcessor struct {
	exporter           sdktrace.SpanExporter
	queue              chan batchQueueItem
	maxExportBatchSize int
	batchTimeout       time.Duration
	losses             *lossReporter
	handleExportError  func(error)
	exportDiagnostics  bool
	stopped            atomic.Bool

	enqueueMu    sync.RWMutex
	stopCh       chan struct{}
	doneCh       chan error
	shutdownOnce sync.Once
}

func newReportingBatchSpanProcessor(exporter sdktrace.SpanExporter, cfg batchProcessorConfig) (*reportingBatchSpanProcessor, error) {
	if exporter == nil {
		return nil, errors.New("langfuse span exporter is nil")
	}
	if cfg.maxQueueSize <= 0 {
		cfg.maxQueueSize = defaultMaxQueueSize
	}
	if cfg.maxExportBatchSize <= 0 {
		cfg.maxExportBatchSize = min(defaultMaxExportBatchSize, cfg.maxQueueSize)
	}
	if cfg.maxExportBatchSize > cfg.maxQueueSize {
		return nil, fmt.Errorf(
			"langfuse max export batch size %d exceeds max queue size %d",
			cfg.maxExportBatchSize,
			cfg.maxQueueSize,
		)
	}
	if cfg.batchTimeout <= 0 {
		cfg.batchTimeout = defaultBatchTimeout
	}
	if cfg.losses == nil {
		cfg.losses = newLossReporter(defaultDropLogInterval, nil)
	}
	if cfg.handleExportError == nil {
		cfg.handleExportError = otel.Handle
	}

	processor := &reportingBatchSpanProcessor{
		exporter:           exporter,
		queue:              make(chan batchQueueItem, cfg.maxQueueSize),
		maxExportBatchSize: cfg.maxExportBatchSize,
		batchTimeout:       cfg.batchTimeout,
		losses:             cfg.losses,
		handleExportError:  cfg.handleExportError,
		exportDiagnostics:  cfg.exportDiagnostics,
		stopCh:             make(chan struct{}),
		doneCh:             make(chan error, 1),
	}
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				err := recoveredPanic("batch span processor", recovered)
				processor.stopped.Store(true)
				select {
				case processor.doneCh <- err:
				default:
				}
			}
		}()
		processor.run()
	}()
	return processor, nil
}

func (p *reportingBatchSpanProcessor) OnStart(context.Context, sdktrace.ReadWriteSpan) {}

func (p *reportingBatchSpanProcessor) OnEnd(span sdktrace.ReadOnlySpan) {
	if span == nil || !span.SpanContext().IsSampled() {
		return
	}
	p.losses.Add(lossOTelDroppedAttributes, uint64(span.DroppedAttributes()))
	p.losses.Add(lossOTelDroppedEvents, uint64(span.DroppedEvents()))
	p.losses.Add(lossOTelDroppedLinks, uint64(span.DroppedLinks()))

	p.enqueueMu.RLock()
	defer p.enqueueMu.RUnlock()
	if p.stopped.Load() {
		p.losses.Add(lossProcessorStopped, 1)
		return
	}
	select {
	case p.queue <- batchQueueItem{span: span}:
	default:
		p.losses.Add(lossQueueFull, 1)
	}
}

func (p *reportingBatchSpanProcessor) ForceFlush(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	p.enqueueMu.RLock()
	if p.stopped.Load() {
		p.enqueueMu.RUnlock()
		return nil
	}
	flushed := make(chan error, 1)
	select {
	case p.queue <- batchQueueItem{ctx: ctx, flushed: flushed}:
		p.enqueueMu.RUnlock()
	case <-ctx.Done():
		p.enqueueMu.RUnlock()
		return ctx.Err()
	}

	select {
	case err := <-flushed:
		return err
	case <-p.stopCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *reportingBatchSpanProcessor) Shutdown(ctx context.Context) error {
	var result error
	p.shutdownOnce.Do(func() {
		p.enqueueMu.Lock()
		p.stopped.Store(true)
		close(p.stopCh)
		p.enqueueMu.Unlock()

		select {
		case result = <-p.doneCh:
			shutdownErr := p.exporter.Shutdown(ctx)
			if shutdownErr != nil {
				p.handleExportError(fmt.Errorf("langfuse callback exporter shutdown failed: %w", shutdownErr))
			}
			result = errors.Join(result, shutdownErr)
		case <-ctx.Done():
			result = ctx.Err()
			p.handleExportError(fmt.Errorf(
				"langfuse callback shutdown interrupted with %d queued items and a possible in-flight batch; telemetry may be discarded if the process exits: %w",
				len(p.queue),
				ctx.Err(),
			))
			go func() {
				defer func() {
					if recovered := recover(); recovered != nil {
						recoveredPanic("deferred exporter shutdown", recovered)
					}
				}()
				p.shutdownExporterWhenDone()
			}()
		}
	})
	return result
}

func (p *reportingBatchSpanProcessor) run() {
	timer := time.NewTimer(p.batchTimeout)
	defer timer.Stop()

	batch := make([]sdktrace.ReadOnlySpan, 0, p.maxExportBatchSize)
	export := func(ctx context.Context) error {
		if len(batch) == 0 {
			resetTimer(timer, p.batchTimeout)
			return nil
		}
		batchSize := len(batch)
		diagnostics := (*exportDiagnostics)(nil)
		if p.exportDiagnostics {
			diagnostics = &exportDiagnostics{}
			ctx = context.WithValue(ctx, exportDiagnosticsContextKey{}, diagnostics)
		}
		err := p.exporter.ExportSpans(ctx, batch)
		clear(batch)
		batch = batch[:0]
		resetTimer(timer, p.batchTimeout)
		if err != nil {
			if diagnostics != nil {
				err = fmt.Errorf("%w; %s", err, diagnostics)
			}
			p.handleExportError(fmt.Errorf(
				"langfuse callback export failed for batch of %d spans; failed or rejected spans will not be retried by the processor: %w",
				batchSize,
				err,
			))
		}
		return err
	}

	process := func(item batchQueueItem) error {
		if item.span != nil {
			batch = append(batch, item.span)
			if len(batch) >= p.maxExportBatchSize {
				return export(context.Background())
			}
			return nil
		}
		if item.flushed != nil {
			if err := item.ctx.Err(); err != nil {
				item.flushed <- err
				return nil
			}
			err := export(item.ctx)
			item.flushed <- err
			return err
		}
		return nil
	}

	var finalErr error
	for {
		select {
		case item := <-p.queue:
			_ = process(item)
		case <-timer.C:
			_ = export(context.Background())
		case <-p.stopCh:
			for {
				select {
				case item := <-p.queue:
					if item.flushed != nil {
						item.flushed <- nil
						continue
					}
					finalErr = errors.Join(finalErr, process(item))
				default:
					finalErr = errors.Join(finalErr, export(context.Background()))
					p.doneCh <- finalErr
					return
				}
			}
		}
	}
}

func (p *reportingBatchSpanProcessor) shutdownExporterWhenDone() {
	<-p.doneCh
	shutdownCtx, cancel := context.WithTimeout(context.Background(), defaultShutdownTimeout)
	defer cancel()
	if err := p.exporter.Shutdown(shutdownCtx); err != nil {
		p.handleExportError(fmt.Errorf("langfuse callback exporter shutdown failed: %w", err))
	}
}

func resetTimer(timer *time.Timer, timeout time.Duration) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(timeout)
}

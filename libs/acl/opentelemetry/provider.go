/*
 * Copyright 2025 CloudWeGo Authors
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

package opentelemetry

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type OtelProvider struct {
	TracerProvider *sdktrace.TracerProvider
	MeterProvider  *metric.MeterProvider
}

func (p *OtelProvider) Shutdown(ctx context.Context) error {
	var err error

	if p.TracerProvider != nil {
		if err = p.TracerProvider.Shutdown(ctx); err != nil {
			otel.Handle(err)
		}
	}

	if p.MeterProvider != nil {
		if err = p.MeterProvider.Shutdown(ctx); err != nil {
			otel.Handle(err)
		}
	}

	return err
}

// NewOpenTelemetryProvider Initializes an otlp trace and metrics provider
func NewOpenTelemetryProvider(opts ...Option) (*OtelProvider, error) {
	var (
		tracerProvider *sdktrace.TracerProvider
		meterProvider  *metric.MeterProvider
	)

	ctx := context.TODO()

	cfg := newConfig(opts)

	if !cfg.enableTracing && !cfg.enableMetrics {
		return nil, nil
	}

	// resource
	res := newResource(cfg)

	// Tracing
	if cfg.enableTracing {
		// trace client
		var traceClientOpts []otlptracegrpc.Option
		if cfg.exportEndpoint != "" {
			traceClientOpts = append(traceClientOpts, otlptracegrpc.WithEndpoint(cfg.exportEndpoint))
		}
		if len(cfg.exportHeaders) > 0 {
			traceClientOpts = append(traceClientOpts, otlptracegrpc.WithHeaders(cfg.exportHeaders))
		}
		if cfg.exportInsecure {
			traceClientOpts = append(traceClientOpts, otlptracegrpc.WithInsecure())
		}

		// trace provider
		tracerProvider = cfg.sdkTracerProvider
		if tracerProvider == nil {
			traceClient := otlptracegrpc.NewClient(traceClientOpts...)

			// trace exporter
			traceExp, err := otlptrace.New(ctx, traceClient)
			if err != nil {
				return nil, fmt.Errorf("failed to create otlp trace exporter: %v", err)
			}

			bsp := sdktrace.NewBatchSpanProcessor(traceExp)

			tracerProvider = sdktrace.NewTracerProvider(
				sdktrace.WithSampler(cfg.sampler),
				sdktrace.WithResource(res),
				sdktrace.WithSpanProcessor(bsp),
			)
		}
	}

	// Metrics
	if cfg.enableMetrics {
		// prometheus only supports CumulativeTemporalitySelector

		var metricsClientOpts []otlpmetricgrpc.Option
		if cfg.exportEndpoint != "" {
			metricsClientOpts = append(metricsClientOpts, otlpmetricgrpc.WithEndpoint(cfg.exportEndpoint))
		}
		if len(cfg.exportHeaders) > 0 {
			metricsClientOpts = append(metricsClientOpts, otlpmetricgrpc.WithHeaders(cfg.exportHeaders))
		}
		if cfg.exportInsecure {
			metricsClientOpts = append(metricsClientOpts, otlpmetricgrpc.WithInsecure())
		}

		meterProvider = cfg.meterProvider
		if meterProvider == nil {
			// metrics exporter
			metricExp, err := otlpmetricgrpc.New(context.Background(), metricsClientOpts...)
			if err != nil {
				return nil, fmt.Errorf("failed to create otlp metric exporter: %v", err)
			}

			reader := metric.WithReader(metric.NewPeriodicReader(metricExp, metric.WithInterval(15*time.Second)))

			meterProvider = metric.NewMeterProvider(reader, metric.WithResource(res))
		}
	}

	return &OtelProvider{
		TracerProvider: tracerProvider,
		MeterProvider:  meterProvider,
	}, nil
}

func newResource(cfg *config) *resource.Resource {
	if cfg.resource != nil {
		return cfg.resource
	}

	res, err := resource.New(
		context.Background(),
		resource.WithHost(),
		resource.WithFromEnv(),
		resource.WithProcessPID(),
		resource.WithTelemetrySDK(),
		resource.WithDetectors(cfg.resourceDetectors...),
		resource.WithAttributes(cfg.resourceAttributes...),
	)
	if err != nil {
		return resource.Default()
	}
	return res
}

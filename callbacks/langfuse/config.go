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
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	defaultServiceName             = "eino-langfuse"
	defaultTimeout                 = 10 * time.Second
	defaultMaxAttributeValueLength = 1_000_000
	defaultMaxSpanAttributeBytes   = 4_000_000
	instrumentationName            = "github.com/cloudwego/eino-ext/callbacks/langfuse"
	instrumentationVersion         = "v0.2.0"
)

// Config configures the Langfuse OTLP/HTTP callback.
type Config struct {
	// Host accepts a Langfuse base URL, an OTLP base URL ending in
	// /api/public/otel, or the full /api/public/otel/v1/traces endpoint.
	Host string

	PublicKey string
	SecretKey string

	ServiceName string
	Environment string
	Release     string
	Version     string
	Tags        []string
	Timeout     time.Duration
	SampleRate  float64

	// Name, UserID, and SessionID configure default root trace attributes.
	// Prefer the corresponding StartTrace options for each trace.
	Name      string
	UserID    string
	SessionID string
	Public    bool

	// Deprecated batching fields are retained for source compatibility with the
	// legacy ingestion client. New code should use MaxQueueSize,
	// MaxExportBatchSize, BatchTimeout, and the OTLP exporter's built-in retry.
	Threads           int
	MaxTaskQueueSize  int
	MaxEventSizeBytes int
	FlushAt           int
	FlushInterval     time.Duration
	LogMessage        string
	MaxRetry          uint64
	// IncludeProcessResourceAttributes adds OTel process.* attributes such as
	// PID, owner, executable path, command arguments, and runtime details. It is
	// disabled by default to reduce metadata noise and accidental disclosure.
	IncludeProcessResourceAttributes bool
	// CollapseAgentInternalSpans suppresses implementation-detail spans emitted
	// by Eino agents: the same-name Chain beneath an Agent, the ReAct Graph and
	// Init Lambda it owns, and unnamed/default-named Lambda wrappers. It is
	// disabled by default so the callback preserves complete framework traces
	// unless the caller explicitly opts into a compact hierarchy. Matching
	// components outside an Agent and named business Lambdas are always retained.
	CollapseAgentInternalSpans bool

	MaxQueueSize       int
	MaxExportBatchSize int
	BatchTimeout       time.Duration
	// DropLogInterval controls how often discarded telemetry is aggregated into
	// one warning. Zero uses one minute. A negative value logs each discard
	// immediately; export failures are always logged immediately.
	DropLogInterval time.Duration
	// QueueDropLogInterval is retained for compatibility. DropLogInterval takes
	// precedence when both are set.
	// Deprecated: use DropLogInterval.
	QueueDropLogInterval time.Duration

	// MaxAttributeValueLength limits each serialized input, output, or metadata
	// value. Set a negative value to disable this callback-level limit.
	MaxAttributeValueLength int
	// MaxSpanAttributeBytes limits the combined keys and values written by this
	// callback to one span. Set a negative value to disable the span-level budget.
	MaxSpanAttributeBytes int
	MaskFunc              func(string) string
	HTTPClient            *http.Client

	// SpanExporter is intended for custom transports and tests. When set, Host
	// and API keys are not used to construct an exporter.
	SpanExporter sdktrace.SpanExporter

	// TracerProvider lets callers supply an existing provider. The callback does
	// not shut down a caller-owned provider. Custom trace IDs requested through
	// WithID are only guaranteed when the callback creates the provider.
	TracerProvider *sdktrace.TracerProvider
}

// CallbackHandler converts Eino callbacks into Langfuse-compatible OTEL spans.
type CallbackHandler struct {
	tracer                     trace.Tracer
	provider                   *sdktrace.TracerProvider
	ownProvider                bool
	prepare                    func(string) string
	maxSpanAttributeBytes      int
	collapseAgentInternalSpans bool
	legacyAutoTrace            bool
	traceDefaults              traceOptions
	async                      asyncGroup
	activeTraceRun             traceRunRegistry
	losses                     *lossReporter
}

// NewHandler creates an Eino callback that exports OTLP/HTTP traces using the
// Langfuse v4 ingestion path.
func NewHandler(ctx context.Context, cfg *Config) (*CallbackHandler, error) {
	if cfg == nil {
		return nil, errors.New("langfuse config is nil")
	}
	cfg = normalizedConfig(cfg)
	if cfg.SampleRate < 0 || cfg.SampleRate > 1 {
		return nil, fmt.Errorf("langfuse sample rate must be between 0 and 1, got %v", cfg.SampleRate)
	}

	dropLogInterval := cfg.DropLogInterval
	if dropLogInterval == 0 {
		dropLogInterval = cfg.QueueDropLogInterval
	}
	losses := newLossReporter(dropLogInterval, nil)
	succeeded := false
	defer func() {
		if !succeeded {
			losses.Stop()
		}
	}()

	provider := cfg.TracerProvider
	ownProvider := false
	if provider == nil {
		exporter := cfg.SpanExporter
		if exporter == nil {
			var err error
			exporter, err = newHTTPExporter(ctx, cfg)
			if err != nil {
				return nil, err
			}
		}

		resourceOptions := []resource.Option{
			resource.WithFromEnv(),
			resource.WithTelemetrySDK(),
			resource.WithAttributes(semconv.ServiceName(defaultString(cfg.ServiceName, defaultServiceName))),
		}
		if cfg.IncludeProcessResourceAttributes {
			resourceOptions = append(resourceOptions, resource.WithProcess())
		}
		res, err := resource.New(ctx, resourceOptions...)
		if err != nil {
			return nil, fmt.Errorf("create langfuse resource: %w", err)
		}

		processor, err := newReportingBatchSpanProcessor(exporter, batchProcessorConfig{
			maxQueueSize:       cfg.MaxQueueSize,
			maxExportBatchSize: cfg.MaxExportBatchSize,
			batchTimeout:       cfg.BatchTimeout,
			losses:             losses,
		})
		if err != nil {
			return nil, err
		}

		provider = sdktrace.NewTracerProvider(
			sdktrace.WithIDGenerator(contextIDGenerator{}),
			sdktrace.WithResource(res),
			sdktrace.WithSampler(reportingSampler{delegate: sampler(cfg.SampleRate), losses: losses}),
			sdktrace.WithSpanProcessor(processor),
		)
		ownProvider = true
	}

	maxLength := cfg.MaxAttributeValueLength
	if maxLength == 0 {
		maxLength = defaultMaxAttributeValueLength
	}
	maxSpanAttributeBytes := cfg.MaxSpanAttributeBytes
	if maxSpanAttributeBytes == 0 {
		maxSpanAttributeBytes = defaultMaxSpanAttributeBytes
	}
	handler := &CallbackHandler{
		tracer: provider.Tracer(
			instrumentationName,
			trace.WithInstrumentationVersion(instrumentationVersion),
		),
		provider:                   provider,
		ownProvider:                ownProvider,
		prepare:                    newValuePreparer(cfg.MaskFunc, maxLength, losses),
		maxSpanAttributeBytes:      maxSpanAttributeBytes,
		collapseAgentInternalSpans: cfg.CollapseAgentInternalSpans,
		traceDefaults: traceOptions{
			Name:        cfg.Name,
			UserID:      cfg.UserID,
			SessionID:   cfg.SessionID,
			Environment: cfg.Environment,
			Release:     cfg.Release,
			Version:     cfg.Version,
			Tags:        append([]string(nil), cfg.Tags...),
			Public:      cfg.Public,
		},
		async:  newAsyncGroup(),
		losses: losses,
	}
	succeeded = true
	return handler, nil
}

func normalizedConfig(cfg *Config) *Config {
	normalized := *cfg
	if normalized.MaxQueueSize == 0 {
		normalized.MaxQueueSize = cfg.MaxTaskQueueSize
	}
	if normalized.MaxExportBatchSize == 0 {
		normalized.MaxExportBatchSize = cfg.FlushAt
	}
	if normalized.BatchTimeout == 0 {
		normalized.BatchTimeout = cfg.FlushInterval
	}
	if normalized.MaxSpanAttributeBytes == 0 {
		normalized.MaxSpanAttributeBytes = cfg.MaxEventSizeBytes
	}
	return &normalized
}

func newHTTPExporter(ctx context.Context, cfg *Config) (sdktrace.SpanExporter, error) {
	endpoint, err := traceEndpoint(cfg.Host)
	if err != nil {
		return nil, err
	}
	if cfg.PublicKey == "" || cfg.SecretKey == "" {
		return nil, errors.New("langfuse public key and secret key are required")
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	auth := base64.StdEncoding.EncodeToString([]byte(cfg.PublicKey + ":" + cfg.SecretKey))
	exporterOptions := []otlptracehttp.Option{
		otlptracehttp.WithEndpointURL(endpoint),
		otlptracehttp.WithHeaders(map[string]string{
			"Authorization":                "Basic " + auth,
			"x-langfuse-ingestion-version": "4",
		}),
		otlptracehttp.WithTimeout(timeout),
		otlptracehttp.WithCompression(otlptracehttp.GzipCompression),
	}
	if cfg.HTTPClient != nil {
		exporterOptions = append(exporterOptions, otlptracehttp.WithHTTPClient(cfg.HTTPClient))
	}
	exporter, err := otlptracehttp.New(ctx, exporterOptions...)
	if err != nil {
		return nil, fmt.Errorf("create langfuse OTLP HTTP exporter: %w", err)
	}
	return exporter, nil
}

func traceEndpoint(host string) (string, error) {
	host = strings.TrimSpace(strings.TrimRight(host, "/"))
	if host == "" {
		return "", errors.New("langfuse host is required")
	}
	parsed, err := url.Parse(host)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid langfuse host %q", host)
	}
	switch {
	case strings.HasSuffix(parsed.Path, "/api/public/otel/v1/traces"):
	case strings.HasSuffix(parsed.Path, "/api/public/otel"):
		parsed.Path += "/v1/traces"
	default:
		parsed.Path += "/api/public/otel/v1/traces"
	}
	return parsed.String(), nil
}

func sampler(sampleRate float64) sdktrace.Sampler {
	if sampleRate <= 0 || sampleRate >= 1 {
		return sdktrace.ParentBased(sdktrace.AlwaysSample())
	}
	return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(sampleRate))
}

type reportingSampler struct {
	delegate sdktrace.Sampler
	losses   *lossReporter
}

func (s reportingSampler) ShouldSample(parameters sdktrace.SamplingParameters) sdktrace.SamplingResult {
	result := s.delegate.ShouldSample(parameters)
	if result.Decision == sdktrace.Drop {
		s.losses.Add(lossSampledOut, 1)
	}
	return result
}

func (s reportingSampler) Description() string {
	return "langfuse-reporting(" + s.delegate.Description() + ")"
}

func defaultString(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

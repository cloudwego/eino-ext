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
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/cloudwego/eino-ext/libs/acl/opentelemetry"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/volcengine/volc-sdk-golang/service/tls/pb"
	"github.com/volcengine/volc-sdk-golang/service/tls/producer"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
	oteltrace "go.opentelemetry.io/otel/trace"
)

const (
	tlsAppType       = "eino"
	tlsRootSpanName  = "agent.turn"
	tlsModelSpanName = "llm.request"
	tlsToolSpanName  = "tool.call"
	tlsTraceFileName = "eino-trace"
)

// tlsLogExporter converts finished OpenTelemetry spans into the TLS trace
// envelope and publishes them through TLS Producer SendLogs.
type tlsLogExporter struct {
	producer tlsLogProducer
	topicID  string
	source   string
}

type tlsLogProducer interface {
	SendLog(shardHash, topic, source, filename string, log *pb.Log, callback producer.CallBack) error
	Close()
}

type tlsProducerCallback struct {
	once sync.Once
	done chan error
}

func newTLSProducerCallback() *tlsProducerCallback {
	return &tlsProducerCallback{done: make(chan error, 1)}
}

func (c *tlsProducerCallback) Success(_ *producer.Result) {
	c.once.Do(func() { c.done <- nil })
}

func (c *tlsProducerCallback) Fail(result *producer.Result) {
	c.once.Do(func() { c.done <- tlsProducerResultError(result) })
}

func tlsProducerResultError(result *producer.Result) error {
	if result == nil || len(result.Attempts) == 0 {
		return fmt.Errorf("TLS Producer SendLog failed without an attempt result")
	}
	last := result.Attempts[len(result.Attempts)-1]
	return fmt.Errorf("TLS Producer SendLog failed: request_id=%s error_code=%s error_message=%s", last.RequestId, last.ErrorCode, last.ErrorMessage)
}

func (e *tlsLogExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	if len(spans) == 0 {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	callbacks := make([]*tlsProducerCallback, 0, len(spans))
	for _, span := range spans {
		log, err := spanToTLSLog(span)
		if err != nil {
			return err
		}
		callback := newTLSProducerCallback()
		if err := e.producer.SendLog("", e.topicID, e.source, tlsTraceFileName, log, callback); err != nil {
			return fmt.Errorf("enqueue TLS span: %w", err)
		}
		callbacks = append(callbacks, callback)
	}

	for _, callback := range callbacks {
		select {
		case err := <-callback.done:
			if err != nil {
				return err
			}
		case <-ctx.Done():
			return fmt.Errorf("wait for TLS Producer SendLog callback: %w", ctx.Err())
		}
	}
	return nil
}

func (e *tlsLogExporter) Shutdown(_ context.Context) error {
	if e != nil && e.producer != nil {
		e.producer.Close()
	}
	return nil
}

func newTLSLogProvider(cfg *TLSConfig) (*opentelemetry.OtelProvider, error) {
	exporter, err := newTLSLogExporter(cfg)
	if err != nil {
		return nil, err
	}

	res, err := resource.New(
		context.Background(),
		resource.WithHost(),
		resource.WithFromEnv(),
		resource.WithProcessPID(),
		resource.WithTelemetrySDK(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(cfg.AppName),
			attribute.String("tls.business_type", "gen_ai"),
			attribute.String("tls.app.type", tlsAppType),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create OpenTelemetry resource: %w", err)
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(exporter),
	)
	return &opentelemetry.OtelProvider{TracerProvider: tracerProvider}, nil
}

func newTLSLogExporter(cfg *TLSConfig) (*tlsLogExporter, error) {
	if cfg == nil {
		return nil, fmt.Errorf("TLS config cannot be nil")
	}

	producerConfig := producer.GetDefaultProducerConfig()
	producerConfig.Endpoint = normalizeTLSProducerEndpoint(cfg.TLSLogEndpoint)
	producerConfig.Region = cfg.TLSLogRegion
	producerConfig.NoRetryStatusCodeList = []int{400, 401, 403, 404}
	if cfg.TLSLogAPIKey != "" {
		producerConfig.APIKey = cfg.TLSLogAPIKey
	} else {
		producerConfig.AccessKeyID = cfg.TLSLogAccessKeyID
		producerConfig.AccessKeySecret = cfg.TLSLogAccessKeySecret
	}

	source, err := os.Hostname()
	if err != nil || source == "" {
		source = "127.0.0.1"
	}

	p := producer.NewProducer(producerConfig)
	p.Start()
	return &tlsLogExporter{producer: p, topicID: cfg.TLSLogTopicID, source: source}, nil
}

func normalizeTLSProducerEndpoint(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	endpoint = strings.TrimRight(endpoint, "/")
	if endpoint == "" || strings.HasPrefix(endpoint, "https://") || strings.HasPrefix(endpoint, "http://") {
		return endpoint
	}
	return "https://" + endpoint
}

func spanToTLSLog(span sdktrace.ReadOnlySpan) (*pb.Log, error) {
	attributesJSON, err := json.Marshal(otelAttributesToMap(span.Attributes()))
	if err != nil {
		return nil, fmt.Errorf("serialize span attributes: %w", err)
	}
	resourceJSON, err := json.Marshal(otelAttributesToMap(span.Resource().Attributes()))
	if err != nil {
		return nil, fmt.Errorf("serialize span resource: %w", err)
	}

	status := "OK"
	if span.Status().Code == codes.Error {
		status = "ERROR"
	}
	resourceAttrs := otelAttributesToMap(span.Resource().Attributes())
	serviceName, _ := resourceAttrs["service.name"].(string)
	host, _ := resourceAttrs["host.name"].(string)
	scope := span.InstrumentationScope()
	// OpenTelemetry represents a root span's missing parent as the all-zero
	// SpanID. TLS TraceTableV2 identifies root spans with an empty
	// ParentSpanID, so preserve that distinction at the transport boundary.
	parentSpanID := ""
	if parent := span.Parent(); parent.IsValid() {
		parentSpanID = parent.SpanID().String()
	}

	contents := map[string]string{
		"TraceID":           span.SpanContext().TraceID().String(),
		"SpanID":            span.SpanContext().SpanID().String(),
		"ParentSpanID":      parentSpanID,
		"Name":              span.Name(),
		"Kind":              tlsSpanKind(span.SpanKind()),
		"Start":             fmt.Sprintf("%d", span.StartTime().UnixMicro()),
		"End":               fmt.Sprintf("%d", span.EndTime().UnixMicro()),
		"Duration":          fmt.Sprintf("%d", span.EndTime().Sub(span.StartTime()).Microseconds()),
		"StatusCode":        status,
		"StatusDescription": span.Status().Description,
		"Attributes":        string(attributesJSON),
		"Resource":          string(resourceJSON),
		"ServiceName":       serviceName,
		"Host":              host,
		"OTLPName":          scope.Name,
		"OTLPVersion":       scope.Version,
		"Events":            "[]",
		"Links":             "[]",
		"TraceState":        span.SpanContext().TraceState().String(),
	}
	logContents := make([]*pb.LogContent, 0, len(contents))
	for key, value := range contents {
		logContents = append(logContents, &pb.LogContent{Key: key, Value: value})
	}
	return &pb.Log{Time: span.StartTime().Unix(), Contents: logContents}, nil
}

func otelAttributesToMap(attributes []attribute.KeyValue) map[string]any {
	result := make(map[string]any, len(attributes))
	for _, kv := range attributes {
		result[string(kv.Key)] = kv.Value.AsInterface()
	}
	return result
}

func tlsSpanName(ctx context.Context, info *callbacks.RunInfo) string {
	if info == nil {
		return tlsRootSpanName
	}
	switch info.Component {
	case components.ComponentOfChatModel:
		return tlsModelSpanName
	case components.ComponentOfTool:
		return tlsToolSpanName
	default:
		if ctx.Value(TLSStateKey{}) == nil {
			return tlsRootSpanName
		}
		return getSpanName(info)
	}
}

func tlsSpanKind(kind oteltrace.SpanKind) string {
	switch kind {
	case oteltrace.SpanKindServer:
		return "server"
	case oteltrace.SpanKindClient:
		return "client"
	case oteltrace.SpanKindProducer:
		return "producer"
	case oteltrace.SpanKindConsumer:
		return "consumer"
	default:
		return "internal"
	}
}

func tlsOperationName(info *callbacks.RunInfo, isRoot bool) string {
	if isRoot {
		return "invoke_agent"
	}
	if info == nil {
		return "invoke_agent"
	}
	switch info.Component {
	case components.ComponentOfChatModel:
		return "chat"
	case components.ComponentOfTool:
		return "execute_tool"
	default:
		return "invoke"
	}
}

func (cfg *TLSConfig) usesTLSLogTransport() bool {
	return cfg != nil && cfg.TLSExporterEnabled && cfg.TLSLogTopicID != ""
}

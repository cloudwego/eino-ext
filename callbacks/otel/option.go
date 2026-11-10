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

package otel

// Option configures the OpenTelemetry callback handler.
type Option func(*options)

type options struct {
	// tracerName is the instrumentation name passed to TracerProvider.Tracer.
	// It becomes the otel.scope.name attribute on every emitted span.
	tracerName string
}

func defaultOptions() *options {
	return &options{
		tracerName: "github.com/cloudwego/eino-ext/callbacks/otel",
	}
}

// WithTracerName overrides the instrumentation (scope) name used to obtain the
// tracer from the TracerProvider. This shows up as the otel.scope.name
// attribute on emitted spans and is useful when multiple Eino integrations
// share a single TracerProvider.
func WithTracerName(name string) Option {
	return func(o *options) {
		if name != "" {
			o.tracerName = name
		}
	}
}

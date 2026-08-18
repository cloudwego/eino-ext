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
	"log"

	"github.com/cloudwego/eino/callbacks"
)

type legacyTraceOptionKey struct{}

// NewLangfuseHandler preserves the legacy constructor while using the OTLP
// implementation. New code should use NewHandler so initialization errors can
// be handled explicitly and call Shutdown during application termination.
func NewLangfuseHandler(cfg *Config) (*CallbackHandler, func()) {
	handler, err := NewHandler(context.Background(), cfg)
	if err != nil {
		panic(err)
	}
	handler.legacyAutoTrace = true
	flushTimeout := defaultTimeout
	if cfg != nil && cfg.Timeout > 0 {
		flushTimeout = cfg.Timeout
	}
	return handler, func() {
		ctx, cancel := context.WithTimeout(context.Background(), flushTimeout)
		defer cancel()
		if err := handler.Flush(ctx); err != nil {
			log.Printf("langfuse callback legacy flush failed: %v", err)
		}
	}
}

// SetTrace preserves the legacy context API. With a handler created by
// NewLangfuseHandler, the first Eino callback creates an automatically managed
// root observation using these options. New code should call StartTrace and
// EndTrace explicitly.
func SetTrace(ctx context.Context, opts ...TraceOption) context.Context {
	options := &traceOptions{}
	for _, opt := range opts {
		opt(options)
	}
	return context.WithValue(ctx, legacyTraceOptionKey{}, options)
}

// UpdateTraceOutput ends the active OTLP root found in ctx with output. OTLP
// spans are immutable after export, so traceID cannot be used to update an
// already completed trace. New code should retain the StartTrace context and
// call EndTrace directly.
func (c *CallbackHandler) UpdateTraceOutput(ctx context.Context, traceID, output string) {
	run, _ := ctx.Value(traceRunKey{}).(*traceRun)
	if run == nil {
		log.Printf("langfuse callback cannot update completed OTLP trace %q; call EndTrace with the active trace context", traceID)
		return
	}
	c.EndTrace(ctx, output)
}

func (c *CallbackHandler) ensureLegacyTrace(ctx context.Context, info *callbacks.RunInfo) context.Context {
	if !c.legacyAutoTrace {
		return ctx
	}
	if run, _ := ctx.Value(traceRunKey{}).(*traceRun); run != nil {
		return ctx
	}

	configured, hasConfigured := ctx.Value(legacyTraceOptionKey{}).(*traceOptions)
	rootCtx := c.StartTrace(ctx, func(options *traceOptions) {
		if hasConfigured {
			*options = cloneTraceOptions(configured)
			return
		}
		if options.Name == "" {
			options.Name = callbackName(info)
		}
	})
	run, _ := rootCtx.Value(traceRunKey{}).(*traceRun)
	if run != nil {
		run.mu.Lock()
		run.autoEnd = true
		run.mu.Unlock()
	}
	return rootCtx
}

func cloneTraceOptions(options *traceOptions) traceOptions {
	cloned := *options
	cloned.Tags = append([]string(nil), options.Tags...)
	if options.Metadata != nil {
		cloned.Metadata = make(map[string]any, len(options.Metadata))
		for key, value := range options.Metadata {
			cloned.Metadata[key] = value
		}
	}
	return cloned
}

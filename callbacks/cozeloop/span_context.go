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

package cozeloop

import (
	"context"

	"github.com/coze-dev/cozeloop-go"
)

type spanContext struct {
	spanID  string
	traceID string
	baggage map[string]string
	isSet   bool
}

func (s *spanContext) GetSpanID() string {
	return s.spanID
}

func (s *spanContext) GetTraceID() string {
	return s.traceID
}

func (s *spanContext) GetBaggage() map[string]string {
	return s.baggage
}

type spanContextKey struct{}

func WithSpanContext(ctx context.Context) context.Context {
	s := &spanContext{}
	ctx = context.WithValue(ctx, spanContextKey{}, s)
	return ctx
}

func getSpanContextImpl(ctx context.Context) *spanContext {
	ctxV := ctx.Value(spanContextKey{})
	if ctxV == nil {
		return nil
	}
	sc, ok := ctxV.(*spanContext)
	if !ok {
		return nil
	}

	return sc
}

func GetSpanContext(ctx context.Context) cozeloop.SpanContext {
	res := getSpanContextImpl(ctx)
	if res == nil {
		return cozeloop.DefaultNoopSpan
	}

	return &spanContext{
		spanID:  res.spanID,
		traceID: res.traceID,
		baggage: res.baggage,
	}
}

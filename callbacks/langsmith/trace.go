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

package langsmith

import "context"

type langsmithTraceOptionKey struct{}

type traceOptions struct {
	SessionName        string
	ReferenceExampleID string
	TraceID            string
	Metadata           map[string]interface{}
	ParentID           string
	ParentDottedOrder  string
}

type TraceOption func(*traceOptions)

// SetTrace 将 trace 选项设置到 context 中
func SetTrace(ctx context.Context, opts ...TraceOption) context.Context {
	options := &traceOptions{}
	for _, opt := range opts {
		opt(options)
	}
	return context.WithValue(ctx, langsmithTraceOptionKey{}, options)
}

// WithSessionName 设置 Langsmith 的项目名称
func WithSessionName(name string) TraceOption {
	return func(o *traceOptions) {
		o.SessionName = name
	}
}

// WithReferenceExampleID 关联到一个 example
func WithReferenceExampleID(id string) TraceOption {
	return func(o *traceOptions) {
		o.ReferenceExampleID = id
	}
}

// WithTraceID 强制指定一个 trace ID
func WithTraceID(id string) TraceOption {
	return func(o *traceOptions) {
		o.TraceID = id
	}
}

// WithMetadata 设置 trace 的元数据
func WithMetadata(metadata map[string]interface{}) TraceOption {
	return func(o *traceOptions) {
		o.Metadata = metadata
	}
}
